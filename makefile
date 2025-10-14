TOP_DIR := $(realpath $(dir $(abspath $(lastword $(MAKEFILE_LIST)))))

.PHONY: all
all: test dns-server

build:
	mkdir -p build

.PHONY: dns-server
dns-server: build
	# we run this on the raspberry pi
	CGO_ENABLED=0 GOARCH=arm64 go build -o build/dns-server ./cmd/server

.PHONY: test
test:
	go test -v -race -covermode=atomic -coverprofile=coverage.out ./...

.PHONY: prereqs
prereqs:
	go install github.com/air-verse/air@latest
	go install github.com/a-h/templ/cmd/templ@latest

.PHONY: clean
clean:
	rm -rf build
	rm -rf tools
	rm -f .prereqs coverage.out

-include makefrag
