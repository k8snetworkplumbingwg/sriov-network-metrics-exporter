IMAGE_REGISTRY?=ghcr.io/k8snetworkplumbingwg/
IMAGE_VERSION?=latest

IMAGE_NAME?=$(IMAGE_REGISTRY)sriov-network-metrics-exporter:$(IMAGE_VERSION)
IMAGE_BUILDER?=docker

# Package related
BINARY_NAME=sriov-exporter
BUILDDIR=$(CURDIR)/build

DOCKERARGS?=
ifdef HTTP_PROXY
	DOCKERARGS += --build-arg http_proxy=$(HTTP_PROXY)
endif
ifdef HTTPS_PROXY
	DOCKERARGS += --build-arg https_proxy=$(HTTPS_PROXY)
endif

# Go settings
GO = go
GO_BUILD_OPTS ?=CGO_ENABLED=0
GO_LDFLAGS ?= -s -w
GO_FLAGS ?= 
GO_TAGS ?=-tags no_openssl
export GOPATH?=$(shell go env GOPATH)

# debug
V ?= 0
Q = $(if $(filter 1,$V),,@)

all: build image-build test 

clean:
	rm -rf bin
	go clean -modcache -testcache
	
build:
	$Q cd $(CURDIR)/cmd && $(GO_BUILD_OPTS) go build -ldflags '$(GO_LDFLAGS)' $(GO_FLAGS) -o $(BUILDDIR)/$(BINARY_NAME) $(GO_TAGS) -v

image-build:
	@echo "Bulding container image $(IMAGE_NAME)"
	$(IMAGE_BUILDER) build -f Dockerfile -t $(IMAGE_NAME) $(DOCKERARGS) .

image-push:
	$(IMAGE_BUILDER) push $(IMAGE_NAME)

test:
	go test ./... -count=1
	
test-coverage:
	go test ./... -coverprofile cover.out
	go tool cover -func cover.out

go-lint-install:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.49

go-lint: go-lint-install
	go mod tidy
	go fmt ./...
	golangci-lint run --color always -v ./... 

go-lint-report: go-lint-install
	golangci-lint run --color always -v ./... &> golangci-lint.txt
