package status

import (
	"reflect"
	"strings"
	"testing"
	"time"

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

func TestDegraded(t *testing.T) {

	threeMinutesAgo := metav1.NewTime(time.Now().Add(-3 * time.Minute))
	fiveSecondsAgo := metav1.NewTime(time.Now().Add(-2 * time.Second))
	yesterday := metav1.NewTime(time.Now().Add(-24 * time.Hour))

	testCases := []struct {
		name             string
		conditions       []operatorv1.OperatorCondition
		expectedType     configv1.ClusterStatusConditionType
		expectedStatus   configv1.ConditionStatus
		expectedMessages []string
		expectedReason   string
	}{
		{
			name:           "no data",
			conditions:     []operatorv1.OperatorCondition{},
			expectedStatus: configv1.ConditionUnknown,
			expectedReason: "NoData",
		},
		{
			name: "one not failing/within threshold",
			conditions: []operatorv1.OperatorCondition{
				{Type: "TypeADegraded", Status: operatorv1.ConditionFalse, LastTransitionTime: fiveSecondsAgo, Message: "a message from type a"},
			},
			expectedStatus: configv1.ConditionFalse,
			expectedReason: "AsExpected",
			expectedMessages: []string{
				"TypeADegraded: a message from type a",
			},
		},
		{
			name: "one not failing/beyond threshold",
			conditions: []operatorv1.OperatorCondition{
				{Type: "TypeADegraded", Status: operatorv1.ConditionFalse, LastTransitionTime: threeMinutesAgo, Message: "a message from type a"},
			},
			expectedStatus: configv1.ConditionFalse,
			expectedReason: "AsExpected",
			expectedMessages: []string{
				"TypeADegraded: a message from type a",
			},
		},
		{
			name: "one failing/within threshold",
			conditions: []operatorv1.OperatorCondition{
				{Type: "TypeADegraded", Status: operatorv1.ConditionTrue, LastTransitionTime: fiveSecondsAgo, Message: "a message from type a"},
			},
			expectedStatus: configv1.ConditionFalse,
			expectedReason: "AsExpected",
			expectedMessages: []string{
				"TypeADegraded: a message from type a",
			},
		},
		{
			name: "one failing/beyond threshold",
			conditions: []operatorv1.OperatorCondition{
				{Type: "TypeADegraded", Status: operatorv1.ConditionTrue, Message: "a message from type a", LastTransitionTime: threeMinutesAgo},
			},
			expectedStatus: configv1.ConditionTrue,
			expectedReason: "TypeADegraded",
			expectedMessages: []string{
				"TypeADegraded: a message from type a",
			},
		},
		{
			name: "two present/one failing/within threshold",
			conditions: []operatorv1.OperatorCondition{
				{Type: "TypeADegraded", Status: operatorv1.ConditionTrue, LastTransitionTime: fiveSecondsAgo, Message: "a message from type a"},
				{Type: "TypeBDegraded", Status: operatorv1.ConditionFalse, LastTransitionTime: yesterday},
			},
			expectedStatus: configv1.ConditionFalse,
			expectedReason: "AsExpected",
			expectedMessages: []string{
				"TypeADegraded: a message from type a",
			},
		},
		{
			name: "two present/one failing/beyond threshold",
			conditions: []operatorv1.OperatorCondition{
				{Type: "TypeADegraded", Status: operatorv1.ConditionTrue, LastTransitionTime: threeMinutesAgo, Message: "a message from type a"},
				{Type: "TypeBDegraded", Status: operatorv1.ConditionFalse, LastTransitionTime: yesterday},
			},
			expectedStatus: configv1.ConditionTrue,
			expectedReason: "TypeADegraded",
			expectedMessages: []string{
				"TypeADegraded: a message from type a",
			},
		},
		{
			name: "two present/second one failing/within threshold",
			conditions: []operatorv1.OperatorCondition{
				{Type: "TypeADegraded", Status: operatorv1.ConditionFalse, LastTransitionTime: yesterday},
				{Type: "TypeBDegraded", Status: operatorv1.ConditionTrue, LastTransitionTime: fiveSecondsAgo, Message: "a message from type b"},
			},
			expectedStatus: configv1.ConditionFalse,
			expectedReason: "AsExpected",
			expectedMessages: []string{
				"TypeBDegraded: a message from type b",
			},
		},
		{
			name: "two present/second one failing/beyond threshold",
			conditions: []operatorv1.OperatorCondition{
				{Type: "TypeADegraded", Status: operatorv1.ConditionFalse, LastTransitionTime: yesterday},
				{Type: "TypeBDegraded", Status: operatorv1.ConditionTrue, LastTransitionTime: threeMinutesAgo, Message: "a message from type b"},
			},
			expectedStatus: configv1.ConditionTrue,
			expectedReason: "TypeBDegraded",
			expectedMessages: []string{
				"TypeBDegraded: a message from type b",
			},
		},
		{
			name: "many present/some failing/all within threshold",
			conditions: []operatorv1.OperatorCondition{
				{Type: "TypeADegraded", Status: operatorv1.ConditionFalse, LastTransitionTime: yesterday},
				{Type: "TypeBDegraded", Status: operatorv1.ConditionTrue, LastTransitionTime: fiveSecondsAgo, Message: "a message from type b\nanother message from type b"},
				{Type: "TypeCDegraded", Status: operatorv1.ConditionFalse, LastTransitionTime: threeMinutesAgo, Message: "a message from type c"},
				{Type: "TypeDDegraded", Status: operatorv1.ConditionTrue, LastTransitionTime: fiveSecondsAgo, Message: "a message from type d"},
			},
			expectedStatus: configv1.ConditionFalse,
			expectedReason: "AsExpected",
			expectedMessages: []string{
				"TypeBDegraded: a message from type b",
				"TypeBDegraded: another message from type b",
				"TypeCDegraded: a message from type c",
				"TypeDDegraded: a message from type d",
			},
		},
		{
			name: "many present/some failing some/within threshold",
			conditions: []operatorv1.OperatorCondition{
				{Type: "TypeADegraded", Status: operatorv1.ConditionFalse, LastTransitionTime: yesterday},
				{Type: "TypeBDegraded", Status: operatorv1.ConditionTrue, LastTransitionTime: fiveSecondsAgo, Message: "a message from type b\nanother message from type b"},
				{Type: "TypeCDegraded", Status: operatorv1.ConditionFalse, LastTransitionTime: threeMinutesAgo, Message: "a message from type c"},
				{Type: "TypeDDegraded", Status: operatorv1.ConditionTrue, LastTransitionTime: threeMinutesAgo, Message: "a message from type d"},
			},
			expectedStatus: configv1.ConditionTrue,
			expectedReason: "MultipleConditionsMatching",
			expectedMessages: []string{
				"TypeBDegraded: a message from type b",
				"TypeBDegraded: another message from type b",
				"TypeDDegraded: a message from type d",
			},
		},
		{
			name: "many present/some failing/all beyond threshold",
			conditions: []operatorv1.OperatorCondition{
				{Type: "TypeADegraded", Status: operatorv1.ConditionFalse, LastTransitionTime: yesterday},
				{Type: "TypeBDegraded", Status: operatorv1.ConditionTrue, LastTransitionTime: threeMinutesAgo, Message: "a message from type b\nanother message from type b"},
				{Type: "TypeCDegraded", Status: operatorv1.ConditionFalse, LastTransitionTime: threeMinutesAgo, Message: "a message from type c"},
				{Type: "TypeDDegraded", Status: operatorv1.ConditionTrue, LastTransitionTime: threeMinutesAgo, Message: "a message from type d"},
			},
			expectedStatus: configv1.ConditionTrue,
			expectedReason: "MultipleConditionsMatching",
			expectedMessages: []string{
				"TypeBDegraded: a message from type b",
				"TypeBDegraded: another message from type b",
				"TypeDDegraded: a message from type d",
			},
		},
		{
			name: "one progressing/within threshold",
			conditions: []operatorv1.OperatorCondition{
				{Type: "TypeAProgressing", Status: operatorv1.ConditionTrue, LastTransitionTime: fiveSecondsAgo, Message: "a message from type a"},
			},
			expectedType:   configv1.OperatorProgressing,
			expectedStatus: configv1.ConditionTrue,
			expectedReason: "TypeAProgressing",
			expectedMessages: []string{
				"TypeAProgressing: a message from type a",
			},
		},
		{
			name: "one not available/within threshold",
			conditions: []operatorv1.OperatorCondition{
				{Type: "TypeAAvailable", Status: operatorv1.ConditionFalse, LastTransitionTime: fiveSecondsAgo, Message: "a message from type a"},
			},
			expectedType:   configv1.OperatorAvailable,
			expectedStatus: configv1.ConditionFalse,
			expectedReason: "TypeAAvailable",
			expectedMessages: []string{
				"TypeAAvailable: a message from type a",
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			for _, condition := range tc.conditions {
				if condition.LastTransitionTime == (metav1.Time{}) {
					t.Fatal("LastTransitionTime not set.")
				}
			}
			if tc.expectedType == "" {
				tc.expectedType = configv1.OperatorDegraded
			}
			clusterOperator := &configv1.ClusterOperator{
				ObjectMeta: metav1.ObjectMeta{Name: "OPERATOR_NAME", ResourceVersion: "12"},
			}
			clusterOperatorClient := fake.NewSimpleClientset(clusterOperator)

			indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
			indexer.Add(clusterOperator)

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
			if tc.expectedStatus != "" {
				expectedCondition = &configv1.ClusterOperatorStatusCondition{
					Type:   tc.expectedType,
					Status: configv1.ConditionStatus(string(tc.expectedStatus)),
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

			actual := v1helpers.FindStatusCondition(result.Status.Conditions, tc.expectedType)
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
