clean:
	rm -rf bin
	go clean --modcache

build:
	go get -u golang.org/x/lint/golint
	go mod tidy
	go fmt ./...
	golint ./...
	GO111MODULE=on go build -ldflags "-s -w" -buildmode=pie -o bin/sriov-exporter cmd/sriov-network-metrics-exporter.go
