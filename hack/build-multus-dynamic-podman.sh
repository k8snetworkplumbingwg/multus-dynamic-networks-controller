#!/bin/bash

if [ -z "$ARCH" ] || [ -z "$PLATFORMS" ] || [ -z "$MULTUS_DYNAMIC_IMAGE_TAGGED" ] || [ -z "$GIT_SHA" ]; then
    echo "Error: ARCH, PLATFORMS, MULTUS_DYNAMIC_IMAGE_TAGGED, and GIT_SHA must be set."
    exit 1
fi

# Split the comma-separated platforms into an array
IFS=',' read -r -a PLATFORM_LIST <<< "$PLATFORMS"

# Remove any existing manifest and image
podman manifest rm "${MULTUS_DYNAMIC_IMAGE_TAGGED}" || true
podman rmi "${MULTUS_DYNAMIC_IMAGE_TAGGED}" || true

# Create a manifest list
podman manifest create "${MULTUS_DYNAMIC_IMAGE_TAGGED}"

#for platform in $PLATFORM_LIST; do
for platform in "${PLATFORM_LIST[@]}"; do
    podman build \
        --no-cache \
        --build-arg BUILD_ARCH="$ARCH" \
	--build-arg git_sha="$GIT_SHA" \
        --platform "$platform" \
        --manifest "${MULTUS_DYNAMIC_IMAGE_TAGGED}" \
        -f images/Dockerfile \
        .
done
