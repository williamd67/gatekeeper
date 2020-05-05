package main

import (
	"sync"
	"time"

	"github.com/erikbos/apiauth/pkg/shared"

	api "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	auth "github.com/envoyproxy/go-control-plane/envoy/api/v2/auth"
	core "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	endpoint "github.com/envoyproxy/go-control-plane/envoy/api/v2/endpoint"
	cache "github.com/envoyproxy/go-control-plane/pkg/cache"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/duration"
	"github.com/golang/protobuf/ptypes/wrappers"
	log "github.com/sirupsen/logrus"
)

const (
	clusterRefreshInterval = 2

	attributeConnectTimeout      = "ConnectTimeout"
	attributeIdleTimeout         = "IdleTimeout"
	attributeMinimumTLSVersion   = "MinimumTLSVersion"
	attributeMaximumTLSVersion   = "MaximumTLSVersion"
	attributeHTTP2Enabled        = "HTTP2Enabled"
	attributeSNIHostName         = "SNIHostName"
	attributeHealthCheck         = "HealthCheck"
	attributeHealthCheckPath     = "HealthCheckPath"
	attributeHealthCheckInterval = "HealthCheckInterval"
	attributeHealthCheckTimeout  = "HealthCheckTimeout"

	attributeValueTrue  = "true"
	attributeValueHTTP  = "HTTP"
	attributeValueTLS10 = "TLSv10"
	attributeValueTLS11 = "TLSv11"
	attributeValueTLS12 = "TLSv12"
	attributeValueTLS13 = "TLSv13"
)

var clusters []shared.Cluster

// FIXME this does not detect removed records

// getClusterConfigFromDatabase continously gets the current configuration
func (s *server) GetClusterConfigFromDatabase() {
	var clustersLastUpdate int64
	var clusterMutex sync.Mutex

	for {
		newClusterList, err := s.db.GetClusters()
		if err != nil {
			log.Errorf("Could not retrieve clusters from database (%s)", err)
		} else {
			// Is one of the cluster updated since last time pushed config to Envoy?
			for _, s := range newClusterList {
				if s.LastmodifiedAt > clustersLastUpdate {

					clusterMutex.Lock()
					clusters = newClusterList
					clusterMutex.Unlock()

					clustersLastUpdate = shared.GetCurrentTimeMilliseconds()

					// FIXME this should be notification via channel
					xdsLastUpdate = shared.GetCurrentTimeMilliseconds()
				}
			}
		}
		time.Sleep(clusterRefreshInterval * time.Second)
	}
}

// getClusterConfig returns array of all envoy clusters
func getEnvoyClusterConfig() ([]cache.Resource, error) {
	envoyClusters := []cache.Resource{}
	for _, s := range clusters {
		envoyClusters = append(envoyClusters, buildEnvoyClusterConfig(s))
	}
	return envoyClusters, nil
}

// buildEnvoyClusterConfig builds one envoy cluster configuration
func buildEnvoyClusterConfig(cluster shared.Cluster) *api.Cluster {

	return &api.Cluster{
		Name:                      cluster.Name,
		ConnectTimeout:            clusterConnectTimeout(cluster),
		ClusterDiscoveryType:      &api.Cluster_Type{Type: api.Cluster_LOGICAL_DNS},
		DnsLookupFamily:           api.Cluster_V4_ONLY,
		LbPolicy:                  api.Cluster_ROUND_ROBIN,
		LoadAssignment:            clusterLoadAssignment(cluster),
		HealthChecks:              clusterHealthCheckConfig(cluster),
		CommonHttpProtocolOptions: clusterCommonHTTPProtocolOptions(cluster),
		HttpProtocolOptions:       clusterHTTP1ProtocolOptions(cluster),
		Http2ProtocolOptions:      clusterHTTP2ProtocolOptions(cluster),
		TransportSocket:           clusterTransportSocket(cluster),
		// Http1ProtocolOptions:  clusterHTTP1ProtocolOptions(cluster),
		// CircuitBreakers:      clusterCircuitBreaker(cluster),
	}
}

