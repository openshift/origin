package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	SubscriptionKind          = "Subscription"
	SubscriptionCRDAPIVersion = GroupName + "/" + GroupVersion
)

// SubscriptionState tracks when updates are available, installing, or service is up to date
type SubscriptionState string

const (
	SubscriptionStateNone             = ""
	SubscriptionStateFailed           = "UpgradeFailed"
	SubscriptionStateUpgradeAvailable = "UpgradeAvailable"
	SubscriptionStateUpgradePending   = "UpgradePending"
	SubscriptionStateAtLatest         = "AtLatestKnown"
)

const (
	SubscriptionReasonInvalidCatalog   ConditionReason = "InvalidCatalog"
	SubscriptionReasonUpgradeSucceeded ConditionReason = "UpgradeSucceeded"
)

// SubscriptionSpec defines an Application that can be installed
type SubscriptionSpec struct {
	CatalogSource          string              `json:"source"`
	CatalogSourceNamespace string              `json:"sourceNamespace"`
	Package                string              `json:"name"`
	Channel                string              `json:"channel,omitempty"`
	StartingCSV            string              `json:"startingCSV,omitempty"`
	InstallPlanApproval    Approval            `json:"installPlanApproval,omitempty"`
	Config                 *SubscriptionConfig `json:"config,omitempty"`
}

// SubscriptionConfig contains configuration specified for a subscription.
type SubscriptionConfig struct {
	// Selector is the label selector for pods to be configured.
	// Existing ReplicaSets whose pods are
	// selected by this will be the ones affected by this deployment.
	// It must match the pod template's labels.
	Selector *metav1.LabelSelector `json:"selector,omitempty"`

	// NodeSelector is a selector which must be true for the pod to fit on a node.
	// Selector which must match a node's labels for the pod to be scheduled on that node.
	// More info: https://kubernetes.io/docs/concepts/configuration/assign-pod-node/
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// Tolerations are the pod's tolerations.
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// Resources represents compute resources required by this container.
	// Immutable.
	// More info: https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// EnvFrom is a list of sources to populate environment variables in the container.
	// The keys defined within a source must be a C_IDENTIFIER. All invalid keys
	// will be reported as an event when the container is starting. When a key exists in multiple
	// sources, the value associated with the last source will take precedence.
	// Values defined by an Env with a duplicate key will take precedence.
	// Immutable.
	// +optional
	EnvFrom []corev1.EnvFromSource `json:"envFrom,omitempty"`
	// Env is a list of environment variables to set in the container.
	// Cannot be updated.
	// +patchMergeKey=name
	// +patchStrategy=merge
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty" patchMergeKey:"name" patchStrategy:"merge"`

	// List of Volumes to set in the podSpec.
	// +optional
	Volumes []corev1.Volume `json:"volumes,omitempty"`

	// List of VolumeMounts to set in the container.
	// +optional
	VolumeMounts []corev1.VolumeMount `json:"volumeMounts,omitempty"`

	// If specified, overrides the pod's scheduling constraints.
	// nil sub-attributes will *not* override the original values in the pod.spec for those sub-attributes.
	// Use empty object ({}) to erase original sub-attribute values.
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty" protobuf:"bytes,18,opt,name=affinity"`

	// Annotations is an unstructured key value map stored with each Deployment, Pod, APIService in the Operator.
	// Typically, annotations may be set by external tools to store and retrieve arbitrary metadata.
	// Use this field to pre-define annotations that OLM should add to each of the Subscription's
	// deployments, pods, and apiservices.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty" protobuf:"bytes,12,rep,name=annotations"`
}

// SubscriptionConditionType indicates an explicit state condition about a Subscription in "abnormal-true"
// polarity form (see https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties).
type SubscriptionConditionType string

const (
	// SubscriptionCatalogSourcesUnhealthy indicates that some or all of the CatalogSources to be used in resolution are unhealthy.
	SubscriptionCatalogSourcesUnhealthy SubscriptionConditionType = "CatalogSourcesUnhealthy"

	// SubscriptionInstallPlanMissing indicates that a Subscription's InstallPlan is missing.
	SubscriptionInstallPlanMissing SubscriptionConditionType = "InstallPlanMissing"

	// SubscriptionInstallPlanPending indicates that a Subscription's InstallPlan is pending installation.
	SubscriptionInstallPlanPending SubscriptionConditionType = "InstallPlanPending"

	// SubscriptionInstallPlanFailed indicates that the installation of a Subscription's InstallPlan has failed.
	SubscriptionInstallPlanFailed SubscriptionConditionType = "InstallPlanFailed"

	// SubscriptionResolutionFailed indicates that the dependency resolution in the namespace in which the subscription is created has failed
	SubscriptionResolutionFailed SubscriptionConditionType = "ResolutionFailed"

	// SubscriptionBundleUnpacking indicates that the unpack job is currently running
	SubscriptionBundleUnpacking SubscriptionConditionType = "BundleUnpacking"

	// SubscriptionBundleUnpackFailed indicates that the unpack job failed
	SubscriptionBundleUnpackFailed SubscriptionConditionType = "BundleUnpackFailed"

	// SubscriptionDeprecated is a roll-up condition which indicates that the Operator currently installed with this Subscription
	//has been deprecated. It will be present when any of the three deprecation types (Package, Channel, Bundle) are present.
	SubscriptionDeprecated SubscriptionConditionType = "Deprecated"

	// SubscriptionOperatorDeprecated indicates that the Package currently installed with this Subscription has been deprecated.
	SubscriptionPackageDeprecated SubscriptionConditionType = "PackageDeprecated"

	// SubscriptionOperatorDeprecated indicates that the Channel used with this Subscription has been deprecated.
	SubscriptionChannelDeprecated SubscriptionConditionType = "ChannelDeprecated"

	// SubscriptionOperatorDeprecated indicates that the Bundle currently installed with this Subscription has been deprecated.
	SubscriptionBundleDeprecated SubscriptionConditionType = "BundleDeprecated"
)

