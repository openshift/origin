package apis

import (
	"errors"
	"strings"
	"testing"
	"time"

	etcdv1 "github.com/openshift/api/etcd/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	testTargetNode              = "master-0"
	testSurvivingNode           = "master-1"
	testBaselineResourceVersion = "baseline-rv"
	testCurrentResourceVersion  = "current-rv"
)

type targetFailureObservation struct {
	condition *operatorv1.OperatorCondition
	cluster   *etcdv1.PacemakerCluster
}

type targetFailureClassificationCase struct {
	name        string
	mutate      func(*targetFailureObservation)
	wantMode    PacemakerTargetFailureMode
	wantErrText string
}

func testNodeCondition(conditionType string, status metav1.ConditionStatus) metav1.Condition {
	return metav1.Condition{Type: conditionType, Status: status}
}

func testPacemakerNode(name string, online, member metav1.ConditionStatus) etcdv1.PacemakerClusterNodeStatus {
	return etcdv1.PacemakerClusterNodeStatus{
		NodeName: name,
		Conditions: []metav1.Condition{
			testNodeCondition(etcdv1.NodeOnlineConditionType, online),
			testNodeCondition(etcdv1.NodeMemberConditionType, member),
		},
	}
}

func validTargetFailureObservation(now time.Time, removed bool) targetFailureObservation {
	condition := &operatorv1.OperatorCondition{
		Type:               PacemakerHealthCheckDegradedCondition,
		Status:             operatorv1.ConditionTrue,
		LastTransitionTime: metav1.NewTime(now),
		Reason:             pacemakerUnhealthyReason,
		Message:            "Node " + testTargetNode + " is offline",
	}
	nodes := []etcdv1.PacemakerClusterNodeStatus{
		testPacemakerNode(testTargetNode, metav1.ConditionFalse, metav1.ConditionTrue),
		testPacemakerNode(testSurvivingNode, metav1.ConditionTrue, metav1.ConditionTrue),
	}
	nodeCount := metav1.Condition{
		Type:    etcdv1.ClusterNodeCountAsExpectedConditionType,
		Status:  metav1.ConditionTrue,
		Reason:  etcdv1.ClusterNodeCountAsExpectedReasonAsExpected,
		Message: "Expected 2 nodes, found 2",
	}
	if removed {
		condition.Message = insufficientNodesHealthMessage
		nodes = []etcdv1.PacemakerClusterNodeStatus{
			testPacemakerNode(testSurvivingNode, metav1.ConditionTrue, metav1.ConditionTrue),
		}
		nodeCount.Status = metav1.ConditionFalse
		nodeCount.Reason = etcdv1.ClusterNodeCountAsExpectedReasonInsufficientNodes
		nodeCount.Message = insufficientNodesCountMessage
	}
	return targetFailureObservation{
		condition: condition,
		cluster: &etcdv1.PacemakerCluster{
			ObjectMeta: metav1.ObjectMeta{ResourceVersion: testCurrentResourceVersion},
			Status: etcdv1.PacemakerClusterStatus{
				LastUpdated: metav1.NewTime(now),
				Conditions:  []metav1.Condition{nodeCount},
				Nodes:       &nodes,
			},
		},
	}
}

func testNode(pc *etcdv1.PacemakerCluster, name string) *etcdv1.PacemakerClusterNodeStatus {
	if pc.Status.Nodes == nil {
		return nil
	}
	for i := range *pc.Status.Nodes {
		if (*pc.Status.Nodes)[i].NodeName == name {
			return &(*pc.Status.Nodes)[i]
		}
	}
	return nil
}

func setTestNodeCondition(pc *etcdv1.PacemakerCluster, nodeName, conditionType string, status metav1.ConditionStatus) {
	node := testNode(pc, nodeName)
	for i := range node.Conditions {
		if node.Conditions[i].Type == conditionType {
			node.Conditions[i].Status = status
			return
		}
	}
	node.Conditions = append(node.Conditions, testNodeCondition(conditionType, status))
}

func removeTestNodeCondition(pc *etcdv1.PacemakerCluster, nodeName, conditionType string) {
	node := testNode(pc, nodeName)
	conditions := node.Conditions[:0]
	for i := range node.Conditions {
		if node.Conditions[i].Type != conditionType {
			conditions = append(conditions, node.Conditions[i])
		}
	}
	node.Conditions = conditions
}

