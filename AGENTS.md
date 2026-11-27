# Cluster API Provider Metal3 (CAPM3)

Cluster API infrastructure provider that turns `BareMetalHost`s managed
by the Baremetal Operator into Kubernetes cluster nodes.
Project overview: [README.md](README.md). Contribution rules:
[CONTRIBUTING.md](CONTRIBUTING.md) and the
[Metal3 community guide](https://github.com/metal3-io/community/blob/main/CONTRIBUTING.md).

This file holds only what you cannot cheaply infer from the code. Where it
disagrees with the code or the Makefile, trust those and report the
mismatch.

## AI-assisted contributions

Metal3 follows the CNCF and
[Linux Foundation generative AI policy](https://www.linuxfoundation.org/legal/generative-ai).

- AI is a tool. The human who opens a PR is its author and is responsible
  for every line of it, including licensing and third-party content.
- Review is a conversation between humans. A PR where an agent or bot
  answers reviewers will be closed. Agents may draft changes; the
  contributor reads them, understands them, and replies personally.

- Keep changes surgical. The smallest complete change that solves the
  stated problem wins. Every added file, function, flag, dependency or
  test helper must trace to that problem; anything that does not is
  bloat and gets removed. Prefer reusing existing code over new code.
  Report unrelated findings instead of fixing them in the same PR.

## Repository map

Five Go modules (root, `api/`, `test/`, `hack/tools/`,
`hack/fake-apiserver/`). Root `go test ./...` does not cover the other
four. Tidy them with `make modules`, not `go mod tidy`; `hack/gomod.sh`
fails on any diff.

- Controllers in `controllers/` are thin; the logic lives in the
  managers in `baremetal/`, which `controllers/` call through interfaces
  mocked in `baremetal/mocks/`.
- Webhooks: `internal/webhooks/v1beta2/`.
- The E2E suite in `test/e2e/` is shared: Jenkins E2E jobs of other
  Metal3 repos (for example `ironic-ipa-downloader`, `project-infra`,
  IPAM and ironic-image release branches) clone this repo and run
  `make test-e2e`. A change there affects their CI too.

## Testing

- Targets: `make help` and [docs/testing.md](docs/testing.md). E2E:
  [docs/e2e-test.md](docs/e2e-test.md). CI jobs:
  [Prow](https://github.com/metal3-io/project-infra/blob/main/prow/config/jobs/metal3-io/cluster-api-provider-metal3.yaml)
  and `.github/workflows/`.
- Conventions are whatever the existing code, tests, and lint configs do.
  Read neighboring files and `_test.go` files before writing new ones.
- Before finishing, add or update tests for every behavior change, then
  run `make test` and the `hack/` checks for the file types you touched.
  All must pass.
- `controllers/` and `baremetal/` tests need envtest. `make unit` sets it
  up; for a targeted `go test`, export the `KUBEBUILDER_ASSETS` path that
  `make setup-envtest` prints.
- Never hand-edit generated output (`zz_generated.*.go`, including the
  mocks, and the CRD, RBAC and webhook manifests under `config/`). Change
  the source and run `make generate`. The `generate` CI job fails on any
  diff.
- Do not touch `releasenotes/` outside a release PR. `make release-notes`
  generates them from PR titles; only release PRs curate them by hand
  ([docs/releasing.md](docs/releasing.md)).
- PR titles start with `:warning:` (breaking), `:sparkles:` (feature),
  `:bug:` (fix), `:book:` (docs) or `:seedling:` (other). The release
  notes tool sorts PRs by this prefix.
- Do not run `make test-e2e`, `make test-clusterclass-e2e`,
  `scripts/ci-e2e.sh`, `make clean-e2e`, or `hack/ensure-*.sh`,
  `hack/install-go.sh` and `hack/setup-bml.sh` unless the user asks: they
  install packages with `sudo`, install Go into `/usr/local/go`, and
  create libvirt VMs, networks and kind clusters on the host. E2E runs in
  Jenkins when a reviewer requests it with `/test`.

## Invariants

These apply to writing and to reviewing changes.

- **API versions.** All kinds exist in `v1beta1` (served, conversion
  spoke) and `v1beta2` (storage version, conversion hub). A field change
  needs both types, the conversion functions in
  `api/v1beta1/conversion.go`, and a passing `TestFuzzyConversion`
  round-trip. Webhooks exist only for `v1beta2`.
- **API compatibility.** Never remove or rename a served field, and never
  tighten validation on existing fields, without a deprecation path.
  Mark such PRs as breaking. `.golangci-kal.yaml` (kube-api-linter, run
  by `make lint` and the `kube-api-lint` CI job) enforces API field
  conventions in `api/`.
- **Cluster API contract.** CAPM3 implements the Cluster API
  infrastructure provider contract. `metadata.yaml` maps each release
  series to its contract version; a new minor release or contract bump
  must update it.
- **Dependent APIs.** CAPM3 consumes the BMO and IPAM APIs as released
  Go modules (`go.mod`, `api/go.mod`). A change that needs a new BMO or
  IPAM field has to wait for, or bump to, a release that has it.
- **Security.** Never weaken security to make something work: no
  disabled TLS verification, broadened RBAC, secrets in code, logs or
  fixtures, or skipped validation. Treat resource specs, user-supplied
  templates, image URLs and data read from workload clusters as
  untrusted input. Call out any change to authentication, TLS, RBAC,
  Secrets or webhooks in the PR description.
- **Tenancy and isolation.** One CAPM3 serves many namespaces, watching
  all of them or the one given by `--namespace`, and its manager role can
  read Secrets cluster-wide. Review these areas with extra care:
   - Cross-namespace access. A reference must never let one namespace
     read or modify another namespace's hosts, Secrets or data. Hosts are
     selected only in the `Metal3Machine`'s namespace, and secret
     references must stay in the referencing object's namespace (see the
     `userData` check in `baremetal/metal3machine_manager.go`). Treat any
     `namespace` field in a reference (`Metal3ObjectRef`,
     `SecretReference`) as a possible cross-namespace path.
   - Role isolation. Grant RBAC with `+kubebuilder:rbac` markers on the
     controller that needs it, then `make generate`. Do not broaden the
     manager role beyond what the change needs.
   - Multi-tenancy. CAPM3 writes to workload clusters through their
     kubeconfig Secrets (`baremetal/remote/`, label sync). Data from one
     cluster or namespace must never reach another, and status, events,
     logs and metrics must not leak it.
- **Pinning.** Pin new external references: GitHub Actions by commit SHA,
  container images by digest, downloaded binaries by checksum.

## External references

- [Metal3 Book, CAPM3 user guide](https://book.metal3.io/capm3/introduction)
- [CAPM3 design proposals](https://github.com/metal3-io/metal3-docs/tree/main/design/cluster-api-provider-metal3)
- [Metal3 API design principles](https://github.com/metal3-io/metal3-docs/blob/main/design/api-design-principles.md)
- [Cluster API provider contracts](https://cluster-api.sigs.k8s.io/developer/providers/contracts/overview)
