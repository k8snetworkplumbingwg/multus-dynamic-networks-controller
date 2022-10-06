#!/bin/sh

set -e
HERE="$(dirname "$(readlink --canonicalize $0)")"
ROOT="$(readlink --canonicalize "$HERE/..")"
templates_dir="$ROOT/templates"

for file in `ls $templates_dir/`; do
	echo $file
	if [ -z $CRIO_RUNTIME ]; then
	  j2 -e IMAGE_REGISTRY -e IMAGE_TAG -e CRI_SOCKET_PATH -e NAMESPACE ${templates_dir}/$file -o "manifests/${file%.j2}"
	else
	  j2 -e CRIO_RUNTIME -e IMAGE_REGISTRY -e IMAGE_TAG -e CRI_SOCKET_PATH -e NAMESPACE ${templates_dir}/$file -o "manifests/crio-${file%.j2}"
	fi
done
unset IMAGE_REGISTRY
unset IMAGE_TAG
unset CRI_SOCKET_PATH
unset NAMESPACE
