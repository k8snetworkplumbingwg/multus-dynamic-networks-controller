GO ?= go

all: build test

build:
	$(GO) build -o bin/dynamic-networks-controller -v cmd/dynamic-networks-controller/networks-controller.go

test:
	$(GO) test -v ./...
