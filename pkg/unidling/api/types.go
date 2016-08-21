package api

const (
	// IdledAtAnnotation indicates that a given object (endpoints or scalable object))
	// is currently idled (and the time at which it was idled)
	IdledAtAnnotation = "idling.alpha.openshift.io/idled-at"

	// UnidleTargetAnnotation contains the references and former scales for the scalable
	// objects associated with the idled endpoints
	UnidleTargetAnnotation = "idling.alpha.openshift.io/unidle-targets"

	// PreviousScaleAnnotation contains the previous scale of a scalable object
	// (currently only applied by the idler)
	PreviousScaleAnnotation = "idling.alpha.openshift.io/previous-scale"

	// NeedPodsReason is the reason for the event emitted to indicate that endpoints should be unidled
	NeedPodsReason = "NeedPods"
)

// NB: if these get changed, you'll need to actually add in the full API machinery for them

// RecordedScaleReference is a CrossGroupObjectReference to a scale subresource that also
// has the previous replica count recorded
type RecordedScaleReference struct {
	// Reference to the idled resource
	CrossGroupObjectReference `json:",inline" protobuf:"bytes,1,opt,name=crossVersionObjectReference"`
	// The last seen scale of the idled resource (before idling)
	Replicas int32 `json:"replicas" protobuf:"varint,2,opt,name=replicas"`
}

// CrossGroupObjectReference is a reference to an object in the same
// namespace in the specified group.  It is similar to
// autoscaling.CrossVersionObjectReference.
type CrossGroupObjectReference struct {
	// Kind of the referent; More info: http://releases.k8s.io/release-1.3/docs/devel/api-conventions.md#types-kinds"
	Kind string `json:"kind" protobuf:"bytes,1,opt,name=kind"`
	// Name of the referent; More info: http://releases.k8s.io/release-1.3/docs/user-guide/identifiers.md#names
	Name string `json:"name" protobuf:"bytes,2,opt,name=name"`
	// API version of the referent (deprecated, prefer usng Group instead)
	APIVersion string `json:"apiVersion,omitempty" protobuf:"bytes,3,opt,name=apiVersion"`
	// Group of the referent
	Group string `json:"group,omitempty" protobuf:"bytes,3,opt,name=group"`
}