const (
	// NoCatalogSourcesFound is a reason string for Subscriptions with unhealthy CatalogSources due to none being available.
	NoCatalogSourcesFound = "NoCatalogSourcesFound"

	// AllCatalogSourcesHealthy is a reason string for Subscriptions that transitioned due to all CatalogSources being healthy.
	AllCatalogSourcesHealthy = "AllCatalogSourcesHealthy"

	// CatalogSourcesAdded is a reason string for Subscriptions that transitioned due to CatalogSources being added.
	CatalogSourcesAdded = "CatalogSourcesAdded"

	// CatalogSourcesUpdated is a reason string for Subscriptions that transitioned due to CatalogSource being updated.
	CatalogSourcesUpdated = "CatalogSourcesUpdated"

	// CatalogSourcesDeleted is a reason string for Subscriptions that transitioned due to CatalogSources being removed.
	CatalogSourcesDeleted = "CatalogSourcesDeleted"

	// UnhealthyCatalogSourceFound is a reason string for Subscriptions that transitioned because an unhealthy CatalogSource was found.
	UnhealthyCatalogSourceFound = "UnhealthyCatalogSourceFound"

	// ReferencedInstallPlanNotFound is a reason string for Subscriptions that transitioned due to a referenced InstallPlan not being found.
	ReferencedInstallPlanNotFound = "ReferencedInstallPlanNotFound"

	// InstallPlanNotYetReconciled is a reason string for Subscriptions that transitioned due to a referenced InstallPlan not being reconciled yet.
	InstallPlanNotYetReconciled = "InstallPlanNotYetReconciled"

	// InstallPlanFailed is a reason string for Subscriptions that transitioned due to a referenced InstallPlan failing without setting an explicit failure condition.
	InstallPlanFailed = "InstallPlanFailed"
)

// SubscriptionCondition represents the latest available observations of a Subscription's state.
type SubscriptionCondition struct {
	// Type is the type of Subscription condition.
	Type SubscriptionConditionType `json:"type" description:"type of Subscription condition"`

	// Status is the status of the condition, one of True, False, Unknown.
	Status corev1.ConditionStatus `json:"status" description:"status of the condition, one of True, False, Unknown"`

	// Reason is a one-word CamelCase reason for the condition's last transition.
	// +optional
	Reason string `json:"reason,omitempty" description:"one-word CamelCase reason for the condition's last transition"`

	// Message is a human-readable message indicating details about last transition.
	// +optional
	Message string `json:"message,omitempty" description:"human-readable message indicating details about last transition"`

	// LastHeartbeatTime is the last time we got an update on a given condition
	// +optional
	LastHeartbeatTime *metav1.Time `json:"lastHeartbeatTime,omitempty" description:"last time we got an update on a given condition"`

	// LastTransitionTime is the last time the condition transit from one status to another
	// +optional
	LastTransitionTime *metav1.Time `json:"lastTransitionTime,omitempty" description:"last time the condition transit from one status to another" hash:"ignore"`
}

// Equals returns true if a SubscriptionCondition equals the one given, false otherwise.
// Equality is determined by the equality of the type, status, reason, and message fields ONLY.
func (s SubscriptionCondition) Equals(condition SubscriptionCondition) bool {
	return s.Type == condition.Type && s.Status == condition.Status && s.Reason == condition.Reason && s.Message == condition.Message
}

