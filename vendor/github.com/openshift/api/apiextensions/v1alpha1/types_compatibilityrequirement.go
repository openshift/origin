package v1alpha1

import (
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CompatibilityRequirement expresses a set of requirements on a target CRD.
// It is used to ensure compatibility between different actors using the same
// CRD.
//
// Compatibility level 4: No compatibility is provided, the API can change at any point for any reason. These capabilities should not be used by applications needing long term support.
// +openshift:compatibility-gen:level=4
// +openshift:file-pattern=cvoRunLevel=0000_20,operatorName=crd-compatibility-checker,operatorOrdering=01
// +openshift:enable:FeatureGate=CRDCompatibilityRequirementOperator
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=compatibilityrequirements,scope=Cluster
// +openshift:api-approved.openshift.io=https://github.com/openshift/api/pull/2479
// +kubebuilder:metadata:annotations="release.openshift.io/feature-gate=CRDCompatibilityRequirementOperator"
type CompatibilityRequirement struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +required
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec is the specification of the desired behavior of the Compatibility Requirement.
	// +required
	Spec CompatibilityRequirementSpec `json:"spec,omitzero"`

	// status is the most recently observed status of the Compatibility Requirement.
	// +optional
	Status CompatibilityRequirementStatus `json:"status,omitzero"`
}

// CompatibilityRequirementSpec is the specification of the desired behavior of the Compatibility Requirement.
type CompatibilityRequirementSpec struct {
	// compatibilitySchema defines the schema used by
	// customResourceDefinitionSchemaValidation and objectSchemaValidation.
	// This field is required.
	// +required
	CompatibilitySchema CompatibilitySchema `json:"compatibilitySchema,omitzero"`

	// customResourceDefinitionSchemaValidation ensures that updates to the
	// installed CRD are compatible with this compatibility requirement. If not
	// specified, admission of the target CRD will not be validated.
	// This field is optional.
	// +optional
	CustomResourceDefinitionSchemaValidation CustomResourceDefinitionSchemaValidation `json:"customResourceDefinitionSchemaValidation,omitzero"`

	// objectSchemaValidation ensures that matching resources conform to
	// compatibilitySchema. If not specified, admission of matching resources
	// will not be validated.
	// This field is optional.
	// +optional
	ObjectSchemaValidation ObjectSchemaValidation `json:"objectSchemaValidation,omitzero"`
}

// CRDDataType indicates the type of the CRD data.
// +kubebuilder:validation:Enum=YAML
type CRDDataType string

const (
	// CRDDataTypeYAML indicates that the CRD data is in YAML format.
	CRDDataTypeYAML CRDDataType = "YAML"
)

// CRDData contains the complete definition of a CRD.
type CRDData struct {
	// type indicates the type of the CRD data. The only supported type is "YAML".
	// This field is required.
	// +required
	Type CRDDataType `json:"type,omitempty"`

	// data contains the complete definition of the CRD. This field must be in
	// the format specified by the type field. It may not be longer than 1572864
	// characters.
	// This field is required.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=1572864
	// +required
	Data string `json:"data,omitempty"`
}

// APIVersionSelectionType specifies a method for automatically selecting a
// set of API versions to require.
// +kubebuilder:validation:Enum=StorageOnly;AllServed
type APIVersionSelectionType string

const (
	APIVersionSetTypeStorageOnly APIVersionSelectionType = "StorageOnly"
	APIVersionSetTypeAllServed   APIVersionSelectionType = "AllServed"
)

// APIVersionString is a string representing a kubernetes API version.
// +kubebuilder:validation:MinLength=1
// +kubebuilder:validation:MaxLength=63
// +kubebuilder:validation:XValidation:rule="!format.dns1035Label().validate(self).hasValue()",message="It must contain only lower-case alphanumeric characters and hyphens and must start with an alphabetic character and end with an alphanumeric character"
type APIVersionString string

