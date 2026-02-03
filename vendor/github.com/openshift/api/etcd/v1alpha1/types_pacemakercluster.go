package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PacemakerCluster is used in Two Node OpenShift with Fencing deployments to monitor the health
// of etcd running under pacemaker.

// Cluster-level condition types for PacemakerCluster.status.conditions
const (
	// ClusterHealthyConditionType tracks the overall health of the pacemaker cluster.
	// This is an aggregate condition that reflects the health of all cluster-level conditions and node health.
	// Specifically, it aggregates the following conditions:
	// - ClusterInServiceConditionType
	// - ClusterNodeCountAsExpectedConditionType
	// - NodeHealthyConditionType (for each node)
	// When True, the cluster is healthy with reason "ClusterHealthy".
	// When False, the cluster is unhealthy with reason "ClusterUnhealthy".
	ClusterHealthyConditionType = "Healthy"

	// ClusterInServiceConditionType tracks whether the cluster is in service (not in maintenance mode).
	// Maintenance mode is a cluster-wide setting that prevents pacemaker from starting or stopping resources.
	// When True, the cluster is in service with reason "InService". This is the normal operating state.
	// When False, the cluster is in maintenance mode with reason "InMaintenance". This is an unexpected state.
	ClusterInServiceConditionType = "InService"

	// ClusterNodeCountAsExpectedConditionType tracks whether the cluster has the expected number of nodes.
	// For Two Node OpenShift with Fencing, we are expecting exactly 2 nodes.
	// When True, the expected number of nodes are present with reason "AsExpected".
	// When False, the node count is incorrect with reason "InsufficientNodes" or "ExcessiveNodes".
	ClusterNodeCountAsExpectedConditionType = "NodeCountAsExpected"
)

// ClusterHealthy condition reasons
const (
	// ClusterHealthyReasonHealthy means the pacemaker cluster is healthy and operating normally.
	ClusterHealthyReasonHealthy = "ClusterHealthy"

	// ClusterHealthyReasonUnhealthy means the pacemaker cluster has issues that need investigation.
	ClusterHealthyReasonUnhealthy = "ClusterUnhealthy"
)

// ClusterInService condition reasons
const (
	// ClusterInServiceReasonInService means the cluster is in service (not in maintenance mode).
	// This is the normal operating state.
	ClusterInServiceReasonInService = "InService"

	// ClusterInServiceReasonInMaintenance means the cluster is in maintenance mode.
	// In maintenance mode, pacemaker will not start or stop any resources. Entering and exiting this state requires
	// manual user intervention, and is unexpected during normal cluster operation.
	ClusterInServiceReasonInMaintenance = "InMaintenance"
)

// ClusterNodeCountAsExpected condition reasons
const (
	// ClusterNodeCountAsExpectedReasonAsExpected means the expected number of nodes are present.
	// For Two Node OpenShift with Fencing, we are expecting exactly 2 nodes. This is the expected healthy state.
	ClusterNodeCountAsExpectedReasonAsExpected = "AsExpected"

	// ClusterNodeCountAsExpectedReasonInsufficientNodes means fewer nodes than expected are present.
	// For Two Node OpenShift with Fencing, this means that less than 2 nodes are present. Under normal operation, this will only happen during
	// a node replacement operation. It's also possible to enter this state with manual user intervention, but
	// will also require user intervention to restore normal functionality.
	ClusterNodeCountAsExpectedReasonInsufficientNodes = "InsufficientNodes"

	// ClusterNodeCountAsExpectedReasonExcessiveNodes means more nodes than expected are present.
	// For Two Node OpenShift with Fencing, this means more than 2 nodes are present. This should be investigated as it is unexpected and should
	// never happen during normal cluster operation. It is possible to enter this state with manual user intervention,
	// but will also require user intervention to restore normal functionality.
	ClusterNodeCountAsExpectedReasonExcessiveNodes = "ExcessiveNodes"
)

