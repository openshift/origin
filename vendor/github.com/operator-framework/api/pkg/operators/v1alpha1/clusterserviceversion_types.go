package v1alpha1

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/operator-framework/api/pkg/lib/version"
)

const (
	ClusterServiceVersionAPIVersion     = GroupName + "/" + GroupVersion
	ClusterServiceVersionKind           = "ClusterServiceVersion"
	OperatorGroupNamespaceAnnotationKey = "olm.operatorNamespace"
	InstallStrategyNameDeployment       = "deployment"
	SkipRangeAnnotationKey              = "olm.skipRange"
)

// InstallModeType is a supported type of install mode for CSV installation
type InstallModeType string

const (
	// InstallModeTypeOwnNamespace indicates that the operator can be a member of an `OperatorGroup` that selects its own namespace.
	InstallModeTypeOwnNamespace InstallModeType = "OwnNamespace"
	// InstallModeTypeSingleNamespace indicates that the operator can be a member of an `OperatorGroup` that selects one namespace.
	InstallModeTypeSingleNamespace InstallModeType = "SingleNamespace"
	// InstallModeTypeMultiNamespace indicates that the operator can be a member of an `OperatorGroup` that selects more than one namespace.
	InstallModeTypeMultiNamespace InstallModeType = "MultiNamespace"
	// InstallModeTypeAllNamespaces indicates that the operator can be a member of an `OperatorGroup` that selects all namespaces (target namespace set is the empty string "").
	InstallModeTypeAllNamespaces InstallModeType = "AllNamespaces"
)

// InstallMode associates an InstallModeType with a flag representing if the CSV supports it
// +k8s:openapi-gen=true
type InstallMode struct {
	Type      InstallModeType `json:"type"`
	Supported bool            `json:"supported"`
}

// InstallModeSet is a mapping of unique InstallModeTypes to whether they are supported.
type InstallModeSet map[InstallModeType]bool

// NamedInstallStrategy represents the block of an ClusterServiceVersion resource
// where the install strategy is specified.
// +k8s:openapi-gen=true
type NamedInstallStrategy struct {
	StrategyName string                    `json:"strategy"`
	StrategySpec StrategyDetailsDeployment `json:"spec,omitempty"`
}

// StrategyDeploymentPermissions describe the rbac rules and service account needed by the install strategy
// +k8s:openapi-gen=true
type StrategyDeploymentPermissions struct {
	ServiceAccountName string            `json:"serviceAccountName"`
	Rules              []rbac.PolicyRule `json:"rules"`
}

// StrategyDeploymentSpec contains the name, spec and labels for the deployment ALM should create
// +k8s:openapi-gen=true
type StrategyDeploymentSpec struct {
	Name  string                `json:"name"`
	Spec  appsv1.DeploymentSpec `json:"spec"`
	Label labels.Set            `json:"label,omitempty"`
}

// StrategyDetailsDeployment represents the parsed details of a Deployment
// InstallStrategy.
// +k8s:openapi-gen=true
type StrategyDetailsDeployment struct {
	DeploymentSpecs    []StrategyDeploymentSpec        `json:"deployments"`
	Permissions        []StrategyDeploymentPermissions `json:"permissions,omitempty"`
	ClusterPermissions []StrategyDeploymentPermissions `json:"clusterPermissions,omitempty"`
}

func (d *StrategyDetailsDeployment) GetStrategyName() string {
	return InstallStrategyNameDeployment
}

// StatusDescriptor describes a field in a status block of a CRD so that OLM can consume it
// +k8s:openapi-gen=true
type StatusDescriptor struct {
	Path         string          `json:"path"`
	DisplayName  string          `json:"displayName,omitempty"`
	Description  string          `json:"description,omitempty"`
	XDescriptors []string        `json:"x-descriptors,omitempty"`
	Value        json.RawMessage `json:"value,omitempty"`
}

