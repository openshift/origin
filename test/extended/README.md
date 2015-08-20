# OpenShift extended test suite

This document describes how a developer can write a new extended test for
OpenShift and the structure of extended tests.

Prerequires
------------------

In order to execute the extended tests, you have to install
[Ginkgo](https://github.com/onsi/ginkgo) framework which is used in extended
tests. You can do it by running following command:

```console
$ go get github.com/onsi/ginkgo/ginkgo
```

You also need to have the `openshift` binary in the `PATH` if you want to use
the shell script helpers to execute the extended tests.

Extended tests structure
------------------------

Extended tests live under the `./test/extended` directory in the origin repository.

The structure of this directory is following:

* [**`test/extended/util`**](util) provides useful helpers and utilities to use in your extended test. It provides a easy-to-use interface to OpenShift CLI and also
access to the Kubernetes [E2E framework](https://github.com/openshift/origin/tree/master/Godeps/_workspace/src/k8s.io/kubernetes/test/e2e) helpers. It also contains OpenShift helpers that are shared across multiple test cases, to make the test cases more DRY.
* [**`test/extended/fixtures`**](fixtures) contains the JSON and YAML fixtures that are meant to be used by the extended tests.
* [**`test/extended/[images,builds,...]`**](builds) each of these Go packages contains extended tests that are related to each other. For example, the `images` directory should contain test cases that are exercising usage of various Docker images in OpenShift.
* [**`hack/test-extended/[group]/run.sh`**](../../hack/test-extended) is the shell script that sets up any needed dependencies and then launches the extended tests whose top level ginkgo spec's Describe call reference the [group](#groups-vs-packages)
* [**`test/extended/extended_test.go`**](extended_test.go) is a runner for all extended test packages. Look inside this file to see how you can add new extended test Go package to be compiled:
```go
	_ "github.com/openshift/origin/test/extended/builds"
	_ "github.com/openshift/origin/test/extended/images"
```

Groups vs. packages
---------------------

Since the extended tests might rely on a specific OpenShift server configuration,
the tests are divided into logical 'test groups'. Each group has its own shell
launcher that bootstraps the OpenShift environment in a way the group requires
to be executed.
For example, you might want to write an extended test for the LDAP
authentication which means that you have to configure the OpenShift server to
enable this authentication method.
You can create a new test group `ldap` and provide a shell launcher
`./hack/test-extended/ldap/run.sh` to start OpenShift with the required
configuration.
Then you place the source code for the extended test into the extended test Go
package that corresponds to functionality you are going to test. In the case of
LDAP, it can be `./test/extended/authentication`.  In order to have your test
case executed by `./hack/test-extended/ldap/run.sh` you have to add the `ldap:`
prefix to the `Describe()` function:

Example:
```go
var _ = g.Describe("ldap: Authenticate using LDAP", func() {
  # ...
})
```

Creating new test group runner
-------------------------

To create a new test group, you have to first create a runner shell script that
executes the test cases with `FOCUS` set to the group name. To do so, create a
new directory under `./hack/test-extended/`. The name of the directory will be
the name of the group you are going to create. Then place a Bash script named
`run.sh` into this directory. You can look into existing test runners and see
the Bash helpers that are provided to you below to start.

In the end of the test group runner, you have to execute the Ginkgo tests:

```console
# Run the tests
pushd ${OS_ROOT}/test/extended >/dev/null
export KUBECONFIG="${ADMIN_KUBECONFIG}"
export EXTENDED_TEST_PATH="${OS_ROOT}/test/extended"
ginkgo -progress -stream -v -focus="default:" -p ${OS_OUTPUT_BINPATH}/extended.test
popd >/dev/null
```

> NOTE: It is necessary that you export the `KUBECONFIG` and `EXTENDED_TEST_PATH` variables.
> The `KUBECONFIG` must contain an absolute path to the `admin.kubeconfig` and
> the `EXTENDED_TEST_PATH` must contain the absolute path to the `test/extended`
> folder.

To run your test group, you can then use the group runner you have created
above, or or you can use the `./hack/test-extended.sh` runner that allows you to
run multiple test groups at once:

```console
$ ./hack/test-extended.sh default ldap
```

To run all test groups, you can pass the `all` option to this runner.


Bash helpers for creating new test group runner
-----------------------------------------

In order to make the test runner shell scripts more DRY, we bundled common
functions into a Bash helpers you can run instead of copy&pasting the code from
other launchers. These helpers are located in `./hack/util.sh`.

* `ginkgo_check_extended()` verify if the Ginkgo binary is installed.
* `compile_extended()` perform the compilation of the Go tests into a test binary.
* `test_privileges()` verify if you have permissions to start OpenShift server.
* `setup_env_vars()` setup all required environment variables related to OpenShift server.
* `configure_os_server()` generates all configuration files for OpenShift server.
* `start_os_server()` starts the OpenShift master and node.
* `install_router_extended()` installs the OpenShift router service.
* `install_registry_extended()` installs the OpenShift Docker registry service.
* `create_image_streams_extended()` creates ImageStream(s) for all OpenShift images.

CLI interface
---------------------

In order to be able to call the OpenShift CLI and Kubernetes and OpenShift REST clients and simulate the OpenShift `oc` command in the test suite, first we need to create an instance of the CLI, in the top-level Ginkgo describe container.
The top-level describe container should also specify the bucket into which the test belongs and a short test description. Other globally accessible variables (eg. fixtures) can be declared as well.

```
package extended

import (
    g "github.com/onsi/ginkgo"
    o "github.com/onsi/gomega"
)

var _ = g.Describe("<test bucket>: <Testing scenario>", func() {
	defer g.GinkgoRecover()
	var (
		oc = exutil.NewCLI("test-name", exutil.KubeConfigPath())
		testFixture = filepath.Join("fixtures", "test.json")
	)
})
```

The test suite should be organized into lower-level Ginkgo describe(s) container, together with a message which elaborates on the goal of the test. Inside each lower-level describe container specify a single spec with the `It` container , which shares the context in which the spec runs. The `It` container also takes a message which explains how the test goal will be achieved.

```
var _ = g.Describe("default: STI build", func() {
	defer GinkgoRecover()
	var (
		stiBuildFixture = filepath.Join("fixtures", "test-build.json")
		oc              = exutil.NewCLI("build-sti", kubeConfigPath())
	)

	g.Describe("Building from a template", func() {
		g.It(fmt.Sprintf("should create a image from %q template", stiBuildFixture), func() {
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

To execute the command you will need to call either `Execute()`, which will execute the command and return any error that occurs, or `Output()`  which returns any error that occurs as well as the output.

```
err := oc.Run("create").Args("-f", testFixture).Execute()
```
```
buildName, err := oc.Run("start-build").Args("test").Output()
```

To print out the purpose of the next command, or set of commands, use the Ginkgo’s `By` function.
```
g.By("starting a test build")
buildName, err := oc.Run("start-build").Args("test").Output()
```

To evaluate if the the command was successfully executed without any errors retrieved, use the Gomega’s `Expect` syntax to make expectations on the error.
```
err = oc.Run("create").Args("-f", stiEnvBuildFixture).Execute()
o.Expect(err).NotTo(o.HaveOccurred())
```
