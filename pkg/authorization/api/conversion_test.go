package api_test // prevent import cycle between authorizationapi and rulevalidation

import (
	"fmt"
	"reflect"
	"testing"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/api/validation"
	"k8s.io/kubernetes/pkg/apis/rbac"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/diff"
	"k8s.io/kubernetes/pkg/util/sets"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/rulevalidation"
	uservalidation "github.com/openshift/origin/pkg/user/api/validation"

	"github.com/google/gofuzz"
)

// make sure rbac <-> origin round trip does not lose any data

func TestOriginClusterRoleFidelity(t *testing.T) {
	for i := 0; i < 100; i++ {
		ocr := &authorizationapi.ClusterRole{}
		ocr2 := &authorizationapi.ClusterRole{}
		rcr := &rbac.ClusterRole{}
		fuzzer.Fuzz(ocr)
		if err := authorizationapi.Convert_api_ClusterRole_To_rbac_ClusterRole(ocr, rcr, nil); err != nil {
			t.Fatal(err)
		}
		if err := authorizationapi.Convert_rbac_ClusterRole_To_api_ClusterRole(rcr, ocr2, nil); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(ocr, ocr2) {
			t.Errorf("origin cluster data not preserved; the diff is %s", diff.ObjectDiff(ocr, ocr2))
		}
	}
}

func TestOriginRoleFidelity(t *testing.T) {
	for i := 0; i < 100; i++ {
		or := &authorizationapi.Role{}
		or2 := &authorizationapi.Role{}
		rr := &rbac.Role{}
		fuzzer.Fuzz(or)
		if err := authorizationapi.Convert_api_Role_To_rbac_Role(or, rr, nil); err != nil {
			t.Fatal(err)
		}
		if err := authorizationapi.Convert_rbac_Role_To_api_Role(rr, or2, nil); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(or, or2) {
			t.Errorf("origin local data not preserved; the diff is %s", diff.ObjectDiff(or, or2))
		}
	}
}

func TestOriginClusterRoleBindingFidelity(t *testing.T) {
	for i := 0; i < 100; i++ {
		ocrb := &authorizationapi.ClusterRoleBinding{}
		ocrb2 := &authorizationapi.ClusterRoleBinding{}
		rcrb := &rbac.ClusterRoleBinding{}
		fuzzer.Fuzz(ocrb)
		if err := authorizationapi.Convert_api_ClusterRoleBinding_To_rbac_ClusterRoleBinding(ocrb, rcrb, nil); err != nil {
			t.Fatal(err)
		}
		if err := authorizationapi.Convert_rbac_ClusterRoleBinding_To_api_ClusterRoleBinding(rcrb, ocrb2, nil); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(ocrb, ocrb2) {
			t.Errorf("origin cluster binding data not preserved; the diff is %s", diff.ObjectDiff(ocrb, ocrb2))
		}
	}
}

func TestOriginRoleBindingFidelity(t *testing.T) {
	for i := 0; i < 100; i++ {
		orb := &authorizationapi.RoleBinding{}
		orb2 := &authorizationapi.RoleBinding{}
		rrb := &rbac.RoleBinding{}
		fuzzer.Fuzz(orb)
		if err := authorizationapi.Convert_api_RoleBinding_To_rbac_RoleBinding(orb, rrb, nil); err != nil {
			t.Fatal(err)
		}
		if err := authorizationapi.Convert_rbac_RoleBinding_To_api_RoleBinding(rrb, orb2, nil); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(orb, orb2) {
			t.Errorf("origin local binding data not preserved; the diff is %s", diff.ObjectDiff(orb, orb2))
		}
	}
}

