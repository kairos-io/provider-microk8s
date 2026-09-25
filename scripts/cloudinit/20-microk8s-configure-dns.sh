#!/bin/bash -xe

# Usage:
#   $0 userHostDNS DNSIP [...]
#
# Assumptions:
#   - microk8s is installed
#   - microk8s apiserver is up and running

# enable community addons, this is for free and avoids confusion if addons are failing to install
microk8s status --wait-ready
microk8s enable community

# The dns addon reads its upstream nameservers from a single comma separated
# argument. Values separated by whitespace are read by "microk8s enable" as
# further addon names instead, and are dropped with an "Addon not found" notice
# and a zero exit code.
to_csv() {
  tr -s '[:space:],' ',' | sed -e 's/^,//' -e 's/,$//'
}

DNS=""
if [ "${1}" == "true" ]; then
  # Match on the keyword as a field, so that a commented out entry and a
  # tab indented one are read correctly.
  DNS=$(awk '$1 == "nameserver" { print $2 }' /etc/resolv.conf | to_csv)
fi
if [ "${2}" != "" ]; then
  DNS=$(printf '%s' "${2}" | to_csv)
fi
if [ "$DNS" != "" ]; then
   microk8s enable "dns:$DNS"
else
  microk8s enable dns
fi

microk8s status --wait-ready