// Node-level condition types for PacemakerCluster.status.nodes[].conditions
const (
	// NodeHealthyConditionType tracks the overall health of a node in the pacemaker cluster.
	// This is an aggregate condition that reflects the health of all node-level conditions and resource health.
	// Specifically, it aggregates the following conditions:
	// - NodeOnlineConditionType
	// - NodeInServiceConditionType
	// - NodeActiveConditionType
	// - NodeReadyConditionType
	// - NodeCleanConditionType
	// - NodeMemberConditionType
	// - NodeFencingAvailableConditionType
	// - NodeFencingHealthyConditionType
	// - ResourceHealthyConditionType (for each resource in the node's resources list)
	// When True, the node is healthy with reason "NodeHealthy".
	// When False, the node is unhealthy with reason "NodeUnhealthy".
	NodeHealthyConditionType = "Healthy"

	// NodeOnlineConditionType tracks whether a node is online.
	// When True, the node is online with reason "Online". This is the normal operating state.
	// When False, the node is offline with reason "Offline". This can occur during reboots, failures, maintenance, or replacement.
	NodeOnlineConditionType = "Online"

	// NodeInServiceConditionType tracks whether a node is in service (not in maintenance mode).
	// A node in maintenance mode is ignored by pacemaker while maintenance mode is active.
	// When True, the node is in service with reason "InService". This is the normal operating state.
	// When False, the node is in maintenance mode with reason "InMaintenance". This is an unexpected state.
	NodeInServiceConditionType = "InService"

	// NodeActiveConditionType tracks whether a node is active (not in standby mode).
	// When a node enters standby mode, pacemaker moves its resources to other nodes in the cluster.
	// In Two Node OpenShift with Fencing, we do not use standby mode during normal operation.
	// When True, the node is active with reason "Active". This is the normal operating state.
	// When False, the node is in standby mode with reason "Standby". This is an unexpected state.
	NodeActiveConditionType = "Active"

	// NodeReadyConditionType tracks whether a node is ready (not in a pending state).
	// A node in a pending state is in the process of joining or leaving the cluster.
	// When True, the node is ready with reason "Ready". This is the normal operating state.
	// When False, the node is pending with reason "Pending". This is expected to be temporary.
	NodeReadyConditionType = "Ready"

	// NodeCleanConditionType tracks whether a node is in a clean state.
	// An unclean state means that pacemaker was unable to confirm the node's state, which signifies issues
	// in fencing, communication, or configuration.
	// When True, the node is clean with reason "Clean". This is the normal operating state.
	// When False, the node is unclean with reason "Unclean". This is an unexpected state.
	NodeCleanConditionType = "Clean"

	// NodeMemberConditionType tracks whether a node is a member of the cluster.
	// Some configurations may use remote nodes or ping nodes, which are nodes that are not members.
	// For Two Node OpenShift with Fencing, we expect both nodes to be members.
	// When True, the node is a member with reason "Member". This is the normal operating state.
	// When False, the node is not a member with reason "NotMember". This is an unexpected state.
	NodeMemberConditionType = "Member"

	// NodeFencingAvailableConditionType tracks whether a node can be fenced by at least one fencing agent.
	// For Two Node OpenShift with Fencing, each node needs at least one healthy fencing agent to ensure
	// that the cluster can recover from a node failure via STONITH (Shoot The Other Node In The Head).
	// When True, at least one fencing agent is healthy with reason "FencingAvailable".
	// When False, all fencing agents are unhealthy with reason "FencingUnavailable". This is a critical
	// state that should degrade the operator.
	NodeFencingAvailableConditionType = "FencingAvailable"

	// NodeFencingHealthyConditionType tracks whether all fencing agents for a node are healthy.
	// This is an aggregate condition that reflects the health of all fencing agents targeting this node.
	// When True, all fencing agents are healthy with reason "FencingHealthy".
	// When False, one or more fencing agents are unhealthy with reason "FencingUnhealthy". Warning events
	// should be emitted for failing agents, but the operator should not be degraded if FencingAvailable is True.
	NodeFencingHealthyConditionType = "FencingHealthy"
)

// NodeHealthy condition reasons
const (
	// NodeHealthyReasonHealthy means the node is healthy and operating normally.
	NodeHealthyReasonHealthy = "NodeHealthy"

	// NodeHealthyReasonUnhealthy means the node has issues that need investigation.
	NodeHealthyReasonUnhealthy = "NodeUnhealthy"
)

// NodeOnline condition reasons
const (
	// NodeOnlineReasonOnline means the node is online. This is the normal operating state.
	NodeOnlineReasonOnline = "Online"

	// NodeOnlineReasonOffline means the node is offline.
	NodeOnlineReasonOffline = "Offline"
)

// NodeInService condition reasons
const (
	// NodeInServiceReasonInService means the node is in service (not in maintenance mode).
	// This is the normal operating state.
	NodeInServiceReasonInService = "InService"

	// NodeInServiceReasonInMaintenance means the node is in maintenance mode.
	// This is an unexpected state.
	NodeInServiceReasonInMaintenance = "InMaintenance"
)

// NodeActive condition reasons
const (
	// NodeActiveReasonActive means the node is active (not in standby mode).
	// This is the normal operating state.
	NodeActiveReasonActive = "Active"

	// NodeActiveReasonStandby means the node is in standby mode.
	// This is an unexpected state.
	NodeActiveReasonStandby = "Standby"
)

// NodeReady condition reasons
const (
	// NodeReadyReasonReady means the node is ready (not in a pending state).
	// This is the normal operating state.
	NodeReadyReasonReady = "Ready"

	// NodeReadyReasonPending means the node is joining or leaving the cluster.
	// This state is expected to be temporary.
	NodeReadyReasonPending = "Pending"
)

