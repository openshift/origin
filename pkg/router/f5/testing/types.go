package testing

// TOOD: fix the nested map deepcopy and generate the code again
// +k8s:deepcopy-gen=false

// mockF5State stores the state necessary to mock the functionality of an F5
// BIG-IP host that the F5 router uses.
type MockF5State struct {
	// Policies is the set of Policies that exist in the mock F5 host.
	Policies map[string]map[string]PolicyRule

	// VserverPolicies represents the associations between vservers and Policies
	// in the mock F5 host.
	VserverPolicies map[string]map[string]bool

	// certs represents the set of certificates that have been installed into
	// the mock F5 host.
	Certs map[string]bool

	// Keys represents the set of certificates that have been installed into
	// the mock F5 host.
	Keys map[string]bool

	// ServerSslProfiles represents the set of server-ssl profiles that exist in
	// the mock F5 host.
	ServerSslProfiles map[string]bool

	// ClientSslProfiles represents the set of client-ssl profiles that exist in
	// the mock F5 host.
	ClientSslProfiles map[string]bool

	// VserverProfiles represents the associations between vservers and
	// client-ssl and server-ssl profiles in the mock F5 host.
	//
	// Note that although the F5 management console displays client and server
	// profiles separately, the F5 iControl REST interface puts these
	// associations under a single REST endpoint.
	VserverProfiles map[string]map[string]bool

	// Datagroups represents the IRules data-groups in the F5 host.  For our
	// purposes, we assume that every data-group maps strings to strings.
	Datagroups map[string]Datagroup

	// IRules represents the IRules that exist in the F5 host.
	IRules map[string]IRule

	// VserverIRules represents the associations between vservers and IRules in
	// the mock F5 host.
	VserverIRules map[string][]string

	// PartitionPaths represents the partitions that exist in
	// the mock F5 host.
	PartitionPaths map[string]string

	// Pools represents the Pools that exist on the mock F5 host.
	Pools map[string]Pool
}

// A PolicyCondition describes a single condition for a policy rule to match.
type PolicyCondition struct {
	HttpHost    bool     `json:"httpHost,omitempty"`
	HttpUri     bool     `json:"httpUri,omitempty"`
	PathSegment bool     `json:"pathSegment,omitempty"`
	Index       int      `json:"index"`
	Host        bool     `json:"host,omitempty"`
	Values      []string `json:"values"`
}

// A PolicyRule has a name and comprises a list of Conditions and a list of
// actions.
type PolicyRule struct {
	Conditions []PolicyCondition
}

// A Datagroup is an associative array.  For our purposes, a Datagroup maps
// strings to strings.
type Datagroup map[string]string

// An IRule comprises a string of TCL code.
type IRule string

// A Pool comprises a set of strings of the form addr:port.
type Pool map[string]bool
