package v1

import (
	operatorv1 "github.com/openshift/api/operator/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Config contains the configuration and detailed condition status for the Samples Operator.
type Config struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata" protobuf:"bytes,1,opt,name=metadata"`

	// +kubebuilder:validation:Required
	// +required
	Spec ConfigSpec `json:"spec" protobuf:"bytes,2,opt,name=spec"`
	// +optional
	Status ConfigStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// ConfigSpec contains the desired configuration and state for the Samples Operator, controlling
// various behavior around the imagestreams and templates it creates/updates in the
// openshift namespace.
type ConfigSpec struct {
	// managementState is top level on/off type of switch for all operators.
	// When "Managed", this operator processes config and manipulates the samples accordingly.
	// When "Unmanaged", this operator ignores any updates to the resources it watches.
	// When "Removed", it reacts that same wasy as it does if the Config object
	// is deleted, meaning any ImageStreams or Templates it manages (i.e. it honors the skipped
	// lists) and the registry secret are deleted, along with the ConfigMap in the operator's
	// namespace that represents the last config used to manipulate the samples,
	ManagementState operatorv1.ManagementState `json:"managementState,omitempty" protobuf:"bytes,1,opt,name=managementState"`

	// samplesRegistry allows for the specification of which registry is accessed
	// by the ImageStreams for their image content.  Defaults on the content in https://github.com/openshift/library
	// that are pulled into this github repository, but based on our pulling only ocp content it typically
	// defaults to registry.redhat.io.
	SamplesRegistry string `json:"samplesRegistry,omitempty" protobuf:"bytes,2,opt,name=samplesRegistry"`

	// architectures determine which hardware architecture(s) to install, where x86_64, ppc64le, and s390x are the only
	// supported choices currently.
	Architectures []string `json:"architectures,omitempty" protobuf:"bytes,4,opt,name=architectures"`

	// skippedImagestreams specifies names of image streams that should NOT be
	// created/updated.  Admins can use this to allow them to delete content
	// they don’t want.  They will still have to manually delete the
	// content but the operator will not recreate(or update) anything
	// listed here.
	SkippedImagestreams []string `json:"skippedImagestreams,omitempty" protobuf:"bytes,5,opt,name=skippedImagestreams"`

	// skippedTemplates specifies names of templates that should NOT be
	// created/updated.  Admins can use this to allow them to delete content
	// they don’t want.  They will still have to manually delete the
	// content but the operator will not recreate(or update) anything
	// listed here.
	SkippedTemplates []string `json:"skippedTemplates,omitempty" protobuf:"bytes,6,opt,name=skippedTemplates"`
}

// ConfigStatus contains the actual configuration in effect, as well as various details
// that describe the state of the Samples Operator.
type ConfigStatus struct {
	// managementState reflects the current operational status of the on/off switch for
	// the operator.  This operator compares the ManagementState as part of determining that we are turning
	// the operator back on (i.e. "Managed") when it was previously "Unmanaged".
	// +patchMergeKey=type
	// +patchStrategy=merge
	ManagementState operatorv1.ManagementState `json:"managementState,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=managementState"`
	// conditions represents the available maintenance status of the sample
	// imagestreams and templates.
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []ConfigCondition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,2,rep,name=conditions"`

	// samplesRegistry allows for the specification of which registry is accessed
	// by the ImageStreams for their image content.  Defaults on the content in https://github.com/openshift/library
	// that are pulled into this github repository, but based on our pulling only ocp content it typically
	// defaults to registry.redhat.io.
	// +patchMergeKey=type
	// +patchStrategy=merge
	SamplesRegistry string `json:"samplesRegistry,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,3,rep,name=samplesRegistry"`

	// architectures determine which hardware architecture(s) to install, where x86_64 and ppc64le are the
	// supported choices.
	// +patchMergeKey=type
	// +patchStrategy=merge
	Architectures []string `json:"architectures,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,5,rep,name=architectures"`

	// skippedImagestreams specifies names of image streams that should NOT be
	// created/updated.  Admins can use this to allow them to delete content
	// they don’t want.  They will still have to manually delete the
	// content but the operator will not recreate(or update) anything
	// listed here.
	// +patchMergeKey=type
	// +patchStrategy=merge
	SkippedImagestreams []string `json:"skippedImagestreams,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,6,rep,name=skippedImagestreams"`

	// skippedTemplates specifies names of templates that should NOT be
	// created/updated.  Admins can use this to allow them to delete content
	// they don’t want.  They will still have to manually delete the
	// content but the operator will not recreate(or update) anything
	// listed here.
	// +patchMergeKey=type
	// +patchStrategy=merge
	SkippedTemplates []string `json:"skippedTemplates,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,7,rep,name=skippedTemplates"`

	// version is the value of the operator's payload based version indicator when it was last successfully processed
	// +patchMergeKey=type
	// +patchStrategy=merge
	Version string `json:"version,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,8,rep,name=version"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata" protobuf:"bytes,1,opt,name=metadata"`
	Items           []Config `json:"items" protobuf:"bytes,2,rep,name=items"`
}

const (
	// SamplesRegistryCredentials is the name for a secret that contains a username+password/token
	// for the registry, where if the secret is present, will be used for authentication.
	// The corresponding secret is required to already be formatted as a
	// dockerconfig secret so that it can just be copied
	// to the openshift namespace
	// for use during imagestream import.
	SamplesRegistryCredentials = "samples-registry-credentials"
	// ConfigName is the name/identifier of the static, singleton operator employed for the samples.
	ConfigName = "cluster"
	// X86Architecture is the value used to specify the x86_64 hardware architecture
	// in the Architectures array field.
	X86Architecture = "x86_64"
	// AMDArchitecture is the golang value for x86 64 bit hardware architecture; for the purposes
	// of this operator, it is equivalent to X86Architecture, which is kept for historical/migration
	// purposes
	AMDArchitecture = "amd64"
	// PPCArchitecture is the value used to specify the x86_64 hardware architecture
	// in the Architectures array field.
	PPCArchitecture = "ppc64le"
	// S390Architecture is the value used to specify the s390x hardware architecture
	// in the Architecture array field.
	S390Architecture = "s390x"
	// ConfigFinalizer is the text added to the Config.Finalizer field
	// to enable finalizer processing.
	ConfigFinalizer = GroupName + "/finalizer"
	// SamplesManagedLabel is the key for a label added to all the imagestreams and templates
	// in the openshift namespace that the Config is managing.  This label is adjusted
	// when changes to the SkippedImagestreams and SkippedTemplates fields are made.
	SamplesManagedLabel = GroupName + "/managed"
	// SamplesVersionAnnotation is the key for an annotation set on the imagestreams, templates,
	// and secret that this operator manages that signifies the version of the operator that
	// last managed the particular resource.
	SamplesVersionAnnotation = GroupName + "/version"
	// SamplesRecreateCredentialAnnotation is the key for an annotation set on the secret used
	// for authentication when configuration moves from Removed to Managed but the associated secret
	// in the openshift namespace does not exist.  This will initiate creation of the credential
	// in the openshift namespace.
	SamplesRecreateCredentialAnnotation = GroupName + "/recreate"
	// OperatorNamespace is the namespace the operator runs in.
	OperatorNamespace = "openshift-cluster-samples-operator"
)

type ConfigConditionType string

// the valid conditions of the Config

const (
	// ImportCredentialsExist represents the state of any credentials specified by
	// the SamplesRegistry field in the Spec.
	ImportCredentialsExist ConfigConditionType = "ImportCredentialsExist"
	// SamplesExist represents whether an incoming Config has been successfully
	// processed or not all, or whether the last Config to come in has been
	// successfully processed.
	SamplesExist ConfigConditionType = "SamplesExist"
	// ConfigurationValid represents whether the latest Config to come in
	// tried to make a support configuration change.  Currently, changes to the
	// InstallType and Architectures list after initial processing is not allowed.
	ConfigurationValid ConfigConditionType = "ConfigurationValid"
	// ImageChangesInProgress represents the state between where the samples operator has
	// started updating the imagestreams and when the spec and status generations for each
	// tag match.  The list of imagestreams that are still in progress will be stored
	// in the Reason field of the condition.  The Reason field being empty corresponds
	// with this condition being marked true.
	ImageChangesInProgress ConfigConditionType = "ImageChangesInProgress"
	// RemovePending represents whether the Config Spec ManagementState
	// has been set to Removed, but we have not completed the deletion of the
	// samples, pull secret, etc. and set the Config Spec ManagementState to Removed.
	// Also note, while a samples creation/update cycle is still in progress, and ImageChagesInProgress
	// is True, the operator will not initiate the deletions, as we
	// do not want the create/updates and deletes of the samples to be occurring in parallel.
	// So the actual Removed processing will be initated only after ImageChangesInProgress is set
	// to false.  Once the deletions are done, and the Status ManagementState is Removed, this
	// condition is set back to False.  Lastly, when this condition is set to True, the
	// ClusterOperator Progressing condition will be set to True.
	RemovePending ConfigConditionType = "RemovePending"
	// MigrationInProgress represents the special case where the operator is running off of
	// a new version of its image, and samples are deployed of a previous version.  This condition
	// facilitates the maintenance of this operator's ClusterOperator object.
	MigrationInProgress ConfigConditionType = "MigrationInProgress"
	// ImportImageErrorsExist registers any image import failures, separate from ImageChangeInProgress,
	// so that we can a) indicate a problem to the ClusterOperator status, b) mark the current
	// change cycle as complete in both ClusterOperator and Config; retry on import will
	// occur by the next relist interval if it was an intermittent issue;
	ImportImageErrorsExist ConfigConditionType = "ImportImageErrorsExist"
)

// ConfigCondition captures various conditions of the Config
// as entries are processed.
type ConfigCondition struct {
	// type of condition.
	Type ConfigConditionType `json:"type" protobuf:"bytes,1,opt,name=type,casttype=ConfigConditionType"`
	// status of the condition, one of True, False, Unknown.
	Status corev1.ConditionStatus `json:"status" protobuf:"bytes,2,opt,name=status,casttype=k8s.io/kubernetes/pkg/api/v1.ConditionStatus"`
	// lastUpdateTime is the last time this condition was updated.
	LastUpdateTime metav1.Time `json:"lastUpdateTime,omitempty" protobuf:"bytes,3,opt,name=lastUpdateTime"`
	// lastTransitionTime is the last time the condition transitioned from one status to another.
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty" protobuf:"bytes,4,opt,name=lastTransitionTime"`
	// reason is what caused the condition's last transition.
	Reason string `json:"reason,omitempty" protobuf:"bytes,5,opt,name=reason"`
	// message is a human readable message indicating details about the transition.
	Message string `json:"message,omitempty" protobuf:"bytes,6,opt,name=message"`
}
