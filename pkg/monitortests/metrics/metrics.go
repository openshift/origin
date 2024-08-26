package metrics

import (
	"context"
	"fmt"
	"strings"
	"time"

	"k8s.io/kubernetes/test/e2e/framework"

	prometheusv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	prometheustypes "github.com/prometheus/common/model"
)

type Callback interface {
	// Name identifies the monitor test
	Name() string

	// StartSeries is invoked when the metrics analyzer discovers a new series
	StartSeries(metric prometheustypes.Metric)

	// NewInterval is invoked when the metrics analyzer discovers a new interval
	NewInterval(metric prometheustypes.Metric, from, to *prometheustypes.SamplePair)

	// EndSeries is invoked when the metrics reaches the end of the given series
	EndSeries()
}

// invokes a prometheus query and returns the results
type QueryRunner interface {
	RunQuery(ctx context.Context, from, to time.Time) (prometheustypes.Value, error)
}

// Analyzes a prometheus series
type SeriesAnalyzer interface {
	Analyze(ctx context.Context, query QueryRunner, start, end time.Time, callback Callback) error
}

// default prometheus query runner in use
type PrometheusQueryRunner struct {
	Client      prometheusv1.API
	QueryString string
	Step        time.Duration
}

func (q *PrometheusQueryRunner) RunQuery(ctx context.Context, from, to time.Time) (prometheustypes.Value, error) {
	result, warnings, err := q.Client.QueryRange(ctx, q.QueryString, prometheusv1.Range{
		Start: from,
		End:   to,
		Step:  q.Step,
	})
	if err != nil {
		return nil, fmt.Errorf("prometheus query %q returned error: %v", q.QueryString, err)
	}
	if len(warnings) > 0 {
		framework.Logf("query %q #### warnings \n\t%v\n", q.QueryString, strings.Join(warnings, "\n\t"))
	}

	return result, nil
}
