package main

import (
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"

	"github.com/erikbos/gatekeeper/pkg/db"
	"github.com/erikbos/gatekeeper/pkg/shared"
)

var (
	version   string
	buildTime string
)

const (
	myName = "dbadmin"
)

type server struct {
	config    *DBAdminConfig
	ginEngine *gin.Engine
	db        *db.Database
	readiness shared.Readiness
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

	StartWebAdminServer(&s)

}
