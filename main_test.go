package main

import (
	"strings"
	"testing"

	"github.com/kairos-io/kairos-sdk/clusterplugin"
)

func commandsFor(t *testing.T, cluster clusterplugin.Cluster) []string {
	t.Helper()

	cfg := clusterProvider(cluster)

	var commands []string
	for _, stage := range cfg.Stages["boot.before"] {
		commands = append(commands, stage.Commands...)
	}

	if len(commands) == 0 {
		t.Fatalf("role %q produced no commands", cluster.Role)
	}

	return commands
}

// cluster.config is optional, so every role has to survive a cluster stanza that
// carries nothing but the three documented top level keys.
func TestRolesWithoutConfig(t *testing.T) {
	for _, role := range []string{clusterplugin.RoleInit, clusterplugin.RoleControlPlane, clusterplugin.RoleWorker} {
		t.Run(role, func(t *testing.T) {
			commandsFor(t, clusterplugin.Cluster{
				ClusterToken:     "randomstring",
				ControlPlaneHost: "cluster.example.com",
				Role:             clusterplugin.Role(role),
			})
		})
	}
}

// Every section inside cluster.config is optional too, so one section must not
// require the other.
func TestRolesWithOneConfigSection(t *testing.T) {
	sections := map[string]string{
		"clusterConfiguration only": "clusterConfiguration:\n  writeKubeconfig: \"/run/kubeconfig\"\n",
		"initConfiguration only":    "initConfiguration:\n  addons:\n    - metallb\n",
	}

	for name, options := range sections {
		for _, role := range []string{clusterplugin.RoleInit, clusterplugin.RoleControlPlane, clusterplugin.RoleWorker} {
			t.Run(name+"/"+role, func(t *testing.T) {
				commandsFor(t, clusterplugin.Cluster{
					ClusterToken:     "randomstring",
					ControlPlaneHost: "cluster.example.com",
					Role:             clusterplugin.Role(role),
					Options:          options,
				})
			})
		}
	}
}

// Without a calico section there is no calico command to run, so nothing may be
// handed to the shell as a bare `true` or `false`.
func TestNoCalicoConfigEmitsNoBareBoolean(t *testing.T) {
	for _, role := range []string{clusterplugin.RoleInit, clusterplugin.RoleControlPlane, clusterplugin.RoleWorker} {
		t.Run(role, func(t *testing.T) {
			for _, command := range commandsFor(t, clusterplugin.Cluster{
				ClusterToken:     "randomstring",
				ControlPlaneHost: "cluster.example.com",
				Role:             clusterplugin.Role(role),
			}) {
				if strings.TrimSpace(command) == "true" || strings.TrimSpace(command) == "false" {
					t.Errorf("command %q is a bare boolean", command)
				}
				if strings.Contains(command, configureCalicoScript) {
					t.Errorf("command %q configures calico, which the user did not ask for", command)
				}
			}
		})
	}
}

// A calico section still reaches the script, on every role.
func TestCalicoConfigIsPassedThrough(t *testing.T) {
	options := "clusterConfiguration:\n  calico:\n    calicoIPinIP: true\n    calicoAutoDetect: \"cidr=10.10.128.0/18\"\n"

	for role, want := range map[string]string{
		clusterplugin.RoleInit:         `10-configure-calico.sh true "cidr=10.10.128.0\\/18" true`,
		clusterplugin.RoleControlPlane: `10-configure-calico.sh true "cidr=10.10.128.0\\/18" false`,
		clusterplugin.RoleWorker:       `10-configure-calico.sh true "cidr=10.10.128.0\\/18" false`,
	} {
		t.Run(role, func(t *testing.T) {
			var found bool
			for _, command := range commandsFor(t, clusterplugin.Cluster{
				ClusterToken:     "randomstring",
				ControlPlaneHost: "cluster.example.com",
				Role:             clusterplugin.Role(role),
				Options:          options,
			}) {
				if strings.HasSuffix(command, want) {
					found = true
				}
			}
			if !found {
				t.Errorf("no command ending in %q", want)
			}
		})
	}
}
