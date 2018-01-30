package headers

import (
	"net/http"
)

func SetStandardHeaders(w http.ResponseWriter) {
	// We cannot set HSTS by default, it has too many drawbacks in environments
	// that use self-signed certs
	standardHeaders := map[string]string{
		// Turn off caching, it never makes sense for authorization pages
		"Cache-Control": "no-cache, no-store",
		"Pragma":        "no-cache",
		"Expires":       "0",
		// Use a reasonably strict Referer policy by default
		"Referrer-Policy": "strict-origin-when-cross-origin",
		// Do not allow embedding as that can lead to clickjacking attacks
		"X-Frame-Options": "DENY",
		// Add other basic scurity hygiene headers
		"X-Content-Type-Options": "nosniff",
		"X-DNS-Prefetch-Control": "off",
		"X-XSS-Protection":       "1; mode=block",
	}

	for key, val := range standardHeaders {
		w.Header().Set(key, val)
	}
}
