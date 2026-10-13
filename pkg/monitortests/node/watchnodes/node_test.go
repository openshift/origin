package watchnodes

import (
	"testing"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestlibrary/utility"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNodeRoles(t *testing.T) {
	var testCases = []struct {
		node     *corev1.Node
		expected string
	}{
		{
			node:     &corev1.Node{},
			expected: "",
		},
		{
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"node-role.kubernetes.io/master": "",
					},
				},
			},
			expected: "master",
		},
		{
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"node-role.kubernetes.io/worker": "",
					},
				},
			},
			expected: "worker",
		},
		{
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"node-role.kubernetes.io/master": "",
						"node-role.kubernetes.io/worker": "",
					},
				},
			},
			expected: "master,worker",
		},
	}

	for _, tc := range testCases {
		if actual := nodeRoles(tc.node); tc.expected != actual {
			t.Errorf("mismatch roles. expected: %s, actual: %s", tc.expected, actual)
		}
	}
}

func TestReportUnexpectedNodeDownFailures(t *testing.T) {
	var testCases = []struct {
		name             string
		rawIntervals     monitorapi.Intervals
		unexpectedReason monitorapi.IntervalReason
		expected         []string
	}{
		{
			name: "node unexpected ready reason with no deleted machines",
			rawIntervals: monitorapi.Intervals{
				{
					Condition: monitorapi.Condition{
						Level: monitorapi.Error,
						Locator: monitorapi.Locator{
							Type: monitorapi.LocatorTypeNode,
							Keys: map[monitorapi.LocatorKey]string{
								"node": "node1",
							},
						},
						Message: monitorapi.Message{
							Reason:       monitorapi.NodeUnexpectedReadyReason,
							HumanMessage: "unexpected node not ready",
							Annotations: map[monitorapi.AnnotationKey]string{
								monitorapi.AnnotationReason: "UnexpectedNotReady",
							},
						},
					},
					From: utility.SystemdJournalLogTime("Nov 11 19:46:00", 2024),
					To:   utility.SystemdJournalLogTime("Nov 11 19:46:00", 2024),
				},
			},
			expected:         []string{"node/node1 - reason/UnexpectedNotReady unexpected node not ready at from: 2024-11-11 19:46:00 +0000 UTC - to: 2024-11-11 19:46:00 +0000 UTC"},
			unexpectedReason: monitorapi.NodeUnexpectedReadyReason,
		},
		{
			// OCPBUG-44244: if machine phase interval does not have node name than we will get a failure
			name: "node unexpected ready reason with a deleted machine but missing node",
			rawIntervals: monitorapi.Intervals{
				{
					Condition: monitorapi.Condition{
						Level: monitorapi.Error,
						Locator: monitorapi.Locator{
							Type: monitorapi.LocatorTypeNode,
							Keys: map[monitorapi.LocatorKey]string{
								"node": "node1",
							},
						},
						Message: monitorapi.Message{
							Reason:       monitorapi.NodeUnexpectedReadyReason,
							HumanMessage: "unexpected node not ready",
							Annotations: map[monitorapi.AnnotationKey]string{
								monitorapi.AnnotationReason: "UnexpectedNotReady",
							},
						},
					},
					From: utility.SystemdJournalLogTime("Nov 11 19:46:00", 2024),
					To:   utility.SystemdJournalLogTime("Nov 11 19:46:00", 2024),
				},
				{
					Condition: monitorapi.Condition{
						Level: monitorapi.Info,
						Locator: monitorapi.Locator{
							Type: monitorapi.LocatorTypeMachine,
							Keys: map[monitorapi.LocatorKey]string{
								"machine": "machine1",
							},
						},
						Message: monitorapi.Message{
							Reason:       monitorapi.MachinePhase,
							HumanMessage: "Machine is in deleted",
							Annotations: map[monitorapi.AnnotationKey]string{
								monitorapi.AnnotationConstructed: "machine-lifecycle-constructor",
								monitorapi.AnnotationPhase:       "Deleting",
								monitorapi.AnnotationReason:      "MachinePhase",
							},
						},
					},
					From: utility.SystemdJournalLogTime("Nov 11 19:45:00", 2024),
					To:   utility.SystemdJournalLogTime("Nov 11 19:47:00", 2024),
				},
			},
			expected:         []string{"node/node1 - reason/UnexpectedNotReady unexpected node not ready at from: 2024-11-11 19:46:00 +0000 UTC - to: 2024-11-11 19:46:00 +0000 UTC"},
			unexpectedReason: monitorapi.NodeUnexpectedReadyReason,
		},
		{
			// OCPBUG-44244: if deleted machine exists for that node than we will get no failures
			name: "node unexpected ready reason with a deleted machine and a node annotation",
			rawIntervals: monitorapi.Intervals{
				{
					Condition: monitorapi.Condition{
						Level: monitorapi.Error,
						Locator: monitorapi.Locator{
							Type: monitorapi.LocatorTypeNode,
							Keys: map[monitorapi.LocatorKey]string{
								"node": "node1",
							},
						},
						Message: monitorapi.Message{
							Reason:       monitorapi.NodeUnexpectedReadyReason,
							HumanMessage: "unexpected node not ready",
							Annotations: map[monitorapi.AnnotationKey]string{
								monitorapi.AnnotationReason: "UnexpectedNotReady",
							},
						},
					},
					From: utility.SystemdJournalLogTime("Nov 11 19:46:00", 2024),
					To:   utility.SystemdJournalLogTime("Nov 11 19:46:00", 2024),
				},
				{
					Condition: monitorapi.Condition{
						Level: monitorapi.Info,
						Locator: monitorapi.Locator{
							Type: monitorapi.LocatorTypeMachine,
							Keys: map[monitorapi.LocatorKey]string{
								"machine": "machine1",
							},
						},
						Message: monitorapi.Message{
							Reason:       monitorapi.MachinePhase,
							HumanMessage: "Machine is in deleted",
							Annotations: map[monitorapi.AnnotationKey]string{
								monitorapi.AnnotationConstructed: "machine-lifecycle-constructor",
								monitorapi.AnnotationNode:        "node1",
								monitorapi.AnnotationPhase:       "Deleting",
								monitorapi.AnnotationReason:      "MachinePhase",
							},
						},
					},
					From: utility.SystemdJournalLogTime("Nov 11 19:45:00", 2024),
					To:   utility.SystemdJournalLogTime("Nov 11 19:47:00", 2024),
				},
			},
			expected:         []string{},
			unexpectedReason: monitorapi.NodeUnexpectedReadyReason,
		},
		{
			name: "node unexpected unreachable reason with no deleted machines",
			rawIntervals: monitorapi.Intervals{
				{
					Condition: monitorapi.Condition{
						Level: monitorapi.Error,
						Locator: monitorapi.Locator{
							Type: monitorapi.LocatorTypeNode,
							Keys: map[monitorapi.LocatorKey]string{
								"node": "node1",
							},
						},
						Message: monitorapi.Message{
							Reason:       monitorapi.NodeUnexpectedUnreachableReason,
							HumanMessage: "unexpected node unreachable",
							Annotations: map[monitorapi.AnnotationKey]string{
								monitorapi.AnnotationReason: "UnexpectedUnreachable",
							},
						},
					},
					From: utility.SystemdJournalLogTime("Nov 11 19:46:00", 2024),
					To:   utility.SystemdJournalLogTime("Nov 11 19:46:00", 2024),
				},
			},
			expected:         []string{"node/node1 - reason/UnexpectedUnreachable unexpected node unreachable at from: 2024-11-11 19:46:00 +0000 UTC - to: 2024-11-11 19:46:00 +0000 UTC"},
			unexpectedReason: monitorapi.NodeUnexpectedUnreachableReason,
		},
		{
			// OCPBUG-44244: if machine phase interval does not have node name than we will get a failure
			name: "node unexpected unreachable reason with a deleted machine but missing node",
			rawIntervals: monitorapi.Intervals{
				{
					Condition: monitorapi.Condition{
						Level: monitorapi.Error,
						Locator: monitorapi.Locator{
							Type: monitorapi.LocatorTypeNode,
							Keys: map[monitorapi.LocatorKey]string{
								"node": "node1",
							},
						},
						Message: monitorapi.Message{
							Reason:       monitorapi.NodeUnexpectedUnreachableReason,
							HumanMessage: "unexpected node unreachable",
							Annotations: map[monitorapi.AnnotationKey]string{
								monitorapi.AnnotationReason: "UnexpectedUnreachable",
							},
						},
					},
					From: utility.SystemdJournalLogTime("Nov 11 19:46:00", 2024),
					To:   utility.SystemdJournalLogTime("Nov 11 19:46:00", 2024),
				},
				{
					Condition: monitorapi.Condition{
						Level: monitorapi.Info,
						Locator: monitorapi.Locator{
							Type: monitorapi.LocatorTypeMachine,
							Keys: map[monitorapi.LocatorKey]string{
								"machine": "machine1",
							},
						},
						Message: monitorapi.Message{
							Reason:       monitorapi.MachinePhase,
							HumanMessage: "Machine is in deleted",
							Annotations: map[monitorapi.AnnotationKey]string{
								monitorapi.AnnotationConstructed: "machine-lifecycle-constructor",
								monitorapi.AnnotationPhase:       "Deleting",
								monitorapi.AnnotationReason:      "MachinePhase",
							},
						},
					},
					From: utility.SystemdJournalLogTime("Nov 11 19:45:00", 2024),
					To:   utility.SystemdJournalLogTime("Nov 11 19:47:00", 2024),
				},
			},
			expected:         []string{"node/node1 - reason/UnexpectedUnreachable unexpected node unreachable at from: 2024-11-11 19:46:00 +0000 UTC - to: 2024-11-11 19:46:00 +0000 UTC"},
			unexpectedReason: monitorapi.NodeUnexpectedUnreachableReason,
		},
		{
			// OCPBUG-44244: if deleted machine exists for that node than we will get no failures
			name: "node unexpected unreachable reason with a deleted machine and a node annotation",
			rawIntervals: monitorapi.Intervals{
				{
					Condition: monitorapi.Condition{
						Level: monitorapi.Error,
						Locator: monitorapi.Locator{
							Type: monitorapi.LocatorTypeNode,
							Keys: map[monitorapi.LocatorKey]string{
								"node": "node1",
							},
						},
						Message: monitorapi.Message{
							Reason:       monitorapi.NodeUnexpectedUnreachableReason,
							HumanMessage: "unexpected node not ready",
							Annotations: map[monitorapi.AnnotationKey]string{
								monitorapi.AnnotationReason: "UnexpectedUnreachable",
							},
						},
					},
					From: utility.SystemdJournalLogTime("Nov 11 19:46:00", 2024),
					To:   utility.SystemdJournalLogTime("Nov 11 19:46:00", 2024),
				},
				{
					Condition: monitorapi.Condition{
						Level: monitorapi.Info,
						Locator: monitorapi.Locator{
							Type: monitorapi.LocatorTypeMachine,
							Keys: map[monitorapi.LocatorKey]string{
								"machine": "machine1",
							},
						},
						Message: monitorapi.Message{
							Reason:       monitorapi.MachinePhase,
							HumanMessage: "Machine is in deleted",
							Annotations: map[monitorapi.AnnotationKey]string{
								monitorapi.AnnotationConstructed: "machine-lifecycle-constructor",
								monitorapi.AnnotationNode:        "node1",
								monitorapi.AnnotationPhase:       "Deleting",
								monitorapi.AnnotationReason:      "MachinePhase",
							},
						},
					},
					From: utility.SystemdJournalLogTime("Nov 11 19:45:00", 2024),
					To:   utility.SystemdJournalLogTime("Nov 11 19:47:00", 2024),
				},
			},
			expected:         []string{},
			unexpectedReason: monitorapi.NodeUnexpectedUnreachableReason,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := reportUnexpectedNodeDownFailures(tc.rawIntervals, tc.unexpectedReason)
			if len(actual) != len(tc.expected) {
				t.Fatalf("mismatch of length from actual to expected")
			}
			for i := range actual {
				if actual[i] != tc.expected[i] {
					t.Errorf("mismatch failures. expected: %s, actual: %s", tc.expected[i], actual[i])
				}
			}
		})
	}
}
