.PHONY: build clean test install example

build:
	go build -o mkdown

clean:
	rm -f mkdown
	rm -f examples/*.html

test:
	go test -v ./...

install:
	go install

example: build
	./mkdown examples/sample.md
	@echo "\nGenerated examples/sample.html - open it in your browser to preview"

run: example

all: clean build test

help:
	@echo "mkdown - Markdown to HTML converter"
	@echo ""
	@echo "Available targets:"
	@echo "  build    - Build the mkdown binary"
	@echo "  clean    - Remove built binaries and generated HTML"
	@echo "  test     - Run tests"
	@echo "  install  - Install to \$$GOPATH/bin"
	@echo "  example  - Build and run on sample.md"
	@echo "  all      - Clean, build, and test"

