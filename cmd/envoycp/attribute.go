package main

import (
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/erikbos/gatekeeper/pkg/shared"
)

const (
	// Virtual host attributes
	attributeAccessLogFileName    = "AccessLogFileName"
	attributeAccessLogClusterName = "AccessLogClusterName"

	// Route attributes
	attributeDisableAuthentication    = "DisableAuthentication"
	attributeDirectResponseStatusCode = "DirectResponseStatusCode"
	attributeDirectResponseBody       = "DirectResponseBody"
	attributeRedirectStatusCode       = "RedirectStatusCode"
	attributeRedirectScheme           = "RedirectScheme"
	attributeRedirectHostName         = "RedirectHostName"
	attributeRedirectPort             = "RedirectPort"
	attributeRedirectPath             = "RedirectPath"
	attributeRedirectStripQuery       = "RedirectStripQuery"
	attributePrefixRewrite            = "PrefixRewrite"
	attributeCORSAllowCredentials     = "CORSAllowCredentials"
	attributeCORSAllowMethods         = "CORSAllowMethods"
	attributeCORSAllowHeaders         = "CORSAllowHeaders"
	attributeCORSExposeHeaders        = "CORSExposeHeaders"
	attributeCORSMaxAge               = "CORSMaxAge"
	attributeHostHeader               = "HostHeader"
	attributeBasicAuth                = "BasicAuth"
	attributeRetryOn                  = "RetryOn"
	attributePerTryTimeout            = "PerTryTimeout"
	attributeNumRetries               = "NumRetries"
	attributeRetryOnStatusCodes       = "RetryOnStatusCodes"

	// Default route configuration values
	perRetryTimeout = 500 * time.Millisecond

	// Cluster attributes
	attributeConnectTimeout                = "ConnectTimeout"
	attributeIdleTimeout                   = "IdleTimeout"
	attributeTLSEnabled                    = "TLSEnabled"
	attributeSNIHostName                   = "SNIHostName"
	attributeHealthCheckProtocol           = "HealthCheckProtocol"
	attributeHealthCheckPath               = "HealthCheckPath"
	attributeHealthCheckInterval           = "HealthCheckInterval"
	attributeHealthCheckTimeout            = "HealthCheckTimeout"
	attributeHealthCheckUnhealthyThreshold = "HealthCheckUnhealthyThreshold"
	attributeHealthCheckHealthyThreshold   = "HealthCheckHealthyThreshold"
	attributeHealthCheckLogFile            = "HealthCheckLogFile"
	attributeMaxConnections                = "MaxConnections"
	attributeMaxPendingRequests            = "MaxPendingRequests"
	attributeMaxRequests                   = "MaxRequests"
	attributeMaxRetries                    = "MaxRetries"
	attributeDNSLookupFamiliy              = "DNSLookupFamily"
	attributeDNSRefreshRate                = "DNSRefreshRate"
	attributeDNSResolvers                  = "DNSResolvers"

	// Default cluster configuration values
	defaultClusterConnectTimeout         = 5 * time.Second
	defaultClusterIdleTimeout            = 15 * time.Minute
	defaultHealthCheckInterval           = 5 * time.Second
	defaultHealthCheckTimeout            = 10 * time.Second
	defaultHealthCheckUnhealthyThreshold = 2
	defaultHealthCheckHealthyThreshold   = 2
	defaultDNSRefreshRate                = 5 * time.Second

	// Attributes shared amongst virtualhost & cluster
	attributeTLSCertificate    = "TLSCertificate"
	attributeTLSCertificateKey = "TLSCertificateKey"
	attributeTLSMinimumVersion = "TLSMinimumVersion"
	attributeTLSMaximumVersion = "TLSMaximumVersion"
	attributeTLSCipherSuites   = "TLSCipherSuites"
	attributeHTTPProtocol      = "HTTPProtocol"

	// Attribute values
	attributeValueTrue                    = "true"
	attributeValueTLSVersion10            = "TLSv10"
	attributeValueTLSVersion11            = "TLSv11"
	attributeValueTLSVersion12            = "TLSv12"
	attributeValueTLSVersion13            = "TLSv13"
	attributeValueHTTPProtocol11          = "HTTP/1.1"
	attributeValueHTTPProtocol2           = "HTTP/2"
	attributeValueHTTPProtocol3           = "HTTP/3"
	attributeValueHealthCheckProtocolHTTP = "HTTP"
)

func warnForUnknownVirtualHostAttributes(virtualhost shared.VirtualHost) {

	var validVirtualHostAttributes = map[string]bool{
		attributeAccessLogFileName:    true,
		attributeAccessLogClusterName: true,
		attributeHTTPProtocol:         true,
		attributeTLSEnabled:           true,
		attributeTLSMinimumVersion:    true,
		attributeTLSMaximumVersion:    true,
		attributeTLSCertificate:       true,
		attributeTLSCertificateKey:    true,
		attributeTLSCipherSuites:      true,
	}

	warnForUnknownAttribute("Virtualhost "+virtualhost.Name,
		virtualhost.Attributes, validVirtualHostAttributes)
}

func warnForUnknownRouteAttributes(route shared.Route) {

	var validRouteAttributes = map[string]bool{
		attributeDisableAuthentication:    true,
		attributeDirectResponseStatusCode: true,
		attributeDirectResponseBody:       true,
		attributePrefixRewrite:            true,
		attributeCORSAllowCredentials:     true,
		attributeCORSAllowMethods:         true,
		attributeCORSAllowHeaders:         true,
		attributeCORSExposeHeaders:        true,
		attributeCORSMaxAge:               true,
		attributeHostHeader:               true,
		attributeBasicAuth:                true,
		attributeRetryOn:                  true,
		attributePerTryTimeout:            true,
		attributeNumRetries:               true,
		attributeRetryOnStatusCodes:       true,
	}

	warnForUnknownAttribute("Route "+route.Name,
		route.Attributes, validRouteAttributes)
}

func warnForUnknownClusterAttributes(cluster shared.Cluster) {

	var validClusterAttributes = map[string]bool{
		attributeConnectTimeout:                true,
		attributeIdleTimeout:                   true,
		attributeTLSEnabled:                    true,
		attributeTLSMinimumVersion:             true,
		attributeTLSMaximumVersion:             true,
		attributeTLSCipherSuites:               true,
		attributeHTTPProtocol:                  true,
		attributeSNIHostName:                   true,
		attributeHealthCheckProtocol:           true,
		attributeHealthCheckPath:               true,
		attributeHealthCheckInterval:           true,
		attributeHealthCheckTimeout:            true,
		attributeHealthCheckUnhealthyThreshold: true,
		attributeHealthCheckHealthyThreshold:   true,
		attributeHealthCheckLogFile:            true,
		attributeMaxConnections:                true,
		attributeMaxPendingRequests:            true,
		attributeMaxRequests:                   true,
		attributeMaxRetries:                    true,
		attributeDNSLookupFamiliy:              true,
		attributeDNSRefreshRate:                true,
		attributeDNSResolvers:                  true,
	}

	warnForUnknownAttribute("Cluster "+cluster.Name,
		cluster.Attributes, validClusterAttributes)
}

func warnForUnknownAttribute(resource string, attributes []shared.AttributeKeyValues, validAttributes map[string]bool) {

	for _, attribute := range attributes {
		if !validAttributes[attribute.Name] {
			log.Warningf("'%s' has unknown attribute '%s' value '%s'",
				resource, attribute.Name, attribute.Value)
		}
	}
}
