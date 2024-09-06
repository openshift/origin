package apiunreachablefromclientmetrics

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/openshift/origin/pkg/monitortests/metrics"

	prometheustypes "github.com/prometheus/common/model"
)

func TestAPIUnreachableMonitor(t *testing.T) {
	fname := filepath.Join("testdata", "rest_client_metrics.json")
	bytes, err := os.ReadFile(fname)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	test := monitorTest{
		queryAnalyzers: []queryAnalyzer{
			{
				query:    byteQuery(bytes),
				analyzer: metrics.RateSeriesAnalyzer{},
			},
		},
		callback: &apiUnreachableCallback{
			resolver: &clusterInfoResolver{
				serviceNetworkIP: "172.30.0.1",
			},
		},
	}

	intervals, _, err := test.CollectData(context.Background(), "", time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("unexpected error from CollectData: %v", err)
	}

	// the file is a true prometheus dump from CI
	// we expect two disruption intervals from these sources
	//  a) internal-lb
	//  b) webhook.openshift-console-operator.svc:9443
	if want, got := 2, len(intervals); want != got {
		t.Errorf("expected %d intervals, but got: %d", want, got)
	}

	for i, want := range []string{"internal-lb", "webhook.openshift-console-operator.svc:9443"} {
		interval := intervals[i]
		t.Logf("disruption interval: %+v", interval)

		if want, got := "APIUnreachableFromClient", string(interval.Source); want != got {
			t.Errorf("expected interval source: %q, got: %q", want, got)
		}
		if got := interval.Locator.Keys["host"]; want != got {
			t.Errorf("expected source: %q, but got: %q", want, got)
		}
		if duration := interval.To.Sub(interval.From); duration <= 0 {
			t.Errorf("expected duration to be positive, got: %s", duration)
		}
	}
}

type byteQuery []byte

func (q byteQuery) RunQuery(ctx context.Context, start, end time.Time) (prometheustypes.Value, error) {
	var matrix prometheustypes.Matrix
	if err := json.Unmarshal([]byte(q), &matrix); err != nil {
		return nil, err
	}
	return matrix, nil
}
