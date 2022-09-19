#!/bin/bash
set -xe

CNI_VERSION=${CNI_VERSION:-0.4.0}
OCI_BIN=${OCI_BIN:-docker}

K8S_VERSION=${K8S_VERSION:-1.24}

IMAGE_REPO=${IMG_REPO:-maiqueb}
IMAGE_NAME="multus-dynamic-networks-controller"
IMAGE_TAG="latest"

setup_cluster() {
    export KUBEVIRTCI_TAG=`curl -L -Ss https://storage.googleapis.com/kubevirt-prow/release/kubevirt/kubevirtci/latest`
    git clone https://github.com/kubevirt/kubevirtci/
    trap "popd" RETURN SIGINT
    pushd kubevirtci
    KUBEVIRT_PROVIDER="k8s-$K8S_VERSION" KUBEVIRT_NUM_SECONDARY_NICS=1 make cluster-up
    export KUBECONFIG="$(pwd)/_ci-configs/k8s-1.24/.kubeconfig"
}

push_local_image() {
    OCI_BIN="$OCI_BIN" IMAGE_REGISTRY="registry:5000/$IMAGE_REPO" make manifests
    OCI_BIN="$OCI_BIN" IMAGE_REGISTRY="$IMG_REGISTRY" make img-build
    "$OCI_BIN" push --tls-verify=false "$IMG_REGISTRY/$IMAGE_NAME:$IMAGE_TAG"
}

publish_kubeconfig() {
    local kube_config_dir="${HOME}/.kube/"
    mkdir -p "$kube_config_dir"
    cp kubevirtci/_ci-configs/k8s-1.24/.kubeconfig "$kube_config_dir/config"
    echo "###"
    echo "   Repo kubeconfig moved to $kube_config_dir/config"
    echo "###"
}

cleanup() {
    rm -rf kubevirtci
    git checkout -- manifests/
}

trap "cleanup" EXIT
setup_cluster

registry_port=$(kubevirtci/cluster-up/cli.sh ports registry | tr -d '\r')
registry="localhost:$registry_port/$IMAGE_REPO"

IMG_REGISTRY="$registry" push_local_image

kubectl apply -f https://raw.githubusercontent.com/k8snetworkplumbingwg/multus-cni/master/deployments/multus-daemonset-thick.yml
kubectl wait -nkube-system --for=condition=ready --timeout=180s -l app=multus pods

publish_kubeconfig

kubectl apply -f manifests/dynamic-networks-controller.yaml
kubectl wait -nkube-system --for=condition=ready --timeout=180s -l app=dynamic-networks-controller pods
