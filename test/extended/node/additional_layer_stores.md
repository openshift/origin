# Additional Layer Stores - Manual Test Cases

## Feature Overview

**Feature Name:** Additional Layer Stores for CRI-O (Lazy Image Pulling)  
**Epic:** OCPNODE-4051  
**Target Release:** OpenShift 4.22 (Tech Preview)  
**Feature Gate:** `AdditionalStorageConfig` (TechPreviewNoUpgrade)

### What is it?

Additional Layer Stores enable **lazy pulling** of container images through integration with stargz-snapshotter. Instead of downloading entire large images before starting containers, only required layers are fetched on-demand using FUSE mounts, dramatically reducing container startup times.

### Why is it needed?

**Primary Use Case:** AI/ML workloads with extremely large model images (10GB-50GB+)

**Problem Being Solved:**
- Containers cannot start until full image is downloaded
- Large AI/ML images (50GB+) can take 30+ minutes to pull
- 70% of container startup time is image pulling
- Prevents rapid autoscaling for AI/ML workloads
- Wastes resources waiting for full downloads when only parts are needed

**Key Benefits:**
- **Instant Startup:** Containers start in seconds instead of minutes
- **On-Demand Fetching:** Only download image layers as needed
- **Bandwidth Efficiency:** Don't download unused layers
- **Autoscaling:** Enable rapid scaling for AI/ML workloads
- **Storage Efficiency:** Don't store full images if only portions are used

### How does it work?

1. **stargz-store Installation:** Install stargz-snapshotter service on worker nodes (DaemonSet)
2. **API Configuration:** Define layer store paths via `ContainerRuntimeConfig`
3. **eStargz Format:** Images must be in eStargz format (estargz-enabled OCI images)
4. **FUSE Mount:** stargz-store creates FUSE mount for lazy layer access
5. **CRI-O Integration:** CRI-O uses layer store for eStargz images
6. **On-Demand Fetch:** Image layers pulled on first access

### eStargz Image Format

**What is eStargz?**
- Extended stargz (seekable tar.gz) format
- Enables random access to image layers without full download
- Compatible with standard OCI registries
- Can be created from any OCI image

**Creating eStargz Images:**
```bash
# Convert standard image to eStargz format
ctr-remote image optimize \
  --oci \
  --estargz \
  quay.io/myorg/large-ml-model:standard \
  quay.io/myorg/large-ml-model:estargz
```

### API Example

```yaml
apiVersion: machineconfiguration.openshift.io/v1
kind: ContainerRuntimeConfig
metadata:
  name: set-additional-layer-stores
spec:
  machineConfigPoolSelector:
    matchLabels:
      pools.operator.machineconfiguration.openshift.io/worker: ""
  containerRuntimeConfig:
    additionalLayerStores:
      - path: /var/lib/stargz-store/store:ref
```

### Generated CRI-O Configuration

```toml
[crio.image]
additional_layer_stores = ["/var/lib/stargz-store/store:ref"]
```

### Constraints

- **Maximum:** Up to 5 additional layer stores per ContainerRuntimeConfig (lower than image/artifact stores due to complexity)
- **Path Format:** Must be absolute paths, can include `:ref` suffix for referencestore
- **Path Length:** Maximum 256 characters
- **Invalid Characters:** No spaces, `@`, `!`, `#`, `$`, `%`
- **Uniqueness:** No duplicate paths
- **stargz-store Required:** Must install stargz-snapshotter service first
- **eStargz Format:** Only works with eStargz-formatted images
- **Fallback:** Standard OCI images fall back to normal pull

### stargz-store Installation

Must install stargz-snapshotter on nodes before configuring layer stores:

```yaml
# Installed via DaemonSet in test automation
# Binary: /usr/local/bin/stargz-store
# Service: /etc/systemd/system/stargz-store.service
# Config: /etc/stargz-store/config.toml
# Data: /var/lib/stargz-store/store
```

### Related Documentation

- Enhancement Proposal: https://github.com/saschagrunert/enhancements/blob/525af9bbc3e9eefc82557f69850429ef8ffce30a/enhancements/machine-config/additional-storage-config-crio.md
- stargz-snapshotter: https://github.com/containerd/stargz-snapshotter
- Epic: OCPNODE-4051 - CRI-O Additional Storage Support

