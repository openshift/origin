// +build integration,!no-etcd

package integration

import (
	"reflect"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/client"
	policy "github.com/openshift/origin/pkg/cmd/experimental/policy"
)

func TestRestrictedAccessForProjectAdmins(t *testing.T) {
	startConfig, err := StartTestMaster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	openshiftClient, openshiftClientConfig, err := startConfig.GetOpenshiftClient()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	haroldClient, err := CreateNewProject(openshiftClient, *openshiftClientConfig, "hammer-project", "harold")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	markClient, err := CreateNewProject(openshiftClient, *openshiftClientConfig, "mallet-project", "mark")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	_, err = haroldClient.Deployments("hammer-project").List(labels.Everything(), labels.Everything())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// TODO make kube and origin authorization failures cause a kapierror.Forbidden
	_, err = markClient.Deployments("hammer-project").List(labels.Everything(), labels.Everything())
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

	// haroldProjects, err := haroldClient.Projects().List(labels.Everything(), labels.Everything())
	// if err != nil {
	// 	t.Errorf("unexpected error: %v", err)
	// }
	// if !((len(haroldProjects.Items) == 1) && (haroldProjects.Items[0].Name == "hammer-project")) {
	// 	t.Errorf("expected hammer-project, got %#v", haroldProjects.Items)
	// }

	// markProjects, err := markClient.Projects().List(labels.Everything(), labels.Everything())
	// if err != nil {
	// 	t.Errorf("unexpected error: %v", err)
	// }
	// if !((len(markProjects.Items) == 1) && (markProjects.Items[0].Name == "mallet-project")) {
	// 	t.Errorf("expected mallet-project, got %#v", markProjects.Items)
	// }
}

// TODO this list should start collapsing as we continue to tighten access on generated system ids
var globalClusterAdminUsers = util.NewStringSet("system:kube-client", "system:openshift-client", "system:openshift-deployer")
var globalClusterAdminGroups = util.NewStringSet("system:cluster-admins")

type resourceAccessReviewTest struct {
	clientInterface client.ResourceAccessReviewInterface
	review          *authorizationapi.ResourceAccessReview

	// TODO use resource access review response once internal types use util.StringSet
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
	startConfig, err := StartTestMaster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	openshiftClient, openshiftClientConfig, err := startConfig.GetOpenshiftClient()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	haroldClient, err := CreateNewProject(openshiftClient, *openshiftClientConfig, "hammer-project", "harold")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	markClient, err := CreateNewProject(openshiftClient, *openshiftClientConfig, "mallet-project", "mark")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	addValerie := &policy.AddUserOptions{
		RoleNamespace:    "master",
		RoleName:         "view",
		BindingNamespace: "hammer-project",
		Client:           haroldClient,
		Users:            []string{"anypassword:valerie"},
	}
	if err := addValerie.Run(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	addEdgar := &policy.AddUserOptions{
		RoleNamespace:    "master",
		RoleName:         "edit",
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
			clientInterface: openshiftClient.RootResourceAccessReviews(),
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

// TODO add test for "this user" once the subject access review supports it
func TestSubjectAccessReview(t *testing.T) {
	startConfig, err := StartTestMaster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	openshiftClient, openshiftClientConfig, err := startConfig.GetOpenshiftClient()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	haroldClient, err := CreateNewProject(openshiftClient, *openshiftClientConfig, "hammer-project", "harold")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	markClient, err := CreateNewProject(openshiftClient, *openshiftClientConfig, "mallet-project", "mark")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	addValerie := &policy.AddUserOptions{
		RoleNamespace:    "master",
		RoleName:         "view",
		BindingNamespace: "hammer-project",
		Client:           haroldClient,
		Users:            []string{"anypassword:valerie"},
	}
	if err := addValerie.Run(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	addEdgar := &policy.AddUserOptions{
		RoleNamespace:    "master",
		RoleName:         "edit",
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
		clientInterface: openshiftClient.RootSubjectAccessReviews(),
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
}
