package prometheus

import (
	"testing"

	v1 "github.com/openshift/api/config/v1"
	"github.com/openshift/origin/pkg/alerts"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
)

func TestMetricConditions_MatchesInterval(t *testing.T) {

	_, allowedFiring, _, _ := alerts.AllowedAlertsDuringConformance(v1.FeatureSet(""))

	type args struct {
		alertInterval monitorapi.Interval
	}
	type want struct {
		alert     string
		namespace string
	}
	tests := []struct {
		name string
		c    alerts.MetricConditions
		args args
		want *want
	}{
		{
			name: "KubePodNotReady openshift-e2e-loki",
			c:    allowedFiring,
			args: args{
				alertInterval: monitorapi.NewInterval(monitorapi.SourceAlert, monitorapi.Warning).
					Locator(monitorapi.NewLocator().AlertFromPromSampleStream(&model.SampleStream{
						Metric: map[model.LabelName]model.LabelValue{
							"alertname":  "KubePodNotReady",
							"alertstate": "firing",
							"namespace":  "openshift-e2e-loki",
							"severity":   "warning",
						},
					})).BuildNow(),
			},
			want: &want{
				alert:     "KubePodNotReady",
				namespace: "openshift-e2e-loki",
			},
		},
		{
			name: "HighOverallControlPlaneCPU",
			c:    allowedFiring,
			args: args{
				alertInterval: monitorapi.NewInterval(monitorapi.SourceAlert, monitorapi.Warning).
					Locator(monitorapi.NewLocator().AlertFromPromSampleStream(&model.SampleStream{
						Metric: map[model.LabelName]model.LabelValue{
							"alertname":  "HighOverallControlPlaneCPU",
							"alertstate": "firing",
							"severity":   "warning",
						},
					})).BuildNow(),
			},
			want: &want{
				alert: "HighOverallControlPlaneCPU",
			},
		},
		{
			name: "FakeAlertWithNoAllowance",
			c:    allowedFiring,
			args: args{
				alertInterval: monitorapi.NewInterval(monitorapi.SourceAlert, monitorapi.Warning).
					Locator(monitorapi.NewLocator().AlertFromPromSampleStream(&model.SampleStream{
						Metric: map[model.LabelName]model.LabelValue{
							"alertname":  "FakeAlert",
							"alertstate": "firing",
							"severity":   "warning",
						},
					})).BuildNow(),
			},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.c.MatchesInterval(tt.args.alertInterval)
			if tt.want == nil {
				assert.Nil(t, got)
			} else {
				assert.Equal(t, tt.want.alert, got.AlertName)
				assert.Equal(t, tt.want.namespace, got.AlertNamespace)
			}
		})
	}
}