---

## Common Prerequisites (All Test Cases)

- OpenShift 4.22+ cluster
- `AdditionalStorageConfig` feature gate enabled (TechPreviewNoUpgrade)
- Cluster admin access
- Worker nodes with available storage
- For E2E tests: stargz-store service installed on workers
- Test images: quay.io/openshifttest/additional-storage-tests:test-*-estargz*

---

## API Validation Test Cases

### Test Case: TC1 - Empty Path

**Description:**
Verify that the API rejects empty paths for `additionalLayerStores`.

**Prerequisites:**
- Common prerequisites above

**Manual Steps:**

1. Attempt to create with empty path:
   ```bash
   cat <<EOF | oc apply -f -
   apiVersion: machineconfiguration.openshift.io/v1
   kind: ContainerRuntimeConfig
   metadata:
     name: layerstore-empty-path-test
   spec:
     machineConfigPoolSelector:
       matchLabels:
         pools.operator.machineconfiguration.openshift.io/worker: ""
     containerRuntimeConfig:
       additionalLayerStores:
       - path: ""
   EOF
   ```

**Expected Result:**
- Must fail with validation error
- Error indicates empty path not allowed
- No resource created

**Cleanup:**
```bash
# No cleanup needed
```

---

### Test Case: TC2 - Relative Path

**Description:**
Verify that relative paths (not starting with `/`) are rejected.

**Prerequisites:**
- Common prerequisites above

**Manual Steps:**

1. Attempt with relative path:
   ```bash
   cat <<EOF | oc apply -f -
   apiVersion: machineconfiguration.openshift.io/v1
   kind: ContainerRuntimeConfig
   metadata:
     name: layerstore-relative-path-test
   spec:
     machineConfigPoolSelector:
       matchLabels:
         pools.operator.machineconfiguration.openshift.io/worker: ""
     containerRuntimeConfig:
       additionalLayerStores:
       - path: "relative/path/stargz"
   EOF
   ```

**Expected Result:**
- Must fail with validation error
- Error indicates path must be absolute
- No resource created

**Cleanup:**
```bash
# No cleanup needed
```

---

### Test Case: TC3 - Path with Spaces

**Description:**
Verify paths containing spaces are rejected.

**Prerequisites:**
- Common prerequisites above

**Manual Steps:**

1. Attempt with path containing spaces:
   ```bash
   cat <<EOF | oc apply -f -
   apiVersion: machineconfiguration.openshift.io/v1
   kind: ContainerRuntimeConfig
   metadata:
     name: layerstore-spaces-test
   spec:
     machineConfigPoolSelector:
       matchLabels:
         pools.operator.machineconfiguration.openshift.io/worker: ""
     containerRuntimeConfig:
       additionalLayerStores:
       - path: "/var/lib/stargz store"
   EOF
   ```

**Expected Result:**
- Must fail with validation error
- Error indicates spaces not allowed
- No resource created

**Cleanup:**
```bash
# No cleanup needed
```

---

### Test Case: TC4 - Invalid Characters

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
     name: layerstore-invalid-$(echo $CHAR | tr -d '!@#$%')-test
   spec:
     machineConfigPoolSelector:
       matchLabels:
         pools.operator.machineconfiguration.openshift.io/worker: ""
     containerRuntimeConfig:
       additionalLayerStores:
       - path: "/var/lib/stargz${CHAR}store"
   EOF
   done
   ```

**Expected Result:**
- All must fail with validation errors
- Errors indicate invalid characters
- No resources created

**Cleanup:**
```bash
# No cleanup needed
```

---

### Test Case: TC5 - Path Exceeding 256 Characters

**Description:**
Verify paths longer than 256 characters are rejected.

**Prerequisites:**
- Common prerequisites above

**Manual Steps:**

1. Attempt with 257-character path:
   ```bash
   cat <<EOF | oc apply -f -
   apiVersion: machineconfiguration.openshift.io/v1
   kind: ContainerRuntimeConfig
   metadata:
     name: layerstore-long-path-test
   spec:
     machineConfigPoolSelector:
       matchLabels:
         pools.operator.machineconfiguration.openshift.io/worker: ""
     containerRuntimeConfig:
       additionalLayerStores:
       - path: "/very/long/stargz/path/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
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