// SpecDescriptor describes a field in a spec block of a CRD so that OLM can consume it
// +k8s:openapi-gen=true
type SpecDescriptor struct {
	Path         string          `json:"path"`
	DisplayName  string          `json:"displayName,omitempty"`
	Description  string          `json:"description,omitempty"`
	XDescriptors []string        `json:"x-descriptors,omitempty"`
	Value        json.RawMessage `json:"value,omitempty"`
}

// ActionDescriptor describes a declarative action that can be performed on a custom resource instance
// +k8s:openapi-gen=true
type ActionDescriptor struct {
	Path         string          `json:"path"`
	DisplayName  string          `json:"displayName,omitempty"`
	Description  string          `json:"description,omitempty"`
	XDescriptors []string        `json:"x-descriptors,omitempty"`
	Value        json.RawMessage `json:"value,omitempty"`
}

// CRDDescription provides details to OLM about the CRDs
// +k8s:openapi-gen=true
type CRDDescription struct {
	Name              string                 `json:"name"`
	Version           string                 `json:"version"`
	Kind              string                 `json:"kind"`
	DisplayName       string                 `json:"displayName,omitempty"`
	Description       string                 `json:"description,omitempty"`
	Resources         []APIResourceReference `json:"resources,omitempty"`
	StatusDescriptors []StatusDescriptor     `json:"statusDescriptors,omitempty"`
	SpecDescriptors   []SpecDescriptor       `json:"specDescriptors,omitempty"`
	ActionDescriptor  []ActionDescriptor     `json:"actionDescriptors,omitempty"`
}

// APIServiceDescription provides details to OLM about apis provided via aggregation
// +k8s:openapi-gen=true
type APIServiceDescription struct {
	Name              string                 `json:"name"`
	Group             string                 `json:"group"`
	Version           string                 `json:"version"`
	Kind              string                 `json:"kind"`
	DeploymentName    string                 `json:"deploymentName,omitempty"`
	ContainerPort     int32                  `json:"containerPort,omitempty"`
	DisplayName       string                 `json:"displayName,omitempty"`
	Description       string                 `json:"description,omitempty"`
	Resources         []APIResourceReference `json:"resources,omitempty"`
	StatusDescriptors []StatusDescriptor     `json:"statusDescriptors,omitempty"`
	SpecDescriptors   []SpecDescriptor       `json:"specDescriptors,omitempty"`
	ActionDescriptor  []ActionDescriptor     `json:"actionDescriptors,omitempty"`
}

// APIResourceReference is a reference to a Kubernetes resource type that the referrer utilizes.
// +k8s:openapi-gen=true
type APIResourceReference struct {
	// Plural name of the referenced resource type (CustomResourceDefinition.Spec.Names[].Plural). Empty string if the referenced resource type is not a custom resource.
	Name string `json:"name"`
	// Kind of the referenced resource type.
	Kind string `json:"kind"`
	// API Version of the referenced resource type.
	Version string `json:"version"`
}

// GetName returns the name of an APIService as derived from its group and version.
func (d APIServiceDescription) GetName() string {
	return fmt.Sprintf("%s.%s", d.Version, d.Group)
}

// WebhookAdmissionType is the type of admission webhooks supported by OLM
type WebhookAdmissionType string

const (
	// ValidatingAdmissionWebhook is for validating admission webhooks
	ValidatingAdmissionWebhook WebhookAdmissionType = "ValidatingAdmissionWebhook"
	// MutatingAdmissionWebhook is for mutating admission webhooks
	MutatingAdmissionWebhook WebhookAdmissionType = "MutatingAdmissionWebhook"
	// ConversionWebhook is for conversion webhooks
	ConversionWebhook WebhookAdmissionType = "ConversionWebhook"
)

