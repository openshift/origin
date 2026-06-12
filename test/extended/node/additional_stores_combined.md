# Combined Additional Stores - Manual Test Cases

## Feature Overview

**Feature Name:** Combined Additional Storage Configuration  
**Epic:** OCPNODE-4051  
**Target Release:** OpenShift 4.22 (Tech Preview)  
**Feature Gate:** `AdditionalStorageConfig` (TechPreviewNoUpgrade)

### What is it?

Combined Additional Stores tests verify that all three storage types (Image Stores, Layer Stores, and Artifact Stores) can be configured simultaneously in a single `ContainerRuntimeConfig` and work together correctly.

### Why test combinations?

**Purpose:** Ensure the three storage features don't conflict when used together

**Real-World Scenarios:**
- AI/ML workloads using all three: cached images, lazy-pulled models, and pre-loaded artifacts
- Complex deployments requiring multiple storage optimizations simultaneously
- Validation that MCO correctly generates CRI-O configuration for all three types

**Test Coverage:**
- API validation with multiple store types in one config
- Same path allowed across different store types (they serve different purposes)
- Maximum limits enforced per store type (not combined totals)
- All three types functional together in real workloads

### The Three Storage Types

| Type | Purpose | Max Count | CRI-O Config Section |
|------|---------|-----------|---------------------|
| **Image Stores** | Pre-cached full images | 10 | `[crio.image] additional_image_stores` |
| **Layer Stores** | Lazy pulling (stargz) | 5 | `[crio.image] additional_layer_stores` |
| **Artifact Stores** | OCI artifacts/models | 10 | `[crio.runtime.artifacts] artifact_stores` |

### API Example - All Three Types

```yaml
apiVersion: machineconfiguration.openshift.io/v1
kind: ContainerRuntimeConfig
metadata:
  name: combined-storage-config
spec:
  machineConfigPoolSelector:
    matchLabels:
      pools.operator.machineconfiguration.openshift.io/worker: ""
  containerRuntimeConfig:
    additionalImageStores:
      - path: /mnt/nfs-images
      - path: /mnt/ssd-images
    additionalLayerStores:
      - path: /var/lib/stargz-store/store:ref
    additionalArtifactStores:
      - path: /mnt/ssd-artifacts
      - path: /mnt/nfs-artifacts
```

### Generated CRI-O Configuration

```toml
[crio.image]
additional_image_stores = ["/mnt/nfs-images", "/mnt/ssd-images"]
additional_layer_stores = ["/var/lib/stargz-store/store:ref"]

[crio.runtime.artifacts]
artifact_stores = ["/mnt/ssd-artifacts", "/mnt/nfs-artifacts"]
```

### Key Rules for Combined Configuration

1. **Independent Limits:** Each store type has its own maximum (10/5/10)
2. **Path Reuse Allowed:** Same path can be used across different types (e.g., `/mnt/shared` for both images and artifacts)
3. **No Duplication Within Type:** Same path cannot appear twice in the same store type
4. **Validation Per Type:** Each store type validated independently
5. **MCO Generation:** MCO generates separate CRI-O config sections for each type

---

## Common Prerequisites (All Test Cases)

- OpenShift 4.22+ cluster
- `AdditionalStorageConfig` feature gate enabled
- Cluster admin access
- For E2E tests: stargz-store installed for layer store tests
- Test images: quay.io/openshifttest/additional-storage-tests:*

---

## API Validation Test Cases

### Test Case: TC1 - Invalid Path in Any Store Type

**Description:**
Verify that if any of the three store types (image/layer/artifact) contains an invalid path, the entire ContainerRuntimeConfig is rejected. Validation applies to all types.

**Prerequisites:**
- Common prerequisites above

**Manual Steps:**

1. Attempt to create combined config with invalid path in image stores:
   ```bash
   cat <<EOF | oc apply -f -
   apiVersion: machineconfiguration.openshift.io/v1
   kind: ContainerRuntimeConfig
   metadata:
     name: combined-invalid-image-path
   spec:
     machineConfigPoolSelector:
       matchLabels:
         pools.operator.machineconfiguration.openshift.io/worker: ""
     containerRuntimeConfig:
       additionalImageStores:
       - path: "relative/invalid"        # INVALID
       additionalLayerStores:
       - path: "/var/lib/stargz:ref"     # valid
       additionalArtifactStores:
       - path: "/mnt/artifacts"           # valid
   EOF
   ```