func TestRBACClusterRoleFidelity(t *testing.T) {
	for i := 0; i < 100; i++ {
		rcr := &rbac.ClusterRole{}
		rcr2 := &rbac.ClusterRole{}
		ocr := &authorizationapi.ClusterRole{}
		fuzzer.Fuzz(rcr)
		if err := authorizationapi.Convert_rbac_ClusterRole_To_api_ClusterRole(rcr, ocr, nil); err != nil {
			t.Fatal(err)
		}
		if err := authorizationapi.Convert_api_ClusterRole_To_rbac_ClusterRole(ocr, rcr2, nil); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(rcr, rcr2) {
			t.Errorf("rbac cluster data not preserved; the diff is %s", diff.ObjectDiff(rcr, rcr2))
		}
	}
}

func TestRBACRoleFidelity(t *testing.T) {
	for i := 0; i < 100; i++ {
		rr := &rbac.Role{}
		rr2 := &rbac.Role{}
		or := &authorizationapi.Role{}
		fuzzer.Fuzz(rr)
		if err := authorizationapi.Convert_rbac_Role_To_api_Role(rr, or, nil); err != nil {
			t.Fatal(err)
		}
		if err := authorizationapi.Convert_api_Role_To_rbac_Role(or, rr2, nil); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(rr, rr2) {
			t.Errorf("rbac local data not preserved; the diff is %s", diff.ObjectDiff(rr, rr2))
		}
	}
}

func TestRBACClusterRoleBindingFidelity(t *testing.T) {
	for i := 0; i < 100; i++ {
		rcrb := &rbac.ClusterRoleBinding{}
		rcrb2 := &rbac.ClusterRoleBinding{}
		ocrb := &authorizationapi.ClusterRoleBinding{}
		fuzzer.Fuzz(rcrb)
		if err := authorizationapi.Convert_rbac_ClusterRoleBinding_To_api_ClusterRoleBinding(rcrb, ocrb, nil); err != nil {
			t.Fatal(err)
		}
		if err := authorizationapi.Convert_api_ClusterRoleBinding_To_rbac_ClusterRoleBinding(ocrb, rcrb2, nil); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(rcrb, rcrb2) {
			t.Errorf("rbac cluster binding data not preserved; the diff is %s", diff.ObjectDiff(rcrb, rcrb2))
		}
	}
}

func TestRBACRoleBindingFidelity(t *testing.T) {
	for i := 0; i < 100; i++ {
		rrb := &rbac.RoleBinding{}
		rrb2 := &rbac.RoleBinding{}
		orb := &authorizationapi.RoleBinding{}
		fuzzer.Fuzz(rrb)
		if err := authorizationapi.Convert_rbac_RoleBinding_To_api_RoleBinding(rrb, orb, nil); err != nil {
			t.Fatal(err)
		}
		if err := authorizationapi.Convert_api_RoleBinding_To_rbac_RoleBinding(orb, rrb2, nil); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(rrb, rrb2) {
			t.Errorf("rbac local binding data not preserved; the diff is %s", diff.ObjectDiff(rrb, rrb2))
		}
	}
}

func TestConversionErrors(t *testing.T) {
	for _, test := range []struct {
		name     string
		expected string
		f        func() error
	}{
		{
			name:     "invalid origin role ref",
			expected: `invalid origin role binding rolebindingname: attempts to reference role in namespace "ns1" instead of current namespace "ns0"`,
			f: func() error {
				return authorizationapi.Convert_api_RoleBinding_To_rbac_RoleBinding(&authorizationapi.RoleBinding{
					ObjectMeta: api.ObjectMeta{Name: "rolebindingname", Namespace: "ns0"},
					RoleRef:    api.ObjectReference{Namespace: "ns1"},
				}, &rbac.RoleBinding{}, nil)
			},
		},
		{
			name:     "invalid origin subject kind",
			expected: `invalid kind for origin subject: "fancyuser"`,
			f: func() error {
				return authorizationapi.Convert_api_ClusterRoleBinding_To_rbac_ClusterRoleBinding(&authorizationapi.ClusterRoleBinding{
					Subjects: []api.ObjectReference{
						{Kind: "fancyuser"},
					},
				}, &rbac.ClusterRoleBinding{}, nil)
			},
		},
		{
			name:     "invalid RBAC subject kind",
			expected: `invalid kind for rbac subject: "evenfancieruser"`,
			f: func() error {
				return authorizationapi.Convert_rbac_ClusterRoleBinding_To_api_ClusterRoleBinding(&rbac.ClusterRoleBinding{
					Subjects: []rbac.Subject{
						{Kind: "evenfancieruser"},
					},
				}, &authorizationapi.ClusterRoleBinding{}, nil)
			},
		},
	} {
		if err := test.f(); err == nil || test.expected != err.Error() {
			t.Errorf("%s failed: expected %q got %v", test.name, test.expected, err)
		}
	}
}

