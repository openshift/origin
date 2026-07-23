## Verify and Push

1. Run `make build` and `go vet ./...`.
2. Run `go test ./pkg/...` to verify unit tests pass.
3. Run `make verify` for lint checks.
4. Commit fixes referencing the review feedback.
5. Push: `git push fork HEAD` (or `git push origin HEAD`).

## Environment

- Dependencies are vendored. Run `go mod tidy && go mod vendor` after changing `go.mod`.
