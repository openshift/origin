package configobserver

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/ghodss/yaml"
	"github.com/imdario/mergo"
	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/condition"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resourcesynccontroller"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
)

func (c *fakeOperatorClient) Informer() cache.SharedIndexInformer {
	return nil
}

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
func (c *fakeOperatorClient) UpdateOperatorStatus(rv string, in *operatorv1.OperatorStatus) (status *operatorv1.OperatorStatus, err error) {
	c.status = in
	return in, nil
}

type fakeOperatorClient struct {
	startingSpec      *operatorv1.OperatorSpec
	specUpdateFailure error

	status *operatorv1.OperatorStatus
	spec   *operatorv1.OperatorSpec
}

type fakeLister struct{}

func (l *fakeLister) ResourceSyncer() resourcesynccontroller.ResourceSyncer {
	return nil
}

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
		expectEvents           [][]string
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
			expectEvents: [][]string{
				{"ObservedConfigChanged", "Writing updated observed config"},
			},
			observers: []ObserveConfigFunc{
				func(listers Listers, recorder events.Recorder, existingConfig map[string]interface{}) (observedConfig map[string]interface{}, errs []error) {
					return map[string]interface{}{"foo": "one"}, nil
				},
				func(listers Listers, recorder events.Recorder, existingConfig map[string]interface{}) (observedConfig map[string]interface{}, errs []error) {
					return map[string]interface{}{"bar": "two"}, nil
				},
				func(listers Listers, recorder events.Recorder, existingConfig map[string]interface{}) (observedConfig map[string]interface{}, errs []error) {
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
				Type:   condition.ConfigObservationDegradedConditionType,
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
			expectEvents: [][]string{
				{"ObservedConfigChanged", "Writing updated observed config"},
			},
			observers: []ObserveConfigFunc{
				func(listers Listers, recorder events.Recorder, existingConfig map[string]interface{}) (observedConfig map[string]interface{}, errs []error) {
					return map[string]interface{}{"foo": "one"}, nil
				},
				func(listers Listers, recorder events.Recorder, existingConfig map[string]interface{}) (observedConfig map[string]interface{}, errs []error) {
					return map[string]interface{}{"bar": "two"}, nil
				},
				func(listers Listers, recorder events.Recorder, existingConfig map[string]interface{}) (observedConfig map[string]interface{}, errs []error) {
					errs = append(errs, fmt.Errorf("some failure"))
					return observedConfig, errs
				},
			},

			expectError: true,
			expectedObservedConfig: &unstructured.Unstructured{Object: map[string]interface{}{
				"foo": "one",
				"bar": "two",
			}},
			expectedCondition: &operatorv1.OperatorCondition{
				Type:    condition.ConfigObservationDegradedConditionType,
				Status:  operatorv1.ConditionTrue,
				Reason:  "Error",
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
			expectEvents: [][]string{
				{"ObservedConfigChanged", "Writing updated observed config"},
				{"ObservedConfigWriteError", "Failed to write observed config: update spec failure"},
			},
			observers: []ObserveConfigFunc{
				func(listers Listers, recorder events.Recorder, existingConfig map[string]interface{}) (observedConfig map[string]interface{}, errs []error) {
					return map[string]interface{}{"foo": "one"}, nil
				},
			},

			expectError:            true,
			expectedObservedConfig: nil,
			expectedCondition: &operatorv1.OperatorCondition{
				Type:    condition.ConfigObservationDegradedConditionType,
				Status:  operatorv1.ConditionTrue,
				Reason:  "Error",
				Message: "error writing updated observed config: update spec failure",
			},
		},
		{
			name: "NonDeterministic",
			fakeClient: func() *fakeOperatorClient {
				return &fakeOperatorClient{
					startingSpec: &operatorv1.OperatorSpec{},
				}
			},
			expectEvents: [][]string{
				{"ObservedConfigChanged", "Writing updated observed config"},
			},
			observers: []ObserveConfigFunc{
				func(listers Listers, recorder events.Recorder, existingConfig map[string]interface{}) (observedConfig map[string]interface{}, errs []error) {
					return map[string]interface{}{"level1": map[string]interface{}{"level2_c": []interface{}{"slice_entry_a"}}}, nil
				},
				func(listers Listers, recorder events.Recorder, existingConfig map[string]interface{}) (observedConfig map[string]interface{}, errs []error) {
					return map[string]interface{}{"level1": map[string]interface{}{"level2_c": []interface{}{"slice_entry_b"}}}, nil
				},
			},

			expectError: true,
			expectedCondition: &operatorv1.OperatorCondition{
				Type:    condition.ConfigObservationDegradedConditionType,
				Status:  operatorv1.ConditionTrue,
				Reason:  "Error",
				Message: "non-deterministic config observation detected",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			operatorConfigClient := tc.fakeClient()
			eventClient := fake.NewSimpleClientset()

			configObserver := ConfigObserver{
				listers:        &fakeLister{},
				operatorClient: operatorConfigClient,
				observers:      tc.observers,
			}
			err := configObserver.sync(context.TODO(), factory.NewSyncContext("test", events.NewRecorder(eventClient.CoreV1().Events("test"), "test-operator", &corev1.ObjectReference{})))
			if tc.expectError && err == nil {
				t.Fatal("error expected")
			}
			if !tc.expectError && err != nil {
				t.Fatal(err)
			}

			observedEvents := [][]string{}
			for _, action := range eventClient.Actions() {
				if !action.Matches("create", "events") {
					continue
				}
				event := action.(ktesting.CreateAction).GetObject().(*corev1.Event)
				observedEvents = append(observedEvents, []string{event.Reason, event.Message})
			}
			for i, event := range tc.expectEvents {
				if observedEvents[i][0] != event[0] {
					t.Errorf("expected %d event reason to be %q, got %q", i, event[0], observedEvents[i][0])
				}
				if !strings.HasPrefix(observedEvents[i][1], event[1]) {
					t.Errorf("expected %d event message to be %q, got %q", i, event[1], observedEvents[i][1])
				}
			}
			if len(tc.expectEvents) != len(observedEvents) {
				t.Errorf("expected %d events, got %d (%#v)", len(tc.expectEvents), len(observedEvents), observedEvents)
			}

			switch {
			case tc.expectedObservedConfig != nil && operatorConfigClient.spec == nil:
				t.Error("missing expected spec")
			case tc.expectedObservedConfig != nil:
				if !reflect.DeepEqual(tc.expectedObservedConfig, operatorConfigClient.spec.ObservedConfig.Object) {
					t.Errorf("\n===== observed config expected:\n%v\n===== observed config actual:\n%v", toYAML(tc.expectedObservedConfig), toYAML(operatorConfigClient.spec.ObservedConfig.Object))
				}
			}

			switch {
			case tc.expectedCondition != nil && operatorConfigClient.status == nil:
				t.Error("missing expected status")
			case tc.expectedCondition != nil:
				condition := v1helpers.FindOperatorCondition(operatorConfigClient.status.Conditions, condition.ConfigObservationDegradedConditionType)
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

func TestMergoVersion(t *testing.T) {
	type test struct{ A string }
	src := test{"src"}
	dest := test{"dest"}
	mergo.Merge(&dest, &src, mergo.WithOverride)
	if dest.A != "src" {
		t.Errorf("incompatible version of github.com/imdario/mergo found")
	}
}

func toYAML(o interface{}) string {
	b, e := yaml.Marshal(o)
	if e != nil {
		return e.Error()
	}
	return string(b)
}

func TestWithPrefix(t *testing.T) {
	const targetField = "changeThisIfYouMust"
	const targetValue = "targetValue"

	var modified bool
	var testErr = fmt.Errorf("error")
	var testedPrefix = []string{"some", "prefix"}

	existingConfig := map[string]interface{}{
		targetField: targetValue,
	}

	getObserverFunc := func(shouldError, returnNil bool) ObserveConfigFunc {
		return func(_ Listers, _ events.Recorder, existingConfig map[string]interface{}) (map[string]interface{}, []error) {
			var errs = []error{}
			if shouldError {
				errs = append(errs, testErr)
			}

			if returnNil {
				return nil, errs
			}

			ret := map[string]interface{}{}
			if v, _, _ := unstructured.NestedString(existingConfig, targetField); v != targetValue {
				modified = true
			}
			unstructured.SetNestedField(ret, targetValue, targetField)

			return ret, errs
		}
	}

	tests := []struct {
		name           string
		observer       ObserveConfigFunc
		testedPrefix   []string
		existingConfig map[string]interface{}
		wantConfig     map[string]interface{}
		wantErrors     []error
		shouldModify   bool
	}{
		{
			name:     "nil prefix, nil return",
			observer: getObserverFunc(false, true),
		},
		{
			name:           "some prefix, nil return",
			observer:       getObserverFunc(false, true),
			testedPrefix:   testedPrefix,
			existingConfig: addPrefixToInterface(existingConfig, testedPrefix...),
		},
		{
			name:           "existing == expected",
			observer:       getObserverFunc(false, false),
			testedPrefix:   testedPrefix,
			existingConfig: addPrefixToInterface(existingConfig, testedPrefix...),
			wantConfig:     addPrefixToInterface(existingConfig, testedPrefix...),
		},
		{
			name:         "update existing",
			observer:     getObserverFunc(false, false),
			testedPrefix: testedPrefix,
			existingConfig: addPrefixToInterface(map[string]interface{}{
				targetField: "100%randomvalue",
			}, testedPrefix...),
			wantConfig:   addPrefixToInterface(existingConfig, testedPrefix...),
			shouldModify: true,
		},
		{
			name:           "observer error gets propagated",
			observer:       getObserverFunc(true, false),
			testedPrefix:   testedPrefix,
			existingConfig: addPrefixToInterface(existingConfig, testedPrefix...),
			wantConfig:     addPrefixToInterface(existingConfig, testedPrefix...),
			wantErrors:     []error{testErr},
		},
		{
			name:           "prefix is empty in existing map, get modified",
			observer:       getObserverFunc(false, false),
			testedPrefix:   testedPrefix,
			existingConfig: existingConfig,
			wantConfig:     addPrefixToInterface(existingConfig, testedPrefix...),
			shouldModify:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// reset modified flag
			defer func() { modified = false }()

			gotConfig, errs := WithPrefix(tt.observer, tt.testedPrefix...)(nil, events.NewInMemoryRecorder("test"), tt.existingConfig)

			if !reflect.DeepEqual(gotConfig, tt.wantConfig) {
				t.Errorf("observed with prefix; got = %v, want %v", gotConfig, tt.wantConfig)
			}

			if len(errs) != len(tt.wantErrors) {
				t.Errorf("observed with prefix; got errors = %v, want %v", errs, tt.wantErrors)
			} else {
				for i := range errs {
					if errs[i].Error() != tt.wantErrors[i].Error() {
						t.Errorf("observed with prefix; got errors = %v, want %v", errs, tt.wantErrors)
						break
					}
				}
			}

			if modified != tt.shouldModify {
				t.Errorf("existing config was modified but it should not have been")
			}

		})
	}
}

func addPrefixToInterface(i interface{}, prefix ...string) map[string]interface{} {
	ret := map[string]interface{}{}
	unstructured.SetNestedField(ret, i, prefix...)
	return ret
}
