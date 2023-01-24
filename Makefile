IMAGE_REGISTRY?=localhost:5000/
IMAGE_VERSION?=latest

IMAGE_NAME?=$(IMAGE_REGISTRY)sriov-metrics-exporter:$(IMAGE_VERSION)

DOCKERARGS?=
ifdef HTTP_PROXY
	DOCKERARGS += --build-arg http_proxy=$(HTTP_PROXY)
endif
ifdef HTTPS_PROXY
	DOCKERARGS += --build-arg https_proxy=$(HTTPS_PROXY)
endif

all: build docker-build test 

clean:
	rm -rf bin
	go clean -modcache -testcache
	
build:
	GO111MODULE=on go build -ldflags "-s -w" -buildmode=pie -o bin/sriov-exporter cmd/sriov-network-metrics-exporter.go

docker-build:
	@echo "Bulding Docker image $(IMAGE_NAME)"
	docker build -f Dockerfile -t $(IMAGE_NAME) $(DOCKERARGS) .

docker-push:
	docker push $(IMAGE_NAME)

test:
	go test ./... -coverprofile cover.out

test-coverage:
	ginkgo -v -r -cover -coverprofile=cover.out --output-dir=.
	go tool cover -html=cover.out	

go-lint:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.49
	go mod tidy
	go fmt ./...
	golangci-lint run --color always -v ./... 

go-lint-report:
	golangci-lint run --color always -v ./... &> golangci-lint.txt
