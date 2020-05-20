package main

import (
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/erikbos/gatekeeper/pkg/shared"
)

const (
	virtualHostDataRefreshInterval = 2 * time.Second
	routeDataRefreshInterval       = 2 * time.Second
)

// var virtualhostsMap map[string]string

// GetVirtualHostConfigFromDatabase continously gets the current configuration
func (a *authorizationServer) GetVirtualHostConfigFromDatabase() {
	var virtualHostsLastUpdate int64
	var virtualHostsMutex sync.Mutex

	for {
		var xdsPushNeeded bool

		newVirtualHosts, err := a.db.GetVirtualHosts()
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
					a.virtualhosts = newVirtualHosts
					// virtualhostsMap = a.buildVhostMap()
					virtualHostsMutex.Unlock()

					virtualHostsLastUpdate = shared.GetCurrentTimeMilliseconds()
					xdsPushNeeded = true
				}
			}
		}
		if xdsPushNeeded {
			// Increase xds deployment metric
			a.metrics.configLoads.WithLabelValues("virtualhosts").Inc()
		}
		time.Sleep(virtualHostDataRefreshInterval)
	}
}

func (a *authorizationServer) buildVhostMap() map[string]string {
	m := make(map[string]string)

	for _, virtualhost := range a.virtualhosts {
		for _, host := range virtualhost.VirtualHosts {
			m[host] = virtualhost.RouteSet
		}
	}

	return m
}

// GetRouteConfigFromDatabase continously gets the current configuration
func (a *authorizationServer) GetRouteConfigFromDatabase() {
	var routesLastUpdate int64
	var routeMutex sync.Mutex

	for {
		var xdsPushNeeded bool

		newRouteList, err := a.db.GetRoutes()
		if err != nil {
			log.Errorf("Could not retrieve routes from database (%s)", err)
		} else {
			if routesLastUpdate == 0 {
				log.Info("Initial load of routes done")
			}
			for _, route := range newRouteList {
				// Is a cluster updated since last time we stored it?
				if route.LastmodifiedAt > routesLastUpdate {
					routeMutex.Lock()
					a.routes = newRouteList
					routeMutex.Unlock()

					routesLastUpdate = shared.GetCurrentTimeMilliseconds()
					xdsPushNeeded = true
				}
			}
		}
		if xdsPushNeeded {
			// Increase xds deployment metric
			a.metrics.configLoads.WithLabelValues("routes").Inc()
		}
		time.Sleep(routeDataRefreshInterval)
	}
}
