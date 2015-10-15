package f5

// f5Result represents an F5 BIG-IP LTM request response.  f5Result is used to
// unmarshal the JSON response when receiving responses from the F5 iControl
// REST API.  These responses generally are JSON blobs containing at least the
// fields described in this structure.
//
// f5Result may be embedded into other types for requests that return objects.
type f5Result struct {
	// Code should match the HTTP status code.
	Code int `json:"code"`

	// Message should contain a short description of the result of the requested
	// operation.
	Message *string `json:"message"`
}

// F5Error represents an error resulting from a request to the F5 BIG-IP
// iControl REST interface.
type F5Error struct {
	// f5result holds the standard header (code and message) that is included in
	// responses from F5.
	f5Result

	// verb is the HTTP verb (GET, POST, PUT, PATCH, or DELETE) that was
	// used in the request that resulted in the error.
	verb string

	// url is the URL that was used in the request that resulted in the error.
	url string

	// httpStatusCode is the HTTP response status code (e.g., 200, 404, etc.).
	httpStatusCode int

	// err contains a descriptive error object for error cases other than HTTP
	// errors (i.e., non-2xx responses), such as socket errors or malformed JSON.
	err error
}

// f5VserverPolicy represents an F5 BIG-IP LTM policy associated with a vserver.
// The F5 router uses it within f5Vserver to unmarshal the JSON response when
// requesting a vserver from F5 BIG-IP.
type f5VserverPolicy struct {
	Name string `json:"name"`
}

// f5VserverPolicies represents the policies associated with an F5 BIG-IP LTM
// vserver.  The F5 router uses it to unmarshal the JSON response when
// requesting the list of policies that are associated with a specific vserver
// on F5 BIG-IP.
type f5VserverPolicies struct {
	Policies []f5VserverPolicy `json:"items"`
}

// f5VserverIRules represents the iRules associated with an F5 BIG-IP LTM
// vserver.  The F5 router uses it to unmarshal the JSON response when
// requesting the list of iRules that are associated with a specific vserver on
// F5 BIG-IP.  f5VserverIRules also describes the payload for a PATCH request by
// which the F5 router associates an iRule with a vserver.
type f5VserverIRules struct {
	Rules []string `json:"rules"`
}

// f5Pool represents an F5 BIG-IP LTM pool.  It describes the payload for a POST
// request by which the F5 router creates a new pool.
type f5Pool struct {
	// Mode is the method of load balancing that F5 BIG-IP employs over members of
	// the pool.  The F5 router uses round-robin; other allowed values are
	// dynamic-ratio-member, dynamic-ratio-node, fastest-app-response,
	// fastest-node, least-connections-node, least-sessions, observed-member,
	// observed-node, ratio-member, ratio-node, ratio-session,
	// ratio-least-connections-member, ratio-least-connections-node, and
	// weighted-least-connections-member.
	Mode string `json:"loadBalancingMode"`

	// Monitor is the name of the monitor associated with the pool.  The F5 router
	// uses /Common/http.
	Monitor string `json:"monitor"`

	// Name is the name of the pool.  The F5 router uses names of the form
	// openshift_<namespace>_<servicename>.
	Name string `json:"name"`
}

// f5PoolMember represents an F5 BIG-IP LTM pool member.  The F5 router uses it
// within f5PoolMemberset to unmarshal the JSON response when requesting a pool
// from F5.  f5PoolMember also describes the payload for a POST request by which
// the F5 router adds a member to a pool.
type f5PoolMember struct {
	// Name is the name of the pool member.  The F5 router uses names of the form
	// ipaddr:port.
	Name string `json:"name"`
}

// f5PoolMemberset represents an F5 BIG-IP LTM pool.  The F5 router uses it to
// unmarshal the JSON response when requesting a pool from F5.
type f5PoolMemberset struct {
	// Members is an array of pool members, which are represented using
	// f5PoolMember objects.
	Members []f5PoolMember `json:"items"`
}

// f5Policy represents an F5 BIG-IP LTM policy.  It describes the payload for
// a POST request by which the F5 router creates a new policy.
type f5Policy struct {
	// Name is the name of the policy.
	Name string `json:"name"`

	// Controls is a list of F5 BIG-IP LTM features enabled for the pool.
	// Typically we use just forwarding; other possible values are caching,
	// classification, compression, request-adaption, response-adaption, and
	// server-ssl.
	Controls []string `json:"controls"`

	// Requires is a list of available profile types.  Typically we use just http;
	// other possible values are client-ssl, ssl-persistence, and tcp.
	Requires []string `json:"requires"`

	// Strategy is the strategy according to which rules are applied to incoming
	// connections when more than one rule matches.  Typically we use best-match;
	// other possible values are all-match and first-match.
	Strategy string `json:"strategy"`
}

// f5Rule represents an F5 BIG-IP LTM policy rule.  The F5 router uses it within
// f5PolicyRuleset to unmarshal the JSON response when requesting a policy from
// F5.  f5Rule also describes the payload for a POST request by which the F5
// router creates a new rule.
type f5Rule struct {
	Name string `json:"name"`
}

// f5PolicyRuleset represents an F5 BIG-IP LTM policy ruleset.  The F5 router
// uses it to unmarshal the JSON response when requesting a policy's rules from
// F5 BIG-IP.
type f5PolicyRuleset struct {
	Rules []f5Rule `json:"items"`
}