func testNodeCountCondition(pc *etcdv1.PacemakerCluster) *metav1.Condition {
	for i := range pc.Status.Conditions {
		if pc.Status.Conditions[i].Type == etcdv1.ClusterNodeCountAsExpectedConditionType {
			return &pc.Status.Conditions[i]
		}
	}
	return nil
}

func runTargetFailureClassificationCases(t *testing.T, removed bool, tests []targetFailureClassificationCase) {
	t.Helper()
	now := time.Date(2026, 10, 2, 22, 0, 58, 0, time.UTC)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			observation := validTargetFailureObservation(now, removed)
			if tt.mutate != nil {
				tt.mutate(&observation)
			}
			mode, err := classifyPacemakerTargetFailure(
				observation.condition,
				observation.cluster,
				testTargetNode,
				testSurvivingNode,
			)
			if tt.wantErrText == "" {
				if err != nil {
					t.Fatalf("classifyPacemakerTargetFailure() unexpected error: %v", err)
				}
				if mode != tt.wantMode {
					t.Fatalf("classifyPacemakerTargetFailure() mode = %q, want %q", mode, tt.wantMode)
				}
				return
			}
			if err == nil {
				t.Fatalf("classifyPacemakerTargetFailure() unexpectedly accepted mode %q", mode)
			}
			if !strings.Contains(err.Error(), tt.wantErrText) {
				t.Fatalf("classifyPacemakerTargetFailure() error = %q, want substring %q", err, tt.wantErrText)
			}
		})
	}
}

func TestClassifyPacemakerTargetFailureCondition(t *testing.T) {
	runTargetFailureClassificationCases(t, false, []targetFailureClassificationCase{
		{name: "valid condition", wantMode: PacemakerTargetOffline},
		{
			name: "condition absent",
			mutate: func(observation *targetFailureObservation) {
				observation.condition = nil
			},
			wantErrText: "condition is absent",
		},
		{
			name: "condition false",
			mutate: func(observation *targetFailureObservation) {
				observation.condition.Status = operatorv1.ConditionFalse
			},
			wantErrText: "expected True",
		},
		{
			name: "condition unknown",
			mutate: func(observation *targetFailureObservation) {
				observation.condition.Status = operatorv1.ConditionUnknown
			},
			wantErrText: "expected True",
		},
		{
			name: "condition has unrelated reason",
			mutate: func(observation *targetFailureObservation) {
				observation.condition.Reason = "OtherReason"
			},
			wantErrText: "expected \"PacemakerUnhealthy\"",
		},
		{
			name: "PacemakerCluster is nil",
			mutate: func(observation *targetFailureObservation) {
				observation.cluster = nil
			},
			wantErrText: "PacemakerCluster is nil",
		},
		{
			name: "PacemakerCluster timestamp is zero",
			mutate: func(observation *targetFailureObservation) {
				observation.cluster.Status.LastUpdated = metav1.Time{}
			},
			wantErrText: "lastUpdated is zero",
		},
	})
}

