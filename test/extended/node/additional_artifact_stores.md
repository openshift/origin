# Additional Artifact Stores - Manual Test Cases

## Feature Overview

**Feature Name:** Additional Artifact Stores for CRI-O  
**Epic:** OCPSTRAT-2623  
**Target Release:** OpenShift 4.22 (Tech Preview)  
**Feature Gate:** `AdditionalStorageConfig` (TechPreviewNoUpgrade)

### What is it?

The Additional Artifact Stores feature enables users to configure multiple read-only artifact storage locations in CRI-O. This allows OpenShift to pull OCI volume images (artifacts) from pre-configured storage locations separate from the default `/var/lib/containers/storage/artifacts/` directory.

### Why is it needed?

**Primary Use Case:** RHOAI (Red Hat OpenShift AI) workloads with large ML models

**Problem Being Solved:**
- Large AI/ML models (10GB+) require high-performance storage separate from OS containers
- Default artifact location is hardcoded to `/var/lib/containers/storage/artifacts/`
- Cannot leverage dedicated SSD storage for ML models
- Cannot pre-populate artifact caches across cluster nodes
- Root filesystem can fill up with large artifacts

**Key Benefits:**
- **Performance:** Dedicated high-speed storage (e.g., SSD) for ML models
- **Pre-population:** Support for pre-cached artifacts in air-gapped deployments
- **Separation:** Keep large artifacts separate from root filesystem
- **Flexibility:** Multiple artifact storage locations at node level

### How does it work?

1. **API Configuration:** Users define additional artifact storage paths via `ContainerRuntimeConfig`
2. **MCO Generation:** Machine Config Operator generates CRI-O TOML configuration
3. **CRI-O Resolution:** CRI-O searches for artifacts sequentially across configured stores
4. **First Match Wins:** First artifact found in any store is used

**Resolution Order:**
```
1. Default store: /var/lib/containers/storage/artifacts/
2. Additional store 1 (if configured)
3. Additional store 2 (if configured)
...
N. Additional store N (up to 10 total)
```

### API Example

```yaml
apiVersion: machineconfiguration.openshift.io/v1
kind: ContainerRuntimeConfig
metadata:
  name: set-additional-artifact-stores
spec:
  machineConfigPoolSelector:
    matchLabels:
      pools.operator.machineconfiguration.openshift.io/worker: ""
  containerRuntimeConfig:
    additionalArtifactStores:
      - path: /mnt/ssd-artifacts      # High-performance SSD storage
      - path: /mnt/nfs-artifacts      # Shared NFS cache
```

### Generated CRI-O Configuration

```toml
[crio.runtime.artifacts]
artifact_stores = [
    "/var/lib/containers/storage/artifacts",  # default (always first)
    "/mnt/ssd-artifacts",
    "/mnt/nfs-artifacts"
]
```

### Constraints

- **Maximum:** Up to 10 additional artifact stores per ContainerRuntimeConfig
- **Path Format:** Must be absolute paths (starting with `/`)
- **Path Length:** Maximum 256 characters
- **Invalid Characters:** No spaces, `@`, `!`, `#`, `$`, `%`
- **Uniqueness:** No duplicate paths within the same configuration
- **Read-Only:** All additional stores are read-only
- **MCP Requirement:** Must specify a valid `machineConfigPoolSelector`

### Related Documentation

- Enhancement Proposal: https://github.com/saschagrunert/enhancements/blob/525af9bbc3e9eefc82557f69850429ef8ffce30a/enhancements/machine-config/additional-storage-config-crio.md
- Epic: OCPSTRAT-2623 - Additional Artifact Store - 4.22 -TP
- Related Epic: OCPNODE-4051 - CRI-O Additional Storage Support
- Documentation: OSDOCS-17312

---

## Common Prerequisites (All Test Cases)

- OpenShift 4.22+ cluster
- `AdditionalStorageConfig` feature gate enabled (TechPreviewNoUpgrade feature set)
- Cluster admin access (`oc` CLI configured)
- Permission to create `ContainerRuntimeConfig` resources
- Worker nodes with available storage paths
- Understanding that ContainerRuntimeConfig changes trigger MCP rollouts (disruptive)

### Verify Feature Gate is Enabled