// f5RuleCondition represents a condition for an F5 BIG-IP LTM policy rule.  The
// F5 router uses it to add a condition to a rule.
type f5RuleCondition struct {
	// Name is the name of the condition.  We generally use unique numerals.
	Name string `json:"name"`

	// CaseInsensitive specifies that string matches are case insensitive.
	CaseInsensitive bool `json:"caseInsensitive"`

	// HttpHost indicates that the condition must match on vhost.
	HttpHost bool `json:"httpHost,omitempty"`

	// HttpUri indicates that the condition must match on the request URI.
	HttpUri bool `json:"httpUri,omitempty"`

	// PathSegment, used with HttpUri, indicates that the condition must match
	// on a particular path segment from the request URI.
	PathSegment bool `json:"pathSegment,omitempty"`

	// Index indicates which component of an item (e.g., which segment of
	// a path) this condition checks.
	Index int `json:"index"`

	// Equals indicates that the condition tests for equality.
	Equals bool `json:"equals"`

	// Request indicates that the rule matches on requests as opposed to
	// responses.
	Request bool `json:"request"`

	// Host, used with HttpHost, indicates that the rule should match against
	// the vhost (as opposed to the port or both the vhost and the port).
	Host bool `json:"host,omitempty"`

	// Values specifies items for matching such as pathnames or hostnames.
	Values []string `json:"values"`
}

// f5RuleAction represents an action for an F5 BIG-IP LTM policy rule.  The F5
// router uses it to add an action to a rule.
type f5RuleAction struct {
	// Name is the name of the action.  We generally just use "0" because we have
	// only one action associated with each rule.
	Name string `json:"name"`

	// Forward indicates that the connection should be forwarded.
	Forward bool `json:"forward"`

	// Pool, used with Forward and Select, indicates a pool to which the
	// connection should be forwarded.
	Pool string `json:"pool"`

	// Request indicates that the action takes effect on requests as opposed to
	// responses.
	Request bool `json:"request"`

	// Select indicates that the action selects a destination for the connection
	// (as opposed to resetting it).
	Select bool `json:"select"`

	// Vlan is the vlan associated with the destination.
	Vlan int `json:"vlanId"`
}

// f5DatagroupRecord represents an F5 BIG-IP LTM data-group record.  The F5
// router uses it within f5Datagroup to unmarshal the JSON response when
// requesting a data-group from F5.
type f5DatagroupRecord struct {
	Key   string `json:"name"`
	Value string `json:"data"`
}

// f5Datagroup represents an F5 BIG-IP LTM data-group.  The unmarshal the JSON
// response when requesting a data-group from F5.  f5Datagroup also describes
// the payload for a PATCH request by which the F5 router creates a new
// data-group.
type f5Datagroup struct {
	// Name is the name of the data-group.
	Name string `json:"name,omitempty"`

	// Type is the type of values in the data-group.
	Type string `json:"type,omitempty"`

	// Records is an array of key-value records in the data-group, which are
	// represented using f5DatagroupRecord objects.
	//
	// Note that if there are no entries in the datagroup, we must post an empty
	// records set, so we must not use omitempty here.
	Records []f5DatagroupRecord `json:"records"`
}

// f5IRule represents an F5 BIG-IP LTM iRule.  It describes the payload for
// a POST request by which the F5 router creates a new iRule.
type f5IRule struct {
	// Name is the name of the iRule.
	Name string `json:"name"`

	// Code is the TCL code of the iRule.
	Code string `json:"apiAnonymous"`
}

// f5InstallCommandPayload represents an install command that will be issued to
// the F5 BIG-IP.  It describes the payload for a POST request by which the F5
// router installs a key or certificate on F5 BIG-IP.
type f5InstallCommandPayload struct {
	// Command specifies to F5 BIG-IP the operation we would like to perform.
	Command string `json:"command"`

	// Name specifies the name by which F5 BIG-IP will identify the file once it
	// is installed.
	Name string `json:"name"`

	// Filename specifies the path to the file that we just scpd to the F5
	// BIG-IP host.
	Filename string `json:"from-local-file"`
}

// f5SslProfilePayload represents an F5 BIG-IP LTM ssl-profile.  It describes
// the payload for a POST request to create a client-ssl or server-ssl profile
// on F5 BIG-IP.
type f5SslProfilePayload struct {
	// Certificate specifies the name of the certificate on the F5 BIG-IP host.
	Certificate string `json:"cert,omitempty"`

	// Key specifies the name of the private key on the F5 BIG-IP host.
	Key string `json:"key,omitempty"`

	// Chain specifies the name of the certificate chain on the F5 BIG-IP host.
	Chain string `json:"chain,omitempty"`

	// Name specifies the name of the profile.
	Name string `json:"name"`

	// ServerName specifies the vhost for the certificate and private key.
	ServerName string `json:"serverName"`
}

// f5VserverProfilePayload represents a profile for an F5 BIG-IP LTM vserver.  It
// describes the payload for a POST request by which the F5 router associates an
// SSL profile with a vserver.
type f5VserverProfilePayload struct {
	// Context specifies where the traffic is encrypted: client-side (i.e.,
	// between client and F5) or server-side (i.e., between F5 and pods).
	Context string `json:"context"`

	// Name specifies the name of the profile.
	Name string `json:"name"`
}

// f5AddPartitionPathPayload adds a folder to an F5 BIG-IP administrative
// partition. It describes the payload for a POST request which allows the
// F5 router to create custom partition paths.
type f5AddPartitionPathPayload struct {
	// Name is the partition path to be added.
	Name string `json:"name"`
}
