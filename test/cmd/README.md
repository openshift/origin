# OpenShift Command-Line Integration Test Suite

This document describes how OpenShift developers should interact with the OpenShift command-line integration test suite.

## Test Structure

The script to run the entire suite lives in [`hack/test-cmd.sh`](./../../hack/test-cmd.sh). All of the test suites that make up
the parent suite live in `test/cmd`, and are divided by functional area. 

## Running Tests

To run the full test suite, use:
```sh
$ hack/test-cmd.sh
``` 

To run a single test suite, use:
```sh
$ hack/test-cmd.sh <name>
```

To run a set of suites matching some regex, use:
```sh
$ hack/test-cmd.sh <regex>
```

Any test suite can also be run if an OpenShift instance is running and you are the cluster admin by running the scripts inside of
`test/cmd`. The scripts will use the current project. All scripts assume cluster-admin privilege.


## Adding Tests

Most new tests belong in a specific suite under `test/cmd`. The only tests that belong in the parent suite (`hack/test-cmd.sh`) are
those that test functionality that does not depend on being logged in and having a project. For instance, some of the tests that live
there now are creating configuration files, logging in and logging out, and starting the master as an instance of Atomic Enterprise.

New suites can be added by placing scripts in `test/cmd`.

## `os::cmd` Utility Functions

The `os::cmd` namespace provides a number of utility functions for writing CLI integration tests. All tests in all CLI test suites
must use these functions, except for some exceptions mentioned later. 

The utility functions have two major functions - expecting a specific exit code from the command to be tested, and expecting something
about the output of that command to `stdout` and/or `stderr`. There are three classes of utility functions - those that expect "success"
or "failure", those that expect a specific exit code, and those that re-try their command until either some condition is met or some
time passes. The latter type of function is useful when waiting for some component to update, be it an imagestream, the project cache,
etc.

All utility functions that expect something about the output of the command to `stdout` or `stderr` use `grep -Eq` to run their test,
so the functions can accept either text literals or regex compliant with `grep -E` for input.

The utility functions use `eval` to run the commands passed to them, and do so in a sub-shell. In order to pass a command into a utility
function, it must be quoted. Therefore, if there is a literal string (`'text'`) in your command, you must use double-quotes (`"there is
the text: 'text'"`) to ensure that when the command is passed to `eval`, the text that you wanted to be a literal string remains so
and does not get interpreted as a command itself. 

Furthermore, variables can be passed in either surrounded by single quotes (`'$var'`) or double quotes (`"$var"`). It is best practice
to use double-quotes in your test scripts when passing variables to the utility functions, as this will allow the test to see the 
expanded variable and display your command exactly as it will be run, instead of displaying the fact that there is a variable that has
yet to be expanded.

In some cases, you may want to pass a string in to the wrapper functions that contains what looks like a `bash` variable but is not.
In this case, you must escape the dollar-sign when passing it in, for example: `"\$notavar"`. 

`bash` variable assignments done inside of a command passed to a utility function are not visible to the shell running your test. 
Therefore, if your test uses bash variables and you would like to do an assignment, you should *not* use the `os::cmd` wrapper functions. 

---

The utility functions contingent on command success or failure are:

#### `os::cmd::expect_success CMD` 
`expect_success` takes one argument, the command to be run, and runs it. If the command succeeds (its return code is `0`), the utility 
function returns `0`. Otherwise, the utility function returns `1`.

In order to test that a command succeeds, pass it to `os::cmd::expect_success` like:
```sh
$ os::cmd::expect_success 'openshift admin config'
```
   
#### `os::cmd::expect_failure CMD`
`expect_failure` takes one argument, the command to be run, and runs it. If the command fails (its return code is not `0`), the utility
function returns `0`. Otherwise, the utility function returns `1`.

In order to test that a command fails, pass it to `os::cmd::expect_failure` like:
```sh
$ os::cmd::expect_failure 'openshift admin policy TYPO'
```

#### `os::cmd::expect_success_and_text CMD TEXT`
`expect_success_and_text` takes two arguments, the command to be run and the text that is expected, and runs the command. If the command
succeeds (its return code is `0`) *and* `stdout` or `stderr` contain the expected text, the utility function returns `0`. Otherwise, the
utility function returns `1`.

In order to test that a command succeeds and outputs some text, pass it to `os::cmd::expect_success_and_text` like:
```sh
$ os::cmd::expect_success_and_text 'oadm create-master-certs -h' 'Create keys and certificates'
```

In order to test that a command succeeds and outputs some text matching a regular expression, pass it to `os::cmd::expect_success_and_text` like:
```sh
$ os::cmd::expect_success_and_text "oc get imageStreams wildfly --template='{{index .metadata.annotations \"openshift.io/image.dockerRepositoryCheck\"}}'" '[0-9]{4}\-[0-9]{2}\-[0-9]{2}' # expect a date like YYYY-MM-DD
```

#### `os::cmd::expect_failure_and_text CMD TEXT`
`expect_failure_and_text` takes two arguments, the command to be run and the text that is expected, and runs the command. If the command
fails (its return code is not `0`) *and* `stdout` or `stderr` contain the expected text, the utility function returns `0`. Otherwise, the
utility function returns `1`.

