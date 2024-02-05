package pathologicaleventlibrary

import (
	"testing"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/stretchr/testify/assert"
)

func Test_singleEventThresholdCheck_getNamespacedFailuresAndFlakes(t *testing.T) {
	namespace := "openshift-etcd-operator"
	samplePod := "etcd-operator-6f9b4d9d4f-4q9q8"

	testName := "[sig-cluster-lifecycle] pathological event should not see excessive Back-off restarting failed containers"
	backoffMatcher := NewSingleEventThresholdCheck(testName, AllowBackOffRestartingFailedContainer,
		DuplicateEventThreshold, BackoffRestartingFlakeThreshold)
	type fields struct {
		testName       string
		matcher        *SimplePathologicalEventMatcher
		failThreshold  int
		flakeThreshold int
	}
	type args struct {
		events monitorapi.Intervals
	}
	tests := []struct {
		name             string
		fields           fields
		args             args
		expectedKeyCount int
	}{
		{
			name: "Successful test yields no keys",
			fields: fields{
				testName:       testName,
				matcher:        backoffMatcher.matcher,
				failThreshold:  DuplicateEventThreshold,
				flakeThreshold: BackoffRestartingFlakeThreshold,
			},
			args: args{
				events: monitorapi.Intervals{
					BuildTestDupeKubeEvent(namespace, samplePod,
						"BackOff",
						"Back-off restarting failed container",
						5),
				},
			},
			expectedKeyCount: 0,
		},
		{
			name: "Failing test yields one key",
			fields: fields{
				testName:       testName,
				matcher:        backoffMatcher.matcher,
				failThreshold:  DuplicateEventThreshold,
				flakeThreshold: BackoffRestartingFlakeThreshold,
			},
			args: args{
				events: monitorapi.Intervals{
					BuildTestDupeKubeEvent(namespace, samplePod,
						"BackOff",
						"Back-off restarting failed container",
						21),
				},
			},
			expectedKeyCount: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &singleEventThresholdCheck{
				testName:       tt.fields.testName,
				matcher:        tt.fields.matcher,
				failThreshold:  tt.fields.failThreshold,
				flakeThreshold: tt.fields.flakeThreshold,
			}
			got := s.getNamespacedFailuresAndFlakes(tt.args.events)
			assert.Equal(t, tt.expectedKeyCount, len(got))
		})
	}
}
