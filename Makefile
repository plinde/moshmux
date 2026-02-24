# moshmux Makefile

BINARY := moshmux

.PHONY: build run clean deps check lint test fmt update help

.DEFAULT_GOAL := help

##@ Development

build: ## Build moshmux binary
	CGO_ENABLED=0 go build -o $(BINARY) ./cmd/moshmux

run: build ## Build and run moshmux
	./$(BINARY)

deps: ## Download Go dependencies
	go mod download

check: lint test ## Run linters and tests

lint: ## Run golangci-lint
	golangci-lint run ./...

test: ## Run tests with race detector
	go test -race ./...

fmt: ## Format Go source files
	gofmt -w .

clean: ## Remove built binary
	rm -f $(BINARY)

##@ Operations

update: ## Pull and rebuild
	git pull
	$(MAKE) build

##@ Help

help: ## Show this help message
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