func TestClassifyPacemakerTargetFailureOffline(t *testing.T) {
	runTargetFailureClassificationCases(t, false, []targetFailureClassificationCase{
		{name: "exact target offline", wantMode: PacemakerTargetOffline},
		{
			name: "producer joined maintenance error",
			mutate: func(observation *targetFailureObservation) {
				observation.condition.Message = "Cluster is unhealthy: Cluster is in maintenance mode; Node master-0 is offline"
				observation.cluster.Status.Conditions = append(observation.cluster.Status.Conditions,
					testNodeCondition(etcdv1.ClusterInServiceConditionType, metav1.ConditionFalse))
			},
			wantMode: PacemakerTargetOffline,
		},
		{
			name: "wrong node named offline",
			mutate: func(observation *targetFailureObservation) {
				observation.condition.Message = "Node master-1 is offline"
			},
			wantErrText: "conflicting offline component",
		},
		{
			name: "target and survivor both named offline",
			mutate: func(observation *targetFailureObservation) {
				observation.condition.Message += "; Node master-1 is offline"
			},
			wantErrText: "conflicting offline component",
		},
		{
			name: "offline and removed components conflict",
			mutate: func(observation *targetFailureObservation) {
				observation.condition.Message += "; " + insufficientNodesHealthMessage
			},
			wantErrText: "conflicting offline and removed",
		},
		{
			name: "offline component repeated",
			mutate: func(observation *targetFailureObservation) {
				observation.condition.Message += "; Node master-0 is offline"
			},
			wantErrText: "repeats a target-failure component",
		},
		{
			name: "target online condition unknown",
			mutate: func(observation *targetFailureObservation) {
				setTestNodeCondition(observation.cluster, testTargetNode, etcdv1.NodeOnlineConditionType, metav1.ConditionUnknown)
			},
			wantErrText: "expected False",
		},
		{
			name: "target online condition true",
			mutate: func(observation *targetFailureObservation) {
				setTestNodeCondition(observation.cluster, testTargetNode, etcdv1.NodeOnlineConditionType, metav1.ConditionTrue)
			},
			wantErrText: "expected False",
		},
		{
			name: "target online condition missing",
			mutate: func(observation *targetFailureObservation) {
				removeTestNodeCondition(observation.cluster, testTargetNode, etcdv1.NodeOnlineConditionType)
			},
			wantErrText: "missing Online condition",
		},
		{
			name: "survivor online condition false",
			mutate: func(observation *targetFailureObservation) {
				setTestNodeCondition(observation.cluster, testSurvivingNode, etcdv1.NodeOnlineConditionType, metav1.ConditionFalse)
			},
			wantErrText: "expected True",
		},
		{
			name: "survivor online condition unknown",
			mutate: func(observation *targetFailureObservation) {
				setTestNodeCondition(observation.cluster, testSurvivingNode, etcdv1.NodeOnlineConditionType, metav1.ConditionUnknown)
			},
			wantErrText: "expected True",
		},
		{
			name: "survivor online condition missing",
			mutate: func(observation *targetFailureObservation) {
				removeTestNodeCondition(observation.cluster, testSurvivingNode, etcdv1.NodeOnlineConditionType)
			},
			wantErrText: "missing Online condition",
		},
		{
			name: "survivor member condition false",
			mutate: func(observation *targetFailureObservation) {
				setTestNodeCondition(observation.cluster, testSurvivingNode, etcdv1.NodeMemberConditionType, metav1.ConditionFalse)
			},
			wantErrText: "expected True",
		},
		{
			name: "survivor member condition unknown",
			mutate: func(observation *targetFailureObservation) {
				setTestNodeCondition(observation.cluster, testSurvivingNode, etcdv1.NodeMemberConditionType, metav1.ConditionUnknown)
			},
			wantErrText: "expected True",
		},
		{
			name: "survivor member condition missing",
			mutate: func(observation *targetFailureObservation) {
				removeTestNodeCondition(observation.cluster, testSurvivingNode, etcdv1.NodeMemberConditionType)
			},
			wantErrText: "missing Member condition",
		},
		{
			name: "nodes nil",
			mutate: func(observation *targetFailureObservation) {
				observation.cluster.Status.Nodes = nil
			},
			wantErrText: "nodes are nil",
		},
		{
			name: "nodes empty",
			mutate: func(observation *targetFailureObservation) {
				nodes := []etcdv1.PacemakerClusterNodeStatus{}
				observation.cluster.Status.Nodes = &nodes
			},
			wantErrText: "has 0 nodes",
		},
		{
			name: "nodes duplicate target",
			mutate: func(observation *targetFailureObservation) {
				nodes := []etcdv1.PacemakerClusterNodeStatus{
					testPacemakerNode(testTargetNode, metav1.ConditionFalse, metav1.ConditionTrue),
					testPacemakerNode(testTargetNode, metav1.ConditionFalse, metav1.ConditionTrue),
				}
				observation.cluster.Status.Nodes = &nodes
			},
			wantErrText: "duplicate node",
		},
		{
			name: "nodes include unrelated node",
			mutate: func(observation *targetFailureObservation) {
				nodes := []etcdv1.PacemakerClusterNodeStatus{
					testPacemakerNode(testTargetNode, metav1.ConditionFalse, metav1.ConditionTrue),
					testPacemakerNode("master-2", metav1.ConditionTrue, metav1.ConditionTrue),
				}
				observation.cluster.Status.Nodes = &nodes
			},
			wantErrText: "unexpected node",
		},
		{
			name: "node name empty",
			mutate: func(observation *targetFailureObservation) {
				(*observation.cluster.Status.Nodes)[0].NodeName = ""
			},
			wantErrText: "empty name",
		},
		{
			name: "target absent",
			mutate: func(observation *targetFailureObservation) {
				nodes := []etcdv1.PacemakerClusterNodeStatus{
					testPacemakerNode(testSurvivingNode, metav1.ConditionTrue, metav1.ConditionTrue),
				}
				observation.cluster.Status.Nodes = &nodes
			},
			wantErrText: "expected exactly 2",
		},
		{
			name: "node count not as expected",
			mutate: func(observation *targetFailureObservation) {
				testNodeCountCondition(observation.cluster).Status = metav1.ConditionFalse
			},
			wantErrText: "expected True",
		},
	})
}

