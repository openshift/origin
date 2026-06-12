# Additional Image Stores - Manual Test Cases

## Feature Overview

**Feature Name:** Additional Image Stores for CRI-O  
**Epic:** OCPSTRAT-2623 (part of OCPNODE-4051)  
**Target Release:** OpenShift 4.22 (Tech Preview)  
**Feature Gate:** `AdditionalStorageConfig` (TechPreviewNoUpgrade)

### What is it?

The Additional Image Stores feature enables configuration of multiple read-only image storage locations in CRI-O. This allows OpenShift to search for container images in pre-configured local caches before pulling from remote registries, significantly reducing startup times for large images.

### Why is it needed?

**Primary Use Case:** AI/ML workloads with large model images (5GB+)

**Problem Being Solved:**
- Large container images (especially AI/ML models) take significant time to pull from registries
- ~70% of container startup time is spent on image pulling for large images
- Cannot leverage pre-populated image caches in air-gapped deployments
- Cannot use shared NFS-backed image caches across nodes
- No way to separate image storage from root filesystem

**Key Benefits:**
- **Performance:** Near-instant pod startup when images are cached locally
- **Air-gapped Support:** Pre-populate images for disconnected environments
- **Shared Caches:** NFS-backed shared image storage across nodes
- **Disk Management:** Separate large images from root filesystem

### How does it work?

1. **API Configuration:** Define additional read-only image storage paths via `ContainerRuntimeConfig`
2. **MCO Translation:** Machine Config Operator generates CRI-O configuration
3. **Image Resolution:** CRI-O searches additional stores before pulling from remote registry
4. **Fallback:** If image not found in local stores, pulls from configured registry

**Image Resolution Order:**
```
1. Additional image store 1 (if configured)
2. Additional image store 2 (if configured)
...
N. Additional image store N
N+1. Default CRI-O storage: /var/lib/containers/storage
N+2. Remote registry pull (fallback)
```

### API Example

```yaml
apiVersion: machineconfiguration.openshift.io/v1
kind: ContainerRuntimeConfig
metadata:
  name: set-additional-image-stores
spec:
  machineConfigPoolSelector:
    matchLabels:
      pools.operator.machineconfiguration.openshift.io/worker: ""
  containerRuntimeConfig:
    additionalImageStores:
      - path: /mnt/nfs-images        # Shared NFS cache
      - path: /mnt/ssd-images        # Local SSD cache
```

### Generated CRI-O Configuration

```toml
[crio.image]
additional_image_stores = ["/mnt/nfs-images", "/mnt/ssd-images"]
```

### Image Prepopulation

Images must be in **OCI directory format** (not podman storage database format):

```bash
# Use skopeo to prepopulate images:
skopeo copy docker://quay.io/myorg/large-ml-model:v1.0 \
  dir:/mnt/nfs-images/large-ml-model
```

### Constraints

- **Maximum:** Up to 10 additional image stores per ContainerRuntimeConfig
- **Path Format:** Must be absolute paths (starting with `/`)
- **Path Length:** Maximum 256 characters
- **Invalid Characters:** No spaces, `@`, `!`, `#`, `$`, `%`
- **Uniqueness:** No duplicate paths within the same configuration
- **Read-Only:** All additional stores are read-only
- **Image Format:** Must use OCI directory format (use `skopeo copy`)

### Related Documentation

- Enhancement Proposal: https://github.com/saschagrunert/enhancements/blob/525af9bbc3e9eefc82557f69850429ef8ffce30a/enhancements/machine-config/additional-storage-config-crio.md
- Epic: OCPNODE-4051 - CRI-O Additional Storage Support
- CRI-O Documentation: additional_image_stores configuration

---

## Common Prerequisites (All Test Cases)

- OpenShift 4.22+ cluster
- `AdditionalStorageConfig` feature gate enabled (TechPreviewNoUpgrade)
- Cluster admin access
- Permission to create `ContainerRuntimeConfig` resources
- Worker nodes with available storage paths
- For E2E tests: Access to test images (quay.io/openshifttest/additional-storage-tests:*)