### Test Case: TC6 - Exceed Maximum Count

**Description:**
Verify that more than 5 layer stores are rejected (lower limit than image/artifact stores).

**Prerequisites:**
- Common prerequisites above

**Manual Steps:**

1. Attempt with 6 layer stores:
   ```bash
   cat <<EOF | oc apply -f -
   apiVersion: machineconfiguration.openshift.io/v1
   kind: ContainerRuntimeConfig
   metadata:
     name: layerstore-exceed-limit-test
   spec:
     machineConfigPoolSelector:
       matchLabels:
         pools.operator.machineconfiguration.openshift.io/worker: ""
     containerRuntimeConfig:
       additionalLayerStores:
       - path: "/mnt/layerstore-0"
       - path: "/mnt/layerstore-1"
       - path: "/mnt/layerstore-2"
       - path: "/mnt/layerstore-3"
       - path: "/mnt/layerstore-4"
       - path: "/mnt/layerstore-5"
   EOF
   ```

**Expected Result:**
- Must fail with validation error
- Error indicates maximum 5 layer stores
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
     name: layerstore-consecutive-slashes-test
   spec:
     machineConfigPoolSelector:
       matchLabels:
         pools.operator.machineconfiguration.openshift.io/worker: ""
     containerRuntimeConfig:
       additionalLayerStores:
       - path: "/var//lib//stargz"
   EOF
   ```

**Expected Result:**
- Must fail with validation error
- Error indicates consecutive slashes not allowed
- No resource created

**Cleanup:**
```bash
# No cleanup needed
```

---

### Test Case: TC8 - Duplicate Paths

**Description:**
Verify duplicate paths within layer stores are rejected.

**Prerequisites:**
- Common prerequisites above

**Manual Steps:**

1. Attempt with duplicate paths:
   ```bash
   cat <<EOF | oc apply -f -
   apiVersion: machineconfiguration.openshift.io/v1
   kind: ContainerRuntimeConfig
   metadata:
     name: layerstore-duplicate-path-test
   spec:
     machineConfigPoolSelector:
       matchLabels:
         pools.operator.machineconfiguration.openshift.io/worker: ""
     containerRuntimeConfig:
       additionalLayerStores:
       - path: "/var/lib/stargz-store/store:ref"
       - path: "/var/lib/stargz-store/store:ref"
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

## E2E Functional Test Cases

### Test Case: TC9 - stargz-store Lifecycle and Lazy Pulling

**Description:**
Complete E2E test: install stargz-store on workers, configure layer store, verify lazy pulling with eStargz image, monitor snapshot creation, and cleanup.

**Prerequisites:**
- Common prerequisites above
- NOT on Microsoft Azure
- Worker nodes with 10GB+ free disk space
- Access to `quay.io/openshifttest/additional-storage-tests:test-6gb-estargz-v1.0`

**Manual Steps:**

**Phase 1: Install stargz-store**

1. Create namespace for stargz installation:
   ```bash
   oc create namespace stargz-store
   ```

2. Grant privileged SCC (required for FUSE mounts):
   ```bash
   oc create sa stargz-store -n stargz-store
   oc adm policy add-scc-to-user privileged system:serviceaccount:stargz-store:stargz-store
   ```

3. Create ConfigMap with stargz config:
   ```bash
   cat <<EOF | oc apply -f -
   apiVersion: v1
   kind: ConfigMap
   metadata:
     name: stargz-store-config
     namespace: stargz-store
   data:
     config.toml: |
       [blob]
       check_always = true
       
       [blob.fetcher]
       max_concurrency = 10
       
       [snapshotter]
       root = "/var/lib/stargz-store/store"
     
     stargz-store.service: |
       [Unit]
       Description=stargz-store FUSE service
       After=network.target
       
       [Service]
       Type=notify
       ExecStart=/usr/local/bin/stargz-store \
         --log-level=debug \
         --config=/etc/stargz-store/config.toml \
         /var/lib/stargz-store/store
       Restart=on-failure
       
       [Install]
       WantedBy=multi-user.target
   EOF
   ```