```bash
oc get featuregate cluster -o yaml | grep -A5 "AdditionalStorageConfig"
```

Expected: Feature should appear in `status.featureGates[].enabled[]` list

---

## API Validation Test Cases

### Test Case: TC1 - Invalid Path Formats

**Description:**
Verify that the API rejects `additionalArtifactStores` paths that don't meet format requirements. Paths must be absolute (starting with `/`) and cannot be empty.

**Prerequisites:**
- Common prerequisites above
- No existing ContainerRuntimeConfig named `invalid-path-test-*`

**Manual Steps:**

1. Attempt to create ContainerRuntimeConfig with a relative path:
   ```bash
   cat <<EOF | oc apply -f -
   apiVersion: machineconfiguration.openshift.io/v1
   kind: ContainerRuntimeConfig
   metadata:
     name: invalid-path-test-relative
   spec:
     machineConfigPoolSelector:
       matchLabels:
         pools.operator.machineconfiguration.openshift.io/worker: ""
     containerRuntimeConfig:
       additionalArtifactStores:
       - path: "relative/path"
   EOF
   ```

2. Verify the error response:
   ```bash
   # Should fail immediately with validation error
   ```

3. Attempt to create ContainerRuntimeConfig with an empty path:
   ```bash
   cat <<EOF | oc apply -f -
   apiVersion: machineconfiguration.openshift.io/v1
   kind: ContainerRuntimeConfig
   metadata:
     name: invalid-path-test-empty
   spec:
     machineConfigPoolSelector:
       matchLabels:
         pools.operator.machineconfiguration.openshift.io/worker: ""
     containerRuntimeConfig:
       additionalArtifactStores:
       - path: ""
   EOF
   ```

4. Verify the error response

**Expected Result:**
- Both create operations must fail with validation errors
- Error messages should clearly indicate the path format requirement
- No ContainerRuntimeConfig resources should be created
- MachineConfigPool should remain unchanged (no rollout triggered)

**Cleanup:**
```bash
# No cleanup needed - resources were not created
```

---

### Test Case: TC2 - Exceed Maximum Count

**Description:**
Verify that the API rejects `additionalArtifactStores` configurations with more than 10 artifact stores (the maximum allowed limit).

**Prerequisites:**
- Common prerequisites above
- No existing ContainerRuntimeConfig named `exceed-limit-test`

**Manual Steps:**

1. Create a ContainerRuntimeConfig with 11 artifact stores (exceeds limit of 10):
   ```bash
   cat <<EOF | oc apply -f -
   apiVersion: machineconfiguration.openshift.io/v1
   kind: ContainerRuntimeConfig
   metadata:
     name: exceed-limit-test
   spec:
     machineConfigPoolSelector:
       matchLabels:
         pools.operator.machineconfiguration.openshift.io/worker: ""
     containerRuntimeConfig:
       additionalArtifactStores:
       - path: "/mnt/artifactstore-0"
       - path: "/mnt/artifactstore-1"
       - path: "/mnt/artifactstore-2"
       - path: "/mnt/artifactstore-3"
       - path: "/mnt/artifactstore-4"
       - path: "/mnt/artifactstore-5"
       - path: "/mnt/artifactstore-6"
       - path: "/mnt/artifactstore-7"
       - path: "/mnt/artifactstore-8"
       - path: "/mnt/artifactstore-9"
       - path: "/mnt/artifactstore-10"
   EOF
   ```

2. Check the error response:
   ```bash
   # Command should fail with validation error
   ```

**Expected Result:**
- Create operation must fail with a validation error
- Error message should contain text like "must have at most 10" or similar indicating the maximum limit
- No ContainerRuntimeConfig resource should be created
- MachineConfigPool should remain unchanged

**Cleanup:**
```bash
# No cleanup needed - resource was not created
```

---

### Test Case: TC3 - Duplicate Paths

**Description:**
Verify that the API rejects duplicate paths within the same `additionalArtifactStores` configuration to prevent misconfiguration.

**Prerequisites:**
- Common prerequisites above
- No existing ContainerRuntimeConfig named `duplicate-path-test`

**Manual Steps:**

