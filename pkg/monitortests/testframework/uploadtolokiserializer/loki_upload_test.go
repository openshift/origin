package uploadtolokiserializer

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/stretchr/testify/assert"
)

func TestIntervalToLogLine(t *testing.T) {
	from := time.Now()
	tests := []struct {
		name           string
		interval       monitorapi.Interval
		expNS          string
		expLevel       string // Info or Error today
		expLogLine     map[string]string
		expDurationSec string
	}{
		{
			name: "interval with duration",
			interval: monitorapi.Interval{
				Condition: monitorapi.Condition{
					Level:   monitorapi.Info,
					Locator: "ns/openshift-e2e-loki pod/event-exporter-55f76dcbf6-xp6fm uid/49fb2fd1-6dd4-45b6-bb39-31acca69a573 container/event-exporter",
					Message: "constructed/true reason/ContainerWait missed real \"ContainerWait\"",
				},
				From: time.Now(),
				To:   time.Now().Add(20 * time.Second),
			},
			expLevel:       "Info",
			expNS:          "openshift-e2e-loki",
			expDurationSec: "20",
		},
		{
			name: "interval with no To timestamp",
			interval: monitorapi.Interval{
				Condition: monitorapi.Condition{
					Level:   monitorapi.Info,
					Locator: "ns/openshift-e2e-loki pod/event-exporter-55f76dcbf6-xp6fm uid/49fb2fd1-6dd4-45b6-bb39-31acca69a573 container/event-exporter",
					Message: "constructed/true reason/ContainerWait missed real \"ContainerWait\"",
				},
				From: time.Now(),
			},
			expLevel:       "Info",
			expNS:          "openshift-e2e-loki",
			expDurationSec: "1",
		},
		{
			name: "interval with identical To timestamp",
			interval: monitorapi.Interval{
				Condition: monitorapi.Condition{
					Level:   monitorapi.Info,
					Locator: "e2e-test/\"[sig-arch][Feature:ClusterUpgrade] Cluster should remain functional during upgrade [Disruptive] [Serial]\"",
					Message: "finishedStatus/Passed",
				},
				From: from,
				To:   from,
			},
			expLevel:       "Info",
			expNS:          "",
			expDurationSec: "1",
		},
		{
			name: "interval with no namespace",
			interval: monitorapi.Interval{
				Condition: monitorapi.Condition{
					Level:   monitorapi.Error,
					Locator: "node/ip-10-0-162-214.us-east-2.compute.internal",
					Message: "reason/FailedToAuthenticateWithOpenShiftUser Jun 16 07:54:45.263010 ip-10-0-162-214 kubenswrapper[1366]: W0616 07:54:45.263004    1366 reflector.go:533] vendor/k8s.io/client-go/informers/factory.go:150: failed to list *v1.Node: nodes \"ip-10-0-162-214.us-east-2.compute.internal\" is forbidden: User \"system:anonymous\" cannot list resource \"nodes\" in API group \"\" at the cluster scope",
				},
				From: time.Now(),
				To:   time.Now().Add(1 * time.Second),
			},
			expLevel:       "Error",
			expNS:          "",
			expDurationSec: "1",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			namespace, logLine, err := intervalToLogLine(test.interval, "20230713_00000")
			assert.NoError(t, err)
			assert.Equal(t, 1, 1)
			assert.Equal(t, test.expNS, namespace)
			assert.Equal(t, test.expNS, logLine["namespace"])
			assert.Equal(t, test.expLevel, logLine["level"])
			assert.Equal(t, test.expDurationSec, logLine["durationSec"])
			assert.True(t, len(logLine["filename"]) > 0)

			// _entry should be a serialized json blob, make sure we can unmarshal:
			var ri monitorapi.Interval
			err = json.Unmarshal([]byte(logLine["_entry"]), &ri)
			assert.NoError(t, err)
			assert.Equal(t, test.interval.Message, ri.Message)

		})
	}
}
