# Fast Track Mode

## Overview

Fast Track mode is an optimization feature in CAPM3 that allows BareMetalHosts
to skip the deprovisioning power-off cycle during machine deletion when certain
conditions are met. This can significantly speed up cluster upgrades and node
replacements by keeping hosts powered on and ready for immediate re-use.

## How It Works

When a Metal3Machine is deleted, CAPM3 normally instructs the BareMetalHost to
power off and deprovision. With Fast Track mode enabled, CAPM3 can keep the host
powered on if automated cleaning is configured to only clean metadata.

The behavior is controlled by three factors:

1. **BareMetalHost.Spec.DisablePowerOff**: If `true`, the host always stays
   online (highest priority)
2. **CAPM3_FAST_TRACK environment variable**: Set to `true` or `false`
3. **BareMetalHost.Spec.AutomatedCleaningMode**: Set to `disabled` or `metadata`

### Behavior Matrix

| DisablePowerOff | CAPM3_FAST_TRACK | AutomatedCleaningMode | BMH Online Status        |
|-----------------|------------------|----------------------|---------------------------|
| true            | any              | any                  | **On** (DisablePowerOff)  |
| false           | false            | disabled             | Off                       |
| false           | true             | disabled             | Off                       |
| false           | false            | metadata             | Off                       |
| false           | true             | metadata             | **On** (Fast Track)       |

The host remains online when:

- `DisablePowerOff` is `true` (takes priority over all other settings), or
- `AutomatedCleaningMode` is set to `metadata` (not `disabled`) AND
  `CAPM3_FAST_TRACK` is set to `true`

When both conditions are met, the BareMetalHost remains powered on after the
Metal3Machine is deleted, allowing it to be quickly re-claimed by a new
Metal3Machine without waiting for a full power cycle.

## When to Use Fast Track

Fast Track is beneficial in scenarios where:

- **Rolling upgrades**: Nodes are being replaced one at a time, and speed is
   important
- **Controlled environments**: You trust that metadata cleaning is sufficient
   (no need for full disk wipes)
- **Development/testing**: Quick iteration cycles where full deprovisioning
   adds unnecessary delay

Fast Track should be avoided when:

- **Security is paramount**: Full disk wiping is required between tenants or
   workloads
- **Disk cleaning is needed**: The `disabled` AutomatedCleaningMode is used
   intentionally
- **Troubleshooting**: You need hosts to fully deprovision to reset state

## Configuration

### Enabling Fast Track

Fast Track is configured via the `CAPM3_FAST_TRACK` environment variable on the
CAPM3 controller. It defaults to `false`.

#### Via clusterctl

Add to your clusterctl configuration file:

```yaml
CAPM3_FAST_TRACK: "true"
```

#### Via Environment Variable

If deploying the controller directly, set the environment variable:

```bash
export CAPM3_FAST_TRACK=true
```

### Configuring AutomatedCleaningMode

For Fast Track to work, your BareMetalHost resources must have
`AutomatedCleaningMode` set to `metadata`:

```yaml
apiVersion: metal3.io/v1alpha1
kind: BareMetalHost
metadata:
  name: node-0
  namespace: metal3
spec:
  automatedCleaningMode: metadata
  # ... other fields
```

## Behavior During Machine Deletion

When a Metal3Machine is being deleted:

1. CAPM3 clears the BareMetalHost's image, customDeploy, userData, metaData,
   and networkData references
2. Based on the AutomatedCleaningMode and CAPM3_FAST_TRACK values, CAPM3 sets
   the BMH's `Online` field:
   - **Fast Track active**: Host stays online (`Online: true`)
   - **Fast Track inactive**: Host is powered off (`Online: false`)
3. CAPM3 waits for the host to reach an available state before fully releasing it
4. The ConsumerRef and other association data are cleared

## Monitoring

When Fast Track is active, you'll see log messages like:

```text
Set host Online field by AutomatedCleaningMode host=node-0
  automatedCleaningMode=metadata hostSpecOnline=true
```

When Fast Track keeps a host online, the BareMetalHost will transition directly
from `Provisioned` to `Available` without going through `Deprovisioning` with a
power-off cycle.

## Troubleshooting

### Host Not Staying Online

If hosts are being powered off despite Fast Track being enabled:

1. Verify `CAPM3_FAST_TRACK` is set to `true` (not `True` or `1`)
2. Check that `AutomatedCleaningMode` is `metadata`, not `disabled`
3. Review controller logs for the decision logic

### Host Stuck in Deprovisioning

If a host appears stuck, it may be waiting for cleaning to complete. Check:

1. The Ironic conductor logs for cleaning status
2. The BareMetalHost status for any error messages
3. Whether the host's BMC is accessible

## Related Documentation

- [Disk Cleaning](disk_cleaning.md) - Details on automated cleaning modes
- [IP Reuse](ip_reuse.md) - Related feature for predictable IP allocation during
   upgrades
- [Baremetal Operator Documentation](https://book.metal3.io/bmo/introduction) -
   BMH lifecycle and cleaning modes
