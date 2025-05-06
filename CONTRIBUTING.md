# How to Contribute

Metal3 projects are [Apache 2.0 licensed](LICENSE) and accept contributions via
GitHub pull requests. Those guidelines are the same as the
[Cluster API guidelines](https://github.com/kubernetes-sigs/cluster-api/blob/master/CONTRIBUTING.md)

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->

- [How to Contribute](#how-to-contribute)
   - [Certificate of Origin](#certificate-of-origin)
      - [Git commit Sign-off](#git-commit-sign-off)
   - [Finding Things That Need Help](#finding-things-that-need-help)
   - [Versioning](#versioning)
      - [Codebase and Go Modules](#codebase-and-go-modules)
         - [Backporting](#backporting)
   - [Branches](#branches)
      - [Support and guarantees](#support-and-guarantees)
      - [Removal of v1alpha5 apiVersion](#removal-of-v1alpha5-apiversion)
   - [Contributing a Patch](#contributing-a-patch)
   - [Backporting a Patch](#backporting-a-patch)
   - [Breaking Changes](#breaking-changes)
      - [Merge Approval](#merge-approval)
      - [Google Doc Viewing Permissions](#google-doc-viewing-permissions)
      - [Issue and Pull Request Management](#issue-and-pull-request-management)
      - [Commands and Workflow](#commands-and-workflow)
   - [Release Process](#release-process)
      - [Exact Steps](#exact-steps)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

## Certificate of Origin

By contributing to this project you agree to the Developer Certificate of Origin
(DCO). This document was created by the Linux Kernel community and is a simple
statement that you, as a contributor, have the legal right to make the
contribution. See the [DCO](DCO) file for details.

### Git commit Sign-off

Commit message should contain signed off section with full name and email. For example:

 ```text
  Signed-off-by: John Doe <jdoe@example.com>
 ```

When making commits, include the `-s` flag and `Signed-off-by` section
will be automatically added to your commit message. If you want GPG
signing too, add the `-S` flag alongside `-s`.

```bash
  # Signing off commit
  git commit -s

  # Signing off commit and also additional signing with GPG
  git commit -s -S
```

## Finding Things That Need Help

If you're new to the project and want to help, but don't know where to start, we
have a semi-curated list of issues that should not need deep knowledge of the
system.
[Have a look and see if anything sounds interesting](https://github.com/metal3-io/cluster-api-provider-metal3/issues?q=is%3Aopen+is%3Aissue+label%3A%22good+first+issue%22).
Alternatively, read some of the docs on other controllers and try to write your
own, file and fix any/all issues that come up, including gaps in documentation!

## Versioning

### Codebase and Go Modules

> :warning: The project does not follow Go Modules guidelines for compatibility
> requirements for 1.x semver releases. Cluster API Provider Metal3 follows
> Cluster API release cadence and versioning which follows upstream Kubernetes
> semantic versioning. With the v1 release of our codebase, we guarantee the
> following:

- A (_minor_) release CAN include:

   - Introduction of new API versions, or new Kinds.
   - Compatible API changes like field additions, deprecation notices, etc.
   - Breaking API changes for deprecated APIs, fields, or code.
   - Features, promotion or removal of feature gates.
   - And more!

- A (_patch_) release SHOULD only include backwards compatible set of bugfixes.

These guarantees extend to all code exposed in our Go Module, including _types
from dependencies in public APIs_. Types and functions not in public APIs are
not considered part of the guarantee. The test module, and experiments do not
provide any backward compatible guarantees.

#### Backporting

We generally do not accept PRs directly against release branches, while we might
accept backports of fixes/changes already merged into the main branch.

We generally allow backports of following changes to all supported branches:

- Critical bug fixes, security issue fixes, or fixes for bugs without easy
workarounds.
- Dependency bumps for CVE (usually limited to CVE resolution; backports of
non-CVE related version bumps are considered exceptions to be evaluated case by
case)
- Changes required to support new Kubernetes patch versions, when possible.
- Changes to use the latest Go patch version to build controller images.
- Changes to bump the Go minor version used to build controller images, if the
Go minor version of a supported branch goes out of support (e.g. to pick up
bug and CVE fixes). This has no impact on users importing Cluster API Provider
Metal3 as we won't modify the version in go.mod and the version in the Makefile
does not affect them.
- Improvements to existing docs
- Improvements to the test framework

Like any other activity in the project, backporting a fix/change is a
community-driven effort and requires that someone volunteers to own the task.
In most cases, the cherry-pick bot can (and should) be used to automate
opening a cherry-pick PR.

We generally do not accept backports to CAPM3 release branches that are out of
support. Check the [Version support](https://github.com/metal3-io/metal3-docs/blob/main/docs/user-guide/src/version_support.md)
guide for reference.

## Branches

Cluster API Provider Metal3 has two types of branches: the _main_ branch and
_release-X_ branches.

The _main_ branch is where development happens. All the latest and greatest
code, including breaking changes, happens on main.

The _release-X_ branches contain stable, backwards compatible code. On every
major or minor release, a new branch is created. It is from these branches that
minor and patch releases are tagged. In some cases, it may be necessary to open
PRs for bugfixes directly against stable branches, but this should generally not
be the case.

### Support and guarantees

Cluster API Provider Metal3 maintains the most recent release/releases for all
supported API and contract versions. Support for this section refers to the
ability to backport and release patch versions; [backport policy](#backporting)
is defined above.

- The API version is determined from the GroupVersion defined in the top-level
  `api/` package.
- The EOL date of each API Version is determined from the last release available
  once a new API version is published.

<!-- markdownlint-disable MD013 -->

| API Version  | Supported Until                                                              |
| ------------ | ---------------------------------------------------------------------------- |
| **v1beta1**  | TBD (current latest)                                                         |
| **v1alpha5** | EOL since 2022-09-30 ([apiVersion removal](#removal-of-v1alpha5-apiversion)) |

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
| v1.2.x        | **v1beta1**  | EOL since 2023-05-17                         |
| v1.1.x        | **v1beta1**  | EOL since 2023-05-17                         |
| v0.5.x        | **v1alpha4** | EOL since 2022-09-30 - API version EOL       |
| v0.4.x        | **v1alpha3** | EOL since 2022-02-23 - API version EOL       |

<!-- markdownlint-enable MD013 -->

(\*) Previous support policy applies, older minor releases were immediately
unsupported when a new major/minor release was available

- Exceptions can be filed with maintainers and taken into consideration on a
  case-by-case basis.

### Removal of v1alpha5 apiVersion

We are going to remove the v1alpha5 apiVersion in upcoming releases:

- v1.7
   - v1alpha5 apiVersion will be removed from the CRDs

For more details and latest information please see the following issue:
[Removing v1alpha5 apiVersion](https://github.com/metal3-io/cluster-api-provider-metal3/issues/971).

## Contributing a Patch

1. If you haven't already done so, sign a Contributor License Agreement (see
   details above).
1. Fork the desired repo, develop and test your code changes.
1. Submit a pull request.

All code PR must be labeled with one of

- ⚠️ (`:warning:`, major or breaking changes)
- ✨ (`:sparkles:`, feature additions)
- 🐛 (`:bug:`, patch and bugfixes)
- 📖 (`:book:`, documentation or proposals)
- 🌱 (`:seedling:`, minor or other)

Individual commits should not be tagged separately, but will generally be
assumed to match the PR. For instance, if you have a bugfix in with a breaking
change, it's generally encouraged to submit the bugfix separately, but if you
must put them in one PR, mark the commit separately.

All changes must be code reviewed. Coding conventions and standards are
explained in the official
[developer docs](https://github.com/kubernetes/community/tree/master/contributors/devel).
Expect reviewers to request that you avoid common
[go style mistakes](https://github.com/golang/go/wiki/CodeReviewComments) in
your PRs.

## Backporting a Patch

Cluster API Provider Metal3 maintains older versions through `release-X.Y`
branches. We accept backports of bug fixes to the most recent release branch.
For example, if the most recent branch is `release-0.2`, and the `main` branch
is under active development for v0.3.0, a bug fix that merged to `main` that
also affects `v0.2.x` may be considered for backporting to `release-0.2`. We
generally do not accept PRs against older release branches.

## Breaking Changes

Breaking changes are generally allowed in the `main` branch, as this is the
branch used to develop the next minor release of Cluster API.

There may be times, however, when `main` is closed for breaking changes. This is
likely to happen as we near the release of a new minor version.

Breaking changes are not allowed in release branches, as these represent minor
versions that have already been released. These versions have consumers who
expect the APIs, behaviors, etc. to remain stable during the life time of the
patch stream for the minor release.

Examples of breaking changes include:

- Removing or renaming a field in a CRD
- Removing or renaming a CRD
- Removing or renaming an exported constant, variable, type, or function
- Updating the version of critical libraries such as controller-runtime,
  client-go, apimachinery, etc.
- Some version updates may be acceptable, for picking up bug fixes, but
  maintainers must exercise caution when reviewing.

There may, at times, need to be exceptions where breaking changes are allowed in
release branches. These are at the discretion of the project's maintainers, and
must be carefully considered before merging. An example of an allowed breaking
change might be a fix for a behavioral bug that was released in an initial minor
version (such as `v0.3.0`).

### Merge Approval

Please see the
[Kubernetes community document on pull requests](https://git.k8s.io/community/contributors/guide/pull-requests.md)
for more information about the merge process.

### Google Doc Viewing Permissions

To gain viewing permissions to google docs in this project, please join the
[metal3-dev](https://groups.google.com/forum/#!forum/metal3-dev) google group.

### Issue and Pull Request Management

Anyone may comment on issues and submit reviews for pull requests. However, in
order to be assigned an issue or pull request, you must be a member of the
[Metal3-io organization](https://github.com/metal3-io) GitHub organization.

Metal3 maintainers can assign you an issue or pull request by leaving a
`/assign <your Github ID>` comment on the issue or pull request.

### Commands and Workflow

Cluster API Provider Metal3 follows the standard Kubernetes workflow: any PR
needs `lgtm` and `approved` labels, and PRs must pass the tests before being
merged. See
[the contributor docs](https://github.com/kubernetes/community/blob/master/contributors/guide/pull-requests.md#the-testing-and-merge-workflow)
for more info.

We use the same priority and kind labels as Kubernetes. See the labels tab in
GitHub for the full list.

The standard Kubernetes comment commands should work in Cluster API Provider
Metal3. See [Prow](https://prow.apps.test.metal3.io/command-help) for a command
reference.

## Release Process

Minor and patch releases are generally done immediately after a feature or
bugfix is landed, or sometimes a series of features tied together.

Minor releases will only be tagged on the _most recent_ major release branch,
except in exceptional circumstances. Patches will be backported to maintained
stable versions, as needed.

Major releases are done shortly after a breaking change is merged -- once a
breaking change is merged, the next release _must_ be a major revision. We don't
intend to have a lot of these, so we may put off merging breaking PRs until a
later date.

### Exact Steps

Refer to the [releasing document](./docs/releasing.md) for the exact steps.
