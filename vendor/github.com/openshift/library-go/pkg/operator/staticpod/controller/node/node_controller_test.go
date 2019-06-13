package node

import (
	"fmt"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"

	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/library-go/pkg/operator/condition"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
)

func fakeMasterNode(name string) *corev1.Node {
	n := &corev1.Node{}
	n.Name = name
	n.Labels = map[string]string{
		"node-role.kubernetes.io/master": "",
	}

	return n
}

func makeNodeNotReady(node *corev1.Node) *corev1.Node {
	con := corev1.NodeCondition{}
	con.Type = corev1.NodeReady
	con.Status = corev1.ConditionFalse
	node.Status.Conditions = append(node.Status.Conditions, con)
	return node
}

func validateCommonNodeControllerDegradedCondtion(con operatorv1.OperatorCondition) error {
	if con.Type != condition.NodeControllerDegradedConditionType {
		return fmt.Errorf("incorrect condition.type, expected NodeControllerDegraded, got %s", con.Type)
	}
	if con.Reason != "MasterNodesReady" {
		return fmt.Errorf("incorrect condition.reason, expected MasterNodesReady, got %s", con.Reason)
	}
	return nil
}

func TestNodeControllerDegradedConditionType(t *testing.T) {
	scenarios := []struct {
		name               string
		masterNodes        []runtime.Object
		evaluateNodeStatus func([]operatorv1.OperatorCondition) error
	}{
		// scenario 1
		{
			name:        "scenario 1: one unhealthy master node is reported",
			masterNodes: []runtime.Object{makeNodeNotReady(fakeMasterNode("test-node-1")), fakeMasterNode("test-node-2")},
			evaluateNodeStatus: func(conditions []operatorv1.OperatorCondition) error {
				if len(conditions) != 1 {
					return fmt.Errorf("expected exaclty 1 condition, got %d", len(conditions))
				}

				con := conditions[0]
				if err := validateCommonNodeControllerDegradedCondtion(con); err != nil {
					return err
				}
				if con.Status != operatorv1.ConditionTrue {
					return fmt.Errorf("incorrect condition.status, expected %v, got %v", operatorv1.ConditionTrue, con.Status)
				}
				expectedMsg := "The master node(s) \"test-node-1\" not ready"
				if con.Message != expectedMsg {
					return fmt.Errorf("incorrect condition.message, expected %s, got %s", expectedMsg, con.Message)
				}
				return nil
			},
		},

		// scenario 2
		{
			name:        "scenario 2: all master nodes are healthy",
			masterNodes: []runtime.Object{fakeMasterNode("test-node-1"), fakeMasterNode("test-node-2")},
			evaluateNodeStatus: func(conditions []operatorv1.OperatorCondition) error {
				if len(conditions) != 1 {
					return fmt.Errorf("expected exaclty 1 condition, got %d", len(conditions))
				}

				con := conditions[0]
				if err := validateCommonNodeControllerDegradedCondtion(con); err != nil {
					return err
				}
				if con.Status != operatorv1.ConditionFalse {
					return fmt.Errorf("incorrect condition.status, expected %v, got %v", operatorv1.ConditionFalse, con.Status)
				}
				expectedMsg := "All master node(s) are ready"
				if con.Message != expectedMsg {
					return fmt.Errorf("incorrect condition.message, expected %s, got %s", expectedMsg, con.Message)
				}
				return nil
			},
		},

		// scenario 3
		{
			name:        "scenario 3: multiple master nodes are unhealthy",
			masterNodes: []runtime.Object{makeNodeNotReady(fakeMasterNode("test-node-1")), fakeMasterNode("test-node-2"), makeNodeNotReady(fakeMasterNode("test-node-3"))},
			evaluateNodeStatus: func(conditions []operatorv1.OperatorCondition) error {
				if len(conditions) != 1 {
					return fmt.Errorf("expected exaclty 1 condition, got %d", len(conditions))
				}

				con := conditions[0]
				if err := validateCommonNodeControllerDegradedCondtion(con); err != nil {
					return err
				}
				if con.Status != operatorv1.ConditionTrue {
					return fmt.Errorf("incorrect condition.status, expected %v, got %v", operatorv1.ConditionTrue, con.Status)
				}
				expectedMsg := "The master node(s) \"test-node-1,test-node-3\" not ready"
				if con.Message != expectedMsg {
					return fmt.Errorf("incorrect condition.message, expected %s, got %s", expectedMsg, con.Message)
				}
				return nil
			},
		},
	}
	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			kubeClient := fake.NewSimpleClientset(scenario.masterNodes...)
			fakeLister := v1helpers.NewFakeNodeLister(kubeClient)
			kubeInformers := informers.NewSharedInformerFactory(kubeClient, 1*time.Minute)
			fakeStaticPodOperatorClient := v1helpers.NewFakeStaticPodOperatorClient(
				&operatorv1.StaticPodOperatorSpec{
					OperatorSpec: operatorv1.OperatorSpec{
						ManagementState: operatorv1.Managed,
					},
				},
				&operatorv1.StaticPodOperatorStatus{
					LatestAvailableRevision: 1,
				},
				nil,
				nil,
			)

			eventRecorder := events.NewRecorder(kubeClient.CoreV1().Events("test"), "test-operator", &corev1.ObjectReference{})

			c := NewNodeController(fakeStaticPodOperatorClient, kubeInformers, eventRecorder)
			// override the lister so we don't have to run the informer to list nodes
			c.nodeLister = fakeLister
			if err := c.sync(); err != nil {
				t.Fatal(err)
			}

			_, status, _, _ := fakeStaticPodOperatorClient.GetStaticPodOperatorState()

			if err := scenario.evaluateNodeStatus(status.OperatorStatus.Conditions); err != nil {
				t.Errorf("%s: failed to evaluate operator conditions: %v", scenario.name, err)
			}
		})

	}
}

