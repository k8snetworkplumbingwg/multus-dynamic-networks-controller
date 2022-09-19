#!/bin/bash
set -xe

kubectl apply -f examples/original-setup.yaml
kubectl wait --for=condition=ready --timeout=60s pod macvlan1-worker1

kubectl apply -f examples/interface-add.yaml
kubectl apply -f examples/interface-remove.yaml
kubectl delete -f examples/original-setup.yaml

