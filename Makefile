TOP_DIR := $(realpath $(dir $(abspath $(lastword $(MAKEFILE_LIST)))))

.PHONY: all
all: build test

build:
	mkdir -p build
	go build -o build/golinks cmd/golinks/main.go

.PHONY: test
test:
	go test -v -race -covermode=atomic -coverprofile=coverage.out ./...

.PHONY: clean
clean:
	rm -rf build
	rm -rf tools
	rm -f .prereqs coverage.out
