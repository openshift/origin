# Developer guidelines

## How to write validation tests

Each validation test has a `.go` file in the `validation/` directory and can be compiled into a `.t` file and executed independently.

### TAP output

Each validation test prints TAP output.
So far, we have two kinds of validation tests and they print the TAP output differently:
* tests using `util.RuntimeInsideValidate`: they start the process `runtimetest` inside the container and `runtimetest` prints the TAP output. The test process itself must not output anything to avoid mixing its output with the TAP output. Each test can only call `util.RuntimeInsideValidate` one time because several TAP outputs cannot be concatenated.
* tests using `util.RuntimeOutsideValidate`: they create a container but without executing `runtimetest`. The test program itself must print the TAP output.

### Exit status

When the runtime fails a test, the TAP output indicates so with "not ok" but the exit status of test program normally remains 0.
A non-zero exit status indicates a problem in the test program rather than in the runtime.
