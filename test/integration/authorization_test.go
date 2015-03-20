// +build integration,!no-etcd

package integration

import (
	"reflect"
	"strings"
	"testing"

	kapierror "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	testutil "github.com/openshift/origin/test/util"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/client"
	policy "github.com/openshift/origin/pkg/cmd/experimental/policy"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
)

func TestRestrictedAccessForProjectAdmins(t *testing.T) {
	_, clusterAdminKubeConfig, err := testutil.StartTestMaster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clusterAdminClient, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	haroldClient, err := testutil.CreateNewProject(clusterAdminClient, *clusterAdminClientConfig, "hammer-project", "harold")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	markClient, err := testutil.CreateNewProject(clusterAdminClient, *clusterAdminClientConfig, "mallet-project", "mark")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	_, err = haroldClient.Deployments("hammer-project").List(labels.Everything(), fields.Everything())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// TODO make kube and origin authorization failures cause a kapierror.Forbidden
	_, err = markClient.Deployments("hammer-project").List(labels.Everything(), fields.Everything())
	if (err == nil) || (!strings.Contains(err.Error(), "Forbidden")) {
		t.Errorf("expected forbidden error, but didn't get one")
	}

	// projects are a special case where a get of a project actually sets a namespace.  Make sure that
	// the namespace is properly special cased and set for authorization rules
	_, err = haroldClient.Projects().Get("hammer-project")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	// TODO make kube and origin authorization failures cause a kapierror.Forbidden
	_, err = markClient.Projects().Get("hammer-project")
	if (err == nil) || (!strings.Contains(err.Error(), "Forbidden")) {
		t.Errorf("expected forbidden error, but didn't get one")
	}

	// TODO restore this once we have detection for whether the cache is up to date.
	// wait for the project authorization cache to catch the change.  It is on a one second period
	// time.Sleep(5 * time.Second)

	// haroldProjects, err := haroldClient.Projects().List(labels.Everything(), fields.Everything())
	// if err != nil {
	// 	t.Errorf("unexpected error: %v", err)
	// }
	// if !((len(haroldProjects.Items) == 1) && (haroldProjects.Items[0].Name == "hammer-project")) {
	// 	t.Errorf("expected hammer-project, got %#v", haroldProjects.Items)
	// }

	// markProjects, err := markClient.Projects().List(labels.Everything(), fields.Everything())
	// if err != nil {
	// 	t.Errorf("unexpected error: %v", err)
	// }
	// if !((len(markProjects.Items) == 1) && (markProjects.Items[0].Name == "mallet-project")) {
	// 	t.Errorf("expected mallet-project, got %#v", markProjects.Items)
	// }
}

