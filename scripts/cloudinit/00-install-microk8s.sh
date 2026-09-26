#!/bin/bash -xe

# This script runs from the boot.before stage, and on a first boot snapd is
# usually still seeding. Every snap call below refuses to run until the seed is
# loaded ("too early for operation, device not yet seeded"), and under `-e` that
# single refusal aborts the whole stage: MicroK8s is never installed, the node
# never joins, and nothing retries. Block here until snapd answers and the seed
# is in. `snap wait` is retried because it needs snapd itself to be reachable,
# which is not guaranteed this early in the boot either.
seedTimeout=600
seedDeadline=$(( $(date +%s) + seedTimeout ))
until snap wait system seed.loaded; do
  if [ "$(date +%s)" -ge "$seedDeadline" ]; then
    echo "snapd did not finish seeding within ${seedTimeout}s, refusing to install MicroK8s" >&2
    exit 1
  fi
  sleep 5
done

packageVersion=$(snap info /opt/microk8s/snaps/microk8s.snap |grep "version:" | awk '{ print $2 }')
channel=$(echo "${packageVersion%.*}" |cut -f2 -dv)
snap ack /opt/microk8s/snaps/core.assert
snap install /opt/microk8s/snaps/core.snap
snap ack /opt/microk8s/snaps/microk8s.assert
snap install --classic /opt/microk8s/snaps/microk8s.snap
snap switch  microk8s  --channel=$channel/stable
microk8s status --wait-ready
