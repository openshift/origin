package memorythrashingcollector

import (
	"context"
	"crypto/tls"
	"net/http"
	"os"
	"testing"
	"time"

	prometheusapi "github.com/prometheus/client_golang/api"
	prometheusv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/stretchr/testify/require"
)

// TestCollectMemoryThrashing_Functional runs the collection against a real Prometheus, such
// as a CI job's Prometheus snapshot loaded locally. Doesn't test much but useful for checking
// the threshold against actual data. Node roles are not resolved since no cluster is needed.
//
// Required env vars:
//
//	PROMETHEUS_URL - URL of the Prometheus server
//	START_TIME     - RFC3339 start of the query range
//	END_TIME       - RFC3339 end of the query range
func TestCollectMemoryThrashing_Functional(t *testing.T) {
	promURL := os.Getenv("PROMETHEUS_URL")
	if promURL == "" {
		t.Skip("PROMETHEUS_URL not set")
	}
	startTime, err := time.Parse(time.RFC3339, os.Getenv("START_TIME"))
	require.NoError(t, err, "START_TIME must be RFC3339")
	endTime, err := time.Parse(time.RFC3339, os.Getenv("END_TIME"))
	require.NoError(t, err, "END_TIME must be RFC3339")

	promClient, err := prometheusapi.NewClient(prometheusapi.Config{
		Address: promURL,
		Client: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
	})
	require.NoError(t, err)

	collector := &memoryThrashingCollector{majorFaultsThreshold: 50}
	intervals, err := collector.collectFromPrometheusClient(context.Background(), prometheusv1.NewAPI(promClient), map[string]nodeInfo{}, startTime, endTime)
	require.NoError(t, err)

	for _, interval := range intervals {
		t.Logf("%s - %s %s peak faults/s=%s peak page-in MB/s=%s", interval.From.Format(time.RFC3339), interval.To.Format(time.RFC3339),
			interval.Locator.OldLocator(), interval.Message.Annotations["peak_major_faults_per_second"], interval.Message.Annotations["peak_page_in_mb_per_second"])
	}
	t.Logf("memory thrashing intervals: %d", len(intervals))
}
