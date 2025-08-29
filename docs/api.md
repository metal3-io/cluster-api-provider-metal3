# API and Resource Definitions

This describes a setup where the following Cluster API core components and
Metal3-io components are deployed :

- Cluster API manager
- Cluster API Bootstrap Provider Kubeadm (CABPK) manager
- Cluster API Kubeadm Control Plane manager
- Baremetal Operator (including the Ironic setup)
- Cluster API Provider Metal3 (CAPM3)

## BareMetalHost

The BareMetalHost is an object from
[baremetal-operator](https://github.com/metal3-io/baremetal-operator). Each CR
represents a physical host with BMC credentials, hardware status etc.

BareMetalHost exposes those different fields that are secret references:

- **userData** : for a cloud-init user-data in a secret with the key `userData`
- **metaData** : for a cloud-init metadata in a secret with the key `metaData`
- **networkData** : for a cloud-init network data in a secret with the key
  `networkData`

For the metaData, some values are set by default to maintain compatibility:

- **uuid**: This is the BareMetalHost UID
- **metal3-namespace**: the name of the BareMetalHost
- **metal3-name**: The name of the BareMetalHost
- **local-hostname**: The name of the BareMetalHost
- **local_hostname**: The namespace of the BareMetalHost

However, setting any of those values in the metaData secret will override those
default values.

### Unhealthy annotation

In CAPM3 API version v1alpha4 (which is removed from main branch but exist in
previous release branches) and onwards, it is possible to mark BareMetalHost
object as unhealthy by adding an annotation `capi.metal3.io/unhealthy`. This
annotation prevents CAPM3 to select unhealthy BareMetalHost for newly created
metal3machine. Removing the annotation will enable the normal operations.

## Cluster

A Cluster is a Cluster API core object representing a Kubernetes cluster.

Example cluster:

```yaml
apiVersion: cluster.x-k8s.io/v1beta2
kind: Cluster
metadata:
  name: cluster
  namespace: metal3
spec:
  clusterNetwork:
    services:
      cidrBlocks:
      - 10.96.0.0/12
    pods:
      cidrBlocks:
      - 192.168.0.0/18
  infrastructureRef:
    apiGroup: infrastructure.cluster.x-k8s.io
    kind: Metal3Cluster
    name: m3cluster
  controlPlaneRef:
    kind: KubeadmControlPlane
    apiGroup: controlplane.cluster.x-k8s.io
    name: m3cluster-controlplane
```

## Metal3Cluster

The metal3Cluster object contains information related to the deployment of the
cluster on Baremetal. It currently has two specification fields :

- **controlPlaneEndpoint**: contains the target cluster API server address and
  port
- **noCloudProvider(Deprecated use CloudProviderEnabled)**: (true/false) Whether
  the cluster will not be deployed with an external cloud provider. If set to
  true, CAPM3 will patch the target cluster node objects to add a providerID.
  This will allow the CAPI process to continue even if the cluster is deployed
  without cloud provider.
- **CloudProviderEnabled**: (true/false) Whether the cluster will be deployed
  with an external cloud provider. If set to false, CAPM3 will patch the target
  cluster node objects to add a providerID. This will allow the CAPI process to
  continue even if the cluster is deployed without cloud provider.

Example metal3cluster :

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: Metal3Cluster
metadata:
  name: m3cluster
  namespace: metal3
spec:
  controlPlaneEndpoint:
    host: 192.168.111.249
    port: 6443
  cloudProviderEnabled: false
```

## KubeadmControlPlane

This object contains all information related to the control plane configuration.
It references an **infrastructureTemplate** that must be a
_Metal3MachineTemplate_ in this case.

For example:

```yaml
kind: KubeadmControlPlane
apiVersion: controlplane.cluster.x-k8s.io/v1beta2
metadata:
  name: m3cluster-controlplane
  namespace: metal3
spec:
  machineTemplate:
    spec:
      infrastructureRef:
        apiGroup: infrastructure.cluster.x-k8s.io
        kind: Metal3MachineTemplate
        name: m3cluster-controlplane
      deletion:
        nodeDrainTimeoutSeconds: 0
  replicas: 3
  rollout:
    strategy:
      rollingUpdate:
        maxSurge: 1
      type: RollingUpdate
  version: v1.34.0
  kubeadmConfigSpec:
    joinConfiguration:
      controlPlane: {}
      nodeRegistration:
        name: "{{ ds.meta_data.name }}"
        kubeletExtraArgs:
        - name: node-labels
          value: 'metal3.io/uuid={{ ds.meta_data.uuid }}'
    initConfiguration:
      nodeRegistration:
        name: "{{ ds.meta_data.name }}"
        kubeletExtraArgs:
        - name: node-labels
          value: 'metal3.io/uuid={{ ds.meta_data.uuid }}'
```

## KubeadmConfig

The KubeadmConfig object is for CABPK. It contains the node Kubeadm
configuration and additional commands to run on the node for the setup.

In order to deploy Kubernetes successfully, you need to know the cluster API
address before deployment. However, if you are deploying an HA cluster or if you
are deploying without using static ip addresses, the cluster API server address
is unknown. A solution to go around the problem is to deploy Keepalived.
Keepalived allows you to set up a virtual IP, defined beforehand, and shared by
the nodes. Hence the commands to set up Keepalived have to run before kubeadm.

The content of a KubeadmConfig can contain Jinja2 template elements, since the
cloud-init renders the cloud-config as a Jinja2 template. It is possible to use
metadata from cloud-init, using the following: `{{ ds.meta_data.<key>}}`. The
keys and values are passed to cloud-init through a `Metal3DataTemplate` object
(see below).

Example KubeadmConfig:

```yaml
apiVersion: bootstrap.cluster.x-k8s.io/v1beta2
kind: KubeadmConfig
metadata:
  name: controlplane-0
  namespace: metal3
spec:
  initConfiguration:
    nodeRegistration:
      name: "{{ ds.meta_data.name }}"
      kubeletExtraArgs:
      - name: node-labels
        value: 'metal3.io/uuid={{ ds.meta_data.uuid }}'
  preKubeadmCommands:
    - netplan apply
    - systemctl enable --now crio kubelet
    - if (curl -sk --max-time 10 https://192.168.111.249:6443/healthz); then
      echo "keepalived already running";else systemctl start keepalived; fi
    - systemctl link /lib/systemd/system/monitor.keepalived.service
    - systemctl enable monitor.keepalived.service
    - systemctl start monitor.keepalived.service
  postKubeadmCommands:
    - mkdir -p /home/metal3/.kube
    - chown metal3:metal3 /home/metal3/.kube
    - cp /etc/kubernetes/admin.conf /home/metal3/.kube/config
    - systemctl enable --now keepalived
    - chown metal3:metal3 /home/metal3/.kube/config
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
                192.168.111.249
            }
        }
