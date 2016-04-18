# junitreport

`junitreport` is a tool that allows for the consumption of test output in order to create jUnit XML.

## Installation

In order to build and install `junitreport`, from the root of the OpenShift Origin repository, run `hack/build-go.sh tools/junitreport`. 

## Usage 

`junitreport` can read the output of different types of tests. Specify which output is being read with `--type=<type>`. Supported test output types currently include `'gotest'`, for `go test` output, and `'oscmd'`, for `os::cmd` output. The default test type is `'gotest'`. 

`junitreport` can output flat or nested test suites. To choose which type of output to use, set `--suites=<type>` to either `'flat'` or `'nested'`. The default suite output structure is `'flat'`. When creating nested test suites, `junitreport` will use `/` as the delimeter between suite names: `github.com/maintainer/repository/suite` will be parsed as a hierarchy of `github.com`, `github.com/maintainer`, *etc.* If you are requesting nested test suite output but do not want the root suite(s) to be as general as `github.com`, for example, set `--roots=<root suite names>` to be a comma-delimited list of the names of the suites you wish to use as roots. If the parser encounters a package outside of those roots, it will ignore it. This allows a user to provide a root suite and only collect data for children of that root from a larger data set.

Ensure that the output you are feeding `junitreport` is free of extraneous text - any lines that are not test/suite declarations, metadata, or results are interpreted as test output. Text that you do not expect to see in Jenkins, for example, while looking at the output of a failed test should not be included in the input to `junitreport`.

Currently, `junitreport` does not support the parsing of parallel test output.

### Examples

To parse the output of `go test` into a flat collection of test suites:

```sh

$ go test -v -cover ./... | junitreport > report.xml
```

To parse the output of `go test` into a nested collection of test suites rooted at `github.com/maintainer`:

```sh

$ go test -v -cover ./... | junitreport --suites=nested --roots=github.com/maintainer > report.xml
```

### Testing

`junitreport` has unit tests as well as integration tests. To run the unit tests from the `junitreport` root directory:

```sh
$ go test -v -cover ./...
```

To run the integration tests from the `junitreport` root directory:

```sh
$ test/integration.sh
```