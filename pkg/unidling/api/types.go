package api

const (
	// IdledAtAnnotation indicates that a given object (endpoints or scalable object))
	// is currently idled (and the time at which it was idled)
	IdledAtAnnotation = "idling.alpha.openshift.io/idled-at"

	// UnidleTargetAnnotation contains the references and former scales for the scalable
	// objects associated with the idled endpoints
	UnidleTargetAnnotation = "idling.alpha.openshift.io/unidle-targets"

	// NeedPodsReason is the reason for the event emitted to indicate that endpoints should be unidled
	NeedPodsReason = "NeedPods"
)
