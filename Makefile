VERSION := $(shell git describe --tags --always --dirty)
BUILDTAG := $(shell git rev-parse HEAD)
BUILDTIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
BUILDER := $(shell echo "`git config user.name` <`git config user.email`>")
LDFLAGS := -ldflags "-X main.version=$(VERSION) -X main.buildTime=$(BUILDTIME)"
BIN = bin

all: dbadmin envoyauth envoycp testbackend


dbadmin:
	mkdir -p $(BIN)
	go build -o $(BIN)/dbadmin $(LDFLAGS) cmd/dbadmin/*.go

envoyauth:
	mkdir -p $(BIN)
	go build -o $(BIN)/envoyauth $(LDFLAGS) cmd/envoyauth/*.go

envoycp:
	mkdir -p $(BIN)
	go build -o $(BIN)/envoycp $(LDFLAGS) cmd/envoycp/*.go

testbackend:
	mkdir -p $(BIN)
	go build -o $(BIN)/testbackend $(LDFLAGS) cmd/testbackend/*.go


docker-images: docker-baseimage docker-dbadmin docker-envoyauth docker-envoycp docker-testbackend

docker-baseimage:
	 docker build -f build/Dockerfile.baseimage . -t gatekeeper/baseimage

docker-dbadmin:
	 docker build -f build/Dockerfile.dbadmin . -t gatekeeper/dbadmin:$(VERSION)

docker-envoyauth:
	 docker build -f build/Dockerfile.envoyauth . -t gatekeeper/envoyauth:$(VERSION)

docker-envoycp:
	 docker build -f build/Dockerfile.envoycp . -t gatekeeper/envoycp:$(VERSION)

docker-testbackend:
	 docker build -f  build/Dockerfile.testbackend . -t gatekeeper/testbackend:$(VERSION)

clean:
	rm -f $(BIN)/dbadmin $(BIN)/envoyauth $(BIN)/envoycp $(BIN)/testbackend