```

## Machine

A Machine is a Cluster API core object representing a Kubernetes node. A machine
has a reference to a KubeadmConfig and a reference to a metal3machine.

Example Machine:

```yaml
apiVersion: cluster.x-k8s.io/v1beta2
kind: Machine
metadata:
  name: controlplane-0
  namespace: metal3
  labels:
    cluster.x-k8s.io/control-plane: "true"
    cluster.x-k8s.io/cluster-name: "cluster"
spec:
  bootstrap:
    configRef:
      apiGroup: bootstrap.cluster.x-k8s.io
      kind: KubeadmConfig
      name: controlplane-0
  infrastructureRef:
    apiGroup: infrastructure.cluster.x-k8s.io
    kind: Metal3Machine
    name: controlplane-0
  deletion:
    nodeDrainTimeoutSeconds: 0
  providerID: metal3://68be298f-ed11-439e-9d51-6c5260faede6
  version: v1.34.0
```

## Metal3Machine

The Metal3Machine contains information related to the deployment of the
BareMetalHost such as the image and the host selector. For each machine, there
must be a Metal3Machine.

The fields are:

- **image** -- This includes two sub-fields, `url` and `checksum`, which include
  the URL to the image and the URL to a checksum for that image. These fields
  are required. The image will be used for provisioning of the `BareMetalHost`
  chosen by the `Machine` actuator.

- **userData** -- This includes two sub-fields, `name` and `namespace`, which
  reference a `Secret` that contains base64 encoded user-data to be written to a
  config drive on the provisioned `BareMetalHost`. This field is optional and is
  automatically set by CAPM3 with the userData from the machine object. If you
  want to overwrite the userData, this should be done in the CAPI machine.

- **dataTemplate** -- This includes a reference to a Metal3DataTemplate object
  containing the metadata and network data templates, and includes two fields,
  `name` and `namespace`.

- **metaData** is a reference to a secret containing the metadata rendered from
  the Metal3DataTemplate metadata template object automatically. In case this
  would not be managed by the Metal3DataTemplate controller, if provided by the
  user for example, the ownerreference should be set properly to ensure that the
  secret belongs to the cluster ownerReference tree (see
  [doc](https://cluster-api.sigs.k8s.io/developer/providers/contracts/clusterctl.html#ownerreferences-chain)).

- **networkData** is a reference to a secret containing the network data
  rendered from the Metal3DataTemplate metadata template object automatically.
  In case this would not be managed by the Metal3DataTemplate controller, if
  provided by the user for example, the ownerreference should be set properly to
  ensure that the secret belongs to the cluster ownerReference tree (see
  [doc](https://cluster-api.sigs.k8s.io/developer/providers/contracts/clusterctl.html#ownerreferences-chain)).
  The content of the secret should be a yaml equivalent of a json object that
  follows the format definition that can be found
  [here](https://docs.openstack.org/nova/latest/_downloads/9119ca7ac90aa2990e762c08baea3a36/network_data.json).

- **hostSelector** -- Specify criteria for matching labels on `BareMetalHost`
  objects. This can be used to limit the set of available `BareMetalHost`
  objects chosen for this `Machine`.

- **automatedCleaningMode** -- An interface to enable or disable Ironic
  automated cleaning during provisioning or deprovisioning of a host. When set
  to `disabled`, automated cleaning will be skipped, where `metadata` value
  enables it. It is recommended to tune the cleaning via metal3MachineTemplate
  rather than metal3Machine. When `spec.template.spec.automatedCleaningMode`
  field of metal3MachineTemplate is updated, metal3MachineTemplate controller
  will update all the metal3Machines (generated from the metal3MachineTemplate)
  and eventually BareMetalHosts with the same value.

The `metaData` and `networkData` field in the `spec` section are for the user to
give directly a secret to use as metaData or networkData. The `userData`,
`metaData` and `networkData` fields in the `status` section are for the
controller to store the reference to the secret that is actually being used,
whether it is from one of the spec fields, or somehow generated. This is aimed
at making a clear difference between the desired state from the user (whether it
is with a DataTemplate reference, or direct `metaData` or `userData` secrets)
and what the controller is actually using.

The `dataTemplate` field consists of an object reference to a Metal3DataTemplate
object containing the templates for the metadata and network data generation for
this Metal3Machine. The `renderedData` field is a reference to the Metal3Data
object created for this machine. If the dataTemplate field is set but either the
`renderedData`, `metaData` or `networkData` fields in the status are unset, then
the Metal3Machine controller will wait until it can find the Metal3Data object
and the rendered secrets. It will then populate those fields.

When CAPM3 controller will set the different fields in the BareMetalHost, it
will reference the metadata secret and the network data secret in the
BareMetalHost. If any of the `metaData` or `networkData` status fields are
unset, that field will also remain unset on the BareMetalHost.

When the Metal3Machine gets deleted, the CAPM3 controller will remove its
ownerreference from the data template object. This will trigger the deletion of
the generated Metal3Data object and the secrets generated for this machine.

### hostSelector Examples

The `hostSelector field has two possible optional sub-fields:

