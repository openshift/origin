package policy

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
	clientgotesting "k8s.io/client-go/testing"

	securityv1 "github.com/openshift/api/security/v1"
	fakesecurityclient "github.com/openshift/client-go/security/clientset/versioned/fake"
	fakesecurityv1client "github.com/openshift/client-go/security/clientset/versioned/typed/security/v1/fake"
)

func TestModifySCC(t *testing.T) {
	tests := map[string]struct {
		startingSCC *securityv1.SecurityContextConstraints
		subjects    []corev1.ObjectReference
		expectedSCC *securityv1.SecurityContextConstraints
		remove      bool
	}{
		"add-user-to-empty": {
			startingSCC: &securityv1.SecurityContextConstraints{},
			subjects:    []corev1.ObjectReference{{Name: "one", Kind: "User"}, {Name: "two", Kind: "User"}},
			expectedSCC: &securityv1.SecurityContextConstraints{Users: []string{"one", "two"}},
			remove:      false,
		},
		"add-user-to-existing": {
			startingSCC: &securityv1.SecurityContextConstraints{Users: []string{"one"}},
			subjects:    []corev1.ObjectReference{{Name: "two", Kind: "User"}},
			expectedSCC: &securityv1.SecurityContextConstraints{Users: []string{"one", "two"}},
			remove:      false,
		},
		"add-user-to-existing-with-overlap": {
			startingSCC: &securityv1.SecurityContextConstraints{Users: []string{"one"}},
			subjects:    []corev1.ObjectReference{{Name: "one", Kind: "User"}, {Name: "two", Kind: "User"}},
			expectedSCC: &securityv1.SecurityContextConstraints{Users: []string{"one", "two"}},
			remove:      false,
		},

		"add-sa-to-empty": {
			startingSCC: &securityv1.SecurityContextConstraints{},
			subjects:    []corev1.ObjectReference{{Namespace: "a", Name: "one", Kind: "ServiceAccount"}, {Namespace: "b", Name: "two", Kind: "ServiceAccount"}},
			expectedSCC: &securityv1.SecurityContextConstraints{Users: []string{"system:serviceaccount:a:one", "system:serviceaccount:b:two"}},
			remove:      false,
		},
		"add-sa-to-existing": {
			startingSCC: &securityv1.SecurityContextConstraints{Users: []string{"one"}},
			subjects:    []corev1.ObjectReference{{Namespace: "b", Name: "two", Kind: "ServiceAccount"}},
			expectedSCC: &securityv1.SecurityContextConstraints{Users: []string{"one", "system:serviceaccount:b:two"}},
			remove:      false,
		},
		"add-sa-to-existing-with-overlap": {
			startingSCC: &securityv1.SecurityContextConstraints{Users: []string{"system:serviceaccount:a:one"}},
			subjects:    []corev1.ObjectReference{{Namespace: "a", Name: "one", Kind: "ServiceAccount"}, {Namespace: "b", Name: "two", Kind: "ServiceAccount"}},
			expectedSCC: &securityv1.SecurityContextConstraints{Users: []string{"system:serviceaccount:a:one", "system:serviceaccount:b:two"}},
			remove:      false,
		},

		"add-group-to-empty": {
			startingSCC: &securityv1.SecurityContextConstraints{},
			subjects:    []corev1.ObjectReference{{Name: "one", Kind: "Group"}, {Name: "two", Kind: "Group"}},
			expectedSCC: &securityv1.SecurityContextConstraints{Groups: []string{"one", "two"}},
			remove:      false,
		},
		"add-group-to-existing": {
			startingSCC: &securityv1.SecurityContextConstraints{Groups: []string{"one"}},
			subjects:    []corev1.ObjectReference{{Name: "two", Kind: "Group"}},
			expectedSCC: &securityv1.SecurityContextConstraints{Groups: []string{"one", "two"}},
			remove:      false,
		},
		"add-group-to-existing-with-overlap": {
			startingSCC: &securityv1.SecurityContextConstraints{Groups: []string{"one"}},
			subjects:    []corev1.ObjectReference{{Name: "one", Kind: "Group"}, {Name: "two", Kind: "Group"}},
			expectedSCC: &securityv1.SecurityContextConstraints{Groups: []string{"one", "two"}},
			remove:      false,
		},

		"remove-user": {
			startingSCC: &securityv1.SecurityContextConstraints{Users: []string{"one", "two"}},
			subjects:    []corev1.ObjectReference{{Name: "one", Kind: "User"}, {Name: "two", Kind: "User"}},
			expectedSCC: &securityv1.SecurityContextConstraints{},
			remove:      true,
		},
		"remove-user-from-existing-with-overlap": {
			startingSCC: &securityv1.SecurityContextConstraints{Users: []string{"one", "two"}},
			subjects:    []corev1.ObjectReference{{Name: "two", Kind: "User"}},
			expectedSCC: &securityv1.SecurityContextConstraints{Users: []string{"one"}},
			remove:      true,
		},

		"remove-sa": {
			startingSCC: &securityv1.SecurityContextConstraints{Users: []string{"system:serviceaccount:a:one", "system:serviceaccount:b:two"}},
			subjects:    []corev1.ObjectReference{{Namespace: "a", Name: "one", Kind: "ServiceAccount"}, {Namespace: "b", Name: "two", Kind: "ServiceAccount"}},
			expectedSCC: &securityv1.SecurityContextConstraints{},
			remove:      true,
		},
		"remove-sa-from-existing-with-overlap": {
			startingSCC: &securityv1.SecurityContextConstraints{Users: []string{"system:serviceaccount:a:one", "system:serviceaccount:b:two"}},
			subjects:    []corev1.ObjectReference{{Namespace: "b", Name: "two", Kind: "ServiceAccount"}},
			expectedSCC: &securityv1.SecurityContextConstraints{Users: []string{"system:serviceaccount:a:one"}},
			remove:      true,
		},

		"remove-group": {
			startingSCC: &securityv1.SecurityContextConstraints{Groups: []string{"one", "two"}},
			subjects:    []corev1.ObjectReference{{Name: "one", Kind: "Group"}, {Name: "two", Kind: "Group"}},
			expectedSCC: &securityv1.SecurityContextConstraints{},
			remove:      true,
		},
		"remove-group-from-existing-with-overlap": {
			startingSCC: &securityv1.SecurityContextConstraints{Groups: []string{"one", "two"}},
			subjects:    []corev1.ObjectReference{{Name: "two", Kind: "Group"}},
			expectedSCC: &securityv1.SecurityContextConstraints{Groups: []string{"one"}},
			remove:      true,
		},
	}

	for tcName, tc := range tests {
		fakeClient := fakesecurityv1client.FakeSecurityV1{Fake: &(fakesecurityclient.NewSimpleClientset().Fake)}
		fakeClient.Fake.PrependReactor("get", "securitycontextconstraints", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
			return true, tc.startingSCC, nil
		})
		var actualSCC *securityv1.SecurityContextConstraints
		fakeClient.Fake.PrependReactor("update", "securitycontextconstraints", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
			actualSCC = action.(clientgotesting.UpdateAction).GetObject().(*securityv1.SecurityContextConstraints)
			return true, actualSCC, nil
		})

		o := &SCCModificationOptions{
			PrintFlags: genericclioptions.NewPrintFlags(""),
			ToPrinter:  func(string) (printers.ResourcePrinter, error) { return printers.NewDiscardingPrinter(), nil },

			SCCName:                 "foo",
			SCCInterface:            fakeClient.SecurityContextConstraints(),
			DefaultSubjectNamespace: "",
			Subjects:                tc.subjects,

			IOStreams: genericclioptions.NewTestIOStreamsDiscard(),
		}

		var err error
		if tc.remove {
			err = o.RemoveSCC()
		} else {
			err = o.AddSCC()
		}
		if err != nil {
			t.Errorf("%s: unexpected err %v", tcName, err)
		}
		if e, a := tc.expectedSCC.Users, actualSCC.Users; !reflect.DeepEqual(e, a) {
			t.Errorf("%s: expected %v, actual %v", tcName, e, a)
		}
		if e, a := tc.expectedSCC.Groups, actualSCC.Groups; !reflect.DeepEqual(e, a) {
			t.Errorf("%s: expected %v, actual %v", tcName, e, a)
		}
	}
}
