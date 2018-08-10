# Developer guidelines

## How to write validation tests

Each validation test has a `.go` file in the `validation/` directory and can be compiled into a `.t` file and executed independently.

### TAP output

Each validation test prints TAP output.
So far, we have three kinds of validation tests and they print the TAP output differently:

#### Using `util.RuntimeOutsideValidate`

They create a container but without executing `runtimetest`. The test program itself must print the TAP output.

Example:
```go
err = util.RuntimeOutsideValidate(g, t, func(config *rspec.Spec, t *tap.T, state *rspec.State) error {
        err := testFoo()
        t.Ok((err == nil), "check foo")
        if err != nil {
                t.Diagnostic(err.Error())
                return nil
        }
        return nil
})

```
#### Using `util.RuntimeInsideValidate` and passthrough

They start the process `runtimetest` inside the container and `runtimetest` prints the TAP output.
The test process itself must not output anything to avoid mixing its output with the TAP output.
Each test can only call `util.RuntimeInsideValidate` one time because several TAP outputs cannot be concatenated.

Example:
```go
err = util.RuntimeInsideValidate(g, nil, nil)
if err != nil {
        util.Fatal(err)
}
```

#### Using `util.RuntimeInsideValidate` and encapsulation

Similar to the passthrough variant but the test consumes the output from `runtimetest` and re-emit a single TAP result for the container run.
For that, the TAP object must be passed as parameter to `util.RuntimeInsideValidate`.

Example:
```go
g.AddAnnotation("TestName", "check foo")
err = util.RuntimeInsideValidate(g, t, nil)
if err != nil {
        util.Fatal(err)
}
```

### Exit status

When the runtime fails a test, the TAP output indicates so with "not ok" but the exit status of test program normally remains 0.
A non-zero exit status indicates a problem in the test program rather than in the runtime.
