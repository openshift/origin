package roundtripper

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/openshift/origin/pkg/disruption/backend"
)

func TestWrapClient(t *testing.T) {
	var userAgentGot, protoGot string
	ts := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userAgentGot = r.Header.Get("User-Agent")
		protoGot = r.Proto
		w.WriteHeader(http.StatusOK)
	}))
	ts.EnableHTTP2 = true
	ts.StartTLS()
	defer ts.Close()

	transport, ok := ts.Client().Transport.(*http.Transport)
	if !ok {
		t.Fatalf("expected an object of %T", &http.Transport{})
	}
	transport.DisableKeepAlives = false

	agent := "my-client"
	client := WrapClient(ts.Client(), 0, agent, false, nil, "")

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/echo", nil)
	if err != nil {
		t.Fatalf("failed to create a new HTTP request")
	}
	resp, err := client.Do(req)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status code: %d, but got: %d", http.StatusOK, resp.StatusCode)
	}
	if userAgentGot != agent {
		t.Errorf("expected User-Agent: %s, but got: %s", agent, userAgentGot)
	}
	if protoGot != "HTTP/2.0" {
		t.Errorf("expected protocol to be HTTP/2.0 but got: %s", protoGot)
	}

	// connection reuse should be false for the first request, so let's try again
	req, err = http.NewRequest(http.MethodGet, ts.URL+"/echo", nil)
	if err != nil {
		t.Fatalf("failed to create a new HTTP request")
	}
	req = req.WithContext(backend.WithRequestContextAssociatedData(req.Context(), &backend.RequestContextAssociatedData{}))
	resp, err = client.Do(req)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status code: %d, but got: %d", http.StatusOK, resp.StatusCode)
	}

	infoGot := backend.RequestContextAssociatedDataFrom(req.Context())
	if infoGot == nil {
		t.Errorf("expected an object of type: %T in the request context", &backend.RequestContextAssociatedData{})
		return
	}
	if infoGot.GotConnInfo == nil {
		t.Errorf("expected a non nil %T ", backend.GotConnInfo{})
		return
	}

	if !infoGot.GotConnInfo.Reused {
		t.Errorf("expected connection to be reused")
	}
	if len(infoGot.GotConnInfo.RemoteAddr) == 0 {
		t.Errorf("expected remote address to be set")
	}
}
