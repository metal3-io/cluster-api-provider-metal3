# API and Resource Definitions

This describes a setup where the following Cluster API core components and
Metal3-io components are deployed :

* Cluster API manager
* Cluster API Bootstrap Provider Kubeadm (CABPK) manager
* Cluster API Kubeadm Control Plane manager
* Baremetal Operator (including the Ironic setup)
* Cluster API Provider Metal3 (CAPM3)

## BareMetalHost

The BareMetalHost is an object from
[baremetal-operator](https://github.com/metal3-io/baremetal-operator). Each
CR represents a physical host with BMC credentials, hardware status etc.

## Cluster

A Cluster is a Cluster API core object representing a Kubernetes cluster.

Example cluster:

```yaml
apiVersion: cluster.x-k8s.io/v1alpha3
kind: Cluster
metadata:
  name: cluster
spec:
  clusterNetwork:
    services:
      cidrBlocks: ["10.96.0.0/12"]
    pods:
      cidrBlocks: ["192.168.0.0/18"]
    serviceDomain: "cluster.local"
  infrastructureRef:
    apiVersion: infrastructure.cluster.x-k8s.io/v1alpha4
    kind: Metal3Cluster
    name: m3cluster
  controlPlaneRef:
    kind: KubeadmControlPlane
    apiVersion: controlplane.cluster.x-k8s.io/v1alpha3
    name: m3cluster-controlplane
```

## Metal3Cluster

The metal3Cluster object contains information related to the deployment of
the cluster on Baremetal. It currently has two specification fields :

* **controlPlaneEndpoint**: contains the target cluster API server address and
  port
* **noCloudProvider**: (true/false) Whether the cluster will not be deployed
  with an external cloud provider. If set to true, CAPM3 will patch the target
  cluster node objects to add a providerID. This will allow the CAPI process to
  continue even if the cluster is deployed without cloud provider.

Example metal3cluster :

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha4
kind: Metal3Cluster
metadata:
  name: m3cluster
spec:
 controlPlaneEndpoint:
   host: 192.168.111.249
   port: 6443
 noCloudProvider: true
```

## KubeadmControlPlane

This object contains all information related to the control plane configuration.
It references an **infrastructureTemplate** that must be a
*Metal3MachineTemplate* in this case.

For example:

```yaml
kind: KubeadmControlPlane
apiVersion: controlplane.cluster.x-k8s.io/v1alpha3
metadata:
  name: m3cluster-controlplane
spec:
  replicas: 3
  version: v1.17.0
  infrastructureTemplate:
    kind: Metal3MachineTemplate
    apiVersion: infrastructure.cluster.x-k8s.io/v1alpha4
    name: m3cluster-controlplane
  kubeadmConfigSpec:
    initConfiguration:
      nodeRegistration:
        name: 'host0'
        kubeletExtraArgs:
          cloud-provider: baremetal
    clusterConfiguration:
      apiServer:
        extraArgs:
          cloud-provider: baremetal
      controllerManager:
        extraArgs:
          cloud-provider: baremetal
    joinConfiguration:
      controlPlane: {}
      nodeRegistration:
        name: 'host0'
        kubeletExtraArgs:
          cloud-provider: baremetal
```

## KubeadmConfig

The KubeadmConfig object is for CABPK. It contains the node Kubeadm
configuration and additional commands to run on the node for the setup.

In order to deploy Kubernetes successfully, you need to know the cluster API
address before deployment. However, If you are deploying an HA cluster or if you
are deploying without using static ip addresses, the cluster API server address
is unknown. A solution to go around the problem is to deploy Keepalived.
Keepalived allows you to set up a virtual IP, defined beforehand, and shared
by the nodes. Hence the commands to set up Keepalived have to run before
kubeadm.

Example KubeadmConfig:

```yaml
apiVersion: bootstrap.cluster.x-k8s.io/v1alpha3
kind: KubeadmConfig
metadata:
  name: controlplane-0
spec:
  initConfiguration:
    nodeRegistration:
      name: '{{ ds.meta_data.name }}'
      kubeletExtraArgs:
        node-labels: 'metal3.io/uuid={{ ds.meta_data.uuid }}'
  preKubeadmCommands:
    - apt update -y
    - apt install net-tools -y
    - apt install -y gcc linux-headers-$(uname -r)
    - apt install -y keepalived
    - systemctl start keepalived
    - systemctl enable keepalived
    - >-
      apt install apt-transport-https ca-certificates curl gnupg-agent
      software-properties-common -y
    - curl -fsSL https://download.docker.com/linux/ubuntu/gpg | apt-key add -
    - >-
      add-apt-repository "deb [arch=amd64]
      https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable"
    - apt update -y
    - apt install docker-ce docker-ce-cli containerd.io -y
    - usermod -aG docker ubuntu
    - >-
      curl -s https://packages.cloud.google.com/apt/doc/apt-key.gpg
      | apt-key add -
    - >-
      echo 'deb https://apt.kubernetes.io/ kubernetes-xenial main'
      > /etc/apt/sources.list.d/kubernetes.list
    - apt update
    - apt install -y kubelet kubeadm kubectl
    - systemctl enable --now kubelet
  postKubeadmCommands:
    - mkdir -p /home/ubuntu/.kube
    - cp /etc/kubernetes/admin.conf /home/ubuntu/.kube/config
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
          vrrp_instance VI_1 {
              state MASTER
              interface enp2s0
              virtual_router_id 1
              priority 101
              advert_int 1
              virtual_ipaddress {
                  192.168.111.249
              }
          }
```

## Machine

A Machine is a Cluster API core object representing a Kubernetes node. A machine
has a reference to a KubeadmConfig and a reference to a metal3machine.

Example Machine:

```yaml
apiVersion: cluster.x-k8s.io/v1alpha3
kind: Machine
metadata:
  name: controlplane-0
  labels:
    cluster.x-k8s.io/control-plane: "true"
    cluster.x-k8s.io/cluster-name: "cluster"
spec:
  version: 1.16
  bootstrap:
    configRef:
      apiVersion: bootstrap.cluster.x-k8s.io/v1alpha3
      kind: KubeadmConfig
      name: controlplane-0
  infrastructureRef:
    apiVersion: infrastructure.cluster.x-k8s.io/v1alpha4
    kind: Metal3Machine
    name: controlplane-0
```

## Metal3Machine

The Metal3Machine contains information related to the deployment of the
BareMetalHost such as the image and the host selector. For each machine, there
must be a Metal3Machine.

The fields are :

* **image** -- This includes two sub-fields, `url` and `checksum`, which
  include the URL to the image and the URL to a checksum for that image. These
  fields are required. The image will be used for provisioning of the
  `BareMetalHost` chosen by the `Machine` actuator.

* **userData** -- This includes two sub-fields, `name` and `namespace`, which
  reference a `Secret` that contains base64 encoded user-data to be written to
  a config drive on the provisioned `BareMetalHost`. This field is optional and
  is automatically set by CAPM3 with the userData from the machine object. If
  you want to overwrite the userData, this should be done in the CAPI machine.

* **dataTemplate** -- This includes a reference to a Metal3DataTemplate object
  containing the metadata and network data templates, and includes two fields,
  `name` and `namespace`.

* **metaData** is a reference to a secret containing the metadata rendered from
  the Metal3DataTemplate metadata template object automatically. In case this
  would not be managed by the Metal3DataTemplate controller, if provided by the
  user for example, the ownerreference should be set properly to ensure that the
  secret belongs to the cluster ownerReference tree (see
  https://cluster-api.sigs.k8s.io/clusterctl/provider-contract.html#ownerreferences-chain).

* **networkData** is a reference to a secret containing the network data
  rendered from the Metal3DataTemplate metadata template object automatically.
  In case this would not be managed by the Metal3DataTemplate controller, if
  provided by the user for example, the ownerreference should be set properly to
  ensure that the secret belongs to the cluster ownerReference tree (see
  https://cluster-api.sigs.k8s.io/clusterctl/provider-contract.html#ownerreferences-chain).

* **hostSelector** -- Specify criteria for matching labels on `BareMetalHost`
  objects. This can be used to limit the set of available `BareMetalHost`
  objects chosen for this `Machine`.

### hostSelector Examples

The `hostSelector field has two possible optional sub-fields:

* **matchLabels** -- Key/value pairs of labels that must match exactly.

* **matchExpressions** -- A set of expressions that must evaluate to true for
  the labels on a `BareMetalHost`.

Valid operators include:

* **!** -- Key does not exist. Values ignored.
* **=** -- Key equals specified value. There must only be one
  value specified.
* **==** -- Key equals specified value. There must only be one
  value specified.
* **in** -- Value is a member of a set of possible values
* **!=** -- Key does not equal the specified value. There must
  only be one value specified.
* **notin** -- Value not a member of the specified set of values.
* **exists** -- Key exists. Values ignored.
* **gt** -- Value is greater than the one specified. Value must be
  an integer.
* **lt** -- Value is less than the one specified. Value must be
  an integer.

Example 1: Only consider a `BareMetalHost` with label `key1` set to `value1`.

```yaml
spec:
  providerSpec:
    value:
      hostSelector:
        matchLabels:
          key1: value1
```

Example 2: Only consider `BareMetalHost` with both `key1` set to `value1` AND
`key2` set to `value2`.

```yaml
spec:
  providerSpec:
    value:
      hostSelector:
        matchLabels:
          key1: value1
          key2: value2
```

Example 3: Only consider `BareMetalHost` with `key3` set to either `a`, `b`, or
`c`.

```yaml
spec:
  providerSpec:
    value:
      hostSelector:
        matchExpressions:
          - key: key3
            operator: in
            values: [‘a’, ‘b’, ‘c’]
```

Example 3: Only consider `BareMetalHost` with `key1` set to `value1` AND `key2`
set to `value2` AND `key3` set to either `a`, `b`, or `c`.

```yaml
spec:
  providerSpec:
    value:
      hostSelector:
        matchLabels:
          key1: value1
          key2: value2
        matchExpressions:
          - key: key3
            operator: in
            values: [‘a’, ‘b’, ‘c’]
```

### Metal3Machine example

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha4
kind: Metal3Machine
metadata:
  name: controlplane-0
spec:
  image:
    url: https://cloud-images.ubuntu.com/bionic/current/bionic-server-cloudimg-amd64.img
    checksum: https://cloud-images.ubuntu.com/bionic/current/bionic-server-cloudimg-amd64.img.md5sum
  hostSelector:
    matchLabels:
      key1: value1
    matchExpressions:
      key: key2
      operator: in
      values: {‘abc’, ‘123’, ‘value2’}
  dataTemplate:
    Name: controlplane-metadata
  metaData:
    Name: controlplane-0-metadata-0
  networkData:
    Name: controlplane-0-networkdata-0
```

## MachineDeployment

MachineDeployment is a core Cluster API object that is similar to
deployment for pods. It refers to a KubeadmConfigTemplate and to a
Metal3MachineTemplate.

Example MachineDeployment:

```yaml
apiVersion: cluster.x-k8s.io/v1alpha3
kind: MachineDeployment
metadata:
  name: md-0
  labels:
    cluster.x-k8s.io/cluster-name: cluster
    nodepool: nodepool-0
spec:
  replicas: 1
  selector:
    matchLabels:
      cluster.x-k8s.io/cluster-name: cluster
      nodepool: nodepool-0
  template:
    metadata:
      labels:
        cluster.x-k8s.io/cluster-name: cluster
        nodepool: nodepool-0
    spec:
      version: 1.16
      bootstrap:
        configRef:
          name: md-0
          apiVersion: bootstrap.cluster.x-k8s.io/v1alpha3
          kind: KubeadmConfigTemplate
      infrastructureRef:
        name: md-0
        apiVersion: infrastructure.cluster.x-k8s.io/v1alpha4
        kind: Metal3MachineTemplate
```

## KubeadmConfigTemplate

This contains a template to generate KubeadmConfig.

Example KubeadmConfigTemplate:

```yaml
apiVersion: bootstrap.cluster.x-k8s.io/v1alpha3
kind: KubeadmConfigTemplate
metadata:
  name: md-0
spec:
  template:
    spec:
      joinConfiguration:
        nodeRegistration:
          name: '{{ ds.meta_data.name }}'
          kubeletExtraArgs:
            node-labels: 'metal3.io/uuid={{ ds.meta_data.uuid }}'
      preKubeadmCommands:
        - apt update -y
        - >-
          apt install apt-transport-https ca-certificates curl gnupg-agent
          software-properties-common -y
        - curl -fsSL https://download.docker.com/linux/ubuntu/gpg | apt-key add -
        - >-
          add-apt-repository "deb [arch=amd64]
          https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable"
        - apt update -y
        - apt install docker-ce docker-ce-cli containerd.io -y
        - usermod -aG docker ubuntu
        - >-
          curl -s https://packages.cloud.google.com/apt/doc/apt-key.gpg
          | apt-key add -
        - >-
          echo 'deb https://apt.kubernetes.io/ kubernetes-xenial main'
          > /etc/apt/sources.list.d/kubernetes.list
        - apt update
        - apt install -y kubelet kubeadm kubectl
        - systemctl enable --now kubelet
```

## Metal3MachineTemplate

The Metal3MachineTemplate contains the template to create Metal3Machine.

Example Metal3MachineTemplate :

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha4
kind: Metal3MachineTemplate
metadata:
  name: md-0
spec:
  template:
    spec:
      image:
        url: https://cloud-images.ubuntu.com/bionic/current/bionic-server-cloudimg-amd64.img
        checksum: https://cloud-images.ubuntu.com/bionic/current/bionic-server-cloudimg-amd64.img.md5sum
      hostSelector:
        matchLabels:
          key1: value1
        matchExpressions:
          key: key2
          operator: in
          values: {‘abc’, ‘123’, ‘value2’}
      dataTemplate:
        Name: md-0-metadata
```

## Metal3Metadata

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha4
kind: Metal3DataTemplate
metadata:
  name: controlplane-metadata
spec:
  metaData: |
    node_name: "ctrl-{{ indexWithOffset 1 }}"
    local-hostname: {{ machineName }}
    pool: "pool1"
  networkData: |
    {
        "links": [
            {
                "id": "enp1s0",
                "type": "phy",
                "ethernet_mac_address": "{{ '{{ bareMetalHostMACByName "eth0" }}' }}"
            },
            {
                "id": "enp2s0",
                "type": "phy",
                "ethernet_mac_address": "{{ '{{ bareMetalHostMACByName "eth1" }}' }}"
            }
        ],
        "networks": [
            {
                "id": "Provisioning",
                "type": "ipv4_dhcp",
                "link": "enp1s0"
            },
            {
                "id": "Baremetal",
                "type": "ipv4_dhcp",
                "link": "enp2s0"
            }
        ],
        "services": [
            {
                "type": "dns",
                "address": "8.8.8.8"
            }
        ]
    }
```

The Metal3Metadata object contains a template for the metadata passed to the
BareMetalHost. `spec.metaData` and `spec.networkData` are templates that will
be rendered per node. The controller will create a secret containing the
rendered value for all the metal3machines that are listed as OwnerReference of
the Metal3DataTemplate object.

The `metaData` field should contain a map of strings in yaml format, while
`networkData` should contain a json string that fulfills the requirements of
[Nova network_data.json](https://docs.openstack.org/nova/latest/user/metadata.html#openstack-format-metadata).
The format definition can be found
[here](https://docs.openstack.org/nova/latest/_downloads/9119ca7ac90aa2990e762c08baea3a36/network_data.json).

Each Metal3Machine is given an index number. The index is the lowest integer
that is not given to another Metal3Machine already. When a Metal3Machine is
deleted, its index is released for re-use.

There are multiple functions that can be used in the templates :

- **machineName** : returns the Machine name
- **metal3MachineName** : returns the Metal3Machine name
- **bareMetalHostName**: returns the BareMetalHost name
- **index** : returns the Metal3Machine index for the Metal3Metadata object.
  The index starts from 0.
- **indexWithOffset** : takes an integer as parameter and returns the sum of
  the index and the offset parameter
- **indexWithStep** : takes an integer as parameter and returns the
  multiplication of the index and the step parameter
- **indexWithOffsetAndStep** OR **indexWithStepAndOffset**: takes two
  integers as parameters, order depending on the function name, and returns the
  sum of the offset and the multiplication of the index and the step.
- **index*Hex** : All the `index` function can be suffixed with `Hex` to
  get the same value in hexadecimal format.
- **bareMetalHostMACByName**: Takes a string as parameter and returns the MAC
  address of the nic identified by a name matching the parameter.

In addition, there will be by default a `uuid` key in the metadata with the
BareMetalHost UID as value set by Baremetal Operator.

## Metal3 dev env examples

You can find CR examples in the
[Metal3-io dev env project](https://github.com/metal3-io/metal3-dev-env),
in the [template
folder](https://github.com/metal3-io/metal3-dev-env/tree/master/vm-setup/roles/v1aX_integration_test/templates).
