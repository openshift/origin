package prometheus

import (
	"strings"
	"testing"

	prometheusv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	dto "github.com/prometheus/client_model/go"
)

func TestMetadataToMetricFamilies(t *testing.T) {
	tests := []struct {
		name     string
		metadata map[string][]prometheusv1.Metadata
		wantLen  int
		check    func(t *testing.T, families []*dto.MetricFamily)
	}{
		{
			name:     "empty metadata",
			metadata: map[string][]prometheusv1.Metadata{},
			wantLen:  0,
		},
		{
			name: "counter metric",
			metadata: map[string][]prometheusv1.Metadata{
				"http_requests_total": {{
					Type: prometheusv1.MetricTypeCounter,
					Help: "Total number of HTTP requests.",
				}},
			},
			wantLen: 1,
			check: func(t *testing.T, families []*dto.MetricFamily) {
				f := families[0]
				if f.GetName() != "http_requests_total" {
					t.Errorf("got name %q, want %q", f.GetName(), "http_requests_total")
				}
				if f.GetType() != dto.MetricType_COUNTER {
					t.Errorf("got type %v, want COUNTER", f.GetType())
				}
				if f.GetHelp() != "Total number of HTTP requests." {
					t.Errorf("got help %q, want %q", f.GetHelp(), "Total number of HTTP requests.")
				}
			},
		},
		{
			name: "gauge metric",
			metadata: map[string][]prometheusv1.Metadata{
				"temperature_celsius": {{
					Type: prometheusv1.MetricTypeGauge,
					Help: "Current temperature.",
				}},
			},
			wantLen: 1,
			check: func(t *testing.T, families []*dto.MetricFamily) {
				if families[0].GetType() != dto.MetricType_GAUGE {
					t.Errorf("got type %v, want GAUGE", families[0].GetType())
				}
			},
		},
		{
			name: "histogram metric",
			metadata: map[string][]prometheusv1.Metadata{
				"request_duration_seconds": {{
					Type: prometheusv1.MetricTypeHistogram,
					Help: "Request duration.",
				}},
			},
			wantLen: 1,
			check: func(t *testing.T, families []*dto.MetricFamily) {
				if families[0].GetType() != dto.MetricType_HISTOGRAM {
					t.Errorf("got type %v, want HISTOGRAM", families[0].GetType())
				}
			},
		},
		{
			name: "summary metric",
			metadata: map[string][]prometheusv1.Metadata{
				"rpc_duration_seconds": {{
					Type: prometheusv1.MetricTypeSummary,
					Help: "RPC duration.",
				}},
			},
			wantLen: 1,
			check: func(t *testing.T, families []*dto.MetricFamily) {
				if families[0].GetType() != dto.MetricType_SUMMARY {
					t.Errorf("got type %v, want SUMMARY", families[0].GetType())
				}
			},
		},
		{
			name: "unknown type maps to UNTYPED",
			metadata: map[string][]prometheusv1.Metadata{
				"some_metric": {{
					Type: "unknown",
					Help: "Some metric.",
				}},
			},
			wantLen: 1,
			check: func(t *testing.T, families []*dto.MetricFamily) {
				if families[0].GetType() != dto.MetricType_UNTYPED {
					t.Errorf("got type %v, want UNTYPED", families[0].GetType())
				}
			},
		},
		{
			name: "recording rules are filtered out",
			metadata: map[string][]prometheusv1.Metadata{
				"namespace:container_cpu_usage:sum": {{
					Type: prometheusv1.MetricTypeGauge,
					Help: "Recording rule.",
				}},
				"http_requests_total": {{
					Type: prometheusv1.MetricTypeCounter,
					Help: "Total HTTP requests.",
				}},
			},
			wantLen: 1,
			check: func(t *testing.T, families []*dto.MetricFamily) {
				if families[0].GetName() != "http_requests_total" {
					t.Errorf("expected recording rule to be filtered, got %q", families[0].GetName())
				}
			},
		},
		{
			name: "duplicate metadata entries use first",
			metadata: map[string][]prometheusv1.Metadata{
				"up": {
					{Type: prometheusv1.MetricTypeGauge, Help: "First help."},
					{Type: prometheusv1.MetricTypeGauge, Help: "Second help."},
				},
			},
			wantLen: 1,
			check: func(t *testing.T, families []*dto.MetricFamily) {
				if families[0].GetHelp() != "First help." {
					t.Errorf("got help %q, want first entry", families[0].GetHelp())
				}
			},
		},
		{
			name: "entry with no metadata is skipped",
			metadata: map[string][]prometheusv1.Metadata{
				"empty_metric": {},
			},
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			families := MetadataToMetricFamilies(tt.metadata)
			if len(families) != tt.wantLen {
				t.Fatalf("got %d families, want %d", len(families), tt.wantLen)
			}
			if tt.check != nil {
				tt.check(t, families)
			}
		})
	}
}