func TestClassifyPacemakerTargetFailureRemoved(t *testing.T) {
	runTargetFailureClassificationCases(t, true, []targetFailureClassificationCase{
		{name: "exact target removed", wantMode: PacemakerTargetRemoved},
		{
			name: "producer joined survivor health error",
			mutate: func(observation *targetFailureObservation) {
				observation.condition.Message += "; master-1 node is unhealthy: fencing unavailable (no agents running)"
				setTestNodeCondition(observation.cluster, testSurvivingNode, etcdv1.NodeHealthyConditionType, metav1.ConditionFalse)
				setTestNodeCondition(observation.cluster, testSurvivingNode, etcdv1.NodeFencingAvailableConditionType, metav1.ConditionFalse)
			},
			wantMode: PacemakerTargetRemoved,
		},
		{
			name: "old True transition and collector clock behind are accepted",
			mutate: func(observation *targetFailureObservation) {
				observation.condition.LastTransitionTime = metav1.NewTime(time.Date(2026, 10, 2, 20, 0, 0, 0, time.UTC))
				observation.cluster.Status.LastUpdated = metav1.NewTime(time.Date(2026, 10, 2, 21, 59, 0, 0, time.UTC))
			},
			wantMode: PacemakerTargetRemoved,
		},
		{
			name: "unrelated insufficient text",
			mutate: func(observation *targetFailureObservation) {
				observation.condition.Message = "Insufficient nodes"
			},
			wantErrText: "does not contain the exact target",
		},
		{
			name: "lookalike insufficient component",
			mutate: func(observation *targetFailureObservation) {
				observation.condition.Message = "Cluster is unhealthy: Insufficient nodes in cluster (expected 2, found 0)"
			},
			wantErrText: "conflicting insufficient-node component",
		},
		{
			name: "exact and lookalike insufficient components",
			mutate: func(observation *targetFailureObservation) {
				observation.condition.Message += "; Cluster is unhealthy: Insufficient nodes in cluster (expected 2, found 0)"
			},
			wantErrText: "conflicting insufficient-node component",
		},
		{
			name: "removed and target offline components conflict",
			mutate: func(observation *targetFailureObservation) {
				observation.condition.Message += "; Node master-0 is offline"
			},
			wantErrText: "conflicting offline and removed",
		},
		{
			name: "removed component repeated",
			mutate: func(observation *targetFailureObservation) {
				observation.condition.Message += "; " + insufficientNodesHealthMessage
			},
			wantErrText: "repeats a target-failure component",
		},
		{
			name: "wrong survivor",
			mutate: func(observation *targetFailureObservation) {
				(*observation.cluster.Status.Nodes)[0].NodeName = "master-2"
			},
			wantErrText: "unexpected node",
		},
		{
			name: "target still present",
			mutate: func(observation *targetFailureObservation) {
				*observation.cluster.Status.Nodes = append(*observation.cluster.Status.Nodes,
					testPacemakerNode(testTargetNode, metav1.ConditionFalse, metav1.ConditionTrue))
			},
			wantErrText: "expected exactly 1",
		},
		{
			name: "three nodes present",
			mutate: func(observation *targetFailureObservation) {
				nodes := []etcdv1.PacemakerClusterNodeStatus{
					testPacemakerNode(testSurvivingNode, metav1.ConditionTrue, metav1.ConditionTrue),
					testPacemakerNode(testTargetNode, metav1.ConditionFalse, metav1.ConditionTrue),
					testPacemakerNode("master-2", metav1.ConditionTrue, metav1.ConditionTrue),
				}
				observation.cluster.Status.Nodes = &nodes
			},
			wantErrText: "expected exactly 1",
		},
		{
			name: "survivor online false",
			mutate: func(observation *targetFailureObservation) {
				setTestNodeCondition(observation.cluster, testSurvivingNode, etcdv1.NodeOnlineConditionType, metav1.ConditionFalse)
			},
			wantErrText: "expected True",
		},
		{
			name: "survivor online unknown",
			mutate: func(observation *targetFailureObservation) {
				setTestNodeCondition(observation.cluster, testSurvivingNode, etcdv1.NodeOnlineConditionType, metav1.ConditionUnknown)
			},
			wantErrText: "expected True",
		},
		{
			name: "survivor online missing",
			mutate: func(observation *targetFailureObservation) {
				removeTestNodeCondition(observation.cluster, testSurvivingNode, etcdv1.NodeOnlineConditionType)
			},
			wantErrText: "missing Online condition",
		},
		{
			name: "survivor member false",
			mutate: func(observation *targetFailureObservation) {
				setTestNodeCondition(observation.cluster, testSurvivingNode, etcdv1.NodeMemberConditionType, metav1.ConditionFalse)
			},
			wantErrText: "expected True",
		},
		{
			name: "survivor member unknown",
			mutate: func(observation *targetFailureObservation) {
				setTestNodeCondition(observation.cluster, testSurvivingNode, etcdv1.NodeMemberConditionType, metav1.ConditionUnknown)
			},
			wantErrText: "expected True",
		},
		{
			name: "survivor member missing",
			mutate: func(observation *targetFailureObservation) {
				removeTestNodeCondition(observation.cluster, testSurvivingNode, etcdv1.NodeMemberConditionType)
			},
			wantErrText: "missing Member condition",
		},
		{
			name: "node count condition missing",
			mutate: func(observation *targetFailureObservation) {
				observation.cluster.Status.Conditions = nil
			},
			wantErrText: "missing NodeCountAsExpected condition",
		},
		{
			name: "node count status true",
			mutate: func(observation *targetFailureObservation) {
				testNodeCountCondition(observation.cluster).Status = metav1.ConditionTrue
			},
			wantErrText: "expected False",
		},
		{
			name: "node count reason excessive",
			mutate: func(observation *targetFailureObservation) {
				testNodeCountCondition(observation.cluster).Reason = etcdv1.ClusterNodeCountAsExpectedReasonExcessiveNodes
			},
			wantErrText: "expected False reason=\"InsufficientNodes\"",
		},
		{
			name: "node count reason unknown",
			mutate: func(observation *targetFailureObservation) {
				testNodeCountCondition(observation.cluster).Reason = "UnknownCount"
			},
			wantErrText: "reason=\"UnknownCount\"",
		},
		{
			name: "node count message wrong",
			mutate: func(observation *targetFailureObservation) {
				testNodeCountCondition(observation.cluster).Message = "Expected 2 nodes, found 0"
			},
			wantErrText: "Expected 2 nodes, found 0",
		},
	})
}