2. Attempt with invalid path in layer stores:
   ```bash
   cat <<EOF | oc apply -f -
   apiVersion: machineconfiguration.openshift.io/v1
   kind: ContainerRuntimeConfig
   metadata:
     name: combined-invalid-layer-path
   spec:
     machineConfigPoolSelector:
       matchLabels:
         pools.operator.machineconfiguration.openshift.io/worker: ""
     containerRuntimeConfig:
       additionalImageStores:
       - path: "/mnt/images"              # valid
       additionalLayerStores:
       - path: ""                         # INVALID (empty)
       additionalArtifactStores:
       - path: "/mnt/artifacts"           # valid
   EOF
   ```

3. Attempt with invalid path in artifact stores:
   ```bash
   cat <<EOF | oc apply -f -
   apiVersion: machineconfiguration.openshift.io/v1
   kind: ContainerRuntimeConfig
   metadata:
     name: combined-invalid-artifact-path
   spec:
     machineConfigPoolSelector:
       matchLabels:
         pools.operator.machineconfiguration.openshift.io/worker: ""
     containerRuntimeConfig:
       additionalImageStores:
       - path: "/mnt/images"              # valid
       additionalLayerStores:
       - path: "/var/lib/stargz:ref"     # valid
       additionalArtifactStores:
       - path: "/var//lib//artifacts"    # INVALID (consecutive slashes)
   EOF
   ```

**Expected Result:**
- All three create operations must fail with validation errors
- Errors should clearly indicate which store type has the invalid path
- No ContainerRuntimeConfig resources created
- Demonstrates that validation applies to all store types

**Cleanup:**
```bash
# No cleanup needed
```

---

### Test Case: TC2 - Layer Store Exceeds Max with Valid Image/Artifact Stores

**Description:**
Verify that if layer stores exceed the maximum (5) even when image and artifact stores are valid, the entire configuration is rejected. Each store type has independent limits.

**Prerequisites:**
- Common prerequisites above

**Manual Steps:**

1. Attempt with 6 layer stores (exceeds max 5) but valid image/artifact stores:
   ```bash
   cat <<EOF | oc apply -f -
   apiVersion: machineconfiguration.openshift.io/v1
   kind: ContainerRuntimeConfig
   metadata:
     name: combined-exceed-layer-limit
   spec:
     machineConfigPoolSelector:
       matchLabels:
         pools.operator.machineconfiguration.openshift.io/worker: ""
     containerRuntimeConfig:
       additionalImageStores:
       - path: "/mnt/images"              # valid (1 of max 10)
       additionalLayerStores:
       - path: "/mnt/layer-0:ref"
       - path: "/mnt/layer-1:ref"
       - path: "/mnt/layer-2:ref"
       - path: "/mnt/layer-3:ref"
       - path: "/mnt/layer-4:ref"
       - path: "/mnt/layer-5:ref"        # 6 stores - EXCEEDS MAX 5
       additionalArtifactStores:
       - path: "/mnt/artifacts"           # valid (1 of max 10)
   EOF
   ```

**Expected Result:**
- Must fail with validation error
- Error should indicate layer stores exceed maximum of 5
- No resource created
- Demonstrates independent limit enforcement per store type

**Cleanup:**
```bash
# No cleanup needed
```

---

### Test Case: TC3 - Duplicate Paths Within Same Store Type

**Description:**
Verify that duplicate paths within the same store type are rejected, even when combined with other valid store types.

**Prerequisites:**
- Common prerequisites above

**Manual Steps:**

1. Attempt with duplicate paths in image stores:
   ```bash
   cat <<EOF | oc apply -f -
   apiVersion: machineconfiguration.openshift.io/v1
   kind: ContainerRuntimeConfig
   metadata:
     name: combined-duplicate-image-paths
   spec:
     machineConfigPoolSelector:
       matchLabels:
         pools.operator.machineconfiguration.openshift.io/worker: ""
     containerRuntimeConfig:
       additionalImageStores:
       - path: "/mnt/images"
       - path: "/mnt/images"              # DUPLICATE
       additionalLayerStores:
       - path: "/var/lib/stargz:ref"
       additionalArtifactStores:
       - path: "/mnt/artifacts"
   EOF
   ```

