#!/bin/sh

set -e
HERE="$(dirname "$(readlink --canonicalize $0)")"
ROOT="$(readlink --canonicalize "$HERE/..")"
templates_dir="$ROOT/templates"

for file in `ls $templates_dir/`; do
	echo $file
	if [ -z $CRIO_RUNTIME ]; then
	  j2 -e MULTUS_SOCKET_PATH -e IMAGE_REGISTRY -e IMAGE_TAG -e CRI_SOCKET_PATH -e NAMESPACE ${templates_dir}/$file -o "manifests/${file%.j2}"
	else
	  if [ $file != "dynamic-networks-controller.yaml.j2" ]; then
	    continue
	  fi
	  j2 -e MULTUS_SOCKET_PATH -e CRIO_RUNTIME -e IMAGE_REGISTRY -e IMAGE_TAG -e CRI_SOCKET_PATH -e NAMESPACE ${templates_dir}/$file -o "manifests/crio-${file%.j2}"
	fi
done
unset IMAGE_REGISTRY
unset IMAGE_TAG
unset CRI_SOCKET_PATH
unset NAMESPACE
unset MULTUS_SOCKET_PATH
