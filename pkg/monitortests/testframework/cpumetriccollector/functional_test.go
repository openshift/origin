package cpumetriccollector

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
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// TestCollectCPUMetrics_Functional calls collectCPUMetricsFromPrometheusClient
// against KaaS and PromeCIeus servers from captured CI job data.
// Doesn't test much but useful for debugging with actual data.
//
// Required env vars:
//
//	PROMETHEUS_URL   - URL of the Prometheus server
//	KUBECONFIG       - kubeconfig for node listing
//	CPU_START_TIME   - RFC3339 start time for the query range
func TestCollectCPUMetrics_Functional(t *testing.T) {
	promURL := os.Getenv("PROMETHEUS_URL")
	if promURL == "" {
		t.Skip("PROMETHEUS_URL not set")
	}
	kubeconfigPath := os.Getenv("KUBECONFIG")
	if kubeconfigPath == "" {
		t.Skip("KUBECONFIG not set")
	}
	startStr := os.Getenv("CPU_START_TIME")
	if startStr == "" {
		t.Skip("CPU_START_TIME not set")
	}

	startTime, err := time.Parse(time.RFC3339, startStr)
	require.NoError(t, err, "CPU_START_TIME must be RFC3339")

	ctx := context.Background()

	restConfig, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	require.NoError(t, err)
	kubeClient, err := kubernetes.NewForConfig(restConfig)
	require.NoError(t, err)

	promClient, err := prometheusapi.NewClient(prometheusapi.Config{
		Address: promURL,
		Client: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
	})
	require.NoError(t, err)
	promAPI := prometheusv1.NewAPI(promClient)

	collector := &cpuMetricCollector{highCPUThreshold: 95.0}
	intervals, err := collector.collectCPUMetricsFromPrometheusClient(ctx, promAPI, kubeClient, startTime)
	require.NoError(t, err)

	t.Logf("high-cpu intervals: %d, data points: %d", len(intervals), len(collector.cpuDataPoints))
	require.NotEmpty(t, collector.cpuDataPoints, "expected some cpu data points")
}
