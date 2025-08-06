package util

import (
	"context"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// ClientCurl makes an HTTP request with authentication and optional proxy support
func ClientCurl(oc *CLI, method string, tokenValue string, targetURL string) (string, error) {
	// Use shorter timeout for faster failure detection
	timeoutDuration := 10 * time.Second
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
		// Add connection pooling settings for better performance in proxy environments
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		// Add timeouts for better reliability in proxy environments
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
		ExpectContinueTimeout: 10 * time.Second,
		// Use system proxy settings as fallback
		Proxy: http.ProxyFromEnvironment,
	}

	// Try to get global proxy configuration from cluster
	httpProxy, httpsProxy, noProxy, err := GetGlobalProxy(oc)
	if err == nil && (httpsProxy != "" || httpProxy != "") {
		e2e.Logf("Using cluster proxy configuration: HTTP_PROXY=%s, HTTPS_PROXY=%s, NO_PROXY=%s", httpProxy, httpsProxy, noProxy)
		proxyURLString := httpsProxy
		if proxyURLString == "" {
			proxyURLString = httpProxy
		}

		proxyURL, err := url.Parse(proxyURLString)
		if err == nil {
			transport.Proxy = func(req *http.Request) (*url.URL, error) {
				// Check if the target should bypass proxy
				if shouldBypassProxy(req.URL.Host, noProxy) {
					e2e.Logf("Bypassing proxy for host: %s", req.URL.Host)
					return nil, nil
				}
				e2e.Logf("Using proxy %s for host: %s", proxyURL, req.URL.Host)
				return proxyURL, nil
			}
		}
	} else {
		// Fallback to environment variables if cluster proxy config is not available
		envHTTPProxy := os.Getenv("HTTP_PROXY")
		envHTTPSProxy := os.Getenv("HTTPS_PROXY")
		envNoProxy := os.Getenv("NO_PROXY")

		e2e.Logf("Using environment proxy configuration: HTTP_PROXY=%s, HTTPS_PROXY=%s, NO_PROXY=%s", envHTTPProxy, envHTTPSProxy, envNoProxy)

		if envHTTPSProxy != "" || envHTTPProxy != "" {
			proxyURLString := envHTTPSProxy
			if proxyURLString == "" {
				proxyURLString = envHTTPProxy
			}

			proxyURL, err := url.Parse(proxyURLString)
			if err == nil {
				transport.Proxy = func(req *http.Request) (*url.URL, error) {
					// Check if the target should bypass proxy
					if shouldBypassProxy(req.URL.Host, envNoProxy) {
						e2e.Logf("Bypassing proxy for host: %s", req.URL.Host)
						return nil, nil
					}
					e2e.Logf("Using proxy %s for host: %s", proxyURL, req.URL.Host)
					return proxyURL, nil
				}
			}
		} else {
			e2e.Logf("No proxy configuration found, using direct connection")
		}
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   timeoutDuration,
	}

	e2e.Logf("Making HTTP request to: %s with timeout: %v", targetURL, timeoutDuration)
	errCurl := wait.PollUntilContextTimeout(context.Background(), 5*time.Second, 60*time.Second, true, func(ctx context.Context) (bool, error) {
		resp, err := client.Do(req)
		if err != nil {
			// Log the error for debugging but don't fail immediately
			e2e.Logf("HTTP request failed: %v, retrying...", err)
			return false, nil
		}
		defer resp.Body.Close()

		if resp.StatusCode == 200 {
			bodyBytes, _ := ioutil.ReadAll(resp.Body)
			bodyString = string(bodyBytes)
			e2e.Logf("HTTP request successful, response length: %d", len(bodyString))
			return true, nil
		}
		// Log non-200 status codes for debugging
		e2e.Logf("HTTP request returned status %d, retrying...", resp.StatusCode)
		return false, nil
	})

	if errCurl != nil {
		e2e.Logf("Proxy-based request failed, trying direct connection as fallback")

		// Try with direct connection (no proxy) as fallback
		directTransport := &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   10,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 15 * time.Second,
			ExpectContinueTimeout: 10 * time.Second,
			// No proxy for direct connection
		}

		directClient := &http.Client{
			Transport: directTransport,
			Timeout:   timeoutDuration,
		}

		e2e.Logf("Making direct HTTP request to: %s", targetURL)
		directErr := wait.PollUntilContextTimeout(context.Background(), 5*time.Second, 60*time.Second, true, func(ctx context.Context) (bool, error) {
			resp, err := directClient.Do(req)
			if err != nil {
				e2e.Logf("Direct HTTP request failed: %v, retrying...", err)
				return false, nil
			}
			defer resp.Body.Close()

			if resp.StatusCode == 200 {
				bodyBytes, _ := ioutil.ReadAll(resp.Body)
				bodyString = string(bodyBytes)
				e2e.Logf("Direct HTTP request successful, response length: %d", len(bodyString))
				return true, nil
			}
			e2e.Logf("Direct HTTP request returned status %d, retrying...", resp.StatusCode)
			return false, nil
		})

		if directErr != nil {
			return "", fmt.Errorf("both proxy and direct connection failed. Proxy error: %v, Direct error: %v", errCurl, directErr)
		}
	}
	return bodyString, nil
}

// shouldBypassProxy checks if the given host should bypass the proxy based on noProxy configuration
func shouldBypassProxy(host, noProxy string) bool {
	// Always bypass proxy for localhost and internal cluster addresses
	if strings.Contains(host, "localhost") ||
		strings.Contains(host, "127.0.0.1") ||
		strings.Contains(host, "::1") ||
		strings.Contains(host, ".cluster.local") ||
		strings.Contains(host, ".svc.") ||
		strings.Contains(host, ".apps.") {
		return true
	}

	if noProxy == "" {
		return false
	}

	// Remove port from host for comparison
	hostWithoutPort := host
	if colonIndex := strings.Index(host, ":"); colonIndex != -1 {
		hostWithoutPort = host[:colonIndex]
	}

	// Check each noProxy entry
	for _, entry := range strings.Split(noProxy, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		// Check for exact match
		if entry == hostWithoutPort {
			return true
		}

		// Check for wildcard match (e.g., *.example.com)
		if strings.HasPrefix(entry, "*.") {
			domain := entry[2:] // Remove "*. "
			if strings.HasSuffix(hostWithoutPort, "."+domain) || hostWithoutPort == domain {
				return true
			}
		}

		// Check for IP range (CIDR notation)
		if strings.Contains(entry, "/") {
			// Simple IP range check - in a real implementation, you'd want proper CIDR parsing
			if strings.HasPrefix(hostWithoutPort, entry[:strings.Index(entry, "/")]) {
				return true
			}
		}
	}

	return false
}