// NodeClean condition reasons
const (
	// NodeCleanReasonClean means the node is in a clean state.
	// This is the normal operating state.
	NodeCleanReasonClean = "Clean"

	// NodeCleanReasonUnclean means the node is in an unclean state.
	// Pacemaker was unable to confirm the node's state, which signifies issues in fencing, communication, or configuration.
	// This is an unexpected state.
	NodeCleanReasonUnclean = "Unclean"
)

// NodeMember condition reasons
const (
	// NodeMemberReasonMember means the node is a member of the cluster.
	// For Two Node OpenShift with Fencing, we expect both nodes to be members. This is the normal operating state.
	NodeMemberReasonMember = "Member"

	// NodeMemberReasonNotMember means the node is not a member of the cluster.
	// This is an unexpected state.
	NodeMemberReasonNotMember = "NotMember"
)

// NodeFencingAvailable condition reasons
const (
	// NodeFencingAvailableReasonAvailable means at least one fencing agent for this node is healthy.
	// The cluster can fence this node if needed. This is the normal operating state.
	NodeFencingAvailableReasonAvailable = "FencingAvailable"

	// NodeFencingAvailableReasonUnavailable means all fencing agents for this node are unhealthy.
	// The cluster cannot fence this node, which compromises high availability.
	// This is a critical state that should degrade the operator.
	NodeFencingAvailableReasonUnavailable = "FencingUnavailable"
)

// NodeFencingHealthy condition reasons
const (
	// NodeFencingHealthyReasonHealthy means all fencing agents for this node are healthy.
	// This is the ideal operating state with full redundancy.
	NodeFencingHealthyReasonHealthy = "FencingHealthy"

	// NodeFencingHealthyReasonUnhealthy means one or more fencing agents for this node are unhealthy.
	// Warning events should be emitted for failing agents, but the operator should not be degraded
	// if FencingAvailable is still True.
	NodeFencingHealthyReasonUnhealthy = "FencingUnhealthy"
)

// Resource-level condition types for PacemakerCluster.status.nodes[].resources[].conditions
const (
	// ResourceHealthyConditionType tracks the overall health of a pacemaker resource.
	// This is an aggregate condition that reflects the health of all resource-level conditions.
	// Specifically, it aggregates the following conditions:
	// - ResourceInServiceConditionType
	// - ResourceManagedConditionType
	// - ResourceEnabledConditionType
	// - ResourceOperationalConditionType
	// - ResourceActiveConditionType
	// - ResourceStartedConditionType
	// - ResourceSchedulableConditionType
	// When True, the resource is healthy with reason "ResourceHealthy".
	// When False, the resource is unhealthy with reason "ResourceUnhealthy".
	ResourceHealthyConditionType = "Healthy"

	// ResourceInServiceConditionType tracks whether a resource is in service (not in maintenance mode).
	// Resources in maintenance mode are not monitored or moved by pacemaker.
	// In Two Node OpenShift with Fencing, we do not expect any resources to be in maintenance mode.
	// When True, the resource is in service with reason "InService". This is the normal operating state.
	// When False, the resource is in maintenance mode with reason "InMaintenance". This is an unexpected state.
	ResourceInServiceConditionType = "InService"

	// ResourceManagedConditionType tracks whether a resource is managed by pacemaker.
	// Resources that are not managed by pacemaker are effectively invisible to the pacemaker HA logic.
	// For Two Node OpenShift with Fencing, all resources are expected to be managed.
	// When True, the resource is managed with reason "Managed". This is the normal operating state.
	// When False, the resource is not managed with reason "Unmanaged". This is an unexpected state.
	ResourceManagedConditionType = "Managed"

	// ResourceEnabledConditionType tracks whether a resource is enabled.
	// Resources that are disabled are stopped and not automatically managed or started by the cluster.
	// In Two Node OpenShift with Fencing, we do not expect any resources to be disabled.
	// When True, the resource is enabled with reason "Enabled". This is the normal operating state.
	// When False, the resource is disabled with reason "Disabled". This is an unexpected state.
	ResourceEnabledConditionType = "Enabled"

	// ResourceOperationalConditionType tracks whether a resource is operational (not failed).
	// A failed resource is one that is not able to start or is in an error state.
	// When True, the resource is operational with reason "Operational". This is the normal operating state.
	// When False, the resource has failed with reason "Failed". This is an unexpected state.
	ResourceOperationalConditionType = "Operational"

	// ResourceActiveConditionType tracks whether a resource is active.
	// An active resource is running on a cluster node.
	// In Two Node OpenShift with Fencing, all resources are expected to be active.
	// When True, the resource is active with reason "Active". This is the normal operating state.
	// When False, the resource is not active with reason "Inactive". This is an unexpected state.
	ResourceActiveConditionType = "Active"

	// ResourceStartedConditionType tracks whether a resource is started.
	// It's normal for a resource like etcd to become stopped in the event of a quorum loss event because
	// the pacemaker recovery logic will fence a node and restore etcd quorum on the surviving node as a cluster-of-one.
	// A resource that stays stopped for an extended period of time is an unexpected state and should be investigated.
	// When True, the resource is started with reason "Started". This is the normal operating state.
	// When False, the resource is not started with reason "Stopped". This is expected to be temporary.
	ResourceStartedConditionType = "Started"

	// ResourceSchedulableConditionType tracks whether a resource is schedulable (not blocked).
	// A resource that is not schedulable is unable to start or move to a different node.
	// In Two Node OpenShift with Fencing, we do not expect any resources to be unschedulable.
	// When True, the resource is schedulable with reason "Schedulable". This is the normal operating state.
	// When False, the resource is not schedulable with reason "Unschedulable". This is an unexpected state.
	ResourceSchedulableConditionType = "Schedulable"
)