2. Attempt with duplicate paths in artifact stores:
   ```bash
   cat <<EOF | oc apply -f -
   apiVersion: machineconfiguration.openshift.io/v1
   kind: ContainerRuntimeConfig
   metadata:
     name: combined-duplicate-artifact-paths
   spec:
     machineConfigPoolSelector:
       matchLabels:
         pools.operator.machineconfiguration.openshift.io/worker: ""
     containerRuntimeConfig:
       additionalImageStores:
       - path: "/mnt/images"
       additionalLayerStores:
       - path: "/var/lib/stargz:ref"
       additionalArtifactStores:
       - path: "/mnt/artifacts"
       - path: "/mnt/artifacts"           # DUPLICATE
   EOF
   ```

**Expected Result:**
- Both operations must fail with duplicate path errors
- Errors should indicate which store type has duplicates
- No resources created
- Demonstrates duplicate detection per store type

**Cleanup:**
```bash
# No cleanup needed
```

---

## E2E Functional Test Cases

### Test Case: TC4 - Configure All Three Storage Types

**Description:**
Verify that all three storage types (image, layer, artifact) can be configured together in a single ContainerRuntimeConfig, MCO generates correct CRI-O configuration, and MCP rolls out successfully.

**Prerequisites:**
- Common prerequisites above
- stargz-store installed for layer stores

**Manual Steps:**

**Phase 1: Create Combined Configuration**

1. Create ContainerRuntimeConfig with all three types:
   ```bash
   cat <<EOF | oc apply -f -
   apiVersion: machineconfiguration.openshift.io/v1
   kind: ContainerRuntimeConfig
   metadata:
     name: combined-all-three-types
   spec:
     machineConfigPoolSelector:
       matchLabels:
         pools.operator.machineconfiguration.openshift.io/worker: ""
     containerRuntimeConfig:
       additionalImageStores:
       - path: "/mnt/nfs-images"
       - path: "/mnt/ssd-images"
       additionalLayerStores:
       - path: "/var/lib/stargz-store/store:ref"
       additionalArtifactStores:
       - path: "/mnt/ssd-artifacts"
       - path: "/mnt/nfs-artifacts"
   EOF
   ```

**Phase 2: Monitor MCP Rollout**

2. Wait for rollout:
   ```bash
   oc get mcp worker -w
   # Wait for UPDATED=True (10-20 minutes)
   ```

**Phase 3: Verify CRI-O Configuration**

3. Check generated configuration:
   ```bash
   WORKER_NODE=$(oc get nodes -l node-role.kubernetes.io/worker= -o jsonpath='{.items[0].metadata.name}')
   oc debug node/$WORKER_NODE -- chroot /host \
     cat /etc/crio/crio.conf.d/01-ctrcfg-combined-all-three-types
   
   # Should contain:
   # [crio.image]
   # additional_image_stores = ["/mnt/nfs-images", "/mnt/ssd-images"]
   # additional_layer_stores = ["/var/lib/stargz-store/store:ref"]
   # 
   # [crio.runtime.artifacts]
   # artifact_stores = ["/mnt/ssd-artifacts", "/mnt/nfs-artifacts"]
   ```

4. Verify CRI-O is healthy:
   ```bash
   oc debug node/$WORKER_NODE -- chroot /host systemctl status crio
   # Should be: active (running)
   ```

5. Verify all nodes updated:
   ```bash
   oc get mcp worker
   # MACHINECOUNT = READYMACHINECOUNT = UPDATEDMACHINECOUNT
   ```

**Phase 4: Cleanup**

6. Delete configuration:
   ```bash
   oc delete containerruntimeconfig combined-all-three-types
   oc get mcp worker -w
   ```

**Expected Result:**
- ContainerRuntimeConfig with all three types creates successfully
- MCP rollout completes without errors
- CRI-O config contains all three sections with correct paths
- CRI-O service remains healthy
- All worker nodes updated successfully
- Cleanup completes successfully

