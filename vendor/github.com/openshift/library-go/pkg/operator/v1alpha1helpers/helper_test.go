package v1alpha1helpers

import (
	"fmt"
	"testing"

	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/client-go/dynamic/fake"

	"time"

	"github.com/davecgh/go-spew/spew"
	operatorsv1alpha1 "github.com/openshift/api/operator/v1alpha1"
)

func TestSetErrors(t *testing.T) {
	tests := []struct {
		name     string
		starting *operatorsv1alpha1.VersionAvailability
		errors   []error
		expected *operatorsv1alpha1.VersionAvailability
	}{
		{
			name:     "simple add",
			starting: &operatorsv1alpha1.VersionAvailability{},
			errors:   []error{fmt.Errorf("foo"), fmt.Errorf("bar")},
			expected: &operatorsv1alpha1.VersionAvailability{
				Errors: []string{"foo", "bar"},
			},
		},
		{
			name: "replace",
			starting: &operatorsv1alpha1.VersionAvailability{
				Errors: []string{"bar"},
			},
			errors: []error{fmt.Errorf("foo")},
			expected: &operatorsv1alpha1.VersionAvailability{
				Errors: []string{"foo"},
			},
		},
		{
			name: "clear",
			starting: &operatorsv1alpha1.VersionAvailability{
				Errors: []string{"bar"},
			},
			errors:   []error{},
			expected: &operatorsv1alpha1.VersionAvailability{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			SetErrors(test.starting, test.errors...)
			if !equality.Semantic.DeepEqual(test.expected, test.starting) {
				t.Errorf(diff.ObjectDiff(test.expected, test.starting))
			}
		})
	}
}

func newCondition(name, status, reason, message string, lastTransition *metav1.Time) operatorsv1alpha1.OperatorCondition {
	ret := operatorsv1alpha1.OperatorCondition{
		Type:    name,
		Status:  operatorsv1alpha1.ConditionStatus(status),
		Reason:  reason,
		Message: message,
	}
	if lastTransition != nil {
		ret.LastTransitionTime = *lastTransition
	}

	return ret
}

func TestSetOperatorCondition(t *testing.T) {
	nowish := metav1.Now()
	beforeish := metav1.Time{Time: nowish.Add(-10 * time.Second)}
	afterish := metav1.Time{Time: nowish.Add(10 * time.Second)}

	tests := []struct {
		name         string
		starting     []operatorsv1alpha1.OperatorCondition
		newCondition operatorsv1alpha1.OperatorCondition
		expected     []operatorsv1alpha1.OperatorCondition
	}{
		{
			name:         "add to empty",
			starting:     []operatorsv1alpha1.OperatorCondition{},
			newCondition: newCondition("one", "True", "my-reason", "my-message", nil),
			expected: []operatorsv1alpha1.OperatorCondition{
				newCondition("one", "True", "my-reason", "my-message", nil),
			},
		},
		{
			name: "add to non-conflicting",
			starting: []operatorsv1alpha1.OperatorCondition{
				newCondition("two", "True", "my-reason", "my-message", nil),
			},
			newCondition: newCondition("one", "True", "my-reason", "my-message", nil),
			expected: []operatorsv1alpha1.OperatorCondition{
				newCondition("two", "True", "my-reason", "my-message", nil),
				newCondition("one", "True", "my-reason", "my-message", nil),
			},
		},
		{
			name: "change existing status",
			starting: []operatorsv1alpha1.OperatorCondition{
				newCondition("two", "True", "my-reason", "my-message", nil),
				newCondition("one", "True", "my-reason", "my-message", nil),
			},
			newCondition: newCondition("one", "False", "my-different-reason", "my-othermessage", nil),
			expected: []operatorsv1alpha1.OperatorCondition{
				newCondition("two", "True", "my-reason", "my-message", nil),
				newCondition("one", "False", "my-different-reason", "my-othermessage", nil),
			},
		},
		{
			name: "leave existing transition time",
			starting: []operatorsv1alpha1.OperatorCondition{
				newCondition("two", "True", "my-reason", "my-message", nil),
				newCondition("one", "True", "my-reason", "my-message", &beforeish),
			},
			newCondition: newCondition("one", "True", "my-reason", "my-message", &afterish),
			expected: []operatorsv1alpha1.OperatorCondition{
				newCondition("two", "True", "my-reason", "my-message", nil),
				newCondition("one", "True", "my-reason", "my-message", &beforeish),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			SetOperatorCondition(&test.starting, test.newCondition)
			if len(test.starting) != len(test.expected) {
				t.Fatal(spew.Sdump(test.starting))
			}

			for i := range test.expected {
				expected := test.expected[i]
				actual := test.starting[i]
				if expected.LastTransitionTime == (metav1.Time{}) {
					actual.LastTransitionTime = metav1.Time{}
				}
				if !equality.Semantic.DeepEqual(expected, actual) {
					t.Errorf(diff.ObjectDiff(expected, actual))
				}
			}
		})
	}
}

