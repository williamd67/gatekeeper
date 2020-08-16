package main

import (
	"fmt"
	"sync"
	"time"

	accesslog "github.com/envoyproxy/go-control-plane/envoy/config/accesslog/v3"
	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	listener "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	fileaccesslog "github.com/envoyproxy/go-control-plane/envoy/extensions/access_loggers/file/v3"
	grpcaccesslog "github.com/envoyproxy/go-control-plane/envoy/extensions/access_loggers/grpc/v3"
	extauthz "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/ext_authz/v3"
	hcm "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	tls "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	cache "github.com/envoyproxy/go-control-plane/pkg/cache/types"
	"github.com/envoyproxy/go-control-plane/pkg/wellknown"
	"github.com/golang/protobuf/ptypes"
	log "github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/erikbos/gatekeeper/pkg/shared"
)

const (
	virtualHostDataRefreshInterval = 2 * time.Second
	extAuthzTimeout                = 100 * time.Millisecond
)

// FIXME this does not detect removed records
// GetVirtualHostConfigFromDatabase continuously gets the current configuration
func (s *server) GetVirtualHostConfigFromDatabase(n chan xdsNotifyMesssage) {
	var virtualHostsLastUpdate int64
	var virtualHostsMutex sync.Mutex

	for {
		var xdsPushNeeded bool

		newVirtualHosts, err := s.db.Virtualhost.GetAll()
		if err != nil {
			log.Errorf("Could not retrieve virtualhosts from database (%s)", err)
		} else {
			if virtualHostsLastUpdate == 0 {
				log.Info("Initial load of virtualhosts")
			}
			for _, virtualhost := range newVirtualHosts {
				// Is a virtualhosts updated since last time we stored it?
				if virtualhost.LastmodifiedAt > virtualHostsLastUpdate {
					virtualHostsMutex.Lock()
					s.virtualhosts = newVirtualHosts
					virtualHostsMutex.Unlock()

					virtualHostsLastUpdate = shared.GetCurrentTimeMilliseconds()
					xdsPushNeeded = true

					warnForUnknownVirtualHostAttributes(virtualhost)
				}
			}
		}
		if xdsPushNeeded {
			n <- xdsNotifyMesssage{
				resource: "virtualhost",
			}
			// Increase xds deployment metric
			s.metrics.xdsDeployments.WithLabelValues("virtualhosts").Inc()
		}
		time.Sleep(virtualHostDataRefreshInterval)
	}
}

// GetVirtualHostCount returns number of virtualhosts
func (s *server) GetVirtualHostCount() float64 {
	return float64(len(s.virtualhosts))
}

// getEnvoyListenerConfig returns array of envoy listeners
func (s *server) getEnvoyListenerConfig() ([]cache.Resource, error) {
	envoyListeners := []cache.Resource{}

	uniquePorts := s.getVirtualHostPorts()
	for port := range uniquePorts {
		log.Infof("Adding listener & vhosts on port %d", port)
		envoyListeners = append(envoyListeners, s.buildEnvoyListenerConfig(port))
	}
	return envoyListeners, nil
}

// getVirtualHostPorts return unique set of ports from vhost configuration
func (s *server) getVirtualHostPorts() map[int]bool {
	listenerPorts := map[int]bool{}
	for _, virtualhost := range s.virtualhosts {
		listenerPorts[virtualhost.Port] = true
	}
	return listenerPorts
}

// getVirtualHostPorts return unique set of ports from vhost configuration
func (s *server) getVirtualHostsOfRouteGroup(RouteGroupName string) []string {
	var virtualHostsInRouteGroup []string

	for _, virtualhost := range s.virtualhosts {
		if virtualhost.RouteGroup == RouteGroupName {
			virtualHostsInRouteGroup = append(virtualHostsInRouteGroup, virtualhost.VirtualHosts...)
		}
	}
	return virtualHostsInRouteGroup
}

func (s *server) buildEnvoyListenerConfig(port int) *listener.Listener {

	newListener := &listener.Listener{
		Name:            fmt.Sprintf("port_%d", port),
		Address:         buildAddress("0.0.0.0", port),
		ListenerFilters: buildListenerFilterHTTP(),
	}

	// add all vhosts belonging to this listener's port
	for _, virtualhost := range s.virtualhosts {
		if virtualhost.Port == port {
			newListener.FilterChains = append(newListener.FilterChains, s.buildFilterChainEntry(newListener, virtualhost))
		}
	}

	return newListener
}