**Cleanup:**
```bash
# Already performed in Phase 4
```

---

### Test Case: TC5 - Maximum Stores for Each Type

**Description:**
Verify configuring the maximum number of stores for each type simultaneously: 10 image stores, 5 layer stores, 10 artifact stores (total 25 stores).

**Prerequisites:**
- Common prerequisites above
- stargz-store installed

**Manual Steps:**

1. Create config with maximum stores for each type:
   ```bash
   cat <<EOF | oc apply -f -
   apiVersion: machineconfiguration.openshift.io/v1
   kind: ContainerRuntimeConfig
   metadata:
     name: combined-maximum-all-types
   spec:
     machineConfigPoolSelector:
       matchLabels:
         pools.operator.machineconfiguration.openshift.io/worker: ""
     containerRuntimeConfig:
       additionalImageStores:
       - path: "/mnt/image-0"
       - path: "/mnt/image-1"
       - path: "/mnt/image-2"
       - path: "/mnt/image-3"
       - path: "/mnt/image-4"
       - path: "/mnt/image-5"
       - path: "/mnt/image-6"
       - path: "/mnt/image-7"
       - path: "/mnt/image-8"
       - path: "/mnt/image-9"
       additionalLayerStores:
       - path: "/mnt/layer-0:ref"
       - path: "/mnt/layer-1:ref"
       - path: "/mnt/layer-2:ref"
       - path: "/mnt/layer-3:ref"
       - path: "/mnt/layer-4:ref"
       additionalArtifactStores:
       - path: "/mnt/artifact-0"
       - path: "/mnt/artifact-1"
       - path: "/mnt/artifact-2"
       - path: "/mnt/artifact-3"
       - path: "/mnt/artifact-4"
       - path: "/mnt/artifact-5"
       - path: "/mnt/artifact-6"
       - path: "/mnt/artifact-7"
       - path: "/mnt/artifact-8"
       - path: "/mnt/artifact-9"
   EOF
   ```

2. Wait for rollout:
   ```bash
   oc get mcp worker -w
   ```

3. Verify all stores in config:
   ```bash
   WORKER_NODE=$(oc get nodes -l node-role.kubernetes.io/worker= -o jsonpath='{.items[0].metadata.name}')
   oc debug node/$WORKER_NODE -- chroot /host \
     cat /etc/crio/crio.conf.d/01-ctrcfg-combined-maximum-all-types
   
   # Count entries:
   # Image stores: should have 10 entries
   # Layer stores: should have 5 entries
   # Artifact stores: should have 10 entries
   ```

4. Verify CRI-O healthy:
   ```bash
   oc debug node/$WORKER_NODE -- chroot /host systemctl status crio
   ```

5. Cleanup:
   ```bash
   oc delete containerruntimeconfig combined-maximum-all-types
   ```

**Expected Result:**
- Config with maximum stores (25 total) creates successfully
- MCP rollout completes
- CRI-O config contains all 25 paths correctly distributed
- CRI-O remains healthy despite large config
- All nodes updated successfully

**Cleanup:**
```bash
# Already performed in step 5
```

---

### Test Case: TC6 - Same Path Across Different Store Types

**Description:**
Verify that the same path can be used across different store types (e.g., `/mnt/shared` for both images and artifacts) since they serve different purposes and use different CRI-O config sections.

**Prerequisites:**
- Common prerequisites above

**Manual Steps:**

1. Create config with same path in multiple types:
   ```bash
   cat <<EOF | oc apply -f -
   apiVersion: machineconfiguration.openshift.io/v1
   kind: ContainerRuntimeConfig
   metadata:
     name: combined-shared-path-test
   spec:
     machineConfigPoolSelector:
       matchLabels:
         pools.operator.machineconfiguration.openshift.io/worker: ""
     containerRuntimeConfig:
       additionalImageStores:
       - path: "/mnt/shared"              # Same path
       additionalLayerStores:
       - path: "/mnt/shared:ref"          # Same base path (with :ref)
       additionalArtifactStores:
       - path: "/mnt/shared"              # Same path
   EOF
   ```

