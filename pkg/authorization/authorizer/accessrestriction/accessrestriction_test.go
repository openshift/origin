package accessrestriction

import (
	"reflect"
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/client-go/tools/cache"
	"k8s.io/kubernetes/pkg/serviceaccount"

	authorizationv1 "github.com/openshift/api/authorization/v1"
	authorizationv1alpha1 "github.com/openshift/api/authorization/v1alpha1"
	userv1 "github.com/openshift/api/user/v1"
	authorizationlisters "github.com/openshift/client-go/authorization/listers/authorization/v1alpha1"
	userlisters "github.com/openshift/client-go/user/listers/user/v1"
)

func Test_accessRestrictionAuthorizer_Authorize(t *testing.T) {
	podWhitelistGroup := &authorizationv1alpha1.AccessRestriction{
		Spec: authorizationv1alpha1.AccessRestrictionSpec{
			MatchAttributes: []rbacv1.PolicyRule{
				{
					Verbs:     []string{"get"},
					APIGroups: []string{""},
					Resources: []string{"pods"},
				},
			},
			AllowedSubjects: []authorizationv1alpha1.SubjectMatcher{
				{
					GroupRestriction: &authorizationv1.GroupRestriction{
						Groups: []string{"admins", "system:serviceaccounts"},
					},
				},
			},
			DeniedSubjects: []authorizationv1alpha1.SubjectMatcher{
				{
					GroupRestriction: &authorizationv1.GroupRestriction{
						Groups: []string{"system:authenticated", "system:unauthenticated"},
					},
				},
			},
		},
	}
	secretWhitelistGroup := &authorizationv1alpha1.AccessRestriction{
		Spec: authorizationv1alpha1.AccessRestrictionSpec{
			MatchAttributes: []rbacv1.PolicyRule{
				{
					Verbs:     []string{"get"},
					APIGroups: []string{""},
					Resources: []string{"secrets"},
				},
			},
			AllowedSubjects: []authorizationv1alpha1.SubjectMatcher{
				{
					GroupRestriction: &authorizationv1.GroupRestriction{
						Groups: []string{"system:serviceaccounts:ns2"},
						Selectors: []v1.LabelSelector{
							{
								MatchLabels: map[string]string{
									"can": "secret",
								},
							},
						},
					},
				},
			},
			DeniedSubjects: []authorizationv1alpha1.SubjectMatcher{
				{
					GroupRestriction: &authorizationv1.GroupRestriction{
						Groups: []string{"system:authenticated", "system:unauthenticated"},
					},
				},
			},
		},
	}
	secretLabelGroup := &userv1.Group{
		ObjectMeta: v1.ObjectMeta{
			Labels: map[string]string{
				"can": "secret",
			},
		},
		Users: userv1.OptionalNames{
			"bob",
		},
	}
	secretLabelGroupNoUsers := &userv1.Group{
		ObjectMeta: v1.ObjectMeta{
			Name: "sgroup",
			Labels: map[string]string{
				"can": "secret",
			},
		},
	}
	configmapWhitelistUser := &authorizationv1alpha1.AccessRestriction{
		Spec: authorizationv1alpha1.AccessRestrictionSpec{
			MatchAttributes: []rbacv1.PolicyRule{
				{
					Verbs:     []string{"list"},
					APIGroups: []string{""},
					Resources: []string{"configmaps"},
				},
			},
			AllowedSubjects: []authorizationv1alpha1.SubjectMatcher{
				{
					UserRestriction: &authorizationv1.UserRestriction{
						Users: []string{"nancy"},
					},
				},
			},
			DeniedSubjects: []authorizationv1alpha1.SubjectMatcher{
				{
					GroupRestriction: &authorizationv1.GroupRestriction{
						Groups: []string{"system:authenticated", "system:unauthenticated"},
					},
				},
			},
		},
	}
	identityWhitelistSA := &authorizationv1alpha1.AccessRestriction{
		Spec: authorizationv1alpha1.AccessRestrictionSpec{
			MatchAttributes: []rbacv1.PolicyRule{
				{
					Verbs:     []string{"update"},
					APIGroups: []string{"user.openshift.io"},
					Resources: []string{"identities"},
				},
			},
			AllowedSubjects: []authorizationv1alpha1.SubjectMatcher{
				{
					UserRestriction: &authorizationv1.UserRestriction{
						Users:  []string{"system:serviceaccount:ns3:sa3"},
						Groups: []string{"system:serviceaccounts:ns4"},
						Selectors: []v1.LabelSelector{
							{
								MatchLabels: map[string]string{
									"not": "stable",
								},
							},
						},
					},
				},
			},
			DeniedSubjects: []authorizationv1alpha1.SubjectMatcher{
				{
					GroupRestriction: &authorizationv1.GroupRestriction{
						Groups: []string{"system:authenticated", "system:unauthenticated", "system:serviceaccounts"},
					},
				},
			},
		},
	}
	labeledUserEric := &userv1.User{
		ObjectMeta: v1.ObjectMeta{
			Name: "eric",
			Labels: map[string]string{
				"not": "stable",
			},
		},
	}
	groupedLabeledUserRandy := &userv1.User{
		ObjectMeta: v1.ObjectMeta{
			Name: "randy",
			Labels: map[string]string{
				"not": "stable",
			},
		},
		Groups: []string{"sharks"},
	}
	saBlacklistUser := &authorizationv1alpha1.AccessRestriction{
		Spec: authorizationv1alpha1.AccessRestrictionSpec{
			MatchAttributes: []rbacv1.PolicyRule{
				{
					Verbs:     []string{"delete"},
					APIGroups: []string{""},
					Resources: []string{"serviceaccounts"},
				},
			},
			DeniedSubjects: []authorizationv1alpha1.SubjectMatcher{
				{
					UserRestriction: &authorizationv1.UserRestriction{
						Users:  []string{"gopher"},
						Groups: []string{"pythons"},
						Selectors: []v1.LabelSelector{
							{
								MatchLabels: map[string]string{
									"pandas": "rock",
								},
							},
						},
					},
				},
			},
		},
	}
	groupedLabeledUserFrank := &userv1.User{
		ObjectMeta: v1.ObjectMeta{
			Name: "frank",
			Labels: map[string]string{
				"pandas": "rock",
			},
		},
		Groups: []string{"danger-zone"},
	}
	requiresBothUserAndGroup1 := &authorizationv1alpha1.AccessRestriction{
		Spec: authorizationv1alpha1.AccessRestrictionSpec{
			MatchAttributes: []rbacv1.PolicyRule{
				{
					Verbs:     []string{"update"},
					APIGroups: []string{""},
					Resources: []string{"daemonsets"},
				},
			},
			AllowedSubjects: []authorizationv1alpha1.SubjectMatcher{
				{
					UserRestriction: &authorizationv1.UserRestriction{
						Users: []string{"user1"},
					},
				},
			},
			DeniedSubjects: []authorizationv1alpha1.SubjectMatcher{
				{
					GroupRestriction: &authorizationv1.GroupRestriction{
						Groups: []string{"system:authenticated", "system:unauthenticated"},
					},
				},
			},
		},
	}
	requiresBothUserAndGroup2 := &authorizationv1alpha1.AccessRestriction{
		Spec: authorizationv1alpha1.AccessRestrictionSpec{
			MatchAttributes: []rbacv1.PolicyRule{
				{
					Verbs:     []string{"update"},
					APIGroups: []string{""},
					Resources: []string{"daemonsets"},
				},
			},
			AllowedSubjects: []authorizationv1alpha1.SubjectMatcher{
				{
					GroupRestriction: &authorizationv1.GroupRestriction{
						Groups: []string{"group1"},
					},
				},
			},
			DeniedSubjects: []authorizationv1alpha1.SubjectMatcher{
				{
					GroupRestriction: &authorizationv1.GroupRestriction{
						Groups: []string{"system:authenticated", "system:unauthenticated"},
					},
				},
			},
		},
	}

	type fields struct {
		accessRestrictionLister authorizationlisters.AccessRestrictionLister
		userLister              userlisters.UserLister
		groupLister             userlisters.GroupLister
	}
	type args struct {
		requestAttributes authorizer.Attributes
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    authorizer.Decision
		want1   string
		wantErr bool
	}{
		{
			name: "access restriction list error",
			fields: fields{
				accessRestrictionLister: testAccessRestrictionLister(
					1, // invalid data
				),
			},
			args: args{
				requestAttributes: &authorizer.AttributesRecord{Namespace: "non-empty", ResourceRequest: true},
			},
			want:    authorizer.DecisionDeny,
			want1:   "cannot determine access restrictions",
			wantErr: true,
		},
		{
			name: "simple whitelist deny",
			fields: fields{
				accessRestrictionLister: testAccessRestrictionLister(
					podWhitelistGroup,
					secretWhitelistGroup, // not important for this test, just there to make sure it is ignored
				),
			},
			args: args{
				requestAttributes: &authorizer.AttributesRecord{
					User: &user.DefaultInfo{
						Name:   "bob",
						Groups: []string{"system:authenticated"},
					},
					Verb:            "get",
					Namespace:       "non-empty",
					APIGroup:        "",
					Resource:        "pods",
					Subresource:     "",
					Name:            "mysql",
					ResourceRequest: true,
					Path:            "",
				},
			},
			want:    authorizer.DecisionDeny,
			want1:   "denied by access restriction",
			wantErr: false,
		},
		{
			name: "simple whitelist not deny",
			fields: fields{
				accessRestrictionLister: testAccessRestrictionLister(
					podWhitelistGroup,
					secretWhitelistGroup, // not important for this test, just there to make sure it is ignored
				),
			},
			args: args{
				requestAttributes: &authorizer.AttributesRecord{
					User: &user.DefaultInfo{
						Name:   "bob",
						Groups: []string{"admins", "system:authenticated"},
					},
					Verb:            "get",
					Namespace:       "non-empty",
					APIGroup:        "",
					Resource:        "pods",
					Subresource:     "",
					Name:            "mysql",
					ResourceRequest: true,
					Path:            "",
				},
			},
			want:    authorizer.DecisionNoOpinion,
			want1:   "",
			wantErr: false,
		},
		{
			name: "simple whitelist deny not match",
			fields: fields{
				accessRestrictionLister: testAccessRestrictionLister(
					podWhitelistGroup,
					secretWhitelistGroup, // not important for this test, just there to make sure it is ignored
				),
			},
			args: args{
				requestAttributes: &authorizer.AttributesRecord{
					User: &user.DefaultInfo{
						Name:   "bob",
						Groups: []string{"system:authenticated"},
					},
					Verb:            "get",
					Namespace:       "non-empty",
					APIGroup:        "",
					Resource:        "node",
					Subresource:     "",
					Name:            "foo",
					ResourceRequest: true,
					Path:            "",
				},
			},
			want:    authorizer.DecisionNoOpinion,
			want1:   "",
			wantErr: false,
		},
		{
			name: "whitelist group label deny",
			fields: fields{
				accessRestrictionLister: testAccessRestrictionLister(
					secretWhitelistGroup,
					podWhitelistGroup, // not important for this test, just there to make sure it is ignored
				),
				groupLister: testGroupLister(),
			},
			args: args{
				requestAttributes: &authorizer.AttributesRecord{
					User: &user.DefaultInfo{
						Name:   "bob",
						Groups: []string{"system:authenticated"},
					},
					Verb:            "get",
					Namespace:       "non-empty",
					APIGroup:        "",
					Resource:        "secrets",
					Subresource:     "",
					Name:            "ssh",
					ResourceRequest: true,
					Path:            "",
				},
			},
			want:    authorizer.DecisionDeny,
			want1:   "denied by access restriction",
			wantErr: false,
		},
		{
			name: "whitelist group label not deny group object only",
			fields: fields{
				accessRestrictionLister: testAccessRestrictionLister(
					secretWhitelistGroup,
					podWhitelistGroup, // not important for this test, just there to make sure it is ignored
				),
				groupLister: testGroupLister(
					secretLabelGroup,
				),
			},
			args: args{
				requestAttributes: &authorizer.AttributesRecord{
					User: &user.DefaultInfo{
						Name:   "bob",
						Groups: []string{"system:authenticated"}, // works when only the group object has the user
					},
					Verb:            "get",
					Namespace:       "non-empty",
					APIGroup:        "",
					Resource:        "secrets",
					Subresource:     "",
					Name:            "ssh",
					ResourceRequest: true,
					Path:            "",
				},
			},
			want:    authorizer.DecisionNoOpinion,
			want1:   "",
			wantErr: false,
		},
		{
			name: "whitelist group label not deny virtual user group only",
			fields: fields{
				accessRestrictionLister: testAccessRestrictionLister(
					secretWhitelistGroup,
					podWhitelistGroup, // not important for this test, just there to make sure it is ignored
				),
				groupLister: testGroupLister(
					secretLabelGroupNoUsers,
				),
			},
			args: args{
				requestAttributes: &authorizer.AttributesRecord{
					User: &user.DefaultInfo{
						Name:   "bob",
						Groups: []string{"sgroup", "system:authenticated"}, // works when only the virtual user has the group
					},
					Verb:            "get",
					Namespace:       "non-empty",
					APIGroup:        "",
					Resource:        "secrets",
					Subresource:     "",
					Name:            "ssh",
					ResourceRequest: true,
					Path:            "",
				},
			},
			want:    authorizer.DecisionNoOpinion,
			want1:   "",
			wantErr: false,
		},
		{
			name: "whitelist user deny",
			fields: fields{
				accessRestrictionLister: testAccessRestrictionLister(
					configmapWhitelistUser,
					// the rest are not important for this test, just there to make sure it is ignored
					secretWhitelistGroup,
					podWhitelistGroup,
				),
				groupLister: testGroupLister(
					secretLabelGroupNoUsers, // not important for this test, just there to make sure it is ignored
				),
			},
			args: args{
				requestAttributes: &authorizer.AttributesRecord{
					User: &user.DefaultInfo{
						Name:   "bob",
						Groups: []string{"system:authenticated"},
					},
					Verb:            "list",
					Namespace:       "non-empty",
					APIGroup:        "",
					Resource:        "configmaps",
					Subresource:     "",
					Name:            "console",
					ResourceRequest: true,
					Path:            "",
				},
			},
			want:    authorizer.DecisionDeny,
			want1:   "denied by access restriction",
			wantErr: false,
		},
		{
			name: "whitelist user not deny",
			fields: fields{
				accessRestrictionLister: testAccessRestrictionLister(
					configmapWhitelistUser,
					// the rest are not important for this test, just there to make sure it is ignored
					secretWhitelistGroup,
					podWhitelistGroup,
				),
				groupLister: testGroupLister(
					secretLabelGroupNoUsers, // not important for this test, just there to make sure it is ignored
				),
			},
			args: args{
				requestAttributes: &authorizer.AttributesRecord{
					User: &user.DefaultInfo{
						Name:   "nancy",
						Groups: []string{"system:authenticated"},
					},
					Verb:            "list",
					Namespace:       "non-empty",
					APIGroup:        "",
					Resource:        "configmaps",
					Subresource:     "",
					Name:            "console",
					ResourceRequest: true,
					Path:            "",
				},
			},
			want:    authorizer.DecisionNoOpinion,
			want1:   "",
			wantErr: false,
		},
		{
			name: "whitelist not deny SA global group",
			fields: fields{
				accessRestrictionLister: testAccessRestrictionLister(
					podWhitelistGroup,
					// the rest are not important for this test, just there to make sure it is ignored
					configmapWhitelistUser,
					secretWhitelistGroup,
				),
				groupLister: testGroupLister(
					secretLabelGroupNoUsers, // not important for this test, just there to make sure it is ignored
				),
			},
			args: args{
				requestAttributes: &authorizer.AttributesRecord{
					User:            serviceaccount.UserInfo("ns1", "sa1", "007"),
					Verb:            "get",
					Namespace:       "non-empty",
					APIGroup:        "",
					Resource:        "pods",
					Subresource:     "",
					Name:            "api",
					ResourceRequest: true,
					Path:            "",
				},
			},
			want:    authorizer.DecisionNoOpinion,
			want1:   "",
			wantErr: false,
		},
		{
			name: "whitelist not deny SA ns group",
			fields: fields{
				accessRestrictionLister: testAccessRestrictionLister(
					secretWhitelistGroup,
					// the rest are not important for this test, just there to make sure it is ignored
					configmapWhitelistUser,
					podWhitelistGroup,
				),
				groupLister: testGroupLister(
					secretLabelGroupNoUsers, // not important for this test, just there to make sure it is ignored
				),
			},
			args: args{
				requestAttributes: &authorizer.AttributesRecord{
					User:            serviceaccount.UserInfo("ns2", "sa2", "008"),
					Verb:            "get",
					Namespace:       "non-empty",
					APIGroup:        "",
					Resource:        "secrets",
					Subresource:     "",
					Name:            "dbpass",
					ResourceRequest: true,
					Path:            "",
				},
			},
			want:    authorizer.DecisionNoOpinion,
			want1:   "",
			wantErr: false,
		},
		{
			name: "whitelist not deny SA user",
			fields: fields{
				accessRestrictionLister: testAccessRestrictionLister(
					identityWhitelistSA,
					// the rest are not important for this test, just there to make sure it is ignored
					secretWhitelistGroup,
					configmapWhitelistUser,
					podWhitelistGroup,
				),
				groupLister: testGroupLister(
					secretLabelGroupNoUsers, // not important for this test, just there to make sure it is ignored
				),
			},
			args: args{
				requestAttributes: &authorizer.AttributesRecord{
					User:            serviceaccount.UserInfo("ns3", "sa3", "009"),
					Verb:            "update",
					Namespace:       "non-empty",
					APIGroup:        "user.openshift.io",
					Resource:        "identities",
					Subresource:     "",
					Name:            "github:bob",
					ResourceRequest: true,
					Path:            "",
				},
			},
			want:    authorizer.DecisionNoOpinion,
			want1:   "",
			wantErr: false,
		},
		{
			name: "whitelist deny SA user, correct namespace with incorrect name",
			fields: fields{
				accessRestrictionLister: testAccessRestrictionLister(
					identityWhitelistSA,
					// the rest are not important for this test, just there to make sure it is ignored
					secretWhitelistGroup,
					configmapWhitelistUser,
					podWhitelistGroup,
				),
				userLister: testUserLister(),
				groupLister: testGroupLister(
					secretLabelGroupNoUsers, // not important for this test, just there to make sure it is ignored
				),
			},
			args: args{
				requestAttributes: &authorizer.AttributesRecord{
					User:            serviceaccount.UserInfo("ns3", "sa3.1", "009.1"),
					Verb:            "update",
					Namespace:       "non-empty",
					APIGroup:        "user.openshift.io",
					Resource:        "identities",
					Subresource:     "",
					Name:            "github:adam",
					ResourceRequest: true,
					Path:            "",
				},
			},
			want:    authorizer.DecisionDeny,
			want1:   "denied by access restriction",
			wantErr: false,
		},
		{
			name: "whitelist not deny SA user via group",
			fields: fields{
				accessRestrictionLister: testAccessRestrictionLister(
					identityWhitelistSA,
					// the rest are not important for this test, just there to make sure it is ignored
					secretWhitelistGroup,
					configmapWhitelistUser,
					podWhitelistGroup,
				),
				groupLister: testGroupLister(
					secretLabelGroupNoUsers, // not important for this test, just there to make sure it is ignored
				),
			},
			args: args{
				requestAttributes: &authorizer.AttributesRecord{
					User:            serviceaccount.UserInfo("ns4", "sa4", "010"),
					Verb:            "update",
					Namespace:       "non-empty",
					APIGroup:        "user.openshift.io",
					Resource:        "identities",
					Subresource:     "",
					Name:            "github:alice",
					ResourceRequest: true,
					Path:            "",
				},
			},
			want:    authorizer.DecisionNoOpinion,
			want1:   "",
			wantErr: false,
		},
		{
			name: "whitelist deny SA user via all SA group",
			fields: fields{
				accessRestrictionLister: testAccessRestrictionLister(
					identityWhitelistSA,
					// the rest are not important for this test, just there to make sure it is ignored
					secretWhitelistGroup,
					configmapWhitelistUser,
					podWhitelistGroup,
				),
				userLister: testUserLister(),
				groupLister: testGroupLister(
					secretLabelGroupNoUsers, // not important for this test, just there to make sure it is ignored
				),
			},
			args: args{
				requestAttributes: &authorizer.AttributesRecord{
					User:            serviceaccount.UserInfo("ns5", "sa5", "011"),
					Verb:            "update",
					Namespace:       "non-empty",
					APIGroup:        "user.openshift.io",
					Resource:        "identities",
					Subresource:     "",
					Name:            "github:tom",
					ResourceRequest: true,
					Path:            "",
				},
			},
			want:    authorizer.DecisionDeny,
			want1:   "denied by access restriction",
			wantErr: false,
		},
		{
			name: "whitelist not deny user via label",
			fields: fields{
				accessRestrictionLister: testAccessRestrictionLister(
					identityWhitelistSA,
					// the rest are not important for this test, just there to make sure it is ignored
					secretWhitelistGroup,
					configmapWhitelistUser,
					podWhitelistGroup,
				),
				userLister: testUserLister(
					labeledUserEric,
				),
				groupLister: testGroupLister(
					secretLabelGroupNoUsers, // not important for this test, just there to make sure it is ignored
				),
			},
			args: args{
				requestAttributes: &authorizer.AttributesRecord{
					User: &user.DefaultInfo{
						Name:   "eric",
						Groups: []string{"system:authenticated"},
					},
					Verb:            "update",
					Namespace:       "non-empty",
					APIGroup:        "user.openshift.io",
					Resource:        "identities",
					Subresource:     "",
					Name:            "github:derek",
					ResourceRequest: true,
					Path:            "",
				},
			},
			want:    authorizer.DecisionNoOpinion,
			want1:   "",
			wantErr: false,
		},
		{
			name: "whitelist not deny user via embedded group of other labeled user",
			fields: fields{
				accessRestrictionLister: testAccessRestrictionLister(
					identityWhitelistSA,
					// the rest are not important for this test, just there to make sure it is ignored
					secretWhitelistGroup,
					configmapWhitelistUser,
					podWhitelistGroup,
				),
				userLister: testUserLister(
					groupedLabeledUserRandy, // this matches the label selector for the AR and makes the group allowed
				),
				groupLister: testGroupLister(
					secretLabelGroupNoUsers, // not important for this test, just there to make sure it is ignored
				),
			},
			args: args{
				requestAttributes: &authorizer.AttributesRecord{
					User: &user.DefaultInfo{
						Name:   "some-random-name-ignored",
						Groups: []string{"sharks", "system:authenticated"}, // this is weird because it is the randy user's label matching that allows it
					},
					Verb:            "update",
					Namespace:       "non-empty",
					APIGroup:        "user.openshift.io",
					Resource:        "identities",
					Subresource:     "",
					Name:            "github:phantom",
					ResourceRequest: true,
					Path:            "",
				},
			},
			want:    authorizer.DecisionNoOpinion,
			want1:   "",
			wantErr: false,
		},
		{
			name: "simple blacklist user deny",
			fields: fields{
				accessRestrictionLister: testAccessRestrictionLister(
					saBlacklistUser,
					// the rest are not important for this test, just there to make sure it is ignored
					identityWhitelistSA,
					secretWhitelistGroup,
					configmapWhitelistUser,
					podWhitelistGroup,
				),
			},
			args: args{
				requestAttributes: &authorizer.AttributesRecord{
					User: &user.DefaultInfo{
						Name:   "gopher",
						Groups: []string{"system:authenticated"},
					},
					Verb:            "delete",
					Namespace:       "non-empty",
					APIGroup:        "",
					Resource:        "serviceaccounts",
					Subresource:     "",
					Name:            "builder",
					ResourceRequest: true,
					Path:            "",
				},
			},
			want:    authorizer.DecisionDeny,
			want1:   "denied by access restriction",
			wantErr: false,
		},
		{
			name: "simple blacklist user not deny",
			fields: fields{
				accessRestrictionLister: testAccessRestrictionLister(
					saBlacklistUser,
					// the rest are not important for this test, just there to make sure it is ignored
					identityWhitelistSA,
					secretWhitelistGroup,
					configmapWhitelistUser,
					podWhitelistGroup,
				),
				userLister: testUserLister(
					groupedLabeledUserRandy, // not important for this test, just there to make sure it is ignored
				),
				groupLister: testGroupLister(
					secretLabelGroupNoUsers, // not important for this test, just there to make sure it is ignored
				),
			},
			args: args{
				requestAttributes: &authorizer.AttributesRecord{
					User: &user.DefaultInfo{
						Name:   "not-gopher",
						Groups: []string{"system:authenticated"},
					},
					Verb:            "delete",
					Namespace:       "non-empty",
					APIGroup:        "",
					Resource:        "serviceaccounts",
					Subresource:     "",
					Name:            "builder",
					ResourceRequest: true,
					Path:            "",
				},
			},
			want:    authorizer.DecisionNoOpinion,
			want1:   "",
			wantErr: false,
		},
		{
			name: "simple blacklist group deny",
			fields: fields{
				accessRestrictionLister: testAccessRestrictionLister(
					saBlacklistUser,
					// the rest are not important for this test, just there to make sure it is ignored
					identityWhitelistSA,
					secretWhitelistGroup,
					configmapWhitelistUser,
					podWhitelistGroup,
				),
				userLister: testUserLister(
					groupedLabeledUserRandy, // not important for this test, just there to make sure it is ignored
				),
				groupLister: testGroupLister(
					secretLabelGroupNoUsers, // not important for this test, just there to make sure it is ignored
				),
			},
			args: args{
				requestAttributes: &authorizer.AttributesRecord{
					User: &user.DefaultInfo{
						Name:   "not-gopher",
						Groups: []string{"pythons", "system:authenticated"},
					},
					Verb:            "delete",
					Namespace:       "non-empty",
					APIGroup:        "",
					Resource:        "serviceaccounts",
					Subresource:     "",
					Name:            "builder",
					ResourceRequest: true,
					Path:            "",
				},
			},
			want:    authorizer.DecisionDeny,
			want1:   "denied by access restriction",
			wantErr: false,
		},
		{
			name: "simple blacklist group not deny",
			fields: fields{
				accessRestrictionLister: testAccessRestrictionLister(
					saBlacklistUser,
					// the rest are not important for this test, just there to make sure it is ignored
					identityWhitelistSA,
					secretWhitelistGroup,
					configmapWhitelistUser,
					podWhitelistGroup,
				),
				userLister: testUserLister(
					groupedLabeledUserRandy, // not important for this test, just there to make sure it is ignored
				),
				groupLister: testGroupLister(
					secretLabelGroupNoUsers, // not important for this test, just there to make sure it is ignored
				),
			},
			args: args{
				requestAttributes: &authorizer.AttributesRecord{
					User: &user.DefaultInfo{
						Name:   "not-gopher",
						Groups: []string{"not-pythons", "system:authenticated"},
					},
					Verb:            "delete",
					Namespace:       "non-empty",
					APIGroup:        "",
					Resource:        "serviceaccounts",
					Subresource:     "",
					Name:            "builder",
					ResourceRequest: true,
					Path:            "",
				},
			},
			want:    authorizer.DecisionNoOpinion,
			want1:   "",
			wantErr: false,
		},
		{
			name: "simple blacklist label deny",
			fields: fields{
				accessRestrictionLister: testAccessRestrictionLister(
					saBlacklistUser,
					// the rest are not important for this test, just there to make sure it is ignored
					identityWhitelistSA,
					secretWhitelistGroup,
					configmapWhitelistUser,
					podWhitelistGroup,
				),
				userLister: testUserLister(
					groupedLabeledUserFrank,
					groupedLabeledUserRandy, // not important for this test, just there to make sure it is ignored
				),
				groupLister: testGroupLister(
					secretLabelGroupNoUsers, // not important for this test, just there to make sure it is ignored
				),
			},
			args: args{
				requestAttributes: &authorizer.AttributesRecord{
					User: &user.DefaultInfo{
						Name:   "frank",
						Groups: []string{"not-pythons", "system:authenticated"},
					},
					Verb:            "delete",
					Namespace:       "non-empty",
					APIGroup:        "",
					Resource:        "serviceaccounts",
					Subresource:     "",
					Name:            "builder",
					ResourceRequest: true,
					Path:            "",
				},
			},
			want:    authorizer.DecisionDeny,
			want1:   "denied by access restriction",
			wantErr: false,
		},
		{
			name: "blacklist deny user via embedded group of other labeled user",
			fields: fields{
				accessRestrictionLister: testAccessRestrictionLister(
					saBlacklistUser,
					// the rest are not important for this test, just there to make sure it is ignored
					identityWhitelistSA,
					secretWhitelistGroup,
					configmapWhitelistUser,
					podWhitelistGroup,
				),
				userLister: testUserLister(
					groupedLabeledUserFrank,
					groupedLabeledUserRandy, // not important for this test, just there to make sure it is ignored
				),
				groupLister: testGroupLister(
					secretLabelGroupNoUsers, // not important for this test, just there to make sure it is ignored
				),
			},
			args: args{
				requestAttributes: &authorizer.AttributesRecord{
					User: &user.DefaultInfo{
						Name:   "not-used",
						Groups: []string{"danger-zone", "system:authenticated"}, // this is weird because it is the frank user's label matching that denies it
					},
					Verb:            "delete",
					Namespace:       "non-empty",
					APIGroup:        "",
					Resource:        "serviceaccounts",
					Subresource:     "",
					Name:            "builder",
					ResourceRequest: true,
					Path:            "",
				},
			},
			want:    authorizer.DecisionDeny,
			want1:   "denied by access restriction",
			wantErr: false,
		},
		{
			name: "whitelist deny requires both user and group, only user given",
			fields: fields{
				accessRestrictionLister: testAccessRestrictionLister(
					requiresBothUserAndGroup1,
					requiresBothUserAndGroup2,
					// the rest are not important for this test, just there to make sure it is ignored
					saBlacklistUser,
					identityWhitelistSA,
					secretWhitelistGroup,
					configmapWhitelistUser,
					podWhitelistGroup,
				),
				userLister: testUserLister(
					// the rest are not important for this test, just there to make sure it is ignored
					groupedLabeledUserFrank,
					groupedLabeledUserRandy,
				),
				groupLister: testGroupLister(
					secretLabelGroupNoUsers, // not important for this test, just there to make sure it is ignored
				),
			},
			args: args{
				requestAttributes: &authorizer.AttributesRecord{
					User: &user.DefaultInfo{
						Name:   "user1",
						Groups: []string{"not-group1", "system:authenticated"},
					},
					Verb:            "update",
					Namespace:       "non-empty",
					APIGroup:        "",
					Resource:        "daemonsets",
					Subresource:     "",
					Name:            "proxy",
					ResourceRequest: true,
					Path:            "",
				},
			},
			want:    authorizer.DecisionDeny,
			want1:   "denied by access restriction",
			wantErr: false,
		},
		{
			name: "whitelist deny requires both user and group, only group given",
			fields: fields{
				accessRestrictionLister: testAccessRestrictionLister(
					requiresBothUserAndGroup1,
					requiresBothUserAndGroup2,
					// the rest are not important for this test, just there to make sure it is ignored
					saBlacklistUser,
					identityWhitelistSA,
					secretWhitelistGroup,
					configmapWhitelistUser,
					podWhitelistGroup,
				),
				userLister: testUserLister(
					// the rest are not important for this test, just there to make sure it is ignored
					groupedLabeledUserFrank,
					groupedLabeledUserRandy,
				),
				groupLister: testGroupLister(
					secretLabelGroupNoUsers, // not important for this test, just there to make sure it is ignored
				),
			},
			args: args{
				requestAttributes: &authorizer.AttributesRecord{
					User: &user.DefaultInfo{
						Name:   "not-user1",
						Groups: []string{"group1", "system:authenticated"},
					},
					Verb:            "update",
					Namespace:       "non-empty",
					APIGroup:        "",
					Resource:        "daemonsets",
					Subresource:     "",
					Name:            "proxy",
					ResourceRequest: true,
					Path:            "",
				},
			},
			want:    authorizer.DecisionDeny,
			want1:   "denied by access restriction",
			wantErr: false,
		},
		{
			name: "whitelist not deny requires both user and group, both user and group given",
			fields: fields{
				accessRestrictionLister: testAccessRestrictionLister(
					requiresBothUserAndGroup1,
					requiresBothUserAndGroup2,
					// the rest are not important for this test, just there to make sure it is ignored
					saBlacklistUser,
					identityWhitelistSA,
					secretWhitelistGroup,
					configmapWhitelistUser,
					podWhitelistGroup,
				),
				userLister: testUserLister(
					// the rest are not important for this test, just there to make sure it is ignored
					groupedLabeledUserFrank,
					groupedLabeledUserRandy,
				),
				groupLister: testGroupLister(
					secretLabelGroupNoUsers, // not important for this test, just there to make sure it is ignored
				),
			},
			args: args{
				requestAttributes: &authorizer.AttributesRecord{
					User: &user.DefaultInfo{
						Name:   "user1",
						Groups: []string{"group1", "system:authenticated"},
					},
					Verb:            "update",
					Namespace:       "non-empty",
					APIGroup:        "",
					Resource:        "daemonsets",
					Subresource:     "",
					Name:            "proxy",
					ResourceRequest: true,
					Path:            "",
				},
			},
			want:    authorizer.DecisionNoOpinion,
			want1:   "",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &accessRestrictionAuthorizer{
				synced:                  func() bool { return true },
				accessRestrictionLister: tt.fields.accessRestrictionLister,
				userLister:              tt.fields.userLister,
				groupLister:             tt.fields.groupLister,
			}
			got, got1, err := a.Authorize(tt.args.requestAttributes)
			if (err != nil) != tt.wantErr {
				t.Errorf("accessRestrictionAuthorizer.Authorize() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("accessRestrictionAuthorizer.Authorize() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("accessRestrictionAuthorizer.Authorize() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

type testIndexer struct {
	data          []interface{} // this destroys type safety but allows a simple way to reuse the lister logic
	cache.Indexer               // embed this so we pretend to implement the whole interface, it will panic if anything other than List is called
}

func (i *testIndexer) List() []interface{} {
	return i.data
}

func testAccessRestrictionLister(accessRestrictions ...interface{}) authorizationlisters.AccessRestrictionLister {
	return authorizationlisters.NewAccessRestrictionLister(&testIndexer{data: accessRestrictions})
}

func testUserLister(users ...interface{}) userlisters.UserLister {
	return userlisters.NewUserLister(&testIndexer{data: users})
}

func testGroupLister(groups ...interface{}) userlisters.GroupLister {
	return userlisters.NewGroupLister(&testIndexer{data: groups})
}
