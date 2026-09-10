package prometheus

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	prometheusv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/model"
)

var camelCaseRe = regexp.MustCompile(`[a-z][A-Z]`)

// MetadataToMetricFamilies converts Prometheus API metadata into a slice of
// MetricFamily suitable for promlint validation.
func MetadataToMetricFamilies(metadata map[string][]prometheusv1.Metadata) []*dto.MetricFamily {
	var families []*dto.MetricFamily

	for name, entries := range metadata {
		if len(entries) == 0 {
			continue
		}
		// Recording rules use colons in their names and are not subject to
		// metric naming conventions.
		if strings.Contains(name, ":") {
			continue
		}

		entry := entries[0]
		name := name
		help := entry.Help
		metricType := convertMetricType(entry.Type)
		families = append(families, &dto.MetricFamily{
			Name:   &name,
			Help:   &help,
			Type:   &metricType,
			Metric: []*dto.Metric{{}},
		})
	}

	return families
}

// SetCamelCaseLabels attaches camelCase label pairs to the corresponding
// metric families so that promlint's LintCamelCase validation can detect them.
func SetCamelCaseLabels(families []*dto.MetricFamily, labelsPerMetric map[string][]*dto.LabelPair) {
	for i, family := range families {
		if len(families[i].Metric) == 0 {
			continue
		}
		labels, ok := labelsPerMetric[family.GetName()]
		if ok {
			families[i].Metric[0].Label = labels
		}
	}
}

// FindCamelCaseLabels queries Prometheus for all label names that contain
// camelCase, then for each such label, determines which metrics use it.
func FindCamelCaseLabels(ctx context.Context, promClient prometheusv1.API) (map[string][]*dto.LabelPair, error) {
	labelNames, _, err := promClient.LabelNames(ctx, nil, time.Time{}, time.Time{})
	if err != nil {
		return nil, fmt.Errorf("failed to get label names: %w", err)
	}

	var camelCaseLabels []string
	for _, label := range labelNames {
		if camelCaseRe.MatchString(label) {
			camelCaseLabels = append(camelCaseLabels, label)
		}
	}

	if len(camelCaseLabels) == 0 {
		return nil, nil
	}

	result := make(map[string][]*dto.LabelPair)
	for _, label := range camelCaseLabels {
		query := fmt.Sprintf(`count({__name__=~".+",%s=~".+"}) by (__name__)`, label)
		val, _, err := promClient.Query(ctx, query, time.Time{})
		if err != nil {
			return nil, fmt.Errorf("failed to query metrics for label %q: %w", label, err)
		}

		vector, ok := val.(model.Vector)
		if !ok {
			continue
		}
		for _, sample := range vector {
			metricName := string(sample.Metric[model.MetricNameLabel])
			if metricName == "" {
				continue
			}
			lbl := label
			emptyVal := ""
			result[metricName] = append(result[metricName], &dto.LabelPair{
				Name:  &lbl,
				Value: &emptyVal,
			})
		}
	}

	return result, nil
}

func convertMetricType(mt prometheusv1.MetricType) dto.MetricType {
	switch mt {
	case prometheusv1.MetricTypeCounter:
		return dto.MetricType_COUNTER
	case prometheusv1.MetricTypeGauge:
		return dto.MetricType_GAUGE
	case prometheusv1.MetricTypeHistogram:
		return dto.MetricType_HISTOGRAM
	case prometheusv1.MetricTypeSummary:
		return dto.MetricType_SUMMARY
	default:
		return dto.MetricType_UNTYPED
	}
}
