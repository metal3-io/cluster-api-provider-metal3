#!/usr/bin/env bash

# Ensures Docker is installed and running. If Docker is not found, the latest
# stable version is installed.

set -o errexit
set -o nounset
set -o pipefail

# docker_daemon_ready succeeds if the Docker daemon answers. Talking to the
# socket as a non-root user needs an active 'docker' group membership, which is
# not yet effective in this shell when the user was added to the group by an
# earlier run (that is what the sg re-exec in scripts/ci-e2e.sh is for). Fall
# back to sudo before concluding the daemon is down, otherwise a perfectly
# healthy Docker gets reinstalled underneath a running service.
docker_daemon_ready() {
    docker version &>/dev/null || sudo docker version &>/dev/null
}

# Kernel modules the dockerd bridge driver needs. They are normally autoloaded on
# first use, but loading them up front turns a cryptic daemon start failure into
# a named warning here.
DOCKER_KERNEL_MODULES=(br_netfilter iptable_nat iptable_filter xt_addrtype xt_conntrack overlay)

load_docker_kernel_modules() {
    local mod
    for mod in "${DOCKER_KERNEL_MODULES[@]}"; do
        sudo modprobe "${mod}" &>/dev/null || \
            echo "WARNING: kernel module ${mod} is unavailable on $(uname -r)"
    done
}

dockerd_supports_firewall_backend() {
    local usage
    usage=$(sudo dockerd --help 2>&1 || true)
    [[ "${usage}" == *firewall-backend* ]]
}

# use_nftables_firewall_backend points dockerd at its native nftables firewall
# backend (Docker >= 28). RHEL/CentOS 10 ship an nft-only netfilter stack with no
# xt_addrtype compatibility match, so the default iptables backend cannot install
# its NAT jump rules and the daemon exits with
#   Extension addrtype revision 0 not supported, missing kernel module?
# Returns non-zero when the config is already in place or cannot be written
# safely, i.e. when retrying would be pointless.
use_nftables_firewall_backend() {
    local cfg="/etc/docker/daemon.json"

    # An unknown key in daemon.json makes dockerd refuse to start, so only write
    # it when this dockerd advertises the option.
    if ! dockerd_supports_firewall_backend; then
        echo "WARNING: this dockerd has no firewall-backend option (needs Docker >= 28)." >&2
        echo "         The host kernel must provide xt_addrtype for Docker to work." >&2
        return 1
    fi

    if sudo test -s "${cfg}"; then
        if sudo grep -q '"firewall-backend"' "${cfg}"; then
            echo "firewall-backend is already configured in ${cfg}" >&2
            return 1
        fi
        echo "WARNING: ${cfg} already has content, not modifying it." >&2
        echo "         Add '\"firewall-backend\": \"nftables\"' there manually." >&2
        return 1
    fi

    echo "No xt_addrtype on this host, configuring dockerd to program nftables directly"
    sudo mkdir -p /etc/docker
    printf '{\n  "firewall-backend": "nftables"\n}\n' | sudo tee "${cfg}" >/dev/null
}

# docker_failed_on_addrtype succeeds when the last start attempt died on the
# missing xt_addrtype match rather than on something else. The journal is read
# into a variable instead of piped into grep: `grep -q` exits on the first match
# and the resulting SIGPIPE would fail the pipeline under `set -o pipefail`.
docker_failed_on_addrtype() {
    local journal
    journal=$(sudo journalctl -u docker.service --no-pager -n 100 2>/dev/null || true)
    [[ "${journal}" == *addrtype* ]]
}

