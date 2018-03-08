package v2

const (
	// AcceptsIncomplete is the name of a query parameter that indicates that
	// the client allows a request to complete asynchronously.
	AcceptsIncomplete = "accepts_incomplete"

	// VarKeyInstanceID is the name to use for a mux var representing an
	// instance ID.
	VarKeyInstanceID = "instance_id"

	// VarKeyBindingID is the name to use for a mux var representing a binding
	// ID.
	VarKeyBindingID = "binding_id"

	// VarKeyServiceID is the name to use for a mux var representing a service ID.
	VarKeyServiceID = "service_id"

	// VarKeyPlanID is the name to use for a mux var representing a plan ID.
	VarKeyPlanID = "plan_id"

	// VarKeyOperation is the name to use for a mux var representing an
	// operation.
	VarKeyOperation = "operation"
)