// WebhookDescription provides details to OLM about required webhooks
// +k8s:openapi-gen=true
type WebhookDescription struct {
	GenerateName string `json:"generateName"`
	// +kubebuilder:validation:Enum=ValidatingAdmissionWebhook;MutatingAdmissionWebhook;ConversionWebhook
	Type           WebhookAdmissionType `json:"type"`
	DeploymentName string               `json:"deploymentName,omitempty"`
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=443
	ContainerPort           int32                                           `json:"containerPort,omitempty"`
	TargetPort              *intstr.IntOrString                             `json:"targetPort,omitempty"`
	Rules                   []admissionregistrationv1.RuleWithOperations    `json:"rules,omitempty"`
	FailurePolicy           *admissionregistrationv1.FailurePolicyType      `json:"failurePolicy,omitempty"`
	MatchPolicy             *admissionregistrationv1.MatchPolicyType        `json:"matchPolicy,omitempty"`
	ObjectSelector          *metav1.LabelSelector                           `json:"objectSelector,omitempty"`
	SideEffects             *admissionregistrationv1.SideEffectClass        `json:"sideEffects"`
	TimeoutSeconds          *int32                                          `json:"timeoutSeconds,omitempty"`
	AdmissionReviewVersions []string                                        `json:"admissionReviewVersions"`
	ReinvocationPolicy      *admissionregistrationv1.ReinvocationPolicyType `json:"reinvocationPolicy,omitempty"`
	WebhookPath             *string                                         `json:"webhookPath,omitempty"`
	ConversionCRDs          []string                                        `json:"conversionCRDs,omitempty"`
}

// GetValidatingWebhook returns a ValidatingWebhook generated from the WebhookDescription
func (w *WebhookDescription) GetValidatingWebhook(namespace string, namespaceSelector *metav1.LabelSelector, caBundle []byte) admissionregistrationv1.ValidatingWebhook {
	return admissionregistrationv1.ValidatingWebhook{
		Name:                    w.GenerateName,
		Rules:                   w.Rules,
		FailurePolicy:           w.FailurePolicy,
		MatchPolicy:             w.MatchPolicy,
		NamespaceSelector:       namespaceSelector,
		ObjectSelector:          w.ObjectSelector,
		SideEffects:             w.SideEffects,
		TimeoutSeconds:          w.TimeoutSeconds,
		AdmissionReviewVersions: w.AdmissionReviewVersions,
		ClientConfig: admissionregistrationv1.WebhookClientConfig{
			Service: &admissionregistrationv1.ServiceReference{
				Name:      w.DomainName() + "-service",
				Namespace: namespace,
				Path:      w.WebhookPath,
				Port:      &w.ContainerPort,
			},
			CABundle: caBundle,
		},
	}
}

// GetMutatingWebhook returns a MutatingWebhook generated from the WebhookDescription
func (w *WebhookDescription) GetMutatingWebhook(namespace string, namespaceSelector *metav1.LabelSelector, caBundle []byte) admissionregistrationv1.MutatingWebhook {
	return admissionregistrationv1.MutatingWebhook{
		Name:                    w.GenerateName,
		Rules:                   w.Rules,
		FailurePolicy:           w.FailurePolicy,
		MatchPolicy:             w.MatchPolicy,
		NamespaceSelector:       namespaceSelector,
		ObjectSelector:          w.ObjectSelector,
		SideEffects:             w.SideEffects,
		TimeoutSeconds:          w.TimeoutSeconds,
		AdmissionReviewVersions: w.AdmissionReviewVersions,
		ClientConfig: admissionregistrationv1.WebhookClientConfig{
			Service: &admissionregistrationv1.ServiceReference{
				Name:      w.DomainName() + "-service",
				Namespace: namespace,
				Path:      w.WebhookPath,
				Port:      &w.ContainerPort,
			},
			CABundle: caBundle,
		},
		ReinvocationPolicy: w.ReinvocationPolicy,
	}
}

// DomainName returns the result of replacing all periods in the given Webhook name with hyphens
func (w *WebhookDescription) DomainName() string {
	// Replace all '.'s with "-"s to convert to a DNS-1035 label
	return strings.Replace(w.DeploymentName, ".", "-", -1)
}