### Verify Feature Gate

```bash
oc get featuregate cluster -o yaml | grep -A5 "AdditionalStorageConfig"
```

---

## API Validation Test Cases

### Test Case: TC1 - Invalid Path Formats

**Description:**
Verify that the API rejects `additionalImageStores` paths that don't meet format requirements (must be absolute paths).

**Prerequisites:**
- Common prerequisites above
- No existing ContainerRuntimeConfig named `invalid-path-test-*`

**Manual Steps:**

1. Attempt to create ContainerRuntimeConfig with relative path:
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
       additionalImageStores:
       - path: "relative/path"
   EOF
   ```

2. Attempt to create ContainerRuntimeConfig with empty path:
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
       additionalImageStores:
       - path: ""
   EOF
   ```

**Expected Result:**
- Both operations must fail with validation errors
- Error messages indicate path format requirements
- No ContainerRuntimeConfig resources created
- MachineConfigPool unchanged

**Cleanup:**
```bash
# No cleanup needed
```

---

### Test Case: TC2 - Exceed Maximum Count

**Description:**
Verify that the API rejects configurations with more than 10 image stores.

**Prerequisites:**
- Common prerequisites above

**Manual Steps:**

1. Create ContainerRuntimeConfig with 11 image stores:
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
       additionalImageStores:
       - path: "/mnt/imagestore-0"
       - path: "/mnt/imagestore-1"
       - path: "/mnt/imagestore-2"
       - path: "/mnt/imagestore-3"
       - path: "/mnt/imagestore-4"
       - path: "/mnt/imagestore-5"
       - path: "/mnt/imagestore-6"
       - path: "/mnt/imagestore-7"
       - path: "/mnt/imagestore-8"
       - path: "/mnt/imagestore-9"
       - path: "/mnt/imagestore-10"
   EOF
   ```

**Expected Result:**
- Must fail with validation error
- Error contains "must have at most 10" or similar
- No resource created

**Cleanup:**
```bash
# No cleanup needed
```

---

### Test Case: TC3 - Duplicate Paths

**Description:**
Verify duplicate paths are rejected.

**Prerequisites:**
- Common prerequisites above

**Manual Steps:**

1. Attempt to create with duplicate paths:
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
       additionalImageStores:
       - path: "/mnt/shared-images"
       - path: "/mnt/shared-images"
   EOF
   ```

**Expected Result:**
- Must fail with duplicate path error
- No resource created

**Cleanup:**
```bash
# No cleanup needed
```

---

### Test Case: TC4 - Path with Spaces

**Description:**
Verify paths containing spaces are rejected.

**Prerequisites:**
- Common prerequisites above

**Manual Steps:**

1. Attempt to create with path containing spaces:
   ```bash
   cat <<EOF | oc apply -f -
   apiVersion: machineconfiguration.openshift.io/v1
   kind: ContainerRuntimeConfig
   metadata:
     name: path-spaces-test
   spec:
     machineConfigPoolSelector:
       matchLabels:
         pools.operator.machineconfiguration.openshift.io/worker: ""
     containerRuntimeConfig:
       additionalImageStores:
       - path: "/var/lib/image store"
   EOF
   ```

**Expected Result:**
- Must fail indicating spaces not allowed
- No resource created

**Cleanup:**
```bash
# No cleanup needed
```

---

### Test Case: TC5 - Invalid Characters

**Description:**
Verify paths with invalid characters (`@`, `!`, `#`, `$`, `%`) are rejected.

**Prerequisites:**
- Common prerequisites above

**Manual Steps:**

