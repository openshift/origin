package status

import (
	"reflect"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/client-go/tools/cache"

	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/client-go/config/clientset/versioned/fake"
	configv1listers "github.com/openshift/client-go/config/listers/config/v1"
	"github.com/openshift/library-go/pkg/config/clusteroperator/v1helpers"
	"github.com/openshift/library-go/pkg/operator/events"
)

func TestFailing(t *testing.T) {

	testCases := []struct {
		name                  string
		conditions            []operatorv1.OperatorCondition
		expectedFailingStatus configv1.ConditionStatus
		expectedMessages      []string
		expectedReason        string
	}{
		{
			name:                  "no data",
			conditions:            []operatorv1.OperatorCondition{},
			expectedFailingStatus: configv1.ConditionUnknown,
			expectedReason:        "NoData",
		},
		{
			name: "one failing false",
			conditions: []operatorv1.OperatorCondition{
				{Type: "TypeAFailing", Status: operatorv1.ConditionFalse},
			},
			expectedFailingStatus: configv1.ConditionFalse,
			expectedReason:        "AsExpected",
		},
		{
			name: "one failing true",
			conditions: []operatorv1.OperatorCondition{
				{Type: "TypeAFailing", Status: operatorv1.ConditionTrue},
			},
			expectedFailingStatus: configv1.ConditionTrue,
			expectedReason:        "TypeAFailing",
		},
		{
			name: "two present, one failing",
			conditions: []operatorv1.OperatorCondition{
				{Type: "TypeAFailing", Status: operatorv1.ConditionTrue, Message: "a message from type a"},
				{Type: "TypeBFailing", Status: operatorv1.ConditionFalse},
			},
			expectedFailingStatus: configv1.ConditionTrue,
			expectedReason:        "TypeAFailing",
			expectedMessages: []string{
				"TypeAFailing: a message from type a",
			},
		},
		{
			name: "two present, second one failing",
			conditions: []operatorv1.OperatorCondition{
				{Type: "TypeAFailing", Status: operatorv1.ConditionFalse},
				{Type: "TypeBFailing", Status: operatorv1.ConditionTrue, Message: "a message from type b"},
			},
			expectedFailingStatus: configv1.ConditionTrue,
			expectedReason:        "TypeBFailing",
			expectedMessages: []string{
				"TypeBFailing: a message from type b",
			},
		},
		{
			name: "many present, some failing",
			conditions: []operatorv1.OperatorCondition{
				{Type: "TypeAFailing", Status: operatorv1.ConditionFalse},
				{Type: "TypeBFailing", Status: operatorv1.ConditionTrue, Message: "a message from type b\nanother message from type b"},
				{Type: "TypeCFailing", Status: operatorv1.ConditionFalse, Message: "a message from type c"},
				{Type: "TypeDFailing", Status: operatorv1.ConditionTrue, Message: "a message from type d"},
			},
			expectedFailingStatus: configv1.ConditionTrue,
			expectedReason:        "MultipleConditionsMatching",
			expectedMessages: []string{
				"TypeBFailing: a message from type b",
				"TypeBFailing: another message from type b",
				"TypeDFailing: a message from type d",
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			clusteroperator := &configv1.ClusterOperator{
				ObjectMeta: metav1.ObjectMeta{Name: "OPERATOR_NAME", ResourceVersion: "12"},
			}
			clusterOperatorClient := fake.NewSimpleClientset(clusteroperator)

			indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
			indexer.Add(clusteroperator)

			statusClient := &statusClient{
				t: t,
				status: operatorv1.OperatorStatus{
					Conditions: tc.conditions,
				},
			}
			controller := &StatusSyncer{
				clusterOperatorName:   "OPERATOR_NAME",
				clusterOperatorClient: clusterOperatorClient.ConfigV1(),
				clusterOperatorLister: configv1listers.NewClusterOperatorLister(indexer),
				operatorClient:        statusClient,
				eventRecorder:         events.NewInMemoryRecorder("status"),
				versionGetter:         NewVersionGetter(),
			}
			if err := controller.sync(); err != nil {
				t.Errorf("unexpected sync error: %v", err)
				return
			}
			result, _ := clusterOperatorClient.ConfigV1().ClusterOperators().Get("OPERATOR_NAME", metav1.GetOptions{})

			var expectedCondition *configv1.ClusterOperatorStatusCondition
			if tc.expectedFailingStatus != "" {
				expectedCondition = &configv1.ClusterOperatorStatusCondition{
					Type:   configv1.OperatorFailing,
					Status: configv1.ConditionStatus(string(tc.expectedFailingStatus)),
				}
				if len(tc.expectedMessages) > 0 {
					expectedCondition.Message = strings.Join(tc.expectedMessages, "\n")
				}
				if len(tc.expectedReason) > 0 {
					expectedCondition.Reason = tc.expectedReason
				}
			}

			for i := range result.Status.Conditions {
				result.Status.Conditions[i].LastTransitionTime = metav1.Time{}
			}

			actual := v1helpers.FindStatusCondition(result.Status.Conditions, "Failing")
			if !reflect.DeepEqual(expectedCondition, actual) {
				t.Error(diff.ObjectDiff(expectedCondition, actual))
			}
		})
	}
}

// OperatorStatusProvider
type statusClient struct {
	t      *testing.T
	spec   operatorv1.OperatorSpec
	status operatorv1.OperatorStatus
}

func (c *statusClient) Informer() cache.SharedIndexInformer {
	c.t.Log("Informer called")
	return nil
}

func (c *statusClient) GetOperatorState() (*operatorv1.OperatorSpec, *operatorv1.OperatorStatus, string, error) {
	return &c.spec, &c.status, "", nil
}

func (c *statusClient) UpdateOperatorSpec(string, *operatorv1.OperatorSpec) (spec *operatorv1.OperatorSpec, resourceVersion string, err error) {
	panic("missing")
}

func (c *statusClient) UpdateOperatorStatus(string, *operatorv1.OperatorStatus) (status *operatorv1.OperatorStatus, err error) {
	panic("missing")
}
