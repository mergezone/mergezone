.PHONY: all build css templ jaeger-up jaeger-down watch dev run clean fmt tidy deps

NPM ?= pnpm
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
JAEGER_CONTAINER ?= merge-jaeger
JAEGER_IMAGE ?= cr.jaegertracing.io/jaegertracing/jaeger:2.22.0
OTEL_EXPORTER_OTLP_ENDPOINT ?= http://localhost:4318
DEV_COMMAND = go run -ldflags "-X main.commit=$(COMMIT)" ./cmd/merge

all: build

# Build final binary from Go files
build: templ css
	go build -ldflags "-X main.commit=$(COMMIT)" -o merge ./cmd/merge

# Generates public/main.css file from .scss files
# NOTE See package.json for invocation details
# TODO Remove npm dependency (added as Bulma dependency)
css:
	$(NPM) run build:css

# Generates _templ.go files for .templ components
# NOTE If using generated Go code, place command here
templ:
	go tool templ generate

# Start local Jaeger with OTLP/HTTP and wait for its health endpoint.
jaeger-up:
	@if docker inspect $(JAEGER_CONTAINER) >/dev/null 2>&1; then \
		image=$$(docker inspect -f '{{.Config.Image}}' $(JAEGER_CONTAINER)); \
		if [ "$$image" != "$(JAEGER_IMAGE)" ]; then docker rm -f $(JAEGER_CONTAINER) >/dev/null; fi; \
	fi
	@if docker inspect $(JAEGER_CONTAINER) >/dev/null 2>&1; then \
		docker start $(JAEGER_CONTAINER) >/dev/null 2>&1 || true; \
	else \
		docker run -d --name $(JAEGER_CONTAINER) -p 16686:16686 -p 4317:4317 -p 4318:4318 $(JAEGER_IMAGE); \
	fi
	@ready=0; for attempt in $$(seq 1 30); do \
		if curl -fsS http://localhost:16686/ >/dev/null 2>&1; then ready=1; break; fi; \
		sleep 1; \
	done; \
	if [ "$$ready" -eq 1 ]; then \
		echo "Jaeger UI: http://localhost:16686"; \
	else \
		echo "Jaeger did not become ready; check: docker logs $(JAEGER_CONTAINER)" >&2; \
		exit 1; \
	fi

jaeger-down:
	-docker rm -f $(JAEGER_CONTAINER)

# Starts development server, refreshing when watched files are updated.
watch: jaeger-up
	$(NPM) run watch:css &
	OTEL_EXPORTER_OTLP_ENDPOINT=$(OTEL_EXPORTER_OTLP_ENDPOINT) go tool templ generate --watch --proxy="http://localhost:8080" --cmd='$(DEV_COMMAND)'

dev: jaeger-up
	OTEL_EXPORTER_OTLP_ENDPOINT=$(OTEL_EXPORTER_OTLP_ENDPOINT) $(DEV_COMMAND)

# Run the built executable
run: build
	./merge

# C
clean:
	rm -f merge

# Runs formatter commands
fmt:
	go fmt ./...
	go tool templ fmt .
	$(NPM) run format

# Tidies go modules, updating imports and dependencies
tidy:
	go mod tidy

# Install dependencies
deps:
	go mod download
	$(NPM) install --frozen-lockfile

# Test CI locally
ci:
	act --pull=false --container-options "-u 0:0"
