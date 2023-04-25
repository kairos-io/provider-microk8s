#!/bin/bash -xe

packageVersion=$(snap info /opt/microk8s/snaps/microk8s.snap |grep "version:" | awk '{ print $2 }')
channel=$(echo "${packageVersion%.*}" |cut -f2 -dv)
snap wait system seed.loaded
snap ack /opt/microk8s/snaps/core.assert
snap install /opt/microk8s/snaps/core.snap
snap ack /opt/microk8s/snaps/microk8s.assert
snap install --classic /opt/microk8s/snaps/microk8s.snap
snap switch  microk8s  --channel=$channel/stable
microk8s status --wait-ready