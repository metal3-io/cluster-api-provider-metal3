package e2e

// This file groups the helpers that configure the kind management-cluster node's
// networking for the virtual bare metal lab: the provisioning macvlan interface
// (that Ironic/keepalived binds the provisioning VIP to) and the external subnet
// address (that lets CAPI controllers reach the workload API server VIP). The
// topology is subtle, so it lives here rather than cluttering common.go.

import (
	"context"
	"strings"

	. "github.com/onsi/gomega"
	testexec "sigs.k8s.io/cluster-api/test/framework/exec"
)

// ConfigureProvisioningNetwork creates a macvlan interface (named per provisioningInterface,
// e.g. "ironicendpoint") on top of eth0 in the kind cluster node and assigns the provisioning
// IP with /24 netmask to it. The interface is required by the IRSO keepalived container.
// macvlan in bridge mode gives the interface direct L2 connectivity to the provisioning
// bridge (via eth0 → kind-bridge → veth → provisioning bridge), so the kernel-created connected
// route (172.22.0.0/24 dev ironicendpoint) correctly delivers traffic to provisioning peers.
// The /24 netmask allows dnsmasq to match the DHCP range to the local subnet.
// See https://github.com/metal3-io/baremetal-operator/issues/2792
func ConfigureProvisioningNetwork(ctx context.Context, clusterName string, provisioningIP string, provisioningInterface string) {
	containerName := clusterName + "-control-plane"
	ipWithCIDR := provisioningIP + "/24"

	Logf("Configuring provisioning network: creating %s with %s on %s", provisioningInterface, ipWithCIDR, containerName)

	// Create a macvlan interface on top of eth0. This gives keepalived a named interface
	// to bind to. macvlan in bridge mode provides direct L2 connectivity to all peers on
	// the provisioning bridge, so the kernel's connected route works correctly without
	// any manual override.
	cmd := testexec.NewCommand(
		testexec.WithCommand("docker"),
		testexec.WithArgs("exec", containerName, "ip", "link", "add", provisioningInterface, "link", "eth0", "type", "macvlan", "mode", "bridge"),
	)
	stdout, stderr, err := cmd.Run(ctx)
	if err != nil && !strings.Contains(string(stderr), "File exists") {
		Expect(err).ToNot(HaveOccurred(), "failed to create %s interface (stdout=%q, stderr=%q)", provisioningInterface, string(stdout), string(stderr))
	}

	// Bring the interface up
	cmd = testexec.NewCommand(
		testexec.WithCommand("docker"),
		testexec.WithArgs("exec", containerName, "ip", "link", "set", provisioningInterface, "up"),
	)
	stdout, stderr, err = cmd.Run(ctx)
	if err != nil {
		Expect(err).ToNot(HaveOccurred(), "failed to bring up %s (stdout=%q, stderr=%q)", provisioningInterface, string(stdout), string(stderr))
	}

	// Add the provisioning IP with /24 netmask
	cmd = testexec.NewCommand(
		testexec.WithCommand("docker"),
		testexec.WithArgs("exec", containerName, "ip", "addr", "add", ipWithCIDR, "dev", provisioningInterface),
	)
	stdout, stderr, err = cmd.Run(ctx)
	if err != nil && !strings.Contains(string(stderr), "File exists") {
		Expect(err).ToNot(HaveOccurred(), "failed to add provisioning IP to %s (stdout=%q, stderr=%q)", provisioningInterface, string(stdout), string(stderr))
	}
	Logf("Provisioning network configured successfully")
}

// RemoveProvisioningNetwork removes the provisioning macvlan interface from the kind
// cluster node. This must be called when Ironic moves to a different host (e.g. management
// cluster VM) to avoid an IP conflict where the kind node still responds to the VIP.
func RemoveProvisioningNetwork(ctx context.Context, clusterName string, provisioningInterface string) {
	containerName := clusterName + "-control-plane"

	Logf("Removing provisioning network: deleting %s from %s", provisioningInterface, containerName)

	cmd := testexec.NewCommand(
		testexec.WithCommand("docker"),
		testexec.WithArgs("exec", containerName, "ip", "link", "del", provisioningInterface),
	)

	stdout, stderr, err := cmd.Run(ctx)
	// Ignore "Cannot find device" - the interface may already be removed
	if err != nil && !strings.Contains(string(stderr), "Cannot find") {
		Logf("Warning: failed to remove %s: %v\nstdout: %s\nstderr: %s", provisioningInterface, err, string(stdout), string(stderr))
	} else {
		Logf("Provisioning network removed successfully")
	}

	// Flush stale ARP entries
	flushCmd := testexec.NewCommand(
		testexec.WithCommand("docker"),
		testexec.WithArgs("exec", containerName, "ip", "neigh", "flush", "all"),
	)
	flushCmd.Run(ctx) //nolint:errcheck
}

// ConfigureExternalNetwork adds the external subnet IP with /24 netmask to the kind cluster node.
// This allows CAPI controllers running in the kind cluster to reach the target cluster's
// API server VIP (e.g. 192.168.111.249) via the veth pair connecting kind-bridge to the
// external bridge.
func ConfigureExternalNetwork(ctx context.Context, clusterName string, externalIP string) {
	containerName := clusterName + "-control-plane"
	ipWithCIDR := externalIP + "/24"

	Logf("Configuring external network: adding %s to %s", ipWithCIDR, containerName)

	cmd := testexec.NewCommand(
		testexec.WithCommand("docker"),
		testexec.WithArgs("exec", containerName, "ip", "addr", "add", ipWithCIDR, "dev", "eth0"),
	)

	stdout, stderr, err := cmd.Run(ctx)

	if err != nil && !strings.Contains(string(stderr), "File exists") {
		Expect(err).ToNot(HaveOccurred(), "failed to configure external network (stdout=%q, stderr=%q)", string(stdout), string(stderr))
	}
	Logf("External network configured successfully")
}