func clusterConnectTimeout(cluster shared.Cluster) *duration.Duration {
	connectTimeout, _ := shared.GetAttribute(cluster.Attributes, attributeConnectTimeout)
	connectTimeoutAsDuration, err := time.ParseDuration(connectTimeout)
	if err != nil {
		connectTimeoutAsDuration = 2 * time.Second
	}
	return ptypes.DurationProto(connectTimeoutAsDuration)
}

func clusterLoadAssignment(cluster shared.Cluster) *api.ClusterLoadAssignment {
	address := coreAddress(cluster.HostName, cluster.Port)

	return &api.ClusterLoadAssignment{
		ClusterName: cluster.Name,
		Endpoints: []*endpoint.LocalityLbEndpoints{
			{
				LbEndpoints: []*endpoint.LbEndpoint{
					{
						HostIdentifier: &endpoint.LbEndpoint_Endpoint{
							Endpoint: &endpoint.Endpoint{
								Address: address,
							},
						},
					},
				},
			},
		},
	}
}

// coreAddress builds an Envoy address to connect to
func coreAddress(hostname string, port int) *core.Address {
	return &core.Address{Address: &core.Address_SocketAddress{
		SocketAddress: &core.SocketAddress{
			Address:  hostname,
			Protocol: core.SocketAddress_TCP,
			PortSpecifier: &core.SocketAddress_PortValue{
				PortValue: uint32(port),
			},
		},
	}}
}

// func clusterCircuitBreaker(cluster shared.Cluster) *envoy_cluster.CircuitBreakers {
// 	return &envoy_cluster.CircuitBreakers{
// 		Thresholds: []*envoy_cluster.CircuitBreakers_Thresholds{{
// 			MaxConnections:     u32nil(service.MaxConnections),
// 			MaxPendingRequests: u32nil(service.MaxPendingRequests),
// 			MaxRequests:        u32nil(service.MaxRequests),
// 			MaxRetries:         u32nil(service.MaxRetries),
// 		}},
// 	}
// }

// clusterHealthCheckConfig builds health configuration for a cluster
func clusterHealthCheckConfig(cluster shared.Cluster) []*core.HealthCheck {

	value, err := shared.GetAttribute(cluster.Attributes, attributeHealthCheck)
	if err == nil && value == attributeValueHTTP {
		healthCheckPath, _ := shared.GetAttribute(cluster.Attributes, attributeHealthCheckPath)

		healthCheckInterval, _ := shared.GetAttribute(cluster.Attributes, attributeHealthCheckInterval)
		healthcheckIntervalAsDuration, err := time.ParseDuration(healthCheckInterval)
		if err != nil {
			healthcheckIntervalAsDuration = 10 * time.Second
		}

		healthCheckTimeout, _ := shared.GetAttribute(cluster.Attributes, attributeHealthCheckTimeout)
		healthcheckTimeoutAsDuration, err := time.ParseDuration(healthCheckTimeout)
		if err != nil {
			healthcheckTimeoutAsDuration = 10 * time.Second
		}

		healthCheck := &core.HealthCheck{
			HealthChecker: &core.HealthCheck_HttpHealthCheck_{
				HttpHealthCheck: &core.HealthCheck_HttpHealthCheck{
					Path: healthCheckPath,
				},
			},
			Interval:           ptypes.DurationProto(healthcheckIntervalAsDuration),
			Timeout:            ptypes.DurationProto(healthcheckTimeoutAsDuration),
			UnhealthyThreshold: &wrappers.UInt32Value{Value: 2},
			HealthyThreshold:   &wrappers.UInt32Value{Value: 1},
			EventLogPath:       "/tmp/healthcheck",
		}
		return append([]*core.HealthCheck{}, healthCheck)
	}
	return nil
}