// CustomResourceDefinitions declares all of the CRDs managed or required by
// an operator being ran by ClusterServiceVersion.
//
// If the CRD is present in the Owned list, it is implicitly required.
// +k8s:openapi-gen=true
type CustomResourceDefinitions struct {
	Owned    []CRDDescription `json:"owned,omitempty"`
	Required []CRDDescription `json:"required,omitempty"`
}

// APIServiceDefinitions declares all of the extension apis managed or required by
// an operator being ran by ClusterServiceVersion.
// +k8s:openapi-gen=true
type APIServiceDefinitions struct {
	Owned    []APIServiceDescription `json:"owned,omitempty"`
	Required []APIServiceDescription `json:"required,omitempty"`
}

// ClusterServiceVersionSpec declarations tell OLM how to install an operator
// that can manage apps for a given version.
// +k8s:openapi-gen=true
type ClusterServiceVersionSpec struct {
	InstallStrategy           NamedInstallStrategy      `json:"install"`
	Version                   version.OperatorVersion   `json:"version,omitempty"`
	Maturity                  string                    `json:"maturity,omitempty"`
	CustomResourceDefinitions CustomResourceDefinitions `json:"customresourcedefinitions,omitempty"`
	APIServiceDefinitions     APIServiceDefinitions     `json:"apiservicedefinitions,omitempty"`
	WebhookDefinitions        []WebhookDescription      `json:"webhookdefinitions,omitempty"`
	NativeAPIs                []metav1.GroupVersionKind `json:"nativeAPIs,omitempty"`
	MinKubeVersion            string                    `json:"minKubeVersion,omitempty"`

	// The name of the operator in display format.
	DisplayName string `json:"displayName"`

	// Description of the operator. Can include the features, limitations or use-cases of the
	// operator.
	// +optional
	Description string `json:"description,omitempty"`

	// A list of keywords describing the operator.
	// +optional
	Keywords []string `json:"keywords,omitempty"`

	// A list of organizational entities maintaining the operator.
	// +optional
	Maintainers []Maintainer `json:"maintainers,omitempty"`

	// The publishing entity behind the operator.
	// +optional
	Provider AppLink `json:"provider,omitempty"`

	// A list of links related to the operator.
	// +optional
	Links []AppLink `json:"links,omitempty"`

	// The icon for this operator.
	// +optional
	Icon []Icon `json:"icon,omitempty"`

	// InstallModes specify supported installation types
	// +optional
	InstallModes []InstallMode `json:"installModes,omitempty"`

	// The name of a CSV this one replaces. Should match the `metadata.Name` field of the old CSV.
	// +optional
	Replaces string `json:"replaces,omitempty"`

	// Map of string keys and values that can be used to organize and categorize
	// (scope and select) objects.
	// +optional
	Labels map[string]string `json:"labels,omitempty" protobuf:"bytes,11,rep,name=labels"`

	// Annotations is an unstructured key value map stored with a resource that may be
	// set by external tools to store and retrieve arbitrary metadata.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty" protobuf:"bytes,12,rep,name=annotations"`

	// Label selector for related resources.
	// +optional
	Selector *metav1.LabelSelector `json:"selector,omitempty" protobuf:"bytes,2,opt,name=selector"`

	// Cleanup specifies the cleanup behaviour when the CSV gets deleted
	// +optional
	Cleanup CleanupSpec `json:"cleanup,omitempty"`

	// The name(s) of one or more CSV(s) that should be skipped in the upgrade graph.
	// Should match the `metadata.Name` field of the CSV that should be skipped.
	// This field is only used during catalog creation and plays no part in cluster runtime.
	// +optional
	Skips []string `json:"skips,omitempty"`

	// List any related images, or other container images that your Operator might require to perform their functions.
	// This list should also include operand images as well. All image references should be specified by
	// digest (SHA) and not by tag. This field is only used during catalog creation and plays no part in cluster runtime.
	// +optional
	RelatedImages []RelatedImage `json:"relatedImages,omitempty"`
}

// +k8s:openapi-gen=true
type CleanupSpec struct {
	Enabled bool `json:"enabled"`
}

