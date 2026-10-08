.PHONY: all build run generate clean install tidy fmt

all: generate build

build:
	go build -o merge ./cmd/merge

generate:
	npm run build:css
	go tool templ generate

watch:
	npm run watch:css &
	go tool templ generate --watch --proxy="http://localhost:8080" --cmd="go run ./cmd/merge"

run: build
	./merge

clean:
	rm -f merge

install:
	go install github.com/a-h/templ/cmd/templ@latest

fmt:
	go fmt ./...

tidy:
	go mod tidy
