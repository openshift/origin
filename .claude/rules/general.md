---
paths:
  - "**"
---

### APM context generation

Sources under **`.apm/`** (instructions, prompts, `apm.yml`, etc.) drive generated agent context. After editing those files, run **`make apm`** to regenerate **AGENTS.md**, **CLAUDE.md**, **GEMINI.md**, and the integrated copies under **`.claude/`**, **`.cursor/`**, **`.gemini/`**, **`.github/instructions/`**, **`.github/prompts/`**, and **`.opencode/`**.

**Slash / agent commands:** content under **`.apm/prompts/*.prompt.md`** is the single source of truth. **`apm install`** (part of **`make apm`**) copies each prompt into editor command targets (e.g. **`.claude/commands/`**, **`.opencode/commands/`**, **`.gemini/commands/`**). Do not add those generated paths by hand or installs will skip them as unmanaged duplicates.

### Repository overview

**openshift/origin** builds the `openshift-tests` binary — the orchestrator for all OpenShift end-to-end testing. It contains:
* **Built-in e2e tests** (Ginkgo) that run against a live cluster.
* **Monitor tests** that observe cluster health in the background during e2e runs.
* **Disruption tests** (a category of monitor test) that measure endpoint availability.
* **Extension orchestration** that discovers and runs tests contributed by other components via the OTE framework.

### Key directories

* **`cmd/openshift-tests/`** — main binary entry point with subcommands: `run`, `run-test`, `run-monitor`, `run-upgrade`, `disruption`, `list`, `info`, `extension-admission`, and more.
* **`pkg/testsuites/`** — test suite definitions that compose built-in and extension tests.
* **`test/extended/`** — Ginkgo-based e2e test packages organized by area (e.g. `networking/`, `storage/`, `etcd/`).
* **`test/extended/include.go`** — blank imports that register all e2e test packages. New packages **must** be added here.
* **`test/e2e/upgrade/`** — upgrade-specific e2e tests.
* **`pkg/monitortestframework/`** — `MonitorTest` interface and registry.
* **`pkg/defaultmonitortests/`** — registers default monitor tests for stable and disruptive scenarios.
* **`pkg/monitortests/`** — individual monitor test implementations organized by component (e.g. `etcd/`, `kubeapiserver/`, `network/`, `node/`).
* **`pkg/monitortestlibrary/`** — shared monitor utilities (allowed disruption baselines, alert allowlists, platform identification).
* **`pkg/disruption/`** — disruption measurement framework (HTTP/gRPC sampling, transport hooks, shutdown detection).
* **`pkg/test/extensions/`** — OTE extension binary extraction, execution, and admission control.
* **`vendor/`** — vendored dependencies. Run `go mod tidy && go mod vendor` after changing `go.mod` to keep `go.sum` and `vendor/` in sync.

### E2e tests

Ginkgo-based tests in `test/extended/<area>/` run against a live OpenShift cluster. Each package is registered via blank import in `test/extended/include.go`. These are the "built-in" tests that ship inside the `openshift-tests` binary. Tests are grouped into suites (e.g. `openshift/conformance/parallel`) defined in `pkg/testsuites/`.

### Monitor tests

Monitor tests run **in parallel** with the e2e suite to continuously observe cluster health. They follow a lifecycle:
1. **`StartCollection()`** — begin watching resources, recording events.
2. *(e2e tests execute)*
3. **`CollectData()`** — final data collection pass.
4. **`ConstructComputedIntervals()`** — analyze raw data into structured intervals.
5. **`EvaluateTestsFromConstructedIntervals()`** — produce JUnit pass/fail results.

Monitor tests are registered in `pkg/defaultmonitortests/types.go` with two stability levels: **Stable** (normal operation — runs all monitors including availability checks) and **Disruptive** (tests that intentionally cause outages like upgrades, node drains, etcd recovery, or cert rotation — availability monitors are excluded since measuring uptime during intentional disruption would produce false failures). Implementations live in `pkg/monitortests/` organized by component — examples include etcd log analysis, cluster operator state tracking, node readiness, API server availability, and alert evaluation.

### Disruption tests

Disruption tests are a category of monitor test that measure **endpoint availability** during test runs. They continuously poll services (API servers, ingress, image registry, pod network) and record availability intervals. Allowed disruption baselines in `pkg/monitortestlibrary/allowedbackenddisruption/` define acceptable unavailability thresholds. Key implementations live in `pkg/monitortests/kubeapiserver/`, `pkg/monitortests/network/`, and `pkg/monitortests/imageregistry/`.

### Extensions (OTE — OpenShift Tests Extension)

The `openshift-tests` binary acts as an **orchestrator** for tests contributed by other components. The [openshift-tests-extension](https://github.com/openshift-eng/openshift-tests-extension) (OTE) framework defines a standardized interface that external binaries implement:
* **`<binary> info`** — return extension metadata (component, suites, version).
* **`<binary> list -o jsonl`** — list available tests, filtered by environment (platform, arch, network).
* **`<binary> run-test -n <name>`** — execute a specific test and return JSONL results.

**Payload extensions** are test binaries shipped inside release payload images. The registry in `pkg/test/extensions/binary.go` maps ~35 payload image tags to their test binary paths. At runtime, `openshift-tests` extracts these binaries, discovers their tests, and runs them alongside built-in tests.

**Non-payload extensions** are discovered from ImageStreamTags in the cluster and require explicit approval via a `TestExtensionAdmission` CRD. The `extension-admission` subcommand manages these approval rules.

Origin also registers itself as an extension (`openshift:payload:origin`) so its own built-in Ginkgo tests appear in the unified extension registry alongside external tests.

### Build and verify

* **Build:** `make build` or `make openshift-tests` (produces `./openshift-tests`).
* **Unit tests:** `go test ./pkg/...` (fast, no cluster required).
* **Lint / verify:** `make verify` (runs `go vet`, jsonformat, generated-file checks, TLS ownership).
* **Full check:** `make check` (verify + build + tests).

Favor clarity and maintainability over cleverness. Comments should be minimal, helpful, and explain the "why" not the "what".
