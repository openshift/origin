# etcd.openshift.io API Group

This API group contains CRDs related to etcd cluster management in Two Node OpenShift with Fencing deployments.

## API Versions

### v1 and v1alpha1

Contains the `PacemakerCluster` custom resource for monitoring Pacemaker cluster health in Two Node OpenShift with Fencing deployments.

#### PacemakerCluster

- **Alert-agent status feature gate (v1 only)**: `TNFObservability` (`TechPreviewNoUpgrade` and `DevPreviewNoUpgrade`, self-managed clusters)
- **Component**: `two-node-fencing`
- **Scope**: Cluster-scoped singleton resource (must be named "cluster")
- **Resource Path**: `pacemakerclusters.etcd.openshift.io`

The `PacemakerCluster` resource provides visibility into the health and status of a Pacemaker-managed cluster.
It is periodically updated by the cluster-etcd-operator's status collector.

### Status Subresource Design

This resource uses the standard Kubernetes status subresource pattern (`+kubebuilder:subresource:status`).
The status collector creates the resource without status, then immediately populates it via the `/status` endpoint.

**Why not atomic create-with-status?**

We initially explored removing the status subresource to allow creating the resource with status in a single
atomic operation. This would ensure the resource is never observed in an incomplete state. However:

1. The Kubernetes API server strips the `status` field from create requests when a status subresource is enabled
2. Without the subresource, we cannot use separate RBAC for spec vs status updates
3. The OpenShift API test framework assumes status subresource exists for status update tests

The status collector performs a two-step operation: create resource, then immediately update status.
The brief window where status is empty is acceptable since the healthcheck controller handles missing status gracefully.

### Pacemaker Resources

