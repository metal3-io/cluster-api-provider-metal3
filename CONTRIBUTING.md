# How to Contribute to Cluster API Provider Metal3

> **Note**: Please read the [common Metal3 contributing guidelines](https://github.com/metal3-io/community/blob/main/CONTRIBUTING.md)
> first. This document contains Cluster API Provider Metal3 specific information.

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->

- [Versioning](#versioning)
- [Branches](#branches)
   - [Maintenance and Guarantees](#maintenance-and-guarantees)
- [Backporting](#backporting)
- [Logging Guidelines](#logging-guidelines)
   - [Verbosity Levels](#verbosity-levels)
   - [Structured Logging Fields](#structured-logging-fields)
   - [Best Practices](#best-practices)
- [Release Process](#release-process)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

## Versioning

> :warning: The project does not follow Go Modules guidelines for compatibility
> requirements for 1.x semver releases. Cluster API Provider Metal3 follows
> Cluster API release cadence and versioning which follows upstream Kubernetes
> semantic versioning.

For general versioning semantics (what goes in minor vs patch releases), see the
[Versioning and Release Semantics](https://github.com/metal3-io/community/blob/main/CONTRIBUTING.md#versioning-and-release-semantics)
section in the common contributing guide.

**Note**: The test module and experiments do not provide any backward
compatible guarantees.

## Branches

For general branch structure, see the [Branches](https://github.com/metal3-io/community/blob/main/CONTRIBUTING.md#branches)
section in the common contributing guide.

### Maintenance and Guarantees

Cluster API Provider Metal3 maintains the most recent release/releases for all
supported API and contract versions. Support for this section refers to the
ability to backport and release patch versions; [backport policy](#backporting)
is defined below.

- The API version is determined from the GroupVersion defined in the top-level
  `api/` package.
- The EOL date of each API Version is determined from the last release available
  once a new API version is published.

- For the current stable API version (v1beta1) we support the two most recent
  minor releases; older minor releases are immediately unsupported when a new
  major/minor release is available.
- For older API versions we only support the most recent minor release until the
  API version reaches EOL.
- We will maintain test coverage for all supported minor releases and for one
  additional release for the current stable API version in case we have to do an
  emergency patch release. For example, if v1.7 and v1.6 are currently
  supported, we will also maintain test coverage for v1.5 for one additional
  release cycle. When v1.7 is released, tests for v1.4 will be removed.

<!-- markdownlint-disable MD013 -->

| Minor Release | Supported Until                              |
| ------------- | -------------------------------------------- |
| v1.12.x       | when v1.14.0 will be released                |
| v1.11.x       | when v1.13.0 will be released                |
| v1.10.x       | EOL since 2025-12-19                         |
| v1.9.x        | EOL since 2025-09-15                         |
| v1.8.x        | EOL since 2025-04-30                         |
| v1.7.x        | EOL since 2024-12-19                         |
| v1.6.x        | EOL since 2024-09-04                         |
| v1.5.x        | EOL since 2024-04-18                         |
| v1.4.x        | EOL since 2024-01-10                         |
| v1.3.x        | EOL since 2023-09-27                         |
| v1.2.x        | EOL since 2023-05-17 (*)                     |
| v1.1.x        | EOL since 2023-05-17 (*)                     |
| v0.5.x        | EOL since 2022-09-30 - API version EOL (*)   |
| v0.4.x        | EOL since 2022-02-23 - API version EOL (*)   |

<!-- markdownlint-enable MD013 -->

(*) Previous support policy applies, older minor releases were immediately
unsupported when a new major/minor release was available

- Exceptions can be filed with maintainers and taken into consideration on a
  case-by-case basis.

## Backporting

See the [common backporting guidelines](https://github.com/metal3-io/community/blob/main/CONTRIBUTING.md#backporting)
in the Metal3 community contributing guide.

We generally do not accept backports to CAPM3 release branches
that are out of support. Check the [Version support](https://github.com/metal3-io/metal3-docs/blob/main/docs/user-guide/src/version_support.md)
guide for reference.

## Logging Guidelines

This project uses structured logging with defined verbosity levels. When adding
or modifying log statements, follow these conventions:

### Verbosity Levels

| Level | Constant | Usage |
|-------|----------|-------|
| Error | `.Error()` | Actual errors requiring attention |
| Info | `.Info()` | Important operational events, major state changes |
| Debug | `.V(4)` / `VerbosityLevelDebug` | Internal state, fetched objects, intermediate results |
| Trace | `.V(5)` / `VerbosityLevelTrace` | Function entry/exit, detailed flow tracing |

### Structured Logging Fields

Use the predefined `LogField*` constants from `baremetal/utils.go` for
consistent field names:

```go
log.V(baremetal.VerbosityLevelDebug).Info("Processing machine",
    baremetal.LogFieldMachine, machine.Name,
    baremetal.LogFieldNamespace, machine.Namespace)
```

Common fields include: `LogFieldMachine`, `LogFieldMetal3Machine`,
`LogFieldCluster`, `LogFieldBMH`, `LogFieldError`, `LogFieldPhase`, etc.

### Best Practices

- **Trace level**: Use for entering/exiting functions and processing individual
  items
- **Debug level**: Use for state checks, configuration values, and successful
  operations
- **Info level**: Use sparingly for user-facing operational events
- **Error level**: Always include the error and relevant context
- **Context**: Populate logger with NamespacedName at the start of Reconcile
  functions

## Release Process

See the [common release process guidelines](https://github.com/metal3-io/community/blob/main/CONTRIBUTING.md#release-process)
in the Metal3 community contributing guide.

For exact release steps specific to CAPM3, refer to the [releasing document](./docs/releasing.md).
