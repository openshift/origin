package integration

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	kapierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	appsapi "k8s.io/kubernetes/pkg/apis/apps"
	kubeauthorizationapi "k8s.io/kubernetes/pkg/apis/authorization"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	extensionsapi "k8s.io/kubernetes/pkg/apis/extensions"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/authorization/internalversion"

	authorizationapiv1 "github.com/openshift/api/authorization/v1"
	"github.com/openshift/origin/pkg/api/legacy"
	oappsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	appsclient "github.com/openshift/origin/pkg/apps/generated/internalclientset"
	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	authorizationclient "github.com/openshift/origin/pkg/authorization/generated/internalclientset"
	authorizationclientscheme "github.com/openshift/origin/pkg/authorization/generated/internalclientset/scheme"
	authorizationtypedclient "github.com/openshift/origin/pkg/authorization/generated/internalclientset/typed/authorization/internalversion"
	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	buildclient "github.com/openshift/origin/pkg/build/generated/internalclientset"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	oauthapi "github.com/openshift/origin/pkg/oauth/apis/oauth"
	policy "github.com/openshift/origin/pkg/oc/admin/policy"
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
		oauthapi.Resource("oauthauthorizetokens"):     true,
		oauthapi.Resource("oauthaccesstokens"):        true,
		oauthapi.Resource("oauthclients"):             true,
		imageapi.Resource("imagestreams/secrets"):     true,
		kapi.Resource("secrets"):                      true,
		kapi.Resource("pods/exec"):                    true,
		kapi.Resource("pods/proxy"):                   true,
		kapi.Resource("pods/portforward"):             true,
		kapi.Resource("nodes/proxy"):                  true,
		kapi.Resource("services/proxy"):               true,
		{Group: "", Resource: "oauthauthorizetokens"}: true,
		{Group: "", Resource: "oauthaccesstokens"}:    true,
		{Group: "", Resource: "oauthclients"}:         true,
		{Group: "", Resource: "imagestreams/secrets"}: true,
	}

	readerRole, err := authorizationclient.NewForConfigOrDie(clusterAdminClientConfig).Authorization().ClusterRoles().Get(bootstrappolicy.ClusterReaderRoleName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, rule := range readerRole.Rules {
		for _, group := range rule.APIGroups {
			for resource := range rule.Resources {
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
		buildapi.Resource("buildconfigs/instantiatebinary"),
		buildapi.Resource("buildconfigs/instantiate"),
		buildapi.Resource("builds/clone"),
		oappsapi.Resource("deploymentconfigrollbacks"),
		oappsapi.Resource("generatedeploymentconfigs"),
		oappsapi.Resource("deploymentconfigs/rollback"),
		oappsapi.Resource("deploymentconfigs/instantiate"),
		imageapi.Resource("imagestreamimports"),
		imageapi.Resource("imagestreammappings"),
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

	_, err = appsclient.NewForConfigOrDie(haroldConfig).Apps().DeploymentConfigs("hammer-project").List(metav1.ListOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = appsclient.NewForConfigOrDie(markConfig).Apps().DeploymentConfigs("hammer-project").List(metav1.ListOptions{})
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
	waitForProject(t, projectclient.NewForConfigOrDie(haroldConfig), "hammer-project", 1*time.Second, 10)
	waitForProject(t, projectclient.NewForConfigOrDie(markConfig), "mallet-project", 1*time.Second, 10)
}

// waitForProject will execute a client list of projects looking for the project with specified name
// if not found, it will retry up to numRetries at the specified delayInterval
func waitForProject(t *testing.T, client projectclient.Interface, projectName string, delayInterval time.Duration, numRetries int) {
	for i := 0; i <= numRetries; i++ {
		projects, err := client.Project().Projects().List(metav1.ListOptions{})
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
	clusterAdminAuthorizationClient := authorizationclient.NewForConfigOrDie(clusterAdminClientConfig).Authorization()

	addValerie := &policy.RoleModificationOptions{
		RoleNamespace:       "",
		RoleName:            bootstrappolicy.ViewRoleName,
		RoleBindingAccessor: policy.NewClusterRoleBindingAccessor(authorizationclient.NewForConfigOrDie(clusterAdminClientConfig).Authorization()),
		Users:               []string{"valerie"},
	}
	if err := addValerie.AddRole(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err = clusterAdminAuthorizationClient.ClusterRoles().Delete(bootstrappolicy.ViewRoleName, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	addEdgar := &policy.RoleModificationOptions{
		RoleNamespace:       "",
		RoleName:            bootstrappolicy.EditRoleName,
		RoleBindingAccessor: policy.NewClusterRoleBindingAccessor(authorizationclient.NewForConfigOrDie(clusterAdminClientConfig).Authorization()),
		Users:               []string{"edgar"},
	}
	if err := addEdgar.AddRole(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := addValerie.AddRole(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	roleWithGroup := &authorizationapi.ClusterRole{}
	roleWithGroup.Name = "with-group"
	roleWithGroup.Rules = append(roleWithGroup.Rules, authorizationapi.PolicyRule{
		APIGroups: []string{buildapi.GroupName},
		Verbs:     sets.NewString("list"),
		Resources: sets.NewString("builds"),
	})
	if _, err := clusterAdminAuthorizationClient.ClusterRoles().Create(roleWithGroup); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	addBuildLister := &policy.RoleModificationOptions{
		RoleNamespace:       "",
		RoleName:            "with-group",
		RoleBindingAccessor: policy.NewClusterRoleBindingAccessor(authorizationclient.NewForConfigOrDie(clusterAdminClientConfig).Authorization()),
		Users:               []string{"build-lister"},
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
	buildClient := buildclient.NewForConfigOrDie(userClientConfig)
	appsClient := appsclient.NewForConfigOrDie(userClientConfig)

	// the authorization cache may not be up to date, retry
	if err := wait.Poll(10*time.Millisecond, 2*time.Minute, func() (bool, error) {
		_, err := buildClient.Build().Builds(metav1.NamespaceDefault).List(metav1.ListOptions{})
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
		RoleNamespace:       "",
		RoleName:            bootstrappolicy.ViewRoleName,
		RoleBindingAccessor: policy.NewLocalRoleBindingAccessor("hammer-project", authorizationclient.NewForConfigOrDie(haroldConfig).Authorization()),
		Users:               []string{"valerie"},
	}
	if err := addValerie.AddRole(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	addEdgar := &policy.RoleModificationOptions{
		RoleNamespace:       "",
		RoleName:            bootstrappolicy.EditRoleName,
		RoleBindingAccessor: policy.NewLocalRoleBindingAccessor("mallet-project", authorizationclient.NewForConfigOrDie(markConfig).Authorization()),
		Users:               []string{"edgar"},
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

	clusterAdminKubeClient, err := testutil.GetClusterAdminKubeClient(clusterAdminKubeConfig)
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
			Reason:    "allowed by openshift authorizer",
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
			Reason:    `User "harold" cannot get horizontalpodautoscalers in project "hammer-project"`,
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
			Reason:    `User "harold" cannot get horizontalpodautoscalers.foo in project "hammer-project"`,
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
			Reason:    `User "harold" cannot get horizontalpodautoscalers.* in project "hammer-project"`,
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

	clusterAdminKubeClient, err := testutil.GetClusterAdminKubeClient(clusterAdminKubeConfig)
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
		RoleNamespace:       "",
		RoleName:            bootstrappolicy.EditRoleName,
		RoleBindingAccessor: policy.NewLocalRoleBindingAccessor("hammer-project", authorizationclient.NewForConfigOrDie(clusterAdminClientConfig).Authorization()),
		Users:               []string{"system:anonymous"},
	}
	if err := addAnonymous.AddRole(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	addDanny := &policy.RoleModificationOptions{
		RoleNamespace:       "",
		RoleName:            bootstrappolicy.ViewRoleName,
		RoleBindingAccessor: policy.NewLocalRoleBindingAccessor("default", authorizationclient.NewForConfigOrDie(clusterAdminClientConfig).Authorization()),
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
		localInterface: clusterAdminAuthorizationClient.LocalSubjectAccessReviews("default"),
		localReview: &authorizationapi.LocalSubjectAccessReview{
			User:   "danny",
			Action: authorizationapi.Action{Verb: "get", Resource: "projects"},
		},
		kubeAuthInterface: clusterAdminLocalSARGetter,
		response: authorizationapi.SubjectAccessReviewResponse{
			Allowed:   true,
			Reason:    "allowed by openshift authorizer",
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
			Reason:    `User "danny" cannot get projects at the cluster scope`,
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
		RoleNamespace:       "",
		RoleName:            bootstrappolicy.ViewRoleName,
		RoleBindingAccessor: policy.NewLocalRoleBindingAccessor("hammer-project", authorizationclient.NewForConfigOrDie(haroldConfig).Authorization()),
		Users:               []string{"valerie"},
	}
	if err := addValerie.AddRole(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	addEdgar := &policy.RoleModificationOptions{
		RoleNamespace:       "",
		RoleName:            bootstrappolicy.EditRoleName,
		RoleBindingAccessor: policy.NewLocalRoleBindingAccessor("mallet-project", authorizationclient.NewForConfigOrDie(markConfig).Authorization()),
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
		description:       "harold told valerie can get project hammer-project",
		localInterface:    haroldAuthorizationClient.LocalSubjectAccessReviews("hammer-project"),
		localReview:       askCanValerieGetProject,
		kubeAuthInterface: haroldSARGetter,
		response: authorizationapi.SubjectAccessReviewResponse{
			Allowed:   true,
			Reason:    "allowed by openshift authorizer",
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
			Reason:    `User "valerie" cannot get project "mallet-project"`,
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
			Reason:    "allowed by openshift authorizer",
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
			Reason:    "allowed by openshift authorizer",
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
			Reason:    "allowed by cluster rule",
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
			Reason:    "allowed by openshift authorizer",
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
			Reason:    "allowed by openshift authorizer",
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
			Reason:    `User "harold" cannot create pods in project "mallet-project"`,
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
			Reason:    `User "system:anonymous" cannot create pods in project "mallet-project"`,
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
			Reason:    `User "harold" cannot create pods in project "nonexistent-project"`,
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
			Reason:    `User "system:anonymous" cannot create pods in project "nonexistent-project"`,
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
			Reason:    `User "harold" cannot create policybindings in project "hammer-project"`,
			Namespace: "hammer-project",
		},
	}.run(t)
}

// TestOldLocalSubjectAccessReviewEndpoint checks to make sure that the old subject access review endpoint still functions properly
// this is needed to support old docker registry images
func TestOldLocalSubjectAccessReviewEndpoint(t *testing.T) {
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
	haroldClient := authorizationclient.NewForConfigOrDie(haroldConfig)

	// install the legacy types into the client for decoding
	legacy.InstallLegacy(authorizationapi.GroupName, authorizationapi.AddToSchemeInCoreGroup, authorizationapiv1.AddToSchemeInCoreGroup,
		sets.NewString("ClusterRole", "ClusterRoleBinding", "ResourceAccessReviewResponse", "SubjectAccessReviewResponse"),
		authorizationclientscheme.Registry, authorizationclientscheme.Scheme,
	)

	namespace := "hammer-project"

	// simple check
	{
		sar := &authorizationapi.SubjectAccessReview{
			Action: authorizationapi.Action{
				Verb:     "get",
				Resource: "imagestreams/layers",
			},
		}
		sarBytes, err := runtime.Encode(legacyscheme.Codecs.LegacyCodec(schema.GroupVersion{Version: "v1"}), sar)
		if err != nil {
			t.Fatal(err)
		}
		actualResponse := &authorizationapi.SubjectAccessReviewResponse{}
		err = haroldClient.Authorization().RESTClient().Post().AbsPath("/oapi/v1/namespaces/" + namespace + "/subjectaccessreviews").Body(sarBytes).Do().Into(actualResponse)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		expectedResponse := &authorizationapi.SubjectAccessReviewResponse{
			Allowed:   true,
			Reason:    `allowed by openshift authorizer`,
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
		sarBytes, err := runtime.Encode(legacyscheme.Codecs.LegacyCodec(schema.GroupVersion{Version: "v1"}), sar)
		if err != nil {
			t.Fatal(err)
		}
		actualResponse := &authorizationapi.SubjectAccessReviewResponse{}
		err = haroldClient.Authorization().RESTClient().Post().AbsPath("/oapi/v1/namespaces/" + namespace + "/subjectaccessreviews").Body(sarBytes).Do().Into(actualResponse)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		expectedResponse := &authorizationapi.SubjectAccessReviewResponse{
			Allowed:   true,
			Reason:    `allowed by openshift authorizer`,
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
		if _, _, err := testserver.CreateNewProject(clusterAdminClientConfig, otherNamespace, "charlie"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// remove the new permission for localSAR
		basicUserRole, err := clusterAdminAuthorizationClient.ClusterRoles().Get(bootstrappolicy.BasicUserRoleName, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		for i := range basicUserRole.Rules {
			basicUserRole.Rules[i].Resources.Delete("localsubjectaccessreviews")
		}

		if _, err := clusterAdminAuthorizationClient.ClusterRoles().Update(basicUserRole); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		sar := &authorizationapi.SubjectAccessReview{
			Action: authorizationapi.Action{
				Verb:     "get",
				Resource: "imagestreams/layers",
			},
		}
		sarBytes, err := runtime.Encode(legacyscheme.Codecs.LegacyCodec(schema.GroupVersion{Version: "v1"}), sar)
		if err != nil {
			t.Fatal(err)
		}
		actualResponse := &authorizationapi.SubjectAccessReviewResponse{}
		err = haroldClient.Authorization().RESTClient().Post().AbsPath("/oapi/v1/namespaces/" + otherNamespace + "/subjectaccessreviews").Body(sarBytes).Do().Into(actualResponse)
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
	haroldClient := authorizationclient.NewForConfigOrDie(haroldConfig)

	namespace := "hammer-project"

	// install the legacy types into the client for decoding
	legacy.InstallLegacy(authorizationapi.GroupName, authorizationapi.AddToSchemeInCoreGroup, authorizationapiv1.AddToSchemeInCoreGroup,
		sets.NewString("ClusterRole", "ClusterRoleBinding", "ResourceAccessReviewResponse", "SubjectAccessReviewResponse"),
		authorizationclientscheme.Registry, authorizationclientscheme.Scheme,
	)

	// simple check
	{
		rar := &authorizationapi.ResourceAccessReview{
			Action: authorizationapi.Action{
				Verb:     "get",
				Resource: "imagestreams/layers",
			},
		}
		rarBytes, err := runtime.Encode(legacyscheme.Codecs.LegacyCodec(schema.GroupVersion{Version: "v1"}), rar)
		if err != nil {
			t.Fatal(err)
		}
		actualResponse := &authorizationapi.ResourceAccessReviewResponse{}
		err = haroldClient.Authorization().RESTClient().Post().AbsPath("/oapi/v1/namespaces/" + namespace + "/resourceaccessreviews").Body(rarBytes).Do().Into(actualResponse)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		expectedResponse := &authorizationapi.ResourceAccessReviewResponse{
			Namespace: namespace,
			Users:     sets.NewString("harold", "system:serviceaccount:kube-system:clusterrole-aggregation-controller", "system:serviceaccount:kube-system:generic-garbage-collector", "system:serviceaccount:kube-system:namespace-controller", "system:serviceaccount:openshift-infra:template-instance-controller", "system:serviceaccount:openshift-infra:template-instance-finalizer-controller", "system:serviceaccount:hammer-project:builder", "system:admin", "system:serviceaccount:openshift-infra:default-rolebindings-controller"),
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
		rarBytes, err := runtime.Encode(legacyscheme.Codecs.LegacyCodec(schema.GroupVersion{Version: "v1"}), rar)
		if err != nil {
			t.Fatal(err)
		}
		actualResponse := &authorizationapi.ResourceAccessReviewResponse{}
		err = haroldClient.Authorization().RESTClient().Post().AbsPath("/oapi/v1/namespaces/" + namespace + "/resourceaccessreviews").Body(rarBytes).Do().Into(actualResponse)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		expectedResponse := &authorizationapi.ResourceAccessReviewResponse{
			Namespace: namespace,
			Users:     sets.NewString("harold", "system:serviceaccount:kube-system:clusterrole-aggregation-controller", "system:serviceaccount:kube-system:generic-garbage-collector", "system:serviceaccount:kube-system:namespace-controller", "system:serviceaccount:openshift-infra:template-instance-controller", "system:serviceaccount:openshift-infra:template-instance-finalizer-controller", "system:serviceaccount:hammer-project:builder", "system:admin", "system:serviceaccount:openshift-infra:default-rolebindings-controller"),
			Groups:    sets.NewString("system:cluster-admins", "system:masters", "system:cluster-readers", "system:serviceaccounts:hammer-project"),
		}
		if (actualResponse.Namespace != expectedResponse.Namespace) ||
			!reflect.DeepEqual(actualResponse.Users.List(), expectedResponse.Users.List()) ||
			!reflect.DeepEqual(actualResponse.Groups.List(), expectedResponse.Groups.List()) {
			t.Errorf("review\n\t%#v\nexpected\n\t%#v\ngot\n\t%#v", rar, expectedResponse, actualResponse)
		}
	}
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
		return strings.Contains(errProxy.Error(), `cannot "unsafeproxy" "pods" with name "podX1:8080" in project "ns"`) ||
			strings.Contains(errProxy.Error(), `cannot get pods/unsafeproxy in project "ns"`)
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
