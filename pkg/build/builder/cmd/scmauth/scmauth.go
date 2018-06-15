package scmauth

import "net/url"

// SCMAuth is an interface implemented by different authentication providers
// which are responsible for setting up the credentials to be used when accessing
// private repository.
type SCMAuth interface {
	// Name is the name of the authentication method for use in log and error messages
	Name() string

	// Handles returns true if this authentication method handles a file with the given name
	Handles(name string) bool

	// Setup lays down the required files for this authentication method to work.
	// Returns the the source URL stripped of credentials.
	Setup(baseDir string, context SCMAuthContext) error
}

// SCMAuthContext keeps track of variables needed for SCM authentication.
// The variables will then be used to invoke git
type SCMAuthContext interface {
	// Get returns the value of a variable if previously set on the context and
	// a boolean indicating whether the variable is set or not
	Get(name string) (string, bool)

	// Set will set the value of a variable. If a variable has already been set
	// and the value sent is different, then an error will be returned.
	Set(name, value string) error

	// SetOverrideURL will set an override URL. If a value has already been set
	// and the URL passed is different, then an error will be returned.
	SetOverrideURL(u *url.URL) error
}
