package raven

// Message defines Sentry's spec compliant interface holding Message information - https://docs.sentry.io/development/sdk-dev/interfaces/message/
type Message struct {
	// Required
	Message string `json:"message"`

	// Optional
	Params []interface{} `json:"params,omitempty"`
}

// Class provides name of implemented Sentry's interface
func (m *Message) Class() string { return "logentry" }

// Template defines Sentry's spec compliant interface holding Template information - https://docs.sentry.io/development/sdk-dev/interfaces/template/
type Template struct {
	// Required
	Filename    string `json:"filename"`
	Lineno      int    `json:"lineno"`
	ContextLine string `json:"context_line"`

	// Optional
	PreContext   []string `json:"pre_context,omitempty"`
	PostContext  []string `json:"post_context,omitempty"`
	AbsolutePath string   `json:"abs_path,omitempty"`
}

// Class provides name of implemented Sentry's interface
func (t *Template) Class() string { return "template" }

// User defines Sentry's spec compliant interface holding User information - https://docs.sentry.io/development/sdk-dev/interfaces/user/
type User struct {
	// All fields are optional
	ID       string `json:"id,omitempty"`
	Username string `json:"username,omitempty"`
	Email    string `json:"email,omitempty"`
	IP       string `json:"ip_address,omitempty"`
}

// Class provides name of implemented Sentry's interface
func (h *User) Class() string { return "user" }

// Query defines Sentry's spec compliant interface holding Context information - https://docs.sentry.io/development/sdk-dev/interfaces/contexts/
type Query struct {
	// Required
	Query string `json:"query"`

	// Optional
	Engine string `json:"engine,omitempty"`
}

// Class provides name of implemented Sentry's interface
func (q *Query) Class() string { return "query" }
