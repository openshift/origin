package integration

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	rbacv1 "k8s.io/api/rbac/v1"
	kapierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/discovery"
	rbacv1client "k8s.io/client-go/kubernetes/typed/rbac/v1"
	"k8s.io/client-go/rest"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	appsapi "k8s.io/kubernetes/pkg/apis/apps"
	kubeauthorizationapi "k8s.io/kubernetes/pkg/apis/authorization"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	extensionsapi "k8s.io/kubernetes/pkg/apis/extensions"
	rbacv1helpers "k8s.io/kubernetes/pkg/apis/rbac/v1"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/authorization/internalversion"
	rbacclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/rbac/internalversion"

	oapps "github.com/openshift/api/apps"
	"github.com/openshift/api/build"
	"github.com/openshift/api/image"
	"github.com/openshift/api/oauth"
	appsv1client "github.com/openshift/client-go/apps/clientset/versioned"
	buildv1client "github.com/openshift/client-go/build/clientset/versioned"
	projectv1client "github.com/openshift/client-go/project/clientset/versioned/typed/project/v1"
	"github.com/openshift/origin/pkg/api/legacy"
	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	authorizationclient "github.com/openshift/origin/pkg/authorization/generated/internalclientset"
	authorizationclientscheme "github.com/openshift/origin/pkg/authorization/generated/internalclientset/scheme"
	authorizationtypedclient "github.com/openshift/origin/pkg/authorization/generated/internalclientset/typed/authorization/internalversion"
	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	policy "github.com/openshift/origin/pkg/oc/cli/admin/policy"
	projectclient "github.com/openshift/origin/pkg/project/generated/internalclientset"
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
		groupVer := schema.GroupVersion{Group: act.Group, Version: act.Version}
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
	masterConfig, clusterAdminKubeConfig, err := testserver.StartTestMasterAPI()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer testserver.CleanupMasterEtcd(t, masterConfig)

	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	discoveryClient := discovery.NewDiscoveryClientForConfigOrDie(clusterAdminClientConfig)

	// (map[string]*metav1.APIResourceList, error)
	allResourceList, err := discoveryClient.ServerResources()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	allResources := map[schema.GroupResource]bool{}
	for _, resources := range allResourceList {
		version, err := schema.ParseGroupVersion(resources.GroupVersion)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		for _, resource := range resources.APIResources {
			allResources[version.WithResource(resource.Name).GroupResource()] = true
		}
	}

	escalatingResources := map[schema.GroupResource]bool{
		oauth.Resource("oauthauthorizetokens"):  true,
		oauth.Resource("oauthaccesstokens"):     true,
		oauth.Resource("oauthclients"):          true,
		image.Resource("imagestreams/secrets"):  true,
		kapi.Resource("secrets"):                true,
		kapi.Resource("pods/exec"):              true,
		kapi.Resource("pods/proxy"):             true,
		kapi.Resource("pods/portforward"):       true,
		kapi.Resource("nodes/proxy"):            true,
		kapi.Resource("services/proxy"):         true,
		legacy.Resource("oauthauthorizetokens"): true,
		legacy.Resource("oauthaccesstokens"):    true,
		legacy.Resource("oauthclients"):         true,
		legacy.Resource("imagestreams/secrets"): true,
	}

	readerRole, err := rbacclient.NewForConfigOrDie(clusterAdminClientConfig).ClusterRoles().Get(bootstrappolicy.ClusterReaderRoleName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, rule := range readerRole.Rules {
		for _, group := range rule.APIGroups {
			for _, resource := range rule.Resources {
				gr := schema.GroupResource{Group: group, Resource: resource}
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
	nonreadingResources := []schema.GroupResource{
		oapps.Resource("deploymentconfigrollbacks"),
		oapps.Resource("generatedeploymentconfigs"),
		oapps.Resource("deploymentconfigs/rollback"),
		oapps.Resource("deploymentconfigs/instantiate"),
		build.Resource("buildconfigs/instantiatebinary"),
		build.Resource("buildconfigs/instantiate"),
		build.Resource("builds/clone"),
		image.Resource("imagestreamimports"),
		image.Resource("imagestreammappings"),
		extensionsapi.Resource("deployments/rollback"),
		appsapi.Resource("deployments/rollback"),
		kapi.Resource("pods/attach"),
		kapi.Resource("namespaces/finalize"),
		{Group: "", Resource: "buildconfigs/instantiatebinary"},
		{Group: "", Resource: "buildconfigs/instantiate"},
		{Group: "", Resource: "builds/clone"},
		{Group: "", Resource: "deploymentconfigrollbacks"},
		{Group: "", Resource: "generatedeploymentconfigs"},
		{Group: "", Resource: "deploymentconfigs/rollback"},
		{Group: "", Resource: "deploymentconfigs/instantiate"},
		{Group: "", Resource: "imagestreamimports"},
		{Group: "", Resource: "imagestreammappings"},
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
	masterConfig, clusterAdminKubeConfig, err := testserver.StartTestMasterAPI()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer testserver.CleanupMasterEtcd(t, masterConfig)

	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, haroldConfig, err := testserver.CreateNewProject(clusterAdminClientConfig, "hammer-project", "harold")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, markConfig, err := testserver.CreateNewProject(clusterAdminClientConfig, "mallet-project", "mark")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = appsv1client.NewForConfigOrDie(haroldConfig).Apps().DeploymentConfigs("hammer-project").List(metav1.ListOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = appsv1client.NewForConfigOrDie(markConfig).Apps().DeploymentConfigs("hammer-project").List(metav1.ListOptions{})
	if (err == nil) || !kapierror.IsForbidden(err) {
		t.Fatalf("unexpected error: %v", err)
	}

	// projects are a special case where a get of a project actually sets a namespace.  Make sure that
	// the namespace is properly special cased and set for authorization rules
	_, err = projectclient.NewForConfigOrDie(haroldConfig).Project().Projects().Get("hammer-project", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err = projectclient.NewForConfigOrDie(markConfig).Project().Projects().Get("hammer-project", metav1.GetOptions{})
	if (err == nil) || !kapierror.IsForbidden(err) {
		t.Fatalf("unexpected error: %v", err)
	}

	// wait for the project authorization cache to catch the change.  It is on a one second period
	waitForProject(t, projectv1client.NewForConfigOrDie(haroldConfig), "hammer-project", 1*time.Second, 10)
	waitForProject(t, projectv1client.NewForConfigOrDie(markConfig), "mallet-project", 1*time.Second, 10)
}

// waitForProject will execute a client list of projects looking for the project with specified name
// if not found, it will retry up to numRetries at the specified delayInterval
func waitForProject(t *testing.T, client projectv1client.ProjectV1Interface, projectName string, delayInterval time.Duration, numRetries int) {
	for i := 0; i <= numRetries; i++ {
		projects, err := client.Projects().List(metav1.ListOptions{})
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
	masterConfig, clusterAdminKubeConfig, err := testserver.StartTestMasterAPI()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer testserver.CleanupMasterEtcd(t, masterConfig)

	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	clusterAdminAuthorizationClient := rbacv1client.NewForConfigOrDie(clusterAdminClientConfig)

	addValerie := &policy.RoleModificationOptions{
		RoleName:   bootstrappolicy.ViewRoleName,
		RoleKind:   "ClusterRole",
		RbacClient: clusterAdminAuthorizationClient,
		Users:      []string{"valerie"},
	}
	if err := addValerie.AddRole(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err = clusterAdminAuthorizationClient.ClusterRoles().Delete(bootstrappolicy.ViewRoleName, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	addEdgar := &policy.RoleModificationOptions{
		RoleName:   bootstrappolicy.EditRoleName,
		RoleKind:   "ClusterRole",
		RbacClient: clusterAdminAuthorizationClient,
		Users:      []string{"edgar"},
	}
	if err := addEdgar.AddRole(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := addValerie.AddRole(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	roleWithGroup := &rbacv1.ClusterRole{}
	roleWithGroup.Name = "with-group"
	roleWithGroup.Rules = append(roleWithGroup.Rules,
		rbacv1helpers.NewRule("list").
			Groups(buildapi.GroupName).
			Resources("builds").
			RuleOrDie())
	if _, err := clusterAdminAuthorizationClient.ClusterRoles().Create(roleWithGroup); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	addBuildLister := &policy.RoleModificationOptions{
		RoleName:   "with-group",
		RoleKind:   "ClusterRole",
		RbacClient: clusterAdminAuthorizationClient,
		Users:      []string{"build-lister"},
	}
	if err := addBuildLister.AddRole(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clusterAdminConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, userClientConfig, err := testutil.GetClientForUser(clusterAdminConfig, "build-lister")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	buildClient := buildv1client.NewForConfigOrDie(userClientConfig)
	appsClient := appsv1client.NewForConfigOrDie(userClientConfig)

	// the authorization cache may not be up to date, retry
	if err := wait.Poll(10*time.Millisecond, 2*time.Minute, func() (bool, error) {
		_, err := buildClient.BuildV1().Builds(metav1.NamespaceDefault).List(metav1.ListOptions{})
		if kapierror.IsForbidden(err) {
			return false, nil
		}
		return err == nil, err
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := buildClient.Build().Builds(metav1.NamespaceDefault).List(metav1.ListOptions{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := appsClient.Apps().DeploymentConfigs(metav1.NamespaceDefault).List(metav1.ListOptions{}); !kapierror.IsForbidden(err) {
		t.Errorf("expected forbidden, got %v", err)
	}

}

// This list includes the admins from above, plus users or groups known to have global view access
var globalClusterReaderUsers = sets.NewString("system:admin")
var globalClusterReaderGroups = sets.NewString("system:cluster-readers", "system:cluster-admins", "system:masters")

// this list includes any other users who can get DeploymentConfigs
var globalDeploymentConfigGetterUsers = sets.NewString(
	"system:serviceaccount:kube-system:generic-garbage-collector",
	"system:serviceaccount:kube-system:namespace-controller",
	"system:serviceaccount:kube-system:clusterrole-aggregation-controller",
	"system:serviceaccount:openshift-infra:image-trigger-controller",
	"system:serviceaccount:openshift-infra:deploymentconfig-controller",
	"system:serviceaccount:openshift-infra:template-instance-controller",
	"system:serviceaccount:openshift-infra:template-instance-finalizer-controller",
	"system:serviceaccount:openshift-infra:unidling-controller",
)

type resourceAccessReviewTest struct {
	description     string
	clientInterface authorizationtypedclient.ResourceAccessReviewInterface
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
	clientInterface authorizationtypedclient.LocalResourceAccessReviewInterface
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

		if actualResponse.Namespace != test.response.Namespace {
			failMessage = fmt.Sprintf("%s\n: namespaces does not match (%s!=%s)", test.description, actualResponse.Namespace, test.response.Namespace)
			return false, nil
		}
		if actualResponse.EvaluationError != test.response.EvaluationError {
			failMessage = fmt.Sprintf("%s\n: evaluation errors does not match (%s!=%s)", test.description, actualResponse.EvaluationError, test.response.EvaluationError)
			return false, nil
		}

		if !reflect.DeepEqual(actualResponse.Users.List(), test.response.Users.List()) {
			failMessage = fmt.Sprintf("%s:\n  %s:\n  expected %s\n  got %s", test.description, prettyPrintAction(&test.review.Action, "(in the current namespace)"), prettyPrintReviewResponse(&test.response), prettyPrintReviewResponse(actualResponse))
			return false, nil
		}

		if !reflect.DeepEqual(actualResponse.Groups.List(), test.response.Groups.List()) {
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
	masterConfig, clusterAdminKubeConfig, err := testserver.StartTestMasterAPI()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer testserver.CleanupMasterEtcd(t, masterConfig)

	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	clusterAdminAuthorizationClient := authorizationclient.NewForConfigOrDie(clusterAdminClientConfig).Authorization()

	_, haroldConfig, err := testserver.CreateNewProject(clusterAdminClientConfig, "hammer-project", "harold")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	haroldAuthorizationClient := authorizationclient.NewForConfigOrDie(haroldConfig).Authorization()

	_, markConfig, err := testserver.CreateNewProject(clusterAdminClientConfig, "mallet-project", "mark")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	markAuthorizationClient := authorizationclient.NewForConfigOrDie(markConfig).Authorization()

	addValerie := &policy.RoleModificationOptions{
		RoleBindingNamespace: "hammer-project",
		RoleName:             bootstrappolicy.ViewRoleName,
		RoleKind:             "ClusterRole",
		RbacClient:           rbacv1client.NewForConfigOrDie(haroldConfig),
		Users:                []string{"valerie"},
	}
	if err := addValerie.AddRole(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	addEdgar := &policy.RoleModificationOptions{
		RoleBindingNamespace: "mallet-project",
		RoleName:             bootstrappolicy.EditRoleName,
		RoleKind:             "ClusterRole",
		RbacClient:           rbacv1client.NewForConfigOrDie(markConfig),
		Users:                []string{"edgar"},
	}
	if err := addEdgar.AddRole(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	requestWhoCanViewDeploymentConfigs := &authorizationapi.ResourceAccessReview{
		Action: authorizationapi.Action{Verb: "get", Resource: "deploymentconfigs", Group: ""},
	}

	localRequestWhoCanViewDeploymentConfigs := &authorizationapi.LocalResourceAccessReview{
		Action: authorizationapi.Action{Verb: "get", Resource: "deploymentconfigs", Group: ""},
	}

	{
		test := localResourceAccessReviewTest{
			description:     "who can view deploymentconfigs in hammer by harold",
			clientInterface: haroldAuthorizationClient.LocalResourceAccessReviews("hammer-project"),
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
			clientInterface: markAuthorizationClient.LocalResourceAccessReviews("mallet-project"),
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
			clientInterface: markAuthorizationClient.ResourceAccessReviews(),
			review:          requestWhoCanViewDeploymentConfigs,
			err:             "cannot ",
		}
		test.run(t)
	}

	// a cluster-admin should be able to make global access review requests
	{
		test := resourceAccessReviewTest{
			description:     "who can view deploymentconfigs in all by cluster-admin",
			clientInterface: clusterAdminAuthorizationClient.ResourceAccessReviews(),
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
		if err := clusterAdminAuthorizationClient.ClusterRoles().Delete(bootstrappolicy.AdminRoleName, nil); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		test := localResourceAccessReviewTest{
			description:     "who can view deploymentconfigs in mallet by cluster-admin",
			clientInterface: clusterAdminAuthorizationClient.LocalResourceAccessReviews("mallet-project"),
			review:          localRequestWhoCanViewDeploymentConfigs,
			response: authorizationapi.ResourceAccessReviewResponse{
				Users:           sets.NewString("edgar"),
				Groups:          sets.NewString(),
				Namespace:       "mallet-project",
				EvaluationError: `[clusterrole.rbac.authorization.k8s.io "admin" not found, clusterrole.rbac.authorization.k8s.io "admin" not found, clusterrole.rbac.authorization.k8s.io "admin" not found]`,
			},
		}
		test.response.Users.Insert(globalClusterReaderUsers.List()...)
		test.response.Users.Insert(globalDeploymentConfigGetterUsers.List()...)
		test.response.Users.Delete("system:serviceaccount:openshift-infra:template-instance-controller")
		test.response.Users.Delete("system:serviceaccount:openshift-infra:template-instance-finalizer-controller")
		test.response.Groups.Insert(globalClusterReaderGroups.List()...)
		test.run(t)
	}
}

type subjectAccessReviewTest struct {
	description      string
	localInterface   authorizationtypedclient.LocalSubjectAccessReviewInterface
	clusterInterface authorizationtypedclient.SubjectAccessReviewInterface
	localReview      *authorizationapi.LocalSubjectAccessReview
	clusterReview    *authorizationapi.SubjectAccessReview

	kubeNamespace     string
	kubeErr           string
	kubeSkip          bool
	kubeAuthInterface internalversion.AuthorizationInterface

	response authorizationapi.SubjectAccessReviewResponse
	err      string
}

func (test subjectAccessReviewTest) run(t *testing.T) {
	{
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

	if test.kubeAuthInterface != nil {
		var testNS string
		if test.localReview != nil {
			switch {
			case len(test.localReview.Namespace) > 0:
				testNS = test.localReview.Namespace
			case len(test.response.Namespace) > 0:
				testNS = test.response.Namespace
			case len(test.kubeNamespace) > 0:
				testNS = test.kubeNamespace
			default:
				t.Errorf("%s: no valid namespace found for kube auth test", test.description)
				return
			}
		}

		failMessage := ""
		err := wait.Poll(testutil.PolicyCachePollInterval, testutil.PolicyCachePollTimeout, func() (bool, error) {
			var err error
			var actualResponse kubeauthorizationapi.SubjectAccessReviewStatus
			if test.localReview != nil {
				if len(test.localReview.User) == 0 && (test.localReview.Groups == nil || len(test.localReview.Groups.UnsortedList()) == 0) {
					var tmp *kubeauthorizationapi.SelfSubjectAccessReview
					if tmp, err = test.kubeAuthInterface.SelfSubjectAccessReviews().Create(toKubeSelfSAR(testNS, test.localReview)); err == nil {
						actualResponse = tmp.Status
					}
				} else {
					var tmp *kubeauthorizationapi.LocalSubjectAccessReview
					if tmp, err = test.kubeAuthInterface.LocalSubjectAccessReviews(testNS).Create(toKubeLocalSAR(testNS, test.localReview)); err == nil {
						actualResponse = tmp.Status
					}
				}
			} else {
				var tmp *kubeauthorizationapi.SubjectAccessReview
				if tmp, err = test.kubeAuthInterface.SubjectAccessReviews().Create(toKubeClusterSAR(test.clusterReview)); err == nil {
					actualResponse = tmp.Status
				}
			}
			testErr := test.kubeErr
			if len(testErr) == 0 {
				testErr = test.err
			}
			if len(testErr) > 0 {
				if err == nil {
					failMessage = fmt.Sprintf("%s: Expected error: %v\ngot\n\t%#v", test.description, testErr, actualResponse)
					return false, nil
				} else if !strings.HasPrefix(err.Error(), testErr) {
					failMessage = fmt.Sprintf("%s: expected\n\t%v\ngot\n\t%v", test.description, testErr, err)
					return false, nil
				}
			} else {
				if err != nil {
					failMessage = fmt.Sprintf("%s: unexpected error: %v", test.description, err)
					return false, nil
				}
			}

			if (actualResponse.Allowed != test.response.Allowed) || (!strings.HasPrefix(actualResponse.Reason, test.response.Reason)) {
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
	} else if !test.kubeSkip {
		t.Errorf("%s: missing kube auth interface and test is not whitelisted", test.description)
	}
}

// TODO handle Subresource and NonResourceAttributes
func toKubeSelfSAR(testNS string, sar *authorizationapi.LocalSubjectAccessReview) *kubeauthorizationapi.SelfSubjectAccessReview {
	return &kubeauthorizationapi.SelfSubjectAccessReview{
		Spec: kubeauthorizationapi.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &kubeauthorizationapi.ResourceAttributes{
				Namespace: testNS,
				Verb:      sar.Verb,
				Group:     sar.Group,
				Version:   sar.Version,
				Resource:  sar.Resource,
				Name:      sar.ResourceName,
			},
		},
	}
}

// TODO handle Extra/Scopes, Subresource and NonResourceAttributes
func toKubeLocalSAR(testNS string, sar *authorizationapi.LocalSubjectAccessReview) *kubeauthorizationapi.LocalSubjectAccessReview {
	return &kubeauthorizationapi.LocalSubjectAccessReview{
		ObjectMeta: metav1.ObjectMeta{Namespace: testNS},
		Spec: kubeauthorizationapi.SubjectAccessReviewSpec{
			User:   sar.User,
			Groups: sar.Groups.List(),
			ResourceAttributes: &kubeauthorizationapi.ResourceAttributes{
				Namespace: testNS,
				Verb:      sar.Verb,
				Group:     sar.Group,
				Version:   sar.Version,
				Resource:  sar.Resource,
				Name:      sar.ResourceName,
			},
		},
	}
}

// TODO handle Extra/Scopes, Subresource and NonResourceAttributes
func toKubeClusterSAR(sar *authorizationapi.SubjectAccessReview) *kubeauthorizationapi.SubjectAccessReview {
	return &kubeauthorizationapi.SubjectAccessReview{
		Spec: kubeauthorizationapi.SubjectAccessReviewSpec{
			User:   sar.User,
			Groups: sar.Groups.List(),
			ResourceAttributes: &kubeauthorizationapi.ResourceAttributes{
				Verb:     sar.Verb,
				Group:    sar.Group,
				Version:  sar.Version,
				Resource: sar.Resource,
				Name:     sar.ResourceName,
			},
		},
	}
}

func TestAuthorizationSubjectAccessReviewAPIGroup(t *testing.T) {
	masterConfig, clusterAdminKubeConfig, err := testserver.StartTestMaster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer testserver.CleanupMasterEtcd(t, masterConfig)

	clusterAdminKubeClient, err := testutil.GetClusterAdminKubeInternalClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	clusterAdminSARGetter := clusterAdminKubeClient.Authorization()

	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	clusterAdminAuthorizationClient := authorizationclient.NewForConfigOrDie(clusterAdminClientConfig).Authorization()

	_, _, err = testserver.CreateNewProject(clusterAdminClientConfig, "hammer-project", "harold")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// SAR honors API Group
	subjectAccessReviewTest{
		description:    "cluster admin told harold can get autoscaling.horizontalpodautoscalers in project hammer-project",
		localInterface: clusterAdminAuthorizationClient.LocalSubjectAccessReviews("hammer-project"),
		localReview: &authorizationapi.LocalSubjectAccessReview{
			User:   "harold",
			Action: authorizationapi.Action{Verb: "get", Group: "autoscaling", Resource: "horizontalpodautoscalers"},
		},
		kubeAuthInterface: clusterAdminSARGetter,
		response: authorizationapi.SubjectAccessReviewResponse{
			Allowed:   true,
			Reason:    `RBAC: allowed by RoleBinding "admin/hammer-project" of ClusterRole "admin" to User "harold"`,
			Namespace: "hammer-project",
		},
	}.run(t)
	subjectAccessReviewTest{
		description:    "cluster admin told harold cannot get horizontalpodautoscalers (with no API group) in project hammer-project",
		localInterface: clusterAdminAuthorizationClient.LocalSubjectAccessReviews("hammer-project"),
		localReview: &authorizationapi.LocalSubjectAccessReview{
			User:   "harold",
			Action: authorizationapi.Action{Verb: "get", Group: "", Resource: "horizontalpodautoscalers"},
		},
		kubeAuthInterface: clusterAdminSARGetter,
		response: authorizationapi.SubjectAccessReviewResponse{
			Allowed:   false,
			Reason:    `no RBAC policy matched`,
			Namespace: "hammer-project",
		},
	}.run(t)
	subjectAccessReviewTest{
		description:    "cluster admin told harold cannot get horizontalpodautoscalers (with invalid API group) in project hammer-project",
		localInterface: clusterAdminAuthorizationClient.LocalSubjectAccessReviews("hammer-project"),
		localReview: &authorizationapi.LocalSubjectAccessReview{
			User:   "harold",
			Action: authorizationapi.Action{Verb: "get", Group: "foo", Resource: "horizontalpodautoscalers"},
		},
		kubeAuthInterface: clusterAdminKubeClient.Authorization(),
		response: authorizationapi.SubjectAccessReviewResponse{
			Allowed:   false,
			Reason:    `no RBAC policy matched`,
			Namespace: "hammer-project",
		},
	}.run(t)
	subjectAccessReviewTest{
		description:    "cluster admin told harold cannot get horizontalpodautoscalers (with * API group) in project hammer-project",
		localInterface: clusterAdminAuthorizationClient.LocalSubjectAccessReviews("hammer-project"),
		localReview: &authorizationapi.LocalSubjectAccessReview{
			User:   "harold",
			Action: authorizationapi.Action{Verb: "get", Group: "*", Resource: "horizontalpodautoscalers"},
		},
		kubeAuthInterface: clusterAdminSARGetter,
		response: authorizationapi.SubjectAccessReviewResponse{
			Allowed:   false,
			Reason:    `no RBAC policy matched`,
			Namespace: "hammer-project",
		},
	}.run(t)

	// SAR honors API Group for cluster admin self SAR
	subjectAccessReviewTest{
		description:    "cluster admin told they can get autoscaling.horizontalpodautoscalers in project hammer-project",
		localInterface: clusterAdminAuthorizationClient.LocalSubjectAccessReviews("any-project"),
		localReview: &authorizationapi.LocalSubjectAccessReview{
			Action: authorizationapi.Action{Verb: "get", Group: "autoscaling", Resource: "horizontalpodautoscalers"},
		},
		kubeAuthInterface: clusterAdminSARGetter,
		response: authorizationapi.SubjectAccessReviewResponse{
			Allowed:   true,
			Reason:    "",
			Namespace: "any-project",
		},
	}.run(t)
	subjectAccessReviewTest{
		description:    "cluster admin told they can get horizontalpodautoscalers (with no API group) in project any-project",
		localInterface: clusterAdminAuthorizationClient.LocalSubjectAccessReviews("any-project"),
		localReview: &authorizationapi.LocalSubjectAccessReview{
			Action: authorizationapi.Action{Verb: "get", Group: "", Resource: "horizontalpodautoscalers"},
		},
		kubeAuthInterface: clusterAdminSARGetter,
		response: authorizationapi.SubjectAccessReviewResponse{
			Allowed:   true,
			Reason:    "",
			Namespace: "any-project",
		},
	}.run(t)
	subjectAccessReviewTest{
		description:    "cluster admin told they can get horizontalpodautoscalers (with invalid API group) in project any-project",
		localInterface: clusterAdminAuthorizationClient.LocalSubjectAccessReviews("any-project"),
		localReview: &authorizationapi.LocalSubjectAccessReview{
			Action: authorizationapi.Action{Verb: "get", Group: "foo", Resource: "horizontalpodautoscalers"},
		},
		kubeAuthInterface: clusterAdminSARGetter,
		response: authorizationapi.SubjectAccessReviewResponse{
			Allowed:   true,
			Reason:    "",
			Namespace: "any-project",
		},
	}.run(t)
	subjectAccessReviewTest{
		description:    "cluster admin told they can get horizontalpodautoscalers (with * API group) in project any-project",
		localInterface: clusterAdminAuthorizationClient.LocalSubjectAccessReviews("any-project"),
		localReview: &authorizationapi.LocalSubjectAccessReview{
			Action: authorizationapi.Action{Verb: "get", Group: "*", Resource: "horizontalpodautoscalers"},
		},
		kubeAuthInterface: clusterAdminSARGetter,
		response: authorizationapi.SubjectAccessReviewResponse{
			Allowed:   true,
			Reason:    "",
			Namespace: "any-project",
		},
	}.run(t)
}

func TestAuthorizationSubjectAccessReview(t *testing.T) {
	masterConfig, clusterAdminKubeConfig, err := testserver.StartTestMasterAPI()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer testserver.CleanupMasterEtcd(t, masterConfig)

	clusterAdminKubeClient, err := testutil.GetClusterAdminKubeInternalClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	clusterAdminLocalSARGetter := clusterAdminKubeClient.Authorization()

	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	clusterAdminAuthorizationClient := authorizationclient.NewForConfigOrDie(clusterAdminClientConfig).Authorization()

	_, haroldConfig, err := testserver.CreateNewProject(clusterAdminClientConfig, "hammer-project", "harold")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	haroldKubeClient, _, err := testutil.GetClientForUser(clusterAdminClientConfig, "harold")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	haroldAuthorizationClient := authorizationclient.NewForConfigOrDie(haroldConfig).Authorization()
	haroldSARGetter := haroldKubeClient.Authorization()

	_, markConfig, err := testserver.CreateNewProject(clusterAdminClientConfig, "mallet-project", "mark")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	markKubeClient, _, err := testutil.GetClientForUser(clusterAdminClientConfig, "mark")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	markAuthorizationClient := authorizationclient.NewForConfigOrDie(markConfig).Authorization()
	markSARGetter := markKubeClient.Authorization()

	dannyKubeClient, dannyConfig, err := testutil.GetClientForUser(clusterAdminClientConfig, "danny")
	if err != nil {
		t.Fatalf("error requesting token: %v", err)
	}
	dannyAuthorizationClient := authorizationclient.NewForConfigOrDie(dannyConfig).Authorization()
	dannySARGetter := dannyKubeClient.Authorization()

	anonymousConfig := rest.AnonymousClientConfig(clusterAdminClientConfig)
	anonymousAuthorizationClient := authorizationclient.NewForConfigOrDie(anonymousConfig).Authorization()
	anonymousKubeClient, err := kclientset.NewForConfig(anonymousConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	anonymousSARGetter := anonymousKubeClient.Authorization()

	addAnonymous := &policy.RoleModificationOptions{
		RoleBindingNamespace: "hammer-project",
		RoleName:             bootstrappolicy.EditRoleName,
		RoleKind:             "ClusterRole",
		RbacClient:           rbacv1client.NewForConfigOrDie(clusterAdminClientConfig),
		Users:                []string{"system:anonymous"},
	}
	if err := addAnonymous.AddRole(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	addDanny := &policy.RoleModificationOptions{
		RoleBindingNamespace: "default",
		RoleName:             bootstrappolicy.ViewRoleName,
		RoleKind:             "ClusterRole",
		RbacClient:           rbacv1client.NewForConfigOrDie(clusterAdminClientConfig),
		Users:                []string{"danny"},
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
		localInterface: clusterAdminAuthorizationClient.LocalSubjectAccessReviews("default"),
		localReview: &authorizationapi.LocalSubjectAccessReview{
			User:   "danny",
			Action: authorizationapi.Action{Verb: "get", Resource: "projects"},
		},
		kubeAuthInterface: clusterAdminLocalSARGetter,
		response: authorizationapi.SubjectAccessReviewResponse{
			Allowed:   true,
			Reason:    `RBAC: allowed by RoleBinding "view/default" of ClusterRole "view" to User "danny"`,
			Namespace: "default",
		},
	}.run(t)
	subjectAccessReviewTest{
		description:       "cluster admin told danny cannot get projects cluster-wide",
		clusterInterface:  clusterAdminAuthorizationClient.SubjectAccessReviews(),
		clusterReview:     askCanDannyGetProject,
		kubeAuthInterface: clusterAdminLocalSARGetter,
		response: authorizationapi.SubjectAccessReviewResponse{
			Allowed:   false,
			Reason:    `no RBAC policy matched`,
			Namespace: "",
		},
	}.run(t)
	subjectAccessReviewTest{
		description:       "as danny, can I make cluster subject access reviews",
		clusterInterface:  dannyAuthorizationClient.SubjectAccessReviews(),
		clusterReview:     askCanDannyGetProject,
		kubeAuthInterface: dannySARGetter,
		err:               `subjectaccessreviews.authorization.openshift.io is forbidden: User "danny" cannot create subjectaccessreviews.authorization.openshift.io at the cluster scope`,
		kubeErr:           `subjectaccessreviews.authorization.k8s.io is forbidden: User "danny" cannot create subjectaccessreviews.authorization.k8s.io at the cluster scope`,
	}.run(t)
	subjectAccessReviewTest{
		description:       "as anonymous, can I make cluster subject access reviews",
		clusterInterface:  anonymousAuthorizationClient.SubjectAccessReviews(),
		clusterReview:     askCanDannyGetProject,
		kubeAuthInterface: anonymousSARGetter,
		err:               `subjectaccessreviews.authorization.openshift.io is forbidden: User "system:anonymous" cannot create subjectaccessreviews.authorization.openshift.io at the cluster scope`,
		kubeErr:           `subjectaccessreviews.authorization.k8s.io is forbidden: User "system:anonymous" cannot create subjectaccessreviews.authorization.k8s.io at the cluster scope`,
	}.run(t)

	addValerie := &policy.RoleModificationOptions{
		RoleBindingNamespace: "hammer-project",
		RoleName:             bootstrappolicy.ViewRoleName,
		RoleKind:             "ClusterRole",
		RbacClient:           rbacv1client.NewForConfigOrDie(haroldConfig),
		Users:                []string{"valerie"},
	}
	if err := addValerie.AddRole(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	addEdgar := &policy.RoleModificationOptions{
		RoleBindingNamespace: "mallet-project",
		RoleName:             bootstrappolicy.EditRoleName,
		RoleKind:             "ClusterRole",
		RbacClient:           rbacv1client.NewForConfigOrDie(markConfig),
		Users:                []string{"edgar"},
	}
	if err := addEdgar.AddRole(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	askCanValerieGetProject := &authorizationapi.LocalSubjectAccessReview{
		User:   "valerie",
		Action: authorizationapi.Action{Verb: "get", Resource: "projects"},
	}
	subjectAccessReviewTest{
		description:       "harold told valerie can get project hammer-project",
		localInterface:    haroldAuthorizationClient.LocalSubjectAccessReviews("hammer-project"),
		localReview:       askCanValerieGetProject,
		kubeAuthInterface: haroldSARGetter,
		response: authorizationapi.SubjectAccessReviewResponse{
			Allowed:   true,
			Reason:    `RBAC: allowed by RoleBinding "view/hammer-project" of ClusterRole "view" to User "valerie"`,
			Namespace: "hammer-project",
		},
	}.run(t)
	subjectAccessReviewTest{
		description:       "mark told valerie cannot get project mallet-project",
		localInterface:    markAuthorizationClient.LocalSubjectAccessReviews("mallet-project"),
		localReview:       askCanValerieGetProject,
		kubeAuthInterface: markSARGetter,
		response: authorizationapi.SubjectAccessReviewResponse{
			Allowed:   false,
			Reason:    `no RBAC policy matched`,
			Namespace: "mallet-project",
		},
	}.run(t)

	askCanEdgarDeletePods := &authorizationapi.LocalSubjectAccessReview{
		User:   "edgar",
		Action: authorizationapi.Action{Verb: "delete", Resource: "pods"},
	}
	subjectAccessReviewTest{
		description:       "mark told edgar can delete pods in mallet-project",
		localInterface:    markAuthorizationClient.LocalSubjectAccessReviews("mallet-project"),
		localReview:       askCanEdgarDeletePods,
		kubeAuthInterface: markSARGetter,
		response: authorizationapi.SubjectAccessReviewResponse{
			Allowed:   true,
			Reason:    `RBAC: allowed by RoleBinding "edit/mallet-project" of ClusterRole "edit" to User "edgar"`,
			Namespace: "mallet-project",
		},
	}.run(t)
	// ensure unprivileged users cannot check other users' access
	subjectAccessReviewTest{
		description:       "harold denied ability to run subject access review in project mallet-project",
		localInterface:    haroldAuthorizationClient.LocalSubjectAccessReviews("mallet-project"),
		localReview:       askCanEdgarDeletePods,
		kubeAuthInterface: haroldSARGetter,
		kubeNamespace:     "mallet-project",
		err:               `localsubjectaccessreviews.authorization.openshift.io is forbidden: User "harold" cannot create localsubjectaccessreviews.authorization.openshift.io in the namespace "mallet-project"`,
		kubeErr:           `localsubjectaccessreviews.authorization.k8s.io is forbidden: User "harold" cannot create localsubjectaccessreviews.authorization.k8s.io in the namespace "mallet-project"`,
	}.run(t)
	subjectAccessReviewTest{
		description:       "system:anonymous denied ability to run subject access review in project mallet-project",
		localInterface:    anonymousAuthorizationClient.LocalSubjectAccessReviews("mallet-project"),
		localReview:       askCanEdgarDeletePods,
		kubeAuthInterface: anonymousSARGetter,
		kubeNamespace:     "mallet-project",
		err:               `localsubjectaccessreviews.authorization.openshift.io is forbidden: User "system:anonymous" cannot create localsubjectaccessreviews.authorization.openshift.io in the namespace "mallet-project"`,
		kubeErr:           `localsubjectaccessreviews.authorization.k8s.io is forbidden: User "system:anonymous" cannot create localsubjectaccessreviews.authorization.k8s.io in the namespace "mallet-project"`,
	}.run(t)
	// ensure message does not leak whether the namespace exists or not
	subjectAccessReviewTest{
		description:       "harold denied ability to run subject access review in project nonexistent-project",
		localInterface:    haroldAuthorizationClient.LocalSubjectAccessReviews("nonexistent-project"),
		localReview:       askCanEdgarDeletePods,
		kubeAuthInterface: haroldSARGetter,
		kubeNamespace:     "nonexistent-project",
		err:               `localsubjectaccessreviews.authorization.openshift.io is forbidden: User "harold" cannot create localsubjectaccessreviews.authorization.openshift.io in the namespace "nonexistent-project"`,
		kubeErr:           `localsubjectaccessreviews.authorization.k8s.io is forbidden: User "harold" cannot create localsubjectaccessreviews.authorization.k8s.io in the namespace "nonexistent-project"`,
	}.run(t)
	subjectAccessReviewTest{
		description:       "system:anonymous denied ability to run subject access review in project nonexistent-project",
		localInterface:    anonymousAuthorizationClient.LocalSubjectAccessReviews("nonexistent-project"),
		localReview:       askCanEdgarDeletePods,
		kubeAuthInterface: anonymousSARGetter,
		kubeNamespace:     "nonexistent-project",
		err:               `localsubjectaccessreviews.authorization.openshift.io is forbidden: User "system:anonymous" cannot create localsubjectaccessreviews.authorization.openshift.io in the namespace "nonexistent-project"`,
		kubeErr:           `localsubjectaccessreviews.authorization.k8s.io is forbidden: User "system:anonymous" cannot create localsubjectaccessreviews.authorization.k8s.io in the namespace "nonexistent-project"`,
	}.run(t)

	askCanHaroldUpdateProject := &authorizationapi.LocalSubjectAccessReview{
		User:   "harold",
		Action: authorizationapi.Action{Verb: "update", Resource: "projects"},
	}
	subjectAccessReviewTest{
		description:       "harold told harold can update project hammer-project",
		localInterface:    haroldAuthorizationClient.LocalSubjectAccessReviews("hammer-project"),
		localReview:       askCanHaroldUpdateProject,
		kubeAuthInterface: haroldSARGetter,
		response: authorizationapi.SubjectAccessReviewResponse{
			Allowed:   true,
			Reason:    `RBAC: allowed by RoleBinding "admin/hammer-project" of ClusterRole "admin" to User "harold"`,
			Namespace: "hammer-project",
		},
	}.run(t)

	askCanClusterAdminsCreateProject := &authorizationapi.SubjectAccessReview{
		Groups: sets.NewString("system:cluster-admins"),
		Action: authorizationapi.Action{Verb: "create", Resource: "projects"},
	}
	subjectAccessReviewTest{
		description:       "cluster admin told cluster admins can create projects",
		clusterInterface:  clusterAdminAuthorizationClient.SubjectAccessReviews(),
		clusterReview:     askCanClusterAdminsCreateProject,
		kubeAuthInterface: clusterAdminLocalSARGetter,
		response: authorizationapi.SubjectAccessReviewResponse{
			Allowed:   true,
			Reason:    `RBAC: allowed by ClusterRoleBinding "cluster-admins" of ClusterRole "cluster-admin" to Group "system:cluster-admins"`,
			Namespace: "",
		},
	}.run(t)
	subjectAccessReviewTest{
		description:       "harold denied ability to run cluster subject access review",
		clusterInterface:  haroldAuthorizationClient.SubjectAccessReviews(),
		clusterReview:     askCanClusterAdminsCreateProject,
		kubeAuthInterface: haroldSARGetter,
		err:               `subjectaccessreviews.authorization.openshift.io is forbidden: User "harold" cannot create subjectaccessreviews.authorization.openshift.io at the cluster scope`,
		kubeErr:           `subjectaccessreviews.authorization.k8s.io is forbidden: User "harold" cannot create subjectaccessreviews.authorization.k8s.io at the cluster scope`,
	}.run(t)

	askCanICreatePods := &authorizationapi.LocalSubjectAccessReview{
		Action: authorizationapi.Action{Verb: "create", Resource: "pods"},
	}
	subjectAccessReviewTest{
		description:       "harold told he can create pods in project hammer-project",
		localInterface:    haroldAuthorizationClient.LocalSubjectAccessReviews("hammer-project"),
		localReview:       askCanICreatePods,
		kubeAuthInterface: haroldSARGetter,
		response: authorizationapi.SubjectAccessReviewResponse{
			Allowed:   true,
			Reason:    `RBAC: allowed by RoleBinding "admin/hammer-project" of ClusterRole "admin" to User "harold"`,
			Namespace: "hammer-project",
		},
	}.run(t)
	subjectAccessReviewTest{
		description:       "system:anonymous told he can create pods in project hammer-project",
		localInterface:    anonymousAuthorizationClient.LocalSubjectAccessReviews("hammer-project"),
		localReview:       askCanICreatePods,
		kubeAuthInterface: anonymousSARGetter,
		response: authorizationapi.SubjectAccessReviewResponse{
			Allowed:   true,
			Reason:    `RBAC: allowed by RoleBinding "edit/hammer-project" of ClusterRole "edit" to User "system:anonymous"`,
			Namespace: "hammer-project",
		},
	}.run(t)

	// test checking self permissions when denied
	subjectAccessReviewTest{
		description:       "harold told he cannot create pods in project mallet-project",
		localInterface:    haroldAuthorizationClient.LocalSubjectAccessReviews("mallet-project"),
		localReview:       askCanICreatePods,
		kubeAuthInterface: haroldSARGetter,
		response: authorizationapi.SubjectAccessReviewResponse{
			Allowed:   false,
			Reason:    `no RBAC policy matched`,
			Namespace: "mallet-project",
		},
	}.run(t)
	subjectAccessReviewTest{
		description:       "system:anonymous told he cannot create pods in project mallet-project",
		localInterface:    anonymousAuthorizationClient.LocalSubjectAccessReviews("mallet-project"),
		localReview:       askCanICreatePods,
		kubeAuthInterface: anonymousSARGetter,
		response: authorizationapi.SubjectAccessReviewResponse{
			Allowed:   false,
			Reason:    `no RBAC policy matched`,
			Namespace: "mallet-project",
		},
	}.run(t)

	// test checking self-permissions doesn't leak whether namespace exists or not
	// We carry a patch to allow this
	subjectAccessReviewTest{
		description:       "harold told he cannot create pods in project nonexistent-project",
		localInterface:    haroldAuthorizationClient.LocalSubjectAccessReviews("nonexistent-project"),
		localReview:       askCanICreatePods,
		kubeAuthInterface: haroldSARGetter,
		response: authorizationapi.SubjectAccessReviewResponse{
			Allowed:   false,
			Reason:    `no RBAC policy matched`,
			Namespace: "nonexistent-project",
		},
	}.run(t)
	subjectAccessReviewTest{
		description:       "system:anonymous told he cannot create pods in project nonexistent-project",
		localInterface:    anonymousAuthorizationClient.LocalSubjectAccessReviews("nonexistent-project"),
		localReview:       askCanICreatePods,
		kubeAuthInterface: anonymousSARGetter,
		response: authorizationapi.SubjectAccessReviewResponse{
			Allowed:   false,
			Reason:    `no RBAC policy matched`,
			Namespace: "nonexistent-project",
		},
	}.run(t)

	askCanICreatePolicyBindings := &authorizationapi.LocalSubjectAccessReview{
		Action: authorizationapi.Action{Verb: "create", Resource: "policybindings"},
	}
	subjectAccessReviewTest{
		description:       "harold told he can create policybindings in project hammer-project",
		localInterface:    haroldAuthorizationClient.LocalSubjectAccessReviews("hammer-project"),
		kubeAuthInterface: haroldSARGetter,
		localReview:       askCanICreatePolicyBindings,
		response: authorizationapi.SubjectAccessReviewResponse{
			Allowed:   false,
			Reason:    `no RBAC policy matched`,
			Namespace: "hammer-project",
		},
	}.run(t)
}

func TestBrowserSafeAuthorizer(t *testing.T) {
	masterConfig, clusterAdminKubeConfig, err := testserver.StartTestMasterAPI()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer testserver.CleanupMasterEtcd(t, masterConfig)

	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// this client has an API token so it is safe
	userClient, _, err := testutil.GetClientForUser(clusterAdminClientConfig, "user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// this client has no API token so it is unsafe (like a browser)
	anonymousConfig := rest.AnonymousClientConfig(clusterAdminClientConfig)
	anonymousConfig.ContentConfig.GroupVersion = &schema.GroupVersion{}
	anonymousConfig.ContentConfig.NegotiatedSerializer = legacyscheme.Codecs
	anonymousClient, err := rest.RESTClientFor(anonymousConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	proxyVerb := []string{"api", "v1", "proxy", "namespaces", "ns", "pods", "podX1:8080"}
	proxySubresource := []string{"api", "v1", "namespaces", "ns", "pods", "podX1:8080", "proxy", "appEndPoint"}

	isUnsafeErr := func(errProxy error) (matches bool) {
		if errProxy == nil {
			return false
		}
		return strings.Contains(errProxy.Error(), `cannot proxy pods in the namespace "ns": proxy verb changed to unsafeproxy`) ||
			strings.Contains(errProxy.Error(), `cannot get pods/proxy in the namespace "ns": proxy subresource changed to unsafeproxy`)
	}

	for _, tc := range []struct {
		name   string
		client rest.Interface
		path   []string

		expectUnsafe bool
	}{
		{
			name:   "safe to proxy verb",
			client: userClient.Core().RESTClient(),
			path:   proxyVerb,

			expectUnsafe: false,
		},
		{
			name:   "safe to proxy subresource",
			client: userClient.Core().RESTClient(),
			path:   proxySubresource,

			expectUnsafe: false,
		},
		{
			name:   "unsafe to proxy verb",
			client: anonymousClient,
			path:   proxyVerb,

			expectUnsafe: true,
		},
		{
			name:   "unsafe to proxy subresource",
			client: anonymousClient,
			path:   proxySubresource,

			expectUnsafe: true,
		},
	} {
		errProxy := tc.client.Get().AbsPath(tc.path...).Do().Error()
		if errProxy == nil || !kapierror.IsForbidden(errProxy) || tc.expectUnsafe != isUnsafeErr(errProxy) {
			t.Errorf("%s: expected forbidden error on GET %s, got %#v (isForbidden=%v, expectUnsafe=%v, actualUnsafe=%v)",
				tc.name, tc.path, errProxy, kapierror.IsForbidden(errProxy), tc.expectUnsafe, isUnsafeErr(errProxy))
		}
	}
}

// TestLegacyLocalRoleBindingEndpoint exercises the legacy rolebinding endpoint that is proxied to rbac
func TestLegacyLocalRoleBindingEndpoint(t *testing.T) {
	masterConfig, clusterAdminKubeConfig, err := testserver.StartTestMasterAPI()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer testserver.CleanupMasterEtcd(t, masterConfig)

	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	clusterAdmin := authorizationclient.NewForConfigOrDie(clusterAdminClientConfig)

	namespace := "testproject"
	_, _, err = testserver.CreateNewProject(clusterAdminClientConfig, namespace, "testuser")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	roleBindingsPath := "/oapi/v1/namespaces/" + namespace + "/rolebindings"
	testBindingName := "testrole"

	// install the legacy types into the client for decoding
	legacy.InstallInternalLegacyAuthorization(authorizationclientscheme.Scheme)

	// create rolebinding
	roleBindingToCreate := &authorizationapi.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: testBindingName,
		},
		Subjects: []kapi.ObjectReference{
			{
				Kind: authorizationapi.UserKind,
				Name: "testuser",
			},
		},
		RoleRef: kapi.ObjectReference{
			Kind:      "Role",
			Name:      "edit",
			Namespace: namespace,
		},
	}
	roleBindingToCreateBytes, err := runtime.Encode(legacyscheme.Codecs.LegacyCodec(schema.GroupVersion{Version: "v1"}), roleBindingToCreate)
	if err != nil {
		t.Fatal(err)
	}

	roleBindingCreated := &authorizationapi.RoleBinding{}
	err = clusterAdmin.Authorization().RESTClient().Post().AbsPath(roleBindingsPath).Body(roleBindingToCreateBytes).Do().Into(roleBindingCreated)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if roleBindingCreated.Name != roleBindingToCreate.Name {
		t.Errorf("expected rolebinding %s, got %s", roleBindingToCreate.Name, roleBindingCreated.Name)
	}

	// list rolebindings
	roleBindingList := &authorizationapi.RoleBindingList{}
	err = clusterAdmin.Authorization().RESTClient().Get().AbsPath(roleBindingsPath).Do().Into(roleBindingList)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	checkBindings := sets.String{}
	for _, rb := range roleBindingList.Items {
		checkBindings.Insert(rb.Name)
	}

	// check for the created rolebinding in the list
	if !checkBindings.HasAll(testBindingName) {
		t.Errorf("rolebinding list does not have the expected bindings")
	}

	// edit rolebinding
	roleBindingToEdit := &authorizationapi.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: testBindingName,
		},
		Subjects: []kapi.ObjectReference{
			{
				Kind: authorizationapi.UserKind,
				Name: "testuser",
			},
			{
				Kind: authorizationapi.UserKind,
				Name: "testuser2",
			},
		},
		RoleRef: kapi.ObjectReference{
			Kind:      "Role",
			Name:      "edit",
			Namespace: namespace,
		},
	}
	roleBindingToEditBytes, err := runtime.Encode(legacyscheme.Codecs.LegacyCodec(schema.GroupVersion{Version: "v1"}), roleBindingToEdit)
	if err != nil {
		t.Fatal(err)
	}

	roleBindingEdited := &authorizationapi.RoleBinding{}
	err = clusterAdmin.Authorization().RESTClient().Patch(types.StrategicMergePatchType).AbsPath(roleBindingsPath).Name(roleBindingToEdit.Name).Body(roleBindingToEditBytes).Do().Into(roleBindingEdited)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if roleBindingEdited.Name != roleBindingToEdit.Name {
		t.Errorf("expected rolebinding %s, got %s", roleBindingToEdit.Name, roleBindingEdited.Name)
	}

	checkSubjects := sets.String{}
	for _, subj := range roleBindingEdited.Subjects {
		checkSubjects.Insert(subj.Name)
	}
	if !checkSubjects.HasAll("testuser", "testuser2") {
		t.Errorf("rolebinding not edited")
	}

	// get rolebinding by name
	getRoleBinding := &authorizationapi.RoleBinding{}
	err = clusterAdmin.Authorization().RESTClient().Get().AbsPath(roleBindingsPath).Name(testBindingName).Do().Into(getRoleBinding)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if getRoleBinding.Name != testBindingName {
		t.Errorf("expected rolebinding %s, got %s", testBindingName, getRoleBinding.Name)
	}

	// delete rolebinding
	err = clusterAdmin.Authorization().RESTClient().Delete().AbsPath(roleBindingsPath).Name(testBindingName).Do().Error()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// confirm deletion
	getRoleBinding = &authorizationapi.RoleBinding{}
	err = clusterAdmin.Authorization().RESTClient().Get().AbsPath(roleBindingsPath).Name(testBindingName).Do().Into(getRoleBinding)
	if err == nil {
		t.Errorf("expected error")
	} else if !kapierror.IsNotFound(err) {
		t.Errorf("unexpected error: %v", err)
	}

	// create local rolebinding for cluster role
	localClusterRoleBindingToCreate := &authorizationapi.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-crb",
			Namespace: namespace,
		},
		Subjects: []kapi.ObjectReference{
			{
				Kind: authorizationapi.UserKind,
				Name: "testuser",
			},
		},
		RoleRef: kapi.ObjectReference{
			Kind: "ClusterRole",
			Name: "edit",
		},
	}
	localClusterRoleBindingToCreateBytes, err := runtime.Encode(legacyscheme.Codecs.LegacyCodec(schema.GroupVersion{Version: "v1"}), localClusterRoleBindingToCreate)
	if err != nil {
		t.Fatal(err)
	}

	localClusterRoleBindingCreated := &authorizationapi.RoleBinding{}
	err = clusterAdmin.Authorization().RESTClient().Post().AbsPath(roleBindingsPath).Body(localClusterRoleBindingToCreateBytes).Do().Into(localClusterRoleBindingCreated)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if localClusterRoleBindingCreated.Name != localClusterRoleBindingToCreate.Name {
		t.Errorf("expected clusterrolebinding %s, got %s", localClusterRoleBindingToCreate.Name, localClusterRoleBindingCreated.Name)
	}

}

// TestLegacyClusterRoleBindingEndpoint exercises the legacy clusterrolebinding endpoint that is proxied to rbac
func TestLegacyClusterRoleBindingEndpoint(t *testing.T) {
	masterConfig, clusterAdminKubeConfig, err := testserver.StartTestMasterAPI()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer testserver.CleanupMasterEtcd(t, masterConfig)

	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	clusterAdmin := authorizationclient.NewForConfigOrDie(clusterAdminClientConfig)

	// install the legacy types into the client for decoding
	legacy.InstallInternalLegacyAuthorization(authorizationclientscheme.Scheme)

	clusterRoleBindingsPath := "/oapi/v1/clusterrolebindings"
	testBindingName := "testbinding"

	// list clusterrole bindings
	clusterRoleBindingList := &authorizationapi.ClusterRoleBindingList{}
	err = clusterAdmin.Authorization().RESTClient().Get().AbsPath(clusterRoleBindingsPath).Do().Into(clusterRoleBindingList)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	checkBindings := sets.String{}
	for _, rb := range clusterRoleBindingList.Items {
		checkBindings.Insert(rb.Name)
	}

	// ensure there are at least some of the expected bindings in the list
	if !checkBindings.HasAll("basic-users", "cluster-admin", "cluster-admins", "cluster-readers") {
		t.Errorf("clusterrolebinding list does not have the expected bindings")
	}

	// create clusterrole binding
	clusterRoleBindingToCreate := &authorizationapi.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: testBindingName,
		},
		Subjects: []kapi.ObjectReference{
			{
				Kind: authorizationapi.UserKind,
				Name: "testuser",
			},
		},
		RoleRef: kapi.ObjectReference{
			Kind: "ClusterRole",
			Name: "edit",
		},
	}
	clusterRoleBindingToCreateBytes, err := runtime.Encode(legacyscheme.Codecs.LegacyCodec(schema.GroupVersion{Version: "v1"}), clusterRoleBindingToCreate)
	if err != nil {
		t.Fatal(err)
	}

	clusterRoleBindingCreated := &authorizationapi.ClusterRoleBinding{}
	err = clusterAdmin.Authorization().RESTClient().Post().AbsPath(clusterRoleBindingsPath).Body(clusterRoleBindingToCreateBytes).Do().Into(clusterRoleBindingCreated)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if clusterRoleBindingCreated.Name != clusterRoleBindingToCreate.Name {
		t.Errorf("expected clusterrolebinding %s, got %s", clusterRoleBindingToCreate.Name, clusterRoleBindingCreated.Name)
	}

	// edit clusterrole binding
	clusterRoleBindingToEdit := &authorizationapi.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: testBindingName,
		},
		Subjects: []kapi.ObjectReference{
			{
				Kind: authorizationapi.UserKind,
				Name: "testuser",
			},
			{
				Kind: authorizationapi.UserKind,
				Name: "testuser2",
			},
		},
		RoleRef: kapi.ObjectReference{
			Kind: "ClusterRole",
			Name: "edit",
		},
	}
	clusterRoleBindingToEditBytes, err := runtime.Encode(legacyscheme.Codecs.LegacyCodec(schema.GroupVersion{Version: "v1"}), clusterRoleBindingToEdit)
	if err != nil {
		t.Fatal(err)
	}

	clusterRoleBindingEdited := &authorizationapi.ClusterRoleBinding{}
	err = clusterAdmin.Authorization().RESTClient().Patch(types.StrategicMergePatchType).AbsPath(clusterRoleBindingsPath).Name(clusterRoleBindingToEdit.Name).Body(clusterRoleBindingToEditBytes).Do().Into(clusterRoleBindingEdited)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if clusterRoleBindingEdited.Name != clusterRoleBindingToEdit.Name {
		t.Errorf("expected clusterrolebinding %s, got %s", clusterRoleBindingToEdit.Name, clusterRoleBindingEdited.Name)
	}

	checkSubjects := sets.String{}
	for _, subj := range clusterRoleBindingEdited.Subjects {
		checkSubjects.Insert(subj.Name)
	}
	if !checkSubjects.HasAll("testuser", "testuser2") {
		t.Errorf("clusterrolebinding not edited")
	}

	// get clusterrolebinding by name
	getRoleBinding := &authorizationapi.ClusterRoleBinding{}
	err = clusterAdmin.Authorization().RESTClient().Get().AbsPath(clusterRoleBindingsPath).Name(testBindingName).Do().Into(getRoleBinding)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if getRoleBinding.Name != testBindingName {
		t.Errorf("expected clusterrolebinding %s, got %s", testBindingName, getRoleBinding.Name)
	}

	// delete clusterrolebinding
	err = clusterAdmin.Authorization().RESTClient().Delete().AbsPath(clusterRoleBindingsPath).Name(testBindingName).Do().Error()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// confirm deletion
	getRoleBinding = &authorizationapi.ClusterRoleBinding{}
	err = clusterAdmin.Authorization().RESTClient().Get().AbsPath(clusterRoleBindingsPath).Name(testBindingName).Do().Into(getRoleBinding)
	if err == nil {
		t.Errorf("expected error")
	} else if !kapierror.IsNotFound(err) {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestLegacyClusterRoleEndpoint exercises the legacy clusterrole endpoint that is proxied to rbac
func TestLegacyClusterRoleEndpoint(t *testing.T) {
	masterConfig, clusterAdminKubeConfig, err := testserver.StartTestMasterAPI()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer testserver.CleanupMasterEtcd(t, masterConfig)

	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clusterAdmin := authorizationclient.NewForConfigOrDie(clusterAdminClientConfig)

	// install the legacy types into the client for decoding
	legacy.InstallInternalLegacyAuthorization(authorizationclientscheme.Scheme)

	clusterRolesPath := "/oapi/v1/clusterroles"
	testRole := "testrole"

	// list clusterroles
	clusterRoleList := &authorizationapi.ClusterRoleList{}
	err = clusterAdmin.Authorization().RESTClient().Get().AbsPath(clusterRolesPath).Do().Into(clusterRoleList)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	checkRoles := sets.String{}
	for _, role := range clusterRoleList.Items {
		checkRoles.Insert(role.Name)
	}
	// ensure there are at least some of the expected roles in the clusterrole list
	if !checkRoles.HasAll("admin", "basic-user", "cluster-admin", "edit", "sudoer") {
		t.Errorf("clusterrole list does not have the expected roles")
	}

	// create clusterrole
	clusterRoleToCreate := &authorizationapi.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: testRole},
		Rules: []authorizationapi.PolicyRule{
			authorizationapi.NewRule("get").Groups("").Resources("services").RuleOrDie(),
		},
	}
	clusterRoleToCreateBytes, err := runtime.Encode(legacyscheme.Codecs.LegacyCodec(schema.GroupVersion{Version: "v1"}), clusterRoleToCreate)
	if err != nil {
		t.Fatal(err)
	}
	createdClusterRole := &authorizationapi.ClusterRole{}
	err = clusterAdmin.Authorization().RESTClient().Post().AbsPath(clusterRolesPath).Body(clusterRoleToCreateBytes).Do().Into(createdClusterRole)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if createdClusterRole.Name != clusterRoleToCreate.Name {
		t.Errorf("expected to create %v, got %v", clusterRoleToCreate.Name, createdClusterRole.Name)
	}

	if !createdClusterRole.Rules[0].Verbs.Has("get") {
		t.Errorf("expected clusterrole to have a get rule")
	}

	// update clusterrole
	clusterRoleUpdate := &authorizationapi.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: testRole},
		Rules: []authorizationapi.PolicyRule{
			authorizationapi.NewRule("get", "list").Groups("").Resources("services").RuleOrDie(),
		},
	}

	clusterRoleUpdateBytes, err := runtime.Encode(legacyscheme.Codecs.LegacyCodec(schema.GroupVersion{Version: "v1"}), clusterRoleUpdate)
	if err != nil {
		t.Fatal(err)
	}

	updatedClusterRole := &authorizationapi.ClusterRole{}
	err = clusterAdmin.Authorization().RESTClient().Patch(types.StrategicMergePatchType).AbsPath(clusterRolesPath).Name(testRole).Body(clusterRoleUpdateBytes).Do().Into(updatedClusterRole)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if updatedClusterRole.Name != clusterRoleUpdate.Name {
		t.Errorf("expected to update %s, got %s", clusterRoleUpdate.Name, updatedClusterRole.Name)
	}

	if !updatedClusterRole.Rules[0].Verbs.HasAll("get", "list") {
		t.Errorf("expected clusterrole to have a get and list rule")
	}

	// get clusterrole
	getRole := &authorizationapi.ClusterRole{}
	err = clusterAdmin.Authorization().RESTClient().Get().AbsPath(clusterRolesPath).Name(testRole).Do().Into(getRole)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if getRole.Name != testRole {
		t.Errorf("expected %s role, got %s instead", testRole, getRole.Name)
	}

	// delete clusterrole
	err = clusterAdmin.Authorization().RESTClient().Delete().AbsPath(clusterRolesPath).Name(testRole).Do().Error()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// confirm deletion
	getRole = &authorizationapi.ClusterRole{}
	err = clusterAdmin.Authorization().RESTClient().Get().AbsPath(clusterRolesPath).Name(testRole).Do().Into(getRole)
	if err == nil {
		t.Errorf("expected error")
	} else if !kapierror.IsNotFound(err) {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestLegacyLocalRoleEndpoint exercises the legacy role endpoint that is proxied to rbac
func TestLegacyLocalRoleEndpoint(t *testing.T) {
	masterConfig, clusterAdminKubeConfig, err := testserver.StartTestMasterAPI()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer testserver.CleanupMasterEtcd(t, masterConfig)

	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clusterAdmin := authorizationclient.NewForConfigOrDie(clusterAdminClientConfig)

	namespace := "testproject"
	_, _, err = testserver.CreateNewProject(clusterAdminClientConfig, namespace, "testuser")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// install the legacy types into the client for decoding
	legacy.InstallInternalLegacyAuthorization(authorizationclientscheme.Scheme)

	rolesPath := "/oapi/v1/namespaces/" + namespace + "/roles"
	testRole := "testrole"

	// create role
	roleToCreate := &authorizationapi.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testRole,
			Namespace: namespace,
		},
		Rules: []authorizationapi.PolicyRule{
			authorizationapi.NewRule("get").Groups("").Resources("services").RuleOrDie(),
		},
	}
	roleToCreateBytes, err := runtime.Encode(legacyscheme.Codecs.LegacyCodec(schema.GroupVersion{Version: "v1"}), roleToCreate)
	if err != nil {
		t.Fatal(err)
	}
	createdRole := &authorizationapi.Role{}
	err = clusterAdmin.Authorization().RESTClient().Post().AbsPath(rolesPath).Body(roleToCreateBytes).Do().Into(createdRole)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if createdRole.Name != roleToCreate.Name {
		t.Errorf("expected to create %v, got %v", roleToCreate.Name, createdRole.Name)
	}

	if !createdRole.Rules[0].Verbs.Has("get") {
		t.Errorf("expected clusterRole to have a get rule")
	}

	// list roles
	roleList := &authorizationapi.RoleList{}
	err = clusterAdmin.Authorization().RESTClient().Get().AbsPath(rolesPath).Do().Into(roleList)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	checkRoles := sets.String{}
	for _, role := range roleList.Items {
		checkRoles.Insert(role.Name)
	}
	// ensure the role list has the created role
	if !checkRoles.HasAll(testRole) {
		t.Errorf("role list does not have the expected roles")
	}

	// update role
	roleUpdate := &authorizationapi.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testRole,
			Namespace: namespace,
		},
		Rules: []authorizationapi.PolicyRule{
			authorizationapi.NewRule("get", "list").Groups("").Resources("services").RuleOrDie(),
		},
	}

	roleUpdateBytes, err := runtime.Encode(legacyscheme.Codecs.LegacyCodec(schema.GroupVersion{Version: "v1"}), roleUpdate)
	if err != nil {
		t.Fatal(err)
	}

	updatedRole := &authorizationapi.Role{}
	err = clusterAdmin.Authorization().RESTClient().Patch(types.StrategicMergePatchType).AbsPath(rolesPath).Name(testRole).Body(roleUpdateBytes).Do().Into(updatedRole)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if updatedRole.Name != roleUpdate.Name {
		t.Errorf("expected to update %s, got %s", roleUpdate.Name, updatedRole.Name)
	}

	if !updatedRole.Rules[0].Verbs.HasAll("get", "list") {
		t.Errorf("expected role to have a get and list rule")
	}

	// get role
	getRole := &authorizationapi.Role{}
	err = clusterAdmin.Authorization().RESTClient().Get().AbsPath(rolesPath).Name(testRole).Do().Into(getRole)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if getRole.Name != testRole {
		t.Errorf("expected %s role, got %s instead", testRole, getRole.Name)
	}

	// delete role
	err = clusterAdmin.Authorization().RESTClient().Delete().AbsPath(rolesPath).Name(testRole).Do().Error()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// confirm deletion
	getRole = &authorizationapi.Role{}
	err = clusterAdmin.Authorization().RESTClient().Get().AbsPath(rolesPath).Name(testRole).Do().Into(getRole)
	if err == nil {
		t.Errorf("expected error")
	} else if !kapierror.IsNotFound(err) {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestOldLocalAccessReviewEndpoints(t *testing.T) {
	masterConfig, clusterAdminKubeConfig, err := testserver.StartTestMasterAPI()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer testserver.CleanupMasterEtcd(t, masterConfig)

	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	clusterAdminAuthorizationClient := authorizationclient.NewForConfigOrDie(clusterAdminClientConfig).Authorization()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	namespace := "hammer-project"
	if _, _, err := testserver.CreateNewProject(clusterAdminClientConfig, namespace, "harold"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// install the legacy types into the client for decoding
	legacy.InstallInternalLegacyAuthorization(authorizationclientscheme.Scheme)
	codecFactory := serializer.NewCodecFactory(authorizationclientscheme.Scheme)

	sar := &authorizationapi.SubjectAccessReview{
		Action: authorizationapi.Action{
			Verb:     "get",
			Resource: "imagestreams/layers",
		},
	}
	sarBytes, err := runtime.Encode(codecFactory.LegacyCodec(schema.GroupVersion{Version: "v1"}), sar)
	if err != nil {
		t.Fatal(err)
	}
	err = clusterAdminAuthorizationClient.RESTClient().Post().AbsPath("/oapi/v1/namespaces/" + namespace + "/subjectaccessreviews").Body(sarBytes).Do().Into(&authorizationapi.SubjectAccessReviewResponse{})
	if !kapierror.IsNotFound(err) {
		t.Fatal(err)
	}

	rar := &authorizationapi.ResourceAccessReview{
		Action: authorizationapi.Action{
			Verb:     "get",
			Resource: "imagestreams/layers",
		},
	}
	rarBytes, err := runtime.Encode(codecFactory.LegacyCodec(schema.GroupVersion{Version: "v1"}), rar)
	if err != nil {
		t.Fatal(err)
	}
	err = clusterAdminAuthorizationClient.RESTClient().Post().AbsPath("/oapi/v1/namespaces/" + namespace + "/resourceaccessreviews").Body(rarBytes).Do().Into(&authorizationapi.ResourceAccessReviewResponse{})
	if !kapierror.IsNotFound(err) {
		t.Fatal(err)
	}
}