A **pacemaker resource** is a unit of work managed by pacemaker. In pacemaker terminology, resources are services
or applications that pacemaker monitors, starts, stops, and moves between nodes to maintain high availability.
This is distinct from fencing agents and alert agents, which pacemaker does not manage as resources (see the
[Fencing Agents](#fencing-agents) and [Alert Agents](#alert-agents) sections below).

For Two Node OpenShift with Fencing, we manage two resource types:
- **Kubelet**: The Kubernetes node agent and a prerequisite for etcd
- **Etcd**: The distributed key-value store

Fencing agents and alert agents are tracked separately in each node's `fencingAgents` and `alertAgents`
arrays, not in `resources`.

### Status Structure

```yaml
status:                    # Optional on creation, populated via status subresource
  conditions:              # Required when status present (min 3 items)
    - type: Healthy
    - type: InService
    - type: NodeCountAsExpected
  lastUpdated: <timestamp> # Required when status present, cannot decrease
  nodes:                   # Control-plane nodes (0-5, expects 2 for TNF)
    - nodeName: <hostname> # RFC 1123 subdomain name
      addresses:           # Required: List of node addresses (1-8 items)
        - type: InternalIP # Currently only InternalIP is supported
          address: <ip>    # First address used for etcd peer URLs
      conditions:          # Required: Node-level conditions (min 9 items)
        - type: Healthy
        - type: Online
        - type: InService
        - type: Active
        - type: Ready
        - type: Clean
        - type: Member
        - type: FencingAvailable
        - type: FencingHealthy
      resources:           # Required: Pacemaker resources on this node (min 2: Kubelet + Etcd)
        - name: Kubelet    # Both Kubelet and Etcd must be present
          conditions:      # Required: Resource-level conditions (min 8 items)
            - type: Healthy
            - type: InService
            - type: Managed
            - type: Enabled
            - type: Operational
            - type: Active
            - type: Started
            - type: Schedulable
        - name: Etcd
          conditions: [...]  # Same 8 conditions as Kubelet (abbreviated)
      fencingAgents:       # Required: Fencing agents for THIS node (1-8)
        - name: <unique_id>  # e.g., "master-0_redfish" (unique, max 300 chars)
          method: <method>   # Fencing method: "Redfish" or "IPMI"
          conditions: [...]  # Same 8 conditions as resources (abbreviated)
      alertAgents:         # Optional: Exactly 2 alert agents when present
        - name: TaintAlertAgent
          conditions:      # Required when present: 3 conditions per entry
            - type: Healthy
            - type: Configured  # Registered in the CIB's <alerts> section as expected
            - type: Available   # Delivery chain (script, helper, systemd unit, sudoers) present on this node
        - name: UntaintAlertAgent
          conditions: [...]     # Same 3 conditions as TaintAlertAgent (abbreviated)
```

### Fencing Agents

Fencing agents are STONITH (Shoot The Other Node In The Head) devices used to isolate failed nodes.
Unlike regular pacemaker resources (Kubelet, Etcd), fencing agents are tracked separately because:

1. **Mapping by target, not schedule**: Resources are mapped to the node where they are scheduled to run.
   Fencing agents are mapped to the node they can *fence* (their target), regardless of which node
   their monitoring operations are scheduled on.

2. **Multiple agents per node**: A node can have multiple fencing agents for redundancy
   (e.g., both Redfish and IPMI). Expected: 1 per node, supported: up to 8.

3. **Health tracking via two node-level conditions**:
   - **FencingAvailable**: True if at least one agent is healthy (fencing works), False if all agents unhealthy (degrades operator)
   - **FencingHealthy**: True if all agents are healthy (ideal state), False if any agent is unhealthy (emits warning events)

### Alert Agents

The `v1` API exposes `status.nodes[].alertAgents` only when the `TNFObservability`
feature gate is enabled. The gate is initially enabled for self-managed clusters in
`TechPreviewNoUpgrade` and `DevPreviewNoUpgrade`. When disabled, the CRD schema omits the entire
alert-agent status subtree and its node-level retention validation, and the API server prunes
submitted `alertAgents` fields.
The `v1alpha1` API does not expose alert-agent status.

Alert agents are pacemaker alert handlers used to automatically taint a node after it is fenced, and remove
that taint once the node rejoins the cluster. There are two known alert agents: `TaintAlertAgent` and
`UntaintAlertAgent`. Unlike a pacemaker resource, an alert agent lives in the CIB's `<alerts>` section, not
`<resources>`: pacemaker doesn't start, stop, monitor, or move it. Its delivery chain (script, helper
script, systemd unit, sudoers entry) is delivered to each node independently by MCO.

Alert agents are tracked in each node's `alertAgents` array, using the dedicated `PacemakerClusterAlertAgentStatus`
type. Each entry carries exactly 3 conditions:
- `Configured` - the alert agent is registered in the CIB as expected (same value on every node, since it's a single cluster-wide CIB entry)
- `Available` - the alert agent's delivery chain is present and executable on this node
- `Healthy` - aggregate of the two conditions above

Alert-agent health folds upward through an optional node-level `AlertAgentsHealthy` condition into `node.Healthy`
and `cluster.Healthy`, the same way `fencingAgents[].Healthy` folds into `FencingAvailable`/`FencingHealthy`.

The `alertAgents` field itself is optional: it is omitted by status collector versions
that predate alert-agent tracking, so it cannot be required without breaking status updates from
those collectors during an upgrade. For the same reason, `AlertAgentsHealthy` has no `MinItems` bump or
`self.exists()` rule backing it. It is a value in the extensible node conditions list, so the field
feature-gate marker does not restrict that condition value. Collectors must report it and include
alert-agent health in aggregate health only when the gate is enabled. While the gate is enabled,
once a status collector reports `alertAgents` for a node, it may not be removed on a later update.
When `alertAgents` is present, it must contain exactly two entries: one `TaintAlertAgent` and one
`UntaintAlertAgent`. An empty list is invalid.

### Cluster-Level Conditions

| Condition | True | False |
|-----------|------|-------|
| `Healthy` | Cluster is healthy (`ClusterHealthy`) | Cluster has issues (`ClusterUnhealthy`) |
| `InService` | In service (`InService`) | In maintenance (`InMaintenance`) |
| `NodeCountAsExpected` | Node count is as expected (`AsExpected`) | Wrong count (`InsufficientNodes`, `ExcessiveNodes`) |

### Node-Level Conditions

| Condition | True | False |
|-----------|------|-------|
| `Healthy` | Node is healthy (`NodeHealthy`) | Node has issues (`NodeUnhealthy`) |
| `Online` | Node is online (`Online`) | Node is offline (`Offline`) |
| `InService` | In service (`InService`) | In maintenance (`InMaintenance`) |
| `Active` | Node is active (`Active`) | Node is in standby (`Standby`) |
| `Ready` | Node is ready (`Ready`) | Node is pending (`Pending`) |
| `Clean` | Node is clean (`Clean`) | Node is unclean (`Unclean`) |
| `Member` | Node is a member (`Member`) | Not a member (`NotMember`) |
| `FencingAvailable` | At least one agent healthy (`FencingAvailable`) | All agents unhealthy (`FencingUnavailable`) - degrades operator |
| `FencingHealthy` | All agents healthy (`FencingHealthy`) | Some agents unhealthy (`FencingUnhealthy`) - emits warnings |
| `AlertAgentsHealthy`* | All alert agents healthy (`AllAlertAgentsHealthy`) | Some alert agents unhealthy (`SomeAlertAgentsUnhealthy`) |

\* Optional; absent when a status collector hasn't reported alert-agent status yet.
When reported as Unknown with reason `Pending`, `AlertAgentsHealthy` means the health of the tracked
alert agents has not yet been determined.

### Resource-Level Conditions

Each pacemaker-managed resource (`Kubelet`, `Etcd`) in the `resources` array and each fencing agent in the
`fencingAgents` array has all eight of the following conditions.

| Condition | True | False |
|-----------|------|-------|
| `Healthy` | Resource is healthy (`ResourceHealthy`) | Resource has issues (`ResourceUnhealthy`) |
| `InService` | In service (`InService`) | In maintenance (`InMaintenance`) |
| `Managed` | Managed by pacemaker (`Managed`) | Not managed (`Unmanaged`) |
| `Enabled` | Resource is enabled (`Enabled`) | Resource is disabled (`Disabled`) |
| `Operational` | Resource is operational (`Operational`) | Resource has failed (`Failed`) |
| `Active` | Resource is active (`Active`) | Resource is not active (`Inactive`) |
| `Started` | Resource is started (`Started`) | Resource is stopped (`Stopped`) |
| `Schedulable` | Resource is schedulable (`Schedulable`) | Resource is not schedulable (`Unschedulable`) |

### Alert-Agent-Level Conditions

Each entry in the `alertAgents` array has the following 3 conditions.

| Condition | True | False | Unknown |
|-----------|------|-------|---------|
| `Healthy` | Alert agent is healthy (`AlertAgentHealthy`) | Alert agent has issues (`AlertAgentUnhealthy`) | Health not yet determined because one or both signals have not been observed (`Pending`) |
| `Configured` | Registered in the CIB as expected (`Configured`) | Not registered (`NotRegistered`) or misconfigured (`Misconfigured`) | Not yet observed (`Pending`) |
| `Available` | Delivery chain present on this node (`Available`) | Part of the chain missing (`Unavailable`) | Not yet observed (`Pending`) |

### Validation Rules

**Resource naming:**
- Resource name must be "cluster" (singleton)

**Node name validation:**
- Must be a lowercase RFC 1123 subdomain name
- Consists of lowercase alphanumeric characters, '-' or '.'
- Must start and end with an alphanumeric character
- Maximum 253 characters

**Node addresses:**
- Uses `PacemakerNodeAddress` type (similar to `corev1.NodeAddress` but with IP validation)
- Currently only `InternalIP` type is supported
- Pacemaker allows multiple addresses for Corosync communication between nodes (1-8 addresses)
- The first address in the list is used for IP-based peer URLs for etcd membership
- IP validation:
  - Must be a valid global unicast IPv4 or IPv6 address
  - Must be in canonical form (e.g., `192.168.1.1` not `192.168.001.001`, or `2001:db8::1` not `2001:0db8::1`)
  - Excludes loopback, link-local, and multicast addresses
  - Maximum length is 39 characters (full IPv6 address)

**Timestamp validation:**
- `lastUpdated` is required when status is present
- Once set, cannot be set to an earlier timestamp (validation uses `!has(oldSelf.lastUpdated)` to handle initial creation)
- Timestamps must always increase (prevents stale updates from overwriting newer data)

**Status fields:**
- `status` - Optional on creation (pointer type), populated via status subresource
- When status is present, `conditions`, `lastUpdated`, and `nodes` are required:
  - `conditions` - Required array of cluster conditions (min 3 items: Healthy, InService, NodeCountAsExpected)
  - `lastUpdated` - Required timestamp for staleness detection
  - `nodes` - Required array of control-plane node statuses (min 0, max 5; empty allowed for catastrophic failures)

**Node fields (when node present):**
- `nodeName` - Required, RFC 1123 subdomain
- `addresses` - Required (min 1, max 8 items)
- `conditions` - Required (min 9 items with specific types enforced via XValidation)
- `resources` - Required (min 2 items: Kubelet and Etcd)
- `fencingAgents` - Required (min 1, max 8 items)
- `alertAgents` - Optional (exactly 2 items when present); omitted by status collectors that predate alert-agent tracking; once present, may not be removed; both `TaintAlertAgent` and `UntaintAlertAgent` are required

**Conditions validation:**
- Cluster-level: MinItems=3 (Healthy, InService, NodeCountAsExpected)
- Node-level: MinItems=9 (Healthy, Online, InService, Active, Ready, Clean, Member, FencingAvailable, FencingHealthy); an optional 10th, AlertAgentsHealthy, is not required
- Resource-level: MinItems=8 (Healthy, InService, Managed, Enabled, Operational, Active, Started, Schedulable)
- Fencing agent-level: MinItems=8 (same conditions as resources)
- Alert-agent-level: MinItems=3, MaxItems=3 (exactly Healthy, Configured, Available)

All condition arrays have XValidation rules to ensure specific condition types are present.

**Resource names:**
- Valid values are: `Kubelet`, `Etcd`
- Both resources must be present in each node's `resources` array

**Alert agent names:**
- Valid values are: `TaintAlertAgent`, `UntaintAlertAgent`
- The `alertAgents` array itself is optional and may be omitted before it is first reported
- When present, `alertAgents` must contain exactly 2 entries; an empty list is invalid (enforced via MinItems=2 and MaxItems=2)
- Both `TaintAlertAgent` and `UntaintAlertAgent` must be present (enforced via XValidation)
- Names must be unique within the `alertAgents` array (enforced via the `name` list-map key)
- Once `alertAgents` is reported for a node, it may not be removed on a later update (enforced via XValidation on `PacemakerClusterNodeStatus`)

**Fencing agent fields:**
- `name`: Unique identifier for the fencing agent (e.g., "master-0_redfish")
  - Must be unique within the `fencingAgents` array
  - May contain alphanumeric characters, dots, hyphens, and underscores (`^[a-zA-Z0-9._-]+$`)
  - Maximum 300 characters (provides headroom beyond 253 node name + underscore + method)
- `method`: Fencing method enum - valid values are `Redfish` or `IPMI`
- `conditions`: Required, same 8 conditions as resources

Note: The target node is implied by the parent `PacemakerClusterNodeStatus` - fencing agents are nested under the node they can fence.

### Usage

The cluster-etcd-operator healthcheck controller watches this resource and updates operator conditions based on
the cluster state. The aggregate `Healthy` conditions at each level (cluster, node, resource) provide a quick
way to determine overall health.
