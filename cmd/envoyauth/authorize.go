package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"

	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	authservice "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"
	envoytype "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"github.com/gogo/googleapis/google/rpc"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	"google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/erikbos/gatekeeper/pkg/types"
)

type envoyAuthConfig struct {
	Listen string `yaml:"listen"` // GRPC Address and port to listen for control plane
}

// requestInfo holds all information of a request
type requestInfo struct {
	IP              net.IP
	httpRequest     *authservice.AttributeContext_HttpRequest
	URL             *url.URL
	queryParameters url.Values
	apikey          *string
	oauth2token     *string
	vhost           *types.Listener
	developer       *types.Developer
	developerApp    *types.DeveloperApp
	appCredential   *types.DeveloperAppKey
	APIProduct      *types.APIProduct
}

// startGRPCAuthorizationServer starts extauthz grpc listener
func (a *authorizationServer) StartAuthorizationServer() {

	lis, err := net.Listen("tcp", a.config.EnvoyAuth.Listen)
	if err != nil {
		a.logger.Fatal("failed to listen", zap.Error(err))
	}
	a.logger.Info("GRPC listening on " + a.config.EnvoyAuth.Listen)

	grpcServer := grpc.NewServer()
	authservice.RegisterAuthorizationServer(grpcServer, a)

	if err := grpcServer.Serve(lis); err != nil {
		a.logger.Fatal("Failed to start server", zap.Error(err))
	}
}

// Check (called by Envoy) to authenticate & authorize a HTTP request
func (a *authorizationServer) Check(ctx context.Context,
	authRequest *authservice.CheckRequest) (*authservice.CheckResponse, error) {

	timer := prometheus.NewTimer(a.metrics.authLatencyHistogram)
	defer timer.ObserveDuration()

	request, err := getRequestInfo(authRequest)
	if err != nil {
		a.metrics.connectInfoFailures.Inc()
		return a.rejectRequest(http.StatusBadRequest, nil, nil, fmt.Sprintf("%s", err))
	}
	a.logRequestDebug(request)

	// FIXME not sure if x-forwarded-proto the way to determine original tcp port used
	request.vhost, err = a.vhosts.Lookup(request.httpRequest.Host, request.httpRequest.Headers["x-forwarded-proto"])
	if err != nil {
		a.metrics.increaseCounterRequestRejected(request)
		return a.rejectRequest(http.StatusNotFound, nil, nil, "unknown vhost")
	}

	vhostPolicyOutcome := &PolicyChainResponse{}
	if request.vhost != nil && request.vhost.Policies != "" {
		vhostPolicyOutcome = (&PolicyChain{
			authServer: a,
			request:    request,
			scope:      policyScopeVhost,
		}).Evaluate()
	}

	APIProductPolicyOutcome := &PolicyChainResponse{}
	if request.APIProduct != nil && request.APIProduct.Policies != "" {
		APIProductPolicyOutcome = (&PolicyChain{
			authServer: a,
			request:    request,
			scope:      policyScopeAPIProduct,
		}).Evaluate()
	}

	a.logger.Debug("vhostPolicyOutcome", zap.Reflect("debug", vhostPolicyOutcome))
	a.logger.Debug("APIProductPolicyOutcome", zap.Reflect("debug", APIProductPolicyOutcome))

	// We reject call in case both vhost & apiproduct policy did not authenticate call
	if (vhostPolicyOutcome != nil && !vhostPolicyOutcome.authenticated) &&
		(APIProductPolicyOutcome != nil && !APIProductPolicyOutcome.authenticated) {

		a.metrics.increaseCounterRequestRejected(request)

		return a.rejectRequest(vhostPolicyOutcome.deniedStatusCode,
			mergeMapsStringString(vhostPolicyOutcome.upstreamHeaders,
				APIProductPolicyOutcome.upstreamHeaders),
			mergeMapsStringString(vhostPolicyOutcome.upstreamDynamicMetadata,
				APIProductPolicyOutcome.upstreamDynamicMetadata),
			vhostPolicyOutcome.deniedMessage)
	}

	a.metrics.IncreaseCounterRequestAccept(request)

	return a.allowRequest(
		mergeMapsStringString(vhostPolicyOutcome.upstreamHeaders,
			APIProductPolicyOutcome.upstreamHeaders),
		mergeMapsStringString(vhostPolicyOutcome.upstreamDynamicMetadata,
			APIProductPolicyOutcome.upstreamDynamicMetadata))
}

// mergeMapsStringString returns merged map[string]string
// it overwriting duplicate keys, you should handle that if there is a need
func mergeMapsStringString(maps ...map[string]string) map[string]string {
	result := make(map[string]string)
	for _, m := range maps {
		for k, v := range m {
			result[k] = v
		}
	}
	return result
}

// allowRequest answers Envoyproxy to authorizates request to go upstream
func (a *authorizationServer) allowRequest(headers, metadata map[string]string) (*authservice.CheckResponse, error) {

	dynamicMetadata := buildDynamicMetadataList(metadata)

	response := &authservice.CheckResponse{
		Status: &status.Status{
			Code: int32(rpc.OK),
		},
		HttpResponse: &authservice.CheckResponse_OkResponse{
			OkResponse: &authservice.OkHttpResponse{
				Headers: buildHeadersList(headers),
				// Required for < Envoy 0.17
				DynamicMetadata: dynamicMetadata,
			},
		},
		// Required for > Envoy 0.16
		// DynamicMetadata: dynamicMetadata,
	}
	a.logger.Debug("allowRequest", zap.Reflect("response", response))

	return response, nil
}

