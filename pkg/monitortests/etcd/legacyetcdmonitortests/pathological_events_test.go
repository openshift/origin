package legacyetcdmonitortests

import (
	"testing"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_testRequiredInstallerResourcesMissing(t *testing.T) {
	tests := []struct {
		name      string
		intervals monitorapi.Intervals
		kind      string
	}{
		{
			name: "Reason doesn't match but results in passing junit",
			intervals: monitorapi.Intervals{
				{
					Condition: monitorapi.Condition{
						Level: monitorapi.Info,
						Message: monitorapi.Message{
							Reason:       monitorapi.NodeUpdateReason, // anything but the one we're looking for
							HumanMessage: "secrets: etcd-all-certs-3",
							Annotations: map[monitorapi.AnnotationKey]string{
								monitorapi.AnnotationCount: "25",
							},
						},
					},
					Source: monitorapi.SourceKubeEvent,
				},
			},
			kind: "pass",
		},
		{
			name: "Test failing case",
			intervals: monitorapi.Intervals{
				{
					Condition: monitorapi.Condition{
						Level: monitorapi.Info,
						Message: monitorapi.Message{
							Reason:       monitorapi.IntervalReason("RequiredInstallerResourcesMissing"),
							HumanMessage: "secrets: etcd-all-certs-3",
							Annotations: map[monitorapi.AnnotationKey]string{
								monitorapi.AnnotationCount: "21",
							},
						},
					},
					Source: monitorapi.SourceKubeEvent,
				},
			},
			kind: "fail",
		},
		{
			name: "Test flaking case",
			intervals: monitorapi.Intervals{
				{
					Condition: monitorapi.Condition{
						Level: monitorapi.Info,
						Message: monitorapi.Message{
							Reason:       monitorapi.IntervalReason("RequiredInstallerResourcesMissing"),
							HumanMessage: "secrets: etcd-all-certs-3",
							Annotations: map[monitorapi.AnnotationKey]string{
								monitorapi.AnnotationCount: "16",
							},
						},
					},
					Source: monitorapi.SourceKubeEvent,
				},
			},
			kind: "flake",
		},
		{
			name: "Test passing case",
			intervals: monitorapi.Intervals{
				{
					Condition: monitorapi.Condition{
						Level: monitorapi.Info,
						Message: monitorapi.Message{
							Reason:       monitorapi.IntervalReason("RequiredInstallerResourcesMissing"),
							HumanMessage: "secrets: etcd-all-certs-3",
							Annotations: map[monitorapi.AnnotationKey]string{
								monitorapi.AnnotationCount: "7",
							},
						},
					},
					Source: monitorapi.SourceKubeEvent,
				},
			},
			kind: "pass",
		},
		{
			name: "Test passes because the event is ignored",
			intervals: monitorapi.Intervals{
				{
					Condition: monitorapi.Condition{
						Level: monitorapi.Info,
						Message: monitorapi.Message{
							Reason:       monitorapi.IntervalReason("RequiredInstallerResourcesMissing"),
							HumanMessage: "configmaps: check-endpoints-config-3",
							Annotations: map[monitorapi.AnnotationKey]string{
								monitorapi.AnnotationCount: "21",
							},
						},
					},
					Source: monitorapi.SourceKubeEvent,
				},
			},
			kind: "pass",
		},
		{
			name: "Test fails with ignored events",
			intervals: monitorapi.Intervals{
				{
					Condition: monitorapi.Condition{
						Level: monitorapi.Info,
						Message: monitorapi.Message{
							Reason:       monitorapi.IntervalReason("RequiredInstallerResourcesMissing"),
							HumanMessage: "configmaps: check-endpoints-config-3",
							Annotations: map[monitorapi.AnnotationKey]string{
								monitorapi.AnnotationCount: "21",
							},
						},
					},
					Source: monitorapi.SourceKubeEvent,
				},
				{
					Condition: monitorapi.Condition{
						Level: monitorapi.Info,
						Message: monitorapi.Message{
							Reason:       monitorapi.IntervalReason("RequiredInstallerResourcesMissing"),
							HumanMessage: "secrets: etcd-all-certs-3",
							Annotations: map[monitorapi.AnnotationKey]string{
								monitorapi.AnnotationCount: "21",
							},
						},
					},
					Source: monitorapi.SourceKubeEvent,
				},
			},
			kind: "fail",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := tt.intervals
			junit_tests := testRequiredInstallerResourcesMissing(e)
			switch tt.kind {
			case "pass":
				if len(junit_tests) != 1 {
					t.Errorf("This should've been a single passing Test, but got %d tests", len(junit_tests))
				}
				if len(junit_tests[0].SystemOut) != 0 {
					t.Errorf("This should've been a pass, but got %s", junit_tests[0].SystemErr)
				}
			case "fail":
				if len(junit_tests) != 1 {
					t.Errorf("This should've been a single failing Test, but got %d tests", len(junit_tests))
				}
				if len(junit_tests[0].SystemOut) == 0 {
					t.Error("This should've been a failure but got no output")
				}
			case "flake":
				if len(junit_tests) != 2 {
					t.Errorf("This should've been a two tests as flake, but got %d tests", len(junit_tests))
				}
			default:
				t.Errorf("Unknown Test kind")
			}

		})
	}
}

