.PHONY: all build generate watch run clean fmt tidy

all: generate build

# Build final binary from Go files
build: templ css
	go build -o merge ./cmd/merge

# Generates public/main.css file from .scss files
# NOTE See package.json for invocation details
# TODO Remove npm dependency (added as Bulma dependency)
css:
	npm run build:css

# Generates _templ.go files for .templ components
# TODO If using generated Go code, place command here
templ:
	go tool templ generate

# Starts development server, refreshing when watched files are updated
# TODO Does not auto-heal if .templ files fail to generate
watch:
	npm run watch:css &
	go tool templ generate --watch --proxy="http://localhost:8080" --cmd="go run ./cmd/merge"

# Run the built executable
run: build
	./merge

# C
clean:
	rm -f merge

# Runs formatter commands
# TODO Add Prettier, templ fmt etc.
fmt:
	go fmt ./...
	go tool templ fmt .
	npx prettier --write "**/*.{scss,css,js,html,json}" "!node_modules/**" "!vendor/**"

# Tidies go modules, updating imports and dependencies
tidy:
	go mod tidy

# TODO Add deps target
deps:
	go mod download
	npm ci