func testEtcdWithCondition(condition *operatorv1.OperatorCondition) *operatorv1.Etcd {
	etcd := &operatorv1.Etcd{}
	if condition != nil {
		etcd.Status.Conditions = []operatorv1.OperatorCondition{*condition}
	}
	return etcd
}

func staticTargetFailureReader(condition *operatorv1.OperatorCondition, cluster *etcdv1.PacemakerCluster) pacemakerTargetFailureReader {
	etcd := testEtcdWithCondition(condition)
	return pacemakerTargetFailureReader{
		getPacemakerCluster: func() (*etcdv1.PacemakerCluster, error) { return cluster.DeepCopy(), nil },
		getEtcdOperator:     func() (*operatorv1.Etcd, error) { return etcd.DeepCopy(), nil },
	}
}

func TestCapturePacemakerTargetFailureBaseline(t *testing.T) {
	now := time.Date(2026, 10, 2, 21, 54, 0, 0, time.UTC)
	observation := validTargetFailureObservation(now, false)
	observation.cluster.ResourceVersion = testBaselineResourceVersion
	healthy := &operatorv1.OperatorCondition{
		Type:   PacemakerHealthCheckDegradedCondition,
		Status: operatorv1.ConditionFalse,
	}

	tests := []struct {
		name        string
		condition   *operatorv1.OperatorCondition
		mutate      func(*etcdv1.PacemakerCluster)
		wantErrText string
	}{
		{name: "healthy stable baseline", condition: healthy},
		{name: "condition absent", wantErrText: "missing PacemakerHealthCheckDegraded"},
		{
			name: "condition already degraded",
			condition: &operatorv1.OperatorCondition{
				Type:    PacemakerHealthCheckDegradedCondition,
				Status:  operatorv1.ConditionTrue,
				Reason:  pacemakerUnhealthyReason,
				Message: "Node master-1 is offline",
			},
			wantErrText: "requires PacemakerHealthCheckDegraded=False",
		},
		{
			name: "condition unknown",
			condition: &operatorv1.OperatorCondition{
				Type:   PacemakerHealthCheckDegradedCondition,
				Status: operatorv1.ConditionUnknown,
			},
			wantErrText: "Status=Unknown",
		},
		{
			name:      "empty resourceVersion",
			condition: healthy,
			mutate: func(cluster *etcdv1.PacemakerCluster) {
				cluster.ResourceVersion = ""
			},
			wantErrText: "empty resourceVersion",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cluster := observation.cluster.DeepCopy()
			if tt.mutate != nil {
				tt.mutate(cluster)
			}
			resourceVersion, err := capturePacemakerTargetFailureBaseline(staticTargetFailureReader(tt.condition, cluster))
			if tt.wantErrText == "" {
				if err != nil {
					t.Fatalf("capturePacemakerTargetFailureBaseline() unexpected error: %v", err)
				}
				if resourceVersion != testBaselineResourceVersion {
					t.Fatalf("capturePacemakerTargetFailureBaseline() = %q, want %q", resourceVersion, testBaselineResourceVersion)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErrText) {
				t.Fatalf("capturePacemakerTargetFailureBaseline() error = %v, want substring %q", err, tt.wantErrText)
			}
		})
	}
}

