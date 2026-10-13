package operatorstateanalyzer

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

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
		monitorapi.Interval{
			Source: monitorapi.SourceClusterOperatorMonitor,
			Condition: monitorapi.Condition{
				Level: monitorapi.Info,
				Locator: monitorapi.Locator{
					Type: monitorapi.LocatorTypeClusterOperator,
					Keys: map[monitorapi.LocatorKey]string{
						monitorapi.LocatorClusterOperatorKey: "network",
					},
				},
				Message: monitorapi.Message{
					Reason:       "Deploying",
					HumanMessage: "Deployment \\\"openshift-network-diagnostics/network-check-source\\\" is not available (awaiting 1 nodes)",
					Annotations: map[monitorapi.AnnotationKey]string{
						monitorapi.AnnotationCondition: "Progressing",
						monitorapi.AnnotationStatus:    "True",
						monitorapi.AnnotationReason:    "Deploying",
					},
				},
			},
			From: timeFor("2021-03-29T15:56:00Z"),
			To:   timeFor("2021-03-29T15:56:00Z"),
		},
		monitorapi.Interval{
			Source: monitorapi.SourceClusterOperatorMonitor,
			Condition: monitorapi.Condition{
				Level: monitorapi.Info,
				Locator: monitorapi.Locator{
					Type: monitorapi.LocatorTypeClusterOperator,
					Keys: map[monitorapi.LocatorKey]string{
						monitorapi.LocatorClusterOperatorKey: "network",
					},
				},
				Message: monitorapi.Message{
					HumanMessage: "",
					Annotations: map[monitorapi.AnnotationKey]string{
						monitorapi.AnnotationCondition: "Progressing",
						monitorapi.AnnotationStatus:    "False",
					},
				},
			},
			From: timeFor("2021-03-29T15:56:11Z"),
			To:   timeFor("2021-03-29T15:56:11Z"),
		},
	)

	actual := intervalsFromEvents_OperatorProgressing(intervals, nil, time.Time{}, time.Time{})
	expectedSummary := `Mar 29 15:56:00.000 - 11s   W clusteroperator/network condition/Progressing reason/Deploying status/True Deployment \"openshift-network-diagnostics/network-check-source\" is not available (awaiting 1 nodes)`
	assert.Equal(t, expectedSummary, actual[0].String())
}

func TestIntervalsFromEvents_OperatorProgressing2(t *testing.T) {
	intervals := monitorapi.Intervals{}
	intervals = append(intervals,
		monitorapi.Interval{
			Source: monitorapi.SourceClusterOperatorMonitor,
			Condition: monitorapi.Condition{
				Level: monitorapi.Warning,
				Locator: monitorapi.Locator{
					Type: monitorapi.LocatorTypeClusterOperator,
					Keys: map[monitorapi.LocatorKey]string{
						monitorapi.LocatorClusterOperatorKey: "kube-apiserver",
					},
				},
				Message: monitorapi.Message{
					Reason:       "NodeInstaller",
					HumanMessage: "NodeInstallerProgressing: 3 nodes are at revision 6; 0 nodes have achieved new revision 7",
					Annotations: map[monitorapi.AnnotationKey]string{
						monitorapi.AnnotationCondition: "Progressing",
						monitorapi.AnnotationStatus:    "True",
						monitorapi.AnnotationReason:    "NodeInstaller",
					},
				},
			},
			From: timeFor("2023-09-25T13:32:03Z"),
			To:   timeFor("2023-09-25T13:32:03Z"),
		},
		monitorapi.Interval{
			Source: monitorapi.SourceClusterOperatorMonitor,
			Condition: monitorapi.Condition{
				Level: monitorapi.Warning,
				Locator: monitorapi.Locator{
					Type: monitorapi.LocatorTypeClusterOperator,
					Keys: map[monitorapi.LocatorKey]string{
						monitorapi.LocatorClusterOperatorKey: "kube-apiserver",
					},
				},
				Message: monitorapi.Message{
					Reason:       "AsExpected",
					HumanMessage: "NodeInstallerProgressing: 3 nodes are at revision 7",
					Annotations: map[monitorapi.AnnotationKey]string{
						monitorapi.AnnotationCondition: "Progressing",
						monitorapi.AnnotationStatus:    "False",
						monitorapi.AnnotationReason:    "AsExpected",
					},
				},
			},
			From: timeFor("2023-09-25T13:41:00Z"),
			To:   timeFor("2023-09-25T13:41:00Z"),
		},
	)

	actual := intervalsFromEvents_OperatorProgressing(intervals, nil, time.Time{}, time.Time{})
	assert.Equal(t, 1, len(actual))
}
