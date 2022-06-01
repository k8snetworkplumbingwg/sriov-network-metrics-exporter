clean:
	rm -rf bin
	go clean --modcache

build:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.45.2
	go mod tidy
	go fmt ./...
	golangci-lint run 
	GO111MODULE=on go build -ldflags "-s -w" -buildmode=pie -o bin/sriov-exporter cmd/sriov-network-metrics-exporter.go
