# Cluster API provider Metal³

This provider integrates with the
[Cluster API project](https://github.com/kubernetes-sigs/cluster-api).

## Setup

### Pre-requisites

The pre-requisite for the deployment of CAPM3 are the following:

- Ironic up and running (inside or outside of the cluster)
- BareMetalHost resources created for all hardware nodes and in "ready" or
  "available" state
- all cluster-related CRs (inc. BareMetalHosts and related) must be in the
  same namespace. This is due to the use of owner references.

### Using clusterctl

Please refer to
[Clusterctl documentation](https://main.cluster-api.sigs.k8s.io/clusterctl/overview.html).
Once the [Pre-requisites](#pre-requisites) are fulfilled, you can follow the
normal clusterctl flow for the `init`, `config`, `upgrade`, `move` and `delete`
workflow. Please refer to the [Pivoting Ironic](#pivoting-or-updating-ironic) section for
additional information on the `move` process.

#### Cluster templates variables

CAPM3 provides a cluster template. This requires some environment variables
properly set. You can find an example file containing the environment variables
`example_variables.rc`in the release or
[here](https://github.com/metal3-io/cluster-api-provider-metal3/tree/main/examples/clusterctl-templates/example_variables.rc).
The examples provided there or below assume that you are deploying the target
node using an off-the-shelf Ubuntu image (18.04), served locally in the
metal3-dev-env. They must be adapted for any deployment.

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

#### NODE_DRAIN_TIMEOUT

This variable sets the nodeDrainTimout for cluster, controlplane and machinedeployment template. Users can set desired value in seconds ("300s") or minutes ("5m"). If it is not set, default value will be "0s" which will not make any change in the current deployment. For example:

`NODE_DRAIN_TIMEOUT="300s"`

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

## Pivoting or updating Ironic

Before running the `move` command of Clusterctl, elements such as Ironic if
applicable, need to be moved to the target cluster. It is recommended to
scale down the Ironic pod in the origin cluster before deploying it on the
target cluster to prevent issues with a duplicated DHCP server.

Both for pivoting or updating Ironic, it is critical that the cluster is in
a stable situation. No operations on BareMetal hosts shall be on-going,
otherwise they might fail. Similarly, in order to prevent conflict during
the pivoting of the DHCP server, we recommend to have no BareMetalHosts
running IPA (in `ready` state with `fasttrack` option enabled) during the
the pivoting. It could otherwise result in IP address conflicts or changes
of the IP address of a running host that would not be supported by Ironic.

In the case of a self-hosted cluster, special care must be paid to Ironic.
Since Ironic runs on the target cluster, updating the target cluster means
that Ironic will need to be moved between nodes of the cluster. This results
in similar issues as pivoting. The following points should be ensured to run
a target cluster upgrade:

- no unnecessary hosts are running IPA during the upgrade to limit the amount
  of conflicts
- When upgrading the K8S node or group of nodes that run Ironic currently, only
  one node at a time should be upgraded, and no other parallel upgrade
  operations should be happening.
- All nodes that are hosting Ironic or have connectivity to the provisioning
  network must be using a static IP address. If not, Ironic might not come
  up since Keepalived will not be starting. In addition, if using DHCP,
  conflicts could happen. We highly recommend that ALL nodes with connectivity
  to the provisioning network are using static IP addresses, using
  Metal3DataTemplates for example.
- Ironic should always be the first component upgraded, before a CAPM3 / BMO
  upgrade using clusterctl for example, or before nodes upgrades. This is to
  ensure that the cluster is in a stable condition while upgrading Ironic.

**Important Note:**
Currently, when target cluster is up and node appears, CAPM3 will fetch the node
and set the providerID value to BMH UUID, meaning that it is not advisable to
directly map the K.Node <---> BMH after pivoting. However, if needed, we can
still find the providerID value in Metal3Machine Spec. which enables us to do
the mapping with an intermediary step, i.e K.Node <--> M3Machine <--> BMH.