1. Attempt to create ContainerRuntimeConfig with duplicate paths:
   ```bash
   cat <<EOF | oc apply -f -
   apiVersion: machineconfiguration.openshift.io/v1
   kind: ContainerRuntimeConfig
   metadata:
     name: duplicate-path-test
   spec:
     machineConfigPoolSelector:
       matchLabels:
         pools.operator.machineconfiguration.openshift.io/worker: ""
     containerRuntimeConfig:
       additionalArtifactStores:
       - path: "/mnt/shared-artifacts"
       - path: "/mnt/shared-artifacts"
   EOF
   ```

2. Verify the error response:
   ```bash
   # Should fail with duplicate path error
   ```

**Expected Result:**
- Create operation must fail with a validation error
- Error should indicate duplicate paths are not allowed
- No ContainerRuntimeConfig resource should be created
- MachineConfigPool should remain unchanged

**Cleanup:**
```bash
# No cleanup needed - resource was not created
```

---

### Test Case: TC4 - Path Containing Spaces

**Description:**
Verify that paths containing spaces are rejected by the API to prevent shell injection and parsing issues.

**Prerequisites:**
- Common prerequisites above
- No existing ContainerRuntimeConfig named `artifactstore-path-spaces-test`

**Manual Steps:**

1. Attempt to create ContainerRuntimeConfig with a path containing spaces:
   ```bash
   cat <<EOF | oc apply -f -
   apiVersion: machineconfiguration.openshift.io/v1
   kind: ContainerRuntimeConfig
   metadata:
     name: artifactstore-path-spaces-test
   spec:
     machineConfigPoolSelector:
       matchLabels:
         pools.operator.machineconfiguration.openshift.io/worker: ""
     containerRuntimeConfig:
       additionalArtifactStores:
       - path: "/var/lib/artifact store"
   EOF
   ```

2. Verify the error response

**Expected Result:**
- Create operation must fail with a validation error
- Error should indicate that spaces are not allowed in paths
- No ContainerRuntimeConfig resource should be created
- MachineConfigPool should remain unchanged

**Cleanup:**
```bash
# No cleanup needed - resource was not created
```

---

### Test Case: TC5 - Path Containing Invalid Characters

**Description:**
Verify that paths containing invalid special characters (`@`, `!`, `#`, `$`, `%`) are rejected by the API to prevent security issues and parsing errors.

**Prerequisites:**
- Common prerequisites above
- No existing ContainerRuntimeConfig named `artifactstore-invalid-char-*`

**Manual Steps:**

1. Test path with `@` symbol:
   ```bash
   cat <<EOF | oc apply -f -
   apiVersion: machineconfiguration.openshift.io/v1
   kind: ContainerRuntimeConfig
   metadata:
     name: artifactstore-invalid-char-at
   spec:
     machineConfigPoolSelector:
       matchLabels:
         pools.operator.machineconfiguration.openshift.io/worker: ""
     containerRuntimeConfig:
       additionalArtifactStores:
       - path: "/var/lib/artifact@store"
   EOF
   ```

2. Test path with `!` symbol:
   ```bash
   cat <<EOF | oc apply -f -
   apiVersion: machineconfiguration.openshift.io/v1
   kind: ContainerRuntimeConfig
   metadata:
     name: artifactstore-invalid-char-exclamation
   spec:
     machineConfigPoolSelector:
       matchLabels:
         pools.operator.machineconfiguration.openshift.io/worker: ""
     containerRuntimeConfig:
       additionalArtifactStores:
       - path: "/var/lib/artifact!store"
   EOF
   ```

3. Test path with `#` symbol:
   ```bash
   cat <<EOF | oc apply -f -
   apiVersion: machineconfiguration.openshift.io/v1
   kind: ContainerRuntimeConfig
   metadata:
     name: artifactstore-invalid-char-hash
   spec:
     machineConfigPoolSelector:
       matchLabels:
         pools.operator.machineconfiguration.openshift.io/worker: ""
     containerRuntimeConfig:
       additionalArtifactStores:
       - path: "/var/lib/artifact#store"
   EOF
   ```

4. Test path with `$` symbol:
   ```bash
   cat <<EOF | oc apply -f -
   apiVersion: machineconfiguration.openshift.io/v1
   kind: ContainerRuntimeConfig
   metadata:
     name: artifactstore-invalid-char-dollar
   spec:
     machineConfigPoolSelector:
       matchLabels:
         pools.operator.machineconfiguration.openshift.io/worker: ""
     containerRuntimeConfig:
       additionalArtifactStores:
       - path: "/var/lib/artifact$store"
   EOF
   ```

