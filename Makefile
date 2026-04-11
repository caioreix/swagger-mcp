.DEFAULT_GOAL := help

BINARY := build/swagger-mcp
GO_DIRS := cmd internal test
GOFILES := $(shell find $(GO_DIRS) -type f -name '*.go')
ARGS ?=
EVAL_FILE ?= evaluation.xml
EVAL_PROVIDER ?= copilot
EVAL_MODEL ?=
EVAL_OUTPUT ?=

.PHONY: help build run test vet fmt clean inspector verify evaluate

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
		"  make verify     Run build, test, and vet" \
		"  make evaluate   Run evaluation against $(EVAL_FILE) (EVAL_PROVIDER=copilot|anthropic|openai)"

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

lint:
	golangci-lint run

lint-fix:
	golangci-lint run --fix

clean:
	rm -rf build

inspector: build
	npx @modelcontextprotocol/inspector ./$(BINARY)

evaluate: build
	@cd scripts && \
	[ -d .venv ] || uv venv -q .venv && \
	uv pip install -q -r requirements.txt --python .venv/bin/python3 && \
	GITHUB_TOKEN=$$(gh auth token) .venv/bin/python3 evaluation.py \
		-p $(EVAL_PROVIDER) \
		$(if $(EVAL_MODEL),-m $(EVAL_MODEL)) \
		-t stdio \
		-c ../$(BINARY) \
		-s $(abspath testdata/petstore.json) \
		$(if $(EVAL_OUTPUT),-o $(EVAL_OUTPUT)) \
		../$(EVAL_FILE)


