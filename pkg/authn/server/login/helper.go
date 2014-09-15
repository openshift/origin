package login

import (
	"net/http"
	"net/url"
)

func failed(reason string, w http.ResponseWriter, req *http.Request) {
	uri, err := getBaseURL(req)
	if err != nil {
		http.Redirect(w, req, req.URL.Path, http.StatusTemporaryRedirect)
		return
	}
	query := url.Values{}
	query.Set("reason", reason)
	if then := req.FormValue("then"); then != "" {
		query.Set("then", req.URL.Query().Get("then"))
	}
	uri.RawQuery = query.Encode()
	http.Redirect(w, req, uri.String(), http.StatusTemporaryRedirect)
}

func getBaseURL(req *http.Request) (*url.URL, error) {
	uri, err := url.Parse(req.RequestURI)
	if err != nil {
		return nil, err
	}
	uri.Scheme, uri.Host, uri.RawQuery, uri.Fragment = req.URL.Scheme, req.URL.Host, "", ""
	return uri, nil
}