func buildListenerFilterHTTP() []*listener.ListenerFilter {

	return []*listener.ListenerFilter{
		{
			Name: wellknown.HttpInspector,
		},
	}
}

func (s *server) buildFilterChainEntry(l *listener.Listener, v shared.VirtualHost) *listener.FilterChain {

	httpFilter := s.buildFilter()

	manager := s.buildConnectionManager(httpFilter, v)
	managerProtoBuf, err := ptypes.MarshalAny(manager)
	if err != nil {
		log.Panic(err)
	}

	FilterChainEntry := &listener.FilterChain{
		Filters: []*listener.Filter{{
			Name: wellknown.HTTPConnectionManager,
			ConfigType: &listener.Filter_TypedConfig{
				TypedConfig: managerProtoBuf,
			},
		}},
	}

	// Check if we have a certificate and certificate key
	_, certificateError := v.Attributes.Get(attributeTLSCertificate)
	_, certificateKeyError := v.Attributes.Get(attributeTLSCertificateKey)

	// No certificate details, return and do not try to enable TLS
	if certificateError != nil && certificateKeyError != nil {
		return FilterChainEntry
	}

	// Enable TLS protocol on listener
	l.ListenerFilters = []*listener.ListenerFilter{
		{
			Name: wellknown.TlsInspector,
		},
	}

	// Configure listener to use SNI to match against vhost names
	FilterChainEntry.FilterChainMatch =
		&listener.FilterChainMatch{
			ServerNames: v.VirtualHosts,
		}

	// Set TLS configuration based upon virtualhosts attributes
	downStreamTLSConfig := &tls.DownstreamTlsContext{
		CommonTlsContext: buildCommonTLSContext(v.Name, v.Attributes),
	}
	FilterChainEntry.TransportSocket = buildTransportSocket(v.Name, downStreamTLSConfig)

	return FilterChainEntry
}

func (s *server) buildConnectionManager(httpFilters []*hcm.HttpFilter, v shared.VirtualHost) *hcm.HttpConnectionManager {

	return &hcm.HttpConnectionManager{
		CodecType:        hcm.HttpConnectionManager_AUTO,
		StatPrefix:       "ingress_http",
		UseRemoteAddress: protoBool(true),
		RouteSpecifier:   s.buildRouteSpecifier(v.RouteGroup),
		HttpFilters:      httpFilters,
		AccessLog:        buildAccessLog(s.config.Envoyproxy.Logging, v),
	}
}

func (s *server) buildFilter() []*hcm.HttpFilter {

	return []*hcm.HttpFilter{
		{
			Name: wellknown.HTTPExternalAuthorization,
			ConfigType: &hcm.HttpFilter_TypedConfig{
				TypedConfig: s.buildExtAuthzFilterConfig(),
			},
		},
		{
			Name: wellknown.CORS,
		},
		{
			Name: wellknown.Router,
		},
	}
}

func (s *server) buildExtAuthzFilterConfig() *anypb.Any {
	if s.config.Envoyproxy.ExtAuthz.Cluster == "" {
		return nil
	}

	extAuthz := &extauthz.ExtAuthz{
		FailureModeAllow: s.config.Envoyproxy.ExtAuthz.FailureModeAllow,
		Services: &extauthz.ExtAuthz_GrpcService{
			GrpcService: buildGRPCService(s.config.Envoyproxy.ExtAuthz.Cluster,
				s.config.Envoyproxy.ExtAuthz.Timeout),
		},
		TransportApiVersion: core.ApiVersion_V3,
	}
	if s.config.Envoyproxy.ExtAuthz.RequestBodySize > 0 {
		extAuthz.WithRequestBody = s.extAuthzWithRequestBody()
	}

	extAuthzTypedConf, err := ptypes.MarshalAny(extAuthz)
	if err != nil {
		log.Panic(err)
	}
	return extAuthzTypedConf
}

func (s *server) extAuthzWithRequestBody() *extauthz.BufferSettings {
	if s.config.Envoyproxy.ExtAuthz.RequestBodySize > 0 {
		return &extauthz.BufferSettings{
			MaxRequestBytes:     uint32(s.config.Envoyproxy.ExtAuthz.RequestBodySize),
			AllowPartialMessage: false,
		}
	}
	return nil
}

