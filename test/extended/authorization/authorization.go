package authorization

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	kubeauthorizationv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	kapierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	kauthorizationv1client "k8s.io/client-go/kubernetes/typed/authorization/v1"
	rbacv1client "k8s.io/client-go/kubernetes/typed/rbac/v1"
	"k8s.io/client-go/rest"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	appsapi "k8s.io/kubernetes/pkg/apis/apps"
	extensionsapi "k8s.io/kubernetes/pkg/apis/extensions"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	oapps "github.com/openshift/api/apps"
	authorizationv1 "github.com/openshift/api/authorization/v1"
	"github.com/openshift/api/build"
	configv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/api/image"
	"github.com/openshift/api/oauth"
	projectv1 "github.com/openshift/api/project/v1"
	authorizationv1client "github.com/openshift/client-go/authorization/clientset/versioned"
	authorizationv1typedclient "github.com/openshift/client-go/authorization/clientset/versioned/typed/authorization/v1"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/ibmcloud"
)

var _ = g.Describe("[sig-auth][Feature:OpenShiftAuthorization] authorization", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLI("bootstrap-policy")

	g.Context("", func() {
		g.Describe("TestClusterReaderCoverage", func() {
			g.It(fmt.Sprintf("should succeed"), func() {
				g.Skip("this test was in integration and didn't cover a real configuration, so it's horribly, horribly wrong now")

				t := g.GinkgoT()

				clusterAdminClientConfig := oc.AdminConfig()
				discoveryClient := discovery.NewDiscoveryClientForConfigOrDie(clusterAdminClientConfig)

				// (map[string]*metav1.APIResourceList, error)
				_, allResourceList, err := discoveryClient.ServerGroupsAndResources()
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

				readerRole, err := rbacv1client.NewForConfigOrDie(clusterAdminClientConfig).ClusterRoles().Get(context.Background(), "cluster-reader", metav1.GetOptions{})
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

			})
		})
	})
})

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

	return fmt.Sprintf("%s", nsStr+usersStr+groupsStr)
}

// This list includes the admins from above, plus users or groups known to have global view access
var globalClusterReaderUsers = sets.NewString("system:admin")
var globalClusterReaderGroups = sets.NewString("system:cluster-readers", "system:cluster-admins", "system:masters")
var hyperShiftServiceAccount = sets.NewString("system:hosted-cluster-config")

// this list includes any other users who can get DeploymentConfigs
var globalDeploymentConfigGetterUsers = sets.NewString(
	"system:serviceaccount:kube-system:generic-garbage-collector",
	"system:serviceaccount:kube-system:namespace-controller",
)

type resourceAccessReviewTest struct {
	description     string
	clientInterface authorizationv1typedclient.ResourceAccessReviewInterface
	ocClient        *exutil.CLI
	review          *authorizationv1.ResourceAccessReview

	response authorizationv1.ResourceAccessReviewResponse
	err      string
}

func (test resourceAccessReviewTest) run() {
	PolicyCachePollInterval := 100 * time.Millisecond
	PolicyCachePollTimeout := 10 * time.Second
	failMessage := ""
	var err error

	g.By("resourceAccessReviewTest - "+test.description, func() {
		// keep trying the test until you get a success or you timeout.  Every time you have a failure, set the fail message
		// so that if you never have a success, we can call t.Errorf with a reasonable message
		// exiting the poll with `failMessage=""` indicates success.
		err = wait.Poll(PolicyCachePollInterval, PolicyCachePollTimeout, func() (bool, error) {
			actualResponse, err := test.clientInterface.Create(context.Background(), test.review, metav1.CreateOptions{})
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

			controlPlaneTopology, err := exutil.GetControlPlaneTopology(test.ocClient)
			o.Expect(err).NotTo(o.HaveOccurred())

			// many operators installed in openshift- namespaces are allowed to read all namespaces.  We simply suppress all those users from the user slice
			// so we aren't sensitive to adding or removing them
			actualUsersToCheck := sets.NewString()
			for _, curr := range actualResponse.UsersSlice {
				if strings.HasPrefix(curr, "system:serviceaccount:openshift-") {
					continue
				}
				// Managed ibmcloud openshift has an IAM user that needs to be ignored
				if strings.HasPrefix(curr, "IAM#") {
					if *controlPlaneTopology == configv1.ExternalTopologyMode && e2e.TestContext.Provider == ibmcloud.ProviderName {
						continue
					}
				}
				actualUsersToCheck.Insert(curr)
			}

			if actualResponse.Namespace != test.response.Namespace ||
				!reflect.DeepEqual(actualUsersToCheck, sets.NewString(test.response.UsersSlice...)) ||
				!reflect.DeepEqual(sets.NewString(actualResponse.GroupsSlice...), sets.NewString(test.response.GroupsSlice...)) ||
				actualResponse.EvaluationError != test.response.EvaluationError {
				failMessage = fmt.Sprintf("%s:\n  %s:\n  expected %s\n  got %s", test.description, prettyPrintAction(&test.review.Action, "(in any namespace)"), prettyPrintReviewResponse(&test.response), prettyPrintReviewResponse(actualResponse))
				return false, nil
			}

			failMessage = ""
			return true, nil
		})
	})

	if err != nil {
		if len(failMessage) != 0 {
			g.Fail(failMessage)
		}
		g.Fail(err.Error())
	}
	if len(failMessage) != 0 {
		g.Fail(failMessage)
	}

}

