package integration

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	kapierror "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/unversioned"
	kunvapi "k8s.io/kubernetes/pkg/api/unversioned"
	extensionsapi "k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/util/sets"
	"k8s.io/kubernetes/pkg/util/wait"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	buildapi "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/client"
	policy "github.com/openshift/origin/pkg/cmd/admin/policy"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
	oauthapi "github.com/openshift/origin/pkg/oauth/api"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func prettyPrintAction(act *authorizationapi.Action, defaultNamespaceStr string) string {
	nsStr := fmt.Sprintf("in namespace %q", act.Namespace)
	if act.Namespace == "" {
		nsStr = defaultNamespaceStr
	}

	var resourceStr string
	if act.Group == "" && act.Version == "" {
		resourceStr = act.Resource
	} else {
		groupVer := kunvapi.GroupVersion{Group: act.Group, Version: act.Version}
		resourceStr = fmt.Sprintf("%s/%s", act.Resource, groupVer.String())
	}

	var base string
	if act.ResourceName == "" {
		base = fmt.Sprintf("who can %s %s %s", act.Verb, resourceStr, nsStr)
	} else {
		base = fmt.Sprintf("who can %s the %s named %q %s", act.Verb, resourceStr, act.ResourceName, nsStr)
	}

	if act.Content != nil {
		return fmt.Sprintf("%s with content %#v", base, act.Content)
	}

	return base
}

func prettyPrintReviewResponse(resp *authorizationapi.ResourceAccessReviewResponse) string {
	nsStr := fmt.Sprintf("(in the namespace %q)\n", resp.Namespace)
	if resp.Namespace == "" {
		nsStr = "(in all namespaces)\n"
	}

	var usersStr string
	if resp.Users.Len() > 0 {
		userStrList := make([]string, 0, len(resp.Users))
		for userName := range resp.Users {
			userStrList = append(userStrList, fmt.Sprintf("    - %s\n", userName))
		}

		usersStr = fmt.Sprintf("  users:\n%s", strings.Join(userStrList, ""))
	}

	var groupsStr string
	if resp.Groups.Len() > 0 {
		groupStrList := make([]string, 0, len(resp.Groups))
		for groupName := range resp.Groups {
			groupStrList = append(groupStrList, fmt.Sprintf("    - %s\n", groupName))
		}

		groupsStr = fmt.Sprintf("  groups:\n%s", strings.Join(groupStrList, ""))
	}

	return fmt.Sprintf(nsStr + usersStr + groupsStr)
}

func TestClusterReaderCoverage(t *testing.T) {
	testutil.RequireEtcd(t)
	defer testutil.DumpEtcdOnFailure(t)

	_, clusterAdminKubeConfig, err := testserver.StartTestMasterAPI()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clusterAdminClient, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	discoveryClient := client.NewDiscoveryClient(clusterAdminClient.RESTClient)

	// (map[string]*unversioned.APIResourceList, error)
	allResourceList, err := discoveryClient.ServerResources()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	allResources := map[unversioned.GroupResource]bool{}
	for _, resources := range allResourceList {
		version, err := unversioned.ParseGroupVersion(resources.GroupVersion)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		for _, resource := range resources.APIResources {
			allResources[version.WithResource(resource.Name).GroupResource()] = true
		}
	}

	escalatingResources := map[unversioned.GroupResource]bool{
		oauthapi.Resource("oauthauthorizetokens"): true,
		oauthapi.Resource("oauthaccesstokens"):    true,
		oauthapi.Resource("oauthclients"):         true,
		imageapi.Resource("imagestreams/secrets"): true,
		kapi.Resource("secrets"):                  true,
		kapi.Resource("pods/exec"):                true,
		kapi.Resource("pods/proxy"):               true,
		kapi.Resource("pods/portforward"):         true,
		kapi.Resource("nodes/proxy"):              true,
		kapi.Resource("services/proxy"):           true,
	}

	readerRole, err := clusterAdminClient.ClusterRoles().Get(bootstrappolicy.ClusterReaderRoleName)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, rule := range readerRole.Rules {
		for _, group := range rule.APIGroups {
			for resource := range rule.Resources {
				gr := unversioned.GroupResource{Group: group, Resource: resource}
				if escalatingResources[gr] {
					t.Errorf("cluster-reader role has escalating resource %v.  Check pkg/cmd/server/bootstrappolicy/policy.go.", gr)
				}
				delete(allResources, gr)
			}
		}
	}

	// remove escalating resources that cluster-reader should not have access to
	for resource := range escalatingResources {
		delete(allResources, resource)
	}

	// remove resources without read APIs
	nonreadingResources := []unversioned.GroupResource{
		buildapi.Resource("buildconfigs/instantiatebinary"), buildapi.Resource("buildconfigs/instantiate"), buildapi.Resource("builds/clone"),
		deployapi.Resource("deploymentconfigrollbacks"), deployapi.Resource("generatedeploymentconfigs"),
		deployapi.Resource("deploymentconfigs/rollback"), deployapi.Resource("deploymentconfigs/instantiate"),
		imageapi.Resource("imagestreamimports"), imageapi.Resource("imagestreammappings"),
		extensionsapi.Resource("deployments/rollback"),
		kapi.Resource("pods/attach"), kapi.Resource("namespaces/finalize"),
	}
	for _, resource := range nonreadingResources {
		delete(allResources, resource)
	}

	// anything left in the map is missing from the permissions
	if len(allResources) > 0 {
		t.Errorf("cluster-reader role is missing %v.  Check pkg/cmd/server/bootstrappolicy/policy.go.", allResources)
	}
}

