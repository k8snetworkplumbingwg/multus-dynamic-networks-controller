GO ?= go
OCI_BIN ?= docker

IMAGE_REGISTRY ?= ghcr.io/maiqueb
IMAGE_NAME ?= multus-dynamic-networks-controller
IMAGE_TAG ?= latest-amd64

CRI_SOCKET_PATH ?= "/host/run/containerd/containerd.sock"

.PHONY: manifests

all: build test

build:
	$(GO) build -o bin/dynamic-networks-controller cmd/dynamic-networks-controller/networks-controller.go

clean:
	rm -rf bin/ manifests/

img-build: build test
	$(OCI_BIN) build -t ${IMAGE_REGISTRY}/${IMAGE_NAME}:${IMAGE_TAG} -f images/Dockerfile .

manifests:
	IMAGE_REGISTRY=${IMAGE_REGISTRY} IMAGE_TAG=${IMAGE_TAG} CRI_SOCKET_PATH=${CRI_SOCKET_PATH} hack/generate_manifests.sh

test:
	$(GO) test -v ./...