func Test_testOperatorStatusChanged(t *testing.T) {
	tests := []struct {
		name     string
		interval monitorapi.Interval
		kind     string
	}{
		{
			name: "event count under threshold should pass",
			interval: monitorapi.Interval{
				Condition: monitorapi.Condition{
					Level: monitorapi.Info,
					Locator: monitorapi.Locator{
						Type: monitorapi.LocatorTypePod,
						Keys: map[monitorapi.LocatorKey]string{
							monitorapi.LocatorNamespaceKey: "openshift-etcd",
							monitorapi.LocatorPodKey:       "openshift-etcd-foobar",
						},
					},
					Message: monitorapi.Message{
						Reason:       monitorapi.IntervalReason("OperatorStatusChanged"),
						HumanMessage: "Status for clusteroperator/etcd changed: Degraded message changed from \"NodeControllerDegraded: All master nodes are ready/EtcdMembersDegraded: 2 of 3 members are available, ip-10-0-217-93.us-west-1.compute.internal is unhealthy\" to \"NodeControllerDegraded: All master nodes are ready/EtcdMembersDegraded: No unhealthy members found\"",
						Annotations: map[monitorapi.AnnotationKey]string{
							monitorapi.AnnotationCount: "2",
						},
					},
				},
				Source: monitorapi.SourceKubeEvent,
			},
			kind: "pass",
		},
		{
			name: "event count over threshold should flake",
			interval: monitorapi.Interval{
				Condition: monitorapi.Condition{
					Level: monitorapi.Info,
					Locator: monitorapi.Locator{
						Type: monitorapi.LocatorTypePod,
						Keys: map[monitorapi.LocatorKey]string{
							monitorapi.LocatorNamespaceKey: "openshift-etcd",
							monitorapi.LocatorPodKey:       "openshift-etcd-foobar",
						},
					},
					Message: monitorapi.Message{
						Reason:       monitorapi.IntervalReason("OperatorStatusChanged"),
						HumanMessage: "Status for clusteroperator/etcd changed: Degraded message changed from \"NodeControllerDegraded: All master nodes are ready/EtcdMembersDegraded: 2 of 3 members are available, ip-10-0-217-93.us-west-1.compute.internal is unhealthy\" to \"NodeControllerDegraded: All master nodes are ready/EtcdMembersDegraded: No unhealthy members found\"",
						Annotations: map[monitorapi.AnnotationKey]string{
							monitorapi.AnnotationCount: "24",
						},
					},
				},
				Source: monitorapi.SourceKubeEvent,
			},
			kind: "flake",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := monitorapi.Intervals{tt.interval}
			junitTests := testOperatorStatusChanged(e)
			switch tt.kind {
			case "pass":
				assert.Equal(t, 1, len(junitTests), "This should've been a single passing Test")
				assert.Nil(t, junitTests[0].FailureOutput, "This should've been a pass")
			case "fail":
				require.Equal(t, 1, len(junitTests), "This should've been a single failing Test")
				require.NotEqual(t, 0, len(junitTests[0].SystemOut), "This should've been a failure")
			case "flake":
				assert.Equal(t, 2, len(junitTests), "This should've been two tests as flake")
			default:
				require.Fail(t, "Unknown Test kind")
			}
		})
	}
}
