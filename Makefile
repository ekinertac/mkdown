.PHONY: build clean test test-coverage install example

build:
	go build -o mkdown ./cmd/mkdown

clean:
	rm -f mkdown
	rm -f examples/*.html
	rm -f coverage.out coverage.html

test:
	go test -v ./...

test-coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

install:
	go install ./cmd/mkdown

example: build
	./mkdown examples/sample.md
	@echo "\nGenerated examples/sample.html - open it in your browser to preview"

run: example

all: clean build test

help:
	@echo "mkdown - Markdown to HTML converter"
	@echo ""
	@echo "Available targets:"
	@echo "  build          - Build the mkdown binary"
	@echo "  clean          - Remove built binaries and generated HTML"
	@echo "  test           - Run tests"
	@echo "  test-coverage  - Run tests with coverage report"
	@echo "  install        - Install to \$$GOPATH/bin"
	@echo "  example        - Build and run on sample.md"
	@echo "  all            - Clean, build, and test"

