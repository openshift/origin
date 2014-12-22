package assets

import (
	"net/http"
	"strings"
)

func HTML5ModeHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := Asset(strings.TrimPrefix(r.URL.Path, "/")); err != nil {
			b, err := Asset("index.html")
			if err != nil {
				http.Error(w, "Failed to read index.html", http.StatusInternalServerError)
				return
			} else {
				w.Write(b)
				return
			}
		}
		h.ServeHTTP(w, r)
	})
}
