# OpenShift CSI certification tests

## Intro

OpenShift `openshift/csi` test suite contains tests that exercise features of an already installed CSI driver. We re-use [upstream storage tests](https://github.com/openshift/kubernetes/blob/master/test/e2e/storage/external/README.md), including its YAML file manifest, and add a few OpenShift specific tests on top of it.

Note: this documentation is not supported by Red Hat. It's here to help with debugging the tests or CSI driver. Follow the official Red Hat documentation to submit official CSI driver test results.

## Manifests

Two YAML files control what CSI driver features are tested and how. `openshift-tests` binary accepts two environment variables:

* `TEST_CSI_DRIVER_FILES`: path to a file with **upstream** test manifest. See [upstream documentation](https://github.com/openshift/kubernetes/blob/master/test/e2e/storage/external/README.md) for full details. This env. variable is mandatory.
* `TEST_OCP_CSI_DRIVER_FILES`: path to a file with **OpenShift specific** test manifest, see below for its format.

### OpenShift specific manifest

Example:

```yaml
Driver: <CSI driver name>
LUNStressTest:
  PodsTotal: 260
  Timeout: "40m"
```

`LUNStressTest` is a test that stresses the CSI driver on a single node. The test picks a random scheudlable node and creates configured number of Pods + PVCs on it (260 by default).


* Each Pod has its own PVC that needs to be dynamically provisioned by the CSI driver.
* Each Pod does something very simple (like `ls /mnt/the_volume`) and exits quickly.
* While all these Pods are created relativly quickly, the test *does not* expect for all Pods to run in parallel!
  * We expect the CSI driver to return timeouts and other errors when it gets too many requests. OpenShift / CSI sidecars will retry with exponential backoff.
  * Kubernetes should respect the CSI driver attach limit reported in CSINode, so only that amount of Pods can ever run in parallel.
    * There is [a bug in Kubernetes](https://github.com/kubernetes/kubernetes/issues/126502) when the scheduler can put more Pods on a single node than the CSI driver supports. We expect the CSI driver to be robust and return a reasonable error to `ControllerPublish`, `NodeStage` or `NodePublish` when it's over the limit.
* The timeout can be generous to allow enough time for dynamic provisioning, volume attach, mount, unmount, detach and PV deletion of 260 volumes.
* No other test runs in parallel to this test, so the CSI driver can fully focus on this stress.

* `PodsTotal`: how many Pods to create, 260 by default.
* `Timeout`: how long to wait for these Pods to finish. Accepts [golang `ParseDuration` suffixes](https://pkg.go.dev/time#ParseDuration), such as `"1h30m15s"` for 1 hour, 30 minutes and 15 seconds.

We strongly recommend to tests with 257 or more Pods and we suggest the test to finish in under 1 hour. There were cases where a CSI driver / RHCOS node configuration had issues with LUN numbers higher than 256. Even when a CSI driver does not use LUNs, it's a nice stress test that checks the CSI driver reports reasonable attach limit and can deal with some load.

## Usage

### With `openshift-tests` binary

1. Either compile your own `openshift-tests` binary (run `make` in this repo) or extract it from an OpenShift image. **Always use the `openshift-tests` binary that corresponds to the OpenShift version that you have installed!**
2. Set `KUBECONFIG` environment variable to point to your client configuration.
3. Set `TEST_CSI_DRIVER_FILES` to upstream manifest.
4. Optionally, set `TEST_OCP_CSI_DRIVER_FILES` to OpenShift test manifest.
5. Run the test suite, `openshift-tests run openshift/csi`.

Example:

```shell
export TEST_CSI_DRIVER_FILES=upstream-manifest.yaml # this is mandatory
export TEST_OCP_CSI_DRIVER_FILES=ocp-manifest.yaml  # this is optional
./openshift-tests run openshift/csi |& tee test.log
```

Tips:
* `openshift-tests` runs a set of monitors *before* running any tests. They monitor the overall cluster health while the tests are running to make sure a test does not break the whole cluster. The monitors are *very* talkative and they create a lot of files in the current directory.
* `openshift-tests run openshift/csi --dry-run` can be used to list tests that will run.
* `openshift-tests run openshift/csi --run=<regexp>` can be used to run only specific tests. Optionally with `--dry-run` to fine tune the regexp. Use `--help` to get more command line options.
* `openshift-tests run-test <full test name>` will run just a single test, without any monitors. There is (almost) no noise on the output and it is the best way to debug a single test. The `<full test name>` must be exactly the same as printed by `--dry-run`, including all spaces. Carefuly copy+paste a whole lile from `--dry-run` output, incl. double quotes. For example: `./openshift-tests run-test "External Storage [Driver: cooldriver.coolstorage.com] [Testpattern: Pre-provisioned PV (ext4)] volumes should store data"`.

### With `tests` image from OpenShift release

It's roughly equivalent to running `openshift-tests` binary as describe above, the binary is just in an container image.

1. Prepare `kubeconfig.yaml`, upstream test manifest and optionally OpenShift test manifest in the current directory.
2. Find the image with `openshift-tests` that corresponds to your OpenShift cluster version.
    ```shell
    $ oc adm release info --image-for=tests
    quay.io/openshift-release-dev/ocp-v4.0-art-dev@sha256:8e43b259635d5adcef769f5f4359554395c900d7211915249ee66b5602fea5b9
    ```
3. Run `openshift-tests` inside the `tests` container image. Make the current directory available as `/data` in the container and connect all the env. variables.
    ```shell
    podman run -v `pwd`:/data:z --rm -it quay.io/openshift-release-dev/ocp-v4.0-art-dev@sha256:8e43b259635d5adcef769f5f4359554395c900d7211915249ee66b5602fea5b9 \
        sh -c "KUBECONFIG=/data/kubeconfig.yaml TEST_CSI_DRIVER_FILES=/data/upstream-manifest.yaml TEST_OCP_CSI_DRIVER_FILES=/data/ocp-manifest.yaml /usr/bin/openshift-tests run openshift/csi --junit-dir /data/results”
    ```

Tips:
* You can pass any command line parameters to `openshift-tests` as described above.
