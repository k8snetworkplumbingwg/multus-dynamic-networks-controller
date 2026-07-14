#!/bin/bash
set -xe

CNI_VERSION=${CNI_VERSION:-0.4.0}
OCI_BIN=${OCI_BIN:-docker}

K8S_VERSION=${K8S_VERSION:-1.24}

IMAGE_REPO=${IMG_REPO:-maiqueb}
IMAGE_NAME="multus-dynamic-networks-controller"
IMAGE_TAG="latest"

push_local_image() {
    OCI_BIN="$OCI_BIN" IMAGE_REGISTRY="registry:5000/$IMAGE_REPO" make manifests
    OCI_BIN="$OCI_BIN" IMAGE_REGISTRY="$IMG_REGISTRY" make img-build
    "$OCI_BIN" push --tls-verify=false "$IMG_REGISTRY/$IMAGE_NAME:$IMAGE_TAG"
}

setup_registry() {
  # https://minikube.sigs.k8s.io/docs/handbook/registry/
  minikube addons enable registry
  "$OCI_BIN" run --rm -it --network=host alpine ash -c "apk add socat && socat TCP-LISTEN:5000,reuseaddr,fork TCP:$(minikube ip):5000"
}

cleanup() {
    git checkout -- manifests/
    echo "=============="
    echo "  minikube logs:"
    echo "=============="
    minikube logs
}

trap "cleanup" EXIT

setup_registry

IMG_REGISTRY="localhost:5000/$IMAGE_REPO" push_local_image

kubectl apply -f https://raw.githubusercontent.com/k8snetworkplumbingwg/multus-cni/master/deployments/multus-daemonset-thick.yml
kubectl wait -nkube-system --for=condition=ready --timeout=180s -l app=multus pods

kubectl apply -f manifests/dynamic-networks-controller.yaml
kubectl wait -nkube-system --for=condition=ready --timeout=180s -l app=dynamic-networks-controller pods
