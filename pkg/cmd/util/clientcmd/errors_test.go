package clientcmd

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIsCertificateAuthorityUnknown(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer server.Close()

	req, _ := http.NewRequest("GET", server.URL, nil)
	_, err := http.DefaultClient.Do(req)
	if err == nil {
		t.Fatalf("Expected TLS error")
	}
	if !IsCertificateAuthorityUnknown(err) {
		t.Fatalf("Expected IsCertificateAuthorityUnknown error, error message was %q", err.Error())
	}
}
