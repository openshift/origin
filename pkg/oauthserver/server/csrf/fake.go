package csrf

import "net/http"

// FakeCSRF returns the given token and error for testing purposes
type FakeCSRF struct {
	Token string
	Err   error
}

// Generate implements the CSRF interface
func (c *FakeCSRF) Generate(w http.ResponseWriter, req *http.Request) (string, error) {
	return c.Token, c.Err
}

// Check implements the CSRF interface
func (c *FakeCSRF) Check(req *http.Request, value string) (bool, error) {
	return c.Token == value, c.Err
}
