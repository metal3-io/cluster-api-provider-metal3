# Cluster API provider Bare Metal

This provider integrates with the
[Cluster API project](https://github.com/kubernetes-sigs/cluster-api).

## Setup

### Pre-requisites

The pre-requisite for the deployment of CAPM3 are the following:

- [Baremetal-Operator](https://github.com/metal3-io/baremetal-operator) deployed
- Ironic up and running (inside or outside of the cluster)
- BareMetalHost resources created for all hardware nodes and in "ready" or
  "available" state
- If deploying CAPI in a multi-tenancy scenario, all cluster-related CRs (inc.
  BareMetalHosts and related) must be in the same namespace. This is due to the
  fact that the controllers are restricted to their own namespace with RBAC.

### Using clusterctl

Please refer to
[Clusterctl documentation](https://master.cluster-api.sigs.k8s.io/clusterctl/overview.html).
Once the Pre-requisites are fulfilled, you can follow the normal clusterctl
flow for the `init`, `config`, `upgrade` and `delete` workflow. The `move`
command is supported only if the baremetalhosts are moved independently first,
and their status is preserved during the move. Please refer to the *Pivoting
Ironic* section.

### Cluster templates variables

You can find an example file containing the environment variables
`example_variables.rc`in the release or
[here](https://github.com/metal3-io/cluster-api-provider-metal3/tree/master/examples/clusterctl-templates/example_variables.rc)

#### POD_CIDR

This is the CIDR for the pod. It can be given as a comma separated list of
quoted elements. For example:

`POD_CIDR='"192.168.0.0/24", "192.168.1.0/24"'`

#### SERVICE_CIDR

This is the CIDR for the services. It can be given as a comma separated list of
quoted elements. For example:

`SERVICE_CIDR='"192.168.2.0/24", "192.168.3.0/24"'`

#### API_ENDPOINT_HOST

This is the API endpoint name or IP address. For example:

`API_ENDPOINT_HOST="192.168.111.249"`

#### API_ENDPOINT_PORT

This is the API endpoint port. For example:

`API_ENDPOINT_PORT="6443"`

#### IMAGE_URL

This is the URL of the image to deploy. It should be a qcow2 image. For example:

`IMAGE_URL="http://192.168.0.1/ubuntu.qcow2"`

#### IMAGE_CHECKSUM

This is the URL of the md5sum of the image to deploy. For example:

`IMAGE_CHECKSUM="http://192.168.0.1/ubuntu.qcow2.md5sum"`

#### CTLPLANE_KUBEADM_EXTRA_CONFIG

This contains the extra configuration to pass in KubeadmControlPlane. It is
critical to maintain the indentation. The allowed keys are :

- preKubeadmCommands
- postKubeadmCommands
- files
- users
- ntp
- format

Here is an example for Ubuntu:

```bash
CTLPLANE_KUBEADM_EXTRA_CONFIG="
    preKubeadmCommands:
      - ip link set dev enp2s0 up
      - dhclient enp2s0
      - apt update -y
      - netplan apply
      - >-
        apt install net-tools gcc linux-headers-$(uname -r) bridge-utils
        apt-transport-https ca-certificates curl gnupg-agent
        software-properties-common -y
      - apt install -y keepalived && systemctl stop keepalived
      - curl -fsSL https://download.docker.com/linux/ubuntu/gpg | apt-key add -
      - add-apt-repository \"deb [arch=amd64] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable\"
      - curl -s https://packages.cloud.google.com/apt/doc/apt-key.gpg | apt-key add -
      - echo 'deb https://apt.kubernetes.io/ kubernetes-xenial main' > /etc/apt/sources.list.d/kubernetes.list
      - apt update -y
      - apt install docker-ce docker-ce-cli containerd.io kubelet kubeadm kubectl -y
      - systemctl enable --now docker kubelet
      - if (curl -sk --max-time 10 https://{{ CLUSTER_APIENDPOINT_HOST }}:6443/healthz); then echo \"keepalived already running\";else systemctl start keepalived; fi
      - usermod -aG docker ubuntu
    postKubeadmCommands:
      - mkdir -p /home/ubuntu/.kube
      - cp /etc/kubernetes/admin.conf /home/ubuntu/.kube/config
      - systemctl enable --now keepalived
      - chown ubuntu:ubuntu /home/ubuntu/.kube/config
    files:
        - path: /etc/keepalived/keepalived.conf
          content: |
            ! Configuration File for keepalived
            global_defs {
                notification_email {
                sysadmin@example.com
                support@example.com
                }
                notification_email_from lb@example.com
                smtp_server localhost
                smtp_connect_timeout 30
            }
            vrrp_instance VI_2 {
                state MASTER
                interface enp2s0
                virtual_router_id 2
                priority 101
                advert_int 1
                virtual_ipaddress {
                    {{ CLUSTER_APIENDPOINT_HOST }}
                }
            }
        - path: /etc/netplan/50-cloud-init.yaml
          owner: root:root
          permissions: '0644'
          content: |
            network:
                ethernets:
                    enp2s0:
                        dhcp4: true
                version: 2
        - path : /etc/netplan/60-ironicendpoint.yaml
          owner: root:root
          permissions: '0644'
          content: |
            network:
              version: 2
              renderer: networkd
              bridges:
                ironicendpoint:
                  interfaces: [enp1s0]
                  dhcp4: yes
"
```

#### WORKERS_KUBEADM_EXTRA_CONFIG

This contains the extra configuration to pass in KubeadmConfig for workers. It
is critical to maintain the indentation. The allowed keys are :

- preKubeadmCommands
- postKubeadmCommands
- files
- users
- ntp
- format

Here is an example for Ubuntu:

```bash
WORKERS_KUBEADM_EXTRA_CONFIG="
      preKubeadmCommands:
        - ip link set dev enp2s0 up
        - dhclient enp2s0
        - apt update -y
        - netplan apply
        - >-
          apt install apt-transport-https ca-certificates
          curl gnupg-agent software-properties-common -y
        - curl -fsSL https://download.docker.com/linux/ubuntu/gpg | apt-key add -
        - add-apt-repository \"deb [arch=amd64] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable\"
        - curl -s https://packages.cloud.google.com/apt/doc/apt-key.gpg | apt-key add -
        - echo 'deb https://apt.kubernetes.io/ kubernetes-xenial main' > /etc/apt/sources.list.d/kubernetes.list
        - apt update -y
        - apt install docker-ce docker-ce-cli containerd.io kubelet kubeadm kubectl -y
        - systemctl enable --now docker kubelet
        - usermod -aG docker ubuntu
      files:
        - path: /etc/netplan/50-cloud-init.yaml
          owner: root:root
          permissions: '0644'
          content: |
            network:
                ethernets:
                    enp1s0:
                        dhcp4: true
                    enp2s0:
                        dhcp4: true
                version: 2
"
```

## Pivoting Ironic

Before running the move command of Clusterctl, elements such as Baremetal
Operator, Ironic if applicable, and the BareMetalHost CRs need to be moved to
the target cluster. During the move, the BareMetalHost object must be annotated
with `baremetalhost.metal3.io/paused` key. The value does not matter. The
presence of this annotation will stop the reconciliation loop for that object.

In addition, it is critical that the move of the BMHs does not lead to a lost
status for those objects. If the status is lost, BMO will register the nodes
as available, and introspect them again.

More information TBA on move commands.
