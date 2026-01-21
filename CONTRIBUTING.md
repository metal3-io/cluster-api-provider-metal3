# How to Contribute to Cluster API Provider Metal3

> **Note**: Please read the [common Metal3 contributing guidelines](https://github.com/metal3-io/community/blob/main/CONTRIBUTING.md)
> first. This document contains cluster-api-provider-metal3-specific information.

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->

- [Versioning](#versioning)
- [Branches](#branches)
   - [Support and Guarantees](#support-and-guarantees)
- [Backporting](#backporting)
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

### Support and Guarantees

Cluster API Provider Metal3 maintains the most recent release/releases for all
supported API and contract versions. Support for this section refers to the
ability to backport and release patch versions; [backport policy](#backporting)
is defined below.

- The API version is determined from the GroupVersion defined in the top-level
  `api/` package.
- The EOL date of each API Version is determined from the last release available
  once a new API version is published.

<!-- markdownlint-disable MD013 -->

| API Version  | Supported Until      |
| ------------ | ---------------------|
| **v1beta1**  | TBD (current latest) |
| **v1alpha5** | EOL since 2022-09-30 |

<!-- markdownlint-enable MD013 -->

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

| Minor Release | API Version  | Supported Until                              |
| ------------- | ------------ | -------------------------------------------- |
| v1.10.x       | **v1beta1**  | when v1.12.0 will be released                |
| v1.9.x        | **v1beta1**  | when v1.11.0 will be released                |
| v1.8.x        | **v1beta1**  | EOL since 2025-04-30                         |
| v1.7.x        | **v1beta1**  | EOL since 2024-12-19                         |
| v1.6.x        | **v1beta1**  | EOL since 2024-09-04                         |
| v1.5.x        | **v1beta1**  | EOL since 2024-04-18                         |
| v1.4.x        | **v1beta1**  | EOL since 2024-01-10                         |
| v1.3.x        | **v1beta1**  | EOL since 2023-09-27                         |
| v1.2.x        | **v1beta1**  | EOL since 2023-05-17 (*)                     |
| v1.1.x        | **v1beta1**  | EOL since 2023-05-17 (*)                     |
| v0.5.x        | **v1alpha4** | EOL since 2022-09-30 - API version EOL (*)   |
| v0.4.x        | **v1alpha3** | EOL since 2022-02-23 - API version EOL (*)   |

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

## Release Process

See the [common release process guidelines](https://github.com/metal3-io/community/blob/main/CONTRIBUTING.md#release-process)
in the Metal3 community contributing guide.

For exact release steps specific to CAPM3, refer to the [releasing document](./docs/releasing.md).
