# Kairos Microk8s Cluster Plugin

---

This provider will configure a Microk8s installation based on the cluster section of cloud init.

## Configuration

`cluster_token`: a token all members of the cluster must have to join the cluster.

`control_plane_host`: the host of the cluster control plane.  This is used to join nodes to a cluster.  If this is a single node cluster this is not required.

`role`: defines what operations is this device responsible for. The roles are described in detail below.
- `init` This role denotes a device that should initialize the dqlite cluster and operate as a Microk8s control plane.  There should only be one device with this role per cluster.
- `controlplane`: runs the Microk8s control plane.
- `worker`: runs the  Microk8s worker.

`config`: User provided configuration for microk8s. It supports the following configuration entries
 - `clusterConfiguration`: Defense cluster level parameters
 
          # Changes the default cluster agent port and the dqlite ports to use 30000 and 6443 which might be more likely to be open in firewalls
          `portCompatibilityRemap` : true
          
          # Writes the kubeconfig to a specified location
          `writeKubeconfig`: "/run/kubeconfig" 
          
          # Switch dqlite to use the internal IP of the Node instead of the 127.0.0.1 
          `dqliteUseHostIPV4Addres`: true
          
          # Uses the  DNS server entries from the host for the coredns configuration(reads from /etc/resolv.conf)
          `useHostDNS`: true
          
          # Specifies custom DNs server. Overrides the previous setting
          `DNS` : 75.75.74.74
  -  `initConfiguration`: Configuration only for the init node
  
                  addons:
                    - dns

### Example
```yaml
#cloud-config

cluster:
  cluster_token: randomstring
  control_plane_host: cluster.example.com
  role: init
  config: |
    clusterConfiguration:
          # Changes the default cluster agent port 
          portCompatibilityRemap : true
          writeKubeconfig: "/run/kubeconfig"
          dqliteUseHostIPV4Address: true
          useHostDNS: true
    initConfiguration:
                  addons:
                    - dns
