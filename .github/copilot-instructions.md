# Cluster API Provider Metal3 (CAPM3) - AI Coding Assistant Instructions

## Project Overview

CAPM3 is a Kubernetes Cluster API infrastructure provider that enables
declarative provisioning and management of Kubernetes clusters on bare metal
infrastructure. It bridges Cluster API (CAPI) with Metal3's BareMetalHost
resources, orchestrating the lifecycle of bare metal machines as Kubernetes
cluster nodes.

## Architecture

### Core Components

1. **Custom Resources (CRDs)** - Located in `api/v1beta1/`:

   - `Metal3Cluster` - Infrastructure cluster resource (maps to CAPI Cluster)
   - `Metal3Machine` - Infrastructure machine resource (maps to CAPI Machine)
   - `Metal3MachineTemplate` - Template for creating Metal3Machines
   - `Metal3Data` - Rendered configuration data (user-data, network-data,
     metadata)
   - `Metal3DataTemplate` - Template for generating per-machine data
   - `Metal3Remediation` - Custom remediation resource for machine health

2. **Controllers** - Located in `controllers/`:
   - `Metal3MachineReconciler` - Manages Metal3Machine lifecycle,
     associates with BareMetalHosts
   - `Metal3ClusterReconciler` - Manages cluster-level infrastructure
   - `Metal3DataReconciler` - Renders data templates into concrete
     configurations
   - `Metal3DataTemplateReconciler` - Validates and manages data templates
   - `Metal3RemediationReconciler` - Handles machine remediation (reboots,
     replacements)

   - `Metal3LabelSyncReconciler` - Syncs labels between CAPI and Metal3
     resources

3. **BareMetalHost Integration** - Located in `baremetal/`:
   - `metal3machine_manager.go` - Core logic for associating Metal3Machines
     with BareMetalHosts
   - `remote/` - Client for accessing management clusters during pivoting
   - Claims available BareMetalHosts based on selectors, hardware profiles,
     or explicit references

### Resource Relationships

```text
CAPI Machine → Metal3Machine → BareMetalHost (BMO)

     ↓              ↓
CAPI Cluster → Metal3Cluster
     ↓              ↓
     └──────→ Metal3DataTemplate → Metal3Data
```

- Metal3Machine claims a BareMetalHost and populates its `spec.image`,
  `spec.userData`, `spec.networkData`
- Metal3Data is created from Metal3DataTemplate with machine-specific
  substitutions
- Labels and annotations flow: CAPI resources → Metal3 resources →
  BareMetalHosts

## Development Workflows

### Build and Test

```bash
# Generate manifests after API changes

make generate-manifests  # Runs controller-gen for CRDs, RBAC, webhooks

# Run unit tests (controllers, baremetal package, api, webhooks)
make unit

# Run unit tests with coverage
make unit-cover

# Run linters
make lint

# Build manager binary
make manager

# Build container image
IMG=quay.io/metal3-io/cluster-api-provider-metal3 TAG=dev make docker-build
```

### Local Development with Tilt

```bash
# Create kind cluster named "bmo"
kind create cluster --name bmo

# Configure tilt-settings.json for local development
cat > tilt-settings.json <<EOF
{
  "default_registry": "ghcr.io/yourusername",
  "enable_providers": ["metal3"],
  "kustomize_substitutions": {
    "CLUSTER_TOPOLOGY": "true"
  }
}
EOF

# Start Tilt (auto-rebuilds on file changes)
tilt up
```

Tilt watches for changes and automatically rebuilds/redeploys the controller.

### E2E Tests

E2E tests use the Cluster API test framework:

```bash
# Run full e2e test suite (requires metal3-dev-env or similar setup)

make test-e2e

# Run ClusterClass e2e tests
make test-clusterclass-e2e

# Run specific test scenarios
GINKGO_FOCUS="basic" ./scripts/ci-e2e.sh
```

Test configuration:

- `test/e2e/config/e2e_conf.yaml` - Test configuration
 and versions
- `test/e2e/data/infrastructure-metal3/` - Cluster template definitions
- E2E tests create actual Kubernetes clusters using BareMetalHosts

## Code Patterns and Conventions

### API Modifications

1. Edit types in `api/v1beta1/*_types.go`
2. Run `make generate-manifests` to regenerate:

   - CRDs in `config/crd/bases/`
   - RBAC in `config/rbac/`

   - Webhook configurations
3. Update API documentation if public fields change
4. Ensure backward compatibility for v1beta1 API

### Controller Patterns

**Manager Selection Pattern:**

```go
// Controllers use manager pattern for reconciliation logic

// Manager struct contains business logic, reconciler orchestrates
type Metal3MachineManager struct {
    client        client.Client
    Log           logr.Logger
    Metal3Machine *infrav1.Metal3Machine
    Machine       *clusterv1.Machine
    Cluster       *clusterv1.Cluster
    Metal3Cluster *infrav1.Metal3Cluster
    BMCClient     bmc.Client  // For direct BMC operations if needed
}

// Manager methods return specific errors for reconciler to handle
func (m *Metal3MachineManager) SetNodeProviderID(ctx context.Context) error
func (m *Metal3MachineManager) Associate(ctx context.Context) error
```

**BareMetalHost Association:**

```go
// Association happens via:

// 1. HostSelector (labels) - most common
// 2. DataTemplate with hostSelector
// 3. Explicit BMH reference in spec

// BMH selection in priority order:
// - spec.image.url set → look for BMH in "provisioned" state
// - No image → look for BMH in "available" state
// - Check ownership, labels, and claim BMH by setting consumerRef
```

### Cluster Lifecycle