// +k8s:openapi-gen=true
type Maintainer struct {
	Name  string `json:"name,omitempty"`
	Email string `json:"email,omitempty"`
}

// +k8s:openapi-gen=true
type AppLink struct {
	Name string `json:"name,omitempty"`
	URL  string `json:"url,omitempty"`
}

// +k8s:openapi-gen=true
type Icon struct {
	Data      string `json:"base64data"`
	MediaType string `json:"mediatype"`
}

// +k8s:openapi-gen=true
type RelatedImage struct {
	Name  string `json:"name"`
	Image string `json:"image"`
}

// ClusterServiceVersionPhase is a label for the condition of a ClusterServiceVersion at the current time.
type ClusterServiceVersionPhase string

// These are the valid phases of ClusterServiceVersion
const (
	CSVPhaseNone = ""
	// CSVPhasePending means the csv has been accepted by the system, but the install strategy has not been attempted.
	// This is likely because there are unmet requirements.
	CSVPhasePending ClusterServiceVersionPhase = "Pending"
	// CSVPhaseInstallReady means that the requirements are met but the install strategy has not been run.
	CSVPhaseInstallReady ClusterServiceVersionPhase = "InstallReady"
	// CSVPhaseInstalling means that the install strategy has been initiated but not completed.
	CSVPhaseInstalling ClusterServiceVersionPhase = "Installing"
	// CSVPhaseSucceeded means that the resources in the CSV were created successfully.
	CSVPhaseSucceeded ClusterServiceVersionPhase = "Succeeded"
	// CSVPhaseFailed means that the install strategy could not be successfully completed.
	CSVPhaseFailed ClusterServiceVersionPhase = "Failed"
	// CSVPhaseUnknown means that for some reason the state of the csv could not be obtained.
	CSVPhaseUnknown ClusterServiceVersionPhase = "Unknown"
	// CSVPhaseReplacing means that a newer CSV has been created and the csv's resources will be transitioned to a new owner.
	CSVPhaseReplacing ClusterServiceVersionPhase = "Replacing"
	// CSVPhaseDeleting means that a CSV has been replaced by a new one and will be checked for safety before being deleted
	CSVPhaseDeleting ClusterServiceVersionPhase = "Deleting"
	// CSVPhaseAny matches all other phases in CSV queries
	CSVPhaseAny ClusterServiceVersionPhase = ""
)

// ConditionReason is a camelcased reason for the state transition
type ConditionReason string

const (
	CSVReasonRequirementsUnknown                         ConditionReason = "RequirementsUnknown"
	CSVReasonRequirementsNotMet                          ConditionReason = "RequirementsNotMet"
	CSVReasonRequirementsMet                             ConditionReason = "AllRequirementsMet"
	CSVReasonOwnerConflict                               ConditionReason = "OwnerConflict"
	CSVReasonComponentFailed                             ConditionReason = "InstallComponentFailed"
	CSVReasonComponentFailedNoRetry                      ConditionReason = "InstallComponentFailedNoRetry"
	CSVReasonInvalidStrategy                             ConditionReason = "InvalidInstallStrategy"
	CSVReasonWaiting                                     ConditionReason = "InstallWaiting"
	CSVReasonInstallSuccessful                           ConditionReason = "InstallSucceeded"
	CSVReasonInstallCheckFailed                          ConditionReason = "InstallCheckFailed"
	CSVReasonComponentUnhealthy                          ConditionReason = "ComponentUnhealthy"
	CSVReasonBeingReplaced                               ConditionReason = "BeingReplaced"
	CSVReasonReplaced                                    ConditionReason = "Replaced"
	CSVReasonNeedsReinstall                              ConditionReason = "NeedsReinstall"
	CSVReasonNeedsCertRotation                           ConditionReason = "NeedsCertRotation"
	CSVReasonAPIServiceResourceIssue                     ConditionReason = "APIServiceResourceIssue"
	CSVReasonAPIServiceResourcesNeedReinstall            ConditionReason = "APIServiceResourcesNeedReinstall"
	CSVReasonAPIServiceInstallFailed                     ConditionReason = "APIServiceInstallFailed"
	CSVReasonCopied                                      ConditionReason = "Copied"
	CSVReasonInvalidInstallModes                         ConditionReason = "InvalidInstallModes"
	CSVReasonNoTargetNamespaces                          ConditionReason = "NoTargetNamespaces"
	CSVReasonUnsupportedOperatorGroup                    ConditionReason = "UnsupportedOperatorGroup"
	CSVReasonNoOperatorGroup                             ConditionReason = "NoOperatorGroup"
	CSVReasonTooManyOperatorGroups                       ConditionReason = "TooManyOperatorGroups"
	CSVReasonInterOperatorGroupOwnerConflict             ConditionReason = "InterOperatorGroupOwnerConflict"
	CSVReasonCannotModifyStaticOperatorGroupProvidedAPIs ConditionReason = "CannotModifyStaticOperatorGroupProvidedAPIs"
	CSVReasonDetectedClusterChange                       ConditionReason = "DetectedClusterChange"
	CSVReasonInvalidWebhookDescription                   ConditionReason = "InvalidWebhookDescription"
	CSVReasonOperatorConditionNotUpgradeable             ConditionReason = "OperatorConditionNotUpgradeable"
	CSVReasonWaitingForCleanupToComplete                 ConditionReason = "WaitingOnCleanup"
)