func TestNewNodeController(t *testing.T) {
	tests := []struct {
		name               string
		startNodes         []runtime.Object
		startNodeStatus    []operatorv1.NodeStatus
		evaluateNodeStatus func([]operatorv1.NodeStatus) error
	}{
		{
			name:       "single-node",
			startNodes: []runtime.Object{fakeMasterNode("test-node-1")},
			evaluateNodeStatus: func(s []operatorv1.NodeStatus) error {
				if len(s) != 1 {
					return fmt.Errorf("expected 1 node status, got %d", len(s))
				}
				if s[0].NodeName != "test-node-1" {
					return fmt.Errorf("expected 'test-node-1' as node name, got %q", s[0].NodeName)
				}
				return nil
			},
		},
		{
			name:       "multi-node",
			startNodes: []runtime.Object{fakeMasterNode("test-node-1"), fakeMasterNode("test-node-2"), fakeMasterNode("test-node-3")},
			startNodeStatus: []operatorv1.NodeStatus{
				{
					NodeName: "test-node-1",
				},
			},
			evaluateNodeStatus: func(s []operatorv1.NodeStatus) error {
				if len(s) != 3 {
					return fmt.Errorf("expected 3 node status, got %d", len(s))
				}
				if s[0].NodeName != "test-node-1" {
					return fmt.Errorf("expected first node to be test-node-1, got %q", s[0].NodeName)
				}
				if s[1].NodeName != "test-node-2" {
					return fmt.Errorf("expected second node to be test-node-2, got %q", s[1].NodeName)
				}
				return nil
			},
		},
		{
			name:       "single-node-removed",
			startNodes: []runtime.Object{},
			startNodeStatus: []operatorv1.NodeStatus{
				{
					NodeName: "lost-node",
				},
			},
			evaluateNodeStatus: func(s []operatorv1.NodeStatus) error {
				if len(s) != 0 {
					return fmt.Errorf("expected no node status, got %d", len(s))
				}
				return nil
			},
		},
		{
			name:       "no-op",
			startNodes: []runtime.Object{fakeMasterNode("test-node-1")},
			startNodeStatus: []operatorv1.NodeStatus{
				{
					NodeName: "test-node-1",
				},
			},
			evaluateNodeStatus: func(s []operatorv1.NodeStatus) error {
				if len(s) != 1 {
					return fmt.Errorf("expected one node status, got %d", len(s))
				}
				return nil
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			kubeClient := fake.NewSimpleClientset(test.startNodes...)
			fakeLister := v1helpers.NewFakeNodeLister(kubeClient)
			kubeInformers := informers.NewSharedInformerFactory(kubeClient, 1*time.Minute)
			fakeStaticPodOperatorClient := v1helpers.NewFakeStaticPodOperatorClient(
				&operatorv1.StaticPodOperatorSpec{
					OperatorSpec: operatorv1.OperatorSpec{
						ManagementState: operatorv1.Managed,
					},
				},
				&operatorv1.StaticPodOperatorStatus{
					LatestAvailableRevision: 1,
					NodeStatuses:            test.startNodeStatus,
				},
				nil,
				nil,
			)

			eventRecorder := events.NewRecorder(kubeClient.CoreV1().Events("test"), "test-operator", &corev1.ObjectReference{})

			c := NewNodeController(fakeStaticPodOperatorClient, kubeInformers, eventRecorder)
			// override the lister so we don't have to run the informer to list nodes
			c.nodeLister = fakeLister
			if err := c.sync(); err != nil {
				t.Fatal(err)
			}

			_, status, _, _ := fakeStaticPodOperatorClient.GetStaticPodOperatorState()

			if err := test.evaluateNodeStatus(status.NodeStatuses); err != nil {
				t.Errorf("%s: failed to evaluate node status: %v", test.name, err)
			}
		})

	}
}
