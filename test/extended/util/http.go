package util

import (
	"context"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
)

// ClientCurl makes an HTTP request with authentication and optional proxy support
func ClientCurl(oc *CLI, method string, tokenValue string, targetURL string) (string, error) {
	timeoutDuration := 3 * time.Second
	var bodyString string

	req, err := http.NewRequest(method, targetURL, nil)
	if err != nil {
		return "", fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+tokenValue)
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	// Try to get global proxy configuration from cluster
	httpProxy, httpsProxy, _, err := GetGlobalProxy(oc)
	if err == nil && (httpsProxy != "" || httpProxy != "") {
		proxyURLString := httpsProxy
		if proxyURLString == "" {
			proxyURLString = httpProxy
		}

		proxyURL, err := url.Parse(proxyURLString)
		if err == nil {
			transport.Proxy = http.ProxyURL(proxyURL)
		}
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   timeoutDuration,
	}

	errCurl := wait.PollUntilContextTimeout(context.Background(), 10*time.Second, 300*time.Second, true, func(ctx context.Context) (bool, error) {
		resp, err := client.Do(req)
		if err != nil {
			return false, nil
		}
		defer resp.Body.Close()

		if resp.StatusCode == 200 {
			bodyBytes, _ := ioutil.ReadAll(resp.Body)
			bodyString = string(bodyBytes)
			return true, nil
		}
		return false, nil
	})

	if errCurl != nil {
		return "", fmt.Errorf("error waiting for curl request output: %v", errCurl)
	}
	return bodyString, nil
}