// APIVersions specifies a set of API versions of a CRD.
// +kubebuilder:validation:XValidation:rule="self.defaultSelection != 'AllServed' || !has(self.additionalVersions)",message="additionalVersions may not be defined when defaultSelection is 'AllServed'"
type APIVersions struct {
	// defaultSelection specifies a method for automatically selecting a set of
	// versions to require.
	//
	// Valid options are StorageOnly and AllServed.
	// When set to StorageOnly, only the storage version is selected for
	// compatibility assessment.
	// When set to AllServed, all served versions are selected for compatibility
	// assessment.
	//
	// This field is required.
	// +required
	DefaultSelection APIVersionSelectionType `json:"defaultSelection,omitempty"`

	// additionalVersions specifies a set api versions to require in addition to
	// the default selection. It is explicitly permitted to specify a version in
	// additionalVersions which was also selected by the default selection. The
	// selections will be merged and deduplicated.
	//
	// Each item must be at most 63 characters in length, and must must consist
	// of only lowercase alphanumeric characters and hyphens, and must start
	// with an alphabetic character and end with an alphanumeric character.// with an alphabetic character and end with an alphanumeric character.
	// At most 32 additional versions may be specified.
	//
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=32
	// +listType=set
	// +optional
	AdditionalVersions []APIVersionString `json:"additionalVersions,omitempty"`
}

// APIExcludedField describes a field in the schema which will not be validated by
// crdSchemaValidation or objectSchemaValidation.
type APIExcludedField struct {
	// path is the path to the field in the schema.
	// Paths are dot-separated field names (e.g., "fieldA.fieldB.fieldC") representing nested object fields.
	// If part of the path is a slice (e.g., "status.conditions") the remaining path is applied to all items in the slice
	// (e.g., "status.conditions.lastTransitionTimestamp").
	// Each field name must be a valid Kubernetes CRD field name: start with a letter, contain only
	// letters, digits, and underscores, and be between 1 and 63 characters in length.
	// A path may contain at most 16 fields.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=1023
	// +kubebuilder:validation:XValidation:rule="self.split('.').size() <= 16",message="There may be at most 16 fields in the path."
	// +kubebuilder:validation:XValidation:rule="self.split('.', 16).all(f, f.matches('^[a-zA-Z][a-zA-Z0-9_]{0,62}$'))",message="path must be dot-separated field names, each starting with a letter and containing only letters, digits, and underscores not exceeding 63 characters. There may be at most 16 fields in the path."
	// +required
	Path string `json:"path,omitempty"`

	// versions are the API versions the field is excluded from.
	// When not specified, the field is excluded from all versions.
	//
	// Each item must be at most 63 characters in length, and must must
	// consist of only lowercase alphanumeric characters and hyphens, and must
	// start with an alphabetic character and end with an alphanumeric
	// character.
	// At most 32 versions may be specified.
	//
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=32
	// +listType=set
	// +optional
	Versions []APIVersionString `json:"versions,omitempty"`
}

// CompatibilitySchema defines the schema used by crdSchemaValidation and objectSchemaValidation.
type CompatibilitySchema struct {
	// customResourceDefinition contains the complete definition of the CRD for schema and object validation purposes.
	// This field is required.
	// +required
	CustomResourceDefinition CRDData `json:"customResourceDefinition,omitzero"`

	// requiredVersions specifies a subset of the CRD's API versions which will be asserted for compatibility.
	// This field is required.
	// +required
	RequiredVersions APIVersions `json:"requiredVersions,omitzero"`

	// excludedFields is a set of fields in the schema which will not be validated by
	// crdSchemaValidation or objectSchemaValidation.
	// The list may contain at most 64 fields.
	// When not specified, all fields in the schema will be validated.
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=64
	// +listType=atomic
	// +optional
	ExcludedFields []APIExcludedField `json:"excludedFields,omitempty"`
}

// CustomResourceDefinitionSchemaValidation ensures that updates to the installed CRD are compatible with this compatibility requirement.
type CustomResourceDefinitionSchemaValidation struct {
	// action determines whether violations are rejected (Deny) or admitted with an API warning (Warn).
	// Valid options are Deny and Warn.
	// When set to Deny, incompatible CRDs will be rejected and not admitted to the cluster.
	// When set to Warn, incompatible CRDs will be allowed but a warning will be generated in the API response.
	// This field is required.
	// +required
	Action CRDAdmitAction `json:"action,omitempty"`
}

