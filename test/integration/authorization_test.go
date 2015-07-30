// +build integration,!no-etcd

package integration

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	kapierror "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/wait"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/client"
	policy "github.com/openshift/origin/pkg/cmd/admin/policy"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	testutil "github.com/openshift/origin/test/util"
)

func TestAuthorizationRestrictedAccessForProjectAdmins(t *testing.T) {
	_, clusterAdminKubeConfig, err := testutil.StartTestMaster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clusterAdminClient, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	haroldClient, err := testutil.CreateNewProject(clusterAdminClient, *clusterAdminClientConfig, "hammer-project", "harold")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	markClient, err := testutil.CreateNewProject(clusterAdminClient, *clusterAdminClientConfig, "mallet-project", "mark")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = haroldClient.DeploymentConfigs("hammer-project").List(labels.Everything(), fields.Everything())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = markClient.DeploymentConfigs("hammer-project").List(labels.Everything(), fields.Everything())
	if (err == nil) || !kapierror.IsForbidden(err) {
		t.Fatalf("unexpected error: %v", err)
	}

	// projects are a special case where a get of a project actually sets a namespace.  Make sure that
	// the namespace is properly special cased and set for authorization rules
	_, err = haroldClient.Projects().Get("hammer-project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err = markClient.Projects().Get("hammer-project")
	if (err == nil) || !kapierror.IsForbidden(err) {
		t.Fatalf("unexpected error: %v", err)
	}

	// wait for the project authorization cache to catch the change.  It is on a one second period
	waitForProject(t, haroldClient, "hammer-project", 1*time.Second, 10)
	waitForProject(t, markClient, "mallet-project", 1*time.Second, 10)
}

// waitForProject will execute a client list of projects looking for the project with specified name
// if not found, it will retry up to numRetries at the specified delayInterval
func waitForProject(t *testing.T, client client.Interface, projectName string, delayInterval time.Duration, numRetries int) {
	for i := 0; i <= numRetries; i++ {
		projects, err := client.Projects().List(labels.Everything(), fields.Everything())
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if (len(projects.Items) == 1) && (projects.Items[0].Name == projectName) {
			fmt.Printf("Waited %v times with interval %v\n", i, delayInterval)
			return
		} else {
			time.Sleep(delayInterval)
		}
	}
	t.Errorf("expected project %v not found", projectName)
}

func TestAuthorizationOnlyResolveRolesForBindingsThatMatter(t *testing.T) {
	_, clusterAdminKubeConfig, err := testutil.StartTestMaster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clusterAdminClient, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	addValerie := &policy.RoleModificationOptions{
		RoleNamespace:       "",
		RoleName:            bootstrappolicy.ViewRoleName,
		RoleBindingAccessor: policy.NewClusterRoleBindingAccessor(clusterAdminClient),
		Users:               []string{"valerie"},
	}
	if err := addValerie.AddRole(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err = clusterAdminClient.ClusterRoles().Delete(bootstrappolicy.ViewRoleName); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	addEdgar := &policy.RoleModificationOptions{
		RoleNamespace:       "",
		RoleName:            bootstrappolicy.EditRoleName,
		RoleBindingAccessor: policy.NewClusterRoleBindingAccessor(clusterAdminClient),
		Users:               []string{"edgar"},
	}
	if err := addEdgar.AddRole(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// try to add Valerie to a non-existent role
	if err := addValerie.AddRole(); !kapierror.IsNotFound(err) {
		t.Fatalf("unexpected error: %v", err)
	}

}

// TODO this list should start collapsing as we continue to tighten access on generated system ids
var globalClusterAdminUsers = util.NewStringSet()
var globalClusterAdminGroups = util.NewStringSet("system:cluster-admins", "system:masters")

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

	if actualResponse.Namespace != test.response.Namespace || !reflect.DeepEqual(actualResponse.Users.List(), test.response.Users.List()) || !reflect.DeepEqual(actualResponse.Groups.List(), test.response.Groups.List()) {
		t.Errorf("%#v: expected %v, got %v", test.review, test.response, actualResponse)
	}
}

func TestAuthorizationResourceAccessReview(t *testing.T) {
	_, clusterAdminKubeConfig, err := testutil.StartTestMaster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clusterAdminClient, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	haroldClient, err := testutil.CreateNewProject(clusterAdminClient, *clusterAdminClientConfig, "hammer-project", "harold")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	markClient, err := testutil.CreateNewProject(clusterAdminClient, *clusterAdminClientConfig, "mallet-project", "mark")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	addValerie := &policy.RoleModificationOptions{
		RoleNamespace:       "",
		RoleName:            bootstrappolicy.ViewRoleName,
		RoleBindingAccessor: policy.NewLocalRoleBindingAccessor("hammer-project", haroldClient),
		Users:               []string{"valerie"},
	}
	if err := addValerie.AddRole(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	addEdgar := &policy.RoleModificationOptions{
		RoleNamespace:       "",
		RoleName:            bootstrappolicy.EditRoleName,
		RoleBindingAccessor: policy.NewLocalRoleBindingAccessor("mallet-project", markClient),
		Users:               []string{"edgar"},
	}
	if err := addEdgar.AddRole(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	requestWhoCanViewDeployments := &authorizationapi.ResourceAccessReview{Verb: "get", Resource: "deployments"}

	{
		test := resourceAccessReviewTest{
			clientInterface: haroldClient.ResourceAccessReviews("hammer-project"),
			review:          requestWhoCanViewDeployments,
			response: authorizationapi.ResourceAccessReviewResponse{
				Users:     util.NewStringSet("harold", "valerie"),
				Groups:    globalClusterAdminGroups,
				Namespace: "hammer-project",
			},
		}
		test.response.Users.Insert(globalClusterAdminUsers.List()...)
		test.response.Groups.Insert("system:cluster-readers")
		test.run(t)
	}
	{
		test := resourceAccessReviewTest{
			clientInterface: markClient.ResourceAccessReviews("mallet-project"),
			review:          requestWhoCanViewDeployments,
			response: authorizationapi.ResourceAccessReviewResponse{
				Users:     util.NewStringSet("mark", "edgar"),
				Groups:    globalClusterAdminGroups,
				Namespace: "mallet-project",
			},
		}
		test.response.Users.Insert(globalClusterAdminUsers.List()...)
		test.response.Groups.Insert("system:cluster-readers")
		test.run(t)
	}

	// mark should not be able to make global access review requests
	{
		test := resourceAccessReviewTest{
			clientInterface: markClient.ClusterResourceAccessReviews(),
			review:          requestWhoCanViewDeployments,
			err:             "cannot ",
		}
		test.run(t)
	}

	// a cluster-admin should be able to make global access review requests
	{
		test := resourceAccessReviewTest{
			clientInterface: clusterAdminClient.ClusterResourceAccessReviews(),
			review:          requestWhoCanViewDeployments,
			response: authorizationapi.ResourceAccessReviewResponse{
				Users:  globalClusterAdminUsers,
				Groups: globalClusterAdminGroups,
			},
		}
		test.response.Groups.Insert("system:cluster-readers")
		test.run(t)
	}
}

type subjectAccessReviewTest struct {
	description     string
	clientInterface client.SubjectAccessReviewInterface
	review          *authorizationapi.SubjectAccessReview

	response authorizationapi.SubjectAccessReviewResponse
	err      string
}

func (test subjectAccessReviewTest) run(t *testing.T) {
	failMessage := ""
	err := wait.Poll(testutil.PolicyCachePollInterval, testutil.PolicyCachePollTimeout, func() (bool, error) {
		actualResponse, err := test.clientInterface.Create(test.review)
		if len(test.err) > 0 {
			if err == nil {
				failMessage = fmt.Sprintf("%s: Expected error: %v", test.description, test.err)
				return false, nil
			} else if !strings.HasPrefix(err.Error(), test.err) {
				failMessage = fmt.Sprintf("%s: expected\n\t%v\ngot\n\t%v", test.description, test.err, err)
				return false, nil
			}
		} else {
			if err != nil {
				failMessage = fmt.Sprintf("%s: unexpected error: %v", test.description, err)
				return false, nil
			}
		}

		if (actualResponse.Namespace != test.response.Namespace) ||
			(actualResponse.Allowed != test.response.Allowed) ||
			(!strings.HasPrefix(actualResponse.Reason, test.response.Reason)) {
			failMessage = fmt.Sprintf("%s: from review\n\t%#v\nexpected\n\t%#v\ngot\n\t%#v", test.description, test.review, &test.response, actualResponse)
			return false, nil
		}

		failMessage = ""
		return true, nil
	})

	if err != nil {
		t.Error(err)
	}
	if len(failMessage) != 0 {
		t.Error(failMessage)
	}
}

func TestAuthorizationSubjectAccessReview(t *testing.T) {
	_, clusterAdminKubeConfig, err := testutil.StartTestMaster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clusterAdminClient, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	haroldClient, err := testutil.CreateNewProject(clusterAdminClient, *clusterAdminClientConfig, "hammer-project", "harold")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	markClient, err := testutil.CreateNewProject(clusterAdminClient, *clusterAdminClientConfig, "mallet-project", "mark")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	dannyClient, err := testutil.GetClientForUser(*clusterAdminClientConfig, "danny")
	if err != nil {
		t.Fatalf("error requesting token: %v", err)
	}

	addDanny := &policy.RoleModificationOptions{
		RoleNamespace:       "",
		RoleName:            bootstrappolicy.ViewRoleName,
		RoleBindingAccessor: policy.NewLocalRoleBindingAccessor("default", clusterAdminClient),
		Users:               []string{"danny"},
	}
	if err := addDanny.AddRole(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	askCanDannyGetProject := &authorizationapi.SubjectAccessReview{User: "danny", Verb: "get", Resource: "projects"}
	subjectAccessReviewTest{
		description:     "cluster admin told danny can get project default",
		clientInterface: clusterAdminClient.SubjectAccessReviews("default"),
		review:          askCanDannyGetProject,
		response: authorizationapi.SubjectAccessReviewResponse{
			Allowed:   true,
			Reason:    "allowed by rule in default",
			Namespace: "default",
		},
	}.run(t)
	subjectAccessReviewTest{
		description:     "cluster admin told danny cannot get projects cluster-wide",
		clientInterface: clusterAdminClient.ClusterSubjectAccessReviews(),
		review:          askCanDannyGetProject,
		response: authorizationapi.SubjectAccessReviewResponse{
			Allowed:   false,
			Reason:    `User "danny" cannot get projects at the cluster scope`,
			Namespace: "",
		},
	}.run(t)
	subjectAccessReviewTest{
		description:     "as danny, can I make cluster subject access reviews",
		clientInterface: dannyClient.ClusterSubjectAccessReviews(),
		review:          askCanDannyGetProject,
		err:             `User "danny" cannot create subjectaccessreviews at the cluster scope`,
	}.run(t)

	addValerie := &policy.RoleModificationOptions{
		RoleNamespace:       "",
		RoleName:            bootstrappolicy.ViewRoleName,
		RoleBindingAccessor: policy.NewLocalRoleBindingAccessor("hammer-project", haroldClient),
		Users:               []string{"valerie"},
	}
	if err := addValerie.AddRole(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	addEdgar := &policy.RoleModificationOptions{
		RoleNamespace:       "",
		RoleName:            bootstrappolicy.EditRoleName,
		RoleBindingAccessor: policy.NewLocalRoleBindingAccessor("mallet-project", markClient),
		Users:               []string{"edgar"},
	}
	if err := addEdgar.AddRole(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	askCanValerieGetProject := &authorizationapi.SubjectAccessReview{User: "valerie", Verb: "get", Resource: "projects"}
	subjectAccessReviewTest{
		description:     "harold told valerie can get project hammer-project",
		clientInterface: haroldClient.SubjectAccessReviews("hammer-project"),
		review:          askCanValerieGetProject,
		response: authorizationapi.SubjectAccessReviewResponse{
			Allowed:   true,
			Reason:    "allowed by rule in hammer-project",
			Namespace: "hammer-project",
		},
	}.run(t)
	subjectAccessReviewTest{
		description:     "mark told valerie cannot get project mallet-project",
		clientInterface: markClient.SubjectAccessReviews("mallet-project"),
		review:          askCanValerieGetProject,
		response: authorizationapi.SubjectAccessReviewResponse{
			Allowed:   false,
			Reason:    `User "valerie" cannot get projects in project "mallet-project"`,
			Namespace: "mallet-project",
		},
	}.run(t)

	askCanEdgarDeletePods := &authorizationapi.SubjectAccessReview{User: "edgar", Verb: "delete", Resource: "pods"}
	subjectAccessReviewTest{
		description:     "mark told edgar can delete pods in mallet-project",
		clientInterface: markClient.SubjectAccessReviews("mallet-project"),
		review:          askCanEdgarDeletePods,
		response: authorizationapi.SubjectAccessReviewResponse{
			Allowed:   true,
			Reason:    "allowed by rule in mallet-project",
			Namespace: "mallet-project",
		},
	}.run(t)
	subjectAccessReviewTest{
		description:     "harold denied ability to run subject access review in project mallet-project",
		clientInterface: haroldClient.SubjectAccessReviews("mallet-project"),
		review:          askCanEdgarDeletePods,
		err:             `User "harold" cannot create subjectaccessreviews in project "mallet-project"`,
	}.run(t)

	askCanHaroldUpdateProject := &authorizationapi.SubjectAccessReview{User: "harold", Verb: "update", Resource: "projects"}
	subjectAccessReviewTest{
		description:     "harold told harold can update project hammer-project",
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
		description:     "cluster admin told cluster admins can create projects",
		clientInterface: clusterAdminClient.ClusterSubjectAccessReviews(),
		review:          askCanClusterAdminsCreateProject,
		response: authorizationapi.SubjectAccessReviewResponse{
			Allowed:   true,
			Reason:    "allowed by cluster rule",
			Namespace: "",
		},
	}.run(t)
	subjectAccessReviewTest{
		description:     "harold denied ability to run cluster subject access review",
		clientInterface: haroldClient.ClusterSubjectAccessReviews(),
		review:          askCanClusterAdminsCreateProject,
		err:             `User "harold" cannot create subjectaccessreviews at the cluster scope`,
	}.run(t)

	askCanICreatePods := &authorizationapi.SubjectAccessReview{Verb: "create", Resource: "pods"}
	subjectAccessReviewTest{
		description:     "harold told he can create pods in project hammer-project",
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
		description:     "harold told he can create policybindings in project hammer-project",
		clientInterface: haroldClient.SubjectAccessReviews("hammer-project"),
		review:          askCanICreatePolicyBindings,
		response: authorizationapi.SubjectAccessReviewResponse{
			Allowed:   false,
			Reason:    `User "harold" cannot create policybindings in project "hammer-project"`,
			Namespace: "hammer-project",
		},
	}.run(t)

}