// rules with AttributeRestrictions should not be preserved during conversion
func TestAttributeRestrictionsRuleLoss(t *testing.T) {
	ocr := &authorizationapi.ClusterRole{
		Rules: []authorizationapi.PolicyRule{
			{Verbs: sets.NewString("get"), Resources: sets.NewString("a")},
			{Verbs: sets.NewString("create"), Resources: sets.NewString("b"), AttributeRestrictions: &authorizationapi.Role{}},
			{Verbs: sets.NewString("update"), Resources: sets.NewString("c")},
			{Verbs: sets.NewString("delete"), Resources: sets.NewString("d"), AttributeRestrictions: &authorizationapi.IsPersonalSubjectAccessReview{}},
		},
	}
	ocr2 := &authorizationapi.ClusterRole{}
	rcr := &rbac.ClusterRole{}
	if err := authorizationapi.Convert_api_ClusterRole_To_rbac_ClusterRole(ocr, rcr, nil); err != nil {
		t.Fatal(err)
	}
	if err := authorizationapi.Convert_rbac_ClusterRole_To_api_ClusterRole(rcr, ocr2, nil); err != nil {
		t.Fatal(err)
	}
	if covered, uncoveredRules := rulevalidation.Covers(ocr.Rules, ocr2.Rules); !covered {
		t.Errorf("input rules expected AttributeRestrictions loss not seen; the uncovered rules are %#v", uncoveredRules)
	}
	if covered, uncoveredRules := rulevalidation.Covers(ocr2.Rules, ocr.Rules); !covered {
		t.Errorf("output rules expected AttributeRestrictions loss not seen; the uncovered rules are %#v", uncoveredRules)
	}
}

var fuzzer = fuzz.New().NilChance(0).Funcs(
	func(*unversioned.TypeMeta, fuzz.Continue) {}, // Ignore TypeMeta
	func(*runtime.Object, fuzz.Continue) {},       // Ignore AttributeRestrictions since they are deprecated
	func(ocrb *authorizationapi.ClusterRoleBinding, c fuzz.Continue) {
		c.FuzzNoCustom(ocrb)
		setRandomOriginRoleBindingData(ocrb.Subjects, &ocrb.RoleRef, "", c)
	},
	func(orb *authorizationapi.RoleBinding, c fuzz.Continue) {
		c.FuzzNoCustom(orb)
		setRandomOriginRoleBindingData(orb.Subjects, &orb.RoleRef, orb.Namespace, c)
	},
	func(rcrb *rbac.ClusterRoleBinding, c fuzz.Continue) {
		c.FuzzNoCustom(rcrb)
		setRandomRBACRoleBindingData(rcrb.Subjects, &rcrb.RoleRef, "", c)
	},
	func(rrb *rbac.RoleBinding, c fuzz.Continue) {
		c.FuzzNoCustom(rrb)
		setRandomRBACRoleBindingData(rrb.Subjects, &rrb.RoleRef, rrb.Namespace, c)
	},
	func(rr *rbac.Role, c fuzz.Continue) {
		c.FuzzNoCustom(rr)
		sortAndDeduplicateRBACRulesFields(rr.Rules) // []string <-> sets.String
	},
	func(rcr *rbac.ClusterRole, c fuzz.Continue) {
		c.FuzzNoCustom(rcr)
		sortAndDeduplicateRBACRulesFields(rcr.Rules) // []string <-> sets.String
	},
)

