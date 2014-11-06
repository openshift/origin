Hacking on OpenShift
====================

## Building a Release

To build an OpenShift release you run the `hack/build-release.sh` script on a system with Docker, which
will create a build environment image and then execute a cross platform Go build within it. The build
output will be copied to `_output/releases` as a set of tars containing each version. It will also build
the `openshift/origin-base` image which is the common parent image for all OpenShift Docker images.

    $ hack/build-release.sh

Once the release has been built the official Docker images can be generated with `hack/build-images.sh`.
The resulting images can then be pushed to a Docker registry.

    $ hack/build-images.sh

Note: To build the base and release images, run:

    $ hack/build-base-images.sh


## Test Suites

OpenShift uses three levels of testing - unit tests, integration test, and end-to-end tests (much
like Kubernetes).

### Unit tests

Unit tests follow standard Go conventions and are intended to test the behavior and output of a
single package in isolation. All code is expected to be easily testable with mock interfaces and
stubs, and when they are not it usually means that there's a missing interface or abstraction in the
code. A unit test should focus on testing that branches and error conditions are properly returned
and that the interface and code flows work as described. Unit tests can depend on other packages but
should not depend on other components (an API test should not be writing to etcd).

The unit tests for an entire package should not take more than 0.5s to run, and if they do, are
probably not really unit tests or need to be rewritten to avoid sleeps or pauses. Coverage on a unit
test should be above 70% unless the units are a special case.

See `pkg/template/generator` for examples of unit tests. Unit tests should follow Go conventions.

Run the unit tests with:

    $ hack/test-go.sh

or an individual package unit test with:

    $ hack/test-go.sh pkg/build

To run only a certain regex of tests in a package, use:

    $ hack/test-go.sh pkg/build -test.run=SynchronizeBuildRunning

To get verbose output add `-v` to the end:

    $ hack/test-go.sh pkg/build -test.run=SynchronizeBuildRunning -v

To run all tests with verbose output:

    $ hack/test-go.sh "" -v

To turn off or change the coverage mode, which is `-cover -covermode=atomic` by default, use:

    $ KUBE_COVER="" hack/test-go.sh

To run tests without the go race detector, which is on by default, use:

    $ KUBE_RACE="" hack/test-go.sh

A line coverage report is run by default when testing a single package.
To create a coverage report for all packages:

    $ OUTPUT_COVERAGE=true hack/test-go.sh pkg/build

### Integration tests

Integration tests cover multiple components acting together (generally, 2 or 3). These tests should
focus on ensuring that naturally related components work correctly.  They should not be extensively
testing branches or error conditions inside packages (that's what unit tests do), but they should
validate that important success and error paths work across layers (especially when errors are being
converted from lower level errors). Integration tests should not be testing details of the
intercomponent connections - API tests should not test that the JSON serialized to the wire is
correctly converted back and forth (unit test responsibility), but they should test that those
connections have the expected outcomes. The underlying goal of integration tests is to wire together
the most important components in isolation. Integration tests should be as fast as possible in order
to enable them to be run repeatedly during testing.  Integration tests that take longer than 0.5s
are probably trying to test too much together and should be reorganized into separate tests.
Integration tests should generally be written so that they are starting from a clean slate, but if
that involves costly setup those components should be tested in isolation.

We break integration tests into two categories, those that use Docker and those that do not.  In
general, high-level components that depend on the behavior of code running inside a Docker container
should have at least one or two integration tests that test all the way down to Docker, but those
should be part of their own test suite.  Testing the API and high level API functions should
generally not depend on calling into Docker. They are denoted by special test tags and should be in
their own files so we can selectively build them.

All integration tests are located under `test/integration/*`. All integration tests must set the
`integration` build tag at the top of their source file, and also declare whether they need etcd
with the `!no-etcd` build tag and whether they need Docker with the `!no-docker` build tag. For
special function sets please create subdirectories like `test/integration/deployimages`.

Run the integration tests with:

    $ hack/test-integration.sh

The script launches an instance of etcd and then invokes the integration tests. If you need to
execute an individual test start etcd and then run:

    $ hack/test-go.sh test/integration -tags 'integration no-docker' -test.run=TestBuildClient

There is a CLI integration test suite which covers general non-Docker functionality of the CLI tool
working against the API. Run it with:

    $ hack/test-cmd.sh

### End-to-End (e2e) Tests

The final test category is end to end tests (e2e) which should verify a long set of flows in the
product as a user would see them.  Two e2e tests should not overlap more than 10% of function, and
are not intended to test error conditions in detail. The project examples should be driven by e2e
tests. e2e tests can also test external components working together.

End to end tests should be Go tests with the build tag `e2e` in the `test/e2e` directory.

TODO: implement

Run the end to end tests with:

    $ hack/test-e2e.sh


## Installing Godep

OpenShift and Kubernetes use [Godep](https://github.com/tools/godep) for dependency management.
Godep allows versions of dependent packages to be locked at a specific commit by *vendoring* them
(checking a copy of them into `Godeps/_workspace/`).  This means that everything you need for
OpenShift is checked into this repository, and the `hack/config-go.sh` script will set your GOPATH
appropriately.  To install `godep` locally run:

    $ go get github.com/tools/godep

If you are not updating packages you should not need godep installed.

## Updating Godeps from upstream

To update to a new version of a dependency that's not already included in Kubernetes, checkout the
correct version in your GOPATH and then run `godep save <pkgname>`.  This should create a new
version of `Godeps/Godeps.json`, and update `Godeps/_workspace/src`.  Create a commit that includes
both of these changes with message `bump(<pkgname>): <pkgcommit>`.

To update the Kubernetes version, checkout the new "master" branch from openshift/kubernetes (within
your regular GOPATH directory for Kubernetes), and run `godep restore ./...` from the Kubernetes
dir.  Then switch to the OpenShift directory and run `godep save ./...`
