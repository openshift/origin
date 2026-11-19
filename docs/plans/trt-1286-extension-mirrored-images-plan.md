# TRT-1286: Extension Binaries Should Be Able to Provide and Consume Mirrored Images

**Jira Issues:**
- Original: [TRT-1286](https://redhat.atlassian.net/browse/TRT-1286) - Extension binaries should be able to provide and consume mirrored images
- Planning subtask: [TRT-3108](https://redhat.atlassian.net/browse/TRT-3108) - Plan origin-side mirrored-image flag propagation for TRT-1286

**Target Repository:** openshift/origin

**Status:** Implementation plan (no code changes)

---

## 1. Scope and Desired Outcome

When `openshift-tests` runs in a disconnected or mirrored environment, test images are mirrored to a private registry using `openshift-tests images` + `oc image mirror`. Origin's built-in tests resolve mirrored pull specs via the `--from-repository` flag and the `image.LocationFor()` helper. However, extension binaries currently have **no mechanism** to learn where their images were mirrored to, so they cannot resolve mirrored pull specs at test runtime.

The goal is to:
1. Build a JSON mapping of original-to-mirrored image pull specs during the `openshift-tests` orchestration phase.
2. Pass that mapping to extension binaries when running tests (via `--mirrored-images <path>` or similar).
3. Enable each OTE extension binary to use a `LocationFor`-style helper (implemented in the OTE framework, tracked under TRT-3107) to look up the mirrored pull spec at test time.

This plan covers the **origin side** only: how origin produces, serializes, and propagates the mirrored-images data. The OTE-side `LocationFor` helper is a prerequisite (TRT-3107) and is out of scope here.

## 2. Current Behavior and Repository Findings

### 2.1 How image mirroring works today (origin built-in tests)

- `openshift-tests images --to-repository <repo>` outputs a mirror list mapping original pull specs to their mirrored equivalents (`pkg/cmd/openshift-tests/images/images_command.go`).
- At runtime, `openshift-tests run --from-repository <repo>` calls `image.InitializeImages(repo)` (`test/extended/util/image/image.go:115`), which computes `GetMappedImages(allowedImages, repo)` and stores the mapping in a package-global `images` map.
- `image.LocationFor(originalPullSpec)` returns the mirrored pull spec by looking up the global map (`test/extended/util/image/image.go:186`).
- The `--from-repository` value is propagated to child processes via the `KUBE_TEST_REPO` environment variable (`pkg/cmd/openshift-tests/run/options.go:49`).

### 2.2 How extension binaries are invoked

- `ExtractAllTestBinaries()` discovers and extracts extension binaries (`pkg/test/extensions/binary.go:646`).
- `TestBinary.RunTests()` executes `<binary> run-test -n <name> -o jsonl` in a subprocess (`pkg/test/extensions/binary.go:490-603`).
- The environment for the subprocess is built from `os.Environ()` merged with `updateEnvVars(c.env)` in `commandContext.RunTestInNewProcess()` (`pkg/test/ginkgo/test_runner.go:327`).
- `c.env` comes from `GinkgoRunSuiteOptions.CommandEnv`, populated by `TestCommandEnvironment()` (`pkg/cmd/openshift-tests/run/options.go:46-61`), which includes `KUBE_TEST_REPO`, `TEST_PROVIDER`, `TEST_JUNIT_DIR`, and `TEST_UPGRADE_OPTIONS`.
- `TestBinary.RunTests()` appends `EXTENSION_ARTIFACT_DIR` to the environment (`pkg/test/extensions/binary.go:505`).

### 2.3 How extension binaries report images

- `TestBinary.ListImages()` calls `<binary> images` and parses the JSON response (`pkg/test/extensions/binary.go:605-638`).
- Each image is an `extension.Image` struct with `Index`, `Registry`, `Name`, `Version`, and optional `Mapped` field (`vendor/github.com/openshift-eng/openshift-tests-extension/pkg/extension/types.go:89-97`).
- `TestBinaries.ListImages()` aggregates images from all binaries (`pkg/test/extensions/binary.go:975-1039`).
- The `images` subcommand already queries all extension binaries for their images (`pkg/cmd/openshift-tests/images/images_command.go:146-173`).

### 2.4 How k8s-tests-ext handles mirroring (special case)

- The `hyperkube` / `k8s-tests-ext` binary uses upstream kube's image resolution, which reads `KUBE_TEST_REPO` from the environment.
- Origin already passes `KUBE_TEST_REPO` to child processes, so k8s-tests-ext image mirroring works without additional changes.
- OTE extensions do **not** use kube's image resolution; they need their own mechanism.

### 2.5 Key distinction: origin does not generate the mapping

Per TRT-3108's clarification, origin does not generate the JSON file containing mirrored image mappings. The file is produced externally (by CI tooling or the operator) using the output of `openshift-tests images`. Origin's role is to **accept a path** to this file and **propagate** it to extension binaries.

However, origin needs to be able to **construct** the mapping from `--from-repository` and the images reported by extension binaries, since not all environments supply a pre-built JSON file. The `--from-repository` flag already implies a mapping (via `GetMappedImages()`), so origin can derive the mapping when no explicit file is provided.

## 3. Proposed Approach

### 3.1 New `--mirrored-images` flag on `run` and `run-upgrade`

Add a `--mirrored-images <path>` flag to the `run`, `run-upgrade`, and `run-monitor` subcommands (the same commands that already accept `--from-repository`).

- When `--mirrored-images` is provided, origin reads the JSON file and passes it through to extension binaries.
- When `--mirrored-images` is **not** provided but `--from-repository` is set, origin **generates** the mapping by querying all extension binaries for images (via `ListImages()`) and computing the mirrored pull specs (using the same logic as the `images` subcommand). The generated mapping is written to a temporary file.
- When neither flag is set, no mirrored-images data is passed (extensions use original pull specs).

### 3.2 JSON format for mirrored images

The JSON file is a simple object mapping original pull spec to mirrored pull spec:

```json
{
  "registry.k8s.io/e2e-test-images/agnhost:2.55": "mirror.example.com/test/repo:e2e-1-...",
  "quay.io/openshift/community-e2e-images:e2e-...": "mirror.example.com/test/repo:e2e-..."
}
```

This format is straightforward for `LocationFor()` lookups on the OTE side. It aligns with the acceptance criteria in TRT-1286: "JSON or other serialized data that references the mirrored pull specs."

### 3.3 Propagation mechanism

There are two reasonable propagation approaches:

**Option A: Environment variable (recommended)**
- Set `MIRRORED_IMAGES_FILE=<path>` in the environment passed to extension binaries.
- Consistent with existing patterns (`KUBE_TEST_REPO`, `TEST_PROVIDER`, `EXTENSION_ARTIFACT_DIR`).
- No changes needed to OTE's `run-test` command-line interface.
- Extension binaries read the env var in their `BeforeAll` or similar initialization hook.

**Option B: Command-line flag**
- Pass `--mirrored-images <path>` to `<binary> run-test`.
- Requires OTE to add a new flag to `run-test` (and possibly `list`).
- More explicit but requires coordinated changes to OTE's command protocol.

**Recommendation:** Use **Option A** (environment variable). It follows the established pattern for passing context to extension binaries, requires fewer coordinated changes, and avoids version-gating issues when old binaries don't understand new flags. The OTE-side `LocationFor` helper (TRT-3107) can read the env var and load the JSON file.

### 3.4 Mapping generation from `--from-repository`

When `--from-repository` is set but `--mirrored-images` is not, origin should auto-generate the mapping:

1. After extracting all test binaries and fetching their info, call `ListImages()` on all binaries.
2. For each image, compute the mirrored pull spec using the `--from-repository` value (same logic as `imageSetsForImage()` in `images_command.go`).
3. Also include origin's own `allowedImages` mappings (from `image.GetMappedImages()`).
4. Serialize the combined mapping to a temporary JSON file.
5. Set `MIRRORED_IMAGES_FILE` to point to this file.

This ensures that in the common `--from-repository` flow, extension binaries automatically get the right mirrored pull specs without requiring CI to separately produce a JSON file.

## 4. Implementation Steps

### Step 1: Define the mirrored-images JSON schema and serialization helpers

**Files to change:**
- New file: `pkg/clioptions/mirroredimages/mirrored_images.go` (or similar location under `pkg/clioptions/`)

**Details:**
- Define a `MirroredImages` type (likely `map[string]string`) with `Load(path) error` and `Save(path) error` methods.
- Add a `GenerateFromRepository(repo string, extensionImages []extension.Image, internalImages map[string]k8simage.ImageID) (*MirroredImages, error)` function that computes the full mapping.
- This logic can largely reuse `imageSetsForImage()` from `images_command.go` and `GetMappedImages()` from `image.go`.

### Step 2: Add `--mirrored-images` flag to `run`, `run-upgrade`, and `run-monitor`

**Files to change:**
- `pkg/cmd/openshift-tests/run/flags.go` — add `MirroredImagesFile string` field and bind `--mirrored-images` flag
- `pkg/cmd/openshift-tests/run/options.go` — add `MirroredImagesFile string` field, propagate via `TestCommandEnvironment()`
- `pkg/cmd/openshift-tests/run-upgrade/flags.go` — same changes
- `pkg/cmd/openshift-tests/run-upgrade/options.go` — same changes
- `pkg/cmd/openshift-tests/monitor/run/run_monitor_command.go` — same changes

**Details:**
- Add `--mirrored-images` flag: `flags.StringVar(&f.MirroredImagesFile, "mirrored-images", "", "Path to a JSON file mapping original image pull specs to their mirrored equivalents.")`
- In `TestCommandEnvironment()`, append `MIRRORED_IMAGES_FILE=<path>` when the value is non-empty.
- In `ToOptions()`, validate the file exists and is readable if specified.

### Step 3: Auto-generate mirrored-images mapping when `--from-repository` is set

**Files to change:**
- `pkg/cmd/openshift-tests/run/options.go` — in `Run()`, after binary extraction and before test execution
- `pkg/cmd/openshift-tests/run-upgrade/options.go` — same
- `pkg/test/ginkgo/cmd_runsuite.go` — may need to expose a hook or accept the mapping as a parameter

**Details:**
- If `MirroredImagesFile` is empty and `FromRepository` is non-empty, generate the mapping:
  1. After `ExtractAllTestBinaries()` and `allBinaries.Info()` in `GinkgoRunSuiteOptions.Run()`, call `allBinaries.ListImages()`.
  2. Compute the mapping using `GenerateFromRepository()`.
  3. Write to a temp file and set `MirroredImagesFile` on the options.
- This requires the mapping generation to happen inside `GinkgoRunSuiteOptions.Run()` where the binaries are available. Consider:
  - Adding `MirroredImagesFile` to `GinkgoRunSuiteOptions` and generating it there, or
  - Generating it in `RunSuiteOptions.Run()` / `RunUpgradeSuiteOptions.Run()` and passing it via `CommandEnv`.

**Assumption:** The auto-generation happens in `GinkgoRunSuiteOptions.Run()` after binary extraction since that's where the binaries are available. This keeps the generation close to where `CommandEnv` is consumed.

### Step 4: Pass `MIRRORED_IMAGES_FILE` through the test execution environment

**Files to change:**
- `pkg/test/ginkgo/cmd_runsuite.go` — ensure `CommandEnv` includes the env var
- `pkg/test/extensions/binary.go` — `RunTests()` already inherits env from the caller; verify no additional changes needed

**Details:**
- The existing flow in `RunTestInNewProcess()` builds `testEnv` from `os.Environ()` merged with `updateEnvVars(c.env)` (`test_runner.go:327`), and `RunTests()` uses that env (`binary.go:517`). As long as `MIRRORED_IMAGES_FILE` is in `CommandEnv`, it will propagate automatically.
- Verify that the file path remains valid for the lifetime of the test run (temp file cleanup must happen after all tests complete).

### Step 5: Origin self-consumption (origin's own extension)

**Files to change:**
- `pkg/test/extensions/binary.go` — in `InitializeOpenShiftTestsExtensionFramework()`, the `BeforeAll` hook

**Details:**
- Origin registers itself as an OTE extension and initializes via `BeforeAll` in `InitializeOpenShiftTestsExtensionFramework()` (`binary.go:112-148`).
- Origin's built-in tests already use `image.InitializeImages()` with `KUBE_TEST_REPO`, so they don't need the new env var.
- No changes needed here unless we want to unify origin's internal image resolution with the new mechanism in a future step.

### Step 6: Update `openshift-tests images` to output the JSON mapping format

**Files to change:**
- `pkg/cmd/openshift-tests/images/images_command.go` — add a `--output-mirrored-images <path>` flag (or `--output json`)

**Details:**
- Add an option to output the mapping as a JSON file (the `map[string]string` format) instead of or in addition to the `oc image mirror` text format.
- This allows CI pipelines to produce the JSON file that `--mirrored-images` consumes:
  ```
  openshift-tests images --to-repository mirror.example.com/test/repo --output-mirrored-images /path/to/mirrored-images.json
  ```
- This step is optional for the initial implementation since `--from-repository` auto-generation covers the common case, but it enables the external-file workflow described in TRT-3108.

## 5. Validation

### Unit Tests

- **Serialization round-trip:** Test `MirroredImages.Save()` / `MirroredImages.Load()` with valid and invalid JSON.
- **Mapping generation:** Test `GenerateFromRepository()` with known extension images and verify the output matches expected mirrored pull specs.
- **Environment propagation:** Test `TestCommandEnvironment()` includes `MIRRORED_IMAGES_FILE` when set, and omits it when empty.
- **Flag validation:** Test that `--mirrored-images` with a non-existent file path produces a clear error.

### Integration / Manual Verification

- Build `openshift-tests` (`make build`) and verify:
  - `openshift-tests run --from-repository <repo> --dry-run <suite>` succeeds and logs that a mirrored-images file was generated.
  - `openshift-tests run --mirrored-images /path/to/file --dry-run <suite>` succeeds and logs the loaded mapping.
  - `openshift-tests images --to-repository <repo> --output-mirrored-images /tmp/test.json` produces valid JSON (if step 6 is implemented).
- Verify that `MIRRORED_IMAGES_FILE` appears in the environment of extension binary subprocesses (can be checked via test logging or a debug extension).

### Existing Checks

- `make build` — verify compilation.
- `go vet ./...` — static analysis.
- `go test ./pkg/...` — existing unit tests continue to pass.
- `make verify` — lint, generated files, and verification checks.

## 6. Risks and Compatibility

### Backward Compatibility

- The `--mirrored-images` flag is entirely opt-in. Existing invocations without the flag behave identically.
- Auto-generation from `--from-repository` is additive: it sets an env var that old extension binaries will simply ignore.
- Extension binaries that don't understand `MIRRORED_IMAGES_FILE` are unaffected; they continue using their original pull specs (which may fail in disconnected environments, but that's the existing behavior).

### Ordering / Sequencing

- **TRT-3107 (OTE-side)** must be completed first or in parallel. The OTE `LocationFor` helper reads `MIRRORED_IMAGES_FILE` and resolves pull specs. Without it, the origin-side propagation has no consumer.
- The origin changes can be developed and tested independently (the env var is set even if no extension reads it yet).

### Performance

- Auto-generating the mapping from `--from-repository` requires calling `ListImages()` on all binaries during the `run` command. This is an additional subprocess invocation per binary (similar cost to `info` and `list`). The `images` subcommand already does this, so it is a known-acceptable cost.
- The JSON file is small (a few hundred entries at most) and read once per extension binary startup.

### Edge Cases

- **No images reported:** Some extensions report no images. The mapping will simply have no entries for them — this is correct.
- **Duplicate image indexes across binaries:** The mapping is keyed by original pull spec, not index, so duplicates are handled naturally.
- **File lifetime:** The temp file for auto-generated mappings must outlive all test executions. It should be created alongside other temp resources in `GinkgoRunSuiteOptions.Run()` and cleaned up in the deferred cleanup chain.
- **Multiple `--from-repository` values:** Only one repository is supported today; the mapping is computed for a single target repo.

## 7. Open Questions

1. **Environment variable vs. flag for propagation:** This plan recommends an environment variable (`MIRRORED_IMAGES_FILE`). If the OTE team prefers a command-line flag on `run-test`, the propagation in `TestBinary.RunTests()` would need to append `--mirrored-images <path>` to the command args instead. This requires coordination with TRT-3107 and may need API version gating (`SinceVersion`). **Impact:** Changes the propagation mechanism in steps 2-4. **Resolution:** Confirm with TRT-3107 implementer which approach OTE will support.

2. **JSON format details:** This plan proposes a flat `map[string]string` (original pull spec -> mirrored pull spec). An alternative is a structured format with metadata (e.g., source component, index). **Impact:** Affects serialization helpers and OTE-side parsing. **Resolution:** Agree on format with TRT-3107 before implementation.

3. **Should `--mirrored-images` also affect origin's built-in `LocationFor()`?** Currently, origin's built-in image resolution uses `--from-repository` exclusively. If `--mirrored-images` is provided, should origin's own `image.LocationFor()` also consult it? This would allow unifying the two mechanisms long-term. **Impact:** Additional changes to `test/extended/util/image/image.go`. **Resolution:** Decide whether to unify now or in a follow-up.

4. **Auto-generation timing in `GinkgoRunSuiteOptions.Run()`:** The mapping generation needs to happen after `ExtractAllTestBinaries()` (to have binaries available for `ListImages()`) but before test execution begins. The current code structure in `cmd_runsuite.go` supports this, but the mapping generation adds a new phase. **Impact:** Increases startup time slightly. **Resolution:** Profile if concerned; the `images` command already performs this work successfully.
