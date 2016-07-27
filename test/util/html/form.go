package html

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

func visit(n *html.Node, visitor func(*html.Node)) {
	visitor(n)
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		visit(c, visitor)
	}
}

func GetElementsByTagName(root *html.Node, tagName string) []*html.Node {
	elements := []*html.Node{}
	visit(root, func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == tagName {
			elements = append(elements, n)
		}
	})
	return elements
}

func GetAttr(element *html.Node, attrName string) (string, bool) {
	for _, attr := range element.Attr {
		if attr.Key == attrName {
			return attr.Val, true
		}
	}
	return "", false
}

// NewRequestFromForm builds a request that simulates submitting the given form. It honors:
// Form method (defaults to GET)
// Form action (defaults to currentURL if missing, or currentURL's scheme/host if server-relative)
// <input name="..." value="..."> values (only the first type="submit" input's value is included)
func NewRequestFromForm(form *html.Node, currentURL *url.URL) (*http.Request, error) {
	var (
		reqMethod string
		reqURL    *url.URL
		reqBody   io.Reader
		reqHeader http.Header = http.Header{}
		err       error
	)

	// Method defaults to GET if empty
	if method, _ := GetAttr(form, "method"); len(method) > 0 {
		reqMethod = strings.ToUpper(method)
	} else {
		reqMethod = "GET"
	}

	// URL defaults to current URL if empty
	if action, _ := GetAttr(form, "action"); len(action) > 0 {
		reqURL, err = url.Parse(action)
		if err != nil {
			return nil, err
		}
		if reqURL.Scheme == "" {
			reqURL.Scheme = currentURL.Scheme
		}
		if reqURL.Host == "" {
			reqURL.Host = currentURL.Host
		}
	} else {
		reqURL, err = url.Parse(currentURL.String())
		if err != nil {
			return nil, err
		}
	}

	formData := url.Values{}
	if reqMethod == "GET" {
		// Start with any existing query params when we're submitting via GET
		formData = reqURL.Query()
	}
	addedSubmit := false
	for _, input := range GetElementsByTagName(form, "input") {
		if name, ok := GetAttr(input, "name"); ok {
			if value, ok := GetAttr(input, "value"); ok {
				// Check if this is a submit input
				if inputType, _ := GetAttr(input, "type"); inputType == "submit" {
					// Only add the value of the first one.
					// We're simulating submitting the form.
					if addedSubmit {
						continue
					}
					// Remember we've added a submit input
					addedSubmit = true
				}
				formData.Add(name, value)
			}
		}
	}

	switch reqMethod {
	case "GET":
		reqURL.RawQuery = formData.Encode()
	case "POST":
		reqHeader.Set("Content-Type", "application/x-www-form-urlencoded")
		reqBody = strings.NewReader(formData.Encode())
	default:
		return nil, fmt.Errorf("unknown method: %s", reqMethod)
	}

	req, err := http.NewRequest(reqMethod, reqURL.String(), reqBody)
	if err != nil {
		return nil, err
	}
	req.Header = reqHeader
	return req, nil

}
