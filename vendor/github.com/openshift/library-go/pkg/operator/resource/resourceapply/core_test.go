package resourceapply

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/google/go-cmp/cmp"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
					t.Error(JSONPatchNoError(expected, actual))
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
			name: "don't mutate CA bundle if injected",
			existing: []runtime.Object{
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Namespace: "one-ns", Name: "foo", Labels: map[string]string{"config.openshift.io/inject-trusted-cabundle": "true"}},
					Data: map[string]string{
						"ca-bundle.crt": "value",
					},
				},
			},
			input: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Namespace: "one-ns", Name: "foo", Labels: map[string]string{"config.openshift.io/inject-trusted-cabundle": "true"}},
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
			name: "keep CA bundle if injected, but prune other entries",
			existing: []runtime.Object{
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Namespace: "one-ns", Name: "foo", Labels: map[string]string{"config.openshift.io/inject-trusted-cabundle": "true"}},
					Data: map[string]string{
						"ca-bundle.crt": "value",
						"other":         "something",
					},
				},
			},
			input: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Namespace: "one-ns", Name: "foo", Labels: map[string]string{"config.openshift.io/inject-trusted-cabundle": "true"}},
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
					ObjectMeta: metav1.ObjectMeta{Namespace: "one-ns", Name: "foo", Labels: map[string]string{"config.openshift.io/inject-trusted-cabundle": "true"}},
					Data: map[string]string{
						"ca-bundle.crt": "value",
					},
				}
				actual := actions[1].(clienttesting.UpdateAction).GetObject().(*corev1.ConfigMap)
				if !equality.Semantic.DeepEqual(expected, actual) {
					t.Error(JSONPatchNoError(expected, actual))
				}
			},
		},
		{
			name: "mutate CA bundle if injected, but ca-bundle.crt specified",
			existing: []runtime.Object{
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Namespace: "one-ns", Name: "foo", Labels: map[string]string{"config.openshift.io/inject-trusted-cabundle": "true"}},
					Data: map[string]string{
						"ca-bundle.crt": "value",
					},
				},
			},
			input: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Namespace: "one-ns", Name: "foo", Labels: map[string]string{"config.openshift.io/inject-trusted-cabundle": "true"}},
				Data: map[string]string{
					"ca-bundle.crt": "different",
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
					ObjectMeta: metav1.ObjectMeta{Namespace: "one-ns", Name: "foo", Labels: map[string]string{"config.openshift.io/inject-trusted-cabundle": "true"}},
					Data: map[string]string{
						"ca-bundle.crt": "different",
					},
				}
				actual := actions[1].(clienttesting.UpdateAction).GetObject().(*corev1.ConfigMap)
				if !equality.Semantic.DeepEqual(expected, actual) {
					t.Error(JSONPatchNoError(expected, actual))
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
					t.Error(JSONPatchNoError(expected, actual))
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
					t.Error(JSONPatchNoError(expected, actual))
				}
			},
		},
		{
			name: "update on mismatch binary data",
			existing: []runtime.Object{
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Namespace: "one-ns", Name: "foo", Labels: map[string]string{"extra": "leave-alone"}},
					Data: map[string]string{
						"configmap": "value",
					},
				},
			},
			input: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Namespace: "one-ns", Name: "foo"},
				Data: map[string]string{
					"configmap": "value",
				},
				BinaryData: map[string][]byte{
					"binconfigmap": []byte("value"),
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
					BinaryData: map[string][]byte{
						"binconfigmap": []byte("value"),
					},
				}
				actual := actions[1].(clienttesting.UpdateAction).GetObject().(*corev1.ConfigMap)
				if !equality.Semantic.DeepEqual(expected, actual) {
					t.Error(JSONPatchNoError(expected, actual))
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

func TestApplySecret(t *testing.T) {
	m := metav1.ObjectMeta{
		Name:        "test",
		Namespace:   "default",
		Annotations: map[string]string{},
	}

	r := schema.GroupVersionResource{Group: "", Resource: "secrets", Version: "v1"}

	tt := []struct {
		name     string
		existing []runtime.Object
		required *corev1.Secret
		expected *corev1.Secret
		actions  []clienttesting.Action
		changed  bool
		err      error
	}{
		{
			name:     "secret gets created if it doesn't exist",
			existing: nil,
			required: &corev1.Secret{
				ObjectMeta: m,
				Type:       corev1.SecretTypeTLS,
			},
			changed: false,
			expected: &corev1.Secret{
				ObjectMeta: m,
				Type:       corev1.SecretTypeTLS,
			},
			actions: []clienttesting.Action{
				clienttesting.GetActionImpl{
					Name: m.Name,
					ActionImpl: clienttesting.ActionImpl{
						Namespace: m.Namespace,
						Verb:      "get",
						Resource:  r,
					},
				},
				clienttesting.CreateActionImpl{
					ActionImpl: clienttesting.ActionImpl{
						Namespace: m.Namespace,
						Verb:      "create",
						Resource:  r,
					},
					Object: &corev1.Secret{
						ObjectMeta: m,
						Type:       corev1.SecretTypeTLS,
					},
				},
			},
		},
		{
			name: "replaces data",
			existing: []runtime.Object{
				&corev1.Secret{
					ObjectMeta: m,
					Type:       corev1.SecretTypeTLS,
					Data: map[string][]byte{
						"foo": []byte("aaa"),
					},
				},
			},
			required: &corev1.Secret{
				ObjectMeta: m,
				Type:       corev1.SecretTypeTLS,
				Data: map[string][]byte{
					"bar": []byte("bbb"),
				},
			},
			changed: false,
			expected: &corev1.Secret{
				ObjectMeta: m,
				Type:       corev1.SecretTypeTLS,
				Data: map[string][]byte{
					"bar": []byte("bbb"),
				},
			},
			actions: []clienttesting.Action{
				clienttesting.GetActionImpl{
					Name: m.Name,
					ActionImpl: clienttesting.ActionImpl{
						Namespace: m.Namespace,
						Verb:      "get",
						Resource:  r,
					},
				},
				clienttesting.UpdateActionImpl{
					ActionImpl: clienttesting.ActionImpl{
						Namespace: m.Namespace,
						Verb:      "update",
						Resource:  r,
					},
					Object: &corev1.Secret{
						ObjectMeta: m,
						Type:       corev1.SecretTypeTLS,
						Data: map[string][]byte{
							"bar": []byte("bbb"),
						},
					},
				},
			},
		},
		{
			name: "doesn't replace existing data for service account tokens",
			existing: []runtime.Object{
				&corev1.Secret{
					ObjectMeta: m,
					Type:       corev1.SecretTypeServiceAccountToken,
					Data: map[string][]byte{
						"tls.key": []byte("aaa"),
					},
				},
			},
			required: &corev1.Secret{
				ObjectMeta: m,
				Type:       corev1.SecretTypeServiceAccountToken,
				Data:       nil,
			},
			changed: false,
			expected: &corev1.Secret{
				ObjectMeta: m,
				Type:       corev1.SecretTypeServiceAccountToken,
				Data: map[string][]byte{
					"tls.key": []byte("aaa"),
				},
			},
			actions: []clienttesting.Action{
				clienttesting.GetActionImpl{
					Name: m.Name,
					ActionImpl: clienttesting.ActionImpl{
						Namespace: m.Namespace,
						Verb:      "get",
						Resource:  r,
					},
				},
			},
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			client := fake.NewSimpleClientset(tc.existing...)
			got, changed, err := ApplySecret(client.CoreV1(), events.NewInMemoryRecorder("test"), tc.required)
			if !reflect.DeepEqual(tc.err, err) {
				t.Errorf("expected error %v, got %v", tc.err, err)
			}

			if !equality.Semantic.DeepEqual(tc.expected, got) {
				t.Errorf("objects don't match %s", cmp.Diff(tc.expected, got))
			}

			if !reflect.DeepEqual(tc.err, err) {
				t.Errorf("expected changed %t, got %t", tc.changed, changed)
			}

			gotActions := client.Actions()
			if !equality.Semantic.DeepEqual(tc.actions, gotActions) {
				t.Errorf("actions don't match: %s", cmp.Diff(tc.actions, gotActions))
			}
		})
	}
}

