package integration

import (
	"strings"
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kubernetes/pkg/apis/batch"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/apis/rbac"
	kcoreclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/core/internalversion"
	rbacinternalversion "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/rbac/internalversion"

	authorizationv1 "github.com/openshift/api/authorization/v1"
	authorizationv1alpha1 "github.com/openshift/api/authorization/v1alpha1"
	userv1 "github.com/openshift/api/user/v1"
	authorizationv1alpha1clientset "github.com/openshift/client-go/authorization/clientset/versioned/typed/authorization/v1alpha1"
	userv1clientset "github.com/openshift/client-go/user/clientset/versioned/typed/user/v1"
	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func TestAccessRestrictionEscalationCheck(t *testing.T) {
	masterConfig, clusterAdminKubeConfig := masterWithAccessRestrictionDenyAuthorizer(t)
	defer testserver.CleanupMasterEtcd(t, masterConfig)
	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatal(err)
	}
	rbacClient := rbacinternalversion.NewForConfigOrDie(clusterAdminClientConfig)
	clusterAdminAccessRestrictionClient := authorizationv1alpha1clientset.NewForConfigOrDie(clusterAdminClientConfig).AccessRestrictions()

	clusterRoleName := "almost-cluster-admin" // can do everything except URLs so still not enough for escalation check
	user := "mo"

	if _, err := rbacClient.ClusterRoles().Create(&rbac.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: clusterRoleName},
		Rules: []rbac.PolicyRule{
			rbac.NewRule(rbac.VerbAll).Groups(rbac.APIGroupAll).Resources(rbac.ResourceAll).RuleOrDie(),
		},
	}); err != nil {
		t.Fatal(err)
	}

	clusterRoleBinding := rbac.NewClusterBinding(clusterRoleName).Users(user).BindingOrDie()
	if _, err := rbacClient.ClusterRoleBindings().Create(&clusterRoleBinding); err != nil {
		t.Fatal(err)
	}

	moClient, userConfig, err := testutil.GetClientForUser(clusterAdminClientConfig, user)
	if err != nil {
		t.Fatal(err)
	}
	userAccessRestrictionClient := authorizationv1alpha1clientset.NewForConfigOrDie(userConfig).AccessRestrictions()
	moSelfSar := moClient.Authorization()

	// wait for rbac to catch up
	if err := testutil.WaitForPolicyUpdate(moSelfSar, "", "list", schema.GroupResource{Group: "authorization.openshift.io", Resource: "accessrestrictions"},
		true); err != nil {
		t.Fatalf("failed to list access restriction as user: %#v", err)
	}

	accessRestriction := &authorizationv1alpha1.AccessRestriction{
		ObjectMeta: metav1.ObjectMeta{
			Name: "does-not-matter",
		},
		Spec: authorizationv1alpha1.AccessRestrictionSpec{
			MatchAttributes: []rbacv1.PolicyRule{
				{
					Verbs:     []string{rbacv1.VerbAll},
					APIGroups: []string{rbacv1.APIGroupAll},
					Resources: []string{rbacv1.ResourceAll},
				},
			},
			DeniedSubjects: []authorizationv1alpha1.SubjectMatcher{
				{
					UserRestriction: &authorizationv1.UserRestriction{
						Users: []string{"bad-user"},
					},
				},
			},
		},
	}

	_, err = userAccessRestrictionClient.Create(accessRestriction)
	if err == nil {
		t.Fatal("expected non-nil create error for access restriction")
	}
	if !errors.IsForbidden(err) || !strings.Contains(err.Error(), "must have cluster-admin privileges to write access restrictions") {
		t.Fatalf("expected forbidden error for access restrction create: %#v", err)
	}

	if accessRestriction, err = clusterAdminAccessRestrictionClient.Create(accessRestriction); err != nil {
		t.Fatalf("unexpected error for access restrction create as system:masters: %#v", err)
	}

	// delete the permissions of all users so only system:masters can do anything
	if err := rbacClient.ClusterRoleBindings().DeleteCollection(nil, metav1.ListOptions{}); err != nil {
		t.Fatal(err)
	}

	// wait for rbac to catch up
	if err := testutil.WaitForPolicyUpdate(moSelfSar, "", "list", schema.GroupResource{Group: "authorization.openshift.io", Resource: "accessrestrictions"},
		false); err != nil && !errors.IsForbidden(err) {
		t.Fatalf("failed to revoke right for user: %#v", err)
	}

	// make sure system:masters can still pass the escalation check even with no RBAC rules
	expectedDeniedUser := "other-user"
	accessRestriction.Spec.DeniedSubjects[0].UserRestriction.Users[0] = expectedDeniedUser
	if updatedAccessRestriction, err := clusterAdminAccessRestrictionClient.Update(accessRestriction); err != nil {
		t.Fatalf("failed to update access restriction as system:masters: %#v", err)
	} else {
		if actualDeniedUser := updatedAccessRestriction.Spec.DeniedSubjects[0].UserRestriction.Users[0]; expectedDeniedUser != actualDeniedUser {
			t.Fatalf("updated access restriction does not match, expected %s, actual %s", expectedDeniedUser, actualDeniedUser)
		}
	}
}