5. Test path with `%` symbol:
   ```bash
   cat <<EOF | oc apply -f -
   apiVersion: machineconfiguration.openshift.io/v1
   kind: ContainerRuntimeConfig
   metadata:
     name: artifactstore-invalid-char-percent
   spec:
     machineConfigPoolSelector:
       matchLabels:
         pools.operator.machineconfiguration.openshift.io/worker: ""
     containerRuntimeConfig:
       additionalArtifactStores:
       - path: "/var/lib/artifact%store"
   EOF
   ```

**Expected Result:**
- All create operations must fail with validation errors
- Errors should indicate invalid characters in the path
- No ContainerRuntimeConfig resources should be created
- MachineConfigPool should remain unchanged

**Cleanup:**
```bash
# No cleanup needed - resources were not created
```

---

### Test Case: TC6 - Path Exceeding 256 Characters

**Description:**
Verify that paths longer than 256 characters are rejected by the API to prevent filesystem limitations and potential issues.

**Prerequisites:**
- Common prerequisites above
- No existing ContainerRuntimeConfig named `long-path-test`

**Manual Steps:**

1. Attempt to create ContainerRuntimeConfig with a 257-character path:
   ```bash
   cat <<EOF | oc apply -f -
   apiVersion: machineconfiguration.openshift.io/v1
   kind: ContainerRuntimeConfig
   metadata:
     name: long-path-test
   spec:
     machineConfigPoolSelector:
       matchLabels:
         pools.operator.machineconfiguration.openshift.io/worker: ""
     containerRuntimeConfig:
       additionalArtifactStores:
       - path: "/very/long/path/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
   EOF
   ```
   *(Note: Ensure the path is exactly 257 characters long)*

2. Verify the error response

**Expected Result:**
- Create operation must fail with a validation error
- Error should indicate maximum path length exceeded (max 256 characters)
- No ContainerRuntimeConfig resource should be created
- MachineConfigPool should remain unchanged

**Cleanup:**
```bash
# No cleanup needed - resource was not created
```

---

### Test Case: TC7 - Path with Consecutive Forward Slashes

**Description:**
Verify that paths with consecutive forward slashes (e.g., `//`) are rejected to ensure path normalization and prevent potential issues.

**Prerequisites:**
- Common prerequisites above
- No existing ContainerRuntimeConfig named `consecutive-slashes-test`

**Manual Steps:**

1. Attempt to create ContainerRuntimeConfig with consecutive slashes in path:
   ```bash
   cat <<EOF | oc apply -f -
   apiVersion: machineconfiguration.openshift.io/v1
   kind: ContainerRuntimeConfig
   metadata:
     name: consecutive-slashes-test
   spec:
     machineConfigPoolSelector:
       matchLabels:
         pools.operator.machineconfiguration.openshift.io/worker: ""
     containerRuntimeConfig:
       additionalArtifactStores:
       - path: "/var//lib//artifacts"
   EOF
   ```

2. Verify the error response

**Expected Result:**
- Create operation must fail with a validation error
- Error should indicate consecutive slashes are not allowed
- No ContainerRuntimeConfig resource should be created
- MachineConfigPool should remain unchanged

**Cleanup:**
```bash
# No cleanup needed - resource was not created
```

---

## E2E Functional Test Cases

### Test Case: TC8 - E2E Lifecycle Test

**Description:**
Verify the complete end-to-end lifecycle of configuring `additionalArtifactStores`: create ContainerRuntimeConfig, verify MCP rollout, check CRI-O configuration generation, and perform cleanup.

**Prerequisites:**
- Common prerequisites above
- Worker nodes with at least 5GB free disk space
- No existing ContainerRuntimeConfig named `additional-artifactstore-test`
- Worker MachineConfigPool in healthy state (UPDATED=True, DEGRADED=False)

**Manual Steps:**

**Phase 1: Create ContainerRuntimeConfig**

