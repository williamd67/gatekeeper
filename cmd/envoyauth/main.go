package main

import (
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"

	"github.com/erikbos/gatekeeper/pkg/db"
	"github.com/erikbos/gatekeeper/pkg/db/cassandra"
	"github.com/erikbos/gatekeeper/pkg/shared"
)

var (
	version   string
	buildTime string
)

const (
	applicatioName = "envoyauth"
)

type authorizationServer struct {
	config       *APIAuthConfig
	ginEngine    *gin.Engine
	readiness    shared.Readiness
	virtualhosts []shared.VirtualHost
	routes       []shared.Route
	db           *db.Database
	cache        *Cache
	oauth        *oauthServer
	g            *shared.Geoip
	metrics      metricsCollection
}

func main() {
	shared.StartLogging(applicatioName, version, buildTime)

	a := authorizationServer{}
	a.config = loadConfiguration()
	// FIXME we should check if we have all required parameters (use viper package?)

	shared.SetLoggingConfiguration(a.config.LogLevel)
	a.readiness.RegisterMetrics(applicatioName)

	var err error
	a.db, err = cassandra.New(a.config.Database, applicatioName)
	if err != nil {
		log.Fatalf("Database connect failed: %v", err)
	}

	a.cache = newCache(&a.config.Cache)

	if a.config.Geoip.Filename != "" {
		a.g, err = shared.OpenGeoipDatabase(a.config.Geoip.Filename)
		if err != nil {
			log.Fatalf("Geoip db load failed: %v", err)
		}
	}

	a.registerMetrics()
	go StartWebAdminServer(&a)
	go a.GetVirtualHostConfigFromDatabase()
	go a.GetRouteConfigFromDatabase()

	go StartOAuthServer(&a)

	a.startGRPCAuthorizationServer()
}
