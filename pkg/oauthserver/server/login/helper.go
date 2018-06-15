package login

import (
	"net/http"
	"net/url"
	"strings"
)

func failed(reason string, w http.ResponseWriter, req *http.Request) {
	uri, err := getBaseURL(req)
	if err != nil {
		http.Redirect(w, req, req.URL.Path, http.StatusFound)
		return
	}
	query := url.Values{}
	query.Set("reason", reason)
	if then := req.FormValue("then"); then != "" {
		query.Set("then", then)
	}
	uri.RawQuery = query.Encode()
	http.Redirect(w, req, uri.String(), http.StatusFound)
}

func getBaseURL(req *http.Request) (*url.URL, error) {
	uri, err := url.Parse(req.RequestURI)
	if err != nil {
		return nil, err
	}
	uri.Scheme, uri.Host, uri.RawQuery, uri.Fragment = req.URL.Scheme, req.URL.Host, "", ""
	return uri, nil
}

func postForm(url string, body url.Values) (resp *http.Response, err error) {
	req, err := http.NewRequest("POST", url, strings.NewReader(body.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return http.DefaultTransport.RoundTrip(req)
}

func getURL(url string) (resp *http.Response, err error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	return http.DefaultTransport.RoundTrip(req)
}
