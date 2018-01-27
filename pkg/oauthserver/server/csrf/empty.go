package csrf

import "net/http"

type emptyCsrf struct{}

// NewEmptyCSRF returns a CSRF object which generates empty CSRF tokens,
// and accepts any token as valid
func NewEmptyCSRF() CSRF {
	return emptyCsrf{}
}

// Generate implements the CSRF interface
func (emptyCsrf) Generate(http.ResponseWriter, *http.Request) (string, error) {
	return "", nil
}

// Check implements the CSRF interface
func (emptyCsrf) Check(*http.Request, string) (bool, error) {
	return true, nil
}
