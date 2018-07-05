Service Idler
=============

The service idler is a controller and Kubernetes API type (served via
a CRD) which enables building automated idling and unidling with
Kubernetes.

Summary
-------

It works like this: for every group of scalable Kubernetes resources (like
Deployments, Replica Sets, etc) that you want to be idled and unidled
together, you create an `Idler` object:

```yaml
kind: Idler
apiVersion: idling.openshift.io/v1alpha2
metadata:
  name: sample
spec:
  wantIdled: false
  targetScalables:
  - group: apps
    resource: deployment
    name: some-deployment
  triggerServiceNames:
  - some-svc
```

The Idler object specifies those scalables, as well as a set of trigger
services (see below).  When the `.spec.wantIdled` field is set to true,
the idling controller will ensure that all scalables are scaled to zero,
and that their previous scales are recorded:

```yaml
kind: Idler
apiVersion: idling.openshift.io/v1alpha2
metadata:
  name: sample
spec:
  wantIdled: false
  targetScalables:
  - group: apps
    resource: deployment
    name: some-deployment
  triggerServiceNames:
  - some-svc
status:
  idled: true
  inactiveServiceNames:
  - some-svc
  unidledScales:
  - group: apps
    resource: deployment
    name: some-deployment
    previousScale: 2
```

When the `wantIdled` field is flipped back to false, the controller will
ensure that all target scalables are returned to their previous scales,
and clear `unidledScales` and `inactiveServiceNames` (which represents the
trigger services that don't yet have ready endpoints).

Usage With Network-Traffic-Based Unidling
-----------------------------------------

While the controller itself *does not provide network-based unidling*, it
is designed to make building such a solution relatively straightforward.
Network proxies can take advantage of the `.status.idled` field to
determine when they should consider services for unidling -- the `idled`
field will be set to true from the time that the idling controller
*starts* scaling down its scalables, to the time when all trigger services
have *at least* one endpoint ready.  To trigger unidling, a proxy simply
has to patch the idler to set `.spec.wantIdled` to true.

For an example, see [the OpenShift Origin unidling
proxy](https://github.com/openshift/origin/blob/master/pkg/proxy).

Working With This Repo
----------------------

This repo is build using the excellent
[kubebuilder](https://github.com/kubernetes-sigs/kubebuilder) toolset.

The repository may be built and tested using `Dockerfile.controller`, or
by running the `go build` and `go test` (respectively) commands contained
therein with a sufficiently recent version of Go (1.9+).

The vendor directory is maintained by
[dep](https://github.com/golang/dep), and is not checked in to the
repository.  It must be restored before building using `dep ensure
-vendor-only`.

The `hack/install.yaml` file contains the required Kubernetes objects to
install the CRD and launch the controller on a Kubernetes cluster (run
`kubectl apply -f hack/install.yaml`).

### RPM Images

The `images` directory contains a Dockerfile designed for use with an
RPM-based setup.  The corresponding RPM can be produced using
`service-idler.spec`.   It's probably not relevant for most people.
