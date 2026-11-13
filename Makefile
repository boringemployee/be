VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X github.com/boringemployee/be/internal/version.Version=$(VERSION)

.PHONY: all cli web test cover golden lint clean

all: lint test cli

cli: ## Build the native CLI into bin/be
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/be ./cmd/be

web: ## Build the WebAssembly app into web/dist (Phase 5)
	@echo "web build arrives in Phase 5"

test: ## Run all tests with the race detector
	go test -race ./...

cover: ## Run tests with a coverage report
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

golden: ## Regenerate golden files (Phase 3)
	@echo "golden files arrive in Phase 3"

lint: ## Vet and check formatting
	go vet ./...
	@test -z "$$(gofmt -l .)" || (gofmt -l . && exit 1)

clean: ## Remove build output
	rm -rf bin web/dist coverage.out