4. Create DaemonSet to install stargz-store on all workers:
   ```bash
   cat <<EOF | oc apply -f -
   apiVersion: apps/v1
   kind: DaemonSet
   metadata:
     name: stargz-store-installer
     namespace: stargz-store
   spec:
     selector:
       matchLabels:
         app: stargz-store-installer
     template:
       metadata:
         labels:
           app: stargz-store-installer
       spec:
         serviceAccountName: stargz-store
         hostPID: true
         hostNetwork: true
         nodeSelector:
           node-role.kubernetes.io/worker: ""
         volumes:
         - name: host
           hostPath:
             path: /
         - name: config
           configMap:
             name: stargz-store-config
         containers:
         - name: installer
           image: quay.io/centos/centos:stream9
           securityContext:
             privileged: true
           volumeMounts:
           - name: host
             mountPath: /host
           - name: config
             mountPath: /config
           command: ["/bin/bash", "-c"]
           args:
           - |
             set -e
             
             # Detect architecture
             NODE_ARCH=\$(uname -m)
             case "\$NODE_ARCH" in
               x86_64)  DOWNLOAD_ARCH="amd64" ;;
               aarch64) DOWNLOAD_ARCH="arm64" ;;
               s390x)   DOWNLOAD_ARCH="s390x" ;;
               ppc64le) DOWNLOAD_ARCH="ppc64le" ;;
               *) echo "ERROR: Unsupported arch"; exit 1 ;;
             esac
             
             # Download stargz-snapshotter
             VERSION="v0.18.2"
             URL="https://github.com/containerd/stargz-snapshotter/releases/download/\$VERSION/stargz-snapshotter-\$VERSION-linux-\$DOWNLOAD_ARCH.tar.gz"
             
             curl -L -f -o /tmp/stargz.tar.gz "\$URL"
             tar -xzf /tmp/stargz.tar.gz -C /tmp/
             cp /tmp/stargz-store /host/usr/local/bin/
             chmod +x /host/usr/local/bin/stargz-store
             
             # Setup directories
             mkdir -p /host/etc/stargz-store
             mkdir -p /host/var/lib/stargz-store/store
             
             # Copy config
             cp /config/config.toml /host/etc/stargz-store/
             cp /config/stargz-store.service /host/etc/systemd/system/
             
             # Enable and start service
             nsenter -t 1 -m -u -i -n -p -- systemctl daemon-reload
             nsenter -t 1 -m -u -i -n -p -- systemctl enable stargz-store
             nsenter -t 1 -m -u -i -n -p -- systemctl start stargz-store
             
             echo "stargz-store installed and running"
             sleep infinity
   EOF
   ```

5. Wait for DaemonSet pods to be ready:
   ```bash
   oc get ds -n stargz-store stargz-store-installer -w
   # Wait for DESIRED=READY
   ```

6. Verify stargz-store service running on a worker:
   ```bash
   WORKER_NODE=$(oc get nodes -l node-role.kubernetes.io/worker= -o jsonpath='{.items[0].metadata.name}')
   oc debug node/$WORKER_NODE -- chroot /host systemctl status stargz-store
   # Should show: active (running)
   ```

**Phase 2: Configure Layer Store**

7. Create ContainerRuntimeConfig:
   ```bash
   cat <<EOF | oc apply -f -
   apiVersion: machineconfiguration.openshift.io/v1
   kind: ContainerRuntimeConfig
   metadata:
     name: additional-layerstore-test
   spec:
     machineConfigPoolSelector:
       matchLabels:
         pools.operator.machineconfiguration.openshift.io/worker: ""
     containerRuntimeConfig:
       additionalLayerStores:
       - path: /var/lib/stargz-store/store:ref
   EOF
   ```

8. Monitor MCP rollout:
   ```bash
   oc get mcp worker -w
   # Wait for UPDATED=True (10-20 minutes)
   ```

**Phase 3: Verify CRI-O Configuration**

9. Check CRI-O config:
   ```bash
   oc debug node/$WORKER_NODE -- chroot /host \
     cat /etc/crio/crio.conf.d/01-ctrcfg-additional-layerstore-test
   
   # Should contain: additional_layer_stores = ["/var/lib/stargz-store/store:ref"]
   ```

**Phase 4: Test Lazy Pulling with eStargz Image**

10. Get initial snapshot count:
    ```bash
    oc debug node/$WORKER_NODE -- chroot /host \
      find /var/lib/stargz-store/store -type d -name "sha256:*" | wc -l
    # Record the count
    ```

