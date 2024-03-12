#!/bin/bash
set -xe

CNI_VERSION=${CNI_VERSION:-0.4.0}
OCI_BIN=${OCI_BIN:-docker}
IMG_REGISTRY=${IMAGE_REGISTRY:-localhost:5000/k8snetworkplumbingwg}
IMG_TAG="e2e"

start_registry_container() {
    pushd multus-cni
    trap "popd" RETURN SIGINT
    "$OCI_BIN" run -d --restart=always -p "5000:5000" --name "kind-registry" registry:2
    "$OCI_BIN" build -t localhost:5000/multus:e2e -f images/Dockerfile.thick .
    "$OCI_BIN" push localhost:5000/multus:e2e
}

setup_cluster() {
    pushd multus-cni/e2e
    trap "popd" RETURN SIGINT
    ./get_tools.sh
    CNI_VERSION="$CNI_VERSION" ./generate_yamls.sh
    OCI_BIN="$OCI_BIN" ./setup_cluster.sh
}

push_local_image() {
    OCI_BIN="$OCI_BIN" IMAGE_REGISTRY="$IMG_REGISTRY" IMAGE_TAG="$IMG_TAG" make manifests
    OCI_BIN="$OCI_BIN" IMAGE_REGISTRY="$IMG_REGISTRY" IMAGE_TAG="$IMG_TAG" make img-build
    kind load docker-image $IMG_REGISTRY/multus-dynamic-networks-controller:$IMG_TAG
}

cleanup() {
    rm -rf multus-cni
    git checkout -- manifests/
}

trap "cleanup" EXIT
git clone https://github.com/k8snetworkplumbingwg/multus-cni/
start_registry_container
setup_cluster
push_local_image
kubectl apply -f manifests/dynamic-networks-controller.yaml
kubectl wait -nkube-system --for=condition=ready --timeout=180s -l app=dynamic-networks-controller pods