func TestAuthorizationRestrictedAccessForProjectAdmins(t *testing.T) {
	testutil.RequireEtcd(t)
	defer testutil.DumpEtcdOnFailure(t)

	_, clusterAdminKubeConfig, err := testserver.StartTestMasterAPI()
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

	haroldClient, err := testserver.CreateNewProject(clusterAdminClient, *clusterAdminClientConfig, "hammer-project", "harold")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	markClient, err := testserver.CreateNewProject(clusterAdminClient, *clusterAdminClientConfig, "mallet-project", "mark")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = haroldClient.DeploymentConfigs("hammer-project").List(kapi.ListOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = markClient.DeploymentConfigs("hammer-project").List(kapi.ListOptions{})
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
		projects, err := client.Projects().List(kapi.ListOptions{})
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

func TestAuthorizationResolution(t *testing.T) {
	testutil.RequireEtcd(t)
	defer testutil.DumpEtcdOnFailure(t)

	_, clusterAdminKubeConfig, err := testserver.StartTestMasterAPI()
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

	// try to add Valerie to a non-existent role, looping until it is true due to
	// the policy cache taking time to react
	if err := wait.Poll(time.Second, 2*time.Minute, func() (bool, error) {
		err := addValerie.AddRole()
		if kapierror.IsNotFound(err) {
			return true, nil
		}
		return false, err
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	roleWithGroup := &authorizationapi.ClusterRole{}
	roleWithGroup.Name = "with-group"
	roleWithGroup.Rules = append(roleWithGroup.Rules, authorizationapi.PolicyRule{
		Verbs:     sets.NewString("list"),
		Resources: sets.NewString("resourcegroup:builds"),
	})
	if _, err := clusterAdminClient.ClusterRoles().Create(roleWithGroup); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	addBuildLister := &policy.RoleModificationOptions{
		RoleNamespace:       "",
		RoleName:            "with-group",
		RoleBindingAccessor: policy.NewClusterRoleBindingAccessor(clusterAdminClient),
		Users:               []string{"build-lister"},
	}
	if err := addBuildLister.AddRole(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clusterAdminConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	buildListerClient, _, _, err := testutil.GetClientForUser(*clusterAdminConfig, "build-lister")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// the authorization cache may not be up to date, retry
	if err := wait.Poll(10*time.Millisecond, 2*time.Minute, func() (bool, error) {
		_, err := buildListerClient.Builds(kapi.NamespaceDefault).List(kapi.ListOptions{})
		if kapierror.IsForbidden(err) {
			return false, nil
		}
		return err == nil, err
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := buildListerClient.Builds(kapi.NamespaceDefault).List(kapi.ListOptions{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := buildListerClient.DeploymentConfigs(kapi.NamespaceDefault).List(kapi.ListOptions{}); !kapierror.IsForbidden(err) {
		t.Errorf("expected forbidden, got %v", err)
	}

}

// TODO this list should start collapsing as we continue to tighten access on generated system ids
var globalClusterAdminUsers = sets.NewString("system:admin")
var globalClusterAdminGroups = sets.NewString("system:cluster-admins", "system:masters")

// This list includes the admins from above, plus users or groups known to have global view access
var globalClusterReaderUsers = sets.NewString("system:serviceaccount:openshift-infra:namespace-controller", "system:admin")
var globalClusterReaderGroups = sets.NewString("system:cluster-readers", "system:cluster-admins", "system:masters")

// this list includes any other users who can get DeploymentConfigs
var globalDeploymentConfigGetterUsers = sets.NewString("system:serviceaccount:openshift-infra:unidling-controller")

type resourceAccessReviewTest struct {
	description     string
	clientInterface client.ResourceAccessReviewInterface
	review          *authorizationapi.ResourceAccessReview

	response authorizationapi.ResourceAccessReviewResponse
	err      string
}

func (test resourceAccessReviewTest) run(t *testing.T) {
	failMessage := ""

	// keep trying the test until you get a success or you timeout.  Every time you have a failure, set the fail message
	// so that if you never have a success, we can call t.Errorf with a reasonable message
	// exiting the poll with `failMessage=""` indicates success.
	err := wait.Poll(testutil.PolicyCachePollInterval, testutil.PolicyCachePollTimeout, func() (bool, error) {
		actualResponse, err := test.clientInterface.Create(test.review)
		if len(test.err) > 0 {
			if err == nil {
				failMessage = fmt.Sprintf("%s: Expected error: %v", test.description, test.err)
				return false, nil
			} else if !strings.Contains(err.Error(), test.err) {
				failMessage = fmt.Sprintf("%s: expected %v, got %v", test.description, test.err, err)
				return false, nil
			}
		} else {
			if err != nil {
				failMessage = fmt.Sprintf("%s: unexpected error: %v", test.description, err)
				return false, nil
			}
		}

		if actualResponse.Namespace != test.response.Namespace ||
			!reflect.DeepEqual(actualResponse.Users.List(), test.response.Users.List()) ||
			!reflect.DeepEqual(actualResponse.Groups.List(), test.response.Groups.List()) ||
			actualResponse.EvaluationError != test.response.EvaluationError {
			failMessage = fmt.Sprintf("%s:\n  %s:\n  expected %s\n  got %s", test.description, prettyPrintAction(&test.review.Action, "(in any namespace)"), prettyPrintReviewResponse(&test.response), prettyPrintReviewResponse(actualResponse))
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

type localResourceAccessReviewTest struct {
	description     string
	clientInterface client.LocalResourceAccessReviewInterface
	review          *authorizationapi.LocalResourceAccessReview

	response authorizationapi.ResourceAccessReviewResponse
	err      string
}

func (test localResourceAccessReviewTest) run(t *testing.T) {
	failMessage := ""

	// keep trying the test until you get a success or you timeout.  Every time you have a failure, set the fail message
	// so that if you never have a success, we can call t.Errorf with a reasonable message
	// exiting the poll with `failMessage=""` indicates success.
	err := wait.Poll(testutil.PolicyCachePollInterval, testutil.PolicyCachePollTimeout, func() (bool, error) {
		actualResponse, err := test.clientInterface.Create(test.review)
		if len(test.err) > 0 {
			if err == nil {
				failMessage = fmt.Sprintf("%s: Expected error: %v", test.description, test.err)
				return false, nil
			} else if !strings.Contains(err.Error(), test.err) {
				failMessage = fmt.Sprintf("%s: expected %v, got %v", test.description, test.err, err)
				return false, nil
			}
		} else {
			if err != nil {
				failMessage = fmt.Sprintf("%s: unexpected error: %v", test.description, err)
				return false, nil
			}
		}

		if actualResponse.Namespace != test.response.Namespace ||
			!reflect.DeepEqual(actualResponse.Users.List(), test.response.Users.List()) ||
			!reflect.DeepEqual(actualResponse.Groups.List(), test.response.Groups.List()) ||
			actualResponse.EvaluationError != test.response.EvaluationError {
			failMessage = fmt.Sprintf("%s:\n  %s:\n  expected %s\n  got %s", test.description, prettyPrintAction(&test.review.Action, "(in the current namespace)"), prettyPrintReviewResponse(&test.response), prettyPrintReviewResponse(actualResponse))
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

func TestAuthorizationResourceAccessReview(t *testing.T) {
	testutil.RequireEtcd(t)
	defer testutil.DumpEtcdOnFailure(t)

	_, clusterAdminKubeConfig, err := testserver.StartTestMasterAPI()
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

	haroldClient, err := testserver.CreateNewProject(clusterAdminClient, *clusterAdminClientConfig, "hammer-project", "harold")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	markClient, err := testserver.CreateNewProject(clusterAdminClient, *clusterAdminClientConfig, "mallet-project", "mark")
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

	requestWhoCanViewDeploymentConfigs := &authorizationapi.ResourceAccessReview{
		Action: authorizationapi.Action{Verb: "get", Resource: "deploymentconfigs"},
	}

	localRequestWhoCanViewDeploymentConfigs := &authorizationapi.LocalResourceAccessReview{
		Action: authorizationapi.Action{Verb: "get", Resource: "deploymentconfigs"},
	}

	{
		test := localResourceAccessReviewTest{
			description:     "who can view deploymentconfigs in hammer by harold",
			clientInterface: haroldClient.LocalResourceAccessReviews("hammer-project"),
			review:          localRequestWhoCanViewDeploymentConfigs,
			response: authorizationapi.ResourceAccessReviewResponse{
				Users:     sets.NewString("harold", "valerie"),
				Groups:    sets.NewString(),
				Namespace: "hammer-project",
			},
		}
		test.response.Users.Insert(globalClusterReaderUsers.List()...)
		test.response.Users.Insert(globalDeploymentConfigGetterUsers.List()...)
		test.response.Groups.Insert(globalClusterReaderGroups.List()...)
		test.run(t)
	}
	{
		test := localResourceAccessReviewTest{
			description:     "who can view deploymentconfigs in mallet by mark",
			clientInterface: markClient.LocalResourceAccessReviews("mallet-project"),
			review:          localRequestWhoCanViewDeploymentConfigs,
			response: authorizationapi.ResourceAccessReviewResponse{
				Users:     sets.NewString("mark", "edgar"),
				Groups:    sets.NewString(),
				Namespace: "mallet-project",
			},
		}
		test.response.Users.Insert(globalClusterReaderUsers.List()...)
		test.response.Users.Insert(globalDeploymentConfigGetterUsers.List()...)
		test.response.Groups.Insert(globalClusterReaderGroups.List()...)
		test.run(t)
	}

	// mark should not be able to make global access review requests
	{
		test := resourceAccessReviewTest{
			description:     "who can view deploymentconfigs in all by mark",
			clientInterface: markClient.ResourceAccessReviews(),
			review:          requestWhoCanViewDeploymentConfigs,
			err:             "cannot ",
		}
		test.run(t)
	}

	// a cluster-admin should be able to make global access review requests
	{
		test := resourceAccessReviewTest{
			description:     "who can view deploymentconfigs in all by cluster-admin",
			clientInterface: clusterAdminClient.ResourceAccessReviews(),
			review:          requestWhoCanViewDeploymentConfigs,
			response: authorizationapi.ResourceAccessReviewResponse{
				Users:  sets.NewString(),
				Groups: sets.NewString(),
			},
		}
		test.response.Users.Insert(globalClusterReaderUsers.List()...)
		test.response.Users.Insert(globalDeploymentConfigGetterUsers.List()...)
		test.response.Groups.Insert(globalClusterReaderGroups.List()...)
		test.run(t)
	}

	{
		if err := clusterAdminClient.ClusterRoles().Delete(bootstrappolicy.AdminRoleName); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		test := localResourceAccessReviewTest{
			description:     "who can view deploymentconfigs in mallet by cluster-admin",
			clientInterface: clusterAdminClient.LocalResourceAccessReviews("mallet-project"),
			review:          localRequestWhoCanViewDeploymentConfigs,
			response: authorizationapi.ResourceAccessReviewResponse{
				Users:           sets.NewString("edgar"),
				Groups:          sets.NewString(),
				Namespace:       "mallet-project",
				EvaluationError: `role "admin" not found`,
			},
		}
		test.response.Users.Insert(globalClusterReaderUsers.List()...)
		test.response.Users.Insert(globalDeploymentConfigGetterUsers.List()...)
		test.response.Groups.Insert(globalClusterReaderGroups.List()...)
		test.run(t)
	}
}

type subjectAccessReviewTest struct {
	description      string
	localInterface   client.LocalSubjectAccessReviewInterface
	clusterInterface client.SubjectAccessReviewInterface
	localReview      *authorizationapi.LocalSubjectAccessReview
	clusterReview    *authorizationapi.SubjectAccessReview

	response authorizationapi.SubjectAccessReviewResponse
	err      string
}

func (test subjectAccessReviewTest) run(t *testing.T) {
	failMessage := ""
	err := wait.Poll(testutil.PolicyCachePollInterval, testutil.PolicyCachePollTimeout, func() (bool, error) {
		var err error
		var actualResponse *authorizationapi.SubjectAccessReviewResponse
		if test.localReview != nil {
			actualResponse, err = test.localInterface.Create(test.localReview)
		} else {
			actualResponse, err = test.clusterInterface.Create(test.clusterReview)
		}
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
			if test.localReview != nil {
				failMessage = fmt.Sprintf("%s: from local review\n\t%#v\nexpected\n\t%#v\ngot\n\t%#v", test.description, test.localReview, &test.response, actualResponse)
			} else {
				failMessage = fmt.Sprintf("%s: from review\n\t%#v\nexpected\n\t%#v\ngot\n\t%#v", test.description, test.clusterReview, &test.response, actualResponse)
			}
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

func TestAuthorizationSubjectAccessReviewAPIGroup(t *testing.T) {
	testutil.RequireEtcd(t)
	defer testutil.DumpEtcdOnFailure(t)

	_, clusterAdminKubeConfig, err := testserver.StartTestMaster()
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

	_, err = testserver.CreateNewProject(clusterAdminClient, *clusterAdminClientConfig, "hammer-project", "harold")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// SAR honors API Group
	subjectAccessReviewTest{
		description:    "cluster admin told harold can get extensions.horizontalpodautoscalers in project hammer-project",
		localInterface: clusterAdminClient.LocalSubjectAccessReviews("hammer-project"),
		localReview: &authorizationapi.LocalSubjectAccessReview{
			User:   "harold",
			Action: authorizationapi.Action{Verb: "get", Group: "extensions", Resource: "horizontalpodautoscalers"},
		},
		response: authorizationapi.SubjectAccessReviewResponse{
			Allowed:   true,
			Reason:    "allowed by rule in hammer-project",
			Namespace: "hammer-project",
		},
	}.run(t)
	subjectAccessReviewTest{
		description:    "cluster admin told harold cannot get horizontalpodautoscalers (with no API group) in project hammer-project",
		localInterface: clusterAdminClient.LocalSubjectAccessReviews("hammer-project"),
		localReview: &authorizationapi.LocalSubjectAccessReview{
			User:   "harold",
			Action: authorizationapi.Action{Verb: "get", Group: "", Resource: "horizontalpodautoscalers"},
		},
		response: authorizationapi.SubjectAccessReviewResponse{
			Allowed:   false,
			Reason:    `User "harold" cannot get horizontalpodautoscalers in project "hammer-project"`,
			Namespace: "hammer-project",
		},
	}.run(t)
	subjectAccessReviewTest{
		description:    "cluster admin told harold cannot get horizontalpodautoscalers (with invalid API group) in project hammer-project",
		localInterface: clusterAdminClient.LocalSubjectAccessReviews("hammer-project"),
		localReview: &authorizationapi.LocalSubjectAccessReview{
			User:   "harold",
			Action: authorizationapi.Action{Verb: "get", Group: "foo", Resource: "horizontalpodautoscalers"},
		},
		response: authorizationapi.SubjectAccessReviewResponse{
			Allowed:   false,
			Reason:    `User "harold" cannot get foo.horizontalpodautoscalers in project "hammer-project"`,
			Namespace: "hammer-project",
		},
	}.run(t)
	subjectAccessReviewTest{
		description:    "cluster admin told harold cannot get horizontalpodautoscalers (with * API group) in project hammer-project",
		localInterface: clusterAdminClient.LocalSubjectAccessReviews("hammer-project"),
		localReview: &authorizationapi.LocalSubjectAccessReview{
			User:   "harold",
			Action: authorizationapi.Action{Verb: "get", Group: "*", Resource: "horizontalpodautoscalers"},
		},
		response: authorizationapi.SubjectAccessReviewResponse{
			Allowed:   false,
			Reason:    `User "harold" cannot get *.horizontalpodautoscalers in project "hammer-project"`,
			Namespace: "hammer-project",
		},
	}.run(t)

	// SAR honors API Group for cluster admin self SAR
	subjectAccessReviewTest{
		description:    "cluster admin told they can get extensions.horizontalpodautoscalers in project hammer-project",
		localInterface: clusterAdminClient.LocalSubjectAccessReviews("any-project"),
		localReview: &authorizationapi.LocalSubjectAccessReview{
			Action: authorizationapi.Action{Verb: "get", Group: "extensions", Resource: "horizontalpodautoscalers"},
		},
		response: authorizationapi.SubjectAccessReviewResponse{
			Allowed:   true,
			Reason:    "allowed by rule in any-project",
			Namespace: "any-project",
		},
	}.run(t)
	subjectAccessReviewTest{
		description:    "cluster admin told they can get horizontalpodautoscalers (with no API group) in project any-project",
		localInterface: clusterAdminClient.LocalSubjectAccessReviews("any-project"),
		localReview: &authorizationapi.LocalSubjectAccessReview{
			Action: authorizationapi.Action{Verb: "get", Group: "", Resource: "horizontalpodautoscalers"},
		},
		response: authorizationapi.SubjectAccessReviewResponse{
			Allowed:   true,
			Reason:    "allowed by rule in any-project",
			Namespace: "any-project",
		},
	}.run(t)
	subjectAccessReviewTest{
		description:    "cluster admin told they can get horizontalpodautoscalers (with invalid API group) in project any-project",
		localInterface: clusterAdminClient.LocalSubjectAccessReviews("any-project"),
		localReview: &authorizationapi.LocalSubjectAccessReview{
			Action: authorizationapi.Action{Verb: "get", Group: "foo", Resource: "horizontalpodautoscalers"},
		},
		response: authorizationapi.SubjectAccessReviewResponse{
			Allowed:   true,
			Reason:    "allowed by rule in any-project",
			Namespace: "any-project",
		},
	}.run(t)
	subjectAccessReviewTest{
		description:    "cluster admin told they can get horizontalpodautoscalers (with * API group) in project any-project",
		localInterface: clusterAdminClient.LocalSubjectAccessReviews("any-project"),
		localReview: &authorizationapi.LocalSubjectAccessReview{
			Action: authorizationapi.Action{Verb: "get", Group: "*", Resource: "horizontalpodautoscalers"},
		},
		response: authorizationapi.SubjectAccessReviewResponse{
			Allowed:   true,
			Reason:    "allowed by rule in any-project",
			Namespace: "any-project",
		},
	}.run(t)
}

func TestAuthorizationSubjectAccessReview(t *testing.T) {
	testutil.RequireEtcd(t)
	defer testutil.DumpEtcdOnFailure(t)

	_, clusterAdminKubeConfig, err := testserver.StartTestMasterAPI()
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

	haroldClient, err := testserver.CreateNewProject(clusterAdminClient, *clusterAdminClientConfig, "hammer-project", "harold")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	markClient, err := testserver.CreateNewProject(clusterAdminClient, *clusterAdminClientConfig, "mallet-project", "mark")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	dannyClient, _, dannyConfig, err := testutil.GetClientForUser(*clusterAdminClientConfig, "danny")
	if err != nil {
		t.Fatalf("error requesting token: %v", err)
	}

	anonymousConfig := clientcmd.AnonymousClientConfig(clusterAdminClientConfig)
	anonymousClient, err := client.New(&anonymousConfig)
	if err != nil {
		t.Fatalf("error getting anonymous client: %v", err)
	}

	addAnonymous := &policy.RoleModificationOptions{
		RoleNamespace:       "",
		RoleName:            bootstrappolicy.EditRoleName,
		RoleBindingAccessor: policy.NewLocalRoleBindingAccessor("hammer-project", clusterAdminClient),
		Users:               []string{"system:anonymous"},
	}
	if err := addAnonymous.AddRole(); err != nil {
		t.Errorf("unexpected error: %v", err)
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
	askCanDannyGetProject := &authorizationapi.SubjectAccessReview{
		User:   "danny",
		Action: authorizationapi.Action{Verb: "get", Resource: "projects"},
	}
	subjectAccessReviewTest{
		description:    "cluster admin told danny can get project default",
		localInterface: clusterAdminClient.LocalSubjectAccessReviews("default"),
		localReview: &authorizationapi.LocalSubjectAccessReview{
			User:   "danny",
			Action: authorizationapi.Action{Verb: "get", Resource: "projects"},
		},
		response: authorizationapi.SubjectAccessReviewResponse{
			Allowed:   true,
			Reason:    "allowed by rule in default",
			Namespace: "default",
		},
	}.run(t)
	subjectAccessReviewTest{
		description:      "cluster admin told danny cannot get projects cluster-wide",
		clusterInterface: clusterAdminClient.SubjectAccessReviews(),
		clusterReview:    askCanDannyGetProject,
		response: authorizationapi.SubjectAccessReviewResponse{
			Allowed:   false,
			Reason:    `User "danny" cannot get projects at the cluster scope`,
			Namespace: "",
		},
	}.run(t)
	subjectAccessReviewTest{
		description:      "as danny, can I make cluster subject access reviews",
		clusterInterface: dannyClient.SubjectAccessReviews(),
		clusterReview:    askCanDannyGetProject,
		err:              `User "danny" cannot create subjectaccessreviews at the cluster scope`,
	}.run(t)
	subjectAccessReviewTest{
		description:      "as anonymous, can I make cluster subject access reviews",
		clusterInterface: anonymousClient.SubjectAccessReviews(),
		clusterReview:    askCanDannyGetProject,
		err:              `User "system:anonymous" cannot create subjectaccessreviews at the cluster scope`,
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

	askCanValerieGetProject := &authorizationapi.LocalSubjectAccessReview{
		User:   "valerie",
		Action: authorizationapi.Action{Verb: "get", Resource: "projects"},
	}
	subjectAccessReviewTest{
		description:    "harold told valerie can get project hammer-project",
		localInterface: haroldClient.LocalSubjectAccessReviews("hammer-project"),
		localReview:    askCanValerieGetProject,
		response: authorizationapi.SubjectAccessReviewResponse{
			Allowed:   true,
			Reason:    "allowed by rule in hammer-project",
			Namespace: "hammer-project",
		},
	}.run(t)
	subjectAccessReviewTest{
		description:    "mark told valerie cannot get project mallet-project",
		localInterface: markClient.LocalSubjectAccessReviews("mallet-project"),
		localReview:    askCanValerieGetProject,
		response: authorizationapi.SubjectAccessReviewResponse{
			Allowed:   false,
			Reason:    `User "valerie" cannot get projects in project "mallet-project"`,
			Namespace: "mallet-project",
		},
	}.run(t)

	askCanEdgarDeletePods := &authorizationapi.LocalSubjectAccessReview{
		User:   "edgar",
		Action: authorizationapi.Action{Verb: "delete", Resource: "pods"},
	}
	subjectAccessReviewTest{
		description:    "mark told edgar can delete pods in mallet-project",
		localInterface: markClient.LocalSubjectAccessReviews("mallet-project"),
		localReview:    askCanEdgarDeletePods,
		response: authorizationapi.SubjectAccessReviewResponse{
			Allowed:   true,
			Reason:    "allowed by rule in mallet-project",
			Namespace: "mallet-project",
		},
	}.run(t)
	// ensure unprivileged users cannot check other users' access
	subjectAccessReviewTest{
		description:    "harold denied ability to run subject access review in project mallet-project",
		localInterface: haroldClient.LocalSubjectAccessReviews("mallet-project"),
		localReview:    askCanEdgarDeletePods,
		err:            `User "harold" cannot create localsubjectaccessreviews in project "mallet-project"`,
	}.run(t)
	subjectAccessReviewTest{
		description:    "system:anonymous denied ability to run subject access review in project mallet-project",
		localInterface: anonymousClient.LocalSubjectAccessReviews("mallet-project"),
		localReview:    askCanEdgarDeletePods,
		err:            `User "system:anonymous" cannot create localsubjectaccessreviews in project "mallet-project"`,
	}.run(t)
	// ensure message does not leak whether the namespace exists or not
	subjectAccessReviewTest{
		description:    "harold denied ability to run subject access review in project nonexistent-project",
		localInterface: haroldClient.LocalSubjectAccessReviews("nonexistent-project"),
		localReview:    askCanEdgarDeletePods,
		err:            `User "harold" cannot create localsubjectaccessreviews in project "nonexistent-project"`,
	}.run(t)
	subjectAccessReviewTest{
		description:    "system:anonymous denied ability to run subject access review in project nonexistent-project",
		localInterface: anonymousClient.LocalSubjectAccessReviews("nonexistent-project"),
		localReview:    askCanEdgarDeletePods,
		err:            `User "system:anonymous" cannot create localsubjectaccessreviews in project "nonexistent-project"`,
	}.run(t)

	askCanHaroldUpdateProject := &authorizationapi.LocalSubjectAccessReview{
		User:   "harold",
		Action: authorizationapi.Action{Verb: "update", Resource: "projects"},
	}
	subjectAccessReviewTest{
		description:    "harold told harold can update project hammer-project",
		localInterface: haroldClient.LocalSubjectAccessReviews("hammer-project"),
		localReview:    askCanHaroldUpdateProject,
		response: authorizationapi.SubjectAccessReviewResponse{
			Allowed:   true,
			Reason:    "allowed by rule in hammer-project",
			Namespace: "hammer-project",
		},
	}.run(t)

	askCanClusterAdminsCreateProject := &authorizationapi.SubjectAccessReview{
		Groups: sets.NewString("system:cluster-admins"),
		Action: authorizationapi.Action{Verb: "create", Resource: "projects"},
	}
	subjectAccessReviewTest{
		description:      "cluster admin told cluster admins can create projects",
		clusterInterface: clusterAdminClient.SubjectAccessReviews(),
		clusterReview:    askCanClusterAdminsCreateProject,
		response: authorizationapi.SubjectAccessReviewResponse{
			Allowed:   true,
			Reason:    "allowed by cluster rule",
			Namespace: "",
		},
	}.run(t)
	subjectAccessReviewTest{
		description:      "harold denied ability to run cluster subject access review",
		clusterInterface: haroldClient.SubjectAccessReviews(),
		clusterReview:    askCanClusterAdminsCreateProject,
		err:              `User "harold" cannot create subjectaccessreviews at the cluster scope`,
	}.run(t)

	askCanICreatePods := &authorizationapi.LocalSubjectAccessReview{
		Action: authorizationapi.Action{Verb: "create", Resource: "pods"},
	}
	subjectAccessReviewTest{
		description:    "harold told he can create pods in project hammer-project",
		localInterface: haroldClient.LocalSubjectAccessReviews("hammer-project"),
		localReview:    askCanICreatePods,
		response: authorizationapi.SubjectAccessReviewResponse{
			Allowed:   true,
			Reason:    "allowed by rule in hammer-project",
			Namespace: "hammer-project",
		},
	}.run(t)
	subjectAccessReviewTest{
		description:    "system:anonymous told he can create pods in project hammer-project",
		localInterface: anonymousClient.LocalSubjectAccessReviews("hammer-project"),
		localReview:    askCanICreatePods,
		response: authorizationapi.SubjectAccessReviewResponse{
			Allowed:   true,
			Reason:    "allowed by rule in hammer-project",
			Namespace: "hammer-project",
		},
	}.run(t)

	// test checking self permissions when denied
	subjectAccessReviewTest{
		description:    "harold told he cannot create pods in project mallet-project",
		localInterface: haroldClient.LocalSubjectAccessReviews("mallet-project"),
		localReview:    askCanICreatePods,
		response: authorizationapi.SubjectAccessReviewResponse{
			Allowed:   false,
			Reason:    `User "harold" cannot create pods in project "mallet-project"`,
			Namespace: "mallet-project",
		},
	}.run(t)
	subjectAccessReviewTest{
		description:    "system:anonymous told he cannot create pods in project mallet-project",
		localInterface: anonymousClient.LocalSubjectAccessReviews("mallet-project"),
		localReview:    askCanICreatePods,
		response: authorizationapi.SubjectAccessReviewResponse{
			Allowed:   false,
			Reason:    `User "system:anonymous" cannot create pods in project "mallet-project"`,
			Namespace: "mallet-project",
		},
	}.run(t)

	// test checking self-permissions doesn't leak whether namespace exists or not
	// We carry a patch to allow this
	subjectAccessReviewTest{
		description:    "harold told he cannot create pods in project nonexistent-project",
		localInterface: haroldClient.LocalSubjectAccessReviews("nonexistent-project"),
		localReview:    askCanICreatePods,
		response: authorizationapi.SubjectAccessReviewResponse{
			Allowed:   false,
			Reason:    `User "harold" cannot create pods in project "nonexistent-project"`,
			Namespace: "nonexistent-project",
		},
	}.run(t)
	subjectAccessReviewTest{
		description:    "system:anonymous told he cannot create pods in project nonexistent-project",
		localInterface: anonymousClient.LocalSubjectAccessReviews("nonexistent-project"),
		localReview:    askCanICreatePods,
		response: authorizationapi.SubjectAccessReviewResponse{
			Allowed:   false,
			Reason:    `User "system:anonymous" cannot create pods in project "nonexistent-project"`,
			Namespace: "nonexistent-project",
		},
	}.run(t)

	askCanICreatePolicyBindings := &authorizationapi.LocalSubjectAccessReview{
		Action: authorizationapi.Action{Verb: "create", Resource: "policybindings"},
	}
	subjectAccessReviewTest{
		description:    "harold told he can create policybindings in project hammer-project",
		localInterface: haroldClient.LocalSubjectAccessReviews("hammer-project"),
		localReview:    askCanICreatePolicyBindings,
		response: authorizationapi.SubjectAccessReviewResponse{
			Allowed:   false,
			Reason:    `User "harold" cannot create policybindings in project "hammer-project"`,
			Namespace: "hammer-project",
		},
	}.run(t)

	// impersonate SAR tests
	// impersonated empty token SAR shouldn't be allowed at all
	// impersonated danny token SAR shouldn't be allowed to see pods in hammer or in cluster
	// impersonated danny token SAR should be allowed to see pods in default
	// we need a token client for overriding
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	otherAdminClient, _, _, err := testutil.GetClientForUser(*clusterAdminClientConfig, "other-admin")
	if err != nil {
		t.Fatalf("error requesting token: %v", err)
	}

	addOtherAdmin := &policy.RoleModificationOptions{
		RoleNamespace:       "",
		RoleName:            bootstrappolicy.ClusterAdminRoleName,
		RoleBindingAccessor: policy.NewClusterRoleBindingAccessor(clusterAdminClient),
		Users:               []string{"other-admin"},
	}
	if err := addOtherAdmin.AddRole(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	subjectAccessReviewTest{
		description:    "empty token impersonate can't see pods in namespace",
		localInterface: otherAdminClient.ImpersonateLocalSubjectAccessReviews("hammer-project", ""),
		localReview: &authorizationapi.LocalSubjectAccessReview{
			Action: authorizationapi.Action{Verb: "list", Resource: "pods"},
		},
		err: `impersonating token may not be empty`,
	}.run(t)
	subjectAccessReviewTest{
		description:      "empty token impersonate can't see pods in cluster",
		clusterInterface: otherAdminClient.ImpersonateSubjectAccessReviews(""),
		clusterReview: &authorizationapi.SubjectAccessReview{
			Action: authorizationapi.Action{Verb: "list", Resource: "pods"},
		},
		err: `impersonating token may not be empty`,
	}.run(t)

	subjectAccessReviewTest{
		description:    "danny impersonate can't see pods in hammer namespace",
		localInterface: otherAdminClient.ImpersonateLocalSubjectAccessReviews("hammer-project", dannyConfig.BearerToken),
		localReview: &authorizationapi.LocalSubjectAccessReview{
			Action: authorizationapi.Action{Verb: "list", Resource: "pods"},
		},
		response: authorizationapi.SubjectAccessReviewResponse{
			Allowed:   false,
			Reason:    `User "danny" cannot list pods in project "hammer-project"`,
			Namespace: "hammer-project",
		},
	}.run(t)
	subjectAccessReviewTest{
		description:      "danny impersonate can't see pods in cluster",
		clusterInterface: otherAdminClient.ImpersonateSubjectAccessReviews(dannyConfig.BearerToken),
		clusterReview: &authorizationapi.SubjectAccessReview{
			Action: authorizationapi.Action{Verb: "list", Resource: "pods"},
		},
		response: authorizationapi.SubjectAccessReviewResponse{
			Allowed: false,
			Reason:  `User "danny" cannot list all pods in the cluster`,
		},
	}.run(t)
	subjectAccessReviewTest{
		description:    "danny impersonate can see pods in default",
		localInterface: otherAdminClient.ImpersonateLocalSubjectAccessReviews("default", dannyConfig.BearerToken),
		localReview: &authorizationapi.LocalSubjectAccessReview{
			Action: authorizationapi.Action{Verb: "list", Resource: "pods"},
		},
		response: authorizationapi.SubjectAccessReviewResponse{
			Allowed:   true,
			Reason:    `allowed by rule in default`,
			Namespace: "default",
		},
	}.run(t)
}

// TestOldLocalSubjectAccessReviewEndpoint checks to make sure that the old subject access review endpoint still functions properly
// this is needed to support old docker registry images
func TestOldLocalSubjectAccessReviewEndpoint(t *testing.T) {
	testutil.RequireEtcd(t)
	defer testutil.DumpEtcdOnFailure(t)

	_, clusterAdminKubeConfig, err := testserver.StartTestMasterAPI()
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

	haroldClient, err := testserver.CreateNewProject(clusterAdminClient, *clusterAdminClientConfig, "hammer-project", "harold")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	namespace := "hammer-project"

	// simple check
	{
		sar := &authorizationapi.SubjectAccessReview{
			Action: authorizationapi.Action{
				Verb:     "get",
				Resource: "imagestreams/layers",
			},
		}
		actualResponse := &authorizationapi.SubjectAccessReviewResponse{}
		err := haroldClient.Post().Namespace(namespace).Resource("subjectAccessReviews").Body(sar).Do().Into(actualResponse)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		expectedResponse := &authorizationapi.SubjectAccessReviewResponse{
			Allowed:   true,
			Reason:    `allowed by rule in hammer-project`,
			Namespace: namespace,
		}
		if (actualResponse.Namespace != expectedResponse.Namespace) ||
			(actualResponse.Allowed != expectedResponse.Allowed) ||
			(!strings.HasPrefix(actualResponse.Reason, expectedResponse.Reason)) {
			t.Errorf("review\n\t%#v\nexpected\n\t%#v\ngot\n\t%#v", sar, expectedResponse, actualResponse)
		}
	}

	// namespace forced to allowed namespace so we can't trick the server into leaking
	{
		sar := &authorizationapi.SubjectAccessReview{
			Action: authorizationapi.Action{
				Namespace: "sneaky-user",
				Verb:      "get",
				Resource:  "imagestreams/layers",
			},
		}
		actualResponse := &authorizationapi.SubjectAccessReviewResponse{}
		err := haroldClient.Post().Namespace(namespace).Resource("subjectAccessReviews").Body(sar).Do().Into(actualResponse)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		expectedResponse := &authorizationapi.SubjectAccessReviewResponse{
			Allowed:   true,
			Reason:    `allowed by rule in hammer-project`,
			Namespace: namespace,
		}
		if (actualResponse.Namespace != expectedResponse.Namespace) ||
			(actualResponse.Allowed != expectedResponse.Allowed) ||
			(!strings.HasPrefix(actualResponse.Reason, expectedResponse.Reason)) {
			t.Errorf("review\n\t%#v\nexpected\n\t%#v\ngot\n\t%#v", sar, expectedResponse, actualResponse)
		}
	}

	// harold should be able to issue a self SAR against any project with the OLD policy
	{
		otherNamespace := "chisel-project"
		// we need a real project for this to make it past admission.
		// TODO, this is an information leaking problem.  This admission plugin leaks knowledge of which projects exist via SARs
		if _, err := testserver.CreateNewProject(clusterAdminClient, *clusterAdminClientConfig, otherNamespace, "charlie"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// remove the new permission for localSAR
		basicUserRole, err := clusterAdminClient.ClusterRoles().Get(bootstrappolicy.BasicUserRoleName)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		for i := range basicUserRole.Rules {
			basicUserRole.Rules[i].Resources.Delete("localsubjectaccessreviews")
		}

		if _, err := clusterAdminClient.ClusterRoles().Update(basicUserRole); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		sar := &authorizationapi.SubjectAccessReview{
			Action: authorizationapi.Action{
				Verb:     "get",
				Resource: "imagestreams/layers",
			},
		}
		actualResponse := &authorizationapi.SubjectAccessReviewResponse{}
		err = haroldClient.Post().Namespace(otherNamespace).Resource("subjectAccessReviews").Body(sar).Do().Into(actualResponse)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		expectedResponse := &authorizationapi.SubjectAccessReviewResponse{
			Allowed:   false,
			Reason:    `User "harold" cannot get imagestreams/layers in project "chisel-project"`,
			Namespace: otherNamespace,
		}
		if (actualResponse.Namespace != expectedResponse.Namespace) ||
			(actualResponse.Allowed != expectedResponse.Allowed) ||
			(!strings.HasPrefix(actualResponse.Reason, expectedResponse.Reason)) {
			t.Errorf("review\n\t%#v\nexpected\n\t%#v\ngot\n\t%#v", sar, expectedResponse, actualResponse)
		}
	}

}

// TestOldLocalResourceAccessReviewEndpoint checks to make sure that the old resource access review endpoint still functions properly
// this is needed to support old who-can client
func TestOldLocalResourceAccessReviewEndpoint(t *testing.T) {
	testutil.RequireEtcd(t)
	defer testutil.DumpEtcdOnFailure(t)

	_, clusterAdminKubeConfig, err := testserver.StartTestMasterAPI()
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

	haroldClient, err := testserver.CreateNewProject(clusterAdminClient, *clusterAdminClientConfig, "hammer-project", "harold")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	namespace := "hammer-project"

	// simple check
	{
		rar := &authorizationapi.ResourceAccessReview{
			Action: authorizationapi.Action{
				Verb:     "get",
				Resource: "imagestreams/layers",
			},
		}
		actualResponse := &authorizationapi.ResourceAccessReviewResponse{}
		err := haroldClient.Post().Namespace(namespace).Resource("resourceAccessReviews").Body(rar).Do().Into(actualResponse)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		expectedResponse := &authorizationapi.ResourceAccessReviewResponse{
			Namespace: namespace,
			Users:     sets.NewString("harold", "system:serviceaccount:hammer-project:builder", "system:serviceaccount:openshift-infra:namespace-controller", "system:admin"),
			Groups:    sets.NewString("system:cluster-admins", "system:masters", "system:cluster-readers", "system:serviceaccounts:hammer-project"),
		}
		if (actualResponse.Namespace != expectedResponse.Namespace) ||
			!reflect.DeepEqual(actualResponse.Users.List(), expectedResponse.Users.List()) ||
			!reflect.DeepEqual(actualResponse.Groups.List(), expectedResponse.Groups.List()) {
			t.Errorf("review\n\t%#v\nexpected\n\t%#v\ngot\n\t%#v", rar, expectedResponse, actualResponse)
		}
	}

	// namespace forced to allowed namespace so we can't trick the server into leaking
	{
		rar := &authorizationapi.ResourceAccessReview{
			Action: authorizationapi.Action{
				Namespace: "sneaky-user",
				Verb:      "get",
				Resource:  "imagestreams/layers",
			},
		}
		actualResponse := &authorizationapi.ResourceAccessReviewResponse{}
		err := haroldClient.Post().Namespace(namespace).Resource("resourceAccessReviews").Body(rar).Do().Into(actualResponse)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		expectedResponse := &authorizationapi.ResourceAccessReviewResponse{
			Namespace: namespace,
			Users:     sets.NewString("harold", "system:serviceaccount:hammer-project:builder", "system:serviceaccount:openshift-infra:namespace-controller", "system:admin"),
			Groups:    sets.NewString("system:cluster-admins", "system:masters", "system:cluster-readers", "system:serviceaccounts:hammer-project"),
		}
		if (actualResponse.Namespace != expectedResponse.Namespace) ||
			!reflect.DeepEqual(actualResponse.Users.List(), expectedResponse.Users.List()) ||
			!reflect.DeepEqual(actualResponse.Groups.List(), expectedResponse.Groups.List()) {
			t.Errorf("review\n\t%#v\nexpected\n\t%#v\ngot\n\t%#v", rar, expectedResponse, actualResponse)
		}
	}
}