func TestOnlyResolveRolesForBindingsThatMatter(t *testing.T) {
	_, clusterAdminKubeConfig, err := testutil.StartTestMaster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clusterAdminClient, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	addValerie := &policy.AddUserOptions{
		RoleNamespace:    bootstrappolicy.DefaultMasterAuthorizationNamespace,
		RoleName:         bootstrappolicy.ViewRoleName,
		BindingNamespace: bootstrappolicy.DefaultMasterAuthorizationNamespace,
		Client:           clusterAdminClient,
		Users:            []string{"anypassword:valerie"},
	}
	if err := addValerie.Run(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if err = clusterAdminClient.Roles(bootstrappolicy.DefaultMasterAuthorizationNamespace).Delete(bootstrappolicy.ViewRoleName); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	addEdgar := &policy.AddUserOptions{
		RoleNamespace:    bootstrappolicy.DefaultMasterAuthorizationNamespace,
		RoleName:         bootstrappolicy.EditRoleName,
		BindingNamespace: bootstrappolicy.DefaultMasterAuthorizationNamespace,
		Client:           clusterAdminClient,
		Users:            []string{"anypassword:edgar"},
	}
	if err := addEdgar.Run(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// try to add Valerie to a non-existent role
	if err := addValerie.Run(); err == nil || !kapierror.IsNotFound(err) {
		t.Errorf("unexpected error %v", err)
	}

}

// TODO this list should start collapsing as we continue to tighten access on generated system ids
var globalClusterAdminUsers = util.NewStringSet("system:kube-client", "system:openshift-client", "system:openshift-deployer")
var globalClusterAdminGroups = util.NewStringSet("system:cluster-admins", "system:nodes")

type resourceAccessReviewTest struct {
	clientInterface client.ResourceAccessReviewInterface
	review          *authorizationapi.ResourceAccessReview

	response authorizationapi.ResourceAccessReviewResponse
	err      string
}

func (test resourceAccessReviewTest) run(t *testing.T) {
	actualResponse, err := test.clientInterface.Create(test.review)
	if len(test.err) > 0 {
		if err == nil {
			t.Errorf("Expected error: %v", test.err)
		} else if !strings.Contains(err.Error(), test.err) {
			t.Errorf("expected %v, got %v", test.err, err)
		}
	} else {
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}

	if reflect.DeepEqual(actualResponse, test.response) {
		t.Errorf("%#v: expected %v, got %v", test.review, test.response, actualResponse)
	}
}

func TestResourceAccessReview(t *testing.T) {
	_, clusterAdminKubeConfig, err := testutil.StartTestMaster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clusterAdminClient, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	haroldClient, err := testutil.CreateNewProject(clusterAdminClient, *clusterAdminClientConfig, "hammer-project", "harold")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	markClient, err := testutil.CreateNewProject(clusterAdminClient, *clusterAdminClientConfig, "mallet-project", "mark")

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	addValerie := &policy.AddUserOptions{
		RoleNamespace:    bootstrappolicy.DefaultMasterAuthorizationNamespace,
		RoleName:         bootstrappolicy.ViewRoleName,
		BindingNamespace: "hammer-project",
		Client:           haroldClient,
		Users:            []string{"anypassword:valerie"},
	}
	if err := addValerie.Run(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	addEdgar := &policy.AddUserOptions{
		RoleNamespace:    bootstrappolicy.DefaultMasterAuthorizationNamespace,
		RoleName:         bootstrappolicy.EditRoleName,
		BindingNamespace: "mallet-project",
		Client:           markClient,
		Users:            []string{"anypassword:edgar"},
	}
	if err := addEdgar.Run(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	requestWhoCanViewDeployments := &authorizationapi.ResourceAccessReview{Verb: "get", Resource: "deployments"}

	{
		test := resourceAccessReviewTest{
			clientInterface: haroldClient.ResourceAccessReviews("hammer-project"),
			review:          requestWhoCanViewDeployments,
			response: authorizationapi.ResourceAccessReviewResponse{
				Users:     util.NewStringSet("anypassword:harold", "anypassword:valerie"),
				Groups:    globalClusterAdminGroups,
				Namespace: "hammer-project",
			},
		}
		test.response.Users.Insert(globalClusterAdminUsers.List()...)
		test.run(t)
	}
	{
		test := resourceAccessReviewTest{
			clientInterface: markClient.ResourceAccessReviews("mallet-project"),
			review:          requestWhoCanViewDeployments,
			response: authorizationapi.ResourceAccessReviewResponse{
				Users:     util.NewStringSet("anypassword:mark", "anypassword:edgar"),
				Groups:    globalClusterAdminGroups,
				Namespace: "mallet-project",
			},
		}
		test.response.Users.Insert(globalClusterAdminUsers.List()...)
		test.run(t)
	}

	// mark should not be able to make global access review requests
	{
		test := resourceAccessReviewTest{
			clientInterface: markClient.RootResourceAccessReviews(),
			review:          requestWhoCanViewDeployments,
			err:             "Forbidden",
		}
		test.run(t)
	}

	// a cluster-admin should be able to make global access review requests
	{
		test := resourceAccessReviewTest{
			clientInterface: clusterAdminClient.RootResourceAccessReviews(),
			review:          requestWhoCanViewDeployments,
			response: authorizationapi.ResourceAccessReviewResponse{
				Users:  globalClusterAdminUsers,
				Groups: globalClusterAdminGroups,
			},
		}
		test.run(t)
	}
}

type subjectAccessReviewTest struct {
	clientInterface client.SubjectAccessReviewInterface
	review          *authorizationapi.SubjectAccessReview

	response authorizationapi.SubjectAccessReviewResponse
	err      string
}

func (test subjectAccessReviewTest) run(t *testing.T) {
	actualResponse, err := test.clientInterface.Create(test.review)
	if len(test.err) > 0 {
		if err == nil {
			t.Errorf("Expected error: %v", test.err)
		} else if !strings.Contains(err.Error(), test.err) {
			t.Errorf("expected %v, got %v", test.err, err)
		}
	} else {
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}

	if reflect.DeepEqual(actualResponse, test.response) {
		t.Errorf("%#v: expected %v, got %v", test.review, test.response, actualResponse)
	}
}

func TestSubjectAccessReview(t *testing.T) {
	_, clusterAdminKubeConfig, err := testutil.StartTestMaster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clusterAdminClient, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	haroldClient, err := testutil.CreateNewProject(clusterAdminClient, *clusterAdminClientConfig, "hammer-project", "harold")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	markClient, err := testutil.CreateNewProject(clusterAdminClient, *clusterAdminClientConfig, "mallet-project", "mark")

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	addValerie := &policy.AddUserOptions{
		RoleNamespace:    bootstrappolicy.DefaultMasterAuthorizationNamespace,
		RoleName:         bootstrappolicy.ViewRoleName,
		BindingNamespace: "hammer-project",
		Client:           haroldClient,
		Users:            []string{"anypassword:valerie"},
	}
	if err := addValerie.Run(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	addEdgar := &policy.AddUserOptions{
		RoleNamespace:    bootstrappolicy.DefaultMasterAuthorizationNamespace,
		RoleName:         bootstrappolicy.EditRoleName,
		BindingNamespace: "mallet-project",
		Client:           markClient,
		Users:            []string{"anypassword:edgar"},
	}
	if err := addEdgar.Run(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	askCanValerieGetProject := &authorizationapi.SubjectAccessReview{User: "anypassword:valerie", Verb: "get", Resource: "projects"}
	subjectAccessReviewTest{
		clientInterface: haroldClient.SubjectAccessReviews("hammer-project"),
		review:          askCanValerieGetProject,
		response: authorizationapi.SubjectAccessReviewResponse{
			Allowed:   true,
			Reason:    "allowed by rule in hammer-project",
			Namespace: "hammer-project",
		},
	}.run(t)
	subjectAccessReviewTest{
		clientInterface: markClient.SubjectAccessReviews("mallet-project"),
		review:          askCanValerieGetProject,
		response: authorizationapi.SubjectAccessReviewResponse{
			Allowed:   false,
			Reason:    "denied by default",
			Namespace: "mallet-project",
		},
	}.run(t)

	askCanEdgarDeletePods := &authorizationapi.SubjectAccessReview{User: "anypassword:edgar", Verb: "delete", Resource: "pods"}
	subjectAccessReviewTest{
		clientInterface: markClient.SubjectAccessReviews("mallet-project"),
		review:          askCanEdgarDeletePods,
		response: authorizationapi.SubjectAccessReviewResponse{
			Allowed:   true,
			Reason:    "allowed by rule in mallet-project",
			Namespace: "mallet-project",
		},
	}.run(t)
	subjectAccessReviewTest{
		clientInterface: haroldClient.SubjectAccessReviews("mallet-project"),
		review:          askCanEdgarDeletePods,
		err:             "Forbidden",
	}.run(t)

	askCanHaroldUpdateProject := &authorizationapi.SubjectAccessReview{User: "anypassword:harold", Verb: "update", Resource: "projects"}
	subjectAccessReviewTest{
		clientInterface: haroldClient.SubjectAccessReviews("hammer-project"),
		review:          askCanHaroldUpdateProject,
		response: authorizationapi.SubjectAccessReviewResponse{
			Allowed:   true,
			Reason:    "allowed by rule in hammer-project",
			Namespace: "hammer-project",
		},
	}.run(t)

	askCanClusterAdminsCreateProject := &authorizationapi.SubjectAccessReview{Groups: util.NewStringSet("system:cluster-admins"), Verb: "create", Resource: "projects"}
	subjectAccessReviewTest{
		clientInterface: clusterAdminClient.RootSubjectAccessReviews(),
		review:          askCanClusterAdminsCreateProject,
		response: authorizationapi.SubjectAccessReviewResponse{
			Allowed:   true,
			Reason:    "",
			Namespace: "",
		},
	}.run(t)
	subjectAccessReviewTest{
		clientInterface: haroldClient.RootSubjectAccessReviews(),
		review:          askCanClusterAdminsCreateProject,
		err:             "Forbidden",
	}.run(t)

	askCanICreatePods := &authorizationapi.SubjectAccessReview{Verb: "create", Resource: "projects"}
	subjectAccessReviewTest{
		clientInterface: haroldClient.SubjectAccessReviews("hammer-project"),
		review:          askCanICreatePods,
		response: authorizationapi.SubjectAccessReviewResponse{
			Allowed:   true,
			Reason:    "allowed by rule in hammer-project",
			Namespace: "hammer-project",
		},
	}.run(t)
	askCanICreatePolicyBindings := &authorizationapi.SubjectAccessReview{Verb: "create", Resource: "policybindings"}
	subjectAccessReviewTest{
		clientInterface: haroldClient.SubjectAccessReviews("hammer-project"),
		review:          askCanICreatePolicyBindings,
		response: authorizationapi.SubjectAccessReviewResponse{
			Allowed:   false,
			Reason:    "denied by default",
			Namespace: "hammer-project",
		},
	}.run(t)

}
