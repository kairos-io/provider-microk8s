package main

type MicroK8sSpec struct {
	// InitConfiguration along with ClusterConfiguration are the configurations necessary for the init command
	ClusterConfiguration *ClusterConfiguration `json:"clusterConfiguration,omitempty"`

	InitConfiguration *InitConfiguration `json:"initConfiguration,omitempty"`
}

type ClusterConfiguration struct {

	// cluster agent port (25000) and dqlite port (19001) set to use ports 30000 and 2379 respectively
	// The default ports of cluster agent and dqlite are blocked by security groups and as a temporary
	// workaround we reuse the etcd and calico ports that are open in the infra providers because kubeadm uses those.

	// PortCompatibilityRemap switches the default ports used by cluster agent (25000) and dqlite (19001)
	// to 30000 and 2379. The default ports are blocked via security groups in several infra providers.
	PortCompatibilityRemap bool   `json:"portCompatibilityRemap,omitempty"`
	WriteKubeconfig        string `json:"writeKubeconfig,omitempty"`
	// Switches dqlite to bind to the first IPV4 address instead of the default 127.0.0.1
	DqliteUseHostIPV4Address bool `json:"dqliteUseHostIPV4Address,omitempty"`
	// Use host network dns
	UseHostDNS bool `json:"useHostDNS,omitempty"`
	// Use specified dns server
	DNS string `json:"dns,omitempty"`

	CalicoConfiguration *CalicoConfiguration `json:"calico,omitempty"`

}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type InitConfiguration struct {

	// The join token will expire after the specified seconds, defaults to 10 years

	JoinTokenTTLInSecs int64 `json:"joinTokenTTLInSecs,omitempty"`

	// The optional https proxy configuration
	HTTPSProxy string `json:"httpsProxy,omitempty"`

	// The optional http proxy configuration
	HTTPProxy string `json:"httpProxy,omitempty"`

	// The optional no proxy configuration
	NoProxy string `json:"noProxy,omitempty"`

	// List of addons to be enabled upon cluster creation
	Addons []string `json:"addons,omitempty"`
}

type CalicoConfiguration struct {
	// Calico IPinIP
	CalicoIPinIP bool `json:"calicoIPinIP,omitempty"`

	// Calico Autodetect
	CalicoAutoDetect string `json:"calicoAutoDetect,omitempty"`
}