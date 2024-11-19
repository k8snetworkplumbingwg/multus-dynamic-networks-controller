#!/bin/bash

if [ -z "$ARCH" ] || [ -z "$PLATFORMS" ] || [ -z "$MULTUS_DYNAMIC_IMAGE_TAGGED" ] || [ -z "$GIT_SHA" ]; then
    echo "Error: ARCH, PLATFORMS, MULTUS_DYNAMIC_IMAGE_TAGGED, and GIT_SHA must be set."
    exit 1
fi

# Split the comma-separated platforms into an array
IFS=',' read -r -a PLATFORM_LIST <<< "$PLATFORMS"

BUILD_ARGS="--no-cache --build-arg BUILD_ARCH=$ARCH --build-arg git_sha=$GIT_SHA -f images/Dockerfile -t $MULTUS_DYNAMIC_IMAGE_TAGGED --push ."

if [ ${#PLATFORM_LIST[@]} -eq 1 ]; then
    # Single platform build
    docker build --platform "$PLATFORMS" $BUILD_ARGS
else
    # Multi-platform build
    ./hack/init-buildx.sh "$DOCKER_BUILDER"
    docker buildx build --platform "$PLATFORMS" $BUILD_ARGS
fi
