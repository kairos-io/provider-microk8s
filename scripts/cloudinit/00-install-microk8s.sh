#!/bin/bash -xe

snap ack /opt/microk8s/snaps/core.assert
snap install /opt/microk8s/snaps/core.snap
snap ack /opt/microk8s/snaps/microk8s.assert
snap install --classic /opt/microk8s/snaps/microk8s.snap
microk8s status --wait-ready