type SubscriptionStatus struct {
	// CurrentCSV is the CSV the Subscription is progressing to.
	// +optional
	CurrentCSV string `json:"currentCSV,omitempty"`

	// InstalledCSV is the CSV currently installed by the Subscription.
	// +optional
	InstalledCSV string `json:"installedCSV,omitempty"`

	// Install is a reference to the latest InstallPlan generated for the Subscription.
	// DEPRECATED: InstallPlanRef
	// +optional
	Install *InstallPlanReference `json:"installplan,omitempty"`

	// State represents the current state of the Subscription
	// +optional
	State SubscriptionState `json:"state,omitempty"`

	// Reason is the reason the Subscription was transitioned to its current state.
	// +optional
	Reason ConditionReason `json:"reason,omitempty"`

	// InstallPlanGeneration is the current generation of the installplan
	// +optional
	InstallPlanGeneration int `json:"installPlanGeneration,omitempty"`

	// InstallPlanRef is a reference to the latest InstallPlan that contains the Subscription's current CSV.
	// +optional
	InstallPlanRef *corev1.ObjectReference `json:"installPlanRef,omitempty"`

	// CatalogHealth contains the Subscription's view of its relevant CatalogSources' status.
	// It is used to determine SubscriptionStatusConditions related to CatalogSources.
	// +optional
	CatalogHealth []SubscriptionCatalogHealth `json:"catalogHealth,omitempty"`

	// Conditions is a list of the latest available observations about a Subscription's current state.
	// +optional
	Conditions []SubscriptionCondition `json:"conditions,omitempty" hash:"set"`

	// LastUpdated represents the last time that the Subscription status was updated.
	LastUpdated metav1.Time `json:"lastUpdated"`
}

// GetCondition returns the SubscriptionCondition of the given type if it exists in the SubscriptionStatus' Conditions.
// Returns a condition of the given type with a ConditionStatus of "Unknown" if not found.
func (s SubscriptionStatus) GetCondition(conditionType SubscriptionConditionType) SubscriptionCondition {
	for _, cond := range s.Conditions {
		if cond.Type == conditionType {
			return cond
		}
	}

	return SubscriptionCondition{
		Type:   conditionType,
		Status: corev1.ConditionUnknown,
	}
}

// SetCondition sets the given SubscriptionCondition in the SubscriptionStatus' Conditions.
func (s *SubscriptionStatus) SetCondition(condition SubscriptionCondition) {
	for i, cond := range s.Conditions {
		if cond.Type == condition.Type {
			s.Conditions[i] = condition
			return
		}
	}

	s.Conditions = append(s.Conditions, condition)
}

// RemoveConditions removes any conditions of the given types from the SubscriptionStatus' Conditions.
func (s *SubscriptionStatus) RemoveConditions(remove ...SubscriptionConditionType) {
	exclusions := map[SubscriptionConditionType]struct{}{}
	for _, r := range remove {
		exclusions[r] = struct{}{}
	}

	var filtered []SubscriptionCondition
	for _, cond := range s.Conditions {
		if _, ok := exclusions[cond.Type]; ok {
			// Skip excluded condition types
			continue
		}
		filtered = append(filtered, cond)
	}

	s.Conditions = filtered
}

type InstallPlanReference struct {
	APIVersion string    `json:"apiVersion"`
	Kind       string    `json:"kind"`
	Name       string    `json:"name"`
	UID        types.UID `json:"uuid"`
}

// SubscriptionCatalogHealth describes the health of a CatalogSource the Subscription knows about.
type SubscriptionCatalogHealth struct {
	// CatalogSourceRef is a reference to a CatalogSource.
	CatalogSourceRef *corev1.ObjectReference `json:"catalogSourceRef"`

	// LastUpdated represents the last time that the CatalogSourceHealth changed
	LastUpdated *metav1.Time `json:"lastUpdated"`

	// Healthy is true if the CatalogSource is healthy; false otherwise.
	Healthy bool `json:"healthy"`
}

// Equals returns true if a SubscriptionCatalogHealth equals the one given, false otherwise.
// Equality is based SOLEY on health and UID.
func (s SubscriptionCatalogHealth) Equals(health SubscriptionCatalogHealth) bool {
	return s.Healthy == health.Healthy && s.CatalogSourceRef.UID == health.CatalogSourceRef.UID
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient
// +kubebuilder:resource:shortName={sub, subs},categories=olm
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Package",type=string,JSONPath=`.spec.name`,description="The package subscribed to"
// +kubebuilder:printcolumn:name="Source",type=string,JSONPath=`.spec.source`,description="The catalog source for the specified package"
// +kubebuilder:printcolumn:name="Channel",type=string,JSONPath=`.spec.channel`,description="The channel of updates to subscribe to"

// Subscription keeps operators up to date by tracking changes to Catalogs.
type Subscription struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec *SubscriptionSpec `json:"spec"`
	// +optional
	Status SubscriptionStatus `json:"status"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SubscriptionList is a list of Subscription resources.
type SubscriptionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Subscription `json:"items"`
}

// GetInstallPlanApproval gets the configured install plan approval or the default
func (s *Subscription) GetInstallPlanApproval() Approval {
	if s.Spec.InstallPlanApproval == ApprovalManual {
		return ApprovalManual
	}
	return ApprovalAutomatic
}

// NewInstallPlanReference returns an InstallPlanReference for the given ObjectReference.
func NewInstallPlanReference(ref *corev1.ObjectReference) *InstallPlanReference {
	return &InstallPlanReference{
		APIVersion: ref.APIVersion,
		Kind:       ref.Kind,
		Name:       ref.Name,
		UID:        ref.UID,
	}
}
