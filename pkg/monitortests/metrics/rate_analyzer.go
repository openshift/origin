package metrics

import (
	"context"
	"fmt"
	"time"

	prometheustypes "github.com/prometheus/common/model"
)

// RateSeriesAnalyzer analyzes a rate based time series, it scans a prometheus
// Matrix type time series, and for each sequence of non-zero values
// discovered, it publishes it as an interval of interest via the given Callback.
type RateSeriesAnalyzer struct{}

func (RateSeriesAnalyzer) Analyze(ctx context.Context, query QueryRunner, start, end time.Time, callback Callback) error {
	result, err := query.RunQuery(ctx, start, end)
	if err != nil {
		return fmt.Errorf("query returned error, monitor: %s, err: %w", callback.Name(), err)
	}
	if result.Type() != prometheustypes.ValMatrix {
		return fmt.Errorf("expected a prometheus Matrix type, but got: %q, monitor: %s", result.Type().String(), callback.Name())
	}
	matrix := result.(prometheustypes.Matrix)

	zero := prometheustypes.SampleValue(0)
	for _, series := range matrix {
		func() {
			callback.StartSeries(series.Metric)
			defer callback.EndSeries()

			var intervalStart, intervalEnd *prometheustypes.SamplePair
			for i := range series.Values {
				current := series.Values[i]
				switch {
				case current.Value.Equal(zero):
					// we have reached a zero:
					//  a) this marks a known interval [intervalStart ... intervalEnd]
					//     the current sample is outside of this interval
					//  b) it's the first element in the series
					if intervalStart != nil {
						callback.NewInterval(series.Metric, intervalStart, intervalEnd)
						intervalStart = nil
					}
				default:
					// beginning of an interval
					if intervalStart == nil {
						intervalStart = &current
					}
				}

				// an open interval could end here
				intervalEnd = &current
			}

			// is the entire range a disruption?
			if intervalStart != nil {
				callback.NewInterval(series.Metric, intervalStart, intervalEnd)
			}
		}()
	}
	return nil
}
