package httprequest

import (
	"net/http"
	"strings"

	"bitbucket.org/ww/goautoneg"
)

// PrefersHTML returns true if the request was made by something that looks like a browser, or can receive HTML
func PrefersHTML(req *http.Request) bool {
	accepts := goautoneg.ParseAccept(req.Header.Get("Accept"))
	acceptsHTML := false
	acceptsJSON := false
	for _, accept := range accepts {
		if accept.Type == "text" && accept.SubType == "html" {
			acceptsHTML = true
		} else if accept.Type == "application" && accept.SubType == "json" {
			acceptsJSON = true
		}
	}

	// If HTML is accepted, return true
	if acceptsHTML {
		return true
	}

	// If JSON was specifically requested, return false
	// This gives browsers a way to make requests and add an "Accept" header to request JSON
	if acceptsJSON {
		return false
	}

	// In Intranet/Compatibility mode, IE sends an Accept header that does not contain "text/html".
	if strings.HasPrefix(req.UserAgent(), "Mozilla") {
		return true
	}

	return false
}