2. Verify resource creates successfully:
   ```bash
   oc get containerruntimeconfig combined-shared-path-test
   # Should show: Created
   ```

3. Wait for rollout:
   ```bash
   oc get mcp worker -w
   ```

4. Verify config generated correctly:
   ```bash
   WORKER_NODE=$(oc get nodes -l node-role.kubernetes.io/worker= -o jsonpath='{.items[0].metadata.name}')
   oc debug node/$WORKER_NODE -- chroot /host \
     cat /etc/crio/crio.conf.d/01-ctrcfg-combined-shared-path-test
   
   # Should show /mnt/shared in both image stores and artifact stores sections
   ```

5. Cleanup:
   ```bash
   oc delete containerruntimeconfig combined-shared-path-test
   ```

**Expected Result:**
- Configuration with shared paths across types is accepted
- MCP rollout completes successfully
- CRI-O config shows same path in multiple sections (different purposes)
- No conflicts or errors
- Demonstrates path reuse is allowed across types

**Cleanup:**
```bash
# Already performed in step 5
```

---

### Test Case: TC7 - Add Stores to Each Type

**Description:**
Verify that an existing combined configuration can be updated to add more stores to each type, and MCO correctly updates the CRI-O configuration.

**Prerequisites:**
- Common prerequisites above

**Manual Steps:**

1. Create initial config with minimal stores:
   ```bash
   cat <<EOF | oc apply -f -
   apiVersion: machineconfiguration.openshift.io/v1
   kind: ContainerRuntimeConfig
   metadata:
     name: combined-update-test
   spec:
     machineConfigPoolSelector:
       matchLabels:
         pools.operator.machineconfiguration.openshift.io/worker: ""
     containerRuntimeConfig:
       additionalImageStores:
       - path: "/mnt/image-v1"
       additionalLayerStores:
       - path: "/mnt/layer-v1:ref"
       additionalArtifactStores:
       - path: "/mnt/artifact-v1"
   EOF
   ```

2. Wait for initial rollout:
   ```bash
   oc get mcp worker -w
   ```

3. Verify initial config:
   ```bash
   WORKER_NODE=$(oc get nodes -l node-role.kubernetes.io/worker= -o jsonpath='{.items[0].metadata.name}')
   oc debug node/$WORKER_NODE -- chroot /host \
     cat /etc/crio/crio.conf.d/01-ctrcfg-combined-update-test
   # Should show v1 paths
   ```

4. Update to add more stores to each type:
   ```bash
   cat <<EOF | oc apply -f -
   apiVersion: machineconfiguration.openshift.io/v1
   kind: ContainerRuntimeConfig
   metadata:
     name: combined-update-test
   spec:
     machineConfigPoolSelector:
       matchLabels:
         pools.operator.machineconfiguration.openshift.io/worker: ""
     containerRuntimeConfig:
       additionalImageStores:
       - path: "/mnt/image-v1"
       - path: "/mnt/image-v2"            # Added
       - path: "/mnt/image-v3"            # Added
       additionalLayerStores:
       - path: "/mnt/layer-v1:ref"
       - path: "/mnt/layer-v2:ref"        # Added
       additionalArtifactStores:
       - path: "/mnt/artifact-v1"
       - path: "/mnt/artifact-v2"         # Added
       - path: "/mnt/artifact-v3"         # Added
   EOF
   ```

5. Wait for update rollout:
   ```bash
   oc get mcp worker -w
   ```

6. Verify updated config:
   ```bash
   oc debug node/$WORKER_NODE -- chroot /host \
     cat /etc/crio/crio.conf.d/01-ctrcfg-combined-update-test
   
   # Should show:
   # Image stores: v1, v2, v3 (3 total)
   # Layer stores: v1, v2 (2 total)
   # Artifact stores: v1, v2, v3 (3 total)
   ```

7. Cleanup:
   ```bash
   oc delete containerruntimeconfig combined-update-test
   ```

**Expected Result:**
- Initial config creates and rolls out successfully
- Update accepted and rolls out successfully
- CRI-O config updated to include all new paths
- Original paths preserved plus new ones added
- All store types updated correctly

