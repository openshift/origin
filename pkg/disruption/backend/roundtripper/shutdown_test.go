package roundtripper

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/openshift/origin/pkg/disruption/backend"
)

func TestWithShutdownResponseHeaderExtractor(t *testing.T) {
	tests := []struct {
		name     string
		header   string
		expected *backend.RequestContextAssociatedData
	}{
		{
			name:   "missing response header",
			header: "",
			expected: &backend.RequestContextAssociatedData{
				ShutdownResponseHeaderParseErr: fmt.Errorf("expected X-Openshift-Disruption response header"),
			},
		},
		{
			name:   "shutdown not in progress",
			header: "shutdown=false shutdown-delay-duration=1m10s elapsed=0s host=foo",
			expected: &backend.RequestContextAssociatedData{
				ShutdownResponse: &backend.ShutdownResponse{
					ShutdownInProgress:    false,
					ShutdownDelayDuration: 70 * time.Second,
					Elapsed:               time.Duration(0),
					Hostname:              "foo",
				},
			},
		},
		{
			name:   "shutdown not in progress",
			header: "shutdown=true shutdown-delay-duration=1m10s elapsed=10s host=foo",
			expected: &backend.RequestContextAssociatedData{
				ShutdownResponse: &backend.ShutdownResponse{
					ShutdownInProgress:    true,
					ShutdownDelayDuration: 70 * time.Second,
					Elapsed:               10 * time.Second,
					Hostname:              "foo",
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ts := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if h := r.Header.Get("X-Openshift-If-Disruption"); h != "true" {
					t.Errorf("expected request to have X-Openshift-If-Disruption header: %q", h)
				}

				if len(test.header) > 0 {
					w.Header().Set("X-Openshift-Disruption", test.header)
				}
				w.WriteHeader(http.StatusOK)
			}))
			ts.EnableHTTP2 = true
			ts.StartTLS()
			defer ts.Close()

			decoder := &fakeDecoder{}
			client := WithShutdownResponseHeaderExtractor(ts.Client(), decoder)

			scoped := &backend.RequestContextAssociatedData{}
			ctx := backend.WithRequestContextAssociatedData(context.Background(), scoped)
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/echo", nil)
			if err != nil {
				t.Fatalf("failed to create a new HTTP request")
			}

			resp, err := client.Do(req)
			if resp.StatusCode != http.StatusOK {
				t.Errorf("expected status code: %d, but got: %d", http.StatusOK, resp.StatusCode)
			}
			if !reflect.DeepEqual(test.expected, scoped) {
				t.Errorf("expected a match - diff: %s", cmp.Diff(test.expected, scoped))
			}

			if sr := test.expected.ShutdownResponse; sr != nil {
				if sr.Hostname != decoder.encoded {
					t.Errorf("expected the hostname to match, want:%s, got:%s", sr.Hostname, decoder.encoded)
				}
			}
		})
	}
}

type fakeDecoder struct {
	encoded string
}

func (f *fakeDecoder) Decode(s string) string {
	f.encoded = s
	return s
}