1-5. Test each invalid character:
   ```bash
   for CHAR in "@" "!" "#" "$" "%"; do
     cat <<EOF | oc apply -f -
   apiVersion: machineconfiguration.openshift.io/v1
   kind: ContainerRuntimeConfig
   metadata:
     name: invalid-char-test-$(echo $CHAR | tr -d '!@#$%')
   spec:
     machineConfigPoolSelector:
       matchLabels:
         pools.operator.machineconfiguration.openshift.io/worker: ""
     containerRuntimeConfig:
       additionalImageStores:
       - path: "/var/lib/image${CHAR}store"
   EOF
   done
   ```

**Expected Result:**
- All create operations must fail
- Errors indicate invalid characters
- No resources created

**Cleanup:**
```bash
# No cleanup needed
```

---

### Test Case: TC6 - Path Exceeding 256 Characters

**Description:**
Verify paths longer than 256 characters are rejected.

**Prerequisites:**
- Common prerequisites above

**Manual Steps:**

1. Create 257-character path:
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
       additionalImageStores:
       - path: "/very/long/path/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
   EOF
   ```

**Expected Result:**
- Must fail with path length error
- No resource created

**Cleanup:**
```bash
# No cleanup needed
```

---

### Test Case: TC7 - Consecutive Slashes

**Description:**
Verify paths with consecutive slashes are rejected.

**Prerequisites:**
- Common prerequisites above

**Manual Steps:**

1. Attempt with consecutive slashes:
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
       additionalImageStores:
       - path: "/var//lib//images"
   EOF
   ```

**Expected Result:**
- Must fail with consecutive slash error
- No resource created

**Cleanup:**
```bash
# No cleanup needed
```

---

## E2E Functional Test Cases

### Test Case: TC8 - E2E with Prepopulated Images and Fallback

**Description:**
Complete lifecycle test: prepopulate image on node, configure additional image store, verify pod uses cached image quickly, test fallback to remote pull on uncached node, and cleanup.

**Prerequisites:**
- Common prerequisites above
- NOT running on Microsoft Azure (known storage configuration issue)
- Worker nodes with 8GB+ free disk space
- Access to `quay.io/openshifttest/additional-storage-tests:test-6gb-standard-v1.0`

**Manual Steps:**

**Phase 1: Prepopulate Image**

1. Select a worker node:
   ```bash
   WORKER_NODE=$(oc get nodes -l node-role.kubernetes.io/worker= -o jsonpath='{.items[0].metadata.name}')
   echo "Using node: $WORKER_NODE"
   ```

2. Create directory and prepopulate image using skopeo:
   ```bash
   oc debug node/$WORKER_NODE -- chroot /host mkdir -p /var/lib/additional-images/prepopulated-image
   
   oc debug node/$WORKER_NODE -- chroot /host \
     skopeo copy docker://quay.io/openshifttest/additional-storage-tests:test-6gb-standard-v1.0 \
       dir:/var/lib/additional-images/prepopulated-image
   ```

**Phase 2: Create ContainerRuntimeConfig**

3. Configure additional image store:
   ```bash
   cat <<EOF | oc apply -f -
   apiVersion: machineconfiguration.openshift.io/v1
   kind: ContainerRuntimeConfig
   metadata:
     name: additional-imagestore-test
   spec:
     machineConfigPoolSelector:
       matchLabels:
         pools.operator.machineconfiguration.openshift.io/worker: ""
     containerRuntimeConfig:
       additionalImageStores:
       - path: "/var/lib/additional-images"
   EOF
   ```

**Phase 3: Wait for MCP Rollout**

4. Monitor MCP:
   ```bash
   oc get mcp worker -w
   # Wait for UPDATED=True (10-20 minutes)
   ```

**Phase 4: Verify CRI-O Config**

5. Check configuration:
   ```bash
   oc debug node/$WORKER_NODE -- chroot /host \
     cat /etc/crio/crio.conf.d/01-ctrcfg-additional-imagestore-test
   
   # Should contain: additional_image_stores = ["/var/lib/additional-images"]
   ```

**Phase 5: Test Cached Image Pull**