type localResourceAccessReviewTest struct {
	description     string
	clientInterface authorizationv1typedclient.LocalResourceAccessReviewInterface
	ocClient        *exutil.CLI
	review          *authorizationv1.LocalResourceAccessReview

	response authorizationv1.ResourceAccessReviewResponse
	err      string
}

func (test localResourceAccessReviewTest) run() {
	PolicyCachePollInterval := 100 * time.Millisecond
	PolicyCachePollTimeout := 10 * time.Second
	failMessage := ""
	var err error

	g.By("localResourceAccessReviewTest - "+test.description+" [apigroup:authorization.openshift.io]", func() {
		// keep trying the test until you get a success or you timeout.  Every time you have a failure, set the fail message
		// so that if you never have a success, we can call t.Errorf with a reasonable message
		// exiting the poll with `failMessage=""` indicates success.
		err = wait.Poll(PolicyCachePollInterval, PolicyCachePollTimeout, func() (bool, error) {
			actualResponse, err := test.clientInterface.Create(context.Background(), test.review, metav1.CreateOptions{})
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

			controlPlaneTopology, err := exutil.GetControlPlaneTopology(test.ocClient)
			o.Expect(err).NotTo(o.HaveOccurred())

			// many operators installed in openshift- namespaces are allowed to read all namespaces.  We simply suppress all those users from the user slice
			// so we aren't sensitive to adding or removing them
			actualUsersToCheck := sets.NewString()
			for _, curr := range actualResponse.UsersSlice {
				if strings.HasPrefix(curr, "system:serviceaccount:openshift-") {
					continue
				}
				// Managed ibmcloud openshift has an IAM user that needs to be ignored
				if strings.HasPrefix(curr, "IAM#") {
					if *controlPlaneTopology == configv1.ExternalTopologyMode && e2e.TestContext.Provider == ibmcloud.ProviderName {
						continue
					}
				}
				// skip the ci provisioner to pass gcp: ci-provisioner@openshift-gce-devel-ci.iam.gserviceaccount.com
				if strings.HasPrefix(curr, "ci-provisioner@") {
					continue
				}
				actualUsersToCheck.Insert(curr)
			}
			if !reflect.DeepEqual(actualUsersToCheck, sets.NewString(test.response.UsersSlice...)) {
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
	})

	if err != nil {
		if len(failMessage) != 0 {
			g.Fail(failMessage)
		}
		g.Fail(err.Error())
	}
	if len(failMessage) != 0 {
		g.Fail(failMessage)
	}
}

// serial because it is vulnerable to access added by other tests
var _ = g.Describe("[sig-auth][Feature:OpenShiftAuthorization][Serial] authorization", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLI("bootstrap-policy")

	g.Context("", func() {
		g.Describe("TestAuthorizationResourceAccessReview", func() {
			g.It(fmt.Sprintf("should succeed [apigroup:authorization.openshift.io]"), func() {
				clusterAdminAuthorizationClient := oc.AdminAuthorizationClient().AuthorizationV1()

				hammerProjectName := createProject(oc, "hammer")
				haroldName := oc.CreateUser("harold-").Name
				haroldConfig := oc.GetClientConfigForUser(haroldName)
				haroldAuthorizationClient := authorizationv1client.NewForConfigOrDie(haroldConfig).AuthorizationV1()
				AddUserAdminToProject(oc, hammerProjectName, haroldName)

				malletProjectName := createProject(oc, "mallet")
				markName := oc.CreateUser("mark-").Name
				markConfig := oc.GetClientConfigForUser(markName)
				markAuthorizationClient := authorizationv1client.NewForConfigOrDie(markConfig).AuthorizationV1()
				AddUserAdminToProject(oc, malletProjectName, markName)

				valerieName := oc.CreateUser("valerie-").Name
				AddUserViewToProject(oc, hammerProjectName, valerieName)
				edgarName := oc.CreateUser("edgar-").Name
				AddUserEditToProject(oc, malletProjectName, edgarName)

				requestWhoCanViewDeploymentConfigs := &authorizationv1.ResourceAccessReview{
					Action: authorizationv1.Action{Verb: "get", Resource: "deploymentconfigs", Group: ""},
				}

				localRequestWhoCanViewDeploymentConfigs := &authorizationv1.LocalResourceAccessReview{
					Action: authorizationv1.Action{Verb: "get", Resource: "deploymentconfigs", Group: ""},
				}

				{
					test := localResourceAccessReviewTest{
						description:     "who can view deploymentconfigs in hammer by harold",
						clientInterface: haroldAuthorizationClient.LocalResourceAccessReviews(hammerProjectName),
						ocClient:        oc,
						review:          localRequestWhoCanViewDeploymentConfigs,
						response: authorizationv1.ResourceAccessReviewResponse{
							UsersSlice:  []string{oc.Username(), haroldName, valerieName},
							GroupsSlice: []string{},
							Namespace:   hammerProjectName,
						},
					}
					if ok, _ := exutil.IsHypershift(context.Background(), oc.AdminConfigClient()); ok {
						test.response.UsersSlice = append(test.response.UsersSlice, hyperShiftServiceAccount.List()...)
					}
					test.response.UsersSlice = append(test.response.UsersSlice, globalClusterReaderUsers.List()...)
					test.response.UsersSlice = append(test.response.UsersSlice, globalDeploymentConfigGetterUsers.List()...)
					test.response.GroupsSlice = append(test.response.GroupsSlice, globalClusterReaderGroups.List()...)
					test.run()
				}
				{
					test := localResourceAccessReviewTest{
						description:     "who can view deploymentconfigs in mallet by mark",
						clientInterface: markAuthorizationClient.LocalResourceAccessReviews(malletProjectName),
						ocClient:        oc,
						review:          localRequestWhoCanViewDeploymentConfigs,
						response: authorizationv1.ResourceAccessReviewResponse{
							UsersSlice:  []string{oc.Username(), markName, edgarName},
							GroupsSlice: []string{},
							Namespace:   malletProjectName,
						},
					}
					if ok, _ := exutil.IsHypershift(context.Background(), oc.AdminConfigClient()); ok {
						test.response.UsersSlice = append(test.response.UsersSlice, hyperShiftServiceAccount.List()...)
					}
					test.response.UsersSlice = append(test.response.UsersSlice, globalClusterReaderUsers.List()...)
					test.response.UsersSlice = append(test.response.UsersSlice, globalDeploymentConfigGetterUsers.List()...)
					test.response.GroupsSlice = append(test.response.GroupsSlice, globalClusterReaderGroups.List()...)
					test.run()
				}

				// mark should not be able to make global access review requests
				{
					test := resourceAccessReviewTest{
						description:     "who can view deploymentconfigs in all by mark",
						clientInterface: markAuthorizationClient.ResourceAccessReviews(),
						ocClient:        oc,
						review:          requestWhoCanViewDeploymentConfigs,
						err:             "cannot ",
					}
					test.run()
				}

				// a cluster-admin should be able to make global access review requests
				{
					test := resourceAccessReviewTest{
						description:     "who can view deploymentconfigs in all by cluster-admin",
						clientInterface: clusterAdminAuthorizationClient.ResourceAccessReviews(),
						ocClient:        oc,
						review:          requestWhoCanViewDeploymentConfigs,
						response: authorizationv1.ResourceAccessReviewResponse{
							UsersSlice:  []string{},
							GroupsSlice: []string{},
						},
					}
					if ok, _ := exutil.IsHypershift(context.Background(), oc.AdminConfigClient()); ok {
						test.response.UsersSlice = append(test.response.UsersSlice, hyperShiftServiceAccount.List()...)
					}
					test.response.UsersSlice = append(test.response.UsersSlice, globalClusterReaderUsers.List()...)
					test.response.UsersSlice = append(test.response.UsersSlice, globalDeploymentConfigGetterUsers.List()...)
					test.response.GroupsSlice = append(test.response.GroupsSlice, globalClusterReaderGroups.List()...)
					test.run()
				}
			})
		})
	})
})

var _ = g.Describe("[sig-auth][Feature:OpenShiftAuthorization] authorization", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLI("bootstrap-policy")

	g.Context("", func() {
		g.Describe("TestAuthorizationSubjectAccessReview", func() {
			g.It(fmt.Sprintf("should succeed [apigroup:authorization.openshift.io]"), func() {
				t := g.GinkgoT()

				clusterAdminLocalSARGetter := oc.AdminKubeClient().AuthorizationV1()
				clusterAdminAuthorizationClient := oc.AdminAuthorizationClient().AuthorizationV1()

				g.By("creating projects")
				hammerProjectName := createProject(oc, "hammer")
				malletProjectName := createProject(oc, "mallet")

				haroldName := oc.CreateUser("harold-").Name
				markName := oc.CreateUser("mark-").Name
				dannyName := oc.CreateUser("danny-").Name
				edgarName := oc.CreateUser("edgar-").Name
				valerieName := oc.CreateUser("valerie-").Name

				g.By("adding user permissions")
				haroldAdminRoleBindingName := AddUserAdminToProject(oc, hammerProjectName, haroldName)
				// TODO should be done by harold
				valerieViewRoleBindingName := AddUserViewToProject(oc, hammerProjectName, valerieName)
				AddUserAdminToProject(oc, malletProjectName, markName)
				// TODO should be done by mark
				edgarEditRoleBindingName := AddUserEditToProject(oc, malletProjectName, edgarName)
				dannyViewRoleBindingName := AddUserViewToProject(oc, "default", dannyName)

				g.By("creating clients")
				haroldConfig := oc.GetClientConfigForUser(haroldName)
				haroldAuthorizationClient := authorizationv1typedclient.NewForConfigOrDie(haroldConfig)
				haroldSARGetter := kubernetes.NewForConfigOrDie(haroldConfig).AuthorizationV1()

				markConfig := oc.GetClientConfigForUser(markName)
				markAuthorizationClient := authorizationv1typedclient.NewForConfigOrDie(markConfig)
				markSARGetter := kubernetes.NewForConfigOrDie(markConfig).AuthorizationV1()

				anonymousConfig := rest.AnonymousClientConfig(oc.AdminConfig())
				anonymousAuthorizationClient := authorizationv1typedclient.NewForConfigOrDie(anonymousConfig)
				anonymousSARGetter := kubernetes.NewForConfigOrDie(anonymousConfig).AuthorizationV1()

				dannyConfig := oc.GetClientConfigForUser(dannyName)
				dannyAuthorizationClient := authorizationv1typedclient.NewForConfigOrDie(dannyConfig)
				dannySARGetter := kubernetes.NewForConfigOrDie(dannyConfig).AuthorizationV1()

				askCanDannyGetProject := &authorizationv1.SubjectAccessReview{
					User:   dannyName,
					Action: authorizationv1.Action{Verb: "get", Resource: "projects"},
				}

				subjectAccessReviewTest{
					description:    "cluster admin told danny can get project default",
					localInterface: clusterAdminAuthorizationClient.LocalSubjectAccessReviews("default"),
					localReview: &authorizationv1.LocalSubjectAccessReview{
						User:   dannyName,
						Action: authorizationv1.Action{Verb: "get", Resource: "projects"},
					},
					kubeAuthInterface: clusterAdminLocalSARGetter,
					response: authorizationv1.SubjectAccessReviewResponse{
						Allowed:   true,
						Reason:    `RBAC: allowed by RoleBinding "` + dannyViewRoleBindingName + `/default" of ClusterRole "view" to User "` + dannyName + `"`,
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
					err:               `subjectaccessreviews.authorization.openshift.io is forbidden: User "` + dannyName + `" cannot create resource "subjectaccessreviews" in API group "authorization.openshift.io" at the cluster scope`,
					kubeErr:           `subjectaccessreviews.authorization.k8s.io is forbidden: User "` + dannyName + `" cannot create resource "subjectaccessreviews" in API group "authorization.k8s.io" at the cluster scope`,
				}.run(t)
				subjectAccessReviewTest{
					description:       "as anonymous, can I make cluster subject access reviews",
					clusterInterface:  anonymousAuthorizationClient.SubjectAccessReviews(),
					clusterReview:     askCanDannyGetProject,
					kubeAuthInterface: anonymousSARGetter,
					err:               `subjectaccessreviews.authorization.openshift.io is forbidden: User "system:anonymous" cannot create resource "subjectaccessreviews" in API group "authorization.openshift.io" at the cluster scope`,
					kubeErr:           `subjectaccessreviews.authorization.k8s.io is forbidden: User "system:anonymous" cannot create resource "subjectaccessreviews" in API group "authorization.k8s.io" at the cluster scope`,
				}.run(t)

				askCanValerieGetProject := &authorizationv1.LocalSubjectAccessReview{
					User:   valerieName,
					Action: authorizationv1.Action{Verb: "get", Resource: "projects"},
				}
				subjectAccessReviewTest{
					description:       "harold told valerie can get project hammer-project",
					localInterface:    haroldAuthorizationClient.LocalSubjectAccessReviews(hammerProjectName),
					localReview:       askCanValerieGetProject,
					kubeAuthInterface: haroldSARGetter,
					response: authorizationv1.SubjectAccessReviewResponse{
						Allowed:   true,
						Reason:    `RBAC: allowed by RoleBinding "` + valerieViewRoleBindingName + `/` + hammerProjectName + `" of ClusterRole "view" to User "` + valerieName + `"`,
						Namespace: hammerProjectName,
					},
				}.run(t)
				subjectAccessReviewTest{
					description:       "mark told valerie cannot get project mallet-project",
					localInterface:    markAuthorizationClient.LocalSubjectAccessReviews(malletProjectName),
					localReview:       askCanValerieGetProject,
					kubeAuthInterface: markSARGetter,
					response: authorizationv1.SubjectAccessReviewResponse{
						Allowed:   false,
						Reason:    "",
						Namespace: malletProjectName,
					},
				}.run(t)

				askCanEdgarDeletePods := &authorizationv1.LocalSubjectAccessReview{
					User:   edgarName,
					Action: authorizationv1.Action{Verb: "delete", Resource: "pods"},
				}
				subjectAccessReviewTest{
					description:       "mark told edgar can delete pods in mallet-project",
					localInterface:    markAuthorizationClient.LocalSubjectAccessReviews(malletProjectName),
					localReview:       askCanEdgarDeletePods,
					kubeAuthInterface: markSARGetter,
					response: authorizationv1.SubjectAccessReviewResponse{
						Allowed:   true,
						Reason:    `RBAC: allowed by RoleBinding "` + edgarEditRoleBindingName + `/` + malletProjectName + `" of ClusterRole "edit" to User "` + edgarName + `"`,
						Namespace: malletProjectName,
					},
				}.run(t)
				// ensure unprivileged users cannot check other users' access
				subjectAccessReviewTest{
					description:       "harold denied ability to run subject access review in project mallet-project",
					localInterface:    haroldAuthorizationClient.LocalSubjectAccessReviews(malletProjectName),
					localReview:       askCanEdgarDeletePods,
					kubeAuthInterface: haroldSARGetter,
					kubeNamespace:     malletProjectName,
					err:               `localsubjectaccessreviews.authorization.openshift.io is forbidden: User "` + haroldName + `" cannot create resource "localsubjectaccessreviews" in API group "authorization.openshift.io" in the namespace "` + malletProjectName + `"`,
					kubeErr:           `localsubjectaccessreviews.authorization.k8s.io is forbidden: User "` + haroldName + `" cannot create resource "localsubjectaccessreviews" in API group "authorization.k8s.io" in the namespace "` + malletProjectName + `"`,
				}.run(t)
				subjectAccessReviewTest{
					description:       "system:anonymous denied ability to run subject access review in project mallet-project",
					localInterface:    anonymousAuthorizationClient.LocalSubjectAccessReviews(malletProjectName),
					localReview:       askCanEdgarDeletePods,
					kubeAuthInterface: anonymousSARGetter,
					kubeNamespace:     malletProjectName,
					err:               `localsubjectaccessreviews.authorization.openshift.io is forbidden: User "system:anonymous" cannot create resource "localsubjectaccessreviews" in API group "authorization.openshift.io" in the namespace "` + malletProjectName + `"`,
					kubeErr:           `localsubjectaccessreviews.authorization.k8s.io is forbidden: User "system:anonymous" cannot create resource "localsubjectaccessreviews" in API group "authorization.k8s.io" in the namespace "` + malletProjectName + `"`,
				}.run(t)
				// ensure message does not leak whether the namespace exists or not
				subjectAccessReviewTest{
					description:       "harold denied ability to run subject access review in project nonexistent-project",
					localInterface:    haroldAuthorizationClient.LocalSubjectAccessReviews("nonexistent-project"),
					localReview:       askCanEdgarDeletePods,
					kubeAuthInterface: haroldSARGetter,
					kubeNamespace:     "nonexistent-project",
					err:               `localsubjectaccessreviews.authorization.openshift.io is forbidden: User "` + haroldName + `" cannot create resource "localsubjectaccessreviews" in API group "authorization.openshift.io" in the namespace "nonexistent-project"`,
					kubeErr:           `localsubjectaccessreviews.authorization.k8s.io is forbidden: User "` + haroldName + `" cannot create resource "localsubjectaccessreviews" in API group "authorization.k8s.io" in the namespace "nonexistent-project"`,
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
					User:   haroldName,
					Action: authorizationv1.Action{Verb: "update", Resource: "projects"},
				}
				subjectAccessReviewTest{
					description:       "harold told harold can update project hammer-project",
					localInterface:    haroldAuthorizationClient.LocalSubjectAccessReviews(hammerProjectName),
					localReview:       askCanHaroldUpdateProject,
					kubeAuthInterface: haroldSARGetter,
					response: authorizationv1.SubjectAccessReviewResponse{
						Allowed:   true,
						Reason:    `RBAC: allowed by RoleBinding "` + haroldAdminRoleBindingName + `/` + hammerProjectName + `" of ClusterRole "admin" to User "` + haroldName + `"`,
						Namespace: hammerProjectName,
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
					err:               `subjectaccessreviews.authorization.openshift.io is forbidden: User "` + haroldName + `" cannot create resource "subjectaccessreviews" in API group "authorization.openshift.io" at the cluster scope`,
					kubeErr:           `subjectaccessreviews.authorization.k8s.io is forbidden: User "` + haroldName + `" cannot create resource "subjectaccessreviews" in API group "authorization.k8s.io" at the cluster scope`,
				}.run(t)

				askCanICreatePods := &authorizationv1.LocalSubjectAccessReview{
					Action: authorizationv1.Action{Verb: "create", Resource: "pods"},
				}
				subjectAccessReviewTest{
					description:       "harold told he can create pods in project hammer-project",
					localInterface:    haroldAuthorizationClient.LocalSubjectAccessReviews(hammerProjectName),
					localReview:       askCanICreatePods,
					kubeAuthInterface: haroldSARGetter,
					response: authorizationv1.SubjectAccessReviewResponse{
						Allowed:   true,
						Reason:    `RBAC: allowed by RoleBinding "` + haroldAdminRoleBindingName + `/` + hammerProjectName + `" of ClusterRole "admin" to User "` + haroldName + `"`,
						Namespace: hammerProjectName,
					},
				}.run(t)

				// test checking self permissions when denied
				subjectAccessReviewTest{
					description:       "harold told he cannot create pods in project mallet-project",
					localInterface:    haroldAuthorizationClient.LocalSubjectAccessReviews(malletProjectName),
					localReview:       askCanICreatePods,
					kubeAuthInterface: haroldSARGetter,
					response: authorizationv1.SubjectAccessReviewResponse{
						Allowed:   false,
						Reason:    "",
						Namespace: malletProjectName,
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

				askCanICreatePolicyBindings := &authorizationv1.LocalSubjectAccessReview{
					Action: authorizationv1.Action{Verb: "create", Resource: "policybindings"},
				}
				subjectAccessReviewTest{
					description:       "harold told he can create policybindings in project hammer-project",
					localInterface:    haroldAuthorizationClient.LocalSubjectAccessReviews(hammerProjectName),
					kubeAuthInterface: haroldSARGetter,
					localReview:       askCanICreatePolicyBindings,
					response: authorizationv1.SubjectAccessReviewResponse{
						Allowed:   false,
						Reason:    "",
						Namespace: hammerProjectName,
					},
				}.run(t)
			})
		})
	})
})

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

func (test subjectAccessReviewTest) run(t g.GinkgoTInterface) {
	PolicyCachePollInterval := 100 * time.Millisecond
	PolicyCachePollTimeout := 10 * time.Second

	g.By(test.description+" with openshift api [apigroup:authorization.openshift.io]", func() {
		failMessage := ""
		err := wait.Poll(PolicyCachePollInterval, PolicyCachePollTimeout, func() (bool, error) {
			var err error
			var actualResponse *authorizationv1.SubjectAccessReviewResponse
			if test.localReview != nil {
				actualResponse, err = test.localInterface.Create(context.Background(), test.localReview, metav1.CreateOptions{})
			} else {
				actualResponse, err = test.clusterInterface.Create(context.Background(), test.clusterReview, metav1.CreateOptions{})
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
			if len(failMessage) != 0 {
				t.Error(failMessage)
			}
			t.Error(err)
		}
		if len(failMessage) != 0 {
			t.Error(failMessage)
		}
	})

	if test.kubeAuthInterface != nil {
		g.By(test.description+" with kube api", func() {
			var testNS string
			if test.localReview != nil {
				switch {
				case len(test.localReview.Action.Namespace) > 0:
					testNS = test.localReview.Action.Namespace
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
			err := wait.Poll(PolicyCachePollInterval, PolicyCachePollTimeout, func() (bool, error) {
				var err error
				var actualResponse kubeauthorizationv1.SubjectAccessReviewStatus
				if test.localReview != nil {
					if len(test.localReview.User) == 0 && (len(test.localReview.GroupsSlice) == 0) {
						var tmp *kubeauthorizationv1.SelfSubjectAccessReview
						if tmp, err = test.kubeAuthInterface.SelfSubjectAccessReviews().Create(context.Background(), toKubeSelfSAR(testNS, test.localReview), metav1.CreateOptions{}); err == nil {
							actualResponse = tmp.Status
						}
					} else {
						var tmp *kubeauthorizationv1.LocalSubjectAccessReview
						if tmp, err = test.kubeAuthInterface.LocalSubjectAccessReviews(testNS).Create(context.Background(), toKubeLocalSAR(testNS, test.localReview), metav1.CreateOptions{}); err == nil {
							actualResponse = tmp.Status
						}
					}
				} else {
					var tmp *kubeauthorizationv1.SubjectAccessReview
					if tmp, err = test.kubeAuthInterface.SubjectAccessReviews().Create(context.Background(), toKubeClusterSAR(test.clusterReview), metav1.CreateOptions{}); err == nil {
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
		})

	} else if !test.kubeSkip {
		t.Errorf("%s: missing kube auth interface and test is not whitelisted", test.description)
	}
}

func createProject(oc *exutil.CLI, nsPrefix string) string {
	newNamespace := fmt.Sprintf("e2e-test-%s-%s", nsPrefix, utilrand.String(5))
	_, err := oc.ProjectClient().ProjectV1().ProjectRequests().Create(context.Background(), &projectv1.ProjectRequest{
		ObjectMeta: metav1.ObjectMeta{Name: newNamespace},
	}, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	return newNamespace
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

func AddUserToRoleInProject(oc *exutil.CLI, clusterrolebinding, namespace, user string) string {
	roleBinding := &authorizationv1.RoleBinding{}
	roleBinding.GenerateName = clusterrolebinding
	roleBinding.RoleRef.Name = clusterrolebinding
	roleBinding.Subjects = []corev1.ObjectReference{
		{Kind: "User", Name: user},
	}
	actual, err := oc.AdminAuthorizationClient().AuthorizationV1().RoleBindings(namespace).Create(context.Background(), roleBinding, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	err = oc.WaitForAccessAllowed(&kubeauthorizationv1.SelfSubjectAccessReview{
		Spec: kubeauthorizationv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &kubeauthorizationv1.ResourceAttributes{
				// TODO this works for now, but isn't logically correct
				Namespace: namespace,
				Verb:      "get",
				Group:     "",
				Resource:  "pods",
			},
		},
	}, user)
	o.Expect(err).NotTo(o.HaveOccurred())

	return actual.Name
}

func AddUserAdminToProject(oc *exutil.CLI, namespace, user string) string {
	return AddUserToRoleInProject(oc, "admin", namespace, user)
}

func AddUserEditToProject(oc *exutil.CLI, namespace, user string) string {
	return AddUserToRoleInProject(oc, "edit", namespace, user)
}

func AddUserViewToProject(oc *exutil.CLI, namespace, user string) string {
	return AddUserToRoleInProject(oc, "view", namespace, user)
}

var _ = g.Describe("[sig-auth][Feature:OpenShiftAuthorization] authorization", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLI("bootstrap-policy")

	g.Context("", func() {
		g.Describe("TestAuthorizationSubjectAccessReviewAPIGroup", func() {
			g.It(fmt.Sprintf("should succeed [apigroup:authorization.openshift.io]"), func() {
				t := g.GinkgoT()

				clusterAdminKubeClient := oc.AdminKubeClient()
				clusterAdminSARGetter := oc.AdminKubeClient().AuthorizationV1()
				clusterAdminAuthorizationClient := oc.AdminAuthorizationClient().AuthorizationV1()

				g.By("creating projects")
				hammerProjectName := createProject(oc, "hammer")
				haroldName := "harold-" + oc.Namespace()

				g.By("adding user permissions")
				haroldAdminRoleBindingName := AddUserAdminToProject(oc, hammerProjectName, haroldName)

				// SAR honors API Group
				subjectAccessReviewTest{
					description:    "cluster admin told harold can get autoscaling.horizontalpodautoscalers in project hammer-project",
					localInterface: clusterAdminAuthorizationClient.LocalSubjectAccessReviews(hammerProjectName),
					localReview: &authorizationv1.LocalSubjectAccessReview{
						User:   haroldName,
						Action: authorizationv1.Action{Verb: "get", Group: "autoscaling", Resource: "horizontalpodautoscalers"},
					},
					kubeAuthInterface: clusterAdminSARGetter,
					response: authorizationv1.SubjectAccessReviewResponse{
						Allowed:   true,
						Reason:    `RBAC: allowed by RoleBinding "` + haroldAdminRoleBindingName + `/` + hammerProjectName + `" of ClusterRole "admin" to User "` + haroldName + `"`,
						Namespace: hammerProjectName,
					},
				}.run(t)
				subjectAccessReviewTest{
					description:    "cluster admin told harold cannot get horizontalpodautoscalers (with no API group) in project hammer-project",
					localInterface: clusterAdminAuthorizationClient.LocalSubjectAccessReviews(hammerProjectName),
					localReview: &authorizationv1.LocalSubjectAccessReview{
						User:   haroldName,
						Action: authorizationv1.Action{Verb: "get", Group: "", Resource: "horizontalpodautoscalers"},
					},
					kubeAuthInterface: clusterAdminSARGetter,
					response: authorizationv1.SubjectAccessReviewResponse{
						Allowed:   false,
						Reason:    "",
						Namespace: hammerProjectName,
					},
				}.run(t)
				subjectAccessReviewTest{
					description:    "cluster admin told harold cannot get horizontalpodautoscalers (with invalid API group) in project hammer-project",
					localInterface: clusterAdminAuthorizationClient.LocalSubjectAccessReviews(hammerProjectName),
					localReview: &authorizationv1.LocalSubjectAccessReview{
						User:   haroldName,
						Action: authorizationv1.Action{Verb: "get", Group: "foo", Resource: "horizontalpodautoscalers"},
					},
					kubeAuthInterface: clusterAdminKubeClient.AuthorizationV1(),
					response: authorizationv1.SubjectAccessReviewResponse{
						Allowed:   false,
						Reason:    "",
						Namespace: hammerProjectName,
					},
				}.run(t)
				subjectAccessReviewTest{
					description:    "cluster admin told harold cannot get horizontalpodautoscalers (with * API group) in project hammer-project",
					localInterface: clusterAdminAuthorizationClient.LocalSubjectAccessReviews(hammerProjectName),
					localReview: &authorizationv1.LocalSubjectAccessReview{
						User:   haroldName,
						Action: authorizationv1.Action{Verb: "get", Group: "*", Resource: "horizontalpodautoscalers"},
					},
					kubeAuthInterface: clusterAdminSARGetter,
					response: authorizationv1.SubjectAccessReviewResponse{
						Allowed:   false,
						Reason:    "",
						Namespace: hammerProjectName,
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
			})
		})
	})
})

var _ = g.Describe("[sig-auth][Feature:OpenShiftAuthorization] authorization", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLI("bootstrap-policy")

	g.Context("", func() {
		g.Describe("TestBrowserSafeAuthorizer", func() {
			g.It(fmt.Sprintf("should succeed [apigroup:user.openshift.io]"), func() {
				t := g.GinkgoT()

				// this client has an API token so it is safe
				username := oc.CreateUser("someuser-").Name
				userClient := kubernetes.NewForConfigOrDie(oc.GetClientConfigForUser(username))

				// this client has no API token so it is unsafe (like a browser)
				anonymousConfig := rest.AnonymousClientConfig(oc.AdminConfig())
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
					errProxy := tc.client.Get().AbsPath(tc.path...).Do(context.Background()).Error()
					if errProxy == nil || !kapierror.IsForbidden(errProxy) || tc.expectUnsafe != isUnsafeErr(errProxy) {
						t.Errorf("%s: expected forbidden error on GET %s, got %#v (isForbidden=%v, expectUnsafe=%v, actualUnsafe=%v)",
							tc.name, tc.path, errProxy, kapierror.IsForbidden(errProxy), tc.expectUnsafe, isUnsafeErr(errProxy))
					}
				}
			})
		})
	})
})
