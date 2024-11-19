GO ?= go
OCI_BIN ?= docker

IMAGE_REGISTRY ?= ghcr.io/k8snetworkplumbingwg
IMAGE_NAME ?= multus-dynamic-networks-controller
IMAGE_TAG ?= latest
NAMESPACE ?= kube-system

CONTAINERD_SOCKET_PATH ?= "/run/containerd/containerd.sock"
CRIO_SOCKET_PATH ?= "/run/crio/crio.sock"
MULTUS_SOCKET_PATH ?= "/run/multus/multus.sock"
PLATFORMS ?= linux/amd64,linux/s390x
# Set the platforms for building a multi-platform supported image.
# Example:
# PLATFORMS ?= linux/amd64,linux/arm64,linux/s390x
# Alternatively, you can export the PLATFORMS variable like this:
# export PLATFORMS=linux/arm64,linux/s390x,linux/amd64
ARCH := $(shell uname -m | sed 's/x86_64/amd64/')
MULTUS_DYNAMIC_IMAGE_TAGGED := $(IMAGE_REGISTRY)/$(IMAGE_NAME):$(IMAGE_TAG)
DOCKER_BUILDER ?= multus-dynamic-docker-builder
GIT_SHA := $(shell git describe --no-match  --always --abbrev=40 --dirty)

.PHONY: manifests \
        vendor

all: build test

build:
	$(GO) build -o bin/dynamic-networks-controller cmd/dynamic-networks-controller/networks-controller.go

clean:
	rm -rf bin/ manifests/

img-build: build test
ifeq ($(OCI_BIN),podman)
	$(MAKE) build-multiarch-multus-dynamic-podman
else ifeq ($(OCI_BIN),docker)
	$(MAKE) build-multiarch-multus-dynamic-docker
else
	$(error Unsupported OCI_BIN value: $(OCI_BIN))
endif

manifests:
	MULTUS_SOCKET_PATH=${MULTUS_SOCKET_PATH} IMAGE_REGISTRY=${IMAGE_REGISTRY} IMAGE_TAG=${IMAGE_TAG} CRI_SOCKET_PATH=${CONTAINERD_SOCKET_PATH} NAMESPACE=${NAMESPACE} hack/generate_manifests.sh
	CRIO_RUNTIME="yes" MULTUS_SOCKET_PATH=${MULTUS_SOCKET_PATH} IMAGE_REGISTRY=${IMAGE_REGISTRY} IMAGE_TAG=${IMAGE_TAG} CRI_SOCKET_PATH=${CRIO_SOCKET_PATH} NAMESPACE=${NAMESPACE} hack/generate_manifests.sh

test:
	$(GO) test -v ./pkg/...

e2e/test:
	$(GO) test -v -count=1 ./e2e/...

vendor:
	$(GO) mod tidy
	$(GO) mod vendor

build-multiarch-multus-dynamic-docker:
	ARCH=$(ARCH) PLATFORMS=$(PLATFORMS) MULTUS_DYNAMIC_IMAGE_TAGGED=$(MULTUS_DYNAMIC_IMAGE_TAGGED) GIT_SHA=$(GIT_SHA) DOCKER_BUILDER=$(DOCKER_BUILDER) ./hack/build-multus-dynamic-docker.sh

build-multiarch-multus-dynamic-podman:
	ARCH=$(ARCH) PLATFORMS=$(PLATFORMS) MULTUS_DYNAMIC_IMAGE_TAGGED=$(MULTUS_DYNAMIC_IMAGE_TAGGED) GIT_SHA=$(GIT_SHA) ./hack/build-multus-dynamic-podman.sh
