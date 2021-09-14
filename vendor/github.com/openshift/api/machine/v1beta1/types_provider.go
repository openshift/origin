package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// ProviderSpec defines the configuration to use during node creation.
type ProviderSpec struct {

	// No more than one of the following may be specified.

	// Value is an inlined, serialized representation of the resource
	// configuration. It is recommended that providers maintain their own
	// versioned API types that should be serialized/deserialized from this
	// field, akin to component config.
	// +optional
	// +kubebuilder:validation:XPreserveUnknownFields
	Value *runtime.RawExtension `json:"value,omitempty" protobuf:"bytes,1,opt,name=value"`
}

// ObjectMeta is metadata that all persisted resources must have, which includes all objects
// users must create. This is a copy of customizable fields from metav1.ObjectMeta.
//
// ObjectMeta is embedded in `Machine.Spec`, `MachineDeployment.Template` and `MachineSet.Template`,
// which are not top-level Kubernetes objects. Given that metav1.ObjectMeta has lots of special cases
// and read-only fields which end up in the generated CRD validation, having it as a subset simplifies
// the API and some issues that can impact user experience.
//
// During the [upgrade to controller-tools@v2](https://github.com/kubernetes-sigs/cluster-api/pull/1054)
// for v1alpha2, we noticed a failure would occur running Cluster API test suite against the new CRDs,
// specifically `spec.metadata.creationTimestamp in body must be of type string: "null"`.
// The investigation showed that `controller-tools@v2` behaves differently than its previous version
// when handling types from [metav1](k8s.io/apimachinery/pkg/apis/meta/v1) package.
//
// In more details, we found that embedded (non-top level) types that embedded `metav1.ObjectMeta`
// had validation properties, including for `creationTimestamp` (metav1.Time).
// The `metav1.Time` type specifies a custom json marshaller that, when IsZero() is true, returns `null`
// which breaks validation because the field isn't marked as nullable.
//
// In future versions, controller-tools@v2 might allow overriding the type and validation for embedded
// types. When that happens, this hack should be revisited.
type ObjectMeta struct {
	// Name must be unique within a namespace. Is required when creating resources, although
	// some resources may allow a client to request the generation of an appropriate name
	// automatically. Name is primarily intended for creation idempotence and configuration
	// definition.
	// Cannot be updated.
	// More info: http://kubernetes.io/docs/user-guide/identifiers#names
	// +optional
	Name string `json:"name,omitempty" protobuf:"bytes,1,opt,name=name"`

	// GenerateName is an optional prefix, used by the server, to generate a unique
	// name ONLY IF the Name field has not been provided.
	// If this field is used, the name returned to the client will be different
	// than the name passed. This value will also be combined with a unique suffix.
	// The provided value has the same validation rules as the Name field,
	// and may be truncated by the length of the suffix required to make the value
	// unique on the server.
	//
	// If this field is specified and the generated name exists, the server will
	// NOT return a 409 - instead, it will either return 201 Created or 500 with Reason
	// ServerTimeout indicating a unique name could not be found in the time allotted, and the client
	// should retry (optionally after the time indicated in the Retry-After header).
	//
	// Applied only if Name is not specified.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#idempotency
	// +optional
	GenerateName string `json:"generateName,omitempty" protobuf:"bytes,2,opt,name=generateName"`

	// Namespace defines the space within each name must be unique. An empty namespace is
	// equivalent to the "default" namespace, but "default" is the canonical representation.
	// Not all objects are required to be scoped to a namespace - the value of this field for
	// those objects will be empty.
	//
	// Must be a DNS_LABEL.
	// Cannot be updated.
	// More info: http://kubernetes.io/docs/user-guide/namespaces
	// +optional
	Namespace string `json:"namespace,omitempty" protobuf:"bytes,3,opt,name=namespace"`

	// Map of string keys and values that can be used to organize and categorize
	// (scope and select) objects. May match selectors of replication controllers
	// and services.
	// More info: http://kubernetes.io/docs/user-guide/labels
	// +optional
	Labels map[string]string `json:"labels,omitempty" protobuf:"bytes,4,rep,name=labels"`

	// Annotations is an unstructured key value map stored with a resource that may be
	// set by external tools to store and retrieve arbitrary metadata. They are not
	// queryable and should be preserved when modifying objects.
	// More info: http://kubernetes.io/docs/user-guide/annotations
	// +optional
	Annotations map[string]string `json:"annotations,omitempty" protobuf:"bytes,5,rep,name=annotations"`

	// List of objects depended by this object. If ALL objects in the list have
	// been deleted, this object will be garbage collected. If this object is managed by a controller,
	// then an entry in this list will point to this controller, with the controller field set to true.
	// There cannot be more than one managing controller.
	// +optional
	// +patchMergeKey=uid
	// +patchStrategy=merge
	OwnerReferences []metav1.OwnerReference `json:"ownerReferences,omitempty" patchStrategy:"merge" patchMergeKey:"uid" protobuf:"bytes,6,rep,name=ownerReferences"`
}

// ConditionSeverity expresses the severity of a Condition Type failing.
type ConditionSeverity string

const (
	// ConditionSeverityError specifies that a condition with `Status=False` is an error.
	ConditionSeverityError ConditionSeverity = "Error"

	// ConditionSeverityWarning specifies that a condition with `Status=False` is a warning.
	ConditionSeverityWarning ConditionSeverity = "Warning"

	// ConditionSeverityInfo specifies that a condition with `Status=False` is informative.
	ConditionSeverityInfo ConditionSeverity = "Info"

	// ConditionSeverityNone should apply only to conditions with `Status=True`.
	ConditionSeverityNone ConditionSeverity = ""
)

// ConditionType is a valid value for Condition.Type.
type ConditionType string

// Condition defines an observation of a Machine API resource operational state.
type Condition struct {
	// Type of condition in CamelCase or in foo.example.com/CamelCase.
	// Many .condition.type values are consistent across resources like Available, but because arbitrary conditions
	// can be useful (see .node.status.conditions), the ability to deconflict is important.
	// +required
	Type ConditionType `json:"type" protobuf:"bytes,1,opt,name=type,casttype=ConditionType"`

	// Status of the condition, one of True, False, Unknown.
	// +required
	Status corev1.ConditionStatus `json:"status" protobuf:"bytes,2,opt,name=status,casttype=k8s.io/api/core/v1.ConditionStatus"`

	// Severity provides an explicit classification of Reason code, so the users or machines can immediately
	// understand the current situation and act accordingly.
	// The Severity field MUST be set only when Status=False.
	// +optional
	Severity ConditionSeverity `json:"severity,omitempty" protobuf:"bytes,3,opt,name=severity,casttype=ConditionSeverity"`

	// Last time the condition transitioned from one status to another.
	// This should be when the underlying condition changed. If that is not known, then using the time when
	// the API field changed is acceptable.
	// +required
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty" protobuf:"bytes,4,opt,name=lastTransitionTime"`

	// The reason for the condition's last transition in CamelCase.
	// The specific API may choose whether or not this field is considered a guaranteed API.
	// This field may not be empty.
	// +optional
	Reason string `json:"reason,omitempty" protobuf:"bytes,5,opt,name=reason"`

	// A human readable message indicating details about the transition.
	// This field may be empty.
	// +optional
	Message string `json:"message,omitempty" protobuf:"bytes,6,opt,name=message"`
}

// Conditions provide observations of the operational state of a Machine API resource.
type Conditions []Condition