11. Create pod with eStargz image:
    ```bash
    cat <<EOF | oc apply -f -
    apiVersion: v1
    kind: Pod
    metadata:
      name: test-estargz-pod
      namespace: default
    spec:
      nodeName: $WORKER_NODE
      containers:
      - name: test
        image: quay.io/openshifttest/additional-storage-tests:test-6gb-estargz-v1.0
        command: ["sh", "-c", "echo Lazy pulled; sleep 3600"]
      restartPolicy: Never
    EOF
    ```

12. Verify rapid startup (lazy pull, not full download):
    ```bash
    time oc wait --for=condition=Ready pod/test-estargz-pod --timeout=120s
    # Should complete in < 60 seconds (much faster than 6GB download)
    ```

13. Verify snapshots created (lazy pull evidence):
    ```bash
    oc debug node/$WORKER_NODE -- chroot /host \
      find /var/lib/stargz-store/store -type d -name "sha256:*" | wc -l
    # Should be > initial count (new snapshots created)
    ```

14. Check FUSE mount exists:
    ```bash
    oc debug node/$WORKER_NODE -- chroot /host mount | grep stargz
    # Should show FUSE mounts
    ```

**Phase 5: Cleanup**

15. Delete test pod:
    ```bash
    oc delete pod test-estargz-pod
    ```

16. Delete ContainerRuntimeConfig:
    ```bash
    oc delete containerruntimeconfig additional-layerstore-test
    oc get mcp worker -w
    ```

17. Cleanup stargz-store:
    ```bash
    oc delete ds -n stargz-store stargz-store-installer
    oc delete namespace stargz-store
    
    # Clean stargz-store from nodes
    oc debug node/$WORKER_NODE -- chroot /host systemctl stop stargz-store
    oc debug node/$WORKER_NODE -- chroot /host systemctl disable stargz-store
    oc debug node/$WORKER_NODE -- chroot /host rm -f /usr/local/bin/stargz-store
    oc debug node/$WORKER_NODE -- chroot /host rm -rf /etc/stargz-store
    oc debug node/$WORKER_NODE -- chroot /host rm -rf /var/lib/stargz-store
    ```

**Expected Result:**
- stargz-store DaemonSet installs successfully on all workers
- stargz-store service running and healthy
- MCP rollout completes successfully
- CRI-O config contains additional_layer_stores
- Pod with eStargz image starts rapidly (< 60 seconds for 6GB image)
- Snapshot count increases (evidence of lazy pulling)
- FUSE mounts visible
- Cleanup completes successfully

**Cleanup:**
```bash
# Already performed in Phase 5
```

---

### Test Case: TC10 - Update Configuration

**Description:**
Verify updating layer store paths triggers MCP rollout correctly.

**Prerequisites:**
- Common prerequisites above
- stargz-store installed on workers (from TC9)

**Manual Steps:**

1. Create initial config:
   ```bash
   cat <<EOF | oc apply -f -
   apiVersion: machineconfiguration.openshift.io/v1
   kind: ContainerRuntimeConfig
   metadata:
     name: update-layerstore-test
   spec:
     machineConfigPoolSelector:
       matchLabels:
         pools.operator.machineconfiguration.openshift.io/worker: ""
     containerRuntimeConfig:
       additionalLayerStores:
       - path: "/var/lib/stargz-store-v1/store:ref"
   EOF
   ```

2. Wait for rollout and verify:
   ```bash
   oc get mcp worker -w
   
   WORKER_NODE=$(oc get nodes -l node-role.kubernetes.io/worker= -o jsonpath='{.items[0].metadata.name}')
   oc debug node/$WORKER_NODE -- chroot /host \
     cat /etc/crio/crio.conf.d/01-ctrcfg-update-layerstore-test
   # Should show v1 path
   ```

3. Update config:
   ```bash
   cat <<EOF | oc apply -f -
   apiVersion: machineconfiguration.openshift.io/v1
   kind: ContainerRuntimeConfig
   metadata:
     name: update-layerstore-test
   spec:
     machineConfigPoolSelector:
       matchLabels:
         pools.operator.machineconfiguration.openshift.io/worker: ""
     containerRuntimeConfig:
       additionalLayerStores:
       - path: "/var/lib/stargz-store-v2/store:ref"
   EOF
   ```

