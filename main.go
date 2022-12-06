package main

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/kairos-io/kairos/sdk/clusterplugin"
	yip "github.com/mudler/yip/pkg/schema"
	"github.com/sirupsen/logrus"
	"io"
	"net"
	"os"
	"path/filepath"
	kyaml "sigs.k8s.io/yaml"
	"strings"
)

const (
	scriptBasePath                  = "/opt/microk8s/scripts/"
	installMicrok8sScript           = "00-install-microk8s.sh"
	upgradeMicrok8sScript           = "00-upgrade-microk8s.sh"
	configureApiServerScript        = "10-configure-apiserver.sh"
	configureCPKubeletScript        = "10-configure-cp-kubelet.sh"
	configureClusterAgentPortScript = "10-configure-cluster-agent-port.sh"
	configureDqlitePortScript       = "10-configure-dqlite-port.sh"
	configureDqliteAddressScript    = "10-configure-dqlite-address.sh"
	configureDNSScript              = "20-microk8s-configure-dns.sh"
	microk8sJoinScript              = "20-microk8s-join.sh"
	microk8sEnableScript            = "20-microk8s-enable.sh"
	microk8sKubeConfigScript        = "20-microk8s-kubeconfig.sh"

	defaultClusterAgentPort  = "25000"
	remappedClusterAgentPort = "30000"
	remappedDqlitePort       = "2379"

	tokenTTL               = 315569260
	ifmicroK8sInstalled    = "[ -d \"/var/snap/microk8s\" ]"
	ifmicrok8sNotInstalled = "[ ! -d \"/var/snap/microk8s\" ]"
)

func main() {
	logFile := "/var/log/agent-provider-microk8s.log"
	f, err := os.OpenFile(logFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		fmt.Println("Failed to create logfile" + logFile)
		logrus.Fatal(err)
	}
	defer f.Close()
	logrus.SetOutput(f)
	logrus.Printf("running provider %v", os.Args)
	plugin := clusterplugin.ClusterPlugin{
		Provider: clusterProvider,
	}

	if err := plugin.Run(); err != nil {
		logrus.Fatal(err)
	}
}
func clusterProvider(cluster clusterplugin.Cluster) yip.YipConfig {
	var stages []yip.Stage
	var microk8sConfig MicroK8sSpec
	token := createMicroK8SToken(cluster.ClusterToken)
	if cluster.Options != "" {
		userOptions, _ := kyaml.YAMLToJSON([]byte(cluster.Options))
		_ = json.Unmarshal(userOptions, &microk8sConfig)
	}
	switch cluster.Role {
	case clusterplugin.RoleInit:
		stages = generateInitStages(cluster, token, microk8sConfig)
	case clusterplugin.RoleControlPlane:
		stages = generateControlPlaneJoinStages(cluster, token, microk8sConfig)
	case clusterplugin.RoleWorker:
		stages = generateWorkerJoinStages(cluster, token, microk8sConfig)
	}
	cfg := yip.YipConfig{
		Name: "MicroK8S Kairos Cluster Provider",
		Stages: map[string][]yip.Stage{
			"boot.before": stages,
		},
	}
	cfgStr, _ := kyaml.Marshal(cfg)
	logrus.Printf("out %s", string(cfgStr))
	return cfg
}

func generateInitStages(cluster clusterplugin.Cluster, token string, userConfig MicroK8sSpec) []yip.Stage {
	var installCommands []string
	var upgradeCommands []string
	installCommands = getBaseInstallCommands(cluster, token, installCommands)
	// figure out endpoint type
	endpointType := "DNS"
	if net.ParseIP(cluster.ControlPlaneHost) != nil {
		endpointType = "IP"
	}
	installCommands = append(installCommands, fmt.Sprintf("%s %q %q", scriptPath(configureApiServerScript), endpointType, cluster.ControlPlaneHost))

	if userConfig.ClusterConfiguration.PortCompatibilityRemap {
		installCommands = append(installCommands, fmt.Sprintf("%s %q", scriptPath(configureClusterAgentPortScript), remappedClusterAgentPort))
		installCommands = append(installCommands, fmt.Sprintf("%s %q", scriptPath(configureDqlitePortScript), remappedDqlitePort))
	}
	if userConfig.ClusterConfiguration.DqliteUseHostIPV4Address {
		installCommands = append(installCommands, scriptPath(configureDqliteAddressScript))
	}

	// add the bootstrap token
	installCommands = append(installCommands, fmt.Sprintf("microk8s add-node --token-ttl %v --token %q", tokenTTL, token))
	installCommands = append(installCommands, scriptPath(configureCPKubeletScript))
	installCommands = append(installCommands, fmt.Sprintf("%s %v %q", scriptPath(configureDNSScript), userConfig.ClusterConfiguration.UseHostDNS, userConfig.ClusterConfiguration.DNS))

	addons := parseAddons(userConfig)
	installCommands = append(installCommands, fmt.Sprintf("%s %s", scriptPath(microk8sEnableScript), strings.Join(addons, " ")))
	writeKubeConfigCommand := fmt.Sprintf("%s %s", scriptPath(microk8sKubeConfigScript), userConfig.ClusterConfiguration.WriteKubeconfig)
	installCommands = append(installCommands, writeKubeConfigCommand)
	upgradeCommands = append(upgradeCommands, scriptPath(upgradeMicrok8sScript))
	upgradeCommands = append(upgradeCommands, writeKubeConfigCommand)

	return []yip.Stage{
		{
			Name:     "Upgrade MicroK8S on control Plane init",
			Commands: upgradeCommands,
			If:       ifmicroK8sInstalled,
		},
		{
			Name:     "Install MicroK8S on control Plane init",
			Commands: installCommands,
			If:       ifmicrok8sNotInstalled,
		},
	}
}

