package validation

import (
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/apis/rbac"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
)

func TestValidateRoleBinding(t *testing.T) {
	errs := ValidateRoleBinding(
		&authorizationapi.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{Namespace: metav1.NamespaceDefault, Name: "master"},
			RoleRef:    kapi.ObjectReference{Namespace: "master", Name: "valid"},
			Subjects: []kapi.ObjectReference{
				{Name: "validsaname", Kind: authorizationapi.ServiceAccountKind},
				{Name: "valid@username", Kind: authorizationapi.UserKind},
				{Name: "system:admin", Kind: authorizationapi.SystemUserKind},
				{Name: "valid@groupname", Kind: authorizationapi.GroupKind},
				{Name: "system:authenticated", Kind: authorizationapi.SystemGroupKind},
			},
		},
		true,
	)
	if len(errs) != 0 {
		t.Errorf("expected success: %v", errs)
	}

	errorCases := map[string]struct {
		A authorizationapi.RoleBinding
		T field.ErrorType
		F string
	}{
		"zero-length namespace": {
			A: authorizationapi.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{Name: authorizationapi.PolicyName},
				RoleRef:    kapi.ObjectReference{Namespace: "master", Name: "valid"},
			},
			T: field.ErrorTypeRequired,
			F: "metadata.namespace",
		},
		"zero-length name": {
			A: authorizationapi.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{Namespace: metav1.NamespaceDefault},
				RoleRef:    kapi.ObjectReference{Namespace: "master", Name: "valid"},
			},
			T: field.ErrorTypeRequired,
			F: "metadata.name",
		},
		"invalid ref": {
			A: authorizationapi.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{Namespace: metav1.NamespaceDefault, Name: "name"},
				RoleRef:    kapi.ObjectReference{Namespace: "-192083", Name: "valid"},
			},
			T: field.ErrorTypeInvalid,
			F: "roleRef.namespace",
		},
		"bad role": {
			A: authorizationapi.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{Namespace: metav1.NamespaceDefault, Name: authorizationapi.PolicyName},
				RoleRef:    kapi.ObjectReference{Namespace: authorizationapi.PolicyName},
			},
			T: field.ErrorTypeRequired,
			F: "roleRef.name",
		},
		"bad subject kind": {
			A: authorizationapi.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{Namespace: metav1.NamespaceDefault, Name: "master"},
				RoleRef:    kapi.ObjectReference{Namespace: "master", Name: "valid"},
				Subjects:   []kapi.ObjectReference{{Name: "subject"}},
			},
			T: field.ErrorTypeNotSupported,
			F: "subjects[0].kind",
		},
		"bad subject name": {
			A: authorizationapi.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{Namespace: metav1.NamespaceDefault, Name: "master"},
				RoleRef:    kapi.ObjectReference{Namespace: "master", Name: "valid"},
				Subjects:   []kapi.ObjectReference{{Name: "subject:bad", Kind: authorizationapi.ServiceAccountKind}},
			},
			T: field.ErrorTypeInvalid,
			F: "subjects[0].name",
		},
		"bad system user name": {
			A: authorizationapi.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{Namespace: metav1.NamespaceDefault, Name: "master"},
				RoleRef:    kapi.ObjectReference{Namespace: "master", Name: "valid"},
				Subjects:   []kapi.ObjectReference{{Name: "user", Kind: authorizationapi.SystemUserKind}},
			},
			T: field.ErrorTypeInvalid,
			F: "subjects[0].name",
		},
		"bad system group name": {
			A: authorizationapi.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{Namespace: metav1.NamespaceDefault, Name: "master"},
				RoleRef:    kapi.ObjectReference{Namespace: "master", Name: "valid"},
				Subjects:   []kapi.ObjectReference{{Name: "valid", Kind: authorizationapi.SystemGroupKind}},
			},
			T: field.ErrorTypeInvalid,
			F: "subjects[0].name",
		},
		"forbidden fields": {
			A: authorizationapi.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{Namespace: metav1.NamespaceDefault, Name: "master"},
				RoleRef:    kapi.ObjectReference{Namespace: "master", Name: "valid"},
				Subjects:   []kapi.ObjectReference{{Name: "subject", Kind: authorizationapi.ServiceAccountKind, APIVersion: "foo"}},
			},
			T: field.ErrorTypeForbidden,
			F: "subjects[0].apiVersion",
		},
		"missing subject name": {
			A: authorizationapi.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{Namespace: metav1.NamespaceDefault, Name: "master"},
				RoleRef:    kapi.ObjectReference{Namespace: "master", Name: "valid"},
				Subjects:   []kapi.ObjectReference{{Kind: authorizationapi.ServiceAccountKind}},
			},
			T: field.ErrorTypeRequired,
			F: "subjects[0].name",
		},
	}
	for k, v := range errorCases {
		errs := ValidateRoleBinding(&v.A, true)
		if len(errs) == 0 {
			t.Errorf("expected failure %s for %v", k, v.A)
			continue
		}
		for i := range errs {
			if errs[i].Type != v.T {
				t.Errorf("%s: expected errors to have type %s: %v", k, v.T, errs[i])
			}
			if errs[i].Field != v.F {
				t.Errorf("%s: expected errors to have field %s: %v", k, v.F, errs[i])
			}
		}
	}
}