6. Create pod using cached image:
   ```bash
   cat <<EOF | oc apply -f -
   apiVersion: v1
   kind: Pod
   metadata:
     name: test-pod-cached
     namespace: default
   spec:
     nodeName: $WORKER_NODE
     containers:
     - name: test
       image: quay.io/openshifttest/additional-storage-tests:test-6gb-standard-v1.0
       command: ["sh", "-c", "echo Cached image; sleep 3600"]
     restartPolicy: Never
   EOF
   ```

7. Verify quick startup (< 30 seconds):
   ```bash
   time oc wait --for=condition=Ready pod/test-pod-cached --timeout=60s
   # Should complete in < 30 seconds
   ```

**Phase 6: Test Fallback to Remote Pull**

8. Get second worker node:
   ```bash
   WORKER_NODE_2=$(oc get nodes -l node-role.kubernetes.io/worker= -o jsonpath='{.items[1].metadata.name}')
   ```

9. Create pod on uncached node:
   ```bash
   cat <<EOF | oc apply -f -
   apiVersion: v1
   kind: Pod
   metadata:
     name: test-pod-remote
     namespace: default
   spec:
     nodeName: $WORKER_NODE_2
     containers:
     - name: test
       image: quay.io/openshifttest/additional-storage-tests:test-6gb-standard-v1.0
       command: ["sh", "-c", "echo Remote pull; sleep 3600"]
     restartPolicy: Never
   EOF
   ```

10. Verify fallback works (slower but succeeds):
    ```bash
    oc wait --for=condition=Ready pod/test-pod-remote --timeout=600s
    # May take several minutes (remote pull)
    ```

**Phase 7: Cleanup**

11. Delete pods:
    ```bash
    oc delete pod test-pod-cached test-pod-remote
    ```

12. Delete ContainerRuntimeConfig:
    ```bash
    oc delete containerruntimeconfig additional-imagestore-test
    oc get mcp worker -w  # Wait for rollout
    ```

13. Clean prepopulated image:
    ```bash
    oc debug node/$WORKER_NODE -- chroot /host \
      rm -rf /var/lib/additional-images
    ```

**Expected Result:**
- Prepopulation succeeds
- MCP rollout completes successfully
- CRI-O config contains additional_image_stores
- Pod on cached node starts in < 30 seconds
- Pod on uncached node starts successfully (remote pull fallback)
- Cleanup completes successfully

**Cleanup:**
```bash
# Already performed in Phase 7
```

---

### Test Case: TC9 - Update Configuration

**Description:**
Verify updating `additionalImageStores` paths triggers MCP rollout and updates CRI-O correctly.

**Prerequisites:**
- Common prerequisites above

**Manual Steps:**

1. Create initial config:
   ```bash
   cat <<EOF | oc apply -f -
   apiVersion: machineconfiguration.openshift.io/v1
   kind: ContainerRuntimeConfig
   metadata:
     name: update-imagestore-test
   spec:
     machineConfigPoolSelector:
       matchLabels:
         pools.operator.machineconfiguration.openshift.io/worker: ""
     containerRuntimeConfig:
       additionalImageStores:
       - path: "/mnt/images-v1"
   EOF
   ```

2. Wait for rollout:
   ```bash
   oc get mcp worker -w
   ```

3. Verify initial config:
   ```bash
   WORKER_NODE=$(oc get nodes -l node-role.kubernetes.io/worker= -o jsonpath='{.items[0].metadata.name}')
   oc debug node/$WORKER_NODE -- chroot /host \
     cat /etc/crio/crio.conf.d/01-ctrcfg-update-imagestore-test
   # Should show: ["/mnt/images-v1"]
   ```

4. Update config:
   ```bash
   cat <<EOF | oc apply -f -
   apiVersion: machineconfiguration.openshift.io/v1
   kind: ContainerRuntimeConfig
   metadata:
     name: update-imagestore-test
   spec:
     machineConfigPoolSelector:
       matchLabels:
         pools.operator.machineconfiguration.openshift.io/worker: ""
     containerRuntimeConfig:
       additionalImageStores:
       - path: "/mnt/images-v2"
       - path: "/mnt/images-v3"
   EOF
   ```

