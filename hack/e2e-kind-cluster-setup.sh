#!/bin/bash
set -xe

CNI_VERSION=${CNI_VERSION:-0.4.0}
OCI_BIN=${OCI_BIN:-docker}
IMG_REGISTRY=${IMAGE_REGISTRY:-localhost:5000/k8snetworkplumbingwg}
IMG_TAG="latest"

setup_cluster() {
    git clone https://github.com/k8snetworkplumbingwg/multus-cni/
    pushd multus-cni/e2e
    trap "popd" RETURN SIGINT
    ./get_tools.sh
    CNI_VERSION="$CNI_VERSION" ./generate_yamls.sh
    OCI_BIN="$OCI_BIN" ./setup_cluster.sh
}

push_local_image() {
    OCI_BIN="$OCI_BIN" IMAGE_REGISTRY="$IMG_REGISTRY" IMAGE_TAG="$IMG_TAG" make manifests
    OCI_BIN="$OCI_BIN" IMAGE_REGISTRY="$IMG_REGISTRY" IMAGE_TAG="$IMG_TAG" make img-build
    "$OCI_BIN" push "$IMG_REGISTRY/multus-dynamic-networks-controller:$IMG_TAG"
}

cleanup() {
    rm -rf multus-cni
    git checkout -- manifests/
}

trap "cleanup" EXIT
setup_cluster
push_local_image
kubectl apply -f manifests/dynamic-networks-controller.yaml
kubectl wait -nkube-system --for=condition=ready --timeout=180s -l app=dynamic-networks-controller pods