// ObjectSchemaValidation ensures that matching objects conform to the compatibilitySchema.
type ObjectSchemaValidation struct {
	// action determines whether violations are rejected (Deny) or admitted with an API warning (Warn).
	// Valid options are Deny and Warn.
	// When set to Deny, incompatible Objects will be rejected and not admitted to the cluster.
	// When set to Warn, incompatible Objects will be allowed but a warning will be generated in the API response.
	// This field is required.
	// +required
	Action CRDAdmitAction `json:"action,omitempty"`

	// namespaceSelector defines a label selector for namespaces. If defined,
	// only objects in a namespace with matching labels will be subject to
	// validation. When not specified, objects for validation will not be
	// filtered by namespace.
	// +kubebuilder:validation:XValidation:rule="size(self.matchLabels) > 0 || size(self.matchExpressions) > 0",message="must have at least one of matchLabels or matchExpressions when specified"
	// +optional
	NamespaceSelector metav1.LabelSelector `json:"namespaceSelector,omitempty"`
	// objectSelector defines a label selector for objects. If defined, only
	// objects with matching labels will be subject to validation. When not
	// specified, objects for validation will not be filtered by label.
	// +kubebuilder:validation:XValidation:rule="size(self.matchLabels) > 0 || size(self.matchExpressions) > 0",message="must have at least one of matchLabels or matchExpressions when specified"
	// +optional
	ObjectSelector metav1.LabelSelector `json:"objectSelector,omitempty"`

	// matchConditions defines the matchConditions field of the resulting ValidatingWebhookConfiguration.
	// When present, must contain between 1 and 64 match conditions.
	// When not specified, the webhook will match all requests according to its other selectors.
	// +listType=map
	// +listMapKey=name
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=64
	// +optional
	MatchConditions []admissionregistrationv1.MatchCondition `json:"matchConditions,omitempty"`
}

// CRDAdmitAction determines the action taken when a CRD is not compatible.
// +kubebuilder:validation:Enum=Deny;Warn
// +enum
type CRDAdmitAction string

const (
	// CRDAdmitActionDeny means that incompatible CRDs will be rejected.
	CRDAdmitActionDeny CRDAdmitAction = "Deny"

	// CRDAdmitActionWarn means that incompatible CRDs will be allowed but a warning will be generated.
	CRDAdmitActionWarn CRDAdmitAction = "Warn"
)

// CompatibilityRequirement's Progressing condition and corresponding reasons.
const (
	// CompatibilityRequirementProgressing is false if the spec has been
	// completely reconciled against the condition's observed generation.
	// True indicates that reconciliation is still in progress and the current status does not represent
	// a stable state. Progressing false with an error reason indicates that the object cannot be reconciled.
	CompatibilityRequirementProgressing string = "Progressing"

	// CompatibilityRequirementConfigurationErrorReason indicates that
	// reconciliation cannot progress due to an invalid spec. The controller
	// will not reconcile this object again until the spec is updated.
	CompatibilityRequirementConfigurationErrorReason string = "ConfigurationError"

	// CompatibilityRequirementTransientErrorReason indicates that
	// reconciliation failed due to an error that can be retried.
	CompatibilityRequirementTransientErrorReason string = "TransientError"

	// CompatibilityRequirementUpToDateReason surfaces when reconciliation
	// completed successfully for the condition's observed generation.
	CompatibilityRequirementUpToDateReason string = "UpToDate"
)

// CompatibilityRequirement's Admitted condition and corresponding reasons.
const (
	// CompatibilityRequirementAdmitted is true if the requirement has been configured in the validating webhook,
	// otherwise false.
	CompatibilityRequirementAdmitted string = "Admitted"

	// CompatibilityRequirementAdmittedReason surfaces when the requirement has been configured in the validating webhook.
	CompatibilityRequirementAdmittedReason string = "Admitted"

	// CompatibilityRequirementNotAdmittedReason surfaces when the requirement has not been configured in the validating webhook.
	CompatibilityRequirementNotAdmittedReason string = "NotAdmitted"
)

