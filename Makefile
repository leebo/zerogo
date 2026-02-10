.PHONY: all build clean agent controller relay cli test lint

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION)"

all: build

build: agent controller relay cli

agent:
	go build $(LDFLAGS) -o bin/zerogo-agent ./cmd/zerogo-agent

controller:
	go build $(LDFLAGS) -o bin/zerogo-controller ./cmd/zerogo-controller

relay:
	go build $(LDFLAGS) -o bin/zerogo-relay ./cmd/zerogo-relay

cli:
	go build $(LDFLAGS) -o bin/zerogo-cli ./cmd/zerogo-cli

test:
	go test -race -count=1 ./...

lint:
	golangci-lint run ./...

clean:
	rm -rf bin/

# Cross-compilation targets
build-linux-amd64:
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o bin/zerogo-agent-linux-amd64 ./cmd/zerogo-agent
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o bin/zerogo-controller-linux-amd64 ./cmd/zerogo-controller

build-linux-arm64:
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o bin/zerogo-agent-linux-arm64 ./cmd/zerogo-agent

build-windows:
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o bin/zerogo-agent-windows-amd64.exe ./cmd/zerogo-agent

build-openwrt-mips:
	GOOS=linux GOARCH=mipsle GOMIPS=softfloat go build $(LDFLAGS) -o bin/zerogo-agent-mipsle ./cmd/zerogo-agent