1. Create ContainerRuntimeConfig with artifact store path:
   ```bash
   cat <<EOF | oc apply -f -
   apiVersion: machineconfiguration.openshift.io/v1
   kind: ContainerRuntimeConfig
   metadata:
     name: additional-artifactstore-test
   spec:
     machineConfigPoolSelector:
       matchLabels:
         pools.operator.machineconfiguration.openshift.io/worker: ""
     containerRuntimeConfig:
       additionalArtifactStores:
       - path: "/mnt/artifacts-cache"
   EOF
   ```

2. Verify resource created:
   ```bash
   oc get containerruntimeconfig additional-artifactstore-test
   ```

**Phase 2: Monitor MachineConfigPool Rollout**

3. Check initial MCP status:
   ```bash
   oc get mcp worker
   ```

4. Wait for MCP to begin updating:
   ```bash
   oc get mcp worker -w
   # Watch for UPDATING=True
   ```

5. Monitor rollout completion (may take 10-20 minutes):
   ```bash
   # Wait until:
   # UPDATED=True
   # UPDATING=False
   # DEGRADED=False
   # MACHINECOUNT=READYMACHINECOUNT=UPDATEDMACHINECOUNT
   ```

**Phase 3: Verify CRI-O Configuration**

6. Select a worker node to verify:
   ```bash
   WORKER_NODE=$(oc get nodes -l node-role.kubernetes.io/worker= -o jsonpath='{.items[0].metadata.name}')
   echo "Verifying on node: $WORKER_NODE"
   ```

7. Check CRI-O configuration file was created:
   ```bash
   oc debug node/$WORKER_NODE -- chroot /host \
     cat /etc/crio/crio.conf.d/01-ctrcfg-additional-artifactstore-test
   ```

8. Verify configuration contains the artifact store path:
   ```bash
   # Output should contain:
   # [crio.runtime.artifacts]
   # artifact_stores = ["/mnt/artifacts-cache"]
   ```

9. Verify CRI-O service is healthy:
   ```bash
   oc debug node/$WORKER_NODE -- chroot /host systemctl status crio
   # Should show: active (running)
   ```

**Phase 4: Verify Node Status**

10. Check all worker nodes are Ready:
    ```bash
    oc get nodes -l node-role.kubernetes.io/worker=
    # All nodes should show STATUS=Ready
    ```

**Phase 5: Cleanup**

11. Delete the ContainerRuntimeConfig:
    ```bash
    oc delete containerruntimeconfig additional-artifactstore-test
    ```

12. Monitor MCP rollout to remove configuration:
    ```bash
    oc get mcp worker -w
    # Wait for rollout to complete (UPDATED=True)
    ```

13. Verify CRI-O configuration file was removed:
    ```bash
    oc debug node/$WORKER_NODE -- chroot /host \
      ls /etc/crio/crio.conf.d/ | grep additional-artifactstore-test
    # Should return no results
    ```

**Expected Result:**
- Phase 1: ContainerRuntimeConfig created successfully
- Phase 2: MCP rollout completes successfully within 20 minutes, all nodes updated
- Phase 3: CRI-O config file exists with correct `artifact_stores` configuration
- Phase 3: CRI-O service remains healthy
- Phase 4: All worker nodes remain in Ready state
- Phase 5: MCP rollout completes successfully, configuration file removed
- No errors or degraded states throughout the lifecycle

**Cleanup:**
```bash
# Already performed in Phase 5, verify:
oc get containerruntimeconfig additional-artifactstore-test
# Should return: Error from server (NotFound)
```

---

### Test Case: TC9 - Update Configuration

**Description:**
Verify that modifying an existing ContainerRuntimeConfig with different `additionalArtifactStores` paths triggers MCP rollout and updates CRI-O configuration correctly.

**Prerequisites:**
- Common prerequisites above
- Worker nodes with available storage paths
- No existing ContainerRuntimeConfig named `update-artifactstore-test`
- Worker MCP in healthy state

**Manual Steps:**

**Phase 1: Create Initial Configuration**

1. Create ContainerRuntimeConfig with one artifact store:
   ```bash
   cat <<EOF | oc apply -f -
   apiVersion: machineconfiguration.openshift.io/v1
   kind: ContainerRuntimeConfig
   metadata:
     name: update-artifactstore-test
   spec:
     machineConfigPoolSelector:
       matchLabels:
         pools.operator.machineconfiguration.openshift.io/worker: ""
     containerRuntimeConfig:
       additionalArtifactStores:
       - path: "/mnt/artifacts-v1"
   EOF
   ```

