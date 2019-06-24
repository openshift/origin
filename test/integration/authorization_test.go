package integration

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	kubeauthorizationv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	kapierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	kauthorizationv1client "k8s.io/client-go/kubernetes/typed/authorization/v1"
	rbacv1client "k8s.io/client-go/kubernetes/typed/rbac/v1"
	"k8s.io/client-go/rest"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	appsapi "k8s.io/kubernetes/pkg/apis/apps"
	extensionsapi "k8s.io/kubernetes/pkg/apis/extensions"

	oapps "github.com/openshift/api/apps"
	authorizationv1 "github.com/openshift/api/authorization/v1"
	"github.com/openshift/api/build"
	"github.com/openshift/api/image"
	"github.com/openshift/api/oauth"
	authorizationv1client "github.com/openshift/client-go/authorization/clientset/versioned"
	authorizationv1typedclient "github.com/openshift/client-go/authorization/clientset/versioned/typed/authorization/v1"
	projectv1client "github.com/openshift/client-go/project/clientset/versioned/typed/project/v1"
	"github.com/openshift/oc/pkg/cli/admin/policy"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func prettyPrintAction(act *authorizationv1.Action, defaultNamespaceStr string) string {
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

	if len(act.Content.Raw) != 0 {
		return fmt.Sprintf("%s with content %#v", base, act.Content)
	}

	return base
}

