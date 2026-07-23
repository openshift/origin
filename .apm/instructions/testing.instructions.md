---
description: "Testing guidelines and patterns for origin"
applyTo: "**/*_test.go"
---

* Use `go vet ./...` and `go test ./pkg/...` to validate changes before committing.
* E2e tests in `test/extended/<area>/` are **platform-level tests for OpenShift** — they run against a live cluster to validate the product, not just this repo. They use the **Ginkgo** framework and are compiled into the `openshift-tests` binary that CI runs against every payload.
* `test/extended/include.go` blank-imports every e2e test package — new packages **must** be added there or they won't be compiled into the binary.
* Unit tests in `pkg/` use standard Go `testing` with **table-driven** patterns.
* Build the test binary: `make openshift-tests` or `make build`.
* For unit tests, prefer **table-driven tests** with descriptive case names. Search the same package for existing test patterns before writing new ones.
