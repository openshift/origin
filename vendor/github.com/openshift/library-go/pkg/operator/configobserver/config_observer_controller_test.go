package configobserver

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/davecgh/go-spew/spew"

	"github.com/ghodss/yaml"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/cache"

	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
)

func (c *fakeOperatorClient) GetOperatorState() (spec *operatorv1.OperatorSpec, status *operatorv1.OperatorStatus, resourceVersion string, err error) {
	return c.startingSpec, &operatorv1.OperatorStatus{}, "", nil

}
func (c *fakeOperatorClient) UpdateOperatorSpec(rv string, in *operatorv1.OperatorSpec) (spec *operatorv1.OperatorSpec, resourceVersion string, err error) {
	if c.specUpdateFailure != nil {
		return &operatorv1.OperatorSpec{}, rv, c.specUpdateFailure
	}
	c.spec = in
	return in, rv, c.specUpdateFailure
}
func (c *fakeOperatorClient) UpdateOperatorStatus(rv string, in *operatorv1.OperatorStatus) (status *operatorv1.OperatorStatus, resourceVersion string, err error) {
	c.status = in
	return in, rv, nil
}

type fakeOperatorClient struct {
	startingSpec      *operatorv1.OperatorSpec
	specUpdateFailure error

	status *operatorv1.OperatorStatus
	spec   *operatorv1.OperatorSpec
}

type fakeLister struct{}

func (l *fakeLister) PreRunHasSynced() []cache.InformerSynced {
	return []cache.InformerSynced{
		func() bool { return true },
	}
}

func TestSyncStatus(t *testing.T) {
	testCases := []struct {
		name       string
		fakeClient func() *fakeOperatorClient
		observers  []ObserveConfigFunc

		expectError            bool
		expectedObservedConfig *unstructured.Unstructured
		expectedCondition      *operatorv1.OperatorCondition
	}{
		{
			name: "HappyPath",
			fakeClient: func() *fakeOperatorClient {
				return &fakeOperatorClient{
					startingSpec: &operatorv1.OperatorSpec{},
				}
			},
			observers: []ObserveConfigFunc{
				func(listers Listers, existingConfig map[string]interface{}) (observedConfig map[string]interface{}, errs []error) {
					return map[string]interface{}{"foo": "one"}, nil
				},
				func(listers Listers, existingConfig map[string]interface{}) (observedConfig map[string]interface{}, errs []error) {
					return map[string]interface{}{"bar": "two"}, nil
				},
				func(listers Listers, existingConfig map[string]interface{}) (observedConfig map[string]interface{}, errs []error) {
					return map[string]interface{}{"baz": "three"}, nil
				},
			},

			expectError: false,
			expectedObservedConfig: &unstructured.Unstructured{Object: map[string]interface{}{
				"foo": "one",
				"bar": "two",
				"baz": "three",
			}},
			expectedCondition: &operatorv1.OperatorCondition{
				Type:   operatorStatusTypeConfigObservationFailing,
				Status: operatorv1.ConditionFalse,
			},
		},
		{
			name: "MergeTwoOfThreeWithError",
			fakeClient: func() *fakeOperatorClient {
				return &fakeOperatorClient{
					startingSpec: &operatorv1.OperatorSpec{},
				}
			},
			observers: []ObserveConfigFunc{
				func(listers Listers, existingConfig map[string]interface{}) (observedConfig map[string]interface{}, errs []error) {
					return map[string]interface{}{"foo": "one"}, nil
				},
				func(listers Listers, existingConfig map[string]interface{}) (observedConfig map[string]interface{}, errs []error) {
					return map[string]interface{}{"bar": "two"}, nil
				},
				func(listers Listers, existingConfig map[string]interface{}) (observedConfig map[string]interface{}, errs []error) {
					errs = append(errs, fmt.Errorf("some failure"))
					return observedConfig, errs
				},
			},

			expectError: false,
			expectedObservedConfig: &unstructured.Unstructured{Object: map[string]interface{}{
				"foo": "one",
				"bar": "two",
			}},
			expectedCondition: &operatorv1.OperatorCondition{
				Type:    operatorStatusTypeConfigObservationFailing,
				Status:  operatorv1.ConditionTrue,
				Reason:  configObservationErrorConditionReason,
				Message: "some failure",
			},
		},
		{
			name: "TestUpdateFailed",
			fakeClient: func() *fakeOperatorClient {
				return &fakeOperatorClient{
					startingSpec:      &operatorv1.OperatorSpec{},
					specUpdateFailure: fmt.Errorf("update spec failure"),
				}
			},
			observers: []ObserveConfigFunc{
				func(listers Listers, existingConfig map[string]interface{}) (observedConfig map[string]interface{}, errs []error) {
					return map[string]interface{}{"foo": "one"}, nil
				},
			},

			expectError:            false,
			expectedObservedConfig: nil,
			expectedCondition: &operatorv1.OperatorCondition{
				Type:    operatorStatusTypeConfigObservationFailing,
				Status:  operatorv1.ConditionTrue,
				Reason:  configObservationErrorConditionReason,
				Message: "error writing updated observed config: update spec failure",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			operatorConfigClient := tc.fakeClient()

			configObserver := ConfigObserver{
				listers:        &fakeLister{},
				operatorClient: operatorConfigClient,
				observers:      tc.observers,
			}
			err := configObserver.sync()
			if tc.expectError && err == nil {
				t.Fatal("error expected")
			}
			if err != nil {
				t.Fatal(err)
			}

			switch {
			case tc.expectedObservedConfig != nil && operatorConfigClient.spec == nil:
				t.Error("missing expected spec")
			case tc.expectedObservedConfig != nil:
				if !reflect.DeepEqual(tc.expectedObservedConfig, operatorConfigClient.spec.ObservedConfig.Object) {
					t.Errorf("\n===== observed config expected:\n%v\n===== observed config actual:\n%v", toYAML(tc.expectedObservedConfig), toYAML(operatorConfigClient.spec.ObservedConfig.Object))
				}
			default:
				if operatorConfigClient.spec != nil {
					t.Errorf("unexpected %v", spew.Sdump(operatorConfigClient.spec))
				}
			}

			switch {
			case tc.expectedCondition != nil && operatorConfigClient.status == nil:
				t.Error("missing expected status")
			case tc.expectedCondition != nil:
				condition := v1helpers.FindOperatorCondition(operatorConfigClient.status.Conditions, operatorStatusTypeConfigObservationFailing)
				condition.LastTransitionTime = tc.expectedCondition.LastTransitionTime
				if !reflect.DeepEqual(tc.expectedCondition, condition) {
					t.Fatalf("\n===== condition expected:\n%v\n===== condition actual:\n%v", toYAML(tc.expectedCondition), toYAML(condition))
				}
			default:
				if operatorConfigClient.status != nil {
					t.Errorf("unexpected %v", spew.Sdump(operatorConfigClient.status))
				}
			}

		})
	}
}

func toYAML(o interface{}) string {
	b, e := yaml.Marshal(o)
	if e != nil {
		return e.Error()
	}
	return string(b)
}
