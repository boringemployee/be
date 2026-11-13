VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X github.com/boringemployee/be/internal/version.Version=$(VERSION)

.PHONY: all cli web test cover fuzz golden lint clean

all: lint test cover cli

cli: ## Build the native CLI into bin/be
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/be ./cmd/be

web: ## Build the WebAssembly app into web/dist (Phase 5)
	@echo "web build arrives in Phase 5"

test: ## Run all tests with the race detector
	go test -race ./...

SHARED := content internal/engine internal/generator internal/profile internal/render

cover: ## Run tests and require 100% coverage on shared packages
	go test -coverprofile=coverage.out ./...
	@go tool cover -func=coverage.out | awk -v shared="$(SHARED)" ' \
		BEGIN { n = split(shared, s, " ") } \
		{ for (i = 1; i <= n; i++) if (index($$1, "/" s[i] "/") && $$NF != "100.0%") { print "not fully covered:", $$0; bad = 1 } } \
		END { if (bad) exit 1; print "shared packages: 100% covered" }'

fuzz: ## Run each fuzz test briefly
	go test -run='^$$' -fuzz=FuzzParse -fuzztime=30s ./internal/profile
	go test -run='^$$' -fuzz=FuzzHandleLine -fuzztime=30s ./internal/engine

golden: ## Regenerate golden files (Phase 3)
	@echo "golden files arrive in Phase 3"

lint: ## Vet and check formatting
	go vet ./...
	@test -z "$$(gofmt -l .)" || (gofmt -l . && exit 1)

clean: ## Remove build output
	rm -rf bin web/dist coverage.out
