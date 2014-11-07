package csrf

import "net/http"

// CSRF handles generating a csrf value, and checking the submitted value
type CSRF interface {
	// Generate returns a CSRF token suitable for inclusion in a form
	Generate(http.ResponseWriter, *http.Request) (string, error)
	// Check returns true if the given token is valid for the given request
	Check(*http.Request, string) (bool, error)
}