2. Wait for initial MCP rollout to complete:
   ```bash
   oc get mcp worker -w
   # Wait for UPDATED=True
   ```

3. Verify initial CRI-O configuration:
   ```bash
   WORKER_NODE=$(oc get nodes -l node-role.kubernetes.io/worker= -o jsonpath='{.items[0].metadata.name}')
   oc debug node/$WORKER_NODE -- chroot /host \
     cat /etc/crio/crio.conf.d/01-ctrcfg-update-artifactstore-test
   
   # Should contain: artifact_stores = ["/mnt/artifacts-v1"]
   ```

**Phase 2: Update Configuration**

4. Modify the ContainerRuntimeConfig to change paths:
   ```bash
   cat <<EOF | oc apply -f -
   apiVersion: machineconfiguration.openshift.io/v1
   kind: ContainerRuntimeConfig
   metadata:
     name: update-artifactstore-test
   spec:
     machineConfigPoolSelector:
       matchLabels:
         pools.operator.machineconfiguration.openshift.io/worker: ""
     containerRuntimeConfig:
       additionalArtifactStores:
       - path: "/mnt/artifacts-v2"
       - path: "/mnt/artifacts-v3"
   EOF
   ```

5. Monitor MCP rollout for the update:
   ```bash
   oc get mcp worker -w
   # MCP should show UPDATING=True, then UPDATED=True when complete
   ```

**Phase 3: Verify Updated Configuration**

6. Verify updated CRI-O configuration:
   ```bash
   oc debug node/$WORKER_NODE -- chroot /host \
     cat /etc/crio/crio.conf.d/01-ctrcfg-update-artifactstore-test
   
   # Should now contain: artifact_stores = ["/mnt/artifacts-v2", "/mnt/artifacts-v3"]
   # Old path "/mnt/artifacts-v1" should NOT be present
   ```

7. Verify CRI-O service is healthy after update:
   ```bash
   oc debug node/$WORKER_NODE -- chroot /host systemctl status crio
   ```

8. Verify all nodes are Ready:
   ```bash
   oc get nodes -l node-role.kubernetes.io/worker=
   ```

**Phase 4: Cleanup**

9. Delete the ContainerRuntimeConfig:
   ```bash
   oc delete containerruntimeconfig update-artifactstore-test
   ```

10. Wait for cleanup rollout:
    ```bash
    oc get mcp worker -w
    ```

**Expected Result:**
- Phase 1: Initial configuration creates successfully, MCP rolls out, config shows `/mnt/artifacts-v1`
- Phase 2: Update accepted, MCP rolls out again
- Phase 3: CRI-O config updated to show both new paths `/mnt/artifacts-v2` and `/mnt/artifacts-v3`
- Phase 3: Old path `/mnt/artifacts-v1` is removed from configuration
- Phase 3: CRI-O service remains healthy
- Phase 3: All nodes remain Ready
- Phase 4: Cleanup completes successfully

**Cleanup:**
```bash
# Already performed in Phase 4
oc get containerruntimeconfig update-artifactstore-test
# Should return: Error from server (NotFound)
```

---

### Test Case: TC10 - Multiple Artifact Stores

**Description:**
Verify that multiple additional artifact stores (up to the maximum of 10) can be configured simultaneously and CRI-O generates correct configuration with all paths in order.

**Prerequisites:**
- Common prerequisites above
- Worker nodes with available storage paths
- No existing ContainerRuntimeConfig named `multiple-artifactstores-test`
- Worker MCP in healthy state

**Manual Steps:**

**Phase 1: Create Configuration with Multiple Stores**

1. Create ContainerRuntimeConfig with 5 artifact stores:
   ```bash
   cat <<EOF | oc apply -f -
   apiVersion: machineconfiguration.openshift.io/v1
   kind: ContainerRuntimeConfig
   metadata:
     name: multiple-artifactstores-test
   spec:
     machineConfigPoolSelector:
       matchLabels:
         pools.operator.machineconfiguration.openshift.io/worker: ""
     containerRuntimeConfig:
       additionalArtifactStores:
       - path: "/mnt/artifactstore-0"
       - path: "/mnt/artifactstore-1"
       - path: "/mnt/artifactstore-2"
       - path: "/mnt/artifactstore-3"
       - path: "/mnt/artifactstore-4"
   EOF
   ```