- **matchLabels** -- Key/value pairs of labels that must match exactly.

- **matchExpressions** -- A set of expressions that must evaluate to true for
  the labels on a `BareMetalHost`.

Valid operators include:

- **!** -- Key does not exist. Values ignored.
- **=** -- Key equals specified value. There must only be one value specified.
- **==** -- Key equals specified value. There must only be one value specified.
- **in** -- Value is a member of a set of possible values
- **!=** -- Key does not equal the specified value. There must only be one value
  specified.
- **notin** -- Value not a member of the specified set of values.
- **exists** -- Key exists. Values ignored.
- **gt** -- Value is greater than the one specified. Value must be an integer.
- **lt** -- Value is less than the one specified. Value must be an integer.

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
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: Metal3Machine
metadata:
  name: controlplane-0
  namespace: metal3
spec:
  automatedCleaningMode: metadata
  image:
    checksum: http://172.22.0.1/images/UBUNTU_24.04_NODE_IMAGE_K8S_v1.34.0-raw.img.sha256sum
    checksumType: sha256
    format: raw
    url: http://172.22.0.1/images/UBUNTU_24.04_NODE_IMAGE_K8S_v1.34.0-raw.img
  hostSelector:
    matchLabels:
      key1: value1
    matchExpressions:
      key: key2
      operator: in
      values: { ‘abc’, ‘123’, ‘value2’ }
  dataTemplate:
    Name: controlplane-metadata
  metaData:
    Name: controlplane-0-metadata-0
