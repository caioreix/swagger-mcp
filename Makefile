.DEFAULT_GOAL := help

BINARY := build/swagger-mcp
GO_DIRS := cmd internal test
GOFILES := $(shell find $(GO_DIRS) -type f -name '*.go')
ARGS ?=

.PHONY: help build run test vet fmt clean inspector verify

help:
	@printf "%s\n" \
		"Available targets:" \
		"  make build      Build the Go binary at $(BINARY)" \
		"  make run        Build and run the binary (pass extra args with ARGS='...')" \
		"  make test       Run go test ./..." \
		"  make vet        Run go vet ./..." \
		"  make fmt        Run gofmt on Go source files" \
		"  make clean      Remove build artifacts" \
		"  make inspector  Build and open the MCP inspector" \
		"  make verify     Run build, test, and vet"

build:
	mkdir -p $(dir $(BINARY))
	go build -o $(BINARY) ./cmd/swagger-mcp

run: build
	./$(BINARY) $(ARGS)

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w $(GOFILES)

clean:
	rm -rf build

inspector: build
	npx @modelcontextprotocol/inspector ./$(BINARY)

verify: build test vet