// HasCaResources returns true if the CSV has owned APIServices or Webhooks.
func (c *ClusterServiceVersion) HasCAResources() bool {
	// Return early if there are no owned APIServices
	if len(c.Spec.APIServiceDefinitions.Owned)+len(c.Spec.WebhookDefinitions) == 0 {
		return false
	}
	return true
}

// Conditions appear in the status as a record of state transitions on the ClusterServiceVersion
// +k8s:openapi-gen=true
type ClusterServiceVersionCondition struct {
	// Condition of the ClusterServiceVersion
	Phase ClusterServiceVersionPhase `json:"phase,omitempty"`
	// A human readable message indicating details about why the ClusterServiceVersion is in this condition.
	// +optional
	Message string `json:"message,omitempty"`
	// A brief CamelCase message indicating details about why the ClusterServiceVersion is in this state.
	// e.g. 'RequirementsNotMet'
	// +optional
	Reason ConditionReason `json:"reason,omitempty"`
	// Last time we updated the status
	// +optional
	LastUpdateTime *metav1.Time `json:"lastUpdateTime,omitempty"`
	// Last time the status transitioned from one status to another.
	// +optional
	LastTransitionTime *metav1.Time `json:"lastTransitionTime,omitempty"`
}

// OwnsCRD determines whether the current CSV owns a particular CRD.
func (csv ClusterServiceVersion) OwnsCRD(name string) bool {
	for _, desc := range csv.Spec.CustomResourceDefinitions.Owned {
		if desc.Name == name {
			return true
		}
	}

	return false
}

// OwnsAPIService determines whether the current CSV owns a particular APIService.
func (csv ClusterServiceVersion) OwnsAPIService(name string) bool {
	for _, desc := range csv.Spec.APIServiceDefinitions.Owned {
		apiServiceName := fmt.Sprintf("%s.%s", desc.Version, desc.Group)
		if apiServiceName == name {
			return true
		}
	}

	return false
}

// StatusReason is a camelcased reason for the status of a RequirementStatus or DependentStatus
type StatusReason string

const (
	RequirementStatusReasonPresent             StatusReason = "Present"
	RequirementStatusReasonNotPresent          StatusReason = "NotPresent"
	RequirementStatusReasonPresentNotSatisfied StatusReason = "PresentNotSatisfied"
	// The CRD is present but the Established condition is False (not available)
	RequirementStatusReasonNotAvailable StatusReason = "PresentNotAvailable"
	DependentStatusReasonSatisfied      StatusReason = "Satisfied"
	DependentStatusReasonNotSatisfied   StatusReason = "NotSatisfied"
)

