package policy

import (
	"bytes"
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	clientgotesting "k8s.io/client-go/testing"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	securityapi "github.com/openshift/origin/pkg/security/apis/security"
	securityfakeclient "github.com/openshift/origin/pkg/security/generated/internalclientset/fake"
)

func TestModifySCC(t *testing.T) {
	tests := map[string]struct {
		startingSCC *securityapi.SecurityContextConstraints
		subjects    []kapi.ObjectReference
		expectedSCC *securityapi.SecurityContextConstraints
		remove      bool
	}{
		"add-user-to-empty": {
			startingSCC: &securityapi.SecurityContextConstraints{},
			subjects:    []kapi.ObjectReference{{Name: "one", Kind: authorizationapi.UserKind}, {Name: "two", Kind: authorizationapi.UserKind}},
			expectedSCC: &securityapi.SecurityContextConstraints{Users: []string{"one", "two"}},
			remove:      false,
		},
		"add-user-to-existing": {
			startingSCC: &securityapi.SecurityContextConstraints{Users: []string{"one"}},
			subjects:    []kapi.ObjectReference{{Name: "two", Kind: authorizationapi.UserKind}},
			expectedSCC: &securityapi.SecurityContextConstraints{Users: []string{"one", "two"}},
			remove:      false,
		},
		"add-user-to-existing-with-overlap": {
			startingSCC: &securityapi.SecurityContextConstraints{Users: []string{"one"}},
			subjects:    []kapi.ObjectReference{{Name: "one", Kind: authorizationapi.UserKind}, {Name: "two", Kind: authorizationapi.UserKind}},
			expectedSCC: &securityapi.SecurityContextConstraints{Users: []string{"one", "two"}},
			remove:      false,
		},

		"add-sa-to-empty": {
			startingSCC: &securityapi.SecurityContextConstraints{},
			subjects:    []kapi.ObjectReference{{Namespace: "a", Name: "one", Kind: authorizationapi.ServiceAccountKind}, {Namespace: "b", Name: "two", Kind: authorizationapi.ServiceAccountKind}},
			expectedSCC: &securityapi.SecurityContextConstraints{Users: []string{"system:serviceaccount:a:one", "system:serviceaccount:b:two"}},
			remove:      false,
		},
		"add-sa-to-existing": {
			startingSCC: &securityapi.SecurityContextConstraints{Users: []string{"one"}},
			subjects:    []kapi.ObjectReference{{Namespace: "b", Name: "two", Kind: authorizationapi.ServiceAccountKind}},
			expectedSCC: &securityapi.SecurityContextConstraints{Users: []string{"one", "system:serviceaccount:b:two"}},
			remove:      false,
		},
		"add-sa-to-existing-with-overlap": {
			startingSCC: &securityapi.SecurityContextConstraints{Users: []string{"system:serviceaccount:a:one"}},
			subjects:    []kapi.ObjectReference{{Namespace: "a", Name: "one", Kind: authorizationapi.ServiceAccountKind}, {Namespace: "b", Name: "two", Kind: authorizationapi.ServiceAccountKind}},
			expectedSCC: &securityapi.SecurityContextConstraints{Users: []string{"system:serviceaccount:a:one", "system:serviceaccount:b:two"}},
			remove:      false,
		},

		"add-group-to-empty": {
			startingSCC: &securityapi.SecurityContextConstraints{},
			subjects:    []kapi.ObjectReference{{Name: "one", Kind: authorizationapi.GroupKind}, {Name: "two", Kind: authorizationapi.GroupKind}},
			expectedSCC: &securityapi.SecurityContextConstraints{Groups: []string{"one", "two"}},
			remove:      false,
		},
		"add-group-to-existing": {
			startingSCC: &securityapi.SecurityContextConstraints{Groups: []string{"one"}},
			subjects:    []kapi.ObjectReference{{Name: "two", Kind: authorizationapi.GroupKind}},
			expectedSCC: &securityapi.SecurityContextConstraints{Groups: []string{"one", "two"}},
			remove:      false,
		},
		"add-group-to-existing-with-overlap": {
			startingSCC: &securityapi.SecurityContextConstraints{Groups: []string{"one"}},
			subjects:    []kapi.ObjectReference{{Name: "one", Kind: authorizationapi.GroupKind}, {Name: "two", Kind: authorizationapi.GroupKind}},
			expectedSCC: &securityapi.SecurityContextConstraints{Groups: []string{"one", "two"}},
			remove:      false,
		},

		"remove-user": {
			startingSCC: &securityapi.SecurityContextConstraints{Users: []string{"one", "two"}},
			subjects:    []kapi.ObjectReference{{Name: "one", Kind: authorizationapi.UserKind}, {Name: "two", Kind: authorizationapi.UserKind}},
			expectedSCC: &securityapi.SecurityContextConstraints{},
			remove:      true,
		},
		"remove-user-from-existing-with-overlap": {
			startingSCC: &securityapi.SecurityContextConstraints{Users: []string{"one", "two"}},
			subjects:    []kapi.ObjectReference{{Name: "two", Kind: authorizationapi.UserKind}},
			expectedSCC: &securityapi.SecurityContextConstraints{Users: []string{"one"}},
			remove:      true,
		},

		"remove-sa": {
			startingSCC: &securityapi.SecurityContextConstraints{Users: []string{"system:serviceaccount:a:one", "system:serviceaccount:b:two"}},
			subjects:    []kapi.ObjectReference{{Namespace: "a", Name: "one", Kind: authorizationapi.ServiceAccountKind}, {Namespace: "b", Name: "two", Kind: authorizationapi.ServiceAccountKind}},
			expectedSCC: &securityapi.SecurityContextConstraints{},
			remove:      true,
		},
		"remove-sa-from-existing-with-overlap": {
			startingSCC: &securityapi.SecurityContextConstraints{Users: []string{"system:serviceaccount:a:one", "system:serviceaccount:b:two"}},
			subjects:    []kapi.ObjectReference{{Namespace: "b", Name: "two", Kind: authorizationapi.ServiceAccountKind}},
			expectedSCC: &securityapi.SecurityContextConstraints{Users: []string{"system:serviceaccount:a:one"}},
			remove:      true,
		},

		"remove-group": {
			startingSCC: &securityapi.SecurityContextConstraints{Groups: []string{"one", "two"}},
			subjects:    []kapi.ObjectReference{{Name: "one", Kind: authorizationapi.GroupKind}, {Name: "two", Kind: authorizationapi.GroupKind}},
			expectedSCC: &securityapi.SecurityContextConstraints{},
			remove:      true,
		},
		"remove-group-from-existing-with-overlap": {
			startingSCC: &securityapi.SecurityContextConstraints{Groups: []string{"one", "two"}},
			subjects:    []kapi.ObjectReference{{Name: "two", Kind: authorizationapi.GroupKind}},
			expectedSCC: &securityapi.SecurityContextConstraints{Groups: []string{"one"}},
			remove:      true,
		},
	}

	for tcName, tc := range tests {
		fakeClient := securityfakeclient.NewSimpleClientset()
		fakeClient.Fake.PrependReactor("get", "securitycontextconstraints", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
			return true, tc.startingSCC, nil
		})
		var actualSCC *securityapi.SecurityContextConstraints
		fakeClient.Fake.PrependReactor("update", "securitycontextconstraints", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
			actualSCC = action.(clientgotesting.UpdateAction).GetObject().(*securityapi.SecurityContextConstraints)
			return true, actualSCC, nil
		})

		o := &SCCModificationOptions{
			SCCName:                 "foo",
			SCCInterface:            fakeClient.Security().SecurityContextConstraints(),
			DefaultSubjectNamespace: "",
			Subjects:                tc.subjects,

			Out: &bytes.Buffer{},
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
