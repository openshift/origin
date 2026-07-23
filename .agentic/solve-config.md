## Build, Test, and Verify

1. Run `make build` and `go vet ./...` to verify the build.
2. Run `go test ./pkg/...` to verify unit tests pass.
3. Run `make verify` to check for linting and verification issues.

## Environment

- Dependencies are vendored. Run `go mod tidy && go mod vendor` after changing `go.mod`.