// DependentStatus is the status for a dependent requirement (to prevent infinite nesting)
// +k8s:openapi-gen=true
type DependentStatus struct {
	Group   string       `json:"group"`
	Version string       `json:"version"`
	Kind    string       `json:"kind"`
	Status  StatusReason `json:"status"`
	UUID    string       `json:"uuid,omitempty"`
	Message string       `json:"message,omitempty"`
}

// +k8s:openapi-gen=true
type RequirementStatus struct {
	Group      string            `json:"group"`
	Version    string            `json:"version"`
	Kind       string            `json:"kind"`
	Name       string            `json:"name"`
	Status     StatusReason      `json:"status"`
	Message    string            `json:"message"`
	UUID       string            `json:"uuid,omitempty"`
	Dependents []DependentStatus `json:"dependents,omitempty"`
}

// ClusterServiceVersionStatus represents information about the status of a CSV. Status may trail the actual
// state of a system.
// +k8s:openapi-gen=true
type ClusterServiceVersionStatus struct {
	// Current condition of the ClusterServiceVersion
	Phase ClusterServiceVersionPhase `json:"phase,omitempty"`
	// A human readable message indicating details about why the ClusterServiceVersion is in this condition.
	// +optional
	Message string `json:"message,omitempty"`
	// A brief CamelCase message indicating details about why the ClusterServiceVersion is in this state.
	// e.g. 'RequirementsNotMet'
	// +optional
	Reason ConditionReason `json:"reason,omitempty"`
	// Last time we updated the status
	// +optional
	LastUpdateTime *metav1.Time `json:"lastUpdateTime,omitempty"`
	// Last time the status transitioned from one status to another.
	// +optional
	LastTransitionTime *metav1.Time `json:"lastTransitionTime,omitempty"`
	// List of conditions, a history of state transitions
	Conditions []ClusterServiceVersionCondition `json:"conditions,omitempty"`
	// The status of each requirement for this CSV
	RequirementStatus []RequirementStatus `json:"requirementStatus,omitempty"`
	// Last time the owned APIService certs were updated
	// +optional
	CertsLastUpdated *metav1.Time `json:"certsLastUpdated,omitempty"`
	// Time the owned APIService certs will rotate next
	// +optional
	CertsRotateAt *metav1.Time `json:"certsRotateAt,omitempty"`
	// CleanupStatus represents information about the status of cleanup while a CSV is pending deletion
	// +optional
	Cleanup CleanupStatus `json:"cleanup,omitempty"`
}

// CleanupStatus represents information about the status of cleanup while a CSV is pending deletion
// +k8s:openapi-gen=true
type CleanupStatus struct {
	// PendingDeletion is the list of custom resource objects that are pending deletion and blocked on finalizers.
	// This indicates the progress of cleanup that is blocking CSV deletion or operator uninstall.
	// +optional
	PendingDeletion []ResourceList `json:"pendingDeletion,omitempty"`
}

// ResourceList represents a list of resources which are of the same Group/Kind
// +k8s:openapi-gen=true
type ResourceList struct {
	Group     string             `json:"group"`
	Kind      string             `json:"kind"`
	Instances []ResourceInstance `json:"instances"`
}