// clusterCommonHTTPProtocolOptions sets HTTP options applicable to both HTTP/1 and /2
func clusterCommonHTTPProtocolOptions(cluster shared.Cluster) *core.HttpProtocolOptions {
	idleTimeout, _ := shared.GetAttribute(cluster.Attributes, attributeIdleTimeout)
	idleTimeoutAsDuration, err := time.ParseDuration(idleTimeout)
	if err != nil {
		return nil
	}
	return &core.HttpProtocolOptions{
		IdleTimeout: ptypes.DurationProto(idleTimeoutAsDuration),
	}
}

func clusterHTTP1ProtocolOptions(cluster shared.Cluster) *core.Http1ProtocolOptions {
	return nil
	// return &core.Http1ProtocolOptions{}
}

// clusterHTTP2ProtocolOptions returns HTTP/2 parameters
// according to spec we need to return at least empty struct to enable HTTP/2
func clusterHTTP2ProtocolOptions(cluster shared.Cluster) *core.Http2ProtocolOptions {
	value, err := shared.GetAttribute(cluster.Attributes, attributeHTTP2Enabled)
	if err == nil && value == attributeValueTrue {
		return &core.Http2ProtocolOptions{}
	}
	return nil
}

// clusterTransportSocket configures TLS settings
func clusterTransportSocket(cluster shared.Cluster) *core.TransportSocket {
	TLSContext := &auth.UpstreamTlsContext{
		Sni: clusterSNIHostname(cluster),
		CommonTlsContext: &auth.CommonTlsContext{
			AlpnProtocols: clusterALPNOptions(cluster),
			TlsParams:     clusterTLSOptions(cluster),
		},
	}
	tlsContextProtoBuf, err := ptypes.MarshalAny(TLSContext)
	if err != nil {
		return nil
	}
	return &core.TransportSocket{
		Name: "tls",
		ConfigType: &core.TransportSocket_TypedConfig{
			TypedConfig: tlsContextProtoBuf,
		},
	}
}

// clusterSNIHostname sets SNI hostname used by TLS
func clusterSNIHostname(cluster shared.Cluster) string {
	value, err := shared.GetAttribute(cluster.Attributes, attributeSNIHostName)
	if err == nil && value != "" {
		return value
	}
	return cluster.HostName
}

// clusterALPNOptions sets TLS's ALPN supported protocols
func clusterALPNOptions(cluster shared.Cluster) []string {
	value, err := shared.GetAttribute(cluster.Attributes, attributeHTTP2Enabled)
	if err == nil && value == attributeValueTrue {
		return []string{"h2", "http/1.1"}
	}
	return []string{"http/1.1"}
}

// clusterALPNOptions sets TLS minimum and max cipher options
func clusterTLSOptions(cluster shared.Cluster) *auth.TlsParameters {
	tlsParameters := &auth.TlsParameters{}
	if minVersion, err := shared.GetAttribute(cluster.Attributes, attributeMinimumTLSVersion); err == nil {
		tlsParameters.TlsMinimumProtocolVersion = tlsVersion(minVersion)
	}
	if maxVersion, err := shared.GetAttribute(cluster.Attributes, attributeMaximumTLSVersion); err == nil {
		tlsParameters.TlsMaximumProtocolVersion = tlsVersion(maxVersion)
	}
	return tlsParameters
}

func tlsVersion(version string) auth.TlsParameters_TlsProtocol {
	switch version {
	case attributeValueTLS10:
		return auth.TlsParameters_TLSv1_0
	case attributeValueTLS11:
		return auth.TlsParameters_TLSv1_1
	case attributeValueTLS12:
		return auth.TlsParameters_TLSv1_2
	case attributeValueTLS13:
		return auth.TlsParameters_TLSv1_3
	}
	return auth.TlsParameters_TLS_AUTO
}

// func u32nil(val uint32) *wrappers.UInt32Value {
// 	switch val {
// 	case 0:
// 		return nil
// 	default:
// 		return &wrappers.UInt32Value{Value: val}
// 	}
// }