```yaml
**Owner References:**

- All cluster-scoped resources must be in the same namespace
- Metal3Machine sets ownerRef on Metal3Data
- BareMetalHost gets consumerRef pointing to Metal3Machine
- OwnerRefs enable garbage collection on cluster deletion

### Data Template Rendering

Metal3DataTemplate supports variable substitution:

```yaml
# Template with variables
spec:
  metaData:
    strings:
      - key: "node-hostname"
        value: "{{ .Machine.Name }}"
    indexes:
      - key: "ip-index"
        offset: 10
        step: 1
        prefix: "192.168.1."
        suffix: "/24"
```

Testing practices:

- Use `gomega` for assertions

Available objects: `.Machine`, `.Metal3Machine`, `.BaremetalHost`

### Testing

**Unit Tests:**

- Use `gomega` for assertions

- Mock external dependencies (CAPI client, BMH client)
- Test files alongside source: `*_controller_test.go`
- Integration tests use envtest: `*_controller_integration_test.go`

**E2E Tests:**

- Tag with `//go:build e2e`

- Use CAPI e2e framework helpers
- Located in `test/e2e/`
- Tests run against real kind clusters with BareMetalHosts

### Webhooks

Located in `internal/webhooks/v1beta1/`:

- Validation webhooks for all resources

- Defaulting logic for optional fields
- Immutability checks for sensitive fields
- Webhook tests in `*_webhook_test.go`

## Integration Points

### With Cluster API

- CAPM3 registers as infrastructure provider via `clusterctl`
- Implements InfrastructureCluster and InfrastructureMachine contracts
- Responds to CAPI machine lifecycle events (create, update, delete)
- Sets machine providerID once node joins cluster

### With Baremetal Operator

- Reads BareMetalHost CRD from BMO
- Sets `spec.consumerRef` to claim hosts
- Writes `spec.image`, `spec.userData`, `spec.networkData` to provision
- Monitors `status.provisioning.state` for provisioning progress
- Releases hosts on machine deletion by clearing consumerRef

### With IP Address Manager (IPAM)

- Optional integration for IP allocation
- Metal3DataTemplate can reference IPPool, IPClaim resources
- Automatic IP allocation for machines when IPPool configured

## Key Files Reference

- `main.go` - Entrypoint, controller registration, manager setup
- `clusterctl-settings.json` - ClusterCTL provider configuration
- `metadata.yaml` - Provider metadata for clusterctl
- `Makefile` - Primary build/test interface
- `config/default/` - Default deployment manifests
- `examples/clusterctl-templates/` - Example cluster templates
- `scripts/ci-e2e.sh` - E2E orchestration script

## Common Workflows

### Deploying a Cluster

```bash
# Initialize management cluster with CAPM3
clusterctl init --infrastructure metal3

# Generate cluster manifest from template
clusterctl generate cluster my-cluster \
  --kubernetes-version v1.28.0 \
  --control-plane-machine-count 3 \
  --worker-machine-count 2 > cluster.yaml

# Apply cluster manifest
kubectl apply -f cluster.yaml

# Watch machines get associated with BareMetalHosts
kubectl get metal3machines -w
kubectl get baremetalhosts -w
```

### Pivoting (Moving Management Components)

```bash
# Move CAPI resources to target cluster (includes CAPM3)

clusterctl move --to-kubeconfig=target-kubeconfig.yaml

# Note: BareMetalHosts and Metal3Data moved via status annotations
# CAPM3 handles re-establishing BMH associations after pivot
```

## Common Pitfalls and Solutions

## Common Pitfalls

1. **Namespace Mismatch** - All resources (Cluster, Machine, Metal3Machine,
   BareMetalHost) must be in same namespace
2. **Missing BareMetalHosts** - Metal3Machine stuck in "Pending" if no
   available BMHs match selector
3. **DataTemplate Errors** - Check Metal3Data status for rendering errors
   (missing variables, syntax issues)
4. **Pivoting Complexity** - Ensure Ironic accessible from target cluster,
   BMH status annotations preserved
5. **Update `docs/api.md`** when changing API types (run
   `make generate-manifests`)
6. **Owner References** - Metal3Machine should own Metal3Data,
   Machine owns Metal3Machine
7. **Finalizers** - Use `metal3machine.infrastructure.cluster.x-k8s.io` to
   ensure cleanup
8. **Cluster Infrastructure Ready** - Metal3Cluster must be Ready before
   machines reconcile

## Integration with Other Metal3 Components

- **IPAM** - cluster-api-provider-metal3 integrates with ip-address-manager:
  IPPool resources define address ranges, Metal3Data references IPClaims
- **BMO** - Direct dependency on baremetal-operator CRDs and controllers:
  imports `github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1`
- **Ironic** - Consumed via BMO; CAPM3 never talks to Ironic directly:
    all hardware operations go through BareMetalHost resources

## Key Files and References

- `main.go` - Controller manager setup, webhook registration
- `api/v1beta1/` - CRD type definitions
- `controllers/` - Reconciliation logic
- `baremetal/metal3machine_manager.go` - BMH association logic
- `examples/` - Sample cluster definitions
- `config/` - Kustomize deployment configurations
- `docs/api.md` - API documentation (update when APIs change)

## CI/PR Integration

- E2E tests run via Jenkins on Nordix infrastructure
- Tests create real clusters with BareMetalHosts:
  `/test metal3-e2e-integration-test-centos`
- Tests use metal3-dev-env for full stack testing
- Sign commits with `-s` flag (DCO required)
- Multiple test variants: basic integration, feature tests (pivoting,
  remediation), upgrade tests
- PR tests triggered via `/test` comments

## Version Compatibility

CAPM3 follows CAPI release cadence:

- API version: v1beta1 (stable)
- CAPI version compatibility in README
- Maintains compatibility with BMO API (metal3.io/v1alpha1)
- Kubernetes version support matches CAPI support matrix