func generateControlPlaneJoinStages(cluster clusterplugin.Cluster, token string, userConfig MicroK8sSpec) []yip.Stage {
	var installCommands []string
	var upgradeCommands []string
	var clusterAgentPort string = defaultClusterAgentPort

	installCommands = getBaseInstallCommands(cluster, token, installCommands)
	// figure out endpoint type
	endpointType := "DNS"
	if net.ParseIP(cluster.ControlPlaneHost) != nil {
		endpointType = "IP"
	}
	installCommands = append(installCommands, fmt.Sprintf("%s %q %q", scriptPath(configureApiServerScript), endpointType, cluster.ControlPlaneHost))

	if userConfig.ClusterConfiguration.PortCompatibilityRemap {
		clusterAgentPort = remappedClusterAgentPort
		installCommands = append(installCommands, fmt.Sprintf("%s %q", scriptPath(configureClusterAgentPortScript), remappedClusterAgentPort))
		installCommands = append(installCommands, fmt.Sprintf("%s %q", scriptPath(configureDqlitePortScript), remappedDqlitePort))
	}
	if userConfig.ClusterConfiguration.DqliteUseHostIPV4Address {
		installCommands = append(installCommands, scriptPath(configureDqliteAddressScript))
	}
	installCommands = append(installCommands, scriptPath(configureCPKubeletScript))
	// add join command
	installCommands = append(installCommands, fmt.Sprintf("%s %q", scriptPath(microk8sJoinScript), fmt.Sprintf("%s:%s/%s", cluster.ControlPlaneHost, clusterAgentPort, token)))
	// add the bootstrap token
	installCommands = append(installCommands, fmt.Sprintf("microk8s add-node --token-ttl %v --token %q", tokenTTL, token))
	installCommands = append(installCommands, fmt.Sprintf("%s %s", scriptPath(microk8sKubeConfigScript), userConfig.ClusterConfiguration.WriteKubeconfig))

	upgradeCommands = append(upgradeCommands, scriptPath(upgradeMicrok8sScript))
	return []yip.Stage{

		{
			Name:     "Upgrade MicroK8S on control Plane Join",
			Commands: upgradeCommands,
			If:       ifmicroK8sInstalled,
		},
		{
			Name:     "Install MicroK8S on control Plane Join",
			Commands: installCommands,
			If:       ifmicrok8sNotInstalled,
		},
	}
}
func generateWorkerJoinStages(cluster clusterplugin.Cluster, token string, userConfig MicroK8sSpec) []yip.Stage {

	var installCommands []string
	var upgradeCommands []string
	var clusterAgentPort string = defaultClusterAgentPort

	installCommands = getBaseInstallCommands(cluster, token, installCommands)
	if userConfig.ClusterConfiguration.PortCompatibilityRemap {
		clusterAgentPort = remappedClusterAgentPort
		installCommands = append(installCommands, fmt.Sprintf("%s %q", scriptPath(configureClusterAgentPortScript), clusterAgentPort))
	}
	// add join string
	installCommands = append(installCommands, fmt.Sprintf("%s %q --worker", scriptPath(microk8sJoinScript), fmt.Sprintf("%s:%s/%s", cluster.ControlPlaneHost, clusterAgentPort, token)))

	upgradeCommands = append(upgradeCommands, scriptPath(upgradeMicrok8sScript))
	return []yip.Stage{

		{
			Name:     "Upgrade MicroK8S on control Plane Join",
			Commands: upgradeCommands,
			If:       ifmicroK8sInstalled,
		},
		{
			Name:     "Install MicroK8S on control Plane Join",
			Commands: installCommands,
			If:       ifmicrok8sNotInstalled,
		},
	}
}

func createMicroK8SToken(token string) string {
	md5 := md5.New()
	_, err := io.WriteString(md5, token)
	if err != nil {
		logrus.Fatal("Unable to create token", err)
	}
	return hex.EncodeToString(md5.Sum(nil))
}
func getBaseInstallCommands(cluster clusterplugin.Cluster, token string, installCommands []string) []string {

	// run the script to install microk8s
	installCommands = append(installCommands, scriptPath(installMicrok8sScript))
	// Add installed sentinel files
	installCommands = append(installCommands, "mkdir -p /usr/local/.microk8s")
	installCommands = append(installCommands, "snap list |grep microk8s |cut -f 3 -d ' ' > /usr/local/.microk8s/installed")

	return installCommands
}
func parseAddons(userConfig MicroK8sSpec) []string {

	addons := make([]string, 0, len(userConfig.InitConfiguration.Addons))
	for _, addon := range userConfig.InitConfiguration.Addons {
		// if dns is enabled by the user, we skip it in the list since we always enable by default
		if strings.Contains(addon, "dns") {
			continue
		}
		addons = append(addons, fmt.Sprintf("%q", addon))
	}
	return addons
}
func scriptPath(scriptName string) string {
	return filepath.Join(scriptBasePath, scriptName)
}
