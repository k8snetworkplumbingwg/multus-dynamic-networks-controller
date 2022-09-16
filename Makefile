GO ?= go
OCI_BIN ?= docker

IMAGE_REGISTRY ?= quay.io/mduarted
IMAGE_NAME ?= multus-dynamic-networks-controller
IMAGE_TAG ?= latest

.PHONY: manifests

all: build test

build:
	$(GO) build -o bin/dynamic-networks-controller cmd/dynamic-networks-controller/networks-controller.go

clean:
	rm -rf bin/ manifests/

img-build: build test
	$(OCI_BIN) build -t ${IMAGE_REGISTRY}/${IMAGE_NAME}:${IMAGE_TAG} -f images/Dockerfile .

manifests:
	IMAGE_REGISTRY=${IMAGE_REGISTRY} hack/generate_manifests.sh

test:
	$(GO) test -v ./...
