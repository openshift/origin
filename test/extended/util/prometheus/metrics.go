package prometheus

import (
	"encoding/json"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	dto "github.com/prometheus/client_model/go"
)

// SeriesToMetricFamilies takes Prometheus /api/v1/metadata and /api/v1/series
// queries output and creates an array of MetricFamily out of them.
func SeriesToMetricFamilies(rawMetadata, rawSeries []byte) ([]*dto.MetricFamily, error) {
	var (
		metadatas map[string][]struct {
			Help string        `json:"help"`
			Type v1.MetricType `json:"type"`
		}
		series   []map[string]string
		families []*dto.MetricFamily
	)

	err := unmarshalJSONData(rawMetadata, &metadatas)
	if err != nil {
		return nil, err
	}

	err = unmarshalJSONData(rawSeries, &series)
	if err != nil {
		return nil, err
	}

	familySet := make(map[string]*dto.MetricFamily)
	for _, serie := range series {
		name := serie["__name__"]
		for _, metadata := range metadatas[name] {
			metricType := convertToDTOMetricType(metadata.Type)
			metric := newMetric(metricType, serie)

			_, ok := familySet[name]
			if ok {
				familySet[name].Metric = append(familySet[name].Metric, metric)
			} else {
				familySet[name] = &dto.MetricFamily{
					Name:   &name,
					Help:   &metadata.Help,
					Type:   &metricType,
					Metric: []*dto.Metric{metric},
				}
			}
		}
	}

	for _, f := range familySet {
		families = append(families, f)
	}

	return families, nil
}

func unmarshalJSONData(data []byte, v interface{}) error {
	var j map[string]json.RawMessage
	err := json.Unmarshal(data, &j)
	if err != nil {
		return err
	}
	return json.Unmarshal(j["data"], &v)
}

func convertToDTOMetricType(metricType v1.MetricType) dto.MetricType {
	switch metricType {
	case v1.MetricTypeCounter:
		return dto.MetricType_COUNTER
	case v1.MetricTypeGauge:
		return dto.MetricType_GAUGE
	case v1.MetricTypeHistogram:
		return dto.MetricType_HISTOGRAM
	case v1.MetricTypeSummary:
		return dto.MetricType_SUMMARY
	default:
		return dto.MetricType_UNTYPED
	}
}

func newMetric(metricType dto.MetricType, labels map[string]string) *dto.Metric {
	var metric dto.Metric
	for n, v := range labels {
		metric.Label = append(metric.Label, createLabelPair(n, v))
	}
	return &metric
}

func createLabelPair(name string, value string) *dto.LabelPair {
	return &dto.LabelPair{
		Name:  &name,
		Value: &value,
	}
}
