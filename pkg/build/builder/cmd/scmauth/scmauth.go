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
	Setup(baseDir string) (*url.URL, error)
}