// CompatibilityRequirement's Compatible condition and corresponding reasons.
const (
	// CompatibilityRequirementCompatible is true if the observed CRD is compatible with the requirement,
	// otherwise false. Note that Compatible may be false when adding a new requirement which the existing
	// CRD does not meet.
	CompatibilityRequirementCompatible string = "Compatible"

	// CompatibilityRequirementRequirementsNotMetReason surfaces when a CRD exists, and it is not compatible with this requirement.
	CompatibilityRequirementRequirementsNotMetReason string = "RequirementsNotMet"

	// CompatibilityRequirementCRDNotFoundReason surfaces when the referenced CRD does not exist.
	CompatibilityRequirementCRDNotFoundReason string = "CRDNotFound"

	// CompatibilityRequirementCompatibleWithWarningsReason surfaces when the CRD exists and is compatible with this requirement, but Message contains one or more warning messages.
	CompatibilityRequirementCompatibleWithWarningsReason string = "CompatibleWithWarnings"

	// CompatibilityRequirementCompatibleReason surfaces when the CRD exists and is compatible with this requirement.
	CompatibilityRequirementCompatibleReason string = "Compatible"
)

// CompatibilityRequirementStatus defines the observed status of the Compatibility Requirement.
// +kubebuilder:validation:MinProperties=1
// +kubebuilder:validation:XValidation:rule="!has(oldSelf.crdName) || has(self.crdName) && oldSelf.crdName == self.crdName",message="crdName cannot be changed once set"
type CompatibilityRequirementStatus struct {
	// conditions is a list of conditions and their status.
	// Known condition types are Progressing, Admitted, and Compatible.
	//
	// The Progressing condition indicates if reconciliation of a CompatibilityRequirement is still
	// progressing or has finished.
	//
	// The Admitted condition indicates if the validating webhook has been configured.
	//
	// The Compatible condition indicates if the observed CRD is compatible with the requirement.
	//
	// +optional
	// +listType=map
	// +listMapKey=type
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=32
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// observedCRD documents the uid and generation of the CRD object when the current status was written.
	// This field will be omitted if the target CRD does not exist or could not be retrieved.
	// +optional
	ObservedCRD ObservedCRD `json:"observedCRD,omitzero"`

	// crdName is the name of the target CRD. The target CRD is not required to
	// exist, as we may legitimately place requirements on it before it is
	// created.  The observed CRD is given in status.observedCRD, which will be
	// empty if no CRD is observed.
	// When present, must be between 1 and 253 characters and conform to RFC 1123 subdomain format:
	// lowercase alphanumeric characters, '-' or '.', starting and ending with alphanumeric characters.
	// When not specified, the requirement applies to any CRD name discovered from the compatibility schema.
	// This field is optional. Once set, the value cannot be changed and must always remain set.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:XValidation:rule="!format.dns1123Subdomain().validate(self).hasValue()",message="a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character."
	// +optional
	CRDName string `json:"crdName,omitempty"`
}

// ObservedCRD contains information about the observed target CRD.
// +kubebuilder:validation:XValidation:rule="oldSelf.uid != self.uid || self.generation >= oldSelf.generation",message="generation may only increase on the same CRD"
type ObservedCRD struct {
	// uid is the uid of the observed CRD.
	// Must be a valid UUID consisting of lowercase hexadecimal digits in 5 hyphenated blocks (8-4-4-4-12 format).
	// Length must be between 1 and 36 characters.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=36
	// +kubebuilder:validation:Format=uuid
	// +required
	UID string `json:"uid,omitempty"`

	// generation is the observed generation of the CRD.
	// Must be a positive integer (minimum value of 1).
	// +kubebuilder:validation:Minimum=1
	// +required
	Generation int64 `json:"generation,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CompatibilityRequirementList is a collection of CompatibilityRequirements.
//
// Compatibility level 4: No compatibility is provided, the API can change at any point for any reason. These capabilities should not be used by applications needing long term support.
// +openshift:compatibility-gen:level=4
type CompatibilityRequirementList struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard list's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ListMeta `json:"metadata,omitzero"`

	// items is a list of CompatibilityRequirements.
	// +kubebuilder:validation:MaxItems=1000
	// +optional
	Items []CompatibilityRequirement `json:"items,omitempty"`
}
