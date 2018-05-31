package v1alpha1helpers

import (
	"fmt"
	"testing"

	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/diff"

	"time"

	"github.com/davecgh/go-spew/spew"
	operatorsv1alpha1 "github.com/openshift/api/operator/v1alpha1"
)

func TestSetErrors(t *testing.T) {
	tests := []struct {
		name     string
		starting *operatorsv1alpha1.VersionAvailablity
		errors   []error
		expected *operatorsv1alpha1.VersionAvailablity
	}{
		{
			name:     "simple add",
			starting: &operatorsv1alpha1.VersionAvailablity{},
			errors:   []error{fmt.Errorf("foo"), fmt.Errorf("bar")},
			expected: &operatorsv1alpha1.VersionAvailablity{
				Errors: []string{"foo", "bar"},
			},
		},
		{
			name: "replace",
			starting: &operatorsv1alpha1.VersionAvailablity{
				Errors: []string{"bar"},
			},
			errors: []error{fmt.Errorf("foo")},
			expected: &operatorsv1alpha1.VersionAvailablity{
				Errors: []string{"foo"},
			},
		},
		{
			name: "clear",
			starting: &operatorsv1alpha1.VersionAvailablity{
				Errors: []string{"bar"},
			},
			errors:   []error{},
			expected: &operatorsv1alpha1.VersionAvailablity{},
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
	beforeish := metav1.Time{nowish.Add(-10 * time.Second)}
	afterish := metav1.Time{nowish.Add(10 * time.Second)}

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
