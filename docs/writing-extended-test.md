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


CLI interface
---------------------

In order to be able to call the OpenShift CLI and Kubernetes and OpenShift REST client and simulate the OpenShift `oc` command in the test suit, first we need to create an instance of the CLI, in the top-level Ginkgo describe container. The top-level describe container shall also specify the the bucket, into which the test belongs and short test description. Other globally accessible variables can be declared(eg. fixtures) can be declared as well.

```
package extended

import (
    . "github.com/onsi/ginkgo"
    . "github.com/onsi/gomega"
)

var _ = Describe("<test bucket>: <Testing scenario>", func() {
	defer GinkgoRecover()
	var (
		oc = exutil.NewCLI("test-name", exutil.KubeConfigPath())
		testFixture = filepath.Join("fixtures", "test.json")
	)
})
```

The test suit shall be organized in lower-level Ginkgo describe container, together with an message which informs about the goal of the test. Inside the lower-level describe container specify a single spec with the `It` container , which shares the context in which the spec runs. The `It` container also takes a message, which informs how should be the goal achieved.

```
var _ = Describe("default: STI build", func() {
	defer GinkgoRecover()
	var (
		stiBuildFixture = filepath.Join("fixtures", "test-build.json")
		oc              = exutil.NewCLI("build-sti", kubeConfigPath())
	)

	Describe("Building from a template", func() {
		It(fmt.Sprintf("should create a image from %q template", stiBuildFixture), func() {
			...
		}
	}
}
```

After that you are free to simulate any `oc` command by calling the CLI methods from the extended package.

As first, the command verb (get, create, start-build, ...) has to be specified upon the created CLI instance with the `Run()` method.
```
oc = oc.Run("create")
```

Then the command parameters have to be specified by using the `Args()` command. You may also notice the methods can be easily chained.
```
oc = oc.Run("create").Args("-f", testFixture)
```

A Go template can be set as a parameter for the OpenShift CLI command, by using the `Template()` method. Keep in mind that in order to use this method, the `get` verb has to be specified by the `Run()` command.
```
oc = oc.Run("get").Template({{ .spec }})
```
is an equivalent to
```
oc get foo -o template -t '{{ .spec }}
```

To execute the command you will need to call either `Execute()`, which will execute the command and return only error if any occurs, or `Output()` command which besides error also returns the output of the command in a form of a string.

```
err := oc.Run("create").Args("-f", testFixture).Execute()
```
```
buildName, err := oc.Run("start-build").Args("test").Output()
```

To print out the purpose the next command, or set of command, use the Ginkgo’s `By` function.
```
By("starting a test build")
buildName, err := oc.Run("start-build").Args("test").Output()
```

To evaluate if the the command was successfully executed, without any errors retrieved, use the Gomega’s `Expect` syntax to make expectations on the error.
```
err = oc.Run("create").Args("-f", stiEnvBuildFixture).Execute()
Expect(err).NotTo(HaveOccurred())
```