func TestCapturePacemakerTargetFailureBaselineReadFailures(t *testing.T) {
	now := time.Now().UTC()
	observation := validTargetFailureObservation(now, false)
	healthy := &operatorv1.OperatorCondition{Type: PacemakerHealthCheckDegradedCondition, Status: operatorv1.ConditionFalse}
	etcd := testEtcdWithCondition(healthy)
	readErr := errors.New("API unavailable")

	tests := []struct {
		name        string
		reader      pacemakerTargetFailureReader
		wantErrText string
	}{
		{
			name: "first PacemakerCluster read",
			reader: pacemakerTargetFailureReader{
				getPacemakerCluster: func() (*etcdv1.PacemakerCluster, error) { return nil, readErr },
				getEtcdOperator:     func() (*operatorv1.Etcd, error) { return etcd.DeepCopy(), nil },
			},
			wantErrText: "before Etcd condition: API unavailable",
		},
		{
			name: "Etcd read",
			reader: pacemakerTargetFailureReader{
				getPacemakerCluster: func() (*etcdv1.PacemakerCluster, error) { return observation.cluster.DeepCopy(), nil },
				getEtcdOperator:     func() (*operatorv1.Etcd, error) { return nil, readErr },
			},
			wantErrText: "get Etcd operator: API unavailable",
		},
		{
			name: "second PacemakerCluster read",
			reader: func() pacemakerTargetFailureReader {
				calls := 0
				return pacemakerTargetFailureReader{
					getPacemakerCluster: func() (*etcdv1.PacemakerCluster, error) {
						calls++
						if calls == 2 {
							return nil, readErr
						}
						return observation.cluster.DeepCopy(), nil
					},
					getEtcdOperator: func() (*operatorv1.Etcd, error) { return etcd.DeepCopy(), nil },
				}
			}(),
			wantErrText: "after Etcd condition: API unavailable",
		},
		{
			name: "PacemakerCluster changes around Etcd read",
			reader: func() pacemakerTargetFailureReader {
				calls := 0
				return pacemakerTargetFailureReader{
					getPacemakerCluster: func() (*etcdv1.PacemakerCluster, error) {
						calls++
						cluster := observation.cluster.DeepCopy()
						if calls == 2 {
							cluster.ResourceVersion = "next-rv"
						}
						return cluster, nil
					},
					getEtcdOperator: func() (*operatorv1.Etcd, error) { return etcd.DeepCopy(), nil },
				}
			}(),
			wantErrText: "changed while reading Etcd condition",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := capturePacemakerTargetFailureBaseline(tt.reader)
			if err == nil || !strings.Contains(err.Error(), tt.wantErrText) {
				t.Fatalf("capturePacemakerTargetFailureBaseline() error = %v, want substring %q", err, tt.wantErrText)
			}
		})
	}
}