func TestValidateRoleBindingUpdate(t *testing.T) {
	old := &authorizationapi.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{Namespace: metav1.NamespaceDefault, Name: "master", ResourceVersion: "1"},
		RoleRef:    kapi.ObjectReference{Namespace: "master", Name: "valid"},
	}

	errs := ValidateRoleBindingUpdate(
		&authorizationapi.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{Namespace: metav1.NamespaceDefault, Name: "master", ResourceVersion: "1"},
			RoleRef:    kapi.ObjectReference{Namespace: "master", Name: "valid"},
		},
		old,
		true,
	)
	if len(errs) != 0 {
		t.Errorf("expected success: %v", errs)
	}

	errorCases := map[string]struct {
		A authorizationapi.RoleBinding
		T field.ErrorType
		F string
	}{
		"changedRef": {
			A: authorizationapi.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{Namespace: metav1.NamespaceDefault, Name: "master", ResourceVersion: "1"},
				RoleRef:    kapi.ObjectReference{Namespace: "master", Name: "changed"},
			},
			T: field.ErrorTypeInvalid,
			F: "roleRef",
		},
	}
	for k, v := range errorCases {
		errs := ValidateRoleBindingUpdate(&v.A, old, true)
		if len(errs) == 0 {
			t.Errorf("expected failure %s for %v", k, v.A)
			continue
		}
		for i := range errs {
			if errs[i].Type != v.T {
				t.Errorf("%s: expected errors to have type %s: %v", k, v.T, errs[i])
			}
			if errs[i].Field != v.F {
				t.Errorf("%s: expected errors to have field %s: %v", k, v.F, errs[i])
			}
		}
	}
}

func TestValidateRole(t *testing.T) {
	errs := ValidateRole(
		&authorizationapi.Role{
			ObjectMeta: metav1.ObjectMeta{Namespace: metav1.NamespaceDefault, Name: "master"},
		},
		true,
	)
	if len(errs) != 0 {
		t.Errorf("expected success: %v", errs)
	}

	errorCases := map[string]struct {
		A authorizationapi.Role
		T field.ErrorType
		F string
	}{
		"zero-length namespace": {
			A: authorizationapi.Role{
				ObjectMeta: metav1.ObjectMeta{Name: authorizationapi.PolicyName},
			},
			T: field.ErrorTypeRequired,
			F: "metadata.namespace",
		},
		"zero-length name": {
			A: authorizationapi.Role{
				ObjectMeta: metav1.ObjectMeta{Namespace: metav1.NamespaceDefault},
			},
			T: field.ErrorTypeRequired,
			F: "metadata.name",
		},
		"invalid rule": {
			A: authorizationapi.Role{
				ObjectMeta: metav1.ObjectMeta{Name: authorizationapi.PolicyName, Namespace: kapi.NamespaceDefault},
				Rules: []authorizationapi.PolicyRule{
					{AttributeRestrictions: &authorizationapi.IsPersonalSubjectAccessReview{}},
				},
			},
			T: field.ErrorTypeInvalid,
			F: "rules[0].attributeRestrictions",
		},
	}
	for k, v := range errorCases {
		errs := ValidateRole(&v.A, true)
		if len(errs) == 0 {
			t.Errorf("expected failure %s for %v", k, v.A)
			continue
		}
		for i := range errs {
			if errs[i].Type != v.T {
				t.Errorf("%s: expected errors to have type %s: %v", k, v.T, errs[i])
			}
			if errs[i].Field != v.F {
				t.Errorf("%s: expected errors to have field %s: %v", k, v.F, errs[i])
			}
		}
	}
}

