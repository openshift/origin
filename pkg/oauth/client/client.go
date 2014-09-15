package client

import (
	"fmt"
	"net/http"
)

type OAuthWrapper struct {
	http.RoundTripper
	Token string
}

func (w OAuthWrapper) RoundTrip(req *http.Request) (*http.Response, error) {
	req = cloneRequest(req)
	req.Header.Set("Authorization", fmt.Sprintf("bearer %s", w.Token))
	return w.RoundTripper.RoundTrip(req)
}

// cloneRequest returns a clone of the provided *http.Request.
// The clone is a shallow copy of the struct and its Header map.
func cloneRequest(r *http.Request) *http.Request {
	// shallow copy of the struct
	r2 := new(http.Request)
	*r2 = *r
	// deep copy of the Header
	r2.Header = make(http.Header)
	for k, s := range r.Header {
		r2.Header[k] = s
	}
	return r2
}
