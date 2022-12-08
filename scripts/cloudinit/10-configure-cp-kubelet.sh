#!/bin/bash -xe

#KUBELET_ARGS="${KUBELET_ARGS:-/var/snap/microk8s/current/args/kubelet}"

#sed -i 's/node-labels="\(.*\)"/node-labels="node.kubernetes.io\/controlplane=controlplane,\1"/g' "${KUBELET_ARGS}"

#snap restart microk8s.daemon-kubelite
#microk8s status --wait-ready
NODE_NAME=$(microk8s kubectl get nodes -o json | jq -r .items[].metadata.name)
microk8s kubectl label nodes $NODE_NAME node-role.kubernetes.io/control-plane=true