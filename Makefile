.DEFAULT_GOAL := help

.PHONY: help dev dev-go dev-web build build-web build-go test test-go test-web lint lint-go lint-web clean e2e

help:
	@echo "Available targets:"
	@echo "  make dev-go     - Run Go server with hot reload (terminal 1)"
	@echo "  make dev-web    - Run Vite frontend (terminal 2)"
	@echo "  make build      - Build SPA + Go binary (production artifact: ./sigil-manager)"
	@echo "  make test       - Run all unit tests (Go + frontend)"
	@echo "  make e2e        - Build the binary then run Playwright e2e specs"
	@echo "  make lint       - Run all linters"
	@echo "  make clean      - Remove build artifacts"

dev-go:
	air

dev-web:
	cd web && npm run dev

build-web:
	cd web && npm run build
	rm -rf internal/server/dist
	cp -r web/dist internal/server/dist

build-go:
	go build -ldflags "-X github.com/Ju571nK/sigil-manager/internal/api.Version=$$(git describe --always --dirty 2>/dev/null || echo dev)" -o sigil-manager ./cmd/sigil-manager

build: build-web build-go

test-go:
	go test ./...

test-web:
	cd web && npm test -- --run

test: test-go test-web

lint-go:
	golangci-lint run ./...

lint-web:
	cd web && npm run lint

lint: lint-go lint-web

clean:
	rm -rf tmp/ sigil-manager web/dist/ internal/server/dist/*
	@mkdir -p internal/server/dist
	@echo '<!doctype html><html><body><p>Placeholder. Build the frontend with `make build`.</p></body></html>' > internal/server/dist/index.html

# Build the production binary (SPA + Go) and run Playwright. Playwright's
# webServer config boots ../sigil-manager with MOCK_FLEET=1 + the test
# admin creds, then runs the specs against that binary's embedded SPA.
e2e: build
	cd web && npm run e2e
