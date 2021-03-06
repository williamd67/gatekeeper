package main

import (
	"net/http"
	"strings"

	"go.uber.org/zap"
)

// PolicyChain holds the input to evaluating a series of policies
type PolicyChain struct {

	// metrics collection for policy counters
	authServer *authorizationServer

	// Request information
	request *requestInfo

	// "vhost" or "apiproduct"
	scope string
}

const (
	policyScopeVhost      = "listener"
	policyScopeAPIProduct = "apiproduct"
)

// PolicyChainResponse holds the output of a policy chain evaluation
type PolicyChainResponse struct {
	// If true the request was authenticated, subsequent policies should be evaluated
	authenticated bool
	// If true the request should be denied, no further policy evaluations required
	denied bool
	// Statuscode to use when denying a request
	deniedStatusCode int
	// Message to return when denying a request
	deniedMessage string
	// Additional HTTP headers to set when forwarding to upstream
	upstreamHeaders map[string]string
	// Dynamic metadata to set when forwarding to subsequent envoyproxy filter
	upstreamDynamicMetadata map[string]string
}

// Evaluate invokes all policy functions one by one, to:
// - check whether call should be allowed or reject
// - set HTTP response payload message
// - set additional upstream headers
func (p PolicyChain) Evaluate() *PolicyChainResponse {

	// Take policies from vhost configuration
	policies := p.request.vhost.Policies
	// Or apiproduct policies in case requested
	if p.scope == policyScopeAPIProduct {
		policies = p.request.APIProduct.Policies
	}

	policyChainResult := PolicyChainResponse{
		// By default we intend to reject request (unauthenticated)
		// This should be overwritten by one of authentication policies to:
		// 1) allow the request
		// 2) reject, with specific deny message
		authenticated:           false,
		denied:                  true,
		deniedStatusCode:        http.StatusForbidden,
		deniedMessage:           "No credentials provided",
		upstreamHeaders:         make(map[string]string, 5),
		upstreamDynamicMetadata: make(map[string]string, 15),
	}

	p.authServer.logger.Debug("Evaluating policy chain",
		zap.String("scope", p.scope),
		zap.String("policies", policies))

	for _, policyName := range strings.Split(policies, ",") {

		trimmedPolicyName := strings.TrimSpace(policyName)
		policyResult := (&Policy{
			request:             p.request,
			authServer:          p.authServer,
			PolicyChainResponse: &policyChainResult,
		}).Evaluate(trimmedPolicyName, p.request)

		p.authServer.logger.Debug("Evaluating policy",
			zap.String("scope", p.scope),
			zap.String("policy", trimmedPolicyName),
			zap.Reflect("result", policyResult))

		if policyResult != nil {
			// Register this policy evaluation successed
			p.authServer.metrics.IncreaseMetricPolicy(p.scope, trimmedPolicyName)

			// Add policy generated headers to upstream
			for key, value := range policyResult.headers {
				policyChainResult.upstreamHeaders[key] = value
			}
			// Add policy generated metadata
			for key, value := range policyResult.metadata {
				policyChainResult.upstreamDynamicMetadata[key] = value
			}
			if policyResult.authenticated {
				policyChainResult.authenticated = true
			}

			// In case policy wants to deny request we do so with provided status code
			if policyResult.denied {
				policyChainResult.denied = policyResult.denied
				policyChainResult.deniedStatusCode = policyResult.deniedStatusCode
				policyChainResult.deniedMessage = policyResult.deniedMessage

				return &policyChainResult
			}
		} else {
			// Register this policy evaluation failed
			p.authServer.metrics.IncreaseMetricPolicyUnknown(p.scope, trimmedPolicyName)
		}
	}
	return &policyChainResult
}
