# NRI Tests

This directory contains OpenShift end-to-end tests for the Node Resource
Interface (NRI) path from CRI-O to a plugin to a workload. Unlike CRI-O's
[test/nri.bats](https://github.com/cri-o/cri-o/blob/main/test/nri.bats), the
plugin is a DaemonSet and the workloads are pods, the way NRI plugins are
deployed in production.

## Test Suite: openshift/nri

Deploys the test plugin in `plugin/` on one node and covers registration,
synchronization, pod and container events, container adjustments (environment,
annotations, mounts, devices, cpuset), the NRI failure policy, namespace and
admission boundaries, two plugins at once, pod churn, and plugin restart. These are the prerequisites plugins such as
[kai-scheduler/gpu-fractioning](https://github.com/kai-scheduler/gpu-fractioning#prerequisites)
rely on; gpu-fractioning gets its devices from the GPU Operator/CDI, so device
injection here covers the generic NRI primitive.

- **nri.go** - the suite; scenarios map to the cases in CRI-O's `test/nri.bats`
- **nri_helpers.go** - test namespace, plugin DaemonSet, pod and plugin log helpers
- **plugin/** - the NRI plugin under test, shipped as the `nri-test-plugin` payload image
- **testplan/** - test plan generated with `openshift-developer:generate-test-plan`, and the detailed test case catalogue

The plugin records every NRI request as JSON lines and only adjusts pods that
opt in with `nri.openshift.io/test: "true"`:

| annotation | effect |
|---|---|
| `nri.openshift.io/test-env` | `KEY=VALUE[,KEY=VALUE]` added to the container environment |
| `nri.openshift.io/test-mounts` | `/host/path:/container/path[:ro]` bind mounts |
| `nri.openshift.io/test-devices` | `/dev/name:type:major:minor[:mode]` device nodes |
| `nri.openshift.io/test-annotations` | `KEY=VALUE` OCI annotations |
| `nri.openshift.io/test-cpuset` | `cpuset.cpus` for the container |
| `nri.openshift.io/test-fail` | `true` fails `CreateContainer` |

## Running

```console
$ openshift-tests run openshift/nri
```

On a cluster whose release image is not reachable with the local pull secret, add
`--extension-local-binaries-only` to skip external test binary extraction.

`run-test` matches test names exactly, so a single case needs its full name:

```console
$ openshift-tests run-test "[Suite:openshift/nri][sig-node] Node Resource Interface injects environment, annotations, mounts and cpuset into opted-in containers"
```

The plugin ships as the `nri-test-plugin` payload image, built from
`images/nri-test-plugin/Dockerfile.rhel`. On a cluster whose payload predates
that image, build and push it and point the suite at it:

```console
$ make -C test/extended/nri/plugin image IMAGE=quay.io/<you>/nri-test-plugin:dev
$ make -C test/extended/nri/plugin push  IMAGE=quay.io/<you>/nri-test-plugin:dev
$ NRI_TEST_PLUGIN_IMAGE=quay.io/<you>/nri-test-plugin:dev openshift-tests run openshift/nri
```

The image build needs network access for the plugin's Go modules. Shipping it in
the payload needs a matching `images:` entry for `nri-test-plugin`
(`dockerfile_path: images/nri-test-plugin/Dockerfile.rhel`) in the
`openshift/release` ci-operator config for this branch.

## Prerequisites and gotchas

* The plugin runs privileged as root: CRI-O's NRI socket lives in a `0700
  root:root` directory labeled `container_var_run_t`. The suite relaxes
  PodSecurity admission on its namespace and binds the `privileged` SCC to the
  `default` service account; operators shipping an NRI plugin need the same (this
  is why the upstream nri-plugins charts need a namespace/SCC tweak on OpenShift).
* NRI-injected mounts keep the source's SELinux label, and CRI-O does not relabel
  them. The suite mounts host files that containers can already read
  (`/proc/version`, `/dev/null`), so it needs no relabeling; a plugin injecting its
  own paths has to label them `container_file_t` (shared) itself.
* The suite picks any Ready worker, then a non-control-plane node, then a
  control-plane node (SNO), and pins the DaemonSet to it. All state lives in the
  test namespace, so runs on the same node do not interfere.
* Device injection goes through the NRI adjustment path; the test injects a
  synthetic character device and checks the node, mode and major:minor, like
  CRI-O's `TestDeviceInjection`.