2. Wait for MCP rollout:
   ```bash
   oc get mcp worker -w
   # Wait for UPDATED=True
   ```

**Phase 2: Verify Configuration**

3. Verify CRI-O configuration contains all paths in correct order:
   ```bash
   WORKER_NODE=$(oc get nodes -l node-role.kubernetes.io/worker= -o jsonpath='{.items[0].metadata.name}')
   oc debug node/$WORKER_NODE -- chroot /host \
     cat /etc/crio/crio.conf.d/01-ctrcfg-multiple-artifactstores-test
   
   # Should contain all 5 paths in order:
   # artifact_stores = ["/mnt/artifactstore-0", "/mnt/artifactstore-1", "/mnt/artifactstore-2", "/mnt/artifactstore-3", "/mnt/artifactstore-4"]
   ```

4. Verify CRI-O service is running:
   ```bash
   oc debug node/$WORKER_NODE -- chroot /host systemctl status crio
   # Should show: active (running)
   ```

5. Verify all worker nodes updated successfully:
   ```bash
   oc get nodes -l node-role.kubernetes.io/worker=
   # All nodes should show STATUS=Ready
   
   oc get mcp worker
   # MACHINECOUNT should equal READYMACHINECOUNT and UPDATEDMACHINECOUNT
   ```

**Phase 3: Cleanup**

6. Delete the ContainerRuntimeConfig:
   ```bash
   oc delete containerruntimeconfig multiple-artifactstores-test
   ```

7. Wait for cleanup rollout:
   ```bash
   oc get mcp worker -w
   ```

8. Verify configuration file removed:
   ```bash
   oc debug node/$WORKER_NODE -- chroot /host \
     ls /etc/crio/crio.conf.d/ | grep multiple-artifactstores-test
   # Should return no results
   ```

**Expected Result:**
- Phase 1: ContainerRuntimeConfig with 5 stores created successfully
- Phase 1: MCP rollout completes without errors
- Phase 2: CRI-O configuration file contains all 5 paths in the exact order specified
- Phase 2: CRI-O service remains healthy and running
- Phase 2: All worker nodes are Ready and updated
- Phase 3: Cleanup removes configuration successfully

**Cleanup:**
```bash
# Already performed in Phase 3
oc get containerruntimeconfig multiple-artifactstores-test
# Should return: Error from server (NotFound)
```

---

## Important Notes

### Disruptive Nature

⚠️ **All tests involving ContainerRuntimeConfig changes are DISRUPTIVE:**
- Trigger MachineConfigPool rollouts
- Cause node reboots as configurations are applied
- Take 10-20 minutes per rollout depending on cluster size
- Should be scheduled during maintenance windows

### Timing Expectations

- **MCP Rollout Time:** 10-20 minutes for worker pool updates
- **Node Reboot Time:** 3-5 minutes per node
- **Total Test Time:** Each E2E test (TC8-TC10) takes approximately 30-45 minutes including creation and cleanup

### Platform Considerations

- **All Platforms:** Feature is platform-agnostic
- **Storage Requirements:** Ensure configured paths exist on nodes or are created via MachineConfig
- **NFS/Shared Storage:** Can be used for artifact stores but requires prior setup

### Feature Gate Requirement

🔑 **Critical:** All tests require `AdditionalStorageConfig` feature gate to be enabled:

```bash
oc get featuregate cluster -o jsonpath='{.spec.featureSet}'
# Must return: TechPreviewNoUpgrade

oc get featuregate cluster -o yaml | grep AdditionalStorageConfig
# Feature must be listed in enabled features
```

---

## Test Execution Summary

| Category | Test Cases | Estimated Time |
|----------|-----------|----------------|
| API Validation | TC1-TC7 | 10-15 minutes |
| E2E Functional | TC8-TC10 | 90-120 minutes |
| **Total** | **10 test cases** | **~2 hours** |

---

**Document Version:** 1.0  
**Last Updated:** 2026-06-03  
**Automation File:** `additional_artifact_stores.go`  
**Related Epic:** OCPSTRAT-2623
