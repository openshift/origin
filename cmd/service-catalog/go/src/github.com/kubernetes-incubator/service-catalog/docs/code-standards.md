# Code & Documentation Standards

This document details our goals for code quality, coverage and documentation standards. At the time
of this writing, these are standards that all of our code may not meet. We should always try to 
improve our codebase and documentation to meet them.

While we do not currently aim to adhere completely to the 
[Kubernetes coding conventions](https://github.com/kubernetes/community/blob/master/contributors/devel/coding-conventions.md),
we aspire to adhere as closely as possible.

## Code Quality And Documentation Standards

We aim to write clear code that is easy to read and comprehend. In particular, that goal means the
following:

1. All Go code must pass the Go linter checks
    - We have these checks integrated into our CI system
2. All Go code should pass `go vet`
    - This check is integrated into our CI system
    - Anyone with the Go toolchain installed on their system can run the vet checks by executing the
    following command on their machine:

    ```go
    go vet ./pkg/... ./cmd/... ./test/...
    ```
3. All Go code must be formatted with [gofmt](https://golang.org/cmd/gofmt/)
    - Anyone can run the formatter by runinng `make format` on their machine
4. Any exported symbols (`type`s, `interface`s, `func`s, `struct`s, etc...) must have Godoc
compatible comments associated with them
5. Unexported symbols must be commented sufficiently to provide direction & context to a developer 
who didn't write the code
6. Inline code should be commented sufficiently to explain what complex code is doing. It's up to
the developer and reviewers how much and what kind of documentation is necessary
7. Unit, integration or end-to-end tests should be written for all business logic
    - Reviewers should take care to ensure the right, and enough testing is written for a PR
    - We do not yet check code coverage in our CI system



