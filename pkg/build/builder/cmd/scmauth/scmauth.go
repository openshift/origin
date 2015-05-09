package scmauth

// SCMAuth is an interface implemented by different authentication providers
// which are responsible for setting up the credentials to be used when accessing
// private repository.
type SCMAuth interface {
	Name() string
	Setup(baseDir string) error
}