```

## MachineDeployment

MachineDeployment is a core Cluster API object that is similar to deployment for
pods. It refers to a KubeadmConfigTemplate and to a Metal3MachineTemplate.

Example MachineDeployment:

```yaml
apiVersion: cluster.x-k8s.io/v1beta2
kind: MachineDeployment
metadata:
  name: md-0
  namespace: metal3
  labels:
    cluster.x-k8s.io/cluster-name: cluster
    nodepool: nodepool-0
spec:
  clusterName: cluster
  replicas: 1
  selector:
    matchLabels:
      cluster.x-k8s.io/cluster-name: cluster
      nodepool: nodepool-0
  rollout:
    strategy:
      type: RollingUpdate
      rollingUpdate:
        maxSurge: 1
        maxUnavailable: 0
  template:
    metadata:
      labels:
        cluster.x-k8s.io/cluster-name: cluster
        nodepool: nodepool-0
    spec:
      bootstrap:
        configRef:
          name: md-0
          apiGroup: bootstrap.cluster.x-k8s.io
          kind: KubeadmConfigTemplate
      infrastructureRef:
        name: md-0
        apiGroup: infrastructure.cluster.x-k8s.io
        kind: Metal3MachineTemplate
      version: v1.34.0
```

## KubeadmConfigTemplate

This contains a template to generate KubeadmConfig.

Example KubeadmConfigTemplate:

```yaml
apiVersion: bootstrap.cluster.x-k8s.io/v1beta2
kind: KubeadmConfigTemplate
metadata:
  name: controlplane-0
  namespace: metal3
spec:
  template:
    spec:
      joinConfiguration:
        nodeRegistration:
          name: "{{ ds.meta_data.name }}"
          kubeletExtraArgs:
          - name: node-labels
            value: 'metal3.io/uuid={{ ds.meta_data.uuid }}'
      preKubeadmCommands:
      - netplan apply
      - systemctl enable --now crio kubelet