4. Wait for update and verify:
   ```bash
   oc get mcp worker -w
   
   oc debug node/$WORKER_NODE -- chroot /host \
     cat /etc/crio/crio.conf.d/01-ctrcfg-update-layerstore-test
   # Should show v2 path, NOT v1
   ```

5. Cleanup:
   ```bash
   oc delete containerruntimeconfig update-layerstore-test
   ```

**Expected Result:**
- Initial config creates and rolls out
- Update accepted and rolls out
- CRI-O config updates to new path only
- Old path removed

**Cleanup:**
```bash
# Already performed in step 5
```

---

### Test Case: TC11 - Multiple Layer Stores

**Description:**
Verify configuring up to 5 layer stores (maximum) works correctly.

**Prerequisites:**
- Common prerequisites above
- stargz-store installed

**Manual Steps:**

1. Create config with 5 stores:
   ```bash
   cat <<EOF | oc apply -f -
   apiVersion: machineconfiguration.openshift.io/v1
   kind: ContainerRuntimeConfig
   metadata:
     name: multiple-layerstores-test
   spec:
     machineConfigPoolSelector:
       matchLabels:
         pools.operator.machineconfiguration.openshift.io/worker: ""
     containerRuntimeConfig:
       additionalLayerStores:
       - path: "/mnt/layerstore-0:ref"
       - path: "/mnt/layerstore-1:ref"
       - path: "/mnt/layerstore-2:ref"
       - path: "/mnt/layerstore-3:ref"
       - path: "/mnt/layerstore-4:ref"
   EOF
   ```

2. Wait and verify:
   ```bash
   oc get mcp worker -w
   
   WORKER_NODE=$(oc get nodes -l node-role.kubernetes.io/worker= -o jsonpath='{.items[0].metadata.name}')
   oc debug node/$WORKER_NODE -- chroot /host \
     cat /etc/crio/crio.conf.d/01-ctrcfg-multiple-layerstores-test
   # Should contain all 5 paths in order
   ```

3. Cleanup:
   ```bash
   oc delete containerruntimeconfig multiple-layerstores-test
   ```

**Expected Result:**
- Config with 5 stores creates successfully
- MCP rollout completes
- All 5 paths in correct order
- CRI-O remains healthy

**Cleanup:**
```bash
# Already performed in step 3
```

---

### Test Case: TC12 - Fallback to Standard Pull (Non-eStargz Image)

**Description:**
Verify that standard (non-eStargz) OCI images fall back to normal pull when layer store is configured. CRI-O should gracefully handle non-eStargz images without errors.

**Prerequisites:**
- Common prerequisites above
- stargz-store installed and layer store configured (from TC9 or TC11)
- Access to standard (non-eStargz) test image

**Manual Steps:**

1. Ensure layer store is configured:
   ```bash
   oc get containerruntimeconfig additional-layerstore-test
   # If not exists, create it as in TC9
   ```

2. Create pod with standard (non-eStargz) image:
   ```bash
   WORKER_NODE=$(oc get nodes -l node-role.kubernetes.io/worker= -o jsonpath='{.items[0].metadata.name}')
   
   cat <<EOF | oc apply -f -
   apiVersion: v1
   kind: Pod
   metadata:
     name: test-standard-image-pod
     namespace: default
   spec:
     nodeName: $WORKER_NODE
     containers:
     - name: test
       image: quay.io/openshifttest/additional-storage-tests:test-6gb-standard-v1.0
       command: ["sh", "-c", "echo Standard image; sleep 3600"]
     restartPolicy: Never
   EOF
   ```

3. Verify pod starts successfully (fallback to standard pull):
   ```bash
   oc wait --for=condition=Ready pod/test-standard-image-pod --timeout=600s
   # May take longer (full download) but should succeed
   ```

4. Verify pod is running:
   ```bash
   oc get pod test-standard-image-pod
   # STATUS should be Running
   ```

5. Check pod logs to confirm it started:
   ```bash
   oc logs test-standard-image-pod
   # Should show: Standard image
   ```

6. Cleanup:
   ```bash
   oc delete pod test-standard-image-pod
   ```

