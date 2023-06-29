# IP reuse

## What is IP reuse

As a developer or user there could be a need to make IPAddresses that will be
attached to the Kubernetes nodes in the chain of: **BareMetalHost <=>
Metal3Machine <=> CAPI Machine <=> Node** objects predictable, for example
during the upgrade process and the main goal of this feature is to make that
possible.

## Logic behind

As is now, IPPool is an object representing a set of IPAddress pools to be used
for IPAddress allocations. An IPClaim is an object representing a request for an
IPAddress allocation. Consequently, the IPClaim object name is structured as
following:

**IPClaimName** = **Metal3DataName** + **(-)** + **IPPoolName**

Example: metal3datatemplate-0-pool0

The `Metal3DataName` is derived from the `Metal3DataTemplateName` with an added
index (`Metal3DataTemplateName-index`), and the `IPPoolName` comes from the
IPPool object directly. (See the
[IP Address manager repo](https://github.com/metal3-io/ip-address-manager) for
more details on these objects). In the CAPM3 workflow, when a Metal3Machine is
created and a Metal3Data object is requested, the process of choosing an `index`
to be appended to the name of the `Metal3DataTemplateName`, is random. For
example, let's imagine we have two Metal3Machines: `metal3machine-0` and
`metal3machine-1` which creates the following `metal3datatemplate-0` and
`metal3datatemplate-1` Metal3Data objects respectively. However, if two nodes
are being upgraded at a time, there is no guarantee that same indices will be
appended to the respective objects and in fact it can be in completely reverse
order (i.e. `metal3machine-0` will get `m3datatemplate-1` and `metal3machine-1`
will get `m3datatemplate-0`). In order to make it predictable, we structure
IPClaim object name using the BareMetalHost name, as following:

**IPClaimName** = **BareMetalHostName** + **(-)** + **IPPoolName**

Example: node-0-pool0

Now, the first part consists of `BareMetalHostName` which is the name of the
BareMetalHost object, and should always stay the same once created
(predictable). The second part of it is kept unchanged.

## What is the use of PreAllocations field

Once we have a predictable `IPClaimName`, we can make use of a
`PreAllocations map[string]IPAddressStr` field in the IPPool object to achieve
our goal.

We simply add the claim name(s) using the new format (BareMetalHost name
included) to the `preAllocations` field in the `IPPool`, i.e:

```yaml
apiVersion: ipam.metal3.io/v1alpha1
kind: IPPool
metadata:
  name: baremetalv4-pool
  namespace: metal3
spec:
  clusterName: test1
  gateway: 192.168.111.1
  namePrefix: test1-bmv4
  pools:
  - end: 192.168.111.200
    start: 192.168.111.100
  prefix: 24
  preAllocations:
    node-0-pool0: 192.168.111.101
    node-1-pool0: 192.168.111.102
status:
  indexes:
    node-0-pool0: 192.168.111.101
    node-1-pool0: 192.168.111.102
```

Since claim names include BareMetalHost names on them, we are able to predict an
IPAddress assigned to the specific node.

## How to enable IP reuse

To enable the feature, a boolean flag called `enableBMHNameBasedPreallocation`
was added. It is configurable via clusterctl and it can be passed to the
clusterctl configuration file by the user, i.e:

```yaml
enableBMHNameBasedPreallocation: true
```