func TestAccessRestrictionAuthorizer(t *testing.T) {
	masterConfig, clusterAdminKubeConfig := masterWithAccessRestrictionDenyAuthorizer(t)
	defer testserver.CleanupMasterEtcd(t, masterConfig)
	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatal(err)
	}
	clusterAdminAccessRestrictionClient := authorizationv1alpha1clientset.NewForConfigOrDie(clusterAdminClientConfig).AccessRestrictions()
	clusterAdminUserAPIClient := userv1clientset.NewForConfigOrDie(clusterAdminClientConfig)
	clusterAdminUserClient := clusterAdminUserAPIClient.Users()
	clusterAdminGroupClient := clusterAdminUserAPIClient.Groups()

	jobGroup := "can-write-jobs"

	// make sure none of these restrictions intersect
	accessRestrictions := []*authorizationv1alpha1.AccessRestriction{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "whitelist-write-jobs",
			},
			Spec: authorizationv1alpha1.AccessRestrictionSpec{
				MatchAttributes: []rbacv1.PolicyRule{
					{
						Verbs:     []string{"create", "update", "patch", "delete", "deletecollection"},
						APIGroups: []string{"batch"},
						Resources: []string{"jobs"},
					},
				},
				AllowedSubjects: []authorizationv1alpha1.SubjectMatcher{
					{
						GroupRestriction: &authorizationv1.GroupRestriction{
							Groups: []string{jobGroup},
						},
					},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "blacklist-label-get-pods",
			},
			Spec: authorizationv1alpha1.AccessRestrictionSpec{
				MatchAttributes: []rbacv1.PolicyRule{
					{
						Verbs:     []string{"list"},
						APIGroups: []string{""},
						Resources: []string{"pods"},
					},
				},
				DeniedSubjects: []authorizationv1alpha1.SubjectMatcher{
					{
						UserRestriction: &authorizationv1.UserRestriction{
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
						GroupRestriction: &authorizationv1.GroupRestriction{
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
	}

	for _, accessRestriction := range accessRestrictions {
		if _, err := clusterAdminAccessRestrictionClient.Create(accessRestriction); err != nil {
			t.Fatal(err)
		}
	}

	project := "mo-project"
	user := "mo"

	moClient, _, err := testserver.CreateNewProject(clusterAdminClientConfig, project, user)
	if err != nil {
		t.Fatal(err)
	}
	moUser, err := clusterAdminUserClient.Get(user, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	moGroup := &userv1.Group{
		ObjectMeta: metav1.ObjectMeta{Name: jobGroup},
		Users:      userv1.OptionalNames{user},
	}

	moSelfSar := moClient.Authorization()
	moJobs := moClient.Batch().Jobs(project)

	{
		// read jobs works
		if _, err := moJobs.List(metav1.ListOptions{}); err != nil {
			t.Fatalf("cannot list jobs as normal user: %#v", err)
		}

		if err := testutil.WaitForPolicyUpdate(moSelfSar, project, "create", schema.GroupResource{Group: "batch", Resource: "jobs"}, false); err != nil {
			t.Fatalf("user permissions not updated to reflect access restrictions: %#v", err)
		}

		// write jobs fails
		_, err = moJobs.Create(&batch.Job{})
		checkAccessRestrictionError(t, err)
	}

	{
		// add user to write jobs group
		if moGroup, err = clusterAdminGroupClient.Create(moGroup); err != nil {
			t.Fatal(err)
		}

		if err := testutil.WaitForPolicyUpdate(moSelfSar, project, "create", schema.GroupResource{Group: "batch", Resource: "jobs"}, true); err != nil {
			t.Fatalf("user permissions not updated to reflect group membership: %#v", err)
		}

		validJob := &batch.Job{
			ObjectMeta: metav1.ObjectMeta{Name: "myjob"},
			Spec: batch.JobSpec{
				Template: kapi.PodTemplateSpec{
					Spec: kapi.PodSpec{
						Containers:    []kapi.Container{{Name: "mycontainer", Image: "myimage"}},
						RestartPolicy: kapi.RestartPolicyNever,
					},
				},
			},
		}

		// write jobs works after being added to correct group
		if _, err := moJobs.Create(validJob); err != nil {
			t.Fatalf("cannot write jobs as grouped user: %#v", err)
		}
	}

	{
		// list works before labeling
		moPods := moClient.Core().Pods(project)
		if _, err := moPods.List(metav1.ListOptions{}); err != nil {
			t.Fatalf("unexpected list error as unlabeled user: %#v", err)
		}

		// label user to match restriction
		moUser.Labels = map[string]string{
			"bad": "yes",
		}
		if moUser, err = clusterAdminUserClient.Update(moUser); err != nil {
			t.Fatal(err)
		}

		if err := testutil.WaitForPolicyUpdate(moSelfSar, project, "list", schema.GroupResource{Group: "", Resource: "pods"}, false); err != nil {
			t.Fatalf("user permissions not updated to reflect user label: %#v", err)
		}

		// list is forbidden after labeling
		_, err = moPods.List(metav1.ListOptions{})
		checkAccessRestrictionError(t, err)

		// impersonating client is also forbidden
		clusterAdminClientConfigCopy := *clusterAdminClientConfig
		clusterAdminClientConfigCopy.Impersonate.UserName = user
		impersonateMoPods := kcoreclient.NewForConfigOrDie(&clusterAdminClientConfigCopy).Pods(project)
		_, err = impersonateMoPods.List(metav1.ListOptions{})
		checkAccessRestrictionError(t, err)

		// remove label
		moUser.Labels = map[string]string{}
		if _, err := clusterAdminUserClient.Update(moUser); err != nil {
			t.Fatal(err)
		}

		if err := testutil.WaitForPolicyUpdate(moSelfSar, project, "list", schema.GroupResource{Group: "", Resource: "pods"}, true); err != nil {
			t.Fatalf("user permissions not updated to reflect removed user label: %#v", err)
		}

		// list works again after removing label
		if _, err := moPods.List(metav1.ListOptions{}); err != nil {
			t.Fatalf("unexpected list error as unlabeled user: %#v", err)
		}

		// label group to match restriction
		moGroup.Labels = map[string]string{
			"alsobad": "yup",
		}
		if _, err := clusterAdminGroupClient.Update(moGroup); err != nil {
			t.Fatal(err)
		}

		if err := testutil.WaitForPolicyUpdate(moSelfSar, project, "list", schema.GroupResource{Group: "", Resource: "pods"}, false); err != nil {
			t.Fatalf("user permissions not updated to reflect group label: %#v", err)
		}

		// list is forbidden after labeling
		_, err = moPods.List(metav1.ListOptions{})
		checkAccessRestrictionError(t, err)
		// impersonation list is forbidden even though we are only impersonating the user (and not the group)
		// this is because the access restriction authorizer checks both the groups on the request user.Info
		// and the members of matching groups that it fetched from its shared informer
		_, err = impersonateMoPods.List(metav1.ListOptions{})
		checkAccessRestrictionError(t, err)
	}
}

func masterWithAccessRestrictionDenyAuthorizer(t *testing.T) (*configapi.MasterConfig, string) {
	t.Helper()

	masterConfig, err := testserver.DefaultMasterOptions()
	if err != nil {
		t.Fatal(err)
	}

	args := masterConfig.KubernetesMasterConfig.APIServerArguments
	if existing, ok := args["feature-gates"]; ok {
		args["feature-gates"] = []string{existing[0] + ",AccessRestrictionDenyAuthorizer=true"}
	} else {
		args["feature-gates"] = []string{"AccessRestrictionDenyAuthorizer=true"}
	}
	masterConfig.KubernetesMasterConfig.APIServerArguments = args

	clusterAdminKubeConfig, err := testserver.StartConfiguredMasterAPI(masterConfig)
	if err != nil {
		t.Fatal(err)
	}

	return masterConfig, clusterAdminKubeConfig
}

func checkAccessRestrictionError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected non-nil error")
	}
	if !errors.IsForbidden(err) || !strings.Contains(err.Error(), "denied by access restriction") {
		t.Fatalf("expected forbidden error: %#v", err)
	}
}