func TestObservePacemakerTargetFailureFreshness(t *testing.T) {
	now := time.Date(2026, 10, 2, 22, 0, 58, 0, time.UTC)
	tests := []struct {
		name        string
		baselineRV  string
		prepare     func(targetFailureObservation) pacemakerTargetFailureReader
		wantMode    PacemakerTargetFailureMode
		wantErrText string
	}{
		{
			name:       "opaque different resourceVersion accepted without ordering",
			baselineRV: "999999",
			prepare: func(observation targetFailureObservation) pacemakerTargetFailureReader {
				observation.cluster.ResourceVersion = "1"
				observation.condition.LastTransitionTime = metav1.NewTime(now.Add(-2 * time.Hour))
				observation.cluster.Status.LastUpdated = metav1.NewTime(now.Add(-time.Minute))
				return staticTargetFailureReader(observation.condition, observation.cluster)
			},
			wantMode: PacemakerTargetRemoved,
		},
		{
			name:       "unchanged baseline resourceVersion rejected",
			baselineRV: testCurrentResourceVersion,
			prepare: func(observation targetFailureObservation) pacemakerTargetFailureReader {
				return staticTargetFailureReader(observation.condition, observation.cluster)
			},
			wantErrText: "unchanged from the pre-destruction baseline",
		},
		{
			name:       "snapshot changing around Etcd read rejected",
			baselineRV: testBaselineResourceVersion,
			prepare: func(observation targetFailureObservation) pacemakerTargetFailureReader {
				calls := 0
				etcd := testEtcdWithCondition(observation.condition)
				return pacemakerTargetFailureReader{
					getPacemakerCluster: func() (*etcdv1.PacemakerCluster, error) {
						calls++
						cluster := observation.cluster.DeepCopy()
						if calls == 2 {
							cluster.ResourceVersion = "next-rv"
						}
						return cluster, nil
					},
					getEtcdOperator: func() (*operatorv1.Etcd, error) { return etcd.DeepCopy(), nil },
				}
			},
			wantErrText: "resourceVersion \"current-rv\" to \"next-rv\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			observation := validTargetFailureObservation(now, true)
			mode, err := observePacemakerTargetFailure(
				tt.prepare(observation),
				testTargetNode,
				testSurvivingNode,
				tt.baselineRV,
			)
			if tt.wantErrText == "" {
				if err != nil {
					t.Fatalf("observePacemakerTargetFailure() unexpected error: %v", err)
				}
				if mode != tt.wantMode {
					t.Fatalf("observePacemakerTargetFailure() mode = %q, want %q", mode, tt.wantMode)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErrText) {
				t.Fatalf("observePacemakerTargetFailure() error = %v, want substring %q", err, tt.wantErrText)
			}
		})
	}
}

