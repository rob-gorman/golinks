TOP_DIR := $(realpath $(dir $(abspath $(lastword $(MAKEFILE_LIST)))))

.PHONY: all
all: \

include Makefrag

build:
	mkdir -p build

.PHONY: clean
clean:
	rm -rf build
	rm -rf tools
	rm -f .prereqs coverage.out
