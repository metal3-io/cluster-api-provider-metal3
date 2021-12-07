# Ironic automated cleaning in Metal3

This document explains how to disable or enable Ironic's automated cleaning
feature from Cluster-api-provider-metal3 (CAPM3) Custom Resources (CR).

## Table of Contents

- [Ironic automated cleaning in Metal3](#ironic-automated-cleaning-in-metal3)
  - [Table of Contents](#table-of-contents)
  - [What is automated cleaning](#what-is-automated-cleaning)
  - [How to enable/disable automated cleaning via Metal3](#how-to-enabledisabled-automated-cleaning-via-metal3)
    - [Via ironic.conf](#via-ironicconf)
    - [Via CAPM3 APIs](#via-capm3-apis)
      - [Which resources do I need to edit](#which-resources-do-i-need-to-edit)
      - [When should I edit my CRs to disable cleaning](#when-should-i-edit-my-crs-to-disable-cleaning)
      - [Selective configuration](#selective-configuration)

How we refer to these objects later on:

- Metal3Machine - m3m
- Metal3MachineTemplate - m3mt
- BareMetalHost - host

## What is automated cleaning

Ironic offers two modes for node cleaning, automated and manual where automated
cleaning is recommended for ironic deployments. Automated cleaning gets triggered
automatically during the first provisioning and on every deprovisioning of a node.
If automated cleaning is enabled, Ironic runs various cleaning steps by priority
and meanwhile the node stays in `cleaning` state (m3m/host in `provisioning`/`deprovisioning`).
To get more info, check Ironic's [documentation](https://docs.openstack.org/ironic/latest/admin/cleaning.html#automated-cleaning).

## How to enable/disabled automated cleaning via Metal3

Let's start with disabling. There are two ways to disable it.

### Via ironic.conf

You can simply set `automated_clean` to false in your ironic.conf. See an [example](https://github.com/metal3-io/ironic-image/blob/a6c4bdce7345c6f85193454b650ac0c59e738247/ironic-config/ironic.conf.j2#L62). This will disable cleaning for all the nodes
in a cluster.

```sh
[conductor]
automated_clean=false
```

Disadvantages:

- once Ironic is deployed, changing the value of `automated_clean` requires rebuilding of
    new Ironic containers images. Thus Ironic containers will need to be down until the new
    ones are ready to serve the requests
- same setting applies to all nodes

### Via CAPM3 APIs

Automated cleaning was exposed to Metal3 as an additional interface so that it is possible
to switch on/off the automated cleaning simply by editing YAMLs in Kubernetes cluster.
m3mt, m3m and host CRs have `automatedCleaningMode`
field in their Spec, which accepts either `disabled` or `metadata` values. By default,
if a user doesn't set the value in a m3mt or m3m object, the field
will be unset and omitted, while for a host the default value is `metadata`. We
explain below why it is so. When set to `disabled`, cleaning will be skipped as the name
suggests, while `metadata` enables it.

Advantages:

- no need to rebuild a new Ironic container image to switch on/off automated cleaning mode
- simply edit K8S CR to switch on/off automated cleaning
- possibility to switch on/off automated cleaning selectively per node

Disadvantages:

- accidental editing of a m3mt/m3m/host might introduce high risks with data corruption

#### Which resources do I need to edit

As mentioned above, `automatedCleaningMode` exists in m3mt, m3m
and host CRs. If you are using only Baremetal Operator (i.e. no CAPI & CAPM3), you
will not have m3mt & m3m objects installed in your Kubernetes
cluster and instead only host CR. As such, you should edit host CR
`automatedCleaningMode` field to switch on/off the cleaning for your desired hosts.

If apart from Baremetal Operator, you are also using CAPI and CAPM3, then you can and
should control the cleaning from CAPM3 CR, specifically m3mt.

Although, you can edit `automatedCleaningMode` value from m3m or host, we
recommend doing it from m3mt, because m3mt controller always
tries to replicate the value of `automatedCleaningMode` from the m3mt to the m3m. In
other words, m3mt takes precedence over m3m, and m3m's value for
`automatedCleaningMode` will be overridden by the value in m3mt.

If you are wondering why `automatedCleaningMode` exists in m3m too, that's because
CAPM3 m3m controller copies `automatedCleaningMode` field value from a m3m to its
corresponding host. Note that m3m takes precedence over host, so if
the value of `automatedCleaningMode` doesn't match between host and m3m, CAPM3 m3m
controller will edit the host. However, this is not always the case, see below for more info.

#### When should I edit my CRs to disable cleaning

Automated cleaning takes place when its first provisioning or during each deprovisioning of a
host. You can disable cleaning before provisioning or deprovisioning. Disabling before
deprovisioning means any data on the host will not be wiped out and kept as it is. Note that
this may introduce security vulnerabilities if you have sensitive data which must be wiped out
when the host is being recycled.

#### What happens if I unset automatedCleaningMode in m3mt

`automatedCleaningMode` accepts `metadata` and `disabled` or it can be unset. Please note that,
there is no default value in CAPM3 m3mt and m3m objects for `automatedCleaningMode`. So,
if you don't set either metadata or disabled in the m3mt, it will be unset both in the
m3mt and m3m objects.

**Note:** There is a default value, `metadata`, in the host CR. For that reason, even if it is
unset in the m3mt and m3m objects, Ironic node will still face automated cleaning because
the host has enabled it by its default value.

#### Selective configuration

If you have more than one m3m referencing to the same m3mt, then the `automatedCleaningMode`
value set on the m3mt will be copied to all those m3ms and eventually to corresponding hosts.
As such, by default you can not have different configurations for two nodes if they are under the same
pool. They both will have the same value, either `metadata` or `disabled`.

However, it is possible to have cleaning disabled for one of the m3m, while enabled for the other,
even though they are in the same pool (i.e. referencing the same m3mt). To achieve that:

1. Don't set `automatedCleaningMode` in the m3mt. If unset in the m3mt, it will be unset in
    m3ms too.

1. Since unset in the m3ms, hosts will get its default value `metadata`.

1. Now we have two hosts, both cleaning enabled. Since `automatedCleaningMode` is unset in the m3mt,
    you can set different cleaning modes for m3ms in the same pool. In other words, replicating the
    value of `automatedCleaningMode` from m3mt to m3ms is blocked because it is unset in the m3mt.
    This gives freedom to users to configure cleaning differently and selectively for each node regardless
    of the fact that they belong to the same m3mt.

1. You can edit one of the m3m to set `automatedCleaningMode` to `disabled`. Once edited,
    m3m controller replicates the value from the m3m to its owning host. As such, you end up
    with two m3m, one cleaning enabled and the other disabled, but still in the same pool.
