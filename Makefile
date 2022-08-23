GO ?= go
OCI_BIN ?= docker

IMAGE_REGISTRY ?= quay.io/maiqueb
IMAGE_NAME ?= multus-dynamic-networks-controller
IMAGE_TAG ?= latest

all: build test

build:
	$(GO) build -o bin/dynamic-networks-controller cmd/dynamic-networks-controller/networks-controller.go

img-build: build test
	$(OCI_BIN) build -t ${IMAGE_REGISTRY}/${IMAGE_NAME}:${IMAGE_TAG} -f images/Dockerfile .

test:
	$(GO) test -v ./...