// ResourceHealthy condition reasons
const (
	// ResourceHealthyReasonHealthy means the resource is healthy and operating normally.
	ResourceHealthyReasonHealthy = "ResourceHealthy"

	// ResourceHealthyReasonUnhealthy means the resource has issues that need investigation.
	ResourceHealthyReasonUnhealthy = "ResourceUnhealthy"
)

// ResourceInService condition reasons
const (
	// ResourceInServiceReasonInService means the resource is in service (not in maintenance mode).
	// This is the normal operating state.
	ResourceInServiceReasonInService = "InService"

	// ResourceInServiceReasonInMaintenance means the resource is in maintenance mode.
	// Resources in maintenance mode are not monitored or moved by pacemaker. This is an unexpected state.
	ResourceInServiceReasonInMaintenance = "InMaintenance"
)

// ResourceManaged condition reasons
const (
	// ResourceManagedReasonManaged means the resource is managed by pacemaker.
	// This is the normal operating state.
	ResourceManagedReasonManaged = "Managed"

	// ResourceManagedReasonUnmanaged means the resource is not managed by pacemaker.
	// Resources that are not managed by pacemaker are effectively invisible to the pacemaker HA logic.
	// This is an unexpected state.
	ResourceManagedReasonUnmanaged = "Unmanaged"
)

// ResourceEnabled condition reasons
const (
	// ResourceEnabledReasonEnabled means the resource is enabled.
	// This is the normal operating state.
	ResourceEnabledReasonEnabled = "Enabled"

	// ResourceEnabledReasonDisabled means the resource is disabled.
	// Resources that are disabled are stopped and not automatically managed or started by the cluster.
	// This is an unexpected state.
	ResourceEnabledReasonDisabled = "Disabled"
)

// ResourceOperational condition reasons
const (
	// ResourceOperationalReasonOperational means the resource is operational (not failed).
	// This is the normal operating state.
	ResourceOperationalReasonOperational = "Operational"

	// ResourceOperationalReasonFailed means the resource has failed.
	// A failed resource is one that is not able to start or is in an error state. This is an unexpected state.
	ResourceOperationalReasonFailed = "Failed"
)

// ResourceActive condition reasons
const (
	// ResourceActiveReasonActive means the resource is active.
	// An active resource is running on a cluster node. This is the normal operating state.
	ResourceActiveReasonActive = "Active"

	// ResourceActiveReasonInactive means the resource is not active.
	// This is an unexpected state.
	ResourceActiveReasonInactive = "Inactive"
)

// ResourceStarted condition reasons
const (
	// ResourceStartedReasonStarted means the resource is started.
	// This is the normal operating state.
	ResourceStartedReasonStarted = "Started"

	// ResourceStartedReasonStopped means the resource is stopped.
	// It's normal for a resource like etcd to become stopped in the event of a quorum loss event because
	// the pacemaker recovery logic will fence a node and restore etcd quorum on the surviving node as a cluster-of-one.
	// A resource that stays stopped for an extended period of time is an unexpected state and should be investigated.
	ResourceStartedReasonStopped = "Stopped"
)

// ResourceSchedulable condition reasons
const (
	// ResourceSchedulableReasonSchedulable means the resource is schedulable (not blocked).
	// This is the normal operating state.
	ResourceSchedulableReasonSchedulable = "Schedulable"

	// ResourceSchedulableReasonUnschedulable means the resource is not schedulable (blocked).
	// A resource that is not schedulable is unable to start or move to a different node. This is an unexpected state.
	ResourceSchedulableReasonUnschedulable = "Unschedulable"
)

// PacemakerNodeAddressType represents the type of a node address.
// Currently only InternalIP is supported.
// +kubebuilder:validation:Enum=InternalIP
// +enum
type PacemakerNodeAddressType string

const (
	// PacemakerNodeInternalIP is an internal IP address assigned to the node.
	// This is typically the IP address used for intra-cluster communication.
	PacemakerNodeInternalIP PacemakerNodeAddressType = "InternalIP"
)