func (s *server) buildRouteSpecifier(RouteGroup string) *hcm.HttpConnectionManager_RouteConfig {
	return &hcm.HttpConnectionManager_RouteConfig{
		RouteConfig: s.buildEnvoyVirtualHostRouteConfig(RouteGroup, s.routes),
	}
}

// func buildRouteSpecifierRDS(RouteGroup string) *hcm.HttpConnectionManager_Rds {
// 	// TODO
// 	XdsCluster := "xds_cluster"

// 	return &hcm.HttpConnectionManager_Rds{
// 		Rds: &hcm.Rds{
// 			RouteConfigName: RouteGroup,
// 			ConfigSource: &core.ConfigSource{
// 				ConfigSourceSpecifier: &core.ConfigSource_ApiConfigSource{
// 					ApiConfigSource: &core.ApiConfigSource{
// 						ApiType: core.ApiConfigSource_GRPC,
// 						GrpcServices: []*core.GrpcService{{
// 							TargetSpecifier: &core.GrpcService_EnvoyGrpc_{
// 								EnvoyGrpc: &core.GrpcService_EnvoyGrpc{ClusterName: XdsCluster},
// 							},
// 						}},
// 					},
// 				},
// 			},
// 		},
// 	}
// }

func buildAccessLog(config envoyLogConfig, v shared.VirtualHost) []*accesslog.AccessLog {

	// Set access log behaviour based upon virtual host attributes
	accessLogFileName, error := v.Attributes.Get(attributeAccessLogFileName)
	if error == nil && accessLogFileName != "" {
		return buildFileAccessLog(config.File.Fields, accessLogFileName)
	}
	accessLogClusterName, error := v.Attributes.Get(attributeAccessLogClusterName)
	if error == nil && accessLogClusterName != "" {
		return buildGRPCAccessLog(accessLogClusterName, v.Name, defaultClusterConnectTimeout, 0)
	}

	// Fallback is default logging based upon configfile
	if config.File.LogFileName != "" {
		return buildFileAccessLog(config.File.Fields, config.File.LogFileName)
	}
	if config.GRPC.Cluster != "" {
		return buildGRPCAccessLog(config.GRPC.Cluster, v.Name, config.GRPC.Timeout, config.GRPC.BufferSize)
	}
	return nil
}

func buildFileAccessLog(fields map[string]string, path string) []*accesslog.AccessLog {

	if len(fields) == 0 {
		log.Warnf("To do file access logging a logfield definition in envoycp configuration is required")
		return nil
	}

	jsonFormat := &structpb.Struct{
		Fields: map[string]*structpb.Value{},
	}
	for field, fieldFormat := range fields {
		if fieldFormat != "" {
			jsonFormat.Fields[field] = &structpb.Value{
				Kind: &structpb.Value_StringValue{StringValue: fieldFormat},
			}
		}
	}
	accessLogConf := &fileaccesslog.FileAccessLog{
		Path: path,
		AccessLogFormat: &fileaccesslog.FileAccessLog_LogFormat{
			LogFormat: &core.SubstitutionFormatString{
				Format: &core.SubstitutionFormatString_JsonFormat{
					JsonFormat: jsonFormat,
				},
			},
		},
	}
	accessLogTypedConf, err := ptypes.MarshalAny(accessLogConf)
	if err != nil {
		log.Panic(err)
	}
	return []*accesslog.AccessLog{{
		Name: wellknown.FileAccessLog,
		ConfigType: &accesslog.AccessLog_TypedConfig{
			TypedConfig: accessLogTypedConf,
		},
	}}
}

func buildGRPCAccessLog(clusterName, LogName string, timeout time.Duration, bufferSize uint32) []*accesslog.AccessLog {

	accessLogConf := &grpcaccesslog.HttpGrpcAccessLogConfig{
		CommonConfig: &grpcaccesslog.CommonGrpcAccessLogConfig{
			LogName:             LogName,
			GrpcService:         buildGRPCService(clusterName, timeout),
			TransportApiVersion: core.ApiVersion_V3,
			BufferSizeBytes:     protoUint32orNil(bufferSize),
		},
	}

	accessLogTypedConf, err := ptypes.MarshalAny(accessLogConf)
	if err != nil {
		log.Panic(err)
	}
	return []*accesslog.AccessLog{{
		Name: wellknown.HTTPGRPCAccessLog,
		ConfigType: &accesslog.AccessLog_TypedConfig{
			TypedConfig: accessLogTypedConf,
		},
	}}
}