func TestCamelCaseRegex(t *testing.T) {
	tests := []struct {
		label string
		want  bool
	}{
		{"snake_case", false},
		{"camelCase", true},
		{"CamelCase", true},    // matches the lC transition
		{"myHTTPHandler", true}, // matches the PH transition
		{"httpCode", true},
		{"__name__", false},
		{"le", false},
		{"quantile", false},
		{"mountPoint", true},
		{"ALL_CAPS", false},
	}

	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			got := camelCaseRe.MatchString(tt.label)
			if got != tt.want {
				t.Errorf("camelCaseRe.MatchString(%q) = %v, want %v", tt.label, got, tt.want)
			}
		})
	}
}

func TestSetCamelCaseLabels(t *testing.T) {
	name1 := "metric_a"
	name2 := "metric_b"
	metricType := dto.MetricType_GAUGE
	help := "help"
	families := []*dto.MetricFamily{
		{Name: &name1, Type: &metricType, Help: &help, Metric: []*dto.Metric{{}}},
		{Name: &name2, Type: &metricType, Help: &help, Metric: []*dto.Metric{{}}},
	}

	labelName := "camelCase"
	labelVal := ""
	labelsPerMetric := map[string][]*dto.LabelPair{
		"metric_a": {{Name: &labelName, Value: &labelVal}},
	}

	SetCamelCaseLabels(families, labelsPerMetric)

	if len(families[0].Metric[0].Label) != 1 {
		t.Fatalf("metric_a should have 1 label, got %d", len(families[0].Metric[0].Label))
	}
	if families[0].Metric[0].Label[0].GetName() != "camelCase" {
		t.Errorf("got label name %q, want %q", families[0].Metric[0].Label[0].GetName(), "camelCase")
	}
	if len(families[1].Metric[0].Label) != 0 {
		t.Errorf("metric_b should have 0 labels, got %d", len(families[1].Metric[0].Label))
	}
}

func TestFormatExceptionEntries(t *testing.T) {
	problems := []Problem{
		{Metric: "z_metric", Text: "counter should have _total suffix"},
		{Metric: "a_metric", Text: "use base unit"},
		{Metric: "z_metric", Text: "duplicate entry"},
	}

	got := FormatExceptionEntries(problems)

	if !strings.Contains(got, `"a_metric"`) {
		t.Error("expected a_metric in output")
	}
	if !strings.Contains(got, `"z_metric"`) {
		t.Error("expected z_metric in output")
	}
	// a_metric should appear before z_metric (sorted)
	aIdx := strings.Index(got, "a_metric")
	zIdx := strings.Index(got, "z_metric")
	if aIdx > zIdx {
		t.Error("entries should be sorted alphabetically")
	}
	// z_metric should only appear once (deduped)
	if strings.Count(got, "z_metric") != 1 {
		t.Error("duplicate metrics should be deduped")
	}
}
