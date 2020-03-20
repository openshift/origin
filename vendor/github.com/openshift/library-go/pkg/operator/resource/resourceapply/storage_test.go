package resourceapply

import (
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourcemerge"
	storagev1beta1 "k8s.io/api/storage/v1beta1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
)

func TestApplyCSIDriver(t *testing.T) {
	tests := []struct {
		name     string
		existing []runtime.Object
		input    *storagev1beta1.CSIDriver

		expectedModified bool
		verifyActions    func(actions []clienttesting.Action, t *testing.T)
	}{
		{
			name: "create",
			input: &storagev1beta1.CSIDriver{
				ObjectMeta: metav1.ObjectMeta{Name: "foo", Annotations: map[string]string{"my.csi.driver/foo": "bar"}},
			},

			expectedModified: true,
			verifyActions: func(actions []clienttesting.Action, t *testing.T) {
				if len(actions) != 2 {
					t.Fatal(spew.Sdump(actions))
				}
				if !actions[0].Matches("get", "csidrivers") || actions[0].(clienttesting.GetAction).GetName() != "foo" {
					t.Error(spew.Sdump(actions))
				}
				if !actions[1].Matches("create", "csidrivers") {
					t.Error(spew.Sdump(actions))
				}
				expected := &storagev1beta1.CSIDriver{
					ObjectMeta: metav1.ObjectMeta{Name: "foo", Annotations: map[string]string{"my.csi.driver/foo": "bar"}},
				}
				actual := actions[1].(clienttesting.CreateAction).GetObject().(*storagev1beta1.CSIDriver)
				if !equality.Semantic.DeepEqual(expected, actual) {
					t.Error(JSONPatchNoError(expected, actual))
				}
			},
		},
		{
			name: "update on missing label",
			existing: []runtime.Object{
				&storagev1beta1.CSIDriver{
					ObjectMeta: metav1.ObjectMeta{Name: "foo"},
				},
			},
			input: &storagev1beta1.CSIDriver{
				ObjectMeta: metav1.ObjectMeta{Name: "foo", Labels: map[string]string{"new": "merge"}},
			},
			expectedModified: true,
			verifyActions: func(actions []clienttesting.Action, t *testing.T) {
				if len(actions) != 2 {
					t.Fatal(spew.Sdump(actions))
				}
				if !actions[0].Matches("get", "csidrivers") || actions[0].(clienttesting.GetAction).GetName() != "foo" {
					t.Error(spew.Sdump(actions))
				}
				if !actions[1].Matches("update", "csidrivers") {
					t.Error(spew.Sdump(actions))
				}
				expected := &storagev1beta1.CSIDriver{
					ObjectMeta: metav1.ObjectMeta{Name: "foo", Labels: map[string]string{"new": "merge"}},
				}
				actual := actions[1].(clienttesting.CreateAction).GetObject().(*storagev1beta1.CSIDriver)
				if !equality.Semantic.DeepEqual(expected, actual) {
					t.Error(JSONPatchNoError(expected, actual))
				}
			},
		},
		{
			name: "don't mutate spec",
			existing: []runtime.Object{
				&storagev1beta1.CSIDriver{
					ObjectMeta: metav1.ObjectMeta{Name: "foo"},
					Spec: storagev1beta1.CSIDriverSpec{
						AttachRequired: resourcemerge.BoolPtr(true),
						PodInfoOnMount: resourcemerge.BoolPtr(true),
					},
				},
			},
			input: &storagev1beta1.CSIDriver{
				ObjectMeta: metav1.ObjectMeta{Name: "foo"},
				Spec: storagev1beta1.CSIDriverSpec{
					AttachRequired: resourcemerge.BoolPtr(false),
					PodInfoOnMount: resourcemerge.BoolPtr(false),
				},
			},
			expectedModified: false,
			verifyActions: func(actions []clienttesting.Action, t *testing.T) {
				if len(actions) != 1 {
					t.Fatal(spew.Sdump(actions))
				}
				if !actions[0].Matches("get", "csidrivers") || actions[0].(clienttesting.GetAction).GetName() != "foo" {
					t.Error(spew.Sdump(actions))
				}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := fake.NewSimpleClientset(test.existing...)
			_, actualModified, err := ApplyCSIDriverV1Beta1(client.StorageV1beta1(), events.NewInMemoryRecorder("test"), test.input)
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