**Cleanup:**
```bash
# Already performed in step 7
```

---

### Test Case: TC8 - Remove One Store Type While Keeping Others

**Description:**
Verify that one store type can be removed from a combined configuration while keeping the other types active and functional.

**Prerequisites:**
- Common prerequisites above

**Manual Steps:**

1. Create combined config with all three types:
   ```bash
   cat <<EOF | oc apply -f -
   apiVersion: machineconfiguration.openshift.io/v1
   kind: ContainerRuntimeConfig
   metadata:
     name: combined-remove-type-test
   spec:
     machineConfigPoolSelector:
       matchLabels:
         pools.operator.machineconfiguration.openshift.io/worker: ""
     containerRuntimeConfig:
       additionalImageStores:
       - path: "/mnt/images"
       additionalLayerStores:
       - path: "/mnt/layers:ref"
       additionalArtifactStores:
       - path: "/mnt/artifacts"
   EOF
   ```

2. Wait for rollout:
   ```bash
   oc get mcp worker -w
   ```

3. Verify all three types in config:
   ```bash
   WORKER_NODE=$(oc get nodes -l node-role.kubernetes.io/worker= -o jsonpath='{.items[0].metadata.name}')
   oc debug node/$WORKER_NODE -- chroot /host \
     cat /etc/crio/crio.conf.d/01-ctrcfg-combined-remove-type-test
   # Should show all three types
   ```

4. Update to remove layer stores:
   ```bash
   cat <<EOF | oc apply -f -
   apiVersion: machineconfiguration.openshift.io/v1
   kind: ContainerRuntimeConfig
   metadata:
     name: combined-remove-type-test
   spec:
     machineConfigPoolSelector:
       matchLabels:
         pools.operator.machineconfiguration.openshift.io/worker: ""
     containerRuntimeConfig:
       additionalImageStores:
       - path: "/mnt/images"
       # additionalLayerStores removed
       additionalArtifactStores:
       - path: "/mnt/artifacts"
   EOF
   ```

5. Wait for update rollout:
   ```bash
   oc get mcp worker -w
   ```

6. Verify layer stores removed, others remain:
   ```bash
   oc debug node/$WORKER_NODE -- chroot /host \
     cat /etc/crio/crio.conf.d/01-ctrcfg-combined-remove-type-test
   
   # Should show:
   # Image stores: still present
   # Layer stores: removed (section should not exist)
   # Artifact stores: still present
   ```

7. Verify CRI-O healthy:
   ```bash
   oc debug node/$WORKER_NODE -- chroot /host systemctl status crio
   ```

8. Cleanup:
   ```bash
   oc delete containerruntimeconfig combined-remove-type-test
   ```

**Expected Result:**
- Initial config with all three types creates successfully
- Update removing layer stores accepted
- MCP rolls out successfully
- Image and artifact stores remain in config
- Layer stores section removed from config
- CRI-O remains healthy
- Demonstrates selective removal of store types

**Cleanup:**
```bash
# Already performed in step 8
```

---

### Test Case: TC9 - Functional Verification of All Three Types

**Description:**
End-to-end functional test verifying all three storage types work correctly together in a real workload scenario: prepopulated image from image store, lazy-pulled eStargz image from layer store, and artifacts accessible.

**Prerequisites:**
- Common prerequisites above
- stargz-store installed
- NOT on Microsoft Azure
- Worker nodes with 15GB+ free disk space
- Test images available

**Manual Steps:**

**Phase 1: Setup Storage**

1. Select worker node and prepopulate image:
   ```bash
   WORKER_NODE=$(oc get nodes -l node-role.kubernetes.io/worker= -o jsonpath='{.items[0].metadata.name}')
   
   # Prepopulate standard image
   oc debug node/$WORKER_NODE -- chroot /host \
     mkdir -p /var/lib/additional-images/prepopulated
   oc debug node/$WORKER_NODE -- chroot /host \
     skopeo copy docker://quay.io/openshifttest/additional-storage-tests:test-6gb-standard-v1.0 \
       dir:/var/lib/additional-images/prepopulated
   ```

**Phase 2: Configure Combined Storage**

