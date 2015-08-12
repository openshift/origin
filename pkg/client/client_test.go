package client

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	kclient "k8s.io/kubernetes/pkg/client"
)

func TestUserAgent(t *testing.T) {
	ch := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		header := req.Header.Get("User-Agent")
		ch <- header
	}))
	defer server.Close()

	c, _ := New(&kclient.Config{
		Host: server.URL,
	})
	c.DeploymentConfigs("test").Get("other")

	header := <-ch
	if !strings.Contains(header, "openshift/") || !strings.Contains(header, "client.test/") {
		t.Fatalf("no user agent header: %s", header)
	}
}