5. Wait for update rollout:
   ```bash
   oc get mcp worker -w
   ```

6. Verify updated config:
   ```bash
   oc debug node/$WORKER_NODE -- chroot /host \
     cat /etc/crio/crio.conf.d/01-ctrcfg-update-imagestore-test
   # Should show: ["/mnt/images-v2", "/mnt/images-v3"]
   # Old path "/mnt/images-v1" should NOT be present
   ```

7. Cleanup:
   ```bash
   oc delete containerruntimeconfig update-imagestore-test
   ```

**Expected Result:**
- Initial config creates and rolls out successfully
- Update accepted and rolls out successfully
- CRI-O config updates to show new paths only
- Old path removed from configuration

**Cleanup:**
```bash
# Already performed in step 7
```

---

### Test Case: TC10 - Multiple Image Stores

**Description:**
Verify multiple additional image stores (up to 10) work correctly.

**Prerequisites:**
- Common prerequisites above

**Manual Steps:**

1. Create config with 5 stores:
   ```bash
   cat <<EOF | oc apply -f -
   apiVersion: machineconfiguration.openshift.io/v1
   kind: ContainerRuntimeConfig
   metadata:
     name: multiple-imagestores-test
   spec:
     machineConfigPoolSelector:
       matchLabels:
         pools.operator.machineconfiguration.openshift.io/worker: ""
     containerRuntimeConfig:
       additionalImageStores:
       - path: "/mnt/imagestore-0"
       - path: "/mnt/imagestore-1"
       - path: "/mnt/imagestore-2"
       - path: "/mnt/imagestore-3"
       - path: "/mnt/imagestore-4"
   EOF
   ```

2. Wait for rollout:
   ```bash
   oc get mcp worker -w
   ```

3. Verify all paths in config:
   ```bash
   WORKER_NODE=$(oc get nodes -l node-role.kubernetes.io/worker= -o jsonpath='{.items[0].metadata.name}')
   oc debug node/$WORKER_NODE -- chroot /host \
     cat /etc/crio/crio.conf.d/01-ctrcfg-multiple-imagestores-test
   
   # Should contain all 5 paths in order
   ```

4. Verify CRI-O healthy:
   ```bash
   oc debug node/$WORKER_NODE -- chroot /host systemctl status crio
   ```

5. Cleanup:
   ```bash
   oc delete containerruntimeconfig multiple-imagestores-test
   ```

**Expected Result:**
- Config with 5 stores creates successfully
- MCP rollout completes
- All 5 paths present in correct order
- CRI-O remains healthy
- All nodes updated successfully

**Cleanup:**
```bash
# Already performed in step 5
```

---

## Important Notes

### Disruptive Nature
⚠️ All ContainerRuntimeConfig changes trigger MCP rollouts with node reboots (10-20 minutes)

### Timing
- MCP Rollout: 10-20 minutes
- Each E2E test: 30-45 minutes

### Image Format Requirement
**Critical:** Images must be in OCI directory format. Use `skopeo copy` to prepopulate:
```bash
skopeo copy docker://IMAGE_URL dir:/path/to/store/image-name
```

Do NOT use `podman --root` as it creates an incompatible storage database format.

### Platform Notes
- Feature works on all platforms
- Azure has known issues with certain storage configurations

### Test Execution Summary

| Category | Tests | Time |
|----------|-------|------|
| API Validation | TC1-TC7 | 15 min |
| E2E Functional | TC8-TC10 | 90-120 min |
| **Total** | **10** | **~2 hours** |

---

**Document Version:** 1.0  
**Last Updated:** 2026-06-03  
**Automation File:** `additional_image_stores.go`  
**Related Epic:** OCPNODE-4051
