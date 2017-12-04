package rbacconversion

import (
	"fmt"
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/apimachinery/pkg/util/sets"
	api "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/apis/core/validation"
	"k8s.io/kubernetes/pkg/apis/rbac"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	"github.com/openshift/origin/pkg/authorization/rulevalidation"
	uservalidation "github.com/openshift/origin/pkg/user/apis/user/validation"

	"github.com/google/gofuzz"
)

// make sure rbac <-> origin round trip does not lose any data

func TestOriginClusterRoleFidelity(t *testing.T) {
	for i := 0; i < 100; i++ {
		ocr := &authorizationapi.ClusterRole{}
		ocr2 := &authorizationapi.ClusterRole{}
		rcr := &rbac.ClusterRole{}
		fuzzer.Fuzz(ocr)
		if err := Convert_authorization_ClusterRole_To_rbac_ClusterRole(ocr, rcr, nil); err != nil {
			t.Fatal(err)
		}
		if err := Convert_rbac_ClusterRole_To_authorization_ClusterRole(rcr, ocr2, nil); err != nil {
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
		if err := Convert_authorization_Role_To_rbac_Role(or, rr, nil); err != nil {
			t.Fatal(err)
		}
		if err := Convert_rbac_Role_To_authorization_Role(rr, or2, nil); err != nil {
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
		if err := Convert_authorization_ClusterRoleBinding_To_rbac_ClusterRoleBinding(ocrb, rcrb, nil); err != nil {
			t.Fatal(err)
		}
		if err := Convert_rbac_ClusterRoleBinding_To_authorization_ClusterRoleBinding(rcrb, ocrb2, nil); err != nil {
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
		if err := Convert_authorization_RoleBinding_To_rbac_RoleBinding(orb, rrb, nil); err != nil {
			t.Fatal(err)
		}
		if err := Convert_rbac_RoleBinding_To_authorization_RoleBinding(rrb, orb2, nil); err != nil {
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
		if err := Convert_rbac_ClusterRole_To_authorization_ClusterRole(rcr, ocr, nil); err != nil {
			t.Fatal(err)
		}
		if err := Convert_authorization_ClusterRole_To_rbac_ClusterRole(ocr, rcr2, nil); err != nil {
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
		if err := Convert_rbac_Role_To_authorization_Role(rr, or, nil); err != nil {
			t.Fatal(err)
		}
		if err := Convert_authorization_Role_To_rbac_Role(or, rr2, nil); err != nil {
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
		if err := Convert_rbac_ClusterRoleBinding_To_authorization_ClusterRoleBinding(rcrb, ocrb, nil); err != nil {
			t.Fatal(err)
		}
		if err := Convert_authorization_ClusterRoleBinding_To_rbac_ClusterRoleBinding(ocrb, rcrb2, nil); err != nil {
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
		if err := Convert_rbac_RoleBinding_To_authorization_RoleBinding(rrb, orb, nil); err != nil {
			t.Fatal(err)
		}
		if err := Convert_authorization_RoleBinding_To_rbac_RoleBinding(orb, rrb2, nil); err != nil {
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
				return Convert_authorization_RoleBinding_To_rbac_RoleBinding(&authorizationapi.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{Name: "rolebindingname", Namespace: "ns0"},
					RoleRef:    api.ObjectReference{Namespace: "ns1"},
				}, &rbac.RoleBinding{}, nil)
			},
		},
		{
			name:     "invalid origin subject kind",
			expected: `invalid kind for origin subject: "fancyuser"`,
			f: func() error {
				return Convert_authorization_ClusterRoleBinding_To_rbac_ClusterRoleBinding(&authorizationapi.ClusterRoleBinding{
					Subjects: []api.ObjectReference{
						{Kind: "fancyuser"},
					},
				}, &rbac.ClusterRoleBinding{}, nil)
			},
		},
		{
			name:     "invalid origin rol ref namespace",
			expected: `invalid origin cluster role binding clusterrolebindingname: attempts to reference role in namespace "fancyns" instead of cluster scope`,
			f: func() error {
				return Convert_authorization_ClusterRoleBinding_To_rbac_ClusterRoleBinding(&authorizationapi.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{Name: "clusterrolebindingname"},
					RoleRef: api.ObjectReference{
						Namespace: "fancyns",
					},
				}, &rbac.ClusterRoleBinding{}, nil)
			},
		},
		{
			name:     "invalid RBAC subject kind",
			expected: `invalid kind for rbac subject: "evenfancieruser"`,
			f: func() error {
				return Convert_rbac_ClusterRoleBinding_To_authorization_ClusterRoleBinding(&rbac.ClusterRoleBinding{
					Subjects: []rbac.Subject{
						{Kind: "evenfancieruser"},
					},
				}, &authorizationapi.ClusterRoleBinding{}, nil)
			},
		},
		{
			name:     "invalid RBAC rol ref kind",
			expected: `invalid kind "anewfancykind" for rbac role ref "fancyrolref"`,
			f: func() error {
				return Convert_rbac_ClusterRoleBinding_To_authorization_ClusterRoleBinding(&rbac.ClusterRoleBinding{
					RoleRef: rbac.RoleRef{
						Name: "fancyrolref",
						Kind: "anewfancykind",
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
	if err := Convert_authorization_ClusterRole_To_rbac_ClusterRole(ocr, rcr, nil); err != nil {
		t.Fatal(err)
	}
	if err := Convert_rbac_ClusterRole_To_authorization_ClusterRole(rcr, ocr2, nil); err != nil {
		t.Fatal(err)
	}
	if covered, uncoveredRules := rulevalidation.Covers(ocr.Rules, ocr2.Rules); !covered {
		t.Errorf("input rules expected AttributeRestrictions loss not seen; the uncovered rules are %#v", uncoveredRules)
	}
	if covered, uncoveredRules := rulevalidation.Covers(ocr2.Rules, ocr.Rules); !covered {
		t.Errorf("output rules expected AttributeRestrictions loss not seen; the uncovered rules are %#v", uncoveredRules)
	}
}

// rules with both resources and non-resources should be split during conversion
func TestResourceAndNonResourceRuleSplit(t *testing.T) {
	ocr := &authorizationapi.ClusterRole{
		Rules: []authorizationapi.PolicyRule{
			{
				Verbs:           sets.NewString("get", "create"),
				APIGroups:       []string{"v1", ""},
				Resources:       sets.NewString("pods", "nodes"),
				ResourceNames:   sets.NewString("foo", "bar"),
				NonResourceURLs: sets.NewString("/api", "/health"),
			},
		},
	}
	ocr2 := &authorizationapi.ClusterRole{}
	rcr := &rbac.ClusterRole{}
	if err := Convert_authorization_ClusterRole_To_rbac_ClusterRole(ocr, rcr, nil); err != nil {
		t.Fatal(err)
	}
	if err := Convert_rbac_ClusterRole_To_authorization_ClusterRole(rcr, ocr2, nil); err != nil {
		t.Fatal(err)
	}
	// We need to break down the input rules so Covers does not get confused by ResourceNames
	ocrRulesBrokenDown := []authorizationapi.PolicyRule{}
	for _, servantRule := range ocr.Rules {
		ocrRulesBrokenDown = append(ocrRulesBrokenDown, rulevalidation.BreakdownRule(servantRule)...)
	}
	if covered, uncoveredRules := rulevalidation.Covers(ocrRulesBrokenDown, ocr2.Rules); !covered {
		t.Errorf("input rules expected rule split not seen; the uncovered rules are %#v", uncoveredRules)
	}
	if covered, uncoveredRules := rulevalidation.Covers(ocr2.Rules, ocr.Rules); !covered {
		t.Errorf("output rules expected rule split not seen; the uncovered rules are %#v", uncoveredRules)
	}
}

func TestAnnotationsConversion(t *testing.T) {
	for _, boolval := range []string{"true", "false"} {
		ocr := &authorizationapi.ClusterRole{
			Rules: []authorizationapi.PolicyRule{},
		}
		ocr.Annotations = map[string]string{"openshift.io/reconcile-protect": boolval}
		ocr2 := &authorizationapi.ClusterRole{}
		crcr := &rbac.ClusterRole{}
		if err := Convert_authorization_ClusterRole_To_rbac_ClusterRole(ocr, crcr, nil); err != nil {
			t.Fatal(err)
		}
		value, ok := crcr.Annotations[rbac.AutoUpdateAnnotationKey]
		if ok {
			if (boolval == "true" && value != "false") || (boolval == "false" && value != "true") {
				t.Fatal(fmt.Errorf("Wrong conversion value, 'true' instead of 'false'"))
			}
		} else {
			t.Fatal(fmt.Errorf("Missing converted Annotation"))
		}
		if err := Convert_rbac_ClusterRole_To_authorization_ClusterRole(crcr, ocr2, nil); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(ocr, ocr2) {
			t.Errorf("origin cluster data not preserved; the diff is %s", diff.ObjectDiff(ocr, ocr2))
		}

		rcr := &rbac.ClusterRole{
			Rules: []rbac.PolicyRule{},
		}
		rcr.Annotations = map[string]string{rbac.AutoUpdateAnnotationKey: boolval}
		rcr2 := &rbac.ClusterRole{}
		cocr := &authorizationapi.ClusterRole{}
		if err := Convert_rbac_ClusterRole_To_authorization_ClusterRole(rcr, cocr, nil); err != nil {
			t.Fatal(err)
		}
		value, ok = cocr.Annotations["openshift.io/reconcile-protect"]
		if ok {
			if (boolval == "true" && value != "false") || (boolval == "false" && value != "true") {
				t.Fatal(fmt.Errorf("Wrong conversion value, 'true' instead of 'false'"))
			}
		} else {
			t.Fatal(fmt.Errorf("Missing converted Annotation"))
		}
		if err := Convert_authorization_ClusterRole_To_rbac_ClusterRole(cocr, rcr2, nil); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(rcr, rcr2) {
			t.Errorf("rbac cluster data not preserved; the diff is %s", diff.ObjectDiff(rcr, rcr2))
		}
	}
}

var fuzzer = fuzz.New().NilChance(0).Funcs(
	func(*metav1.TypeMeta, fuzz.Continue) {}, // Ignore TypeMeta
	func(*runtime.Object, fuzz.Continue) {},  // Ignore AttributeRestrictions since they are deprecated
	func(ocrb *authorizationapi.ClusterRoleBinding, c fuzz.Continue) {
		c.FuzzNoCustom(ocrb)
		setRandomOriginRoleBindingData(ocrb.Subjects, &ocrb.RoleRef, "", c)
	},
	func(orb *authorizationapi.RoleBinding, c fuzz.Continue) {
		c.FuzzNoCustom(orb)
		setRandomOriginRoleBindingData(orb.Subjects, &orb.RoleRef, orb.Namespace, c)
	},
	func(or *authorizationapi.Role, c fuzz.Continue) {
		c.FuzzNoCustom(or)
		setOriginRuleType(or.Rules, c.RandBool())
	},
	func(ocr *authorizationapi.ClusterRole, c fuzz.Continue) {
		c.FuzzNoCustom(ocr)
		setOriginRuleType(ocr.Rules, c.RandBool())
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
		setRBACRuleType(rr.Rules, c.RandBool())
		sortAndDeduplicateRBACRulesFields(rr.Rules) // []string <-> sets.String
	},
	func(rcr *rbac.ClusterRole, c fuzz.Continue) {
		c.FuzzNoCustom(rcr)
		setRBACRuleType(rcr.Rules, c.RandBool())
		sortAndDeduplicateRBACRulesFields(rcr.Rules) // []string <-> sets.String
	},
)

func setOriginRuleType(in []authorizationapi.PolicyRule, isResourceRule bool) {
	if isResourceRule {
		for i := range in {
			rule := &in[i]
			rule.NonResourceURLs = sets.NewString()
		}
	} else {
		for i := range in {
			rule := &in[i]
			rule.APIGroups = []string{}
			rule.Resources = sets.NewString()
			rule.ResourceNames = sets.NewString()
		}
	}
}

func setRBACRuleType(in []rbac.PolicyRule, isResourceRule bool) {
	if isResourceRule {
		for i := range in {
			rule := &in[i]
			rule.NonResourceURLs = []string{}
		}
	} else {
		for i := range in {
			rule := &in[i]
			rule.APIGroups = []string{}
			rule.Resources = []string{}
			rule.ResourceNames = []string{}
		}
	}
}

func setRandomRBACRoleBindingData(subjects []rbac.Subject, roleRef *rbac.RoleRef, namespace string, c fuzz.Continue) {
	for i := range subjects {
		subject := &subjects[i]
		subject.APIGroup = rbac.GroupName
		setValidRBACKindAndNamespace(subject, i, c)
	}
	roleRef.APIGroup = rbac.GroupName
	roleRef.Kind = getRBACRoleRefKind(getRandomScope(namespace, c))
}

func setValidRBACKindAndNamespace(subject *rbac.Subject, i int, c fuzz.Continue) {
	kinds := []string{rbac.UserKind, rbac.GroupKind, rbac.ServiceAccountKind}
	kind := kinds[c.Intn(len(kinds))]
	subject.Kind = kind

	if subject.Kind != rbac.ServiceAccountKind {
		subject.Namespace = ""
	} else {
		subject.APIGroup = ""
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
	roleRef.Namespace = getRandomScope(namespace, c)
}

// we want bindings to both cluster and local roles
func getRandomScope(namespace string, c fuzz.Continue) string {
	if c.RandBool() {
		return ""
	}
	return namespace
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