In order to test that a command fails and outputs some text, pass it to `os::cmd::expect_failure_and_text` like:
```sh
$ os::cmd::expect_failure_and_text 'oc login --certificate-authority=/path/to/invalid' 'no such file or directory'
```

#### `os::cmd::expect_success_and_not_text CMD TEXT`
`expect_success_and_not_text` takes two arguments, the command to be run and the text that is not expected, and runs the command. If the
command succeeds (its return code is `0`) *and* `stdout` or `stderr` *do not* contain the text, the utility function returns `0`. Otherwise,
the utility function returns `1`.

In order to test that a command succeeds and does not output some text, pass it to `os::cmd::expect_success_and_not_text` like:
```sh
$ os::cmd::expect_success_and_not_text 'openshift' 'Atomic'
```

#### `os::cmd::expect_failure_and_not_text CMD TEXT`
`expect_failure_and_not_text` takes two arguments, the command to be run and the text that is not expected, and runs the command. If the
command fails (its return code is not `0`) *and* `stdout` or `stderr` *do not* contain the text, the utility function returns `0`. Otherwise,
the utility function returns `1`.

In order to test that a command fails and does not output some text, pass it to `os::cmd::expect_failure_and_not_text` like:
```sh
$ os::cmd::expect_failure_and_not_text 'oc get' 'NAME'
```

---

The utility functions that re-try the command until a condition is satisified or some time passes all default to trying the command
once every 200ms and time-out after one minute. The functions are:

#### `os::cmd::try_until_success CMD [TIMEOUT INTERVAL]`
`try_until_success` expects at least one argument, the command to be run, but will accept a second argument setting the timeout and a third
setting the command re-try interval. `try_until_success` will run the given command once every interval until the timeout, expecting it to
succeed (its exit code is `0`). If that occurs, the function will return `0`. Otherwise, the utility function will return `1`.

In order to re-try a command until it succeeds, pass it to `os::cmd::try_until_success` like:
```sh
$ os::cmd::try_until_success 'oc get imagestreamTags mysql:5.5'
```

#### `os::cmd::try_until_failure CMD [TIMEOUT INTERVAL]`
`try_until_failure` expects at least one argument, the command to be run, but will accept a second argument setting the timeout and a third
setting the command re-try interval. `try_until_failure` will run the given command once every interval until the timeout, expecting it to fail\
(its exit code is not `0`). If that occurs, the function will return `0`. Otherwise, the utility function will return `1`.

In order to re-try a command until it fails, pass it to `os::cmd::try_until_failure` like:
```sh
$ os::cmd::expect_success 'oc delete project recreated-project'
$ os::cmd::try_until_failure 'oc get project recreated-project'
```

#### `os::cmd::try_until_text CMD TEXT [TIMEOUT INTERVAL]`
`try_until_text` expects at least two arguments, the command to be run and the expected text, but will accept a third argument setting the
timeout and a fourth setting the command re-try interval. `try_until_text` will run the given command once every interval until the timeout,
expecting `stdout` and/or `stderr` to contain the expected text. If that occurs, the function will return `0`. Otherwise, the utility function
will return `1`.

In order to re-try a command until it outputs a certain text, without regard to its exit code, pass it to `os::cmd::try_until_text` like:
```sh
$ os::cmd::try_until_text 'oc get projects' 'ui-test-project'
```

---

The utility functions that allow a developer to expect a specific exit code are:

#### `os::cmd::expect_code CMD CODE`
`expect_code` takes two arguments, the command to be run and the code to be expected from it, and runs the command. If the command returns the
expected code, the utility function returns `0`. Otherwise, the utility function returns `1`.

#### `os::cmd::expect_code_and_text CMD CODE TEXT`
`expect_code_and_text` takes three arguments, the command to be run, the code and the text to be expected from it, and runs the command. If the
command returns the expected code *and* `stdout` or `stderr` contain the expected text, the utility function returns `0`. Otherwise, the utility
function returns `1`.


#### `os::cmd::expect_code_and_not_text CMD CODE TEXT`
`expect_code_and_not_text` takes three arguments, the command to be run, the code to be expected and the text not to be expected from it, and 
runs the command. If the command returns the expected code *and* `stdout` or `stderr` *do not* contain the expected text, the utility function
returns `0`. Otherwise, the utility function returns `1`.

### Correctly Quoting Text and Variables

In order to pass in a command that doesn't contain any quoted text, quote your command with single- or double-quotes:
```sh
$ os::cmd::expect_success 'oc get routes'
$ os::cmd::expect_success "oc get routes"
```

In order to pass in a command that contains literal text, use double quotes around the command and single-quotes around your text literal:
```sh
$ os::cmd::expect_success "oc get dc/ruby-hello-world --template='{{ .spec.replicas }}'"
```

In order to pass in a command that contains a `bash` variable you would like to be expanded, double-quote your command:
```sh
$ imagename="isimage/mysql@${name:0:15}"
$ os::cmd::expect_success "oc describe ${imagename}"
```

In order to pass in a command that contains something that looks like a `bash` variable, but isn't, escape the `$` with a forward-slash:
```sh
$ os::cmd::expect_success "oc new-build --dockerfile=\$'FROM centos:7\nRUN yum install -y httpd'"
```