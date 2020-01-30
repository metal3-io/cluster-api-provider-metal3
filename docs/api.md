# API and Resource Definitions

This describes a setup where the following Cluster API core components and
Metal3-io components are deployed :

* Cluster API manager
* Cluster API Bootstrap Provider Kubeadm (CABPK) manager
* Baremetal Operator (including the Ironic setup)
* Cluster API Provider Baremetal (CAPBM)

## BareMetalHost

The BareMetalHost is an object from
[baremetal-operator](https://github.com/metal3-io/baremetal-operator). Each
CR represents a physical host with BMC credentials, hardware status etc.

## Cluster

A Cluster is a Cluster API core object representing a Kubernetes cluster.

Example cluster:

```yaml
apiVersion: cluster.x-k8s.io/v1alpha2
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
    apiVersion: infrastructure.cluster.x-k8s.io/v1alpha2
    kind: BareMetalCluster
    name: bmcluster
```

## BareMetalCluster

The BaremetalCluster object contains information related to the deployment of
the cluster on Baremetal. It currently has two specification fields :

* **apiEndpoint**: contains the target cluster API server address and port in
  URL format
* **noCloudProvider**: (true/false) Whether the cluster will not be deployed
  with an external cloud provider. If set to true, CAPBM will patch the target
  cluster node objects to add a providerID. This will allow the CAPI process to
  continue even if the cluster is deployed without cloud provider.

Example baremetalcluster :

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha2
kind: BareMetalCluster
metadata:
  name: bmcluster
spec:
 apiEndpoint: http://192.168.111.249:6443
 noCloudProvider: true
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
apiVersion: bootstrap.cluster.x-k8s.io/v1alpha2
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
has a reference to a KubeadmConfig and a reference to a BaremetalMachine.

Example Machine:

```yaml
apiVersion: cluster.x-k8s.io/v1alpha2
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
      apiVersion: bootstrap.cluster.x-k8s.io/v1alpha2
      kind: KubeadmConfig
      name: controlplane-0
  infrastructureRef:
    apiVersion: infrastructure.cluster.x-k8s.io/v1alpha2
    kind: BareMetalMachine
    name: controlplane-0
```

## BareMetalMachine

The BareMetalMachine contains information related to the deployment of the
BareMetalHost such as the image and the host selector. For each machine, there
must be a BareMetalMachine.

The fields are :

* **image** -- This includes two sub-fields, `url` and `checksum`, which
  include the URL to the image and the URL to a checksum for that image. These
  fields are required. The image will be used for provisioning of the
  `BareMetalHost` chosen by the `Machine` actuator.

* **userData** -- This includes two sub-fields, `name` and `namespace`, which
  reference a `Secret` that contains base64 encoded user-data to be written to
  a config drive on the provisioned `BareMetalHost`. This field is optional and
  is automatically set by CAPBM with the userData from the machine object. If
  you want to overwrite the userData, this should be done in the CAPI machine.

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

### BareMetalMachine example

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha2
kind: BareMetalMachine
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
```

## MachineDeployment

MachineDeployment is a core Cluster API object that is similar to
deployment for pods. It refers to a KubeadmConfigTemplate and to a
BareMetalMachineTemplate.

Example MachineDeployment:

```yaml
apiVersion: cluster.x-k8s.io/v1alpha2
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
          apiVersion: bootstrap.cluster.x-k8s.io/v1alpha2
          kind: KubeadmConfigTemplate
      infrastructureRef:
        name: md-0
        apiVersion: infrastructure.cluster.x-k8s.io/v1alpha2
        kind: BareMetalMachineTemplate
```

## KubeadmConfigTemplate

This contains a template to generate KubeadmConfig.

Example KubeadmConfigTemplate:

```yaml
apiVersion: bootstrap.cluster.x-k8s.io/v1alpha2
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

## BareMetalMachineTemplate

The BareMetalMachineTemplate contains the template to create BareMetalMachine.

Example BareMetalMachineTemplate :

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha2
kind: BareMetalMachineTemplate
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
```

## Metal3 dev env examples

You can find CR examples in the
[Metal3-io dev env project](https://github.com/metal3-io/metal3-dev-env),
in the crs/v1alpha2
[folder](https://github.com/metal3-io/metal3-dev-env/tree/master/crs/v1alpha2).
