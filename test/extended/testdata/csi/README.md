# CSI driver installer manifests

Each CSI driver is represented as a directory with two required files:

```
<driver name>/install-template.yaml
<driver name>/manifest.yaml
```

Optionally, there can also be a file for a storageClass:

```
<driver name>/storageclass.yaml
```

This file is typically used to set custom parameters in the storageClass. For instance,
in order to use the [topology](https://kubernetes-csi.github.io/docs/topology.html)
feature of a CSI driver, one might want to have a custom storageClass with `volumeBindingMode`
set to `WaitForFirstConsumer`. Also, the custom storageClass needs to be refereced in the [manifest file](#manifest).

## Driver template
`install-template.yaml` is a [golang template](https://golang.org/pkg/text/template/) of YAML file with all Kubernetes objects of a CSI driver.
It will be instantiated at the beginning of the test via `oc apply -f <file>`, where `<file>` is result of the template evaluation.

It is expected that the YAML file creates also a hardcoded namespace, so multiple `oc apply -f <file>` are idempotent and don't install the driver multiple times.

Following variables are available in the template:

* Name of sidecar image to test with. It is either the last build in the appropriate 4.x branch or image build from PR that's being tested.
  * `{{.AttacherImage}}`
  * `{{.ProvisionerImage}}`
  * `{{.ResizerImage}}`
  * `{{.NodeDriverRegistrarImage}}`
  * `{{.LivenessProbeImage}}`

* `{{.ImageFormat}}`: Generic format of image names for the test, provided in case the template wants to use a different image than the listed above. E.g. `registry.svc.ci.openshift.org/ci-op-pthpkjbt/stable:${component}`.

## Manifest
`manifest.yaml` describes features of the CSI driver. See [upstream documentation](https://github.com/kubernetes/kubernetes/blob/master/test/e2e/storage/external/README.md) for its format and usage.

## Usage

CSI driver defined here can be installed and tested using:

```
TEST_INSTALL_CSI_DRIVERS=<driver name> openshift-tests run openshift/csi
```

Multiple CSI drivers can be installed & tested in one run, `TEST_INSTALL_CSI_DRIVERS` is comma-separated list.
