package condition

const (
	// ManagementStateDegradedConditionType is true when the operator ManagementState is not "Managed"..
	// Possible reasons are Unmanaged, Removed or Unknown. Any of these cases means the operator is not actively managing the operand.
	// This condition is set to false when the ManagementState is set to back to "Managed".
	ManagementStateDegradedConditionType = "ManagementStateDegraded"

	// UnsupportedConfigOverridesUpgradeableConditionType is true when operator unsupported config overrides is changed.
	// When NoUnsupportedConfigOverrides reason is given it means there are no unsupported config overrides.
	// When UnsupportedConfigOverridesSet reason is given it means the unsupported config overrides are set, which might impact the ability
	// of operator to successfully upgrade its operand.
	UnsupportedConfigOverridesUpgradeableConditionType = "UnsupportedConfigOverridesUpgradeable"

	// MonitoringResourceControllerDegradedConditionType is true when the operator is unable to create or reconcile the ServiceMonitor
	// CR resource, which is required by monitoring operator to collect Prometheus data from the operator. When this condition is true and the ServiceMonitor
	// is already created, it won't have impact on collecting metrics. However, if the ServiceMonitor was not created, the metrics won't be available for
	// collection until this condition is set to false.
	// The condition is set to false automatically when the operator successfully synchronize the ServiceMonitor resource.
	MonitoringResourceControllerDegradedConditionType = "MonitoringResourceControllerDegraded"

	// BackingResourceControllerDegradedConditionType is true when the operator is unable to create or reconcile the resources needed
	// to successfully run the installer pods (installer CRB and SA). If these were already created, this condition is not fatal, however if the resources
	// were not created it means the installer pod creation will fail.
	// This condition is set to false when the operator can successfully synchronize installer SA and CRB.
	BackingResourceControllerDegradedConditionType = "BackingResourceControllerDegraded"

	// StaticPodsDegradedConditionType is true when the operator observe errors when installing the new revision static pods.
	// This condition report Error reason when the pods are terminated or not ready or waiting during which the operand quality of service is degraded.
	// This condition is set to False when the pods change state to running and are observed ready.
	StaticPodsDegradedConditionType = "StaticPodsDegraded"

	// StaticPodsAvailableConditionType is true when the static pod is available on at least one node.
	StaticPodsAvailableConditionType = "StaticPodsAvailable"

	// ConfigObservationDegradedConditionType is true when the operator failed to observe or process configuration change.
	// This is not transient condition and normally a correction or manual intervention is required on the config custom resource.
	ConfigObservationDegradedConditionType = "ConfigObservationDegraded"

	// ResourceSyncControllerDegradedConditionType is true when the operator failed to synchronize one or more secrets or config maps required
	// to run the operand. Operand ability to provide service might be affected by this condition.
	// This condition is set to false when the operator is able to create secrets and config maps.
	ResourceSyncControllerDegradedConditionType = "ResourceSyncControllerDegraded"

	// CertRotationDegradedConditionTypeFmt is true when the operator failed to properly rotate one or more certificates required by the operand.
	// The RotationError reason is given with message describing details of this failure. This condition can be fatal when ignored as the existing certificate(s)
	// validity can expire and without rotating/renewing them manual recovery might be required to fix the cluster.
	CertRotationDegradedConditionTypeFmt = "CertRotation_%s_Degraded"

	// InstallerControllerDegradedConditionType is true when the operator is not able to create new installer pods so the new revisions
	// cannot be rolled out. This might happen when one or more required secrets or config maps does not exists.
	// In case the missing secret or config map is available, this condition is automatically set to false.
	InstallerControllerDegradedConditionType = "InstallerControllerDegraded"

	// NodeInstallerDegradedConditionType is true when the operator is not able to create new installer pods because there are no schedulable nodes
	// available to run the installer pods.
	// The AllNodesAtLatestRevision reason is set when all master nodes are updated to the latest revision. It is false when some masters are pending revision.
	// ZeroNodesActive reason is set to True when no active master nodes are observed. Is set to False when there is at least one active master node.
	NodeInstallerDegradedConditionType = "NodeInstallerDegraded"

	// NodeInstallerProgressingConditionType is true when the operator is moving nodes to a new revision.
	NodeInstallerProgressingConditionType = "NodeInstallerProgressing"

	// RevisionControllerDegradedConditionType is true when the operator is not able to create new desired revision because an error occurred when
	// the operator attempted to created required resource(s) (secrets, configmaps, ...).
	// This condition mean no new revision will be created.
	RevisionControllerDegradedConditionType = "RevisionControllerDegraded"

	// NodeControllerDegradedConditionType is true when the operator observed a master node that is not ready.
	// Note that a node is not ready when its Condition.NodeReady wasn't set to true
	NodeControllerDegradedConditionType = "NodeControllerDegraded"
)
