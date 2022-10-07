#!/bin/sh

set -e
HERE="$(dirname "$(readlink --canonicalize $0)")"
ROOT="$(readlink --canonicalize "$HERE/..")"
templates_dir="$ROOT/templates"

for file in `ls $templates_dir/`; do
	echo $file
	j2 -e IMAGE_REGISTRY -e IMAGE_TAG -e CRI_SOCKET_PATH -e NAMESPACE ${templates_dir}/$file -o manifests/${file%.j2}
done
unset IMAGE_REGISTRY
unset IMAGE_TAG
unset CRI_SOCKET_PATH
unset NAMESPACE
