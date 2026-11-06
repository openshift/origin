# NRI Test Case Catalogue

Companion to [nri-test-plan.md](nri-test-plan.md). Detailed steps for every scenario implemented in [PR #31844](https://github.com/openshift/origin/pull/31844). All scenarios run on OCP 4.17+ (CRI-O 1.30+, NRI enabled by default) and target `test/extended/nri`.

Common setup for the e2e cases:

```sh
# plugin image for clusters whose payload does not carry nri-test-plugin yet
make -C test/extended/nri/plugin image IMAGE=quay.io/<you>/nri-test-plugin:dev
make -C test/extended/nri/plugin push  IMAGE=quay.io/<you>/nri-test-plugin:dev

# run the whole suite
NRI_TEST_PLUGIN_IMAGE=quay.io/<you>/nri-test-plugin:dev \
  openshift-tests run openshift/nri

# or one case (run-test matches the full spec name exactly)
openshift-tests run-test "[Suite:openshift/nri][sig-node] Node Resource Interface <spec name>"
```

Each case below lists the spec name that identifies it.

---

## TC-UNIT-001 — Annotation parser unit tests

- **Tier:** `unit`
- **Target:** `test/extended/nri/plugin/main_test.go`
- **Coverage:** Covered
- **Implementation:** Implemented in #31844; runs in the `nri-test-plugin` image build
- **OCP applicability:** all
- **Traceability:** parser correctness for the test plugin

**Steps**

1. `cd test/extended/nri/plugin && GOFLAGS=-mod=mod go test ./...`
   **Expected:** `ok`.
2. Inspect the table cases: key-value parsing (`A=1`, `A=b=c`, empty, missing `=`), mounts (`/host:/ctr` defaults to `rw`, `:ro`, unknown option stays `rw`, missing destination and too many fields error), devices (`/dev/x:c:1:3` defaults to mode `0666`, explicit mode `664`, missing minor, invalid major/minor/mode, negative mode).
   **Expected:** every valid case parses; every invalid case returns an error.

---

## TC-FUNC-001 — Plugin registers and synchronizes with CRI-O

- **Spec:** `enables NRI in CRI-O and registers the plugin`
- **Tier:** `e2e-serial`
- **Target:** `test/extended/nri/nri.go`
- **Coverage:** Covered
- **Implementation:** Implemented in #31844
- **OCP applicability:** 4.17+; skips when the plugin cannot connect (NRI disabled)
- **Traceability:** OCPNODE-4852 scope

**Steps**

1. Run the suite on a node with NRI enabled.
   **Expected:** the plugin DaemonSet becomes Ready on the selected node.
2. Read the plugin pod logs after `BeforeAll`.
   **Expected:** a `{"kind":"configure"}` line and at least one `synchronize-pod` or `synchronize-container` line.

---

## TC-FUNC-002 — Pod and container lifecycle events reach the plugin

- **Spec:** `relays pod and container lifecycle events to the plugin`
- **Tier:** `e2e-serial`
- **Target:** `test/extended/nri/nri.go`
- **Coverage:** Covered
- **Implementation:** Implemented in #31844
- **OCP applicability:** 4.17+
- **Traceability:** NRI event surface

**Steps**

1. Create a pod with `nri.openshift.io/test: "true"` and wait for Running.
   **Expected:** plugin logs gain `run-pod`, `create-container` and `start-container` for `<namespace>/<pod>`.
2. Delete the pod.
   **Expected:** plugin logs gain `stop-container`, `stop-pod` and `remove-pod` for the same pod within 2 minutes.

---

## TC-FUNC-003 — Environment, annotations, read-only mount and cpuset adjustments

- **Spec:** `injects environment, annotations, mounts and cpuset into opted-in containers`
- **Tier:** `e2e-serial`
- **Target:** `test/extended/nri/nri.go`
- **Coverage:** Covered
- **Implementation:** Implemented in #31844
- **OCP applicability:** 4.17+; skips when the node has a single online CPU
- **Traceability:** gpu-fractioning prerequisites (environment + mount injection at create time)

**Steps**

1. Create an opted-in pod with:
   - `nri.openshift.io/test-env: NRI_TEST_ENV=hello-from-nri,NRI_SECOND=2`
   - `nri.openshift.io/test-mounts: /proc/version:/mnt/nri/version:ro,/dev/null:/mnt/nri/null`
   - `nri.openshift.io/test-annotations: nri.openshift.io/injected=yes`
   - `nri.openshift.io/test-cpuset: <last CPU from node capacity>`
2. Check the plugin's `create-container` event.
   **Expected:** `adjustments` contains `env`, `mounts`, `annotations`, `cpuset`.
3. `env` in the container.
   **Expected:** both `NRI_TEST_ENV` and `NRI_SECOND` are present.
4. `cat /mnt/nri/version`.
   **Expected:** output contains `Linux version` (host file bind).
5. `echo nri > /mnt/nri/version`.
   **Expected:** fails with `Read-only file system` (the `ro` mount option was applied).
6. `stat -c '%F %t:%T' /mnt/nri/null` and `echo nri > /mnt/nri/null`.
   **Expected:** `character special file 1:3`; the write succeeds (writable device bind).
7. `grep Cpus_allowed_list /proc/self/status`.
   **Expected:** equals the requested CPU.

---

## TC-FUNC-004 — Device node injection and usability

- **Spec:** `injects device nodes into opted-in containers`
- **Tier:** `e2e-serial`
- **Target:** `test/extended/nri/nri.go`
- **Coverage:** Covered
- **Implementation:** Implemented in #31844
- **OCP applicability:** 4.17+
- **Traceability:** CRI-O `TestDeviceInjection` equivalent

**Steps**

1. Create an opted-in pod with `nri.openshift.io/test-devices: /dev/nri-null:c:1:3:666`.
2. `stat -c '%F %a %t:%T' /dev/nri-null`.
   **Expected:** `character special file 666 1:3`.
3. `dd if=/dev/nri-null of=/dev/null bs=1 count=1`.
   **Expected:** success (the injected device is usable, not just present).

---

## TC-FUNC-005 — Two plugins at different indices, in order, with merged adjustments

- **Spec:** `merges adjustments from two plugins with different indices`
- **Tier:** `e2e-serial`
- **Target:** `test/extended/nri/nri.go`
- **Coverage:** Covered
- **Implementation:** Implemented in #31844
- **OCP applicability:** 4.17+
- **Traceability:** NRI plugin ordering

**Steps**

1. Deploy a second plugin DaemonSet with `--idx=60`, `--extra-env=NRI_PLUGIN_B=yes` and `--ignore-annotations`; wait for its `configure` event.
2. Create an opted-in pod with `nri.openshift.io/test-env: NRI_PLUGIN_A=yes`.
3. `env` in the pod.
   **Expected:** both `NRI_PLUGIN_A=yes` (index 50, from the annotation) and `NRI_PLUGIN_B=yes` (index 60, from the flag) are present.
4. Read the `create-container` event from each plugin's logs.
   **Expected:** both plugins recorded the container; each timestamp is nonzero; the index-50 timestamp is less than or equal to the index-60 timestamp.

---

## TC-FUNC-006 — Pod creation while the plugin is down, then resynchronization

- **Spec:** `keeps creating pods while the plugin is down and resynchronizes when it returns`
- **Tier:** `e2e-serial`
- **Target:** `test/extended/nri/nri.go`
- **Coverage:** Covered
- **Implementation:** Implemented in #31844
- **OCP applicability:** 4.17+
- **Traceability:** NRI failure policy

**Steps**

1. Park the plugin DaemonSet on a nonexistent node name and wait until no plugin pod exists.
2. Create an opted-in pod.
   **Expected:** the pod reaches Running with no plugin registered.
3. Restore the DaemonSet.
   **Expected:** the new plugin pod logs a `configure` event and a `synchronize-pod` event for the pod created while it was down, within 3 minutes.

---

## TC-NEG-001 — A plugin error fails container creation and is logged

- **Spec:** `fails container creation when the plugin fails the adjustment`
- **Tier:** `e2e-serial`
- **Target:** `test/extended/nri/nri.go`
- **Coverage:** Covered
- **Implementation:** Implemented in #31844
- **OCP applicability:** 4.17+
- **Traceability:** NRI failure policy; observability

**Steps**

1. Create an opted-in pod with `nri.openshift.io/test-fail: "true"` and `activeDeadlineSeconds: 10`.
2. Watch pod events.
   **Expected:** a `Failed` event containing `failing CreateContainer on request of`.
3. Read the plugin logs.
   **Expected:** the same error string appears.
4. Wait for the pod phase.
   **Expected:** never Running; reaches `Failed` after the deadline (so the pod-lifecycle monitor does not flag it as stuck Pending).

---

## TC-NEG-002 — Pods without the opt-in annotation are not adjusted

- **Spec:** `does not adjust pods that do not opt in`
- **Tier:** `e2e-serial`
- **Target:** `test/extended/nri/nri.go`
- **Coverage:** Covered
- **Implementation:** Implemented in #31844
- **OCP applicability:** 4.17+
- **Traceability:** plugin gating

**Steps**

1. Create a pod with `nri.openshift.io/test-mounts: /proc/version:/mnt/nri/version:ro` and no `nri.openshift.io/test` annotation.
2. `env` and `cat /mnt/nri/version`.
   **Expected:** no injected environment; the file does not exist.
3. Read the plugin logs.
   **Expected:** no `create-container` event for that pod.

---

## TC-SEC-001 — Pods outside the plugin's namespace are not adjusted

- **Spec:** `ignores pods in another namespace`
- **Tier:** `e2e-serial`
- **Target:** `test/extended/nri/nri.go`
- **Coverage:** Covered
- **Implementation:** Implemented in #31844
- **OCP applicability:** 4.17+
- **Traceability:** privilege confinement

**Steps**

1. Create a second namespace with PSA `enforce=privileged` and the `privileged` SCC bound to its `default` service account (same privileges as the suite namespace).
2. Create an opted-in pod there with env and mount annotations.
3. `env` and `cat /mnt/nri/version`.
   **Expected:** no injected environment; the file does not exist.
4. Read the plugin logs.
   **Expected:** no `create-container` event for the other-namespace pod.

---

## TC-SEC-002 — A privileged plugin pod is rejected without SCC/PSA privileges

- **Spec:** `does not admit a privileged plugin pod without the suite's privileges`
- **Tier:** `e2e-serial`
- **Target:** `test/extended/nri/nri.go`
- **Coverage:** Covered
- **Implementation:** Implemented in #31844
- **OCP applicability:** 4.17+
- **Traceability:** privilege confinement

**Steps**

1. In a namespace with PSA `enforce=privileged` but no `privileged` SCC binding, create a Deployment with a privileged plugin-shaped pod (hostPath `/var/run/nri`).
2. Watch the ReplicaSet events.
   **Expected:** `FailedCreate` containing `security context constraint`.
3. In a default (unlabeled) namespace, create the same Deployment.
   **Expected:** `FailedCreate` matching `security context constraint|PodSecurity`.

---

## TC-NFR-001 — 10-pod churn with a plugin registered

- **Spec:** `starts a batch of pods with a plugin registered`
- **Tier:** `e2e-serial`
- **Target:** `test/extended/nri/nri.go`
- **Coverage:** Covered
- **Implementation:** Implemented in #31844
- **OCP applicability:** 4.17+
- **Traceability:** scale sanity

**Steps**

1. Create 10 opted-in pods (`nri-churn-0` … `nri-churn-9`) and wait for all to be Running (5-minute timeout).
2. Read the plugin logs.
   **Expected:** a `create-container` event for each distinct pod identity (not just 10 events).
3. Delete the pods.

---

## Deferred scenarios

Detailed steps for the proposed scenarios (`TC-UNIT-002`, `TC-NEG-003`, `TC-DEPLOY-001` to `TC-DEPLOY-004`, `TC-UPGRD-001`, `TC-NFR-002` to `TC-NFR-004`, `TC-OPS-001`) will be added when they are automated. See the plan's §17.2 and the Deferred Scenarios appendix for their tier, target file and rationale.