// PacemakerNodeAddress contains information for a node's address.
// This is similar to corev1.NodeAddress but adds validation for IP addresses.
type PacemakerNodeAddress struct {
	// type is the type of node address.
	// Currently only "InternalIP" is supported.
	// +required
	Type PacemakerNodeAddressType `json:"type,omitempty"`

	// address is the node address.
	// For InternalIP, this must be a valid global unicast IPv4 or IPv6 address in canonical form.
	// Canonical form means the shortest standard representation (e.g., "192.168.1.1" not "192.168.001.001",
	// or "2001:db8::1" not "2001:0db8::1"). Maximum length is 39 characters (full IPv6 address).
	// Global unicast includes private/RFC1918 addresses but excludes loopback, link-local, and multicast.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=39
	// +kubebuilder:validation:XValidation:rule="isIP(self) && ip.isCanonical(self) && ip(self).isGlobalUnicast()",message="must be a valid global unicast IPv4 or IPv6 address in canonical form"
	// +required
	Address string `json:"address,omitempty"`
}

// PacemakerClusterResourceName represents the name of a pacemaker resource.
// Fencing agents are tracked separately in the fencingAgents field.
// +kubebuilder:validation:Enum=Kubelet;Etcd
// +enum
type PacemakerClusterResourceName string

// PacemakerClusterResourceName values
const (
	// PacemakerClusterResourceNameKubelet is the kubelet pacemaker resource.
	// The kubelet resource is a prerequisite for etcd in Two Node OpenShift with Fencing deployments.
	PacemakerClusterResourceNameKubelet PacemakerClusterResourceName = "Kubelet"

	// PacemakerClusterResourceNameEtcd is the etcd pacemaker resource.
	// The etcd resource may temporarily transition to stopped during pacemaker quorum-recovery operations.
	PacemakerClusterResourceNameEtcd PacemakerClusterResourceName = "Etcd"
)

// FencingMethod represents the method used by a fencing agent to isolate failed nodes.
// Valid values are "Redfish" and "IPMI".
// +kubebuilder:validation:Enum=Redfish;IPMI
// +enum
type FencingMethod string

// FencingMethod values
const (
	// FencingMethodRedfish uses Redfish, a standard RESTful API for server management.
	FencingMethodRedfish FencingMethod = "Redfish"

	// FencingMethodIPMI uses IPMI (Intelligent Platform Management Interface), a hardware management interface.
	FencingMethodIPMI FencingMethod = "IPMI"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// PacemakerCluster represents the current state of the pacemaker cluster as reported by the pcs status command.
// PacemakerCluster is a cluster-scoped singleton resource. The name of this instance is "cluster". This
// resource provides a view into the health and status of a pacemaker-managed cluster in Two Node OpenShift with Fencing deployments.
//
// Compatibility level 4: No compatibility is provided, the API can change at any point for any reason. These capabilities should not be used by applications needing long term support.
// +openshift:compatibility-gen:level=4
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=pacemakerclusters,scope=Cluster,singular=pacemakercluster
// +kubebuilder:subresource:status
// +openshift:api-approved.openshift.io=https://github.com/openshift/api/pull/2544
// +openshift:file-pattern=cvoRunLevel=0000_25,operatorName=etcd,operatorOrdering=01,operatorComponent=two-node-fencing
// +openshift:enable:FeatureGate=DualReplica
// +kubebuilder:validation:XValidation:rule="self.metadata.name == 'cluster'",message="PacemakerCluster must be named 'cluster'"
// +kubebuilder:validation:XValidation:rule="!has(oldSelf.status) || has(self.status)",message="status may not be removed once set"
type PacemakerCluster struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +required
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// status contains the actual pacemaker cluster status information collected from the cluster.
	// The goal of this status is to be able to quickly identify if pacemaker is in a healthy state.
	// In Two Node OpenShift with Fencing, a healthy pacemaker cluster has 2 nodes, both of which have healthy kubelet, etcd, and fencing resources.
	// This field is optional on creation - the status collector populates it immediately after creating
	// the resource via the status subresource.
	// +optional
	Status PacemakerClusterStatus `json:"status,omitzero"`
}

