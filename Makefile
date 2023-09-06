GO ?= go
OCI_BIN ?= docker

IMAGE_REGISTRY ?= ghcr.io/k8snetworkplumbingwg
IMAGE_NAME ?= multus-dynamic-networks-controller
IMAGE_TAG ?= latest-amd64
NAMESPACE ?= kube-system

CONTAINERD_SOCKET_PATH ?= "/run/containerd/containerd.sock"
CRIO_SOCKET_PATH ?= "/run/crio/crio.sock"
MULTUS_SOCKET_PATH ?= "/run/multus/socket/multus.sock"

GIT_SHA := $(shell git describe --no-match  --always --abbrev=40 --dirty)

.PHONY: manifests

all: build test

build:
	$(GO) build -o bin/dynamic-networks-controller cmd/dynamic-networks-controller/networks-controller.go

clean:
	rm -rf bin/ manifests/

img-build: build test
	$(OCI_BIN) build -t ${IMAGE_REGISTRY}/${IMAGE_NAME}:${IMAGE_TAG} -f images/Dockerfile --build-arg git_sha=$(GIT_SHA) .

manifests:
	MULTUS_SOCKET_PATH=${MULTUS_SOCKET_PATH} IMAGE_REGISTRY=${IMAGE_REGISTRY} IMAGE_TAG=${IMAGE_TAG} CRI_SOCKET_PATH=${CONTAINERD_SOCKET_PATH} NAMESPACE=${NAMESPACE} hack/generate_manifests.sh
	CRIO_RUNTIME="yes" MULTUS_SOCKET_PATH=${MULTUS_SOCKET_PATH} IMAGE_REGISTRY=${IMAGE_REGISTRY} IMAGE_TAG=${IMAGE_TAG} CRI_SOCKET_PATH=${CRIO_SOCKET_PATH} NAMESPACE=${NAMESPACE} hack/generate_manifests.sh

test:
	$(GO) test -v ./pkg/...

e2e/test:
	$(GO) test -v -count=1 ./e2e/...