**Expected Result:**
- Pod with standard OCI image creates successfully
- CRI-O falls back to normal pull (no lazy pulling)
- Pod reaches Running state
- No errors related to layer store
- Demonstrates graceful fallback behavior

**Cleanup:**
```bash
# Already performed in step 6
```

---

### Test Case: TC13 - Fallback When stargz-store Service Stopped

**Description:**
Verify that when stargz-store service is stopped, CRI-O falls back to standard pull for eStargz images instead of failing. This tests resilience and graceful degradation.

**Prerequisites:**
- Common prerequisites above
- stargz-store installed and layer store configured
- stargz-store service currently running

**Manual Steps:**

1. Verify stargz-store is running:
   ```bash
   WORKER_NODE=$(oc get nodes -l node-role.kubernetes.io/worker= -o jsonpath='{.items[0].metadata.name}')
   oc debug node/$WORKER_NODE -- chroot /host systemctl status stargz-store
   # Should be active (running)
   ```

2. Stop stargz-store service:
   ```bash
   oc debug node/$WORKER_NODE -- chroot /host systemctl stop stargz-store
   ```

3. Verify service is stopped:
   ```bash
   oc debug node/$WORKER_NODE -- chroot /host systemctl is-active stargz-store
   # Should return: inactive
   ```

4. Create pod with eStargz image:
   ```bash
   cat <<EOF | oc apply -f -
   apiVersion: v1
   kind: Pod
   metadata:
     name: test-fallback-pod
     namespace: default
   spec:
     nodeName: $WORKER_NODE
     containers:
     - name: test
       image: quay.io/openshifttest/additional-storage-tests:test-6gb-estargz-v1.0
       command: ["sh", "-c", "echo Fallback to standard; sleep 3600"]
     restartPolicy: Never
   EOF
   ```

5. Verify pod starts successfully (fallback to standard pull):
   ```bash
   oc wait --for=condition=Ready pod/test-fallback-pod --timeout=600s
   # Will be slower (full download) but should succeed
   ```

6. Check pod status:
   ```bash
   oc get pod test-fallback-pod
   # STATUS should be Running
   ```

7. Restart stargz-store service:
   ```bash
   oc debug node/$WORKER_NODE -- chroot /host systemctl start stargz-store
   ```

8. Verify service restarted:
   ```bash
   oc debug node/$WORKER_NODE -- chroot /host systemctl status stargz-store
   # Should be active (running)
   ```

9. Cleanup:
   ```bash
   oc delete pod test-fallback-pod
   ```

**Expected Result:**
- stargz-store service stops successfully
- Pod with eStargz image creates successfully despite service being down
- CRI-O falls back to standard pull (no lazy pulling)
- Pod reaches Running state
- No critical errors (warnings may appear in logs)
- Service can be restarted successfully
- Demonstrates resilient fallback behavior

**Cleanup:**
```bash
# Already performed in step 9
# Ensure stargz-store is running for subsequent tests
```

---

## Important Notes

### Disruptive Nature
⚠️ All ContainerRuntimeConfig changes trigger MCP rollouts with node reboots (10-20 minutes)

### Timing
- stargz-store Installation: 5-10 minutes
- MCP Rollout: 10-20 minutes
- Each E2E test: 45-60 minutes

### eStargz Requirement
**Critical:** Lazy pulling only works with eStargz-formatted images. Standard OCI images fall back to normal pull.

Create eStargz images using:
```bash
ctr-remote image optimize --oci --estargz SOURCE:TAG DEST:TAG
```

### stargz-store Version
Current automation uses v0.18.2. Update periodically from: https://github.com/containerd/stargz-snapshotter/releases

### Snapshot Growth
Snapshots accumulate in `/var/lib/stargz-store/store` over time. Monitor disk usage.

### FUSE Requirement
stargz-store requires FUSE support. Privileged SCC is needed for FUSE mounts.

### Test Execution Summary

| Category | Tests | Time |
|----------|-------|------|
| API Validation | TC1-TC8 | 15 min |
| E2E Functional | TC9-TC13 | 150-180 min |
| **Total** | **13** | **~3 hours** |

---

**Document Version:** 1.0  
**Last Updated:** 2026-06-04  
**Automation File:** `additional_layer_stores.go`  
**Related Epic:** OCPNODE-4051
