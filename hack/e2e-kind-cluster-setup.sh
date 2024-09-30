#!/bin/bash
set -xe

CLUSTER_TYPE=${CLUSTER_TYPE:-kind}
SKIP_MULTUS_DEPLOYMENT=${SKIP_MULTUS_DEPLOYMENT:-false}
MULTUS_VERSION=${MULTUS_VERSION:-latest}

CNI_VERSION=${CNI_VERSION:-0.4.0}
OCI_BIN=${OCI_BIN:-docker}
CRI=${CRI:-"containerd"} # possible values: containerd / crio
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

    if [[ $CLUSTER_TYPE == "kind" ]]; then
      kind load docker-image $IMG_REGISTRY/multus-dynamic-networks-controller:$IMG_TAG
    else
      "$OCI_BIN" push $IMG_REGISTRY/multus-dynamic-networks-controller:$IMG_TAG 
    fi
}

cleanup() {
    rm -rf multus-cni
    git checkout -- manifests/
}

install_specific_multus_version() {
  if [[ $MULTUS_VERSION == "latest" ]]; then
    echo "error: MULTUS_VERSION is required to be a specific version (i.e v4.1.2), not 'latest' when using non kind provider"
    exit 1
  fi

  echo "Installing multus-cni $MULTUS_VERSION daemonset ..."
  wget -qO- "https://raw.githubusercontent.com/k8snetworkplumbingwg/multus-cni/${MULTUS_VERSION}/deployments/multus-daemonset-thick.yml" |\
    sed -e "s|multus-cni:snapshot|multus-cni:${MULTUS_VERSION}|g" |\
    kubectl apply -f -
}

trap "cleanup" EXIT

if [[ $CLUSTER_TYPE == "kind" ]]; then
  git clone https://github.com/k8snetworkplumbingwg/multus-cni/
  start_registry_container
  setup_cluster
elif [[ $SKIP_MULTUS_DEPLOYMENT != true ]]; then
  install_specific_multus_version
fi

push_local_image

if [[ $CRI == "containerd" ]]; then
  kubectl apply -f manifests/dynamic-networks-controller.yaml
else
  kubectl apply -f manifests/crio-dynamic-networks-controller.yaml
fi

kubectl wait -nkube-system --for=condition=ready --timeout=180s -l app=dynamic-networks-controller pods
