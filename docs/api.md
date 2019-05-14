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

* **hostSelector** -- Specify criteria for matching labels on `BareMetalHost`
  objects.  This can be used to limit the set of available `BareMetalHost`
  objects chosen for this `Machine`.

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
      hostSelector:
        matchLabels:
          key1: value1
        matchExpressions:
          key: key2
          operator: in
          values: {‘abc’, ‘123’, ‘value2’}
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

## hostSelector Examples

The `hostSelector field has two possible optional sub-fields:

* **matchLabels** -- Key/value pairs of labels that must match exactly.

* **matchExpressions** -- A set of expressions that must evaluate to true for
  the labels on a `BareMetalHost`.

Valid operators include:

* **!** -- Key does not exist.  Values ignored.
* **=** -- Key equals specified value.  There must only be one
  value specified.
* **==** -- Key equals specified value.  There must only be one
  value specified.
* **in** -- Value is a member of a set of possible values
* **!=** -- Key does not equal the specified value.  There must
  only be one value specified.
* **notin** -- Value not a member of the specified set of values.
* **exists** -- Key exists.  Values ignored.
* **gt** -- Value is greater than the one specified.  Value must be
  an integer.
* **lt** -- Value is less than the one specified.  Value must be
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