2. Create combined config:
   ```bash
   cat <<EOF | oc apply -f -
   apiVersion: machineconfiguration.openshift.io/v1
   kind: ContainerRuntimeConfig
   metadata:
     name: combined-functional-test
   spec:
     machineConfigPoolSelector:
       matchLabels:
         pools.operator.machineconfiguration.openshift.io/worker: ""
     containerRuntimeConfig:
       additionalImageStores:
       - path: "/var/lib/additional-images"
       additionalLayerStores:
       - path: "/var/lib/stargz-store/store:ref"
       additionalArtifactStores:
       - path: "/mnt/artifacts"
   EOF
   ```

3. Wait for rollout:
   ```bash
   oc get mcp worker -w
   ```

**Phase 3: Test Image Store (Cached Pull)**

4. Create pod using prepopulated image:
   ```bash
   cat <<EOF | oc apply -f -
   apiVersion: v1
   kind: Pod
   metadata:
     name: test-image-store
   spec:
     nodeName: $WORKER_NODE
     containers:
     - name: test
       image: quay.io/openshifttest/additional-storage-tests:test-6gb-standard-v1.0
       command: ["sh", "-c", "echo Image store works; sleep 600"]
   EOF
   
   # Should start quickly (cached)
   time oc wait --for=condition=Ready pod/test-image-store --timeout=60s
   ```

**Phase 4: Test Layer Store (Lazy Pull)**

5. Create pod with eStargz image:
   ```bash
   cat <<EOF | oc apply -f -
   apiVersion: v1
   kind: Pod
   metadata:
     name: test-layer-store
   spec:
     nodeName: $WORKER_NODE
     containers:
     - name: test
       image: quay.io/openshifttest/additional-storage-tests:test-6gb-estargz-v1.0
       command: ["sh", "-c", "echo Layer store works; sleep 600"]
   EOF
   
   # Should start quickly (lazy pull)
   time oc wait --for=condition=Ready pod/test-layer-store --timeout=120s
   ```

6. Verify snapshots created:
   ```bash
   oc debug node/$WORKER_NODE -- chroot /host \
     find /var/lib/stargz-store/store -type d -name "sha256:*" | wc -l
   # Should show > 0 (snapshots exist)
   ```

**Phase 5: Verify All Pods Running**

7. Check all test pods:
   ```bash
   oc get pods test-image-store test-layer-store
   # Both should be Running
   ```

**Phase 6: Cleanup**

8. Delete pods:
   ```bash
   oc delete pod test-image-store test-layer-store
   ```

9. Delete config:
   ```bash
   oc delete containerruntimeconfig combined-functional-test
   oc get mcp worker -w
   ```

10. Clean prepopulated data:
    ```bash
    oc debug node/$WORKER_NODE -- chroot /host \
      rm -rf /var/lib/additional-images
    ```

**Expected Result:**
- Image store: Pod with prepopulated image starts rapidly (< 30s)
- Layer store: Pod with eStargz image starts quickly (< 60s) via lazy pull
- Snapshots created in stargz-store (evidence of lazy pulling)
- Both pods reach Running state
- All three storage types functional simultaneously
- No conflicts or errors
- Cleanup completes successfully

**Cleanup:**
```bash
# Already performed in Phase 6
```

---

## Important Notes

### Disruptive Nature
⚠️ All ContainerRuntimeConfig changes trigger MCP rollouts with node reboots (10-20 minutes)

### Complexity
Combined configurations increase complexity. Test individual types first before combining.

### Timing
- Each E2E test: 60-90 minutes (includes rollouts)
- Full suite: 6-8 hours

### Storage Requirements
Functional tests (TC9) require significant disk space for prepopulated images and snapshots.

### Independent Validation
Each store type validated independently - errors in one type don't mask issues in others.

### Test Execution Summary

| Category | Tests | Time |
|----------|-------|------|
| API Validation | TC1-TC3 | 10 min |
| E2E Functional | TC4-TC9 | 300-360 min |
| **Total** | **9** | **~6 hours** |

---

**Document Version:** 1.0  
**Last Updated:** 2026-06-04  
**Automation File:** `additional_stores_combined.go`  
**Related Epic:** OCPNODE-4051
