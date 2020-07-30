package prometheus

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	dto "github.com/prometheus/client_model/go"
)

// CreateMetricFamilies takes Prometheus /api/v1/metadata query result and
// creates an array of MetricFamily out of it.
func CreateMetricFamilies(rawMetadata []byte) ([]*dto.MetricFamily, error) {
	var (
		metadatas map[string][]struct {
			Help string        `json:"help"`
			Type v1.MetricType `json:"type"`
		}
		families []*dto.MetricFamily
	)

	err := unmarshalJSONData(rawMetadata, &metadatas)
	if err != nil {
		return nil, err
	}

	for name := range metadatas {
		name := name
		for _, metadata := range metadatas[name] {
			metricType := convertToDTOMetricType(metadata.Type)
			families = append(families, &dto.MetricFamily{
				Name:   &name,
				Help:   &metadata.Help,
				Type:   &metricType,
				Metric: []*dto.Metric{{}},
			})
		}
	}

	return families, nil
}

// SetMetricsLabels takes an array of MetricFamily and sets its labels
// according to the one stored in the map of labels per metric.
func SetMetricsLabels(families []*dto.MetricFamily, metricsLabels map[string][]*dto.LabelPair) {
	for i, family := range families {
		labels, ok := metricsLabels[*family.Name]
		if ok {
			families[i].Metric[0].Label = labels
		}
	}
}

// GetInvalidLabelsPerMetric gets all the labels through /api/v1/labels
// Prometheus endpoint and returns a map of the invalid ones associated with
// the name of the metric exposing it.
func GetInvalidLabelsPerMetric(ns, execPodName, baseURL, bearerToken string) (map[string][]*dto.LabelPair, error) {
	rawLabels, err := GetBearerTokenURLViaPod(ns, execPodName, fmt.Sprintf("%s/api/v1/labels", baseURL), bearerToken)
	if err != nil {
		return nil, err
	}

	var labels []string
	err = unmarshalJSONData([]byte(rawLabels), &labels)
	if err != nil {
		return nil, err
	}

	var invalidLabels []string
	for _, label := range labels {
		if !isValidLabel(label) {
			invalidLabels = append(invalidLabels, label)
		}
	}

	metricInvalidLabels := make(map[string][]*dto.LabelPair)
	for _, label := range invalidLabels {
		query := fmt.Sprintf(`count({__name__=~".+",%s=~".+"}) by (__name__)`, label)
		res, err := GetBearerTokenURLViaPod(ns, execPodName, fmt.Sprintf("%s/api/v1/query?%s", baseURL, url.Values{"query": []string{query}}.Encode()), bearerToken)
		if err != nil {
			return nil, err
		}

		var data struct {
			Results []struct {
				Metric struct {
					Name string `json:"__name__"`
				} `json:"metric"`
			} `json:"result"`
		}
		err = unmarshalJSONData([]byte(res), &data)
		if err != nil {
			return nil, err
		}

		for _, result := range data.Results {
			name := result.Metric.Name
			metricInvalidLabels[name] = append(metricInvalidLabels[name], createLabelPair(label, ""))
		}
	}
	return metricInvalidLabels, nil
}

var camelCase = regexp.MustCompile(`[a-z][A-Z]`)

func isValidLabel(name string) bool {
	return camelCase.FindString(name) == ""
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

func createLabelPair(name string, value string) *dto.LabelPair {
	return &dto.LabelPair{
		Name:  &name,
		Value: &value,
	}
}
