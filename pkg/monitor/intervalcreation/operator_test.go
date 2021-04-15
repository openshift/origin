package intervalcreation

import (
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

func timeFor(asString string) time.Time {
	ret, err := time.Parse(time.RFC3339, asString)
	if err != nil {
		panic(err)
	}
	return ret
}

func TestIntervalsFromEvents_OperatorProgressing(t *testing.T) {
	intervals := monitorapi.Intervals{}
	intervals = append(intervals,
		monitorapi.EventInterval{
			Condition: monitorapi.Condition{
				Level:   monitorapi.Info,
				Locator: "clusteroperator/network",
				Message: "condition/Progressing status/True reason/Deploying changed: Deployment \\\"openshift-network-diagnostics/network-check-source\\\" is not available (awaiting 1 nodes)",
			},
			From: timeFor("2021-03-29T15:56:00Z"),
		},
		monitorapi.EventInterval{
			Condition: monitorapi.Condition{
				Level:   monitorapi.Info,
				Locator: "clusteroperator/network",
				Message: "condition/Progressing status/False changed: ",
			},
			From: timeFor("2021-03-29T15:56:11Z"),
		},
	)

	actual := IntervalsFromEvents_OperatorProgressing(intervals, time.Time{}, time.Time{})
	expectedSummary := `Mar 29 15:56:00.000 - 11s   W clusteroperator/network condition/Progressing status/True reason/Deployment \"openshift-network-diagnostics/network-check-source\" is not available (awaiting 1 nodes)`
	if actual[0].String() != expectedSummary {
		t.Fatal(spew.Sdump(actual))
	}
}