func TestValidateRoleUpdate(t *testing.T) {
	successCases := map[string]struct {
		newO authorizationapi.Role
		old  authorizationapi.Role
	}{
		"no attribute restrictions": {
			newO: authorizationapi.Role{
				ObjectMeta: metav1.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: authorizationapi.PolicyName, ResourceVersion: "0"},
				Rules: []authorizationapi.PolicyRule{
					{},
				},
			},
			old: authorizationapi.Role{
				ObjectMeta: metav1.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: authorizationapi.PolicyName},
			},
		},
		"same attribute restrictions": {
			newO: authorizationapi.Role{
				ObjectMeta: metav1.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: authorizationapi.PolicyName, ResourceVersion: "0"},
				Rules: []authorizationapi.PolicyRule{
					{AttributeRestrictions: &authorizationapi.IsPersonalSubjectAccessReview{}},
				},
			},
			old: authorizationapi.Role{
				ObjectMeta: metav1.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: authorizationapi.PolicyName},
				Rules: []authorizationapi.PolicyRule{
					{AttributeRestrictions: &authorizationapi.IsPersonalSubjectAccessReview{}},
				},
			},
		},
	}
	for k, v := range successCases {
		errs := ValidateRoleUpdate(&v.newO, &v.old, true, nil)
		if len(errs) != 0 {
			t.Errorf("%s: expected success: %v", k, errs)
		}
	}

	errorCases := map[string]struct {
		newO authorizationapi.Role
		old  authorizationapi.Role
		T    field.ErrorType
		F    string
	}{
		"more attribute restrictions": {
			newO: authorizationapi.Role{
				ObjectMeta: metav1.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: authorizationapi.PolicyName, ResourceVersion: "0"},
				Rules: []authorizationapi.PolicyRule{
					{AttributeRestrictions: &authorizationapi.Role{}},
					{AttributeRestrictions: &authorizationapi.IsPersonalSubjectAccessReview{}},
					{AttributeRestrictions: &authorizationapi.ClusterRoleBinding{}},
				},
			},
			old: authorizationapi.Role{
				ObjectMeta: metav1.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: authorizationapi.PolicyName},
				Rules: []authorizationapi.PolicyRule{
					{AttributeRestrictions: &authorizationapi.Role{}},
					{AttributeRestrictions: &authorizationapi.IsPersonalSubjectAccessReview{}},
				},
			},
			T: field.ErrorTypeInvalid,
			F: "rules[2].attributeRestrictions",
		},
		"added attribute restrictions": {
			newO: authorizationapi.Role{
				ObjectMeta: metav1.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: authorizationapi.PolicyName, ResourceVersion: "0"},
				Rules: []authorizationapi.PolicyRule{
					{AttributeRestrictions: &authorizationapi.Role{}},
				},
			},
			old: authorizationapi.Role{
				ObjectMeta: metav1.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: authorizationapi.PolicyName},
				Rules: []authorizationapi.PolicyRule{
					{},
				},
			},
			T: field.ErrorTypeInvalid,
			F: "rules[0].attributeRestrictions",
		},
		"same number of different attribute restrictions": {
			newO: authorizationapi.Role{
				ObjectMeta: metav1.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: authorizationapi.PolicyName, ResourceVersion: "0"},
				Rules: []authorizationapi.PolicyRule{
					{AttributeRestrictions: &authorizationapi.IsPersonalSubjectAccessReview{}},
				},
			},
			old: authorizationapi.Role{
				ObjectMeta: metav1.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: authorizationapi.PolicyName},
				Rules: []authorizationapi.PolicyRule{
					{AttributeRestrictions: &authorizationapi.Role{}},
				},
			},
			T: field.ErrorTypeInvalid,
			F: "rules[0].attributeRestrictions",
		},
		"less but different attribute restrictions": {
			newO: authorizationapi.Role{
				ObjectMeta: metav1.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: authorizationapi.PolicyName, ResourceVersion: "0"},
				Rules: []authorizationapi.PolicyRule{
					{AttributeRestrictions: &authorizationapi.ClusterRoleBinding{}},
				},
			},
			old: authorizationapi.Role{
				ObjectMeta: metav1.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: authorizationapi.PolicyName},
				Rules: []authorizationapi.PolicyRule{
					{AttributeRestrictions: &authorizationapi.Role{}},
					{AttributeRestrictions: &authorizationapi.IsPersonalSubjectAccessReview{}},
				},
			},
			T: field.ErrorTypeInvalid,
			F: "rules[0].attributeRestrictions",
		},
	}
	for k, v := range errorCases {
		errs := ValidateRoleUpdate(&v.newO, &v.old, true, nil)
		if len(errs) == 0 {
			t.Errorf("expected failure %s for %v %v", k, v.newO, v.old)
			continue
		}
		for i := range errs {
			if errs[i].Type != v.T {
				t.Errorf("%s: expected errors to have type %s: %v", k, v.T, errs[i])
			}
			if errs[i].Field != v.F {
				t.Errorf("%s: expected errors to have field %s: %v", k, v.F, errs[i])
			}
		}
	}
}

