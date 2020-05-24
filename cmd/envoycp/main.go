package main

import (
	"github.com/envoyproxy/go-control-plane/pkg/cache"
	xds "github.com/envoyproxy/go-control-plane/pkg/server"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"

	"github.com/erikbos/gatekeeper/pkg/db"
	"github.com/erikbos/gatekeeper/pkg/shared"
)

var (
	version   string
	buildTime string
)

const (
	myName = "envoycp"
)

type server struct {
	config       *EnvoyCPConfig
	ginEngine    *gin.Engine
	db           *db.Database
	readiness    shared.Readiness
	virtualhosts []shared.VirtualHost
	routes       []shared.Route
	clusters     []shared.Cluster
	xds          xds.Server
	xdsCache     cache.SnapshotCache
	metrics      struct {
		xdsDeployments *prometheus.CounterVec
	}
}

type xdsNotifyMesssage struct {
	resource string
}

func main() {
	shared.StartLogging(myName, version, buildTime)

	s := server{
		config: loadConfiguration(),
	}

	shared.SetLoggingConfiguration(s.config.LogLevel)
	s.readiness.RegisterMetrics(myName)

	var err error
	s.db, err = db.Connect(s.config.Database, &s.readiness, myName)
	if err != nil {
		log.Fatalf("Database connect failed: %v", err)
	}

	s.registerMetrics()
	go s.StartWebAdminServer()

	xdsNotify := make(chan xdsNotifyMesssage)
	go s.GetVirtualHostConfigFromDatabase(xdsNotify)
	go s.GetRouteConfigFromDatabase(xdsNotify)
	go s.GetClusterConfigFromDatabase(xdsNotify)
	s.StartXDS(xdsNotify)
}