// +k8s:openapi-gen=true
type ResourceInstance struct {
	Name string `json:"name"`
	// Namespace can be empty for cluster-scoped resources
	Namespace string `json:"namespace,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient
// +kubebuilder:storageversion
// +kubebuilder:resource:shortName={csv, csvs},categories=olm
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Display",type=string,JSONPath=`.spec.displayName`,description="The name of the CSV"
// +kubebuilder:printcolumn:name="Version",type=string,JSONPath=`.spec.version`,description="The version of the CSV"
// +kubebuilder:printcolumn:name="Replaces",type=string,JSONPath=`.spec.replaces`,description="The name of a CSV that this one replaces"
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`

// ClusterServiceVersion is a Custom Resource of type `ClusterServiceVersionSpec`.
type ClusterServiceVersion struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec ClusterServiceVersionSpec `json:"spec"`
	// +optional
	Status ClusterServiceVersionStatus `json:"status"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterServiceVersionList represents a list of ClusterServiceVersions.
type ClusterServiceVersionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ClusterServiceVersion `json:"items"`
}

// GetAllCRDDescriptions returns a deduplicated set of CRDDescriptions that is
// the union of the owned and required CRDDescriptions.
//
// Descriptions with the same name prefer the value in Owned.
// Descriptions are returned in alphabetical order.
func (csv ClusterServiceVersion) GetAllCRDDescriptions() []CRDDescription {
	set := make(map[string]CRDDescription)
	for _, required := range csv.Spec.CustomResourceDefinitions.Required {
		set[required.Name] = required
	}

	for _, owned := range csv.Spec.CustomResourceDefinitions.Owned {
		set[owned.Name] = owned
	}

	keys := make([]string, 0)
	for key := range set {
		keys = append(keys, key)
	}
	sort.StringSlice(keys).Sort()

	descs := make([]CRDDescription, 0)
	for _, key := range keys {
		descs = append(descs, set[key])
	}

	return descs
}

// GetAllAPIServiceDescriptions returns a deduplicated set of APIServiceDescriptions that is
// the union of the owned and required APIServiceDescriptions.
//
// Descriptions with the same name prefer the value in Owned.
// Descriptions are returned in alphabetical order.
func (csv ClusterServiceVersion) GetAllAPIServiceDescriptions() []APIServiceDescription {
	set := make(map[string]APIServiceDescription)
	for _, required := range csv.Spec.APIServiceDefinitions.Required {
		name := fmt.Sprintf("%s.%s", required.Version, required.Group)
		set[name] = required
	}

	for _, owned := range csv.Spec.APIServiceDefinitions.Owned {
		name := fmt.Sprintf("%s.%s", owned.Version, owned.Group)
		set[name] = owned
	}

	keys := make([]string, 0)
	for key := range set {
		keys = append(keys, key)
	}
	sort.StringSlice(keys).Sort()

	descs := make([]APIServiceDescription, 0)
	for _, key := range keys {
		descs = append(descs, set[key])
	}

	return descs
}

// GetRequiredAPIServiceDescriptions returns a deduplicated set of required APIServiceDescriptions
// with the intersection of required and owned removed
// Equivalent to the set subtraction required - owned
//
// Descriptions are returned in alphabetical order.
func (csv ClusterServiceVersion) GetRequiredAPIServiceDescriptions() []APIServiceDescription {
	set := make(map[string]APIServiceDescription)
	for _, required := range csv.Spec.APIServiceDefinitions.Required {
		name := fmt.Sprintf("%s.%s", required.Version, required.Group)
		set[name] = required
	}

	// Remove any shared owned from the set
	for _, owned := range csv.Spec.APIServiceDefinitions.Owned {
		name := fmt.Sprintf("%s.%s", owned.Version, owned.Group)
		if _, ok := set[name]; ok {
			delete(set, name)
		}
	}

	keys := make([]string, 0)
	for key := range set {
		keys = append(keys, key)
	}
	sort.StringSlice(keys).Sort()

	descs := make([]APIServiceDescription, 0)
	for _, key := range keys {
		descs = append(descs, set[key])
	}

	return descs
}

// GetOwnedAPIServiceDescriptions returns a deduplicated set of owned APIServiceDescriptions
//
// Descriptions are returned in alphabetical order.
func (csv ClusterServiceVersion) GetOwnedAPIServiceDescriptions() []APIServiceDescription {
	set := make(map[string]APIServiceDescription)
	for _, owned := range csv.Spec.APIServiceDefinitions.Owned {
		name := owned.GetName()
		set[name] = owned
	}

	keys := make([]string, 0)
	for key := range set {
		keys = append(keys, key)
	}
	sort.StringSlice(keys).Sort()

	descs := make([]APIServiceDescription, 0)
	for _, key := range keys {
		descs = append(descs, set[key])
	}

	return descs
}