func prettyPrintReviewResponse(resp *authorizationv1.ResourceAccessReviewResponse) string {
	nsStr := fmt.Sprintf("(in the namespace %q)\n", resp.Namespace)
	if resp.Namespace == "" {
		nsStr = "(in all namespaces)\n"
	}

	var usersStr string
	if len(resp.UsersSlice) > 0 {
		userStrList := make([]string, 0, len(resp.UsersSlice))
		for _, userName := range resp.UsersSlice {
			userStrList = append(userStrList, fmt.Sprintf("    - %s\n", userName))
		}

		usersStr = fmt.Sprintf("  users:\n%s", strings.Join(userStrList, ""))
	}

	var groupsStr string
	if len(resp.GroupsSlice) > 0 {
		groupStrList := make([]string, 0, len(resp.GroupsSlice))
		for _, groupName := range resp.GroupsSlice {
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
		oauth.Resource("oauthauthorizetokens"): true,
		oauth.Resource("oauthaccesstokens"):    true,
		oauth.Resource("oauthclients"):         true,
		image.Resource("imagestreams/secrets"): true,
		corev1.Resource("secrets"):             true,
		corev1.Resource("pods/exec"):           true,
		corev1.Resource("pods/proxy"):          true,
		corev1.Resource("pods/portforward"):    true,
		corev1.Resource("nodes/proxy"):         true,
		corev1.Resource("services/proxy"):      true,
		{Resource: "oauthauthorizetokens"}:     true,
		{Resource: "oauthaccesstokens"}:        true,
		{Resource: "oauthclients"}:             true,
		{Resource: "imagestreams/secrets"}:     true,
	}

	readerRole, err := rbacv1client.NewForConfigOrDie(clusterAdminClientConfig).ClusterRoles().Get("cluster-reader", metav1.GetOptions{})
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
		corev1.Resource("pods/attach"),
		corev1.Resource("namespaces/finalize"),
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
	clientInterface authorizationv1typedclient.ResourceAccessReviewInterface
	review          *authorizationv1.ResourceAccessReview

	response authorizationv1.ResourceAccessReviewResponse
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
			!reflect.DeepEqual(sets.NewString(actualResponse.UsersSlice...), sets.NewString(test.response.UsersSlice...)) ||
			!reflect.DeepEqual(sets.NewString(actualResponse.GroupsSlice...), sets.NewString(test.response.GroupsSlice...)) ||
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
	clientInterface authorizationv1typedclient.LocalResourceAccessReviewInterface
	review          *authorizationv1.LocalResourceAccessReview

	response authorizationv1.ResourceAccessReviewResponse
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

		if !reflect.DeepEqual(sets.NewString(actualResponse.UsersSlice...), sets.NewString(test.response.UsersSlice...)) {
			failMessage = fmt.Sprintf("%s:\n  %s:\n  expected %s\n  got %s", test.description, prettyPrintAction(&test.review.Action, "(in the current namespace)"), prettyPrintReviewResponse(&test.response), prettyPrintReviewResponse(actualResponse))
			return false, nil
		}

		if !reflect.DeepEqual(sets.NewString(actualResponse.GroupsSlice...), sets.NewString(test.response.GroupsSlice...)) {
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
	clusterAdminAuthorizationClient := authorizationv1client.NewForConfigOrDie(clusterAdminClientConfig).AuthorizationV1()

	_, haroldConfig, err := testserver.CreateNewProject(clusterAdminClientConfig, "hammer-project", "harold")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	haroldAuthorizationClient := authorizationv1client.NewForConfigOrDie(haroldConfig).AuthorizationV1()

	_, markConfig, err := testserver.CreateNewProject(clusterAdminClientConfig, "mallet-project", "mark")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	markAuthorizationClient := authorizationv1client.NewForConfigOrDie(markConfig).AuthorizationV1()

	addValerie := &policy.RoleModificationOptions{
		RoleBindingNamespace: "hammer-project",
		RoleName:             "view",
		RoleKind:             "ClusterRole",
		RbacClient:           rbacv1client.NewForConfigOrDie(haroldConfig),
		Users:                []string{"valerie"},
		PrintFlags:           genericclioptions.NewPrintFlags(""),
		ToPrinter:            func(string) (printers.ResourcePrinter, error) { return printers.NewDiscardingPrinter(), nil },
	}
	if err := addValerie.AddRole(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	addEdgar := &policy.RoleModificationOptions{
		RoleBindingNamespace: "mallet-project",
		RoleName:             "edit",
		RoleKind:             "ClusterRole",
		RbacClient:           rbacv1client.NewForConfigOrDie(markConfig),
		Users:                []string{"edgar"},
		PrintFlags:           genericclioptions.NewPrintFlags(""),
		ToPrinter:            func(string) (printers.ResourcePrinter, error) { return printers.NewDiscardingPrinter(), nil },
	}
	if err := addEdgar.AddRole(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	requestWhoCanViewDeploymentConfigs := &authorizationv1.ResourceAccessReview{
		Action: authorizationv1.Action{Verb: "get", Resource: "deploymentconfigs", Group: ""},
	}

	localRequestWhoCanViewDeploymentConfigs := &authorizationv1.LocalResourceAccessReview{
		Action: authorizationv1.Action{Verb: "get", Resource: "deploymentconfigs", Group: ""},
	}

	{
		test := localResourceAccessReviewTest{
			description:     "who can view deploymentconfigs in hammer by harold",
			clientInterface: haroldAuthorizationClient.LocalResourceAccessReviews("hammer-project"),
			review:          localRequestWhoCanViewDeploymentConfigs,
			response: authorizationv1.ResourceAccessReviewResponse{
				UsersSlice:  []string{"harold", "valerie"},
				GroupsSlice: []string{},
				Namespace:   "hammer-project",
			},
		}
		test.response.UsersSlice = append(test.response.UsersSlice, globalClusterReaderUsers.List()...)
		test.response.UsersSlice = append(test.response.UsersSlice, globalDeploymentConfigGetterUsers.List()...)
		test.response.GroupsSlice = append(test.response.GroupsSlice, globalClusterReaderGroups.List()...)
		test.run(t)
	}
	{
		test := localResourceAccessReviewTest{
			description:     "who can view deploymentconfigs in mallet by mark",
			clientInterface: markAuthorizationClient.LocalResourceAccessReviews("mallet-project"),
			review:          localRequestWhoCanViewDeploymentConfigs,
			response: authorizationv1.ResourceAccessReviewResponse{
				UsersSlice:  []string{"mark", "edgar"},
				GroupsSlice: []string{},
				Namespace:   "mallet-project",
			},
		}
		test.response.UsersSlice = append(test.response.UsersSlice, globalClusterReaderUsers.List()...)
		test.response.UsersSlice = append(test.response.UsersSlice, globalDeploymentConfigGetterUsers.List()...)
		test.response.GroupsSlice = append(test.response.GroupsSlice, globalClusterReaderGroups.List()...)
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
			response: authorizationv1.ResourceAccessReviewResponse{
				UsersSlice:  []string{},
				GroupsSlice: []string{},
			},
		}
		test.response.UsersSlice = append(test.response.UsersSlice, globalClusterReaderUsers.List()...)
		test.response.UsersSlice = append(test.response.UsersSlice, globalDeploymentConfigGetterUsers.List()...)
		test.response.GroupsSlice = append(test.response.GroupsSlice, globalClusterReaderGroups.List()...)
		test.run(t)
	}

	{
		if err := clusterAdminAuthorizationClient.ClusterRoles().Delete("admin", nil); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		test := localResourceAccessReviewTest{
			description:     "who can view deploymentconfigs in mallet by cluster-admin",
			clientInterface: clusterAdminAuthorizationClient.LocalResourceAccessReviews("mallet-project"),
			review:          localRequestWhoCanViewDeploymentConfigs,
			response: authorizationv1.ResourceAccessReviewResponse{
				UsersSlice:      []string{"edgar"},
				GroupsSlice:     []string{},
				Namespace:       "mallet-project",
				EvaluationError: `clusterrole.rbac.authorization.k8s.io "admin" not found`,
			},
		}
		test.response.UsersSlice = append(test.response.UsersSlice, globalClusterReaderUsers.List()...)
		test.response.UsersSlice = append(test.response.UsersSlice, globalDeploymentConfigGetterUsers.List()...)

		newList := []string{}
		toRemove := sets.NewString("system:serviceaccount:openshift-infra:template-instance-controller", "system:serviceaccount:openshift-infra:template-instance-finalizer-controller")
		for _, curr := range test.response.UsersSlice {
			if toRemove.Has(curr) {
				continue
			}
			newList = append(newList, curr)
		}
		test.response.UsersSlice = newList
		test.response.GroupsSlice = append(test.response.GroupsSlice, globalClusterReaderGroups.List()...)
		test.run(t)
	}
}

type subjectAccessReviewTest struct {
	description      string
	localInterface   authorizationv1typedclient.LocalSubjectAccessReviewInterface
	clusterInterface authorizationv1typedclient.SubjectAccessReviewInterface
	localReview      *authorizationv1.LocalSubjectAccessReview
	clusterReview    *authorizationv1.SubjectAccessReview

	kubeNamespace     string
	kubeErr           string
	kubeSkip          bool
	kubeAuthInterface kauthorizationv1client.AuthorizationV1Interface

	response authorizationv1.SubjectAccessReviewResponse
	err      string
}

func (test subjectAccessReviewTest) run(t *testing.T) {
	{
		failMessage := ""
		err := wait.Poll(testutil.PolicyCachePollInterval, testutil.PolicyCachePollTimeout, func() (bool, error) {
			var err error
			var actualResponse *authorizationv1.SubjectAccessReviewResponse
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
			var actualResponse kubeauthorizationv1.SubjectAccessReviewStatus
			if test.localReview != nil {
				if len(test.localReview.User) == 0 && (len(test.localReview.GroupsSlice) == 0) {
					var tmp *kubeauthorizationv1.SelfSubjectAccessReview
					if tmp, err = test.kubeAuthInterface.SelfSubjectAccessReviews().Create(toKubeSelfSAR(testNS, test.localReview)); err == nil {
						actualResponse = tmp.Status
					}
				} else {
					var tmp *kubeauthorizationv1.LocalSubjectAccessReview
					if tmp, err = test.kubeAuthInterface.LocalSubjectAccessReviews(testNS).Create(toKubeLocalSAR(testNS, test.localReview)); err == nil {
						actualResponse = tmp.Status
					}
				}
			} else {
				var tmp *kubeauthorizationv1.SubjectAccessReview
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
func toKubeSelfSAR(testNS string, sar *authorizationv1.LocalSubjectAccessReview) *kubeauthorizationv1.SelfSubjectAccessReview {
	return &kubeauthorizationv1.SelfSubjectAccessReview{
		Spec: kubeauthorizationv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &kubeauthorizationv1.ResourceAttributes{
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
func toKubeLocalSAR(testNS string, sar *authorizationv1.LocalSubjectAccessReview) *kubeauthorizationv1.LocalSubjectAccessReview {
	return &kubeauthorizationv1.LocalSubjectAccessReview{
		ObjectMeta: metav1.ObjectMeta{Namespace: testNS},
		Spec: kubeauthorizationv1.SubjectAccessReviewSpec{
			User:   sar.User,
			Groups: sar.GroupsSlice,
			ResourceAttributes: &kubeauthorizationv1.ResourceAttributes{
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
func toKubeClusterSAR(sar *authorizationv1.SubjectAccessReview) *kubeauthorizationv1.SubjectAccessReview {
	return &kubeauthorizationv1.SubjectAccessReview{
		Spec: kubeauthorizationv1.SubjectAccessReviewSpec{
			User:   sar.User,
			Groups: sar.GroupsSlice,
			ResourceAttributes: &kubeauthorizationv1.ResourceAttributes{
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
	clusterAdminSARGetter := clusterAdminKubeClient.AuthorizationV1()

	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	clusterAdminAuthorizationClient := authorizationv1client.NewForConfigOrDie(clusterAdminClientConfig).AuthorizationV1()

	_, _, err = testserver.CreateNewProject(clusterAdminClientConfig, "hammer-project", "harold")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// SAR honors API Group
	subjectAccessReviewTest{
		description:    "cluster admin told harold can get autoscaling.horizontalpodautoscalers in project hammer-project",
		localInterface: clusterAdminAuthorizationClient.LocalSubjectAccessReviews("hammer-project"),
		localReview: &authorizationv1.LocalSubjectAccessReview{
			User:   "harold",
			Action: authorizationv1.Action{Verb: "get", Group: "autoscaling", Resource: "horizontalpodautoscalers"},
		},
		kubeAuthInterface: clusterAdminSARGetter,
		response: authorizationv1.SubjectAccessReviewResponse{
			Allowed:   true,
			Reason:    `RBAC: allowed by RoleBinding "admin/hammer-project" of ClusterRole "admin" to User "harold"`,
			Namespace: "hammer-project",
		},
	}.run(t)
	subjectAccessReviewTest{
		description:    "cluster admin told harold cannot get horizontalpodautoscalers (with no API group) in project hammer-project",
		localInterface: clusterAdminAuthorizationClient.LocalSubjectAccessReviews("hammer-project"),
		localReview: &authorizationv1.LocalSubjectAccessReview{
			User:   "harold",
			Action: authorizationv1.Action{Verb: "get", Group: "", Resource: "horizontalpodautoscalers"},
		},
		kubeAuthInterface: clusterAdminSARGetter,
		response: authorizationv1.SubjectAccessReviewResponse{
			Allowed:   false,
			Reason:    "",
			Namespace: "hammer-project",
		},
	}.run(t)
	subjectAccessReviewTest{
		description:    "cluster admin told harold cannot get horizontalpodautoscalers (with invalid API group) in project hammer-project",
		localInterface: clusterAdminAuthorizationClient.LocalSubjectAccessReviews("hammer-project"),
		localReview: &authorizationv1.LocalSubjectAccessReview{
			User:   "harold",
			Action: authorizationv1.Action{Verb: "get", Group: "foo", Resource: "horizontalpodautoscalers"},
		},
		kubeAuthInterface: clusterAdminKubeClient.AuthorizationV1(),
		response: authorizationv1.SubjectAccessReviewResponse{
			Allowed:   false,
			Reason:    "",
			Namespace: "hammer-project",
		},
	}.run(t)
	subjectAccessReviewTest{
		description:    "cluster admin told harold cannot get horizontalpodautoscalers (with * API group) in project hammer-project",
		localInterface: clusterAdminAuthorizationClient.LocalSubjectAccessReviews("hammer-project"),
		localReview: &authorizationv1.LocalSubjectAccessReview{
			User:   "harold",
			Action: authorizationv1.Action{Verb: "get", Group: "*", Resource: "horizontalpodautoscalers"},
		},
		kubeAuthInterface: clusterAdminSARGetter,
		response: authorizationv1.SubjectAccessReviewResponse{
			Allowed:   false,
			Reason:    "",
			Namespace: "hammer-project",
		},
	}.run(t)

	// SAR honors API Group for cluster admin self SAR
	subjectAccessReviewTest{
		description:    "cluster admin told they can get autoscaling.horizontalpodautoscalers in project hammer-project",
		localInterface: clusterAdminAuthorizationClient.LocalSubjectAccessReviews("any-project"),
		localReview: &authorizationv1.LocalSubjectAccessReview{
			Action: authorizationv1.Action{Verb: "get", Group: "autoscaling", Resource: "horizontalpodautoscalers"},
		},
		kubeAuthInterface: clusterAdminSARGetter,
		response: authorizationv1.SubjectAccessReviewResponse{
			Allowed:   true,
			Reason:    "",
			Namespace: "any-project",
		},
	}.run(t)
	subjectAccessReviewTest{
		description:    "cluster admin told they can get horizontalpodautoscalers (with no API group) in project any-project",
		localInterface: clusterAdminAuthorizationClient.LocalSubjectAccessReviews("any-project"),
		localReview: &authorizationv1.LocalSubjectAccessReview{
			Action: authorizationv1.Action{Verb: "get", Group: "", Resource: "horizontalpodautoscalers"},
		},
		kubeAuthInterface: clusterAdminSARGetter,
		response: authorizationv1.SubjectAccessReviewResponse{
			Allowed:   true,
			Reason:    "",
			Namespace: "any-project",
		},
	}.run(t)
	subjectAccessReviewTest{
		description:    "cluster admin told they can get horizontalpodautoscalers (with invalid API group) in project any-project",
		localInterface: clusterAdminAuthorizationClient.LocalSubjectAccessReviews("any-project"),
		localReview: &authorizationv1.LocalSubjectAccessReview{
			Action: authorizationv1.Action{Verb: "get", Group: "foo", Resource: "horizontalpodautoscalers"},
		},
		kubeAuthInterface: clusterAdminSARGetter,
		response: authorizationv1.SubjectAccessReviewResponse{
			Allowed:   true,
			Reason:    "",
			Namespace: "any-project",
		},
	}.run(t)
	subjectAccessReviewTest{
		description:    "cluster admin told they can get horizontalpodautoscalers (with * API group) in project any-project",
		localInterface: clusterAdminAuthorizationClient.LocalSubjectAccessReviews("any-project"),
		localReview: &authorizationv1.LocalSubjectAccessReview{
			Action: authorizationv1.Action{Verb: "get", Group: "*", Resource: "horizontalpodautoscalers"},
		},
		kubeAuthInterface: clusterAdminSARGetter,
		response: authorizationv1.SubjectAccessReviewResponse{
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
	clusterAdminLocalSARGetter := clusterAdminKubeClient.AuthorizationV1()

	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	clusterAdminAuthorizationClient := authorizationv1client.NewForConfigOrDie(clusterAdminClientConfig).AuthorizationV1()

	_, haroldConfig, err := testserver.CreateNewProject(clusterAdminClientConfig, "hammer-project", "harold")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	haroldKubeClient, _, err := testutil.GetClientForUser(clusterAdminClientConfig, "harold")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	haroldAuthorizationClient := authorizationv1client.NewForConfigOrDie(haroldConfig).AuthorizationV1()
	haroldSARGetter := haroldKubeClient.AuthorizationV1()

	_, markConfig, err := testserver.CreateNewProject(clusterAdminClientConfig, "mallet-project", "mark")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	markKubeClient, _, err := testutil.GetClientForUser(clusterAdminClientConfig, "mark")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	markAuthorizationClient := authorizationv1client.NewForConfigOrDie(markConfig).AuthorizationV1()
	markSARGetter := markKubeClient.AuthorizationV1()

	dannyKubeClient, dannyConfig, err := testutil.GetClientForUser(clusterAdminClientConfig, "danny")
	if err != nil {
		t.Fatalf("error requesting token: %v", err)
	}
	dannyAuthorizationClient := authorizationv1client.NewForConfigOrDie(dannyConfig).AuthorizationV1()
	dannySARGetter := dannyKubeClient.AuthorizationV1()

	anonymousConfig := rest.AnonymousClientConfig(clusterAdminClientConfig)
	anonymousAuthorizationClient := authorizationv1client.NewForConfigOrDie(anonymousConfig).AuthorizationV1()
	anonymousKubeClient, err := kubernetes.NewForConfig(anonymousConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	anonymousSARGetter := anonymousKubeClient.AuthorizationV1()

	addAnonymous := &policy.RoleModificationOptions{
		RoleBindingNamespace: "hammer-project",
		RoleName:             "edit",
		RoleKind:             "ClusterRole",
		RbacClient:           rbacv1client.NewForConfigOrDie(clusterAdminClientConfig),
		Users:                []string{"system:anonymous"},
		PrintFlags:           genericclioptions.NewPrintFlags(""),
		ToPrinter:            func(string) (printers.ResourcePrinter, error) { return printers.NewDiscardingPrinter(), nil },
	}
	if err := addAnonymous.AddRole(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	addDanny := &policy.RoleModificationOptions{
		RoleBindingNamespace: "default",
		RoleName:             "view",
		RoleKind:             "ClusterRole",
		RbacClient:           rbacv1client.NewForConfigOrDie(clusterAdminClientConfig),
		Users:                []string{"danny"},
		PrintFlags:           genericclioptions.NewPrintFlags(""),
		ToPrinter:            func(string) (printers.ResourcePrinter, error) { return printers.NewDiscardingPrinter(), nil },
	}
	if err := addDanny.AddRole(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	askCanDannyGetProject := &authorizationv1.SubjectAccessReview{
		User:   "danny",
		Action: authorizationv1.Action{Verb: "get", Resource: "projects"},
	}
	subjectAccessReviewTest{
		description:    "cluster admin told danny can get project default",
		localInterface: clusterAdminAuthorizationClient.LocalSubjectAccessReviews("default"),
		localReview: &authorizationv1.LocalSubjectAccessReview{
			User:   "danny",
			Action: authorizationv1.Action{Verb: "get", Resource: "projects"},
		},
		kubeAuthInterface: clusterAdminLocalSARGetter,
		response: authorizationv1.SubjectAccessReviewResponse{
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
		response: authorizationv1.SubjectAccessReviewResponse{
			Allowed:   false,
			Reason:    "",
			Namespace: "",
		},
	}.run(t)
	subjectAccessReviewTest{
		description:       "as danny, can I make cluster subject access reviews",
		clusterInterface:  dannyAuthorizationClient.SubjectAccessReviews(),
		clusterReview:     askCanDannyGetProject,
		kubeAuthInterface: dannySARGetter,
		err:               `subjectaccessreviews.authorization.openshift.io is forbidden: User "danny" cannot create resource "subjectaccessreviews" in API group "authorization.openshift.io" at the cluster scope`,
		kubeErr:           `subjectaccessreviews.authorization.k8s.io is forbidden: User "danny" cannot create resource "subjectaccessreviews" in API group "authorization.k8s.io" at the cluster scope`,
	}.run(t)
	subjectAccessReviewTest{
		description:       "as anonymous, can I make cluster subject access reviews",
		clusterInterface:  anonymousAuthorizationClient.SubjectAccessReviews(),
		clusterReview:     askCanDannyGetProject,
		kubeAuthInterface: anonymousSARGetter,
		err:               `subjectaccessreviews.authorization.openshift.io is forbidden: User "system:anonymous" cannot create resource "subjectaccessreviews" in API group "authorization.openshift.io" at the cluster scope`,
		kubeErr:           `subjectaccessreviews.authorization.k8s.io is forbidden: User "system:anonymous" cannot create resource "subjectaccessreviews" in API group "authorization.k8s.io" at the cluster scope`,
	}.run(t)

	addValerie := &policy.RoleModificationOptions{
		RoleBindingNamespace: "hammer-project",
		RoleName:             "view",
		RoleKind:             "ClusterRole",
		RbacClient:           rbacv1client.NewForConfigOrDie(haroldConfig),
		Users:                []string{"valerie"},
		PrintFlags:           genericclioptions.NewPrintFlags(""),
		ToPrinter:            func(string) (printers.ResourcePrinter, error) { return printers.NewDiscardingPrinter(), nil },
	}
	if err := addValerie.AddRole(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	addEdgar := &policy.RoleModificationOptions{
		RoleBindingNamespace: "mallet-project",
		RoleName:             "edit",
		RoleKind:             "ClusterRole",
		RbacClient:           rbacv1client.NewForConfigOrDie(markConfig),
		Users:                []string{"edgar"},
		PrintFlags:           genericclioptions.NewPrintFlags(""),
		ToPrinter:            func(string) (printers.ResourcePrinter, error) { return printers.NewDiscardingPrinter(), nil },
	}
	if err := addEdgar.AddRole(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	askCanValerieGetProject := &authorizationv1.LocalSubjectAccessReview{
		User:   "valerie",
		Action: authorizationv1.Action{Verb: "get", Resource: "projects"},
	}
	subjectAccessReviewTest{
		description:       "harold told valerie can get project hammer-project",
		localInterface:    haroldAuthorizationClient.LocalSubjectAccessReviews("hammer-project"),
		localReview:       askCanValerieGetProject,
		kubeAuthInterface: haroldSARGetter,
		response: authorizationv1.SubjectAccessReviewResponse{
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
		response: authorizationv1.SubjectAccessReviewResponse{
			Allowed:   false,
			Reason:    "",
			Namespace: "mallet-project",
		},
	}.run(t)

	askCanEdgarDeletePods := &authorizationv1.LocalSubjectAccessReview{
		User:   "edgar",
		Action: authorizationv1.Action{Verb: "delete", Resource: "pods"},
	}
	subjectAccessReviewTest{
		description:       "mark told edgar can delete pods in mallet-project",
		localInterface:    markAuthorizationClient.LocalSubjectAccessReviews("mallet-project"),
		localReview:       askCanEdgarDeletePods,
		kubeAuthInterface: markSARGetter,
		response: authorizationv1.SubjectAccessReviewResponse{
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
		err:               `localsubjectaccessreviews.authorization.openshift.io is forbidden: User "harold" cannot create resource "localsubjectaccessreviews" in API group "authorization.openshift.io" in the namespace "mallet-project"`,
		kubeErr:           `localsubjectaccessreviews.authorization.k8s.io is forbidden: User "harold" cannot create resource "localsubjectaccessreviews" in API group "authorization.k8s.io" in the namespace "mallet-project"`,
	}.run(t)
	subjectAccessReviewTest{
		description:       "system:anonymous denied ability to run subject access review in project mallet-project",
		localInterface:    anonymousAuthorizationClient.LocalSubjectAccessReviews("mallet-project"),
		localReview:       askCanEdgarDeletePods,
		kubeAuthInterface: anonymousSARGetter,
		kubeNamespace:     "mallet-project",
		err:               `localsubjectaccessreviews.authorization.openshift.io is forbidden: User "system:anonymous" cannot create resource "localsubjectaccessreviews" in API group "authorization.openshift.io" in the namespace "mallet-project"`,
		kubeErr:           `localsubjectaccessreviews.authorization.k8s.io is forbidden: User "system:anonymous" cannot create resource "localsubjectaccessreviews" in API group "authorization.k8s.io" in the namespace "mallet-project"`,
	}.run(t)
	// ensure message does not leak whether the namespace exists or not
	subjectAccessReviewTest{
		description:       "harold denied ability to run subject access review in project nonexistent-project",
		localInterface:    haroldAuthorizationClient.LocalSubjectAccessReviews("nonexistent-project"),
		localReview:       askCanEdgarDeletePods,
		kubeAuthInterface: haroldSARGetter,
		kubeNamespace:     "nonexistent-project",
		err:               `localsubjectaccessreviews.authorization.openshift.io is forbidden: User "harold" cannot create resource "localsubjectaccessreviews" in API group "authorization.openshift.io" in the namespace "nonexistent-project"`,
		kubeErr:           `localsubjectaccessreviews.authorization.k8s.io is forbidden: User "harold" cannot create resource "localsubjectaccessreviews" in API group "authorization.k8s.io" in the namespace "nonexistent-project"`,
	}.run(t)
	subjectAccessReviewTest{
		description:       "system:anonymous denied ability to run subject access review in project nonexistent-project",
		localInterface:    anonymousAuthorizationClient.LocalSubjectAccessReviews("nonexistent-project"),
		localReview:       askCanEdgarDeletePods,
		kubeAuthInterface: anonymousSARGetter,
		kubeNamespace:     "nonexistent-project",
		err:               `localsubjectaccessreviews.authorization.openshift.io is forbidden: User "system:anonymous" cannot create resource "localsubjectaccessreviews" in API group "authorization.openshift.io" in the namespace "nonexistent-project"`,
		kubeErr:           `localsubjectaccessreviews.authorization.k8s.io is forbidden: User "system:anonymous" cannot create resource "localsubjectaccessreviews" in API group "authorization.k8s.io" in the namespace "nonexistent-project"`,
	}.run(t)

	askCanHaroldUpdateProject := &authorizationv1.LocalSubjectAccessReview{
		User:   "harold",
		Action: authorizationv1.Action{Verb: "update", Resource: "projects"},
	}
	subjectAccessReviewTest{
		description:       "harold told harold can update project hammer-project",
		localInterface:    haroldAuthorizationClient.LocalSubjectAccessReviews("hammer-project"),
		localReview:       askCanHaroldUpdateProject,
		kubeAuthInterface: haroldSARGetter,
		response: authorizationv1.SubjectAccessReviewResponse{
			Allowed:   true,
			Reason:    `RBAC: allowed by RoleBinding "admin/hammer-project" of ClusterRole "admin" to User "harold"`,
			Namespace: "hammer-project",
		},
	}.run(t)

	askCanClusterAdminsCreateProject := &authorizationv1.SubjectAccessReview{
		GroupsSlice: []string{"system:cluster-admins"},
		Action:      authorizationv1.Action{Verb: "create", Resource: "projects"},
	}
	subjectAccessReviewTest{
		description:       "cluster admin told cluster admins can create projects",
		clusterInterface:  clusterAdminAuthorizationClient.SubjectAccessReviews(),
		clusterReview:     askCanClusterAdminsCreateProject,
		kubeAuthInterface: clusterAdminLocalSARGetter,
		response: authorizationv1.SubjectAccessReviewResponse{
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
		err:               `subjectaccessreviews.authorization.openshift.io is forbidden: User "harold" cannot create resource "subjectaccessreviews" in API group "authorization.openshift.io" at the cluster scope`,
		kubeErr:           `subjectaccessreviews.authorization.k8s.io is forbidden: User "harold" cannot create resource "subjectaccessreviews" in API group "authorization.k8s.io" at the cluster scope`,
	}.run(t)

	askCanICreatePods := &authorizationv1.LocalSubjectAccessReview{
		Action: authorizationv1.Action{Verb: "create", Resource: "pods"},
	}
	subjectAccessReviewTest{
		description:       "harold told he can create pods in project hammer-project",
		localInterface:    haroldAuthorizationClient.LocalSubjectAccessReviews("hammer-project"),
		localReview:       askCanICreatePods,
		kubeAuthInterface: haroldSARGetter,
		response: authorizationv1.SubjectAccessReviewResponse{
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
		response: authorizationv1.SubjectAccessReviewResponse{
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
		response: authorizationv1.SubjectAccessReviewResponse{
			Allowed:   false,
			Reason:    "",
			Namespace: "mallet-project",
		},
	}.run(t)
	subjectAccessReviewTest{
		description:       "system:anonymous told he cannot create pods in project mallet-project",
		localInterface:    anonymousAuthorizationClient.LocalSubjectAccessReviews("mallet-project"),
		localReview:       askCanICreatePods,
		kubeAuthInterface: anonymousSARGetter,
		response: authorizationv1.SubjectAccessReviewResponse{
			Allowed:   false,
			Reason:    "",
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
		response: authorizationv1.SubjectAccessReviewResponse{
			Allowed:   false,
			Reason:    "",
			Namespace: "nonexistent-project",
		},
	}.run(t)
	subjectAccessReviewTest{
		description:       "system:anonymous told he cannot create pods in project nonexistent-project",
		localInterface:    anonymousAuthorizationClient.LocalSubjectAccessReviews("nonexistent-project"),
		localReview:       askCanICreatePods,
		kubeAuthInterface: anonymousSARGetter,
		response: authorizationv1.SubjectAccessReviewResponse{
			Allowed:   false,
			Reason:    "",
			Namespace: "nonexistent-project",
		},
	}.run(t)

	askCanICreatePolicyBindings := &authorizationv1.LocalSubjectAccessReview{
		Action: authorizationv1.Action{Verb: "create", Resource: "policybindings"},
	}
	subjectAccessReviewTest{
		description:       "harold told he can create policybindings in project hammer-project",
		localInterface:    haroldAuthorizationClient.LocalSubjectAccessReviews("hammer-project"),
		kubeAuthInterface: haroldSARGetter,
		localReview:       askCanICreatePolicyBindings,
		response: authorizationv1.SubjectAccessReviewResponse{
			Allowed:   false,
			Reason:    "",
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
		return strings.Contains(errProxy.Error(), `cannot proxy resource "pods" in API group "" in the namespace "ns": proxy verb changed to unsafeproxy`) ||
			strings.Contains(errProxy.Error(), `cannot get resource "pods/proxy" in API group "" in the namespace "ns": proxy subresource changed to unsafeproxy`)
	}

	for _, tc := range []struct {
		name   string
		client rest.Interface
		path   []string

		expectUnsafe bool
	}{
		{
			name:   "safe to proxy verb",
			client: userClient.CoreV1().RESTClient(),
			path:   proxyVerb,

			expectUnsafe: false,
		},
		{
			name:   "safe to proxy subresource",
			client: userClient.CoreV1().RESTClient(),
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