func TestWaitForPacemakerTargetFailureRejectsInvalidArgumentsBeforePolling(t *testing.T) {
	tests := []struct {
		name       string
		targetNode string
		survivor   string
		baselineRV string
	}{
		{name: "empty target", survivor: testSurvivingNode, baselineRV: testBaselineResourceVersion},
		{name: "empty survivor", targetNode: testTargetNode, baselineRV: testBaselineResourceVersion},
		{name: "same target and survivor", targetNode: testTargetNode, survivor: testTargetNode, baselineRV: testBaselineResourceVersion},
		{name: "empty baseline", targetNode: testTargetNode, survivor: testSurvivingNode},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			readCalls := 0
			reader := pacemakerTargetFailureReader{
				getPacemakerCluster: func() (*etcdv1.PacemakerCluster, error) {
					readCalls++
					return nil, errors.New("must not read")
				},
				getEtcdOperator: func() (*operatorv1.Etcd, error) {
					readCalls++
					return nil, errors.New("must not read")
				},
			}
			_, err := waitForPacemakerTargetFailure(reader, tt.targetNode, tt.survivor, tt.baselineRV, time.Second, time.Millisecond)
			if err == nil {
				t.Fatal("waitForPacemakerTargetFailure() unexpectedly accepted invalid arguments")
			}
			if readCalls != 0 {
				t.Fatalf("waitForPacemakerTargetFailure() made %d reads, want fail-fast with 0", readCalls)
			}
		})
	}
}

func TestWaitForPacemakerTargetFailureRetriesTransientReads(t *testing.T) {
	now := time.Now().UTC()
	observation := validTargetFailureObservation(now, false)
	etcd := testEtcdWithCondition(observation.condition)
	transientErr := errors.New("transient API error")

	tests := []struct {
		name       string
		makeReader func() pacemakerTargetFailureReader
	}{
		{
			name: "first PacemakerCluster read",
			makeReader: func() pacemakerTargetFailureReader {
				calls := 0
				return pacemakerTargetFailureReader{
					getPacemakerCluster: func() (*etcdv1.PacemakerCluster, error) {
						calls++
						if calls == 1 {
							return nil, transientErr
						}
						return observation.cluster.DeepCopy(), nil
					},
					getEtcdOperator: func() (*operatorv1.Etcd, error) { return etcd.DeepCopy(), nil },
				}
			},
		},
		{
			name: "Etcd operator read",
			makeReader: func() pacemakerTargetFailureReader {
				calls := 0
				return pacemakerTargetFailureReader{
					getPacemakerCluster: func() (*etcdv1.PacemakerCluster, error) { return observation.cluster.DeepCopy(), nil },
					getEtcdOperator: func() (*operatorv1.Etcd, error) {
						calls++
						if calls == 1 {
							return nil, transientErr
						}
						return etcd.DeepCopy(), nil
					},
				}
			},
		},
		{
			name: "second PacemakerCluster read",
			makeReader: func() pacemakerTargetFailureReader {
				calls := 0
				return pacemakerTargetFailureReader{
					getPacemakerCluster: func() (*etcdv1.PacemakerCluster, error) {
						calls++
						if calls == 2 {
							return nil, transientErr
						}
						return observation.cluster.DeepCopy(), nil
					},
					getEtcdOperator: func() (*operatorv1.Etcd, error) { return etcd.DeepCopy(), nil },
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mode, err := waitForPacemakerTargetFailure(
				tt.makeReader(),
				testTargetNode,
				testSurvivingNode,
				testBaselineResourceVersion,
				time.Second,
				time.Millisecond,
			)
			if err != nil {
				t.Fatalf("waitForPacemakerTargetFailure() did not recover from transient read: %v", err)
			}
			if mode != PacemakerTargetOffline {
				t.Fatalf("waitForPacemakerTargetFailure() mode = %q, want %q", mode, PacemakerTargetOffline)
			}
		})
	}
}

func TestWaitForPacemakerTargetFailureReportsPersistentReadFailure(t *testing.T) {
	persistentErr := errors.New("persistent API error")
	reader := pacemakerTargetFailureReader{
		getPacemakerCluster: func() (*etcdv1.PacemakerCluster, error) { return nil, persistentErr },
		getEtcdOperator:     func() (*operatorv1.Etcd, error) { return nil, persistentErr },
	}

	_, err := waitForPacemakerTargetFailure(
		reader,
		testTargetNode,
		testSurvivingNode,
		testBaselineResourceVersion,
		20*time.Millisecond,
		time.Millisecond,
	)
	if err == nil || !strings.Contains(err.Error(), "last: get PacemakerCluster before Etcd condition: persistent API error") {
		t.Fatalf("waitForPacemakerTargetFailure() error = %v, want last persistent read failure", err)
	}
}
