Origin Kubernetes
=================

[![Go Report Card](https://goreportcard.com/badge/github.com/openshift/origin)](https://goreportcard.com/report/github.com/openshift/origin)
[![GoDoc](https://godoc.org/github.com/openshift/origin?status.png)](https://godoc.org/github.com/openshift/origin)
[![Licensed under Apache License version 2.0](https://img.shields.io/github/license/openshift/origin.svg?maxAge=2592000)](https://www.apache.org/licenses/LICENSE-2.0)

This repo was previously the core Kubernetes tracking repo for
[OKD](https://github.com/openshift/okd), and where OpenShift's
`hyperkube` and `openshift-test` binaries were maintained. As of July
2020, the purpose and maintenance strategy of the repo varies by
branch.

## Maintenance of `master` and `release-x.x` branches for 4.6 and above

These branches no longer include the code required to produce
`hyperkube` binaries, and are limited to maintaining the `openshift-tests`
binary.  Responsibility for maintaining hyperkube has transitioned to
the [openshift/kubernetes](https://github.com/openshift/kubernetes)
repo.

Backports and carries against upstream should be proposed to
`openshift/kubernetes`. If changes merged to `openshift/kubernetes`
need to land in `origin`, it will be necessary to follow up with a PR
to `origin` that bumps the vendoring.

Branch names are correlated across the 2 repositories such that
changes merged to a given branch in `openshift/kubernetes` should be
vendored into the same branch in `origin` (e.g. `master` in
`openshift/kubernetes` is vendored into `master` in `origin`).

**NOTE:** Vendoring of the `master` and `release-x.x` branches of
`openshift/kubernetes` into the equivalent branches in `origin` is
intended to be temporary. At some point in the near future, `origin`
will switch to vendoring origin-specific branches (e.g
`origin-4.6-kubernetes-1.19.2`) to minimize the scope of backports and
carries that need to be considered in the context of
`openshift/kubernetes` rebases.

### Test annotation rules

Test annotation rules are used to label e2e tests so that they can be
filtered or skipped. For example, rules can be defined that match kube
e2e tests that are known to be incompatible with openshift and label
those tests to be skipped.

Maintenance of test annotation rules is split between the
`openshift/kubernetes` and `origin` repos to ensure that PRs proposed
to `openshift/kubernetes` can be validated against the set of kube e2e
tests known to be compatible with openshift.

Test annotation rules for kubernetes e2e tests are maintained in:

https://github.com/openshift/kubernetes/blob/master/openshift-hack/e2e/annotate/rules.go

Test annotation rules for openshift e2e tests are maintained in:

https://github.com/openshift/origin/blob/master/test/extended/util/annotate/rules.go

Origin vendors the kube rules and applies both the kube and openshift
rules to the set of tests included in the `openshift-tests` binary.

In order to update test annotation rules for kube e2e tests, it will
be necessary to:

 - Update `rules.go` in `openshift/kubernetes`
 - Bump the version of `openshift/kubernetes` vendored in origin

### Vendoring from `openshift/kubernetes`

These origin branches vendor `k8s.io/kubernetes` and some of its
staging repos (e.g. `k8s.io/api`) from our
[openshift/kubernetes](https://github.com/openshift/kubernetes) fork.
Upstream staging repos are used where possible, but some tests depends
on functionality that is only present in the fork.

When a change has merged to an `openshift/kubernetes` branch that
needs to be vendored into the same branch in `origin`, the
`hack/update-kube-vendor.sh` helper script simplifies updating the go
module configuration for all dependencies sourced from
`openshift/kubernetes` for that branch. The script requires either the
name of a branch or a SHA from `openshift/kubernetes`:

```bash
$ hack/update-kube-vendor.sh <openshift/kubernetes branch name or SHA>
```

The script also supports performing a fake bump to validate an as-yet
unmerged change to `openshift/kubernetes`. This can be accomplished by
supplying the name of a fork repo as the second argument to the
script:

```bash
$ hack/update-kube-vendor.sh <branch name or SHA> github.com/myname/kubernetes
```

Once the script has executed, the vendoring changes will need to be
committed and proposed to the repo.

## Maintenance of release-4.5, release-4.4 and release-4.3

Releases prior to 4.6 continue to maintain hyperkube in the `origin`
repo in the `release-4.x` branches. Persistent carries and backports
for those branches should continue to be submitted directly to
origin. `openshift/kubernetes` is not involved except for rebases.

## End-to-End (e2e) and Extended Tests

End to end tests (e2e) should verify a long set of flows in the
product as a user would see them.  Two e2e tests should not overlap
more than 10% of function and are not intended to test error
conditions in detail. The project examples should be driven by e2e
tests. e2e tests can also test external components working together.

All e2e tests are compiled into the `openshift-tests` binary.
To build the test binary, run `make`.

To run a specific test, or an entire suite of tests, read
[test/extended/README](https://github.com/openshift/origin/blob/master/test/extended/README.md)
for more information.

## Updating external examples

`hack/update-external-example.sh` will pull down example files from external
repositories and deposit them under the `examples` directory.
Run this script if you need to refresh an example file, or add a new one.  See
the script and `examples/quickstarts/README.md` for more details.