// PacemakerClusterStatus contains the actual pacemaker cluster status information. As part of validating the status
// object, we need to ensure that the lastUpdated timestamp may not be set to an earlier timestamp than the current value.
// The validation rule checks if oldSelf has lastUpdated before comparing, to handle the initial status creation case.
// +kubebuilder:validation:XValidation:rule="!has(oldSelf.lastUpdated) || self.lastUpdated >= oldSelf.lastUpdated",message="lastUpdated may not be set to an earlier timestamp"
type PacemakerClusterStatus struct {
	// conditions represent the observations of the pacemaker cluster's current state.
	// Known condition types are: "Healthy", "InService", "NodeCountAsExpected".
	// The "Healthy" condition is an aggregate that tracks the overall health of the cluster.
	// The "InService" condition tracks whether the cluster is in service (not in maintenance mode).
	// The "NodeCountAsExpected" condition tracks whether the expected number of nodes are present.
	// Each of these conditions is required, so the array must contain at least 3 items.
	// +listType=map
	// +listMapKey=type
	// +kubebuilder:validation:MinItems=3
	// +kubebuilder:validation:MaxItems=8
	// +kubebuilder:validation:XValidation:rule="self.exists(c, c.type == 'Healthy')",message="conditions must contain a condition of type Healthy"
	// +kubebuilder:validation:XValidation:rule="self.exists(c, c.type == 'InService')",message="conditions must contain a condition of type InService"
	// +kubebuilder:validation:XValidation:rule="self.exists(c, c.type == 'NodeCountAsExpected')",message="conditions must contain a condition of type NodeCountAsExpected"
	// +required
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// lastUpdated is the timestamp when this status was last updated. This is useful for identifying
	// stale status reports. It must be a valid timestamp in RFC3339 format. Once set, this field cannot
	// be removed and cannot be set to an earlier timestamp than the current value.
	// +kubebuilder:validation:Format=date-time
	// +required
	LastUpdated metav1.Time `json:"lastUpdated,omitempty,omitzero"`

	// nodes provides detailed status for each control-plane node in the Pacemaker cluster.
	// While Pacemaker supports up to 32 nodes, the limit is set to 5 (max OpenShift control-plane nodes).
	// For Two Node OpenShift with Fencing, exactly 2 nodes are expected in a healthy cluster.
	// An empty list indicates a catastrophic failure where Pacemaker reports no nodes.
	// +listType=map
	// +listMapKey=nodeName
	// +kubebuilder:validation:MinItems=0
	// +kubebuilder:validation:MaxItems=5
	// +required
	Nodes *[]PacemakerClusterNodeStatus `json:"nodes,omitempty"`
}

// PacemakerClusterNodeStatus represents the status of a single node in the pacemaker cluster including
// the node's conditions and the health of critical resources running on that node.
type PacemakerClusterNodeStatus struct {
	// conditions represent the observations of the node's current state.
	// Known condition types are: "Healthy", "Online", "InService", "Active", "Ready", "Clean", "Member",
	// "FencingAvailable", "FencingHealthy".
	// The "Healthy" condition is an aggregate that tracks the overall health of the node.
	// The "Online" condition tracks whether the node is online.
	// The "InService" condition tracks whether the node is in service (not in maintenance mode).
	// The "Active" condition tracks whether the node is active (not in standby mode).
	// The "Ready" condition tracks whether the node is ready (not in a pending state).
	// The "Clean" condition tracks whether the node is in a clean (status known) state.
	// The "Member" condition tracks whether the node is a member of the cluster.
	// The "FencingAvailable" condition tracks whether this node can be fenced by at least one healthy agent.
	// The "FencingHealthy" condition tracks whether all fencing agents for this node are healthy.
	// Each of these conditions is required, so the array must contain at least 9 items.
	// +listType=map
	// +listMapKey=type
	// +kubebuilder:validation:MinItems=9
	// +kubebuilder:validation:MaxItems=16
	// +kubebuilder:validation:XValidation:rule="self.exists(c, c.type == 'Healthy')",message="conditions must contain a condition of type Healthy"
	// +kubebuilder:validation:XValidation:rule="self.exists(c, c.type == 'Online')",message="conditions must contain a condition of type Online"
	// +kubebuilder:validation:XValidation:rule="self.exists(c, c.type == 'InService')",message="conditions must contain a condition of type InService"
	// +kubebuilder:validation:XValidation:rule="self.exists(c, c.type == 'Active')",message="conditions must contain a condition of type Active"
	// +kubebuilder:validation:XValidation:rule="self.exists(c, c.type == 'Ready')",message="conditions must contain a condition of type Ready"
	// +kubebuilder:validation:XValidation:rule="self.exists(c, c.type == 'Clean')",message="conditions must contain a condition of type Clean"
	// +kubebuilder:validation:XValidation:rule="self.exists(c, c.type == 'Member')",message="conditions must contain a condition of type Member"
	// +kubebuilder:validation:XValidation:rule="self.exists(c, c.type == 'FencingAvailable')",message="conditions must contain a condition of type FencingAvailable"
	// +kubebuilder:validation:XValidation:rule="self.exists(c, c.type == 'FencingHealthy')",message="conditions must contain a condition of type FencingHealthy"
	// +required
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// nodeName is the name of the node. This is expected to match the Kubernetes node's name, which must be a lowercase
	// RFC 1123 subdomain consisting of lowercase alphanumeric characters, '-' or '.', starting and ending with
	// an alphanumeric character, and be at most 253 characters in length.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:XValidation:rule="!format.dns1123Subdomain().validate(self).hasValue()",message="nodeName must be a lowercase RFC 1123 subdomain consisting of lowercase alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character"
	// +required
	NodeName string `json:"nodeName,omitempty"`

	// addresses is a list of IP addresses for the node.
	// Pacemaker allows multiple IP addresses for Corosync communication between nodes.
	// The first address in this list is used for IP-based peer URLs for etcd membership.
	// Each address must be a valid global unicast IPv4 or IPv6 address in canonical form
	// (e.g., "192.168.1.1" not "192.168.001.001", or "2001:db8::1" not "2001:0db8::1").
	// This excludes loopback, link-local, and multicast addresses.
	// +listType=atomic
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=8
	// +required
	Addresses []PacemakerNodeAddress `json:"addresses,omitempty"`

	// resources contains the status of pacemaker resources scheduled on this node.
	// Each resource entry includes the resource name and its health conditions.
	// For Two Node OpenShift with Fencing, we track Kubelet and Etcd resources per node.
	// Both resources are required to be present, so the array must contain at least 2 items.
	// Valid resource names are "Kubelet" and "Etcd".
	// Fencing agents are tracked separately in the fencingAgents field.
	// +listType=map
	// +listMapKey=name
	// +kubebuilder:validation:MinItems=2
	// +kubebuilder:validation:MaxItems=8
	// +kubebuilder:validation:XValidation:rule="self.exists(r, r.name == 'Kubelet')",message="resources must contain a resource named Kubelet"
	// +kubebuilder:validation:XValidation:rule="self.exists(r, r.name == 'Etcd')",message="resources must contain a resource named Etcd"
	// +required
	Resources []PacemakerClusterResourceStatus `json:"resources,omitempty"`

	// fencingAgents contains the status of fencing agents that can fence this node.
	// Unlike resources (which are scheduled to run on this node), fencing agents are mapped
	// to the node they can fence (their target), not the node where monitoring operations run.
	// Each fencing agent entry includes a unique name, fencing type, target node, and health conditions.
	// A node is considered fence-capable if at least one fencing agent is healthy.
	// Expected to have 1 fencing agent per node, but up to 8 are supported for redundancy.
	// Names must be unique within this array.
	// +listType=map
	// +listMapKey=name
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=8
	// +kubebuilder:validation:XValidation:rule="self.all(x, self.exists_one(y, x.name == y.name))",message="fencing agent names must be unique"
	// +required
	FencingAgents []PacemakerClusterFencingAgentStatus `json:"fencingAgents,omitempty"`
}

