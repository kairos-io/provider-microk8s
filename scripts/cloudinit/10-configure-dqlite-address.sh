#!/bin/bash -xe

# Usage:
#   $0 $new_dqlite_addr $new_dqlite_address
#
# Assumptions:
#   - microk8s is installed
#   - dqlite has been initialized on the node and is running

DQLITE="/var/snap/microk8s/current/var/kubernetes/backend"

microk8s status --wait-ready

INTERNAL_IP=$(microk8s kubectl get nodes -o json | jq -r '.items[].status.addresses[] | select(.type=="InternalIP") | .address')


grep "Address" "${DQLITE}/info.yaml" | sed "s/127.0.0.1/$INTERNAL_IP/" | tee "${DQLITE}/update.yaml"

snap restart microk8s.daemon-k8s-dqlite