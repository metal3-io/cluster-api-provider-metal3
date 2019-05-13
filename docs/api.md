# API and Resource Definitions

## Machine

The `Machine` resource is defined by the
[cluster-api](https://github.com/kubernetes-sigs/cluster-api) project.  A
`Machine` includes a `providerSpec` field which includes the data specific to
this `cluster-api` provider.

## BareMetalMachineProviderSpec

* **image** -- This includes two sub-fields, `url` and `checksum`, which
  include the URL to the image and the URL to a checksum for that image.  These
  fields are required.  The image will be used for provisioning of the
  `BareMetalHost` chosen by the `Machine` actuator.

* **userData** -- This includes two sub-fields, `name` and `namespace`, which
  reference a `Secret` that contains base64 encoded user-data to be written to
  a config drive on the provisioned `BareMetalHost`.  This field is optional.

## Sample Machine

```yaml
apiVersion: cluster.k8s.io/v1alpha1
kind: Machine
metadata:
  annotations:
    metal3.io/BareMetalHost: metal3/master-0
  creationTimestamp: "2019-05-13T13:00:51Z"
  finalizers:
  - machine.cluster.k8s.io
  generateName: baremetal-machine-
  generation: 2
  name: centos
  namespace: metal3
  resourceVersion: "1112"
  selfLink: /apis/cluster.k8s.io/v1alpha1/namespaces/metal3/machines/centos
  uid: 22acee54-757f-11e9-8091-280d3563c053
spec:
  metadata:
    creationTimestamp: null
  providerSpec:
    value:
      apiVersion: baremetal.cluster.k8s.io/v1alpha1
      image:
        checksum: http://172.22.0.1/images/CentOS-7-x86_64-GenericCloud-1901.qcow2.md5sum
        url: http://172.22.0.1/images/CentOS-7-x86_64-GenericCloud-1901.qcow2
      kind: BareMetalMachineProviderSpec
      userData:
        name: centos-user-data
        namespace: metal3
  versions:
    kubelet: ""
```

## Sample userData Secret

```yaml
apiVersion: v1
data:
  userData: BASE64_ENCODED_USER_DATA
kind: Secret
metadata:
  annotations:
  creationTimestamp: 2019-05-13T13:00:51Z
  name: centos-user-data
  namespace: metal3
  resourceVersion: "1108"
  selfLink: /api/v1/namespaces/metal3/secrets/centos-user-data
  uid: 22792b3e-757f-11e9-8091-280d3563c053
type: Opaque
```
