package sampler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/openshift/origin/pkg/disruption/backend"
	"github.com/openshift/origin/pkg/disruption/backend/roundtripper"
	"github.com/openshift/origin/pkg/disruption/sampler"
)

func TestProducer(t *testing.T) {
	ts := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-OpenShift-Disruption", "shutdown=false shutdown-delay-duration=1m10s elapsed=0s host=foo")
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

	wantAgent := "test"
	client := roundtripper.WrapClient(ts.Client(), 0, wantAgent, true, nil, "")
	var producer sampler.Producer
	producer = NewSampleProducerConsumer(client, NewHostPathRequestor(ts.URL, "/echo"), NewResponseChecker(), nil)
	info, err := producer.Produce(context.TODO(), 1)

	if err != nil {
		t.Errorf("expected no error, but got: %v", err)
	}
	reqRespInfoGot, ok := info.(backend.RequestResponse)
	if !ok {
		t.Errorf("expected an object of %T", backend.RequestResponse{})
		return
	}

	if reqRespInfoGot.Request == nil {
		t.Errorf("expected the HTTP request sent, but got nil")
		return
	}
	reqGot := reqRespInfoGot.Request
	if got := reqGot.Header.Get("User-Agent"); wantAgent != got {
		t.Errorf("expected User-Agent: %s, but got: %s", wantAgent, got)
	}

	if reqRespInfoGot.Response == nil {
		t.Errorf("expected an HTTP Respone, but got nil")
		return
	}
	respGot := reqRespInfoGot.Response
	if respGot.Proto != "HTTP/2.0" {
		t.Errorf("expected protocl to be HTTP/2.0 but got: %s", respGot.Proto)
	}

	if reqRespInfoGot.GotConnInfo == nil {
		t.Errorf("expected a GotConnInfo object, but got nil")
		return
	}
	gotConnInfo := reqRespInfoGot.GotConnInfo
	if gotConnInfo.Reused {
		t.Errorf("expected the first request to use a new connection")
	}
	if len(gotConnInfo.RemoteAddr) == 0 {
		t.Errorf("expected remote address to be set")
	}

	if got := reqRespInfoGot.ShutdownResponse; got == nil {
		t.Errorf("expected ShutdownResponse to be set")
	}

	info, err = producer.Produce(context.TODO(), 2)
	if err != nil {
		t.Errorf("expected no error, but got: %v", err)
	}
	reqRespInfoGot, ok = info.(backend.RequestResponse)
	if !ok {
		t.Errorf("expected an object of %T", backend.RequestResponse{})
		return
	}
	if reqRespInfoGot.GotConnInfo == nil {
		t.Errorf("expected a GotConnInfo object, but got nil")
		return
	}
	gotConnInfo = reqRespInfoGot.GotConnInfo
	if !gotConnInfo.Reused {
		t.Errorf("expected the second request to use an existing connection")
	}
}
