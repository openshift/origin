package operator

import (
	"reflect"
	"testing"
	"time"

	configv1 "github.com/openshift/api/config/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func fakeNow() time.Time {
	fakeNow, err := time.Parse("Mon Jan 2 15:04:05 -0700 MST 2006", "Mon Jan 2 15:04:05 -0700 MST 2006")
	if err != nil {
		panic(err)
	}
	return fakeNow
}

func newOperator(name string,
	available, availableMessage string, availableDuration time.Duration,
	degraded, degradedMessage string, degradedDuration time.Duration,
	progressing, progressingMessage string, progressingDuration time.Duration,
) configv1.ClusterOperator {
	return configv1.ClusterOperator{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Status: configv1.ClusterOperatorStatus{
			Conditions: []configv1.ClusterOperatorStatusCondition{
				{
					Type:               "Available",
					Status:             configv1.ConditionStatus(available),
					LastTransitionTime: metav1.Time{Time: fakeNow().Add(-1 * availableDuration)},
					Message:            availableMessage,
				},
				{
					Type:               "Degraded",
					Status:             configv1.ConditionStatus(degraded),
					LastTransitionTime: metav1.Time{Time: fakeNow().Add(-1 * degradedDuration)},
					Message:            degradedMessage,
				},
				{
					Type:               "Progressing",
					Status:             configv1.ConditionStatus(progressing),
					LastTransitionTime: metav1.Time{Time: fakeNow().Add(-1 * progressingDuration)},
					Message:            progressingMessage,
				},
			},
		},
	}
}

func TestWaitForOperatorsToSettle(t *testing.T) {
	nowFn = fakeNow
	tests := []struct {
		name      string
		operators []configv1.ClusterOperator
		expected  []string
	}{
		{
			name: "all pass",
			operators: []configv1.ClusterOperator{
				newOperator("foo",
					"True", "as expected", time.Minute,
					"False", "as expected", time.Minute,
					"False", "as expected", time.Minute,
				),
			},
			expected: []string{},
		},
		{
			name: "one not available",
			operators: []configv1.ClusterOperator{
				newOperator("foo",
					"False", "OH NO", time.Minute,
					"False", "as expected", time.Minute,
					"False", "as expected", time.Minute,
				),
				newOperator("bar",
					"True", "as expected", time.Minute,
					"False", "as expected", time.Minute,
					"False", "as expected", time.Minute,
				),
			},
			expected: []string{
				`clusteroperator/foo is not Available for 1m0s because "OH NO"`,
			},
		},
		{
			name: "one degraded, another unavailable",
			operators: []configv1.ClusterOperator{
				newOperator("foo",
					"False", "OH NO", time.Minute,
					"False", "as expected", time.Minute,
					"False", "as expected", time.Minute,
				),
				newOperator("bar",
					"True", "as expected", time.Minute,
					"True", "Degraded", 2*time.Minute,
					"False", "as expected", time.Minute,
				),
			},
			expected: []string{
				`clusteroperator/foo is not Available for 1m0s because "OH NO"`,
				`clusteroperator/bar is Degraded for 2m0s because "Degraded"`,
			},
		},
		{
			name: "one progressing",
			operators: []configv1.ClusterOperator{
				newOperator("foo",
					"True", "as expected", time.Minute,
					"False", "as expected", time.Minute,
					"False", "as expected", time.Minute,
				),
				newOperator("bar",
					"True", "as expected", time.Minute,
					"False", "as expected", time.Minute,
					"True", "rolling out", time.Minute,
				),
			},
			expected: []string{
				`clusteroperator/bar is Progressing for 1m0s because "rolling out"`,
			},
		},
		{
			name: "one doing both",
			operators: []configv1.ClusterOperator{
				newOperator("foo",
					"True", "as expected", time.Minute,
					"True", "Degraded", 2*time.Minute,
					"True", "rolling out", time.Minute,
				),
				newOperator("bar",
					"True", "as expected", time.Minute,
					"False", "as expected", time.Minute,
					"False", "as expected", time.Minute,
				),
			},
			expected: []string{
				`clusteroperator/foo is Degraded for 2m0s because "Degraded"`,
				`clusteroperator/foo is Progressing for 1m0s because "rolling out"`,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := unsettledOperators(tt.operators)
			if !reflect.DeepEqual(actual, tt.expected) {
				t.Error(actual)
			}
		})
	}
}
