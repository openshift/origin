package resourceapply

import (
	"testing"

	"github.com/davecgh/go-spew/spew"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"

	"github.com/openshift/library-go/pkg/operator/events"
)

func TestApplyConfigMap(t *testing.T) {
	tests := []struct {
		name     string
		existing []runtime.Object
		input    *corev1.ConfigMap

		expectedModified bool
		verifyActions    func(actions []clienttesting.Action, t *testing.T)
	}{
		{
			name: "create",
			input: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Namespace: "one-ns", Name: "foo"},
			},

			expectedModified: true,
			verifyActions: func(actions []clienttesting.Action, t *testing.T) {
				if len(actions) != 2 {
					t.Fatal(spew.Sdump(actions))
				}
				if !actions[0].Matches("get", "configmaps") || actions[0].(clienttesting.GetAction).GetName() != "foo" {
					t.Error(spew.Sdump(actions))
				}
				if !actions[1].Matches("create", "configmaps") {
					t.Error(spew.Sdump(actions))
				}
				expected := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Namespace: "one-ns", Name: "foo"},
				}
				actual := actions[1].(clienttesting.CreateAction).GetObject().(*corev1.ConfigMap)
				if !equality.Semantic.DeepEqual(expected, actual) {
					t.Error(JSONPatch(expected, actual))
				}
			},
		},
		{
			name: "skip on extra label",
			existing: []runtime.Object{
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Namespace: "one-ns", Name: "foo", Labels: map[string]string{"extra": "leave-alone"}},
				},
			},
			input: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Namespace: "one-ns", Name: "foo"},
			},

			expectedModified: false,
			verifyActions: func(actions []clienttesting.Action, t *testing.T) {
				if len(actions) != 1 {
					t.Fatal(spew.Sdump(actions))
				}
				if !actions[0].Matches("get", "configmaps") || actions[0].(clienttesting.GetAction).GetName() != "foo" {
					t.Error(spew.Sdump(actions))
				}
			},
		},
		{
			name: "update on missing label",
			existing: []runtime.Object{
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Namespace: "one-ns", Name: "foo", Labels: map[string]string{"extra": "leave-alone"}},
				},
			},
			input: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Namespace: "one-ns", Name: "foo", Labels: map[string]string{"new": "merge"}},
			},

			expectedModified: true,
			verifyActions: func(actions []clienttesting.Action, t *testing.T) {
				if len(actions) != 2 {
					t.Fatal(spew.Sdump(actions))
				}
				if !actions[0].Matches("get", "configmaps") || actions[0].(clienttesting.GetAction).GetName() != "foo" {
					t.Error(spew.Sdump(actions))
				}
				if !actions[1].Matches("update", "configmaps") {
					t.Error(spew.Sdump(actions))
				}
				expected := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Namespace: "one-ns", Name: "foo", Labels: map[string]string{"extra": "leave-alone", "new": "merge"}},
				}
				actual := actions[1].(clienttesting.UpdateAction).GetObject().(*corev1.ConfigMap)
				if !equality.Semantic.DeepEqual(expected, actual) {
					t.Error(JSONPatch(expected, actual))
				}
			},
		},
		{
			name: "update on mismatch data",
			existing: []runtime.Object{
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Namespace: "one-ns", Name: "foo", Labels: map[string]string{"extra": "leave-alone"}},
				},
			},
			input: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Namespace: "one-ns", Name: "foo"},
				Data: map[string]string{
					"configmap": "value",
				},
			},

			expectedModified: true,
			verifyActions: func(actions []clienttesting.Action, t *testing.T) {
				if len(actions) != 2 {
					t.Fatal(spew.Sdump(actions))
				}
				if !actions[0].Matches("get", "configmaps") || actions[0].(clienttesting.GetAction).GetName() != "foo" {
					t.Error(spew.Sdump(actions))
				}
				if !actions[1].Matches("update", "configmaps") {
					t.Error(spew.Sdump(actions))
				}
				expected := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Namespace: "one-ns", Name: "foo", Labels: map[string]string{"extra": "leave-alone"}},
					Data: map[string]string{
						"configmap": "value",
					},
				}
				actual := actions[1].(clienttesting.UpdateAction).GetObject().(*corev1.ConfigMap)
				if !equality.Semantic.DeepEqual(expected, actual) {
					t.Error(JSONPatch(expected, actual))
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := fake.NewSimpleClientset(test.existing...)
			_, actualModified, err := ApplyConfigMap(client.CoreV1(), events.NewInMemoryRecorder("test"), test.input)
			if err != nil {
				t.Fatal(err)
			}
			if test.expectedModified != actualModified {
				t.Errorf("expected %v, got %v", test.expectedModified, actualModified)
			}
			test.verifyActions(client.Actions(), t)
		})
	}
}
