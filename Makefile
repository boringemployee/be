VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X github.com/boringemployee/be/internal/version.Version=$(VERSION)

.PHONY: all cli web serve e2e dist test cover fuzz golden lint clean

all: lint test cover cli

cli: ## Build the native CLI into bin/be
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/be ./cmd/be

WEB_FILES := web/index.html web/main.js web/style.css web/favicon.svg web/_headers

web: ## Build the static web app into web/dist
	cd web && npm ci --ignore-scripts --no-audit --no-fund
	mkdir -p web/dist
	GOOS=js GOARCH=wasm go build -trimpath -ldflags "$(LDFLAGS)" -o web/dist/be.wasm ./cmd/wasm
	cp "$$(go env GOROOT)/lib/wasm/wasm_exec.js" web/dist/
	cp web/node_modules/@xterm/xterm/lib/xterm.js web/node_modules/@xterm/xterm/css/xterm.css web/dist/
	cp $(WEB_FILES) web/dist/

serve: web ## Serve web/dist on http://localhost:8080 with production headers
	node tests/web/serve.mjs web/dist 8080

e2e: web ## Browser tests: golden files and an interactive session
	cd tests/web && npm ci --ignore-scripts --no-audit --no-fund
	node tests/web/e2e.mjs

PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64

dist: ## Build release archives and checksums.txt into dist/
	mkdir -p dist
	set -e; for p in $(PLATFORMS); do \
		os=$${p%/*}; arch=$${p#*/}; name=be_$(VERSION)_$${os}_$${arch}; ext=; \
		[ $$os = windows ] && ext=.exe; \
		mkdir -p dist/$$name; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -trimpath -ldflags "$(LDFLAGS)" -o dist/$$name/be$$ext ./cmd/be; \
		cp LICENSE README.md dist/$$name/; \
		if [ $$os = windows ]; then (cd dist && zip -qr $$name.zip $$name); \
		else tar -C dist -czf dist/$$name.tar.gz $$name; fi; \
	done
	cd dist && sha256sum *.tar.gz *.zip > checksums.txt

test: ## Run all tests with the race detector
	go test -race ./...

SHARED := content internal/engine internal/generator internal/profile internal/render internal/scenario

cover: ## Run tests and require 100% coverage on shared packages
	go test -coverprofile=coverage.out ./...
	@go tool cover -func=coverage.out | awk -v shared="$(SHARED)" ' \
		BEGIN { n = split(shared, s, " ") } \
		{ for (i = 1; i <= n; i++) if (index($$1, "/" s[i] "/") && $$NF != "100.0%") { print "not fully covered:", $$0; bad = 1 } } \
		END { if (bad) exit 1; print "shared packages: 100% covered" }'

fuzz: ## Run each fuzz test briefly
	go test -run='^$$' -fuzz=FuzzParse -fuzztime=30s ./internal/profile
	go test -run='^$$' -fuzz=FuzzHandleLine -fuzztime=30s ./internal/engine

golden: ## Regenerate golden files; review the diff before committing
	go test ./tests/golden -update

lint: ## Vet and check formatting
	go vet ./...
	@test -z "$$(gofmt -l .)" || (gofmt -l . && exit 1)

clean: ## Remove build output
	rm -rf bin dist web/dist coverage.out