func TestSyncSecret(t *testing.T) {
	tt := []struct {
		name                        string
		sourceNamespace, sourceName string
		targetNamespace, targetName string
		ownerRefs                   []metav1.OwnerReference
		existingObjects             []runtime.Object
		expectedSecret              *corev1.Secret
		expectedChanged             bool
		expectedErr                 error
	}{
		{
			name:            "syncing existing secret succeeds when the target is missing",
			sourceNamespace: "sourceNamespace",
			sourceName:      "sourceName",
			targetNamespace: "targetNamespace",
			targetName:      "targetName",
			ownerRefs:       nil,
			existingObjects: []runtime.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "sourceNamespace",
						Name:      "sourceName",
					},
					Type: corev1.SecretTypeOpaque,
					Data: map[string][]byte{"foo": []byte("bar")},
				},
			},
			expectedSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "targetNamespace",
					Name:      "targetName",
				},
				Type: corev1.SecretTypeOpaque,
				Data: map[string][]byte{"foo": []byte("bar")},
			},
			expectedChanged: true,
			expectedErr:     nil,
		},
		{
			name:            "syncing existing secret succeeds when the target is present and up to date",
			sourceNamespace: "sourceNamespace",
			sourceName:      "sourceName",
			targetNamespace: "targetNamespace",
			targetName:      "targetName",
			ownerRefs:       nil,
			existingObjects: []runtime.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "sourceNamespace",
						Name:      "sourceName",
					},
					Type: corev1.SecretTypeOpaque,
					Data: map[string][]byte{"foo": []byte("bar")},
				},
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "targetNamespace",
						Name:      "targetName",
					},
					Type: corev1.SecretTypeOpaque,
					Data: map[string][]byte{"foo": []byte("bar")},
				},
			},
			expectedSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "targetNamespace",
					Name:      "targetName",
				},
				Type: corev1.SecretTypeOpaque,
				Data: map[string][]byte{"foo": []byte("bar")},
			},
			expectedChanged: false,
			expectedErr:     nil,
		},
		{
			name:            "syncing existing secret succeeds when the target is present and needs update",
			sourceNamespace: "sourceNamespace",
			sourceName:      "sourceName",
			targetNamespace: "targetNamespace",
			targetName:      "targetName",
			ownerRefs:       nil,
			existingObjects: []runtime.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "sourceNamespace",
						Name:      "sourceName",
					},
					Type: corev1.SecretTypeOpaque,
					Data: map[string][]byte{"foo": []byte("bar2")},
				},
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "targetNamespace",
						Name:      "targetName",
					},
					Type: corev1.SecretTypeOpaque,
					Data: map[string][]byte{"foo": []byte("bar1")},
				},
			},
			expectedSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "targetNamespace",
					Name:      "targetName",
				},
				Type: corev1.SecretTypeOpaque,
				Data: map[string][]byte{"foo": []byte("bar2")},
			},
			expectedChanged: true,
			expectedErr:     nil,
		},
		{
			name:            "syncing missing source secret doesn't fail",
			sourceNamespace: "sourceNamespace",
			sourceName:      "sourceName",
			targetNamespace: "targetNamespace",
			targetName:      "targetName",
			ownerRefs:       nil,
			existingObjects: []runtime.Object{},
			expectedSecret:  nil,
			expectedChanged: true,
			expectedErr:     nil,
		},
		{
			name:            "syncing service account token doesn't sync without the token being present",
			sourceNamespace: "sourceNamespace",
			sourceName:      "sourceName",
			targetNamespace: "targetNamespace",
			targetName:      "targetName",
			ownerRefs:       nil,
			existingObjects: []runtime.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "sourceNamespace",
						Name:      "sourceName",
					},
					Type: corev1.SecretTypeServiceAccountToken,
					Data: map[string][]byte{"foo": []byte("bar")},
				},
			},
			expectedSecret:  nil,
			expectedChanged: false,
			expectedErr:     fmt.Errorf("secret sourceNamespace/sourceName doesn't have a token yet"),
		},
		{
			name:            "syncing service account token strips \"managed\" annotations",
			sourceNamespace: "sourceNamespace",
			sourceName:      "sourceName",
			targetNamespace: "targetNamespace",
			targetName:      "targetName",
			ownerRefs:       nil,
			existingObjects: []runtime.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "sourceNamespace",
						Name:      "sourceName",
						Annotations: map[string]string{
							corev1.ServiceAccountNameKey: "foo",
							corev1.ServiceAccountUIDKey:  "bar",
						},
					},
					Type: corev1.SecretTypeServiceAccountToken,
					Data: map[string][]byte{"token": []byte("top-secret")},
				},
			},
			expectedSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "targetNamespace",
					Name:      "targetName",
				},
				Type: corev1.SecretTypeOpaque,
				Data: map[string][]byte{"token": []byte("top-secret")},
			},
			expectedChanged: true,
			expectedErr:     nil,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			client := fake.NewSimpleClientset(tc.existingObjects...)
			secret, changed, err := SyncSecret(client.CoreV1(), events.NewInMemoryRecorder("test"), tc.sourceNamespace, tc.sourceName, tc.targetNamespace, tc.targetName, tc.ownerRefs)

			if !reflect.DeepEqual(err, tc.expectedErr) {
				t.Errorf("expected error %v, got %v", tc.expectedErr, err)
				return
			}

			if !equality.Semantic.DeepEqual(secret, tc.expectedSecret) {
				t.Errorf("secrets differ: %s", cmp.Diff(tc.expectedSecret, secret))
			}

			if changed != tc.expectedChanged {
				t.Errorf("expected changed %t, got %t", tc.expectedChanged, changed)
			}
		})
	}
}