```

## Metal3MachineTemplate

The Metal3MachineTemplate contains following two specification fields:

- **nodeReuse**: (true/false) Whether the same pool of BareMetalHosts will be
  re-used during the upgrade/remediation operations. By default set to false, if
  set to true, CAPM3 Machine controller will pick the same pool of
  BareMetalHosts that were released while upgrading/remediation - for the next
  provisioning phase.
- **template**: is a template containing the data needed to create a
  Metal3Machine.

### Enabling nodeReuse feature

This feature can be desirable and enabled in scenarios such as upgrade or node
remediation. For example, the same pool of hosts need to be used after cluster
upgrade and no data of secondary storage should be lost. To achieve that:

1. `spec.nodeReuse` field of metal3MachineTemplate must be set to `True`. This
   tells that we want to reuse the same hosts after the upgrade, or to be exact
   same BareMetalHosts should be provisioned.

1. `spec.template.spec.automatedCleaningMode` field of metal3MachineTemplate
   must be set to `disabled`. This tells that we want secondary/hosted storage
   data to persist even after upgrade.

Above field changes need to be made before you start upgrading your cluster.

#### Node Reuse flow

When `spec.nodeReuse` field of metal3MachineTemplate is set to `True`, CAPM3
Machine controller:

- Sets `infrastructure.cluster.x-k8s.io/node-reuse` label to the
  corresponding CAPI object name (a `controlplane.cluster.x-k8s.io`
  object such as `KubeadmControlPlane` or a `MachineDeployment`) on the
  BareMetalHost during deprovisioning;
- Selects the BareMetalHost that contains
  `infrastructure.cluster.x-k8s.io/node-reuse` label and matches exact
  same CAPI object name set in the previous step during next
  provisioning.

Example Metal3MachineTemplate :

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: Metal3MachineTemplate
metadata:
  name: m3mt-0
  namespace: metal3
spec:
  nodeReuse: false
  template:
    spec:
      automatedCleaningMode: metadata
      image:
        checksum: http://172.22.0.1/images/UBUNTU_24.04_NODE_IMAGE_K8S_v1.34.0-raw.img.sha256sum
        checksumType: sha256
        format: raw
        url: http://172.22.0.1/images/UBUNTU_24.04_NODE_IMAGE_K8S_v1.34.0-raw.img
      hostSelector:
        matchLabels:
          key1: value1
        matchExpressions:
          key: key2
          operator: in
          values: { ‘abc’, ‘123’, ‘value2’ }
      dataTemplate:
        Name: m3mt-0-metadata
```

## Metal3DataTemplate

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: Metal3DataTemplate
metadata:
  name: nodepool-1
  namespace: default
  ownerReferences:
  - apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
    controller: true
    kind: Metal3Cluster
    name: cluster-1
spec:
  metaData:
    strings:
    - key: abc
      value: def
    objectNames:
    - key: name_m3m
      object: metal3machine
    - key: name_machine
      object: machine
    - key: name_bmh
      object: baremetalhost
    indexes:
    - key: index
      offset: 0
      step: 1
    ipAddressesFromIPPool:
    - key: ip
      Name: pool-1
    prefixesFromIPPool:
    - key: ip
      Name: pool-1
    gatewaysFromIPPool:
    - key: gateway
      Name: pool-1
    dnsServersFromIPPool:
    - key: dns
      Name: pool-1
    fromHostInterfaces:
    - key: mac
      interface: "eth0"
    fromLabels:
    - key: label-1
      object: machine
      label: mylabelkey
    fromAnnotations:
    - key: annotation-1
      object: machine
      annotation: myannotationkey
  networkData:
    links:
      ethernets:
      - type: "phy"
        id: "enp1s0"
        mtu: 1500
        macAddress:
          fromAnnotation:
            object: machine
            annotation: primary-mac
      - type: "phy"
        id: "enp2s0"
        mtu: 1500
        macAddress:
          fromHostInterface: "eth1"
      bonds:
      - id: "bond0"
        mtu: 1500
        macAddress:
          string: "XX:XX:XX:XX:XX:XX"
        bondMode: "802.3ad"
        bondLinks:
          - enp1s0
          - enp2s0
      vlans:
      - id: "vlan1"
        mtu: 1500
        macAddress:
          string: "YY:YY:YY:YY:YY:YY"
        vlanID: 1
        vlanLink: bond0
    networks:
      ipv4DHCP:
      - id: "provisioning"
        link: "bond0"

      ipv4:
      - id: "Baremetal"
        link: "vlan1"
        IPAddressFromIPPool: pool-1
        routes:
        - network: "0.0.0.0"
          netmask: 0
          gateway:
            fromIPPool: pool-1
          services:
            dns:
            - "8.8.4.4"
            dnsFromIPPool: pool-1
      ipv6DHCP:
      - id: "provisioning6"
        link: "bond0"
      ipv6SLAAC:
      - id: "provisioning6slaac"
        link: "bond0"
      ipv6:
      - id: "Baremetal6"
        link: "vlan1"
        IPAddressFromIPPool: pool6-1
        routes:
        - network: "0::0"
          netmask: 0
          gateway:
            string: "2001:0db8:85a3::8a2e:0370:1"
          services:
            dns:
            - "2001:4860:4860::8844"
            dnsFromIPPool: pool6-1
    services:
      dns:
      - "8.8.8.8"
      - "2001:4860:4860::8888"
