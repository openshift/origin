# OpenShift extended test suite

This document describes how a developer can write a new extended test for
OpenShift and the structure of extended tests.

Prerequires
------------------

In order to execute the extended tests, you have to install
[Ginkgo](https://github.com/onsi/ginkgo) framework which is used in extended
tests.

You also need to have the `openshift` binary in the `PATH` if you want to use
the shell script helpers to execute the extended tests.

Extended tests structure
------------------------

Extended tests live under the `./test/extended` directory in the origin repository.

The structure of this directory is following:

* **`test/extended/util`** provides useful helpers and utilities to use in your extended test. It provides a easy-to-use interface to OpenShift CLI and also
access to the Kubernetes [E2E framework](https://github.com/openshift/origin/tree/master/Godeps/_workspace/src/k8s.io/kubernetes/test/e2e) helpers. It also contains OpenShift helpers are shared across multiple test cases, to make the test cases more DRY.
* **`test/extended/fixtures`** contains the JSON and YAML fixtures are meant to be used by the extended tests.
* **`test/extended/[images,builds,...]`** each of this Go package contains extended tests that are related together. For example, the `images` directory should contain test cases that are exercising usage of various Docker images in OpenShift.
* **`hack/test-extended/[group]/run.sh`** contains a shell script that helps you to execute extended tests for certain group
* **`test/extended/extended_test.go`** is a runner for all extended test packages. Looking inside this file, you can see how you can add new extended test Go package to be compiled:
```go
	_ "github.com/openshift/origin/test/extended/builds"
	_ "github.com/openshift/origin/test/extended/images"
```

Groups vs. packages
---------------------

Since the extended tests might rely on specify OpenShift server configuration,
the tests are divided into logical 'test groups'. Each group has its own shell
launcher that bootstraps the OpenShift environment in a way the group requires
to be executed.
For example, you might want to write an extended test for the LDAP
authentication which means that you have to configure OpenShift server to
enable this authentication method. 
You can create a new test group `ldap` and provide a shell launcher
`./hack/test-extended/ldap/run.sh` to start OpenShift with required
configuration.
Then you place the source code with extended test into the extended test Go
package that corresponds to functionality you are going to test. In case of
LDAP, it can be `./test/extended/authentication`.  In order to have your test
case executed by `./hack/test-extended/ldap/run.sh` you have to add `ldap:`
prefix to the `Describe()` function:

Example:
```go
var _ = Describe("ldap: Authenticate using LDAP", func() {
  # ...
})
```