// rejectRequest answers Envoyproxy to reject HTTP request
func (a *authorizationServer) rejectRequest(statusCode int, headers, metadata map[string]string,
	message string) (*authservice.CheckResponse, error) {

	var envoyStatusCode envoytype.StatusCode

	switch statusCode {
	case http.StatusUnauthorized:
		envoyStatusCode = envoytype.StatusCode_Unauthorized
	case http.StatusForbidden:
		envoyStatusCode = envoytype.StatusCode_Forbidden
	case http.StatusServiceUnavailable:
		envoyStatusCode = envoytype.StatusCode_ServiceUnavailable
	default:
		envoyStatusCode = envoytype.StatusCode_Forbidden
	}

	response := &authservice.CheckResponse{
		Status: &status.Status{
			Code: int32(rpc.UNAUTHENTICATED),
		},
		HttpResponse: &authservice.CheckResponse_DeniedResponse{
			DeniedResponse: &authservice.DeniedHttpResponse{
				Status: &envoytype.HttpStatus{
					Code: envoyStatusCode,
				},
				Headers: buildHeadersList(headers),
				Body:    buildJSONErrorMessage(&message),
			},
		},
		DynamicMetadata: buildDynamicMetadataList(metadata),
	}
	a.logger.Debug("rejectRequest", zap.Reflect("response", response))
	return response, nil
}

// buildHeadersList creates map to hold additional upstream headers
func buildHeadersList(headers map[string]string) []*core.HeaderValueOption {

	if len(headers) == 0 {
		return nil
	}

	headerList := make([]*core.HeaderValueOption, 0, len(headers))
	for key, value := range headers {
		headerList = append(headerList, &core.HeaderValueOption{
			Header: &core.HeaderValue{
				Key:   key,
				Value: value,
			},
		})
	}
	return headerList
}

// buildDynamicMetadataList creates struct for all additional metadata to be returned when accepting a request.
//
// Potential use cases:
// 1) insert metadata into upstream headers using %DYNAMIC_METADATA%
// 2) have accesslog log metadata which are not susposed to go upstream as HTTP headers
// 3) to provide configuration to ratelimiter filter
func buildDynamicMetadataList(metadata map[string]string) *structpb.Struct {

	if len(metadata) == 0 {
		return nil
	}
	metadataStruct := structpb.Struct{
		Fields: map[string]*structpb.Value{},
	}
	for key, value := range metadata {
		metadataStruct.Fields[key] = &structpb.Value{
			Kind: &structpb.Value_StringValue{StringValue: value},
		}
	}
	// Convert rate limiter values into ratelimiter configuration
	// a route's ratelimiteraction will check for this metadata key
	if rateLimitOverride := buildRateLimiterOveride(metadata); rateLimitOverride != nil {
		metadataStruct.Fields["rl.override"] = rateLimitOverride
	}
	return &metadataStruct
}

// buildRateLimiterOveride builds route RateLimiterOverride configuration based upon
// metadata keys "rl.requests_per_unit" & "rl.unit"
func buildRateLimiterOveride(metadata map[string]string) *structpb.Value {

	var requestsPerUnit float64
	if value, found := metadata["rl.requests_per_unit"]; found {
		var err error
		if requestsPerUnit, err = strconv.ParseFloat(value, 64); err != nil {
			return nil
		}
	}
	var unit string
	unit, found := metadata["rl.unit"]
	if !found {
		return nil
	}
	return &structpb.Value{
		Kind: &structpb.Value_StructValue{
			StructValue: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"requests_per_unit": {
						Kind: &structpb.Value_NumberValue{
							NumberValue: requestsPerUnit,
						},
					},
					"unit": {
						Kind: &structpb.Value_StringValue{
							StringValue: unit,
						},
					},
				},
			},
		},
	}
}

// getRequestInfo returns HTTP data of a request
func getRequestInfo(req *authservice.CheckRequest) (*requestInfo, error) {

	newConnection := requestInfo{
		httpRequest: req.Attributes.Request.Http,
	}
	if ipaddress, ok := newConnection.httpRequest.Headers["x-forwarded-for"]; ok {
		newConnection.IP = net.ParseIP(ipaddress)
	}

	var err error
	if newConnection.URL, err = url.ParseRequestURI(newConnection.httpRequest.Path); err != nil {
		return nil, errors.New("cannot parse url")
	}

	if newConnection.queryParameters, err = url.ParseQuery(newConnection.URL.RawQuery); err != nil {
		return nil, errors.New("cannot parse query parameters")
	}

	return &newConnection, nil
}

func (a *authorizationServer) logRequestDebug(request *requestInfo) {
	a.logger.Debug("Check() rx path", zap.String("path", request.httpRequest.Path))

	for key, value := range request.httpRequest.Headers {
		a.logger.Debug("Check() rx header", zap.String("key", key), zap.String("value", value))
	}
}

// JSONErrorMessage is the format for our error messages
const JSONErrorMessage = `{
 "message": "%s"
}
`

// returns a well structured JSON-formatted message
func buildJSONErrorMessage(message *string) string {

	return fmt.Sprintf(JSONErrorMessage, *message)
}
