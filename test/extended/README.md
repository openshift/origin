# OpenShift extended test suite

This document describes how a developer can write a new extended test for
OpenShift and the structure of extended tests.


Prerequisites
-------------

* Compile both `oc` and `openshift-tests` in this repository (with `make WHAT=cmd/openshift-tests`)
* Have the environment variable `KUBECONFIG` set pointing to your cluster.


Running Tests
-------------

To run a test by name:

```console
$ openshift-tests run-test <FULL_TEST_NAME>
```

To see the list of suites available, run:

```console
$ openshift-tests help run
```

See the description on the test for more info about what prerequites may exist for the test.

To run a subset of tests using a regexp, run:

```console
$ openshift-tests run all --dry-run | grep -E "<REGEX>" | openshift-tests run -f -
```


Test labels
-----------

See [kinds of tests](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-testing/e2e-tests.md#kinds-of-tests)
for a full explanation of the labels used for each test spec.  In brief:

- If a test has no labels, it is expected to run fast (under five minutes), be
  able to be run in parallel, and be consistent.

- \[Serial\]: If a test cannot be run in parallel with other tests (e.g. it
  takes too many resources or restarts nodes), it is labeled \[Serial\], and
  should be run in serial as part of a separate suite.

- \[Slow\]: If a test takes more than five minutes to run (by itself or in
  parallel with many other tests), it is labeled \[Slow\]. This partition allows
  us to run almost all of our tests quickly in parallel, without waiting for the
  stragglers to finish.

  OpenShift extended tests that run builds should be marked \[Slow\].

- Tests should be marked \[Conformance\] when they provide test coverage for
  functionality considered core and critical to a functional cluster (i.e. not
  exotic features/configurations) and which is not overlapping with coverage
  provided in other conformance tests.  Example of a valid conformance test: "Do
  builds work." Example of an invalid conformance test: "Do builds work when the
  forcePull flag is set."

- In general, accessing the local host (e.g. using the docker socket) in
  extended tests should be avoided.  If this is unavoidable, the test should be
  marked \[local\].

Extended tests structure
------------------------

Extended tests live under the `./test/extended` directory in the origin repository.

The structure of this directory is following:

* [**`test/extended/util`**](util) provides useful helpers and utilities to use in your extended test. It provides a easy-to-use interface to OpenShift CLI and also
access to the Kubernetes [E2E framework](https://github.com/openshift/origin/tree/master/vendor/k8s.io/kubernetes/test/e2e) helpers. It also contains OpenShift helpers that are shared across multiple test cases, to make the test cases more DRY.
* [**`test/extended/testdata`**](testdata) contains the JSON and YAML fixtures that are meant to be used by the extended tests.
* [**`test/extended/[images,builds,...]`**](builds) each of these Go packages contains extended tests that are related to each other. For example, the `images` directory should contain test cases that are exercising usage of various container images in OpenShift.

Groups vs. packages
-------------------

Each type of functional test should be in its own package. However, if your
package needs to specifically configure the server in a different way than
the standard path, you would create a new launcher script with the same name
as your package in the `test/extended` dir.

For example, you might want to write an extended test for the LDAP
authentication which means that you have to configure the OpenShift server to
enable this authentication method. You can create a new test group `ldap` and
provide a shell launcher `./test/extended/ldap.sh` to start OpenShift with the
required configuration.

Then you place the source code for the extended test into the extended test Go
package that corresponds to functionality you are going to test. In the case of
LDAP, it can be `./test/extended/ldap`. You should include a prefix for your
test cases at your root suite level.

Example:
```go
var _ = g.Describe("[ldap] Authenticate using LDAP", func() {
  # ...
})
```

Creating new test group runner
------------------------------

If your test requires different configuration than the rest of the extended
test cases, you should create a new execution script in `test/extended`. Be
sure to set your focus to ginkgo appropriately to select only your test.

If your tests cannot run as part of the default group, be sure to ensure your
package is not included by `test/extended`.

Bash helpers for creating new test group runner
-----------------------------------------------

Common functions for extended tests are located in `./hack/util.sh`. Environment setup scripts are located in `hack/lib/util/evironment.sh`.

* `ginkgo_check_extended()` verify if the Ginkgo binary is installed.
* `compile_extended()` perform the compilation of the Go tests into a test binary.
* `test_privileges()` verify if you have permissions to start OpenShift server.
* `os::util::environment::setup_all_server_vars()` setup all required environment variables related to OpenShift server.
* `os::start::configure_server()` generates all configuration files for OpenShift server.
* `os::start::server()` starts the OpenShift master and node.
* `os::start::router()` installs the OpenShift router service.
* `os::start::registry()` installs the OpenShift container image registry service.
* `create_image_streams_extended()` creates ImageStream(s) for all OpenShift images.

CLI interface
-------------

In order to be able to call the OpenShift CLI and Kubernetes and OpenShift clients and simulate the OpenShift `oc` command in the test suite, first we need to create an instance of the CLI, in the top-level Ginkgo describe container.
The top-level describe container should also specify the bucket into which the test belongs and a short test description. Other globally accessible variables (eg. fixtures) can be declared as well.

```go
package extended

import (
    g "github.com/onsi/ginkgo/v2"
    o "github.com/onsi/gomega"
)

var _ = g.Describe("[<test bucket>] <Testing scenario>", func() {
	defer g.GinkgoRecover()
	var (
		oc = exutil.NewCLI("test-name")
		testFixture = filepath.Join("testdata", "test.json")
	)
})
```

The test suite should be organized into lower-level Ginkgo describe(s) container, together with a message which elaborates on the goal of the test. Inside each lower-level describe container specify a single spec with the `It` container , which shares the context in which the spec runs. The `It` container also takes a message which explains how the test goal will be achieved.

```go
var _ = g.Describe("[default] STI build", func() {
	defer GinkgoRecover()
	var (
		stiBuildFixture = filepath.Join("testdata", "test-build.yaml")
		oc              = exutil.NewCLI("build-sti", kubeConfigPath())
	)

	g.Describe("Building from a template", func() {
		g.It(fmt.Sprintf("should create a image from %q template", filepath.Base(stiBuildFixture)), func() {
			...
		}
	}
}
```

After that you are free to simulate any `oc` command by calling the CLI methods from the extended package.

As first, the command verb (get, create, start-build, ...) has to be specified upon the created CLI instance with the `Run()` method.
```go
oc = oc.Run("create")
```

Then the command parameters have to be specified by using the `Args()` command. You may also notice the methods can be easily chained.
```go
oc = oc.Run("create").Args("-f", testFixture)
```

A Go template can be set as a parameter for the OpenShift CLI command, by using the `Template()` method. Keep in mind that in order to use this method, the `get` verb has to be specified by the `Run()` command.
```go
oc = oc.Run("get").Args("foo").Template("{{ .spec }}")
```
is an equivalent to
```console
$ oc get foo -o template --template='{{ .spec }}'
```

To execute the command you will need to call either `Execute()`, which will execute the command and return any error that occurs, or `Output()`  which returns any error that occurs as well as the output.

```go
err := oc.Run("create").Args("-f", testFixture).Execute()
```
```go
buildName, err := oc.Run("start-build").Args("test").Output()
```

To print out the purpose of the next command, or set of commands, use the Ginkgo’s `By` function.
```go
g.By("starting a test build")
buildName, err := oc.Run("start-build").Args("test").Output()
```

To evaluate if the the command was successfully executed without any errors retrieved, use the Gomega’s `Expect` syntax to make expectations on the error.
```go
err = oc.Run("create").Args("-f", stiEnvBuildFixture).Execute()
o.Expect(err).NotTo(o.HaveOccurred())
```