# start_docker enables and starts the service. On failure it retries once with
# the nftables firewall backend when that is the diagnosed cause, then dumps the
# service state and journal: `systemctl enable --now` only reports that the job
# failed, which leaves nothing to debug in the CI log.
start_docker() {
    load_docker_kernel_modules

    if sudo systemctl enable --now docker; then
        return 0
    fi

    if docker_failed_on_addrtype && use_nftables_firewall_backend; then
        echo "Retrying docker.service with the nftables firewall backend..."
        if sudo systemctl restart docker; then
            return 0
        fi
    fi

    echo "ERROR: docker.service failed to start, diagnostics follow" >&2
    echo "--- kernel: $(uname -r)" >&2
    sudo modinfo xt_addrtype || true
    sudo systemctl status docker --no-pager --full || true
    sudo journalctl -xeu docker.service --no-pager -n 100 || true
    return 1
}

ensure_docker() {
    if command -v docker &>/dev/null && docker_daemon_ready; then
        echo "Docker is already installed and running: $(docker --version)"
        return 0
    fi

    # Installed but not running: start it instead of reinstalling on top.
    if command -v docker &>/dev/null; then
        echo "Docker is installed but not responding, starting the service..."
        start_docker
        if docker_daemon_ready; then
            echo "Docker is now running: $(docker --version)"
            return 0
        fi
        echo "ERROR: Docker is installed but the daemon is not reachable" >&2
        return 1
    fi

    echo "Installing Docker (latest stable)..."

    if [[ -f /etc/os-release ]]; then
        # shellcheck source=/dev/null
        source /etc/os-release
    fi

    local id="${ID:-}"

    case "${id}" in
        ubuntu|debian)
            # Remove old/conflicting packages
            for pkg in docker.io docker-doc docker-compose podman-docker containerd runc; do
                sudo apt-get remove -y "${pkg}" 2>/dev/null || true
            done

            sudo apt-get update -y
            sudo apt-get install -y ca-certificates curl gnupg

            # Add Docker official GPG key
            sudo install -m 0755 -d /etc/apt/keyrings
            curl -fsSL "https://download.docker.com/linux/${id}/gpg" | \
                sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg
            sudo chmod a+r /etc/apt/keyrings/docker.gpg

            # Add Docker repository
            # shellcheck source=/dev/null
            echo \
                "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] \
                https://download.docker.com/linux/${id} \
                $(. /etc/os-release && echo "${VERSION_CODENAME}") stable" | \
                sudo tee /etc/apt/sources.list.d/docker.list > /dev/null

            sudo apt-get update -y
            sudo apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin
            ;;
        centos|rhel|rocky|almalinux|fedora)
            # Remove old/conflicting packages
            sudo dnf remove -y docker docker-client docker-client-latest \
                docker-common docker-latest docker-latest-logrotate \
                docker-logrotate docker-engine podman-docker 2>/dev/null || true

            sudo dnf install -y dnf-plugins-core

            # For RHEL/CentOS/Rocky/Alma use centos repo; Fedora uses fedora repo
            local repo_id="${id}"
            if [[ "${id}" == "rhel" || "${id}" == "rocky" || "${id}" == "almalinux" ]]; then
                repo_id="centos"
            fi

            sudo dnf config-manager --add-repo \
                "https://download.docker.com/linux/${repo_id}/docker-ce.repo" || \
            sudo dnf config-manager addrepo \
                --from-repofile="https://download.docker.com/linux/${repo_id}/docker-ce.repo"

            sudo dnf install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin
            ;;
        opensuse*|sles|suse)
            sudo zypper --non-interactive install docker docker-buildx
            ;;
        *)
            echo "ERROR: Unsupported OS '${id}' for Docker installation" >&2
            return 1
            ;;
    esac

    # Start and enable Docker
    start_docker

    # Add current user to docker group to allow non-root usage
    if ! groups | grep -q docker; then
        sudo usermod -aG docker "$(whoami)" || true
    fi

    # Verify installation. The group change is not active in this shell yet, so
    # reaching the daemon via sudo is a success too.
    if docker_daemon_ready; then
        echo "Docker installed successfully: $(docker --version)"
    else
        echo "ERROR: Docker installation failed" >&2
        return 1
    fi
}

ensure_docker
