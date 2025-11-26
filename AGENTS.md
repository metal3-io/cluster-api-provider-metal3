# Cluster API Provider Metal3 (CAPM3) - AI Coding Agent Instructions

<!-- markdownlint-configure-file {
  "MD013": {
    "tables": false, "code_blocks": false
  }
} -->

This file provides comprehensive instructions for AI coding agents working on
the Cluster API Provider Metal3 project. It covers architecture, conventions,
tooling, CI/CD, and behavioral guidelines.

## Table of Contents

- [Project Overview](#project-overview)
- [Architecture](#architecture)
- [Development Workflows](#development-workflows)
- [Makefile Reference](#makefile-reference)
- [Hack Scripts Reference](#hack-scripts-reference)
- [CI/CD and Prow Integration](#cicd-and-prow-integration)
- [Code Patterns and Conventions](#code-patterns-and-conventions)
- [Testing Guidelines](#testing-guidelines)
- [Integration Points](#integration-points)
- [Common Workflows](#common-workflows)
- [Common Pitfalls and Solutions](#common-pitfalls-and-solutions)
- [AI Agent Behavioral Guidelines](#ai-agent-behavioral-guidelines)

---

## Project Overview

CAPM3 is a Kubernetes Cluster API infrastructure provider that enables
declarative provisioning and management of Kubernetes clusters on bare metal
infrastructure. It bridges Cluster API (CAPI) with Metal3's BareMetalHost
resources, orchestrating the lifecycle of bare metal machines as Kubernetes
cluster nodes.

CAPM3 focuses on cluster infrastructure provisioning and machine lifecycle management,
integrating with other Metal3 components including the IP Address Manager (IPAM),
Baremetal Operator (BMO), and Ironic.

**Key URLs:**

- Repository: <https://github.com/metal3-io/cluster-api-provider-metal3>
- Container Image: `quay.io/metal3-io/cluster-api-provider-metal3`
- Documentation: <https://book.metal3.io>

### Project Goals

CAPM3 bridges the gap between **Cluster API** and **Ironic** (via BMO),
enabling declarative bare metal Kubernetes cluster management:

1. **CAPI Infrastructure Provider** - Implements the Cluster API provider
   contract, translating CAPI Machine lifecycle into BareMetalHost operations.

2. **Ironic Abstraction** - Users interact with Kubernetes-native resources
   (Metal3Machine, Metal3Cluster) while CAPM3 orchestrates the underlying
   Ironic-based provisioning through BMO.

3. **Data-Driven Configuration** - Metal3DataTemplate enables flexible,
   per-machine configuration (cloud-init, network-data, metadata) with
   variable substitution.

4. **Production Ready** - Stable v1beta1 API with validation webhooks,
   remediation support, and ClusterClass compatibility.

---

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

### Directory Structure

```text
cluster-api-provider-metal3/
├── api/v1beta1/           # CRD type definitions (separate Go module)
├── baremetal/             # BareMetalHost integration logic
│   ├── mocks/            # Generated mocks for testing
│   └── remote/           # Remote cluster client for pivoting
├── config/                # Kustomize manifests
│   ├── crd/bases/        # Generated CRD YAMLs
│   ├── default/          # Default deployment
│   ├── rbac/             # RBAC manifests
│   └── webhook/          # Webhook manifests
├── controllers/           # Controller implementations
├── docs/                  # Documentation
├── examples/              # Example cluster templates
│   ├── clusterctl-templates/  # Templates for clusterctl
│   └── generate.sh       # Example generation script
├── hack/                  # Build and CI scripts
│   ├── boilerplate/      # License headers
│   ├── fake-apiserver/   # Test API server
│   └── tools/            # Tool dependencies
├── internal/webhooks/     # Webhook implementations
├── releasenotes/          # Release notes
├── scripts/               # E2E and CI orchestration
└── test/                  # E2E test suite
    └── e2e/              # E2E test framework integration
```

---

## Development Workflows

### Quick Start Commands

```bash
# Full verification (generate + lint + test)
make test

# Generate code after API changes
make generate

# Run unit tests only
make unit

# Run linters only
make lint

# Build manager binary
make manager

# Verify go modules are tidy
make verify-modules
```

### Local Development with Tilt

Tilt provides live reloading for local development:

```bash
# Create kind cluster with local registry
make kind-create

# Generate tilt-settings.json
make tilt-settings

# Start Tilt (auto-rebuilds on file changes)
make tilt-up
```

The kind cluster is named "capm3" and includes a local container registry.

### Docker Build

```bash
# Build container image
make docker-build IMG=quay.io/metal3-io/cluster-api-provider-metal3 TAG=dev

# Build with debug symbols
make docker-build-debug
```

---

## Makefile Reference

### Testing Targets

| Target | Description |
|--------|-------------|
| `make test` | Run lint + unit tests (main verification command) |
| `make unit` | Run unit tests for controllers, baremetal, api, webhooks |
| `make unit-cover` | Run unit tests with coverage report |
| `make unit-verbose` | Run unit tests with verbose output |
| `make test-e2e` | Run E2E tests (requires metal3-dev-env setup) |
| `make test-clusterclass-e2e` | Run ClusterClass E2E tests |

### Build Targets

| Target | Description |
|--------|-------------|
| `make build` | Build all modules (manager binary, api, e2e tests) |
| `make manager` | Build manager binary to `bin/manager` |
| `make build-api` | Build api module only |
| `make build-e2e` | Build test module only |
| `make build-fkas` | Build fake API server |
| `make binaries` | Alias for manager |

### Code Generation Targets

| Target | Description |
|--------|-------------|
| `make generate` | Generate all code (Go + manifests) |
| `make generate-go` | Generate Go code (deepcopy, mocks, conversions) |
| `make generate-manifests` | Generate CRDs, RBAC, webhooks |
| `make generate-examples` | Generate example configurations |
| `make generate-examples-clusterclass` | Generate ClusterClass examples |

### Linting Targets

| Target | Description |
|--------|-------------|
| `make lint` | Run golangci-lint on all modules (fast mode) |
| `make lint-fix` | Run linters with auto-fix enabled |
| `make lint-full` | Run slower, more thorough linters |
| `make manifest-lint` | Validate Kubernetes manifests |

### Verification Targets

| Target | Description |
|--------|-------------|
| `make verify` | Run all verification (boilerplate + modules) |
| `make verify-boilerplate` | Check Apache 2.0 license headers |
| `make verify-modules` | Verify go.mod files are up to date |

### Module Management

| Target | Description |
|--------|-------------|
| `make modules` | Run go mod tidy on all modules (root, api, test, tools, fake-apiserver) |

### Docker Targets

| Target | Description |
|--------|-------------|
| `make docker-build` | Build controller image for current ARCH |
| `make docker-build-debug` | Build with debug symbols |
| `make docker-push` | Push controller image |
| `make docker-build-all` | Build for all architectures (amd64, arm64, etc.) |
| `make docker-push-all` | Push all architecture images |
| `make docker-push-manifest` | Push multi-arch manifest |
| `make docker-build-fkas` | Build fake API server container |

### Deployment Targets

| Target | Description |
|--------|-------------|
| `make deploy` | Deploy CAPM3 to cluster (generates examples first) |
| `make deploy-clusterclass` | Deploy with ClusterClass support |
| `make deploy-examples` | Deploy example cluster resources |
| `make deploy-bmo-crd` | Deploy BareMetalHost CRDs |
| `make kind-create` | Create kind cluster for local development |
| `make tilt-up` | Start Tilt for live development |
| `make tilt-settings` | Generate tilt-settings.json |

### E2E Targets

| Target | Description |
|--------|-------------|
| `make e2e-substitutions` | Process e2e configuration with envsubst |
| `make cluster-templates` | Generate all version cluster templates |
| `make clusterclass-templates` | Generate ClusterClass templates |
| `make e2e-tests` | Run E2E tests (called from scripts/ci-e2e.sh) |

### Release Targets

| Target | Description |
|--------|-------------|
| `make release-manifests` | Build release manifests to `out/` |
| `make release-notes` | Generate release notes (requires RELEASE_TAG) |
| `make release` | Full release process (requires RELEASE_TAG) |

### Cleanup Targets

| Target | Description |
|--------|-------------|
| `make clean` | Remove all generated files |
| `make clean-bin` | Remove binaries |
| `make clean-temporary` | Remove temporary files |
| `make clean-release` | Remove release directory |
| `make clean-examples` | Remove generated examples |
| `make clean-e2e` | Clean E2E environment |

### Utility Targets

| Target | Description |
|--------|-------------|
| `make go-version` | Print Go version used for builds |
| `make help` | Display all available targets |
| `make setup-envtest` | Set up envtest (download kubebuilder assets) |

---

## Hack Scripts Reference

Scripts in `hack/` are primarily for CI (Prow jobs call them directly).
**For local development, prefer Make targets** which handle setup automatically.

### Scripts Without Make Target Equivalents

| Script | Purpose |
|--------|---------|
| `verify-release.sh` | Comprehensive release verification |
| `gen_tilt_settings.sh` | Generate tilt-settings.json (or use `make tilt-settings`) |

### CI-to-Make Target Mapping

| CI Job | Make Target | Script (for reference) |
|--------|-------------|------------------------|
| `unit` | `make unit` | `unit.sh` |
| `build` | `make build` | `build.sh` |
| `generate` | `make generate` | `codegen.sh` |
| `gomod` | `make modules` | `gomod.sh` |
| `shellcheck` | - | `shellcheck.sh` |
| `markdownlint` | - | `markdownlint.sh` |
| `manifestlint` | `make manifest-lint` | `manifestlint.sh` |

Scripts without Make targets (`shellcheck.sh`, `markdownlint.sh`)
auto-spawn containers and can be run directly: `./hack/shellcheck.sh`

### E2E Scripts

Located in `scripts/`:

| Script | Purpose |
|--------|---------|
| `ci-e2e.sh` | Main E2E orchestration (use `make test-e2e` or run directly) |

---

## CI/CD and Prow Integration

### Prow Jobs (metal3-io/project-infra)

CI is managed via Prow jobs defined in the separate
[metal3-io/project-infra](https://github.com/metal3-io/project-infra) repository.

**Prow job definitions:**

- [Main branch jobs](https://github.com/metal3-io/project-infra/blob/main/prow/config/jobs/metal3-io/cluster-api-provider-metal3.yaml)
- Release branch jobs (one per supported release) in the same directory

**Pre-submit jobs** (run on PRs):

| Job | Trigger | What it does |
|-----|---------|--------------|
| `gomod` | Code changes | Verifies go.mod is tidy |
| `unit` | Code changes | Runs unit tests |
| `generate` | Code changes | Verifies generated code is up to date |
| `test` | Makefile/hack changes | Verifies Makefile/scripts work |
| `build` | api/test/Makefile changes | Builds binaries |
| `build-fkas` | fake-apiserver changes | Builds fake API server |
| `shellcheck` | `.sh` or Makefile changes | Shell script linting |
| `markdownlint` | `.md` changes | Markdown linting |
| `manifestlint` | Code changes | Kubernetes manifest validation |

**Periodic jobs** (Jenkins):

- E2E integration tests (Ubuntu, CentOS)
- Feature tests (pivoting, remediation, other features)
- Upgrade tests
- Conformance tests

**Skip Patterns:**

Jobs skip for:

- OWNERS/OWNERS_ALIASES changes only
- Markdown-only changes (for code jobs)

### GitHub Actions

Located in `.github/workflows/`:

- `pr-verifier.yaml` - Uses shared workflow from project-infra
- `golangci-lint.yml` - Additional linting checks
- `build-images-action.yml` - Build container images
- `release.yaml` - Release automation
- `dependabot.yml` - Dependency updates

### Triggering Tests

From PR comments (Prow commands):

```text
/test all              - Run all pre-submit tests
/retest                - Re-run failed tests
/test <job-name>       - Run specific test

# Available test commands (see Prow config for full list):
/test metal3-centos-e2e-basic-test-main
/test metal3-ubuntu-e2e-basic-test-main
/test metal3-centos-e2e-feature-test-main-pivoting
/test metal3-centos-e2e-feature-test-main-remediation
```

**Other Prow commands:**

```text
/lgtm                  - Add lgtm label (approvers only)
/approve               - Approve PR (maintainers only)
/hold                  - Prevent merge
/hold cancel           - Remove hold
/assign @username      - Assign to user
/cc @username          - Request review
```

See full command reference: <https://prow.apps.test.metal3.io/command-help>

**View test results:**

- Prow dashboard: <https://prow.apps.test.metal3.io>
- Jenkins: <https://jenkins.nordix.org/view/Metal3%20Periodic/>

---

## Code Patterns and Conventions

### Go Code Style

1. **Follow standard Go conventions** - use `gofmt`, follow effective Go guidelines
2. **Use controller-runtime patterns** - reconcilers, managers, clients
3. **Logging** - use structured logging with `logr.Logger`
4. **Errors** - wrap errors with context using `fmt.Errorf` with `%w`
5. **Testing** - use Ginkgo/Gomega for tests, envtest for integration tests

### Shell Script Conventions

Based on existing scripts in `hack/`:

1. **Shebang**: Use `#!/bin/sh` for POSIX-compatible scripts,
   `#!/usr/bin/env bash` for bash-specific features
2. **Error handling**: Use `set -eux` for sh scripts,
   `set -euxo pipefail` for bash scripts
3. **ShellCheck**: All scripts must pass shellcheck; use
   `# shellcheck disable=SCXXXX` only when necessary
4. **Variables**: Quote all variable expansions (`"${VAR}"`) unless word
   splitting is intended
5. **Portability**: Prefer POSIX shell features unless bash-specific features
   are required

**Example script structure:**

```bash
#!/bin/sh
# shellcheck disable=SC2292

set -eux

IS_CONTAINER="${IS_CONTAINER:-false}"
CONTAINER_RUNTIME="${CONTAINER_RUNTIME:-podman}"
WORKDIR="${WORKDIR:-/workdir}"

if [ "${IS_CONTAINER}" != "false" ]; then
    # Container execution logic
    actual_work_here
else
    # Launch container
    "${CONTAINER_RUNTIME}" run --rm \
        --env IS_CONTAINER=TRUE \
        --volume "${PWD}:${WORKDIR}:ro,z" \
        --workdir "${WORKDIR}" \
        image:tag \
        "${WORKDIR}"/script.sh "$@"
fi
```

### Markdown Conventions

Based on `.markdownlint-cli2.yaml`:

1. **Unordered list indentation**: Use 3 spaces (Kramdown style)
2. **Run markdownlint**: Use `make manifest-lint` or `hack/markdownlint.sh`
3. **No auto-fix**: Linting is for verification only, fix manually
4. **Ignore .github**: The .github directory is excluded from linting

**Example:**

```markdown
- Top level item
   - Second level (3 spaces)
      - Third level (6 spaces)
```

### Boilerplate Headers

All Go, shell, and Python files must include Apache 2.0 license header.
Use `make verify-boilerplate` to check.

**Go files** (`hack/boilerplate.go.txt`):

```go
/*
Copyright The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
```

**Shell scripts**: Use similar header with `#` comments

### API Modifications

1. Edit types in `api/v1beta1/*_types.go`
2. Run `make generate-manifests` to regenerate:
   - CRDs in `config/crd/bases/`
   - RBAC in `config/rbac/`
   - Webhook configurations
3. Run `make generate-go` to regenerate:
   - DeepCopy methods
   - Conversion functions
   - Mock interfaces
4. Update `docs/api.md` if public fields change
5. Ensure backward compatibility for v1beta1 API
6. Run `make verify-modules` to check go.mod consistency

### Controller Patterns

**Manager Selection Pattern:**

Controllers use the manager pattern - manager structs contain business logic,
reconcilers orchestrate:

```go
// Manager struct contains business logic
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

Available objects: `.Machine`, `.Metal3Machine`, `.BaremetalHost`

---

## Testing Guidelines

### Unit Test Framework

Tests use Ginkgo (BDD) + Gomega (matchers):

```go
var _ = Describe("Metal3Machine Controller", func() {
    Context("when processing a BareMetalHost", func() {
        It("should associate the host with the machine", func() {
            // Test implementation
            Expect(result).To(Equal(expected))
        })
    })
})
```

### Test Setup

`controllers/suite_test.go` bootstraps envtest for controller tests:

- Starts kube-apiserver and etcd
- Loads CRDs from `config/crd/bases/`
- Registers schemes (CAPI, CAPM3, BMO)

### Running Tests

```bash
# All unit tests (recommended)
make unit

# With coverage
make unit-cover

# With verbose output
make unit-verbose

# E2E tests (requires metal3-dev-env)
make test-e2e

# E2E with specific focus
GINKGO_FOCUS="basic" ./scripts/ci-e2e.sh
```

Note: Use `make unit` rather than `go test` directly, as envtest requires
proper setup via the Makefile.

### Test Organization

- **Unit tests**: `*_controller_test.go`, `*_test.go` alongside source
- **Integration tests**: `*_integration_test.go` using envtest
- **E2E tests**: `test/e2e/` using CAPI e2e framework
- **Mocks**: Generated in `baremetal/mocks/` using mockgen

### E2E Test Configuration

- `test/e2e/config/e2e_conf.yaml` - Test configuration and versions
- `test/e2e/data/infrastructure-metal3/` - Cluster template definitions
- `scripts/ci-e2e.sh` - Main E2E orchestration script
- Tests create actual Kubernetes clusters using BareMetalHosts from metal3-dev-env

### Test Environment Tools

- `kube-apiserver`, `etcd`, `kubectl` - Downloaded via `make setup-envtest`
- Location managed by setup-envtest tool
- Version controlled by `ENVTEST_K8S_VERSION` in Makefile

### Coverage

Coverage is tracked per module:

- Root module: controllers, baremetal packages
- API module: `api/` directory
- Webhooks module: `internal/webhooks/` directory

Run `make unit-cover` to see coverage reports for all modules.

---

## E2E Testing (Critical for CAPM3)

### Overview

**CAPM3 serves as the central repository for E2E testing** for the entire Metal3
ecosystem. E2E tests run here validate not just CAPM3 itself, but also:

- **IPAM** (IP Address Manager)
- **BMO** (Baremetal Operator)
- **Ironic** integration
- **CAPI** compatibility across versions

This makes E2E testing configuration in CAPM3 **critical infrastructure** that
must be maintained carefully, especially when cutting new release branches.

### E2E Architecture

```text
scripts/ci-e2e.sh (orchestration)
    ↓
metal3-dev-env (environment setup)
    ↓
test/e2e/config/e2e_conf.yaml (CAPI e2e framework config)
    ↓
test/e2e/data/infrastructure-metal3/{version}/ (cluster templates)
    ↓
test/e2e/*_test.go (actual test scenarios)
```

### E2E Directory Structure

```text
test/e2e/
├── config/
│   └── e2e_conf.yaml          # Main E2E configuration (provider versions)
├── data/
│   ├── infrastructure-metal3/  # Cluster templates per version
│   │   ├── main/              # Templates for main branch
│   │   ├── v1.11/             # Templates for release-1.11
│   │   ├── v1.10/             # Templates for release-1.10
│   │   └── v1.9/              # Templates for release-1.9
│   ├── shared/                # Shared CAPI metadata
│   │   └── capi/
│   │       ├── v1.12/
│   │       ├── v1.11/
│   │       └── v1.10/
│   ├── bmo-deployment/        # BMO deployment overlays
│   │   └── overlays/
│   │       ├── release-latest/
│   │       ├── release-0.11/
│   │       └── pr-test/
│   └── ironic-deployment/     # Ironic deployment overlays
│       └── overlays/
│           ├── release-latest/
│           ├── release-32.0/
│           └── pr-test/
├── *_test.go                  # Test scenarios
└── *.go                       # Helper functions
```

### E2E Configuration File

`test/e2e/config/e2e_conf.yaml` is the heart of E2E testing:

**Structure:**

- `images:` - Container images to load (CAPM3, BMO, IPAM)
- `providers:` - Provider versions and URLs
   - `cluster-api` - Core CAPI provider (multiple versions)
   - `kubeadm` - Bootstrap provider
   - `kubeadm` - Control plane provider
   - `metal3` - Infrastructure provider (CAPM3)
   - `ipam-in-cluster` - IPAM provider
- `variables:` - Test variables (Kubernetes versions, images, etc.)
- `intervals:` - Test timeout intervals

**Provider version format:**

```yaml
- name: "{go://sigs.k8s.io/cluster-api@v1.11}"  # Dynamic from go.mod
  value: "https://github.com/.../v1.11.0/core-components.yaml"
  type: "url"
  contract: v1beta2
```

### Test Scenarios

| Test File | Label | Description |
|-----------|-------|-------------|
| `basic_integration_test.go` | `basic` | Basic cluster creation and deletion |
| `integration_test.go` | `integration` | Full integration tests (pivoting, remediation, features) |
| `upgrade_clusterctl_test.go` | `clusterctl-upgrade` | CAPI provider upgrade tests |
| `upgrade_kubernetes_test.go` | `k8s-upgrade` | Kubernetes version upgrade tests |
| `remediation_based_feature_test.go` | `remediation` | Machine remediation tests |
| `pivoting_based_feature_test.go` | `pivoting` | Management cluster pivoting |
| `md_remediations_test.go` | `capi-md-tests` | MachineDeployment remediation |
| `k8s_conformance_test.go` | `k8s-conformance` | Kubernetes conformance tests |
| `scalability_test.go` | `scalability` | Scalability tests (50+ nodes) |
| `ip_reuse_test.go` | `features` | IP reuse and other features |

### Environment Variables

**Critical variables for `scripts/ci-e2e.sh`:**

| Variable | Default | Description |
|----------|---------|-------------|
| `CAPM3RELEASEBRANCH` | `main` | CAPM3 branch to test (e.g., `release-1.11`) |
| `IPAMRELEASEBRANCH` | `main` | IPAM branch to test |
| `GINKGO_FOCUS` | (none) | Test label to run (e.g., `basic`, `integration`) |
| `GINKGO_SKIP` | (none) | Test label to skip |
| `IMAGE_OS` | (required) | OS for test clusters (`ubuntu`, `centos`, `opensuse-leap`) |
| `KUBERNETES_VERSION` | (required) | Kubernetes version for workload clusters |
| `NUM_NODES` | `4` | Number of BareMetalHosts to provision |
| `CAPI_NIGHTLY_BUILD` | `false` | Use CAPI nightly builds from main |
| `USE_IRSO` | `false` | Use Ironic standalone mode |

**Derived variables (computed by script):**

- `CAPM3RELEASE` - Computed from `CAPM3RELEASEBRANCH` (e.g., `v1.11.99`)
- `IPAMRELEASE` - Computed from `IPAMRELEASEBRANCH`
- `CAPI_RELEASE_PREFIX` - Computed CAPI version prefix

### Running E2E Tests

**Basic usage:**

```bash
# Run basic integration test with Ubuntu
IMAGE_OS=ubuntu KUBERNETES_VERSION=v1.30.0 GINKGO_FOCUS="basic" ./scripts/ci-e2e.sh

# Run all integration tests with CentOS
IMAGE_OS=centos KUBERNETES_VERSION=v1.30.0 GINKGO_FOCUS="integration" ./scripts/ci-e2e.sh

# Test specific release branch
CAPM3RELEASEBRANCH=release-1.11 IMAGE_OS=ubuntu KUBERNETES_VERSION=v1.30.0 \
  GINKGO_FOCUS="basic" ./scripts/ci-e2e.sh

# Test with CAPI nightly builds
CAPI_NIGHTLY_BUILD=true IMAGE_OS=ubuntu KUBERNETES_VERSION=v1.30.0 \
  GINKGO_FOCUS="basic" ./scripts/ci-e2e.sh
```

**Via Makefile:**

```bash
# Run E2E tests (uses defaults from Makefile)
make test-e2e

# Run ClusterClass E2E tests
make test-clusterclass-e2e
```

### Template Organization

Templates are organized by version to support testing across releases:

**For each version directory** (`main/`, `v1.11/`, etc.):

```text
infrastructure-metal3/main/
├── cluster-template-ubuntu/        # Ubuntu cluster template
├── cluster-template-centos/        # CentOS cluster template
├── cluster-template-opensuse-leap/ # OpenSUSE Leap template
├── cluster-template-upgrade-workload/  # K8s upgrade template
├── cluster-template-centos-md-remediation/  # Remediation template
├── clusterclass-metal3/            # ClusterClass definition
├── clusterclass-template-ubuntu/   # ClusterClass-based cluster
└── clusterclass/                   # ClusterClass for topology
```

**Generated templates:**

```bash
# Generate all templates for all versions
make cluster-templates

# Generate ClusterClass templates
make clusterclass-templates

# Generate specific version
make cluster-templates-main
make cluster-templates-v1.11
```

Output goes to: `test/e2e/_out/{version}/`

### Release Branch E2E Updates

When creating a new release branch, E2E configuration must be updated.
See [docs/releasing.md](./docs/releasing.md) for the complete checklist.

Key areas that need updates:

- Template directories in `test/e2e/data/infrastructure-metal3/`
- Provider versions in `test/e2e/config/e2e_conf.yaml`
- Makefile template generation targets
- Prow job configurations in project-infra

### IPAM and BMO Testing

E2E tests in CAPM3 validate IPAM and BMO by:

**IPAM Testing:**

- IPPool and IPClaim resources created automatically
- Metal3DataTemplate references IPPools for IP allocation
- Tests verify IP assignment and reuse
- IPAM provider loaded from: `quay.io/metal3-io/ip-address-manager:latest`

**BMO Testing:**

- BareMetalHost CRDs deployed from BMO repository
- Tests verify host provisioning, deprovisioning, inspection
- BMO provider loaded from: `quay.io/metal3-io/baremetal-operator:latest`
- Deployment overlays in `test/e2e/data/bmo-deployment/`

**Image loading (see `e2e_conf.yaml`):**

```yaml
images:
- name: quay.io/metal3-io/cluster-api-provider-metal3:latest
  loadBehavior: mustLoad
- name: quay.io/metal3-io/baremetal-operator:latest
  loadBehavior: mustLoad
- name: quay.io/metal3-io/ip-address-manager:latest
  loadBehavior: mustLoad
```

### E2E Test Flow

1. **Environment Setup** (`scripts/ci-e2e.sh`):
   - Clone metal3-dev-env
   - Configure dev-env based on `GINKGO_FOCUS`
   - Set up networking, Ironic, BMO

2. **Bootstrap** (`metal3-dev-env make`):
   - Create kind management cluster
   - Deploy Ironic
   - Deploy BMO
   - Create BareMetalHost resources

3. **Test Execution** (`make e2e-tests`):
   - Load container images (CAPM3, BMO, IPAM)
   - Install CAPI providers via clusterctl
   - Run Ginkgo test suites based on label filter
   - Collect logs on failure

4. **Validation**:
   - Workload cluster creation/deletion
   - Machine provisioning via BMH
   - IP allocation via IPAM
   - Upgrades (CAPI, Kubernetes)
   - Remediation flows

### Debugging E2E Failures

**Check Jenkins logs:**

- <https://jenkins.nordix.org/view/Metal3%20Periodic/>
- Look for E2E job artifacts

**Run locally:**

```bash
# Match CI configuration exactly
IMAGE_OS=centos KUBERNETES_VERSION=v1.30.0 GINKGO_FOCUS="basic" \
  ./scripts/ci-e2e.sh

# Enable verbose logging
export GINKGO_NOCOLOR=false
```

**Common issues:**

- Template version mismatch - check `test/e2e/data/infrastructure-metal3/`
- Provider version incompatibility - check `e2e_conf.yaml`
- BareMetalHost unavailable - check `NUM_NODES` and metal3-dev-env logs
- Timeout - check `intervals` in `e2e_conf.yaml`

**Artifacts location:**

- `_artifacts/` directory in repo root
- Contains cluster logs, resource dumps on failure

### E2E vs Unit Tests

| Aspect | Unit Tests | E2E Tests |
|--------|------------|-----------|
| **Scope** | Controllers, IPAM logic, webhooks | Full cluster lifecycle |
| **Dependencies** | Mocked (envtest) | Real (BMO, Ironic, IPAM) |
| **Duration** | Seconds-minutes | Hours |
| **CI Trigger** | Every PR | Periodic + manual |
| **Env** | Local + CI | metal3-dev-env |
| **Cost** | Low | High (VMs, time) |

---

### Webhooks

Located in `internal/webhooks/`:

- Validation webhooks for all resources
- Defaulting logic for optional fields
- Immutability checks for sensitive fields
- Webhook tests in `*_webhook_test.go`

---

## Integration Points

### With Cluster API

- CAPM3 registers as infrastructure provider via `clusterctl`
- Implements InfrastructureCluster and InfrastructureMachine contracts
- Responds to CAPI machine lifecycle events (create, update, delete)
- Sets machine providerID once node joins cluster
- Follows CAPI versioning and release cadence

**Compatibility matrix:**

- Check README.md for current CAPM3 ↔ CAPI version compatibility table
- Current stable API version: v1beta1
- CAPM3 follows CAPI versioning and release cadence

### With Baremetal Operator

- Reads BareMetalHost CRD from BMO
- Sets `spec.consumerRef` to claim hosts
- Writes `spec.image`, `spec.userData`, `spec.networkData` to provision
- Monitors `status.provisioning.state` for provisioning progress
- Releases hosts on machine deletion by clearing consumerRef
- Never talks to Ironic directly - all hardware operations via BMO

### With IP Address Manager (IPAM)

- Optional integration for IP allocation
- Metal3DataTemplate can reference IPPool, IPClaim resources
- Automatic IP allocation for machines when IPPool configured
- IPAM is a separate project: cluster-api-ipam-provider-metal3

## Key Files Reference

### Core Files

- `main.go` - Entrypoint, controller registration, manager setup
- `Makefile` - Primary build/test interface (ALWAYS check this first)
- `metadata.yaml` - Provider metadata for clusterctl
- `clusterctl-settings.json` - ClusterCTL provider configuration

### Configuration

- `config/default/` - Default deployment manifests
- `config/crd/bases/` - Generated CRDs
- `config/rbac/` - Generated RBAC rules
- `config/webhook/` - Webhook configurations
- `.markdownlint-cli2.yaml` - Markdown linting config
- `.golangci.yml` - Go linting configuration (if present)

### Scripts

- `scripts/ci-e2e.sh` - E2E orchestration script
- `scripts/environment.sh` - Environment setup for CI
- `hack/` - All development/CI scripts

### Examples

- `examples/clusterctl-templates/` - Example cluster templates
- `examples/generate.sh` - Generate example manifests

### Documentation

- `README.md` - Project overview and quick start
- `CONTRIBUTING.md` - Contribution guidelines (DCO, PR process)
- `docs/` - Additional documentation
- `docs/api.md` - API documentation (update when APIs change)
- `docs/releasing.md` - Release process

## Common Workflows

### Making Code Changes

1. **Understand the change:**
   - Read relevant code first
   - Check existing tests
   - Review related issues/PRs
2. **Make changes:**
   - Follow existing patterns
   - Add/update tests
   - Update documentation if needed
3. **Verify changes:**
   - Run `make generate` if APIs changed
   - Run `make test` (lint + unit tests)
   - Run `make verify` (boilerplate + modules)
4. **Commit with DCO sign-off:**
   - `git commit -s -m "descriptive message"`
   - Ensure Apache 2.0 headers on new files

### Adding New API Fields

1. Edit `api/v1beta1/*_types.go`
2. Run `make generate` (generates code + manifests)
3. Run `make verify-modules` (check go.mod consistency)
4. Update `docs/api.md`
5. Add tests for new field behavior
6. Run `make test`

### Deploying a Cluster

```bash
# Initialize management cluster with CAPM3
clusterctl init --infrastructure metal3

# Generate cluster manifest from template
# Check README.md for supported Kubernetes versions
clusterctl generate cluster my-cluster \
  --kubernetes-version v1.30.0 \
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

1. **Namespace Mismatch** - All resources (Cluster, Machine, Metal3Machine,
   BareMetalHost) must be in same namespace
2. **Missing BareMetalHosts** - Metal3Machine stuck in "Pending" if no
   available BMHs match selector
3. **DataTemplate Errors** - Check Metal3Data status for rendering errors
   (missing variables, syntax issues)
4. **Pivoting Complexity** - Ensure Ironic accessible from target cluster,
   BMH status annotations preserved
5. **Forgot to run generate** - Always run `make generate` after API changes
6. **Owner References** - Metal3Machine should own Metal3Data, Machine owns
   Metal3Machine
7. **Finalizers** - Use `metal3machine.infrastructure.cluster.x-k8s.io` to
   ensure cleanup
8. **Cluster Infrastructure Ready** - Metal3Cluster must be Ready before
   machines reconcile
9. **DCO sign-off missing** - All commits require `-s` flag for sign-off
10. **Module inconsistency** - Run `make verify-modules` after dependency changes

## Version Support and Compatibility

### Current Support Status

CAPM3 maintains the two most recent minor releases for the current stable API
version (v1beta1).

See [CONTRIBUTING.md](./CONTRIBUTING.md) for:

- Current support matrix with specific version numbers
- Backporting policy
- EOL dates for older releases

### Kubernetes Version Support

CAPM3 supports Kubernetes versions aligned with CAPI support matrix.
Check compatibility table in README.md.

### Go Version

- Check Makefile for `GO_VERSION` variable for current version
- Minimum version enforced by `hack/ensure-go.sh`
- Build container images use `docker.io/golang:${GO_VERSION}`

## Git and Release Information

- **Branches**: `main` (development), `release-X.Y` (stable releases)
- **DCO Required**: All commits must be signed off (`git commit -s`)
- **PR Labels**: ⚠️ breaking, ✨ feature, 🐛 bug, 📖 docs, 🌱 other
- **Release Process**: See [docs/releasing.md](./docs/releasing.md)

## Additional Resources

### Documentation Links

- [Cluster API](https://cluster-api.sigs.k8s.io/) - Upstream project
  documentation
- [Metal3 Docs](https://metal3.io/documentation.html) - Metal3 project
  documentation
- [Baremetal Operator](https://github.com/metal3-io/baremetal-operator) - BMO
  repository
- [IPAM Provider](https://github.com/metal3-io/ip-address-manager) - Separate
  IPAM project
- [Metal3 Dev Env](https://github.com/metal3-io/metal3-dev-env) - Development
  environment

### Issue Tracking

- File issues: <https://github.com/metal3-io/cluster-api-provider-metal3/issues>
- Good first issues: <https://github.com/metal3-io/cluster-api-provider-metal3/issues?q=is%3Aopen+is%3Aissue+label%3A%22good+first+issue%22>

---

## AI Agent Behavioral Guidelines

### Critical Rules

1. **Use single commands** - Do not concatenate multiple commands with `&&` or
   `;` to avoid interactive permission prompts from the user. Run one command
   at a time.

2. **Be strategic with output filtering** - Use `head`, `tail`, or `grep` when
   output is clearly excessive (e.g., large logs), but prefer full output for
   smaller results to avoid missing context.

3. **Challenge assumptions** - Do not take user statements as granted. If you
   have evidence against an assumption, present it respectfully with file
   references and line numbers.

4. **Search for latest versions** - When suggesting dependencies, libraries,
   or tools, always verify and use the latest stable versions.

5. **Security first** - Take security very seriously. Review code for:
   - Hardcoded credentials or sensitive data
   - Insecure defaults
   - Missing input validation
   - Command injection vulnerabilities
   - Privilege escalation risks
   - OWASP Top 10 vulnerabilities

6. **Pin dependencies by SHA** - All external dependencies must be SHA pinned
   when possible (container images, GitHub Actions, downloaded binaries).
   This prevents supply chain attacks and ensures reproducible builds.

7. **Provide references** - Back up suggestions with links to documentation,
   issues, PRs, or code examples from the repository. Include file paths and
   line numbers when referencing code.

8. **Follow existing conventions** - Match the style of existing code:
   - Shell scripts: use patterns from `hack/` scripts
   - Go code: follow golangci-lint rules (check `.golangci.yaml` if present)
   - Markdown: follow `.markdownlint-cli2.yaml` rules
   - License headers: use templates from `hack/boilerplate/`
   - YAML: use 2-space indentation for Kubernetes manifests

### Before Making Changes

1. Run `make lint` to understand current linting rules
2. Run `make unit` to verify baseline test status
3. Check existing patterns in similar files
4. Verify Go version matches `Makefile` `GO_VERSION` variable
5. Read the code you're about to modify - never propose changes to unread code

### When Modifying Code

1. Make minimal, surgical changes
2. Run `make generate` after API changes
3. Run `make test` before submitting
4. Update documentation if behavior changes
5. Add tests for new functionality
6. Ensure all modified modules are tested (root, api, test, webhooks)

### When Debugging CI Failures

1. Check [Prow job definitions](https://github.com/metal3-io/project-infra/tree/main/prow/config/jobs/metal3-io)
2. Run the same hack script locally with `IS_CONTAINER=TRUE`
3. Use the exact container images from CI (check Prow config)
4. Check if failure is flaky vs consistent
5. Review recent commits for potential causes

### Commit Guidelines

- Sign commits with `-s` flag (DCO required)
- Use conventional commit emoji prefixes:
   - ✨ `:sparkles:` - New feature
   - 🐛 `:bug:` - Bug fix
   - 📖 `:book:` - Documentation
   - 🌱 `:seedling:` - Other changes
   - ⚠️ `:warning:` - Breaking changes
- Write descriptive commit messages explaining the "why"
- Include Apache 2.0 license headers on new files

### Release Process Reference

See [docs/releasing.md](./docs/releasing.md) for full process. Key points:

- Use `hack/verify-release.sh` to check release readiness
- Release notes go in `releasenotes/`
- Tags follow semver: `v1.x.y`
- Always verify working tree is clean before releasing

---

## Quick Reference Card

```bash
# Most common commands
make test              # Full verification (most common)
make unit              # Unit tests only
make lint              # Linting only
make generate          # Regenerate code and manifests
make modules           # Tidy go modules
make manager           # Build binary
make docker-build      # Build container

# Verification
make verify            # All checks
make verify-boilerplate # License headers
make verify-modules    # go.mod tidy

# Local development
make kind-create       # Create test cluster
make tilt-up           # Start Tilt for live development
make deploy            # Deploy to cluster
make deploy-examples   # Deploy test resources

# E2E Testing
make test-e2e          # Run E2E tests
GINKGO_FOCUS="basic" ./scripts/ci-e2e.sh  # Run specific scenario

# Hack scripts (containerized)
./hack/unit.sh         # Unit tests
./hack/codegen.sh      # Verify codegen
./hack/gomod.sh        # Verify modules
./hack/shellcheck.sh   # Shell linting
./hack/markdownlint.sh # Markdown linting
./hack/manifestlint.sh # K8s manifest linting

# Git workflow
git commit -s          # Commit with DCO sign-off

# Release verification
RELEASE_TAG=v1.x.y ./hack/verify-release.sh
```

---

**Remember:** CAPM3 is a Cluster API infrastructure provider for bare metal.
Always verify assumptions against the actual code, run tests after changes,
and follow the existing conventions in the codebase.
