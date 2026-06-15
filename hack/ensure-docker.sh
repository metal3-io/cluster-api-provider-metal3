#!/usr/bin/env bash

# Ensures Docker is installed and running. If Docker is not found, the latest
# stable version is installed.

set -o errexit
set -o nounset
set -o pipefail

ensure_docker() {
    if command -v docker &>/dev/null && docker version &>/dev/null; then
        echo "Docker is already installed and running: $(docker --version)"
        return 0
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
    sudo systemctl enable --now docker

    # Add current user to docker group to allow non-root usage
    if ! groups | grep -q docker; then
        sudo usermod -aG docker "$(whoami)" || true
    fi

    # Verify installation
    if docker version &>/dev/null; then
        echo "Docker installed successfully: $(docker --version)"
    else
        # Try with sudo as group change may not be active yet
        if sudo docker version &>/dev/null; then
            echo "Docker installed successfully (requires sudo): $(sudo docker --version)"
        else
            echo "ERROR: Docker installation failed" >&2
            return 1
        fi
    fi
}

ensure_docker
