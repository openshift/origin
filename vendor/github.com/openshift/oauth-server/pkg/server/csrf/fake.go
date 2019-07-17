package csrf

import "net/http"

// FakeCSRF returns the given token and error for testing purposes
type FakeCSRF struct {
	Token string
}

// Generate implements the CSRF interface
func (c *FakeCSRF) Generate(w http.ResponseWriter, req *http.Request) string {
	return c.Token
}

// Check implements the CSRF interface
func (c *FakeCSRF) Check(req *http.Request, value string) bool {
	return c.Token == value
}
