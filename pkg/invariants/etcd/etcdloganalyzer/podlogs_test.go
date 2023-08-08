package etcdloganalyzer

import (
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	kapi "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func parseTimeOrDie(tStr string) time.Time {
	t, err := time.Parse(time.RFC3339, tStr)
	if err != nil {
		logrus.WithError(err).Fatalf("malformed test timestamp: %s", tStr)
	}
	return t
}

func buildFakePod(namespace, podName, nodeName, uid string) *kapi.Pod {
	return &kapi.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: namespace,
			UID:       types.UID(uid),
		},
		Spec: kapi.PodSpec{
			NodeName: nodeName,
		},
	}
}

func TestPodLogScanner(t *testing.T) {

	tests := []struct {
		name             string
		logLine          string
		pod              *kapi.Pod
		beginning        time.Time
		end              time.Time
		expectedInterval *monitorapi.Interval
	}{
		{
			name:             "no match",
			logLine:          "foobar",
			pod:              buildFakePod("openshift-etcd", "etcd-1", "master-0", "fakeuuid"),
			expectedInterval: nil,
		},
		{
			name:      "etcd log slow fdatasync",
			logLine:   `{"level":"warn","ts":"2023-03-03T01:45:11.871Z","caller":"wal/wal.go:805","msg":"slow fdatasync","took":"1.044022245s","expected-duration":"1s"}`,
			pod:       buildFakePod("openshift-etcd", "etcd-1", "master-0", "fakeuuid"),
			beginning: parseTimeOrDie("2023-03-03T00:00:00.871Z"),
			end:       parseTimeOrDie("2023-03-03T03:00:00.871Z"),
			expectedInterval: &monitorapi.Interval{
				Condition: monitorapi.Condition{
					Level:   monitorapi.Warning,
					Locator: "ns/openshift-etcd pod/etcd-1 node/master-0 uid/fakeuuid container/etcd src/podLog",
					Message: "slow fdatasync",
				},
				From: parseTimeOrDie("2023-03-03T01:45:11.871Z"),
				To:   parseTimeOrDie("2023-03-03T01:45:12.871Z"),
			},
		},
		{
			name:             "etcd log slow fdatasync excluded if outside beginning and end times",
			logLine:          `{"level":"warn","ts":"2023-03-03T01:45:11.871Z","caller":"wal/wal.go:805","msg":"slow fdatasync","took":"1.044022245s","expected-duration":"1s"}`,
			pod:              buildFakePod("openshift-etcd", "etcd-1", "master-0", "fakeuuid"),
			beginning:        parseTimeOrDie("2023-03-03T06:00:00.871Z"),
			end:              parseTimeOrDie("2023-03-03T09:00:00.871Z"),
			expectedInterval: nil,
		},
	}

	for _, test := range tests {
		scanners := buildLogGatherers()
		logger := logrus.WithField("test", test.name)
		var interval *monitorapi.Interval
		var err error
		t.Run(test.name, func(t *testing.T) {
			for _, scanner := range scanners {
				interval, err = scanner.ScanLine(test.pod, test.logLine, test.beginning, test.end, logger)
				if interval != nil || err != nil {
					break
				}
			}
			assert.NoError(t, err)
			assert.Equal(t, test.expectedInterval, interval)
		})
	}

}