status:
  indexes:
    "0": "machine-1"
  dataNames:
    "machine-1": nodepool-1-0
  lastUpdated: "2020-04-02T06:36:09Z"
```

This object will be reconciled by its own controller. When reconciled, the
controller will add a label pointing to the Metal3Cluster that has nodes linking
to this object. The spec contains a `metaData` and a `networkData` field that
contain a template of the values that will be rendered for all nodes.

The `metaData` field will be rendered into a map of strings in yaml format,
while `networkData` will be rendered into a map equivalent of
[Nova network_data.json](https://docs.openstack.org/nova/latest/user/metadata.html#openstack-format-metadata).
On the target node, the network data will be rendered as a json object that
follows the format definition that can be found
[here](https://docs.openstack.org/nova/latest/_downloads/9119ca7ac90aa2990e762c08baea3a36/network_data.json).

### Metadata Specifications

The `metaData` field contains a list of items that will render data in different
ways. The following types of objects are available and accept lists:

- **strings**: renders the given string as value in the metadata. It takes a
  `value` attribute.
- **objectNames** : renders the name of the object that matches the type given.
  It takes an `object` attribute, containing the type of the object.
- **indexes**: renders the index of the current object, with the offset from the
  `offset` field and using the step from the `step` field. The following
  conditions must be matched : `offset` >= 0 and `step` >= 1 if the step is
  unspecified (default value being 0), the controller will automatically change
  it for 1. The `prefix` and `suffix` attributes are to provide a prefix and a
  suffix for the rendered index.
- **ipAddressesFromIPPool**: renders an ip address from an _IPPool_ object. The
  _IPPool_ objects are defined in the
  [IP Address manager repo](https://github.com/metal3-io/ip-address-manager)
- **prefixesFromIPPool**: renders a network prefix from an _IPPool_ object. The
  _IPPool_ objects are defined in the
  [IP Address manager repo](https://github.com/metal3-io/ip-address-manager)
- **gatewaysFromIPPool**: renders a network gateway from an _IPPool_ object. The
  _IPPool_ objects are defined in the
  [IP Address manager repo](https://github.com/metal3-io/ip-address-manager)
- **dnsServersFromIPPool**: renders a dns servers list from an _IPPool_ object.
  The _IPPool_ objects are defined in the
  [IP Address manager repo](https://github.com/metal3-io/ip-address-manager)
- **fromHostInterfaces**: renders the MAC address of the BareMetalHost that
  matches the name given as value.
- **fromLabels**: renders the content of a label on an object or an empty string
  if the label is absent. It takes an `object` attribute to specify the type of
  the object where to fetch the label, and a `label` attribute that contains the
  label key.
- **fromAnnotations**: renders the content of a annotation on an object or an
  empty string if the annotation is absent. It takes an `object` attribute to
  specify the type of the object where to fetch the annotation, and an
  `annotation` attribute that contains the annotation key.

For each object, the attribute **key** is required.

### networkData specifications

The `networkData` field will contain three items :

- **links**: a list of layer 2 interface
- **networks**: a list of layer 3 networks
- **services** : a list of services (DNS)

#### Links specifications

The object for the **links** section list can be:

- **ethernets**: a list of ethernet interfaces
- **bonds**: a list of bond interfaces
- **vlans**: a list of vlan interfaces

The **links/ethernets** objects contain the following:

- **type**: Type of the ethernet interface
- **id**: Interface name
- **mtu**: Interface MTU
- **macAddress**: an object to render the MAC Address

The **links/ethernets/type** can be one of :

- bridge
- dvs
- hw_veb
- hyperv
- ovs
- tap
- vhostuser
- vif
- phy

The **links/ethernets/macAddress** object can be one of:

- **string**: with the desired Mac given as a string
- **fromAnnotation**: with the desired Mac retrieved from an annotation. It
  takes an `object` attribute to specify the type of the object where to fetch
  the annotation, and an `annotation` attribute that contains the annotation key.
- **fromHostInterface**: with the interface name from BareMetalHost hardware
  details.

The **links/bonds** object contains the following:

- **id**: Interface name
- **mtu**: Interface MTU
- **macAddress**: an object to render the MAC Address
- **bondMode**: The bond mode
- **bondLinks** : a list of links to use for the bond

The **links/bonds/bondMode** can be one of :

- 802.3ad
- balance-rr
- active-backup
- balance-xor
- broadcast
- balance-tlb
- balance-alb

The **links/vlans** object contains the following:

- **id**: Interface name
- **mtu**: Interface MTU
- **macAddress**: an object to render the MAC Address
- **vlanID**: The vlan ID
- **vlanLink** : The link on which to create the vlan

#### The networks specifications

The object for the **networks** section can be:

- **ipv4**: a list of ipv4 static allocations
- **ipv4DHCP**: a list of ipv4 DHCP based allocations
- **ipv6**: a list of ipv6 static allocations
- **ipv6DHCP**: a list of ipv6 DHCP based allocations
- **ipv6SLAAC**: a list of ipv6 SLAAC based allocations

The **networks/ipv4** object contains the following:

- **id**: the network name
- **link**: The name of the link to configure this network for
- **ipAddressFromIPPool**: renders an ip address from an _IPPool_ object. The
  _IPPool_ objects are defined in the
  [IP Address manager repo](https://github.com/metal3-io/ip-address-manager)
- **routes**: the list of route objects

The **networks/ipv\*/routes** is a route object containing:

- **network**: the subnet to reach
- **netmask**: the mask of the subnet as integer
- **gateway**: the gateway to use, it can either be given as a string in
  _string_ or as an IPPool name in _fromIPPool_
- **services**: a list of services object as defined later

The **networks/ipv4Dhcp** object contains the following:

- **id**: the network name
- **link**: The name of the link to configure this network for
- **routes**: the list of route objects

The **networks/ipv6** object contains the following:

- **id**: the network name
- **link**: The name of the link to configure this network for
- **ipAddressFromIPPool**: renders an ip address from an _IPPool_ object. The
  _IPPool_ objects are defined in the
  [IP Address manager repo](https://github.com/metal3-io/ip-address-manager)
- **routes**: the list of route objects

The **networks/ipv6Dhcp** object contains the following:

- **id**: the network name
- **link**: The name of the link to configure this network for
- **routes**: the list of route objects

The **networks/ipv6Slaac** object contains the following:

- **id**: the network name
- **link**: The name of the link to configure this network for
- **routes**: the list of route objects

#### the services specifications

The object for the **services** section can be:

- **dns**: a list of dns service with the ip address of a dns server
- **dnsFromIPPool**: the IPPool from which to fetch the dns servers list

## The Metal3DataClaim object

A new object would be created, a Metal3DataClaim type.

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: Metal3DataClaim
metadata:
  name: machine-1
  namespace: default
  ownerReferences:
  - apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
    controller: true
    kind: Metal3Machine
    name: machine-1
spec:
  template:
    name: nodepool-1
status:
  renderedData:
    name: nodepool-1-0
  errorMessage: ""
```

