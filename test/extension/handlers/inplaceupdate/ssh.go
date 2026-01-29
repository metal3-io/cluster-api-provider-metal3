package inplaceupdate

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

const (
	sshTimeoutSeconds       = 60
	minVersionPartsRequired = 2
	sshKeySetupHint         = "Ensure the ssh-key secret is generated via kustomize secretGenerator. Deploy with: export SSH_KEY_PATH=~/.ssh/id_rsa && kubectl apply -k test/extension/config/"
)

// runCommand executes a command on a remote machine via SSH.
// It uses the SSH key from /root/.ssh/id_rsa (mounted via kustomize secretGenerator).
func runCommand(machineIP, user, command string) (string, error) {
	// Use the mounted SSH key path directly
	keyPath := "/root/.ssh/id_rsa"

	// Check if key exists first
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		return "", fmt.Errorf("SSH private key not found at %s. %s", keyPath, sshKeySetupHint)
	}

	privkey, err := os.ReadFile(keyPath) //#nosec G304:gosec
	if err != nil {
		return "", fmt.Errorf("couldn't read private key from %s: %w", keyPath, err)
	}
	signer, err := ssh.ParsePrivateKey(privkey)
	if err != nil {
		return "", fmt.Errorf("couldn't form a signer from ssh key: %w", err)
	}
	cfg := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: func(_ string, _ net.Addr, _ ssh.PublicKey) error { return nil },
		Timeout:         sshTimeoutSeconds * time.Second,
	}
	client, err := ssh.Dial("tcp", machineIP+":22", cfg)
	if err != nil {
		return "", fmt.Errorf("couldn't dial the machine host at %s : %w", machineIP, err)
	}
	defer client.Close()
	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("couldn't open a new session: %w", err)
	}
	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer
	session.Stdout = &stdoutBuf
	session.Stderr = &stderrBuf

	fullCommand := "sudo " + command
	if err := session.Run(fullCommand); err != nil {
		return "", fmt.Errorf("unable to send command %q: %w, stderr: %s", fullCommand, err, stderrBuf.String())
	}

	result := strings.TrimSuffix(stdoutBuf.String(), "\n")
	if stderrBuf.Len() > 0 {
		result += "\n" + strings.TrimSuffix(stderrBuf.String(), "\n")
	}
	return result, nil
}

// upgradeKubernetesInPlace performs an in-place Kubernetes upgrade on the specified machine.
func upgradeKubernetesInPlace(machineIP, user, targetVersion string) error {
	// Strip 'v' prefix if present (e.g., v1.32.0 -> 1.32.0)
	version := strings.TrimPrefix(targetVersion, "v")

	// Extract major.minor version for repository setup (e.g., 1.32.0 -> 1.32)
	versionParts := strings.Split(version, ".")
	if len(versionParts) < minVersionPartsRequired {
		return fmt.Errorf("invalid version format: %s, expected format like '1.32.0'", version)
	}
	majorMinor := versionParts[0] + "." + versionParts[1]

	// Setup Kubernetes repository for the target version
	setupRepoCmd := fmt.Sprintf(`sh -c 'cat <<EOF > /etc/yum.repos.d/kubernetes.repo
[kubernetes]
name=Kubernetes
baseurl=https://pkgs.k8s.io/core:/stable:/v%s/rpm/
enabled=1
gpgcheck=1
gpgkey=https://pkgs.k8s.io/core:/stable:/v%s/rpm/repodata/repomd.xml.key
exclude=kubelet kubeadm kubectl cri-tools kubernetes-cni
EOF'`, majorMinor, majorMinor)

	if _, err := runCommand(machineIP, user, setupRepoCmd); err != nil {
		return fmt.Errorf("failed to configure Kubernetes repository: %w", err)
	}

	// Clean yum cache
	if _, err := runCommand(machineIP, user, "yum clean all"); err != nil {
		return fmt.Errorf("failed to clean yum cache: %w", err)
	}

	// Refresh yum cache
	if _, err := runCommand(machineIP, user, "yum makecache"); err != nil {
		return fmt.Errorf("failed to refresh yum cache: %w", err)
	}

	// Upgrade kubeadm first
	upgradeCmd := fmt.Sprintf("yum install -y kubeadm-%s-* --disableexcludes=kubernetes", version)
	if _, err := runCommand(machineIP, user, upgradeCmd); err != nil {
		return fmt.Errorf("failed to upgrade kubeadm: %w", err)
	}

	// Apply the upgrade to the node (for control plane nodes)
	if _, err := runCommand(machineIP, user, "kubeadm upgrade node"); err != nil {
		return fmt.Errorf("failed to run kubeadm upgrade node: %w", err)
	}

	// Upgrade kubelet and kubectl
	upgradeKubeletCmd := fmt.Sprintf("yum install -y kubelet-%s-* kubectl-%s-* --disableexcludes=kubernetes", version, version)
	if _, err := runCommand(machineIP, user, upgradeKubeletCmd); err != nil {
		return fmt.Errorf("failed to upgrade kubelet/kubectl: %w", err)
	}

	// Reload systemd daemon
	if _, err := runCommand(machineIP, user, "systemctl daemon-reload"); err != nil {
		return fmt.Errorf("failed to reload systemd daemon: %w", err)
	}

	// Restart kubelet to apply the new version
	if _, err := runCommand(machineIP, user, "systemctl restart kubelet"); err != nil {
		return fmt.Errorf("failed to restart kubelet: %w", err)
	}

	return nil
}

// isKubernetesUpgraded checks if the Kubernetes version on the machine matches the target version.
func isKubernetesUpgraded(machineIP, user, targetVersion string) (bool, error) {
	// Get the kubelet version
	getVersionCmd := "kubelet --version"
	output, err := runCommand(machineIP, user, getVersionCmd)
	if err != nil {
		return false, fmt.Errorf("failed to get kubelet version: %w", err)
	}

	// Parse the version from output
	if strings.Contains(output, targetVersion) {
		return true, nil
	}

	return false, nil
}
