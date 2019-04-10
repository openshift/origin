package headers

import "net/http"

// We cannot set HSTS by default, it has too many drawbacks in environments
// that use self-signed certs
var standardHeaders = map[string]string{
	// Turn off caching, it never makes sense for authorization pages
	"Cache-Control": "no-cache, no-store, max-age=0, must-revalidate",
	"Pragma":        "no-cache",
	"Expires":       "0",
	// Use a reasonably strict Referer policy by default
	"Referrer-Policy": "strict-origin-when-cross-origin",
	// Do not allow embedding as that can lead to clickjacking attacks
	"X-Frame-Options": "DENY",
	// Add other basic security hygiene headers
	"X-Content-Type-Options": "nosniff",
	"X-DNS-Prefetch-Control": "off",
	"X-XSS-Protection":       "1; mode=block",
}

func WithStandardHeaders(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// force every request into the OAuth server to have our standard headers
		h := w.Header()
		for k, v := range standardHeaders {
			h.Set(k, v)
		}

		handler.ServeHTTP(w, r)
	})
}
