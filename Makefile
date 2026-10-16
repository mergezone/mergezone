.PHONY: all build css templ watch run clean fmt tidy deps

NPM ?= pnpm

all: build

# Build final binary from Go files
build: templ css
	go build -o merge ./cmd/merge

# Generates public/main.css file from .scss files
# NOTE See package.json for invocation details
# TODO Remove npm dependency (added as Bulma dependency)
css:
	$(NPM) run build:css

# Generates _templ.go files for .templ components
# NOTE If using generated Go code, place command here
templ:
	go tool templ generate

# Starts development server, refreshing when watched files are updated
# TODO Does not auto-heal if .templ files fail to generate
watch:
	$(NPM) run watch:css &
	go tool templ generate --watch --proxy="http://localhost:8080" --cmd="go run ./cmd/merge"

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

deps:
	go mod download
	$(NPM) install --frozen-lockfile