func TestRemoveOperatorCondition(t *testing.T) {
	tests := []struct {
		name            string
		starting        []operatorsv1alpha1.OperatorCondition
		removeCondition string
		expected        []operatorsv1alpha1.OperatorCondition
	}{
		{
			name:            "remove missing",
			starting:        []operatorsv1alpha1.OperatorCondition{},
			removeCondition: "one",
			expected:        []operatorsv1alpha1.OperatorCondition{},
		},
		{
			name: "remove existing",
			starting: []operatorsv1alpha1.OperatorCondition{
				newCondition("two", "True", "my-reason", "my-message", nil),
				newCondition("one", "True", "my-reason", "my-message", nil),
			},
			removeCondition: "two",
			expected: []operatorsv1alpha1.OperatorCondition{
				newCondition("one", "True", "my-reason", "my-message", nil),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			RemoveOperatorCondition(&test.starting, test.removeCondition)
			if len(test.starting) != len(test.expected) {
				t.Fatal(spew.Sdump(test.starting))
			}

			for i := range test.expected {
				expected := test.expected[i]
				actual := test.starting[i]
				if expected.LastTransitionTime == (metav1.Time{}) {
					actual.LastTransitionTime = metav1.Time{}
				}
				if !equality.Semantic.DeepEqual(expected, actual) {
					t.Errorf(diff.ObjectDiff(expected, actual))
				}
			}
		})
	}
}

func TestEnsureOperatorConfigExists(t *testing.T) {
	configBytes := []byte(`apiVersion: openshiftapiserver.operator.openshift.io/v1alpha1
kind: OpenShiftAPIServerOperatorConfig
metadata:
  name: instance
spec:
  imagePullSpec: openshift/origin-hypershift:latest`)
	tests := []struct {
		name        string
		getEnv      GetImageEnvFunc
		gvr         schema.GroupVersionResource
		config      []byte
		expected    *unstructured.Unstructured
		existingObj *unstructured.Unstructured
	}{
		{
			name:   "Update operator config from env variable",
			getEnv: func() string { return "foo" },
			gvr:    schema.GroupVersionResource{Group: "openshiftapiserver.operator.openshift.io", Version: "v1alpha1", Resource: "openshiftapiserveroperatorconfigs"},
			config: configBytes,
			existingObj: newUnstructured("openshiftapiserver.operator.openshift.io/v1alpha1",
				"OpenShiftAPIServerOperatorConfig",
				"instance",
				"openshift/origin-hypershift:latest"),
			expected: newUnstructured("openshiftapiserver.operator.openshift.io/v1alpha1",
				"OpenShiftAPIServerOperatorConfig",
				"instance",
				"foo"),
		},
		{
			name:   "Create operator config if none exists",
			getEnv: func() string { return "foo" },
			gvr:    schema.GroupVersionResource{Group: "openshiftapiserver.operator.openshift.io", Version: "v1alpha1", Resource: "openshiftapiserveroperatorconfigs"},
			config: configBytes,
			expected: newUnstructured("openshiftapiserver.operator.openshift.io/v1alpha1",
				"OpenShiftAPIServerOperatorConfig",
				"instance",
				"foo"),
		},
		{
			name:   "Don't change imagePullSpec if no env var set",
			getEnv: func() string { return "" },
			gvr:    schema.GroupVersionResource{Group: "openshiftapiserver.operator.openshift.io", Version: "v1alpha1", Resource: "openshiftapiserveroperatorconfigs"},
			config: configBytes,
			expected: newUnstructured("openshiftapiserver.operator.openshift.io/v1alpha1",
				"OpenShiftAPIServerOperatorConfig",
				"instance",
				"openshift/origin-hypershift:latest"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			objs := []runtime.Object{}
			if test.existingObj != nil {
				objs = append(objs, test.existingObj)
			}

			fakeClient := fake.NewSimpleDynamicClient(runtime.NewScheme(), objs...)
			EnsureOperatorConfigExists(fakeClient, test.config, test.gvr, test.getEnv)
			actual, err := fakeClient.Resource(test.gvr).Get(test.expected.GetName(), metav1.GetOptions{})
			if err != nil {
				t.Errorf("error getting resource: %v", err)
			}
			if !equality.Semantic.DeepEqual(test.expected, actual) {
				t.Errorf(diff.ObjectDiff(test.expected, actual))
			}
		})
	}
}

func newUnstructured(apiVersion, kind, name, imagePullSpec string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": apiVersion,
			"kind":       kind,
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": "",
			},
			"spec": map[string]interface{}{
				"imagePullSpec": imagePullSpec,
			},
		},
	}
}