The _Metal3DataClaim_ object will reference its target _Metal3DataTemplate_
object. In its status, the _renderedData_ would reference the _Metal3Data_
object when it would be generated. In case of error, the _errorMessage_ would
contain a description of the error.

## The Metal3Data object

The output of the controller would be a Metal3Data object,one per node linking
to the Metal3DataTemplate object and the associated secrets

The Metal3Data object would be:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: Metal3Data
metadata:
  name: nodepool-1-0
  namespace: default
  ownerReferences:
  - apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
    controller: true
    kind: Metal3DataTemplate
    name: nodepool-1
spec:
  index: 0
  claim:
    name: machine-1
    namespace: metal3
  metaData:
    name: machine-1-metadata
    namespace: metal3
  networkData:
    name: machine-1-networkdata
    namespace: metal3
  template:
    name: test1-workers-template
    namespace: metal3
status:
  ready: true
  error: false
  errorMessage: ""
```

The Metal3Data will contain the index of this node, and links to the secrets
generated and to the Metal3Machine using this Metal3Data object.

If the Metal3DataTemplate object is updated, the generated secrets will not be
updated, to allow for reprovisioning of the nodes in the exact same state as
they were initially provisioned. Hence, to do an update, it is necessary to do a
rolling upgrade of all nodes.

The reconciliation of the Metal3DataTemplate object will also be triggered by
changes on Metal3Machines. In the case that a Metal3Machine gets modified, if
the `dataTemplate` references a Metal3DataTemplate, that _Metal3DataClaim_
object will be reconciled. There will be two cases:

- An already generated Metal3Data object exists for that _Metal3DataClaim_. If
  the reference is not in the _Metal3DataClaim_ object, the reconciler will add
  it. The reconciler will also verify that the required secrets exist. If they
  do not, they will be created.
- if no Metal3Data exists for that _Metal3DataClaim_, then the reconciler will
  create one and fill the respective field with the secret name.

To create a Metal3Data object, the _Metal3DataClaim_ controller will select an
index for that Metal3Machine. The selection happens by selecting the lowest
available index that is not in use. To do that, the controller will list all
existing Metal3Data object linked to this Metal3DataTemplate and to get the
unavailable indexes. The indexes always start from 0 and increment by 1. The
lowest available index is to be used next. The `dataNames` field contains the
map of Metal3Machine to Metal3Data and the `indexes` contains the map of
allocated indexes and claims.

Once the next lowest available index is found, it will create the Metal3Data
object. The name would be a concatenation of the Metal3DataTemplate name and
index. Upon conflict, it will fetch again the list to consider the new list of
Metal3Data and try to create the new object with the new index, this will happen
until the new object is created successfully. Upon success, it will render the
content values, and create the secrets containing the rendered data. The
controller will generate the content based on the `metaData` or `networkData`
field of the Metal3DataTemplate Specs. The _ready_ field in _renderedData_ will
then be set accordingly. If any error happens during the rendering, an error
message will be added.

### The generated secrets

The name of the secret will be made of a prefix and the index. The Metal3Machine
object name will be used as the prefix. A `-metadata-` or `-networkdata-` will
be added between the prefix and the index.

## Deployment flow

### Manual secret creation

In the case where the Metal3Machine is created without a `dataTemplate` value,
if the `metaData` or `networkData` fields are set (one or both), the
Metal3Machine reconciler will fetch the secret, set the status field and
directly start the provisioning of the BareMetalHost using the secrets if given.
If one of the secrets does not exist, the controller will wait to start the
provisioning of the BareMetalHost until it exists.

### Dynamic secret creation

In the case where the Metal3Machine is created with a `dataTemplate` value, the
Metal3Machine reconciler will create a _Metal3DataClaim_ for that object.

The _Metal3DataClaim_ would then be reconciled, and its controller will create
an index for this _Metal3DataClaim_ if it does not exist yet, and create a
Metal3Data object with the index. Upon success, it will set the _ready_ field to
true, and the _renderedData_ field to reference the _Metal3Data_ object.

The Metal3Data reconciler will then generate the secrets, based on the index,
the Metal3DataTemplate and the machine. Once created, it will set the status
field `ready` to True.

Once the Metal3Data object is ready, the Metal3Machine controller will fetch the
secrets that have been created (one or both) and use them to start provisioning
the BareMetalHost.

### Hybrid configuration

If the Metal3Machine object is created with a `dataTemplate` field set, but one
of the `metaData` or `networkData` is also set in the spec, this one will
override the template generation for this specific secret. i.e. if the user sets
the three fields, the controller will use the user input secret for both.

This means that some hybrid scenarios are supported, where the user can give
directly the `metaData` secret and let the controller render the `networkData`
secret through the Metal3DataTemplate object.

## Metal3 dev env examples

You can find CR examples in the
[Metal3-io dev env project](https://github.com/metal3-io/metal3-dev-env), in the
[template folder](https://github.com/metal3-io/metal3-dev-env/tree/main/tests/roles/run_tests/templates).