func setRandomRBACRoleBindingData(subjects []rbac.Subject, roleRef *rbac.RoleRef, namespace string, c fuzz.Continue) {
	for i := range subjects {
		subject := &subjects[i]
		subject.APIVersion = rbac.GroupName
		setValidRBACKindAndNamespace(subject, i, c)
	}
	roleRef.APIGroup = rbac.GroupName
	roleRef.Kind = getRBACRoleRefKind(namespace)
}

func setValidRBACKindAndNamespace(subject *rbac.Subject, i int, c fuzz.Continue) {
	kinds := []string{rbac.UserKind, rbac.GroupKind, rbac.ServiceAccountKind}
	kind := kinds[c.Intn(len(kinds))]
	subject.Kind = kind

	if subject.Kind != rbac.ServiceAccountKind {
		subject.Namespace = ""
	} else {
		if len(validation.ValidateServiceAccountName(subject.Name, false)) != 0 {
			subject.Name = fmt.Sprintf("sanamehere%d", i)
		}
	}
}

func setRandomOriginRoleBindingData(subjects []api.ObjectReference, roleRef *api.ObjectReference, namespace string, c fuzz.Continue) {
	for i := range subjects {
		subject := &subjects[i]
		unsetUnusedOriginFields(subject)
		setValidOriginKindAndNamespace(subject, i, c)
	}
	unsetUnusedOriginFields(roleRef)
	roleRef.Kind = ""
	roleRef.Namespace = namespace
}

func setValidOriginKindAndNamespace(subject *api.ObjectReference, i int, c fuzz.Continue) {
	kinds := []string{authorizationapi.UserKind, authorizationapi.SystemUserKind, authorizationapi.GroupKind, authorizationapi.SystemGroupKind, authorizationapi.ServiceAccountKind}
	kind := kinds[c.Intn(len(kinds))]
	subject.Kind = kind

	if subject.Kind != authorizationapi.ServiceAccountKind {
		subject.Namespace = ""
	}

	switch subject.Kind {

	case authorizationapi.UserKind:
		if len(uservalidation.ValidateUserName(subject.Name, false)) != 0 {
			subject.Name = fmt.Sprintf("validusername%d", i)
		}

	case authorizationapi.GroupKind:
		if len(uservalidation.ValidateGroupName(subject.Name, false)) != 0 {
			subject.Name = fmt.Sprintf("validgroupname%d", i)
		}

	case authorizationapi.SystemUserKind, authorizationapi.SystemGroupKind:
		subject.Name = ":" + subject.Name

	case authorizationapi.ServiceAccountKind:
		if len(validation.ValidateServiceAccountName(subject.Name, false)) != 0 {
			subject.Name = fmt.Sprintf("sanamehere%d", i)
		}

	default:
		panic("invalid kind")
	}
}

func unsetUnusedOriginFields(ref *api.ObjectReference) {
	ref.UID = ""
	ref.ResourceVersion = ""
	ref.FieldPath = ""
	ref.APIVersion = ""
}

func sortAndDeduplicateRBACRulesFields(in []rbac.PolicyRule) {
	for i := range in {
		rule := &in[i]
		rule.Verbs = sets.NewString(rule.Verbs...).List()
		rule.Resources = sets.NewString(rule.Resources...).List()
		rule.ResourceNames = sets.NewString(rule.ResourceNames...).List()
		rule.NonResourceURLs = sets.NewString(rule.NonResourceURLs...).List()
	}
}

// copied from authorizationapi since it is a private helper and we need to test in a different package to prevent an import cycle
func getRBACRoleRefKind(namespace string) string {
	kind := "ClusterRole"
	if len(namespace) != 0 {
		kind = "Role"
	}
	return kind
}
