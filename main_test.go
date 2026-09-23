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

// joinTokenTTLInSecs is documented as "the join token will expire after the
// specified seconds, defaults to 10 years". It reached no command at all, so
// an operator who shortened the join window still got the ten year token.
func TestJoinTokenTTLIsHonoured(t *testing.T) {
	for _, role := range []string{clusterplugin.RoleInit, clusterplugin.RoleControlPlane} {
		t.Run(role, func(t *testing.T) {
			var found bool
			for _, command := range commandsFor(t, clusterplugin.Cluster{
				ClusterToken:     "randomstring",
				ControlPlaneHost: "cluster.example.com",
				Role:             clusterplugin.Role(role),
				Options:          "initConfiguration:\n  joinTokenTTLInSecs: 3600\n",
			}) {
				if !strings.Contains(command, "microk8s add-node") {
					continue
				}
				found = true
				if !strings.Contains(command, "--token-ttl 3600 ") {
					t.Errorf("add-node command %q does not carry the configured TTL", command)
				}
			}
			if !found {
				t.Error("no add-node command")
			}
		})
	}
}

// Ten years stays the default when the key is absent.
func TestJoinTokenTTLDefault(t *testing.T) {
	var found bool
	for _, command := range commandsFor(t, clusterplugin.Cluster{
		ClusterToken:     "randomstring",
		ControlPlaneHost: "cluster.example.com",
		Role:             clusterplugin.RoleInit,
	}) {
		if strings.Contains(command, "microk8s add-node") {
			found = true
			if !strings.Contains(command, "--token-ttl 315569260 ") {
				t.Errorf("add-node command %q does not carry the default TTL", command)
			}
		}
	}
	if !found {
		t.Error("no add-node command")
	}
}

// httpProxy, httpsProxy and noProxy were parsed and dropped, while
// 10-configure-containerd-proxy.sh shipped in the image with nothing calling
// it, so a node behind a proxy could not pull an image. Every role installs
// microk8s, so every role has to configure the proxy.
func TestProxyConfigReachesTheScript(t *testing.T) {
	options := "initConfiguration:\n" +
		"  httpProxy: \"http://proxy.example.com:3128\"\n" +
		"  httpsProxy: \"https://proxy.example.com:3129\"\n" +
		"  noProxy: \"10.0.0.0/8,.svc\"\n"

	want := `10-configure-containerd-proxy.sh "http://proxy.example.com:3128" "https://proxy.example.com:3129" "10.0.0.0/8,.svc"`

	for _, role := range []string{clusterplugin.RoleInit, clusterplugin.RoleControlPlane, clusterplugin.RoleWorker} {
		t.Run(role, func(t *testing.T) {
			commands := commandsFor(t, clusterplugin.Cluster{
				ClusterToken:     "randomstring",
				ControlPlaneHost: "cluster.example.com",
				Role:             clusterplugin.Role(role),
				Options:          options,
			})

			var at = -1
			for i, command := range commands {
				if strings.HasSuffix(command, want) {
					at = i
				}
			}
			if at < 0 {
				t.Fatalf("no command ending in %q, got %v", want, commands)
			}

			// The script appends to containerd-env and restarts the daemon, so
			// it has to land after the snap is installed and before the first
			// join or image pull.
			for i, command := range commands {
				if strings.Contains(command, installMicrok8sScript) && i > at {
					t.Errorf("proxy configured at %d, before the install at %d", at, i)
				}
				if (strings.Contains(command, microk8sJoinScript) || strings.Contains(command, microk8sEnableScript)) && i < at {
					t.Errorf("%q at %d runs before the proxy is configured at %d", command, i, at)
				}
			}
		})
	}
}

// A single proxy key is enough, and the two the user left out stay empty
// rather than becoming the string the script treats as set.
func TestProxyConfigWithOneKey(t *testing.T) {
	want := `10-configure-containerd-proxy.sh "" "" "10.0.0.0/8"`

	var found bool
	for _, command := range commandsFor(t, clusterplugin.Cluster{
		ClusterToken:     "randomstring",
		ControlPlaneHost: "cluster.example.com",
		Role:             clusterplugin.RoleWorker,
		Options:          "initConfiguration:\n  noProxy: \"10.0.0.0/8\"\n",
	}) {
		if strings.HasSuffix(command, want) {
			found = true
		}
	}
	if !found {
		t.Errorf("no command ending in %q", want)
	}
}

// With no proxy keys the script must not run: it appends a header to
// containerd-env and would restart containerd for nothing.
func TestNoProxyConfigRunsNoScript(t *testing.T) {
	for _, role := range []string{clusterplugin.RoleInit, clusterplugin.RoleControlPlane, clusterplugin.RoleWorker} {
		t.Run(role, func(t *testing.T) {
			for _, command := range commandsFor(t, clusterplugin.Cluster{
				ClusterToken:     "randomstring",
				ControlPlaneHost: "cluster.example.com",
				Role:             clusterplugin.Role(role),
				Options:          "initConfiguration:\n  addons:\n    - metallb\n",
			}) {
				if strings.Contains(command, configureContainerdProxyScript) {
					t.Errorf("command %q configures a proxy the user did not ask for", command)
				}
			}
		})
	}
}