func TestValidateAccessRestriction(t *testing.T) {
	type args struct {
		obj *authorizationapi.AccessRestriction
	}
	tests := []struct {
		name string
		args args
		want field.ErrorList
	}{
		{
			name: "valid allowed subjects",
			args: args{
				obj: &authorizationapi.AccessRestriction{
					ObjectMeta: metav1.ObjectMeta{
						Name: "whitelist-write-jobs",
					},
					Spec: authorizationapi.AccessRestrictionSpec{
						MatchAttributes: []rbac.PolicyRule{
							{
								Verbs:     []string{"create", "update", "patch", "delete", "deletecollection"},
								APIGroups: []string{"batch"},
								Resources: []string{"jobs"},
							},
						},
						AllowedSubjects: []authorizationapi.SubjectMatcher{
							{
								GroupRestriction: &authorizationapi.GroupRestriction{
									Groups: []string{"jobGroup"},
								},
							},
						},
						DeniedSubjects: []authorizationapi.SubjectMatcher{
							{
								UserRestriction: &authorizationapi.UserRestriction{
									Groups: []string{"system:authenticated", "system:unauthenticated"},
								},
							},
						},
					},
				},
			},
			want: field.ErrorList{},
		},
		{
			name: "valid denied subjects",
			args: args{
				obj: &authorizationapi.AccessRestriction{
					ObjectMeta: metav1.ObjectMeta{
						Name: "blacklist-label-get-pods",
					},
					Spec: authorizationapi.AccessRestrictionSpec{
						MatchAttributes: []rbac.PolicyRule{
							{
								Verbs:     []string{"list"},
								APIGroups: []string{""},
								Resources: []string{"pods"},
							},
						},
						DeniedSubjects: []authorizationapi.SubjectMatcher{
							{
								UserRestriction: &authorizationapi.UserRestriction{
									Groups: []string{"non-admins"},
								},
							},
							{
								GroupRestriction: &authorizationapi.GroupRestriction{
									Selectors: []metav1.LabelSelector{
										{
											MatchLabels: map[string]string{
												"alsobad": "yup",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			want: field.ErrorList{},
		},
		{
			name: "invalid object meta",
			args: args{
				obj: &authorizationapi.AccessRestriction{
					Spec: authorizationapi.AccessRestrictionSpec{
						MatchAttributes: []rbac.PolicyRule{
							{
								Verbs:     []string{"get"},
								APIGroups: []string{""},
								Resources: []string{"pods"},
							},
						},
						DeniedSubjects: []authorizationapi.SubjectMatcher{
							{
								UserRestriction: &authorizationapi.UserRestriction{
									Groups: []string{"non-admins"},
								},
							},
						},
					},
				},
			},
			want: field.ErrorList{
				{Type: field.ErrorTypeRequired, Field: "metadata.name", BadValue: "", Detail: "name or generateName is required"},
			},
		},
		{
			name: "missing match attributes",
			args: args{
				obj: &authorizationapi.AccessRestriction{
					ObjectMeta: metav1.ObjectMeta{
						Name: "name",
					},
					Spec: authorizationapi.AccessRestrictionSpec{
						DeniedSubjects: []authorizationapi.SubjectMatcher{
							{
								UserRestriction: &authorizationapi.UserRestriction{
									Groups: []string{"group"},
								},
							},
						},
					},
				},
			},
			want: field.ErrorList{
				{Type: field.ErrorTypeRequired, Field: "spec.matchAttributes", BadValue: "", Detail: "must supply at least one policy rule"},
			},
		},
		{
			name: "invalid policy rule and user restriction",
			args: args{
				obj: &authorizationapi.AccessRestriction{
					ObjectMeta: metav1.ObjectMeta{
						Name: "name",
					},
					Spec: authorizationapi.AccessRestrictionSpec{
						MatchAttributes: []rbac.PolicyRule{
							{
								Verbs:     []string{"list"},
								Resources: []string{"pods"},
							},
						},
						DeniedSubjects: []authorizationapi.SubjectMatcher{
							{
								UserRestriction: &authorizationapi.UserRestriction{
									Groups: []string{},
								},
							},
						},
					},
				},
			},
			want: field.ErrorList{
				{Type: field.ErrorTypeRequired, Field: "spec.matchAttributes[0].apiGroups", BadValue: "", Detail: "resource rules must supply at least one api group"},
				{Type: field.ErrorTypeRequired, Field: "spec.deniedSubjects[0].userRestriction.users", BadValue: "", Detail: "must specify at least one user, group, or label selector"},
			},
		},
		{
			name: "missing subject matcher",
			args: args{
				obj: &authorizationapi.AccessRestriction{
					ObjectMeta: metav1.ObjectMeta{
						Name: "name",
					},
					Spec: authorizationapi.AccessRestrictionSpec{
						MatchAttributes: []rbac.PolicyRule{
							{
								Verbs:     []string{"get"},
								APIGroups: []string{""},
								Resources: []string{"pods"},
							},
						},
					},
				},
			},
			want: field.ErrorList{
				{Type: field.ErrorTypeRequired, Field: "spec.deniedSubjects", BadValue: "", Detail: "deniedSubjects must be specified"},
			},
		},
		{
			name: "both allow and deny subject matcher",
			args: args{
				obj: &authorizationapi.AccessRestriction{
					ObjectMeta: metav1.ObjectMeta{
						Name: "name",
					},
					Spec: authorizationapi.AccessRestrictionSpec{
						MatchAttributes: []rbac.PolicyRule{
							{
								Verbs:     []string{"get"},
								APIGroups: []string{""},
								Resources: []string{"pods"},
							},
						},
						AllowedSubjects: []authorizationapi.SubjectMatcher{
							{
								UserRestriction: &authorizationapi.UserRestriction{
									Groups: []string{"group"},
								},
							},
						},
						DeniedSubjects: []authorizationapi.SubjectMatcher{
							{
								GroupRestriction: &authorizationapi.GroupRestriction{
									Groups: []string{"group"},
								},
							},
						},
					},
				},
			},
			want: field.ErrorList{},
		},
		{
			name: "both user and group subject matcher",
			args: args{
				obj: &authorizationapi.AccessRestriction{
					ObjectMeta: metav1.ObjectMeta{
						Name: "name",
					},
					Spec: authorizationapi.AccessRestrictionSpec{
						MatchAttributes: []rbac.PolicyRule{
							{
								Verbs:     []string{"create"},
								APIGroups: []string{"batch"},
								Resources: []string{"jobs"},
							},
						},
						AllowedSubjects: []authorizationapi.SubjectMatcher{
							{},
						},
						DeniedSubjects: []authorizationapi.SubjectMatcher{
							{
								UserRestriction: &authorizationapi.UserRestriction{
									Groups: []string{"system:authenticated", "system:unauthenticated"},
								},
							},
						},
					},
				},
			},
			want: field.ErrorList{
				{Type: field.ErrorTypeInvalid, Field: "spec.allowedSubjects[0].userRestriction", BadValue: "<omitted>", Detail: "either userRestriction or groupRestriction must be specified"},
			},
		},
		{
			name: "both user and group subject matcher, allowed subjects",
			args: args{
				obj: &authorizationapi.AccessRestriction{
					ObjectMeta: metav1.ObjectMeta{
						Name: "name",
					},
					Spec: authorizationapi.AccessRestrictionSpec{
						MatchAttributes: []rbac.PolicyRule{
							{
								Verbs:     []string{"create"},
								APIGroups: []string{"batch"},
								Resources: []string{"jobs"},
							},
						},
						AllowedSubjects: []authorizationapi.SubjectMatcher{
							{
								UserRestriction: &authorizationapi.UserRestriction{},
							},
							{
								UserRestriction: &authorizationapi.UserRestriction{
									Selectors: []metav1.LabelSelector{
										{
											MatchLabels: map[string]string{
												"foo": "+",
											},
										},
									},
								},
							},
							{
								UserRestriction: &authorizationapi.UserRestriction{
									Groups: []string{"jobGroup"},
								},
								GroupRestriction: &authorizationapi.GroupRestriction{
									Groups: []string{"jobGroup"},
								},
							},
						},
						DeniedSubjects: []authorizationapi.SubjectMatcher{
							{
								UserRestriction: &authorizationapi.UserRestriction{
									Groups: []string{"system:authenticated", "system:unauthenticated"},
								},
							},
						},
					},
				},
			},
			want: field.ErrorList{
				{Type: field.ErrorTypeRequired, Field: "spec.allowedSubjects[0].userRestriction.users", BadValue: "", Detail: "must specify at least one user, group, or label selector"},
				{Type: field.ErrorTypeInvalid, Field: "spec.allowedSubjects[1].userRestriction.labels[0].matchLabels", BadValue: "+",
					Detail: "a valid label must be an empty string or consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character " +
						"(e.g. 'MyValue',  or 'my_value',  or '12345', regex used for validation is '(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?')"},
				{Type: field.ErrorTypeInvalid, Field: "spec.allowedSubjects[2].groupRestriction", BadValue: "<omitted>", Detail: "both userRestriction and groupRestriction cannot be specified"},
			},
		},
		{
			name: "both user and group subject matcher, denied subjects",
			args: args{
				obj: &authorizationapi.AccessRestriction{
					ObjectMeta: metav1.ObjectMeta{
						Name: "name",
					},
					Spec: authorizationapi.AccessRestrictionSpec{
						MatchAttributes: []rbac.PolicyRule{
							{
								Verbs:     []string{"create"},
								APIGroups: []string{"batch"},
								Resources: []string{"jobs"},
							},
						},
						DeniedSubjects: []authorizationapi.SubjectMatcher{
							{
								UserRestriction: &authorizationapi.UserRestriction{},
							},
							{
								UserRestriction: &authorizationapi.UserRestriction{
									Selectors: []metav1.LabelSelector{
										{
											MatchLabels: map[string]string{
												"foo": "+",
											},
										},
									},
								},
							},
							{
								UserRestriction: &authorizationapi.UserRestriction{
									Groups: []string{"jobGroup"},
								},
								GroupRestriction: &authorizationapi.GroupRestriction{
									Groups: []string{"jobGroup"},
								},
							},
						},
					},
				},
			},
			want: field.ErrorList{
				{Type: field.ErrorTypeRequired, Field: "spec.deniedSubjects[0].userRestriction.users", BadValue: "", Detail: "must specify at least one user, group, or label selector"},
				{Type: field.ErrorTypeInvalid, Field: "spec.deniedSubjects[1].userRestriction.labels[0].matchLabels", BadValue: "+",
					Detail: "a valid label must be an empty string or consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character " +
						"(e.g. 'MyValue',  or 'my_value',  or '12345', regex used for validation is '(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?')"},
				{Type: field.ErrorTypeInvalid, Field: "spec.deniedSubjects[2].groupRestriction", BadValue: "<omitted>", Detail: "both userRestriction and groupRestriction cannot be specified"},
			},
		},
		{
			name: "subject matcher, allowed subjects, group restriction",
			args: args{
				obj: &authorizationapi.AccessRestriction{
					ObjectMeta: metav1.ObjectMeta{
						Name: "name",
					},
					Spec: authorizationapi.AccessRestrictionSpec{
						MatchAttributes: []rbac.PolicyRule{
							{
								Verbs:     []string{"create"},
								APIGroups: []string{"batch"},
								Resources: []string{"jobs"},
							},
						},
						AllowedSubjects: []authorizationapi.SubjectMatcher{
							{
								GroupRestriction: &authorizationapi.GroupRestriction{},
							},
							{
								GroupRestriction: &authorizationapi.GroupRestriction{
									Selectors: []metav1.LabelSelector{
										{
											MatchLabels: map[string]string{
												"foo": "+",
											},
										},
									},
								},
							},
						},
						DeniedSubjects: []authorizationapi.SubjectMatcher{
							{
								UserRestriction: &authorizationapi.UserRestriction{
									Groups: []string{"system:authenticated", "system:unauthenticated"},
								},
							},
						},
					},
				},
			},
			want: field.ErrorList{
				{Type: field.ErrorTypeRequired, Field: "spec.allowedSubjects[0].groupRestriction.groups", BadValue: "", Detail: "must specify at least one group or label selector"},
				{Type: field.ErrorTypeInvalid, Field: "spec.allowedSubjects[1].groupRestriction.labels[0].matchLabels", BadValue: "+",
					Detail: "a valid label must be an empty string or consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character " +
						"(e.g. 'MyValue',  or 'my_value',  or '12345', regex used for validation is '(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?')"},
			},
		},
		{
			name: "subject matcher, denied subjects, group restriction",
			args: args{
				obj: &authorizationapi.AccessRestriction{
					ObjectMeta: metav1.ObjectMeta{
						Name: "name",
					},
					Spec: authorizationapi.AccessRestrictionSpec{
						MatchAttributes: []rbac.PolicyRule{
							{
								Verbs:     []string{"create"},
								APIGroups: []string{"batch"},
								Resources: []string{"jobs"},
							},
						},
						DeniedSubjects: []authorizationapi.SubjectMatcher{
							{
								GroupRestriction: &authorizationapi.GroupRestriction{},
							},
							{
								GroupRestriction: &authorizationapi.GroupRestriction{
									Selectors: []metav1.LabelSelector{
										{
											MatchLabels: map[string]string{
												"foo": "+",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			want: field.ErrorList{
				{Type: field.ErrorTypeRequired, Field: "spec.deniedSubjects[0].groupRestriction.groups", BadValue: "", Detail: "must specify at least one group or label selector"},
				{Type: field.ErrorTypeInvalid, Field: "spec.deniedSubjects[1].groupRestriction.labels[0].matchLabels", BadValue: "+",
					Detail: "a valid label must be an empty string or consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character " +
						"(e.g. 'MyValue',  or 'my_value',  or '12345', regex used for validation is '(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?')"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidateAccessRestriction(tt.args.obj); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ValidateAccessRestriction() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateAccessRestrictionUpdate(t *testing.T) {
	type args struct {
		obj *authorizationapi.AccessRestriction
		old *authorizationapi.AccessRestriction
	}
	tests := []struct {
		name string
		args args
		want field.ErrorList
	}{
		{
			name: "change from whitelist to blacklist",
			args: args{
				obj: &authorizationapi.AccessRestriction{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "some-name",
						ResourceVersion: "1",
					},
					Spec: authorizationapi.AccessRestrictionSpec{
						MatchAttributes: []rbac.PolicyRule{
							{
								Verbs:     []string{"educate"},
								APIGroups: []string{""},
								Resources: []string{"dolphins"},
							},
						},
						DeniedSubjects: []authorizationapi.SubjectMatcher{
							{
								UserRestriction: &authorizationapi.UserRestriction{
									Users: []string{"liggitt"},
								},
							},
							{
								GroupRestriction: &authorizationapi.GroupRestriction{
									Groups: []string{"pandas"},
								},
							},
						},
					},
				},
				old: &authorizationapi.AccessRestriction{
					ObjectMeta: metav1.ObjectMeta{
						Name: "some-name",
					},
					Spec: authorizationapi.AccessRestrictionSpec{
						MatchAttributes: []rbac.PolicyRule{
							{
								Verbs:     []string{"create", "update", "patch", "delete", "deletecollection"},
								APIGroups: []string{"batch"},
								Resources: []string{"jobs"},
							},
						},
						AllowedSubjects: []authorizationapi.SubjectMatcher{
							{
								GroupRestriction: &authorizationapi.GroupRestriction{
									Groups: []string{"jobGroup"},
								},
							},
						},
					},
				},
			},
			want: field.ErrorList{},
		},
		{
			name: "invalid object metadata update name",
			args: args{
				obj: &authorizationapi.AccessRestriction{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "some-other-name",
						ResourceVersion: "1",
					},
					Spec: authorizationapi.AccessRestrictionSpec{
						MatchAttributes: []rbac.PolicyRule{
							{
								Verbs:     []string{"list"},
								APIGroups: []string{""},
								Resources: []string{"pods"},
							},
						},
						DeniedSubjects: []authorizationapi.SubjectMatcher{
							{
								UserRestriction: &authorizationapi.UserRestriction{
									Selectors: []metav1.LabelSelector{
										{
											MatchLabels: map[string]string{
												"bad": "yes",
											},
										},
									},
								},
							},
							{
								GroupRestriction: &authorizationapi.GroupRestriction{
									Selectors: []metav1.LabelSelector{
										{
											MatchLabels: map[string]string{
												"alsobad": "yup",
											},
										},
									},
								},
							},
						},
					},
				},
				old: &authorizationapi.AccessRestriction{
					ObjectMeta: metav1.ObjectMeta{
						Name: "some-name",
					},
					Spec: authorizationapi.AccessRestrictionSpec{
						MatchAttributes: []rbac.PolicyRule{
							{
								Verbs:     []string{"create", "update", "patch", "delete", "deletecollection"},
								APIGroups: []string{"batch"},
								Resources: []string{"jobs"},
							},
						},
						AllowedSubjects: []authorizationapi.SubjectMatcher{
							{
								GroupRestriction: &authorizationapi.GroupRestriction{
									Groups: []string{"jobGroup"},
								},
							},
						},
					},
				},
			},
			want: field.ErrorList{
				{Type: field.ErrorTypeInvalid, Field: "metadata.name", BadValue: "some-other-name", Detail: "field is immutable"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidateAccessRestrictionUpdate(tt.args.obj, tt.args.old); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ValidateAccessRestrictionUpdate() = %v, want %v", got, tt.want)
			}
		})
	}
}