// PacemakerClusterFencingAgentStatus represents the status of a fencing agent that can fence a node.
// Fencing agents are STONITH (Shoot The Other Node In The Head) devices used to isolate failed nodes.
// Unlike regular pacemaker resources, fencing agents are mapped to their target node (the node they
// can fence), not the node where their monitoring operations are scheduled.
type PacemakerClusterFencingAgentStatus struct {
	// conditions represent the observations of the fencing agent's current state.
	// Known condition types are: "Healthy", "InService", "Managed", "Enabled", "Operational",
	// "Active", "Started", "Schedulable".
	// The "Healthy" condition is an aggregate that tracks the overall health of the fencing agent.
	// The "InService" condition tracks whether the fencing agent is in service (not in maintenance mode).
	// The "Managed" condition tracks whether the fencing agent is managed by pacemaker.
	// The "Enabled" condition tracks whether the fencing agent is enabled.
	// The "Operational" condition tracks whether the fencing agent is operational (not failed).
	// The "Active" condition tracks whether the fencing agent is active (available to be used).
	// The "Started" condition tracks whether the fencing agent is started.
	// The "Schedulable" condition tracks whether the fencing agent is schedulable (not blocked).
	// Each of these conditions is required, so the array must contain at least 8 items.
	// +listType=map
	// +listMapKey=type
	// +kubebuilder:validation:MinItems=8
	// +kubebuilder:validation:MaxItems=16
	// +kubebuilder:validation:XValidation:rule="self.exists(c, c.type == 'Healthy')",message="conditions must contain a condition of type Healthy"
	// +kubebuilder:validation:XValidation:rule="self.exists(c, c.type == 'InService')",message="conditions must contain a condition of type InService"
	// +kubebuilder:validation:XValidation:rule="self.exists(c, c.type == 'Managed')",message="conditions must contain a condition of type Managed"
	// +kubebuilder:validation:XValidation:rule="self.exists(c, c.type == 'Enabled')",message="conditions must contain a condition of type Enabled"
	// +kubebuilder:validation:XValidation:rule="self.exists(c, c.type == 'Operational')",message="conditions must contain a condition of type Operational"
	// +kubebuilder:validation:XValidation:rule="self.exists(c, c.type == 'Active')",message="conditions must contain a condition of type Active"
	// +kubebuilder:validation:XValidation:rule="self.exists(c, c.type == 'Started')",message="conditions must contain a condition of type Started"
	// +kubebuilder:validation:XValidation:rule="self.exists(c, c.type == 'Schedulable')",message="conditions must contain a condition of type Schedulable"
	// +required
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// name is the unique identifier for this fencing agent (e.g., "master-0_redfish").
	// The name must be unique within the fencingAgents array for this node.
	// It may contain alphanumeric characters, dots, hyphens, and underscores.
	// Maximum length is 300 characters, providing headroom beyond the typical format of
	// <node_name>_<type> (253 for RFC 1123 node name + 1 underscore + type).
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=300
	// +kubebuilder:validation:XValidation:rule="self.matches('^[a-zA-Z0-9._-]+$')",message="name must contain only alphanumeric characters, dots, hyphens, and underscores"
	// +required
	Name string `json:"name,omitempty"`

	// method is the fencing method used by this agent.
	// Valid values are "Redfish" and "IPMI".
	// Redfish is a standard RESTful API for server management.
	// IPMI (Intelligent Platform Management Interface) is a hardware management interface.
	// +required
	Method FencingMethod `json:"method,omitempty"`
}

