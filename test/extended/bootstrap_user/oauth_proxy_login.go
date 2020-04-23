package bootstrap_user

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

func newHTTPSClient(transport *http.RoundTripper) (*http.Client, error) {
	jar, _ := cookiejar.New(nil)

	client := &http.Client{Transport: *transport, Jar: jar}
	return client, nil
}

func testOAuthProxyLogin(transport *http.RoundTripper, host, user, password string) error {
	// Set up the client cert store
	client, err := newHTTPSClient(transport)
	if err != nil {
		return err
	}

	hostURL := "https://" + host
	baseResp, err := client.Get(hostURL)
	if err != nil {
		return fmt.Errorf("failed to retrieve the base page")
	}
	defer baseResp.Body.Close()
	// we should not be authenticated at this point
	if baseResp.StatusCode != 403 {
		r, _ := ioutil.ReadAll(baseResp.Body)
		return fmt.Errorf("expected to be unauthenticated, got status %q, page:\n%s", baseResp.Status, r)
	}

	if err := confirmOAuthFlow(client, hostURL, user, password); err != nil {
		return fmt.Errorf("failed to perform web oauth flow: %v", err)
	}

	authenticateResp, err := client.Get(hostURL)
	if err != nil {
		return fmt.Errorf("failed to retrieve the base page")
	}
	defer authenticateResp.Body.Close()
	// we should be authenticated now
	if authenticateResp.StatusCode != 200 {
		r, _ := ioutil.ReadAll(authenticateResp.Body)
		return fmt.Errorf("expected to be authenticated, got status %q, page:\n%s", baseResp.Status, r)
	}

	return nil
}

func confirmOAuthFlow(client *http.Client, hostURL, user, password string) error {
	// Bypass the "Login to OpenShift" button by directly going to /oauth/start
	resp, err := client.Get(hostURL + "/oauth/start")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		r, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("expected to be redirected to the oauth-server login page, got %q; page content\n%s", resp.Status, r)
	}

	// OpenShift login page
	loginResp, err := submitOAuthForm(client, resp, user, password)
	if err != nil {
		return err
	}
	defer loginResp.Body.Close()
	if resp.StatusCode != 200 {
		r, _ := ioutil.ReadAll(loginResp.Body)
		return fmt.Errorf("failed to submit the login form: %q\n pageC content\n%s", resp.Status, r)
	}

	// authorization grant form; no password should be expected
	grantResp, err := submitOAuthForm(client, loginResp, user, "")
	if err != nil {
		return err
	}
	defer grantResp.Body.Close()
	if resp.StatusCode != 200 {
		r, _ := ioutil.ReadAll(grantResp.Body)
		return fmt.Errorf("failed to submit the grant form: %q\n pageC content\n%s", resp.Status, r)
	}

	return nil
}

func submitOAuthForm(client *http.Client, response *http.Response, user, password string) (*http.Response, error) {
	bodyParsed, err := html.Parse(response.Body)
	if err != nil {
		return nil, err
	}

	forms := getElementsByTagName(bodyParsed, "form")
	if len(forms) != 1 {
		return nil, fmt.Errorf("expected a single OpenShift form")
	}

	formReq, err := newRequestFromForm(forms[0], response.Request.URL, user, password)
	if err != nil {
		return nil, err
	}

	postResp, err := client.Do(formReq)
	if err != nil {
		return nil, err
	}

	return postResp, nil
}

func getElementsByTagName(root *html.Node, tagName string) []*html.Node {
	elements := []*html.Node{}
	visit(root, func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == tagName {
			elements = append(elements, n)
		}
	})
	return elements
}

func getTextNodes(root *html.Node) []*html.Node {
	elements := []*html.Node{}
	visit(root, func(n *html.Node) {
		if n.Type == html.TextNode {
			elements = append(elements, n)
		}
	})
	return elements
}

func visit(n *html.Node, visitor func(*html.Node)) {
	visitor(n)
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		visit(c, visitor)
	}
}

// newRequestFromForm builds a request that simulates submitting the given form.
func newRequestFromForm(form *html.Node, currentURL *url.URL, user, password string) (*http.Request, error) {
	var (
		reqMethod string
		reqURL    *url.URL
		reqBody   io.Reader
		reqHeader = http.Header{}
		err       error
	)

	// Method defaults to GET if empty
	if method, _ := getAttr(form, "method"); len(method) > 0 {
		reqMethod = strings.ToUpper(method)
	} else {
		reqMethod = "GET"
	}

	// URL defaults to current URL if empty
	action, _ := getAttr(form, "action")
	reqURL, err = currentURL.Parse(action)
	if err != nil {
		return nil, err
	}

	formData := url.Values{}
	if reqMethod == "GET" {
		// Start with any existing query params when we're submitting via GET
		formData = reqURL.Query()
	}
	addedSubmit := false
	for _, input := range getElementsByTagName(form, "input") {
		if name, ok := getAttr(input, "name"); ok {
			if value, ok := getAttr(input, "value"); ok {
				inputType, _ := getAttr(input, "type")

				switch inputType {
				case "text":
					if name == "username" {
						formData.Add(name, user)
					}
				case "password":
					if name == "password" {
						formData.Add(name, password)
					}
				case "submit":
					// If this is a submit input, only add the value of the first one.
					// We're simulating submitting the form.
					if !addedSubmit {
						formData.Add(name, value)
						addedSubmit = true
					}
				case "radio", "checkbox":
					if _, checked := getAttr(input, "checked"); checked {
						formData.Add(name, value)
					}
				default:
					formData.Add(name, value)
				}
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

func getAttr(element *html.Node, attrName string) (string, bool) {
	for _, attr := range element.Attr {
		if attr.Key == attrName {
			return attr.Val, true
		}
	}
	return "", false
}