// PacemakerClusterResourceStatus represents the status of a pacemaker resource scheduled on a node.
// A pacemaker resource is a unit of work managed by pacemaker. In pacemaker terminology, resources are services or
// applications that pacemaker monitors, starts, stops, and moves between nodes to maintain high availability.
// For Two Node OpenShift with Fencing, we track two resources per node:
//   - Kubelet (the Kubernetes node agent and a prerequisite for etcd)
//   - Etcd (the distributed key-value store)
//
// Fencing agents are tracked separately in the fencingAgents field because they are mapped to
// their target node (the node they can fence), not the node where monitoring operations are scheduled.
type PacemakerClusterResourceStatus struct {
	// conditions represent the observations of the resource's current state.
	// Known condition types are: "Healthy", "InService", "Managed", "Enabled", "Operational",
	// "Active", "Started", "Schedulable".
	// The "Healthy" condition is an aggregate that tracks the overall health of the resource.
	// The "InService" condition tracks whether the resource is in service (not in maintenance mode).
	// The "Managed" condition tracks whether the resource is managed by pacemaker.
	// The "Enabled" condition tracks whether the resource is enabled.
	// The "Operational" condition tracks whether the resource is operational (not failed).
	// The "Active" condition tracks whether the resource is active (available to be used).
	// The "Started" condition tracks whether the resource is started.
	// The "Schedulable" condition tracks whether the resource is schedulable (not blocked).
	// Each of these conditions is required, so the array must contain at least 8 items.
	// +listType=map
	// +listMapKey=type
	// +kubebuilder:validation:MinItems=8
	// +kubebuilder:validation:MaxItems=16
	// +kubebuilder:validation:XValidation:rule="self.exists(c, c.type == 'Healthy')",message="conditions must contain a condition of type Healthy"
	// +kubebuilder:validation:XValidation:rule="self.exists(c, c.type == 'InService')",message="conditions must contain a condition of type InService"
	// +kubebuilder:validation:XValidation:rule="self.exists(c, c.type == 'Managed')",message="conditions must contain a condition of type Managed"
	// +kubebuilder:validation:XValidation:rule="self.exists(c, c.type == 'Enabled')",message="conditions must contain a condition of type Enabled"
	// +kubebuilder:validation:XValidation:rule="self.exists(c, c.type == 'Operational')",message="conditions must contain a condition of type Operational"
	// +kubebuilder:validation:XValidation:rule="self.exists(c, c.type == 'Active')",message="conditions must contain a condition of type Active"
	// +kubebuilder:validation:XValidation:rule="self.exists(c, c.type == 'Started')",message="conditions must contain a condition of type Started"
	// +kubebuilder:validation:XValidation:rule="self.exists(c, c.type == 'Schedulable')",message="conditions must contain a condition of type Schedulable"
	// +required
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// name is the name of the pacemaker resource.
	// Valid values are "Kubelet" and "Etcd".
	// The Kubelet resource is a prerequisite for etcd in Two Node OpenShift with Fencing deployments.
	// The Etcd resource may temporarily transition to stopped during pacemaker quorum-recovery operations.
	// Fencing agents are tracked separately in the node's fencingAgents field.
	// +required
	Name PacemakerClusterResourceName `json:"name,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// PacemakerClusterList contains a list of PacemakerCluster objects. PacemakerCluster is a cluster-scoped singleton
// resource; only one instance named "cluster" may exist. This list type exists only to satisfy Kubernetes API
// conventions.
//
// Compatibility level 4: No compatibility is provided, the API can change at any point for any reason. These capabilities should not be used by applications needing long term support.
// +openshift:compatibility-gen:level=4
type PacemakerClusterList struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard list's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ListMeta `json:"metadata,omitempty"`

	// items is a list of PacemakerCluster objects.
	Items []PacemakerCluster `json:"items"`
}
