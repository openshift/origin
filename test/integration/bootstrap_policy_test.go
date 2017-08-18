package integration

import (
	"testing"

	kapierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/util/tokencmd"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func TestBootstrapPolicyAuthenticatedUsersAgainstOpenshiftNamespace(t *testing.T) {
	masterConfig, clusterAdminKubeConfig, err := testserver.StartTestMasterAPI()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer testserver.CleanupMasterEtcd(t, masterConfig)

	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	valerieClientConfig := *clusterAdminClientConfig
	valerieClientConfig.Username = ""
	valerieClientConfig.Password = ""
	valerieClientConfig.BearerToken = ""
	valerieClientConfig.CertFile = ""
	valerieClientConfig.KeyFile = ""
	valerieClientConfig.CertData = nil
	valerieClientConfig.KeyData = nil

	accessToken, err := tokencmd.RequestToken(&valerieClientConfig, nil, "valerie", "security!")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	valerieClientConfig.BearerToken = accessToken
	valerieOpenshiftClient, err := client.New(&valerieClientConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	openshiftSharedResourcesNamespace := "openshift"

	if _, err := valerieOpenshiftClient.Templates(openshiftSharedResourcesNamespace).List(metav1.ListOptions{}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if _, err := valerieOpenshiftClient.Templates(metav1.NamespaceDefault).List(metav1.ListOptions{}); err == nil || !kapierror.IsForbidden(err) {
		t.Errorf("unexpected error: %v", err)
	}

	if _, err := valerieOpenshiftClient.ImageStreams(openshiftSharedResourcesNamespace).List(metav1.ListOptions{}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if _, err := valerieOpenshiftClient.ImageStreams(metav1.NamespaceDefault).List(metav1.ListOptions{}); err == nil || !kapierror.IsForbidden(err) {
		t.Errorf("unexpected error: %v", err)
	}

	if _, err := valerieOpenshiftClient.ImageStreamTags(openshiftSharedResourcesNamespace).Get("name", "tag"); !kapierror.IsNotFound(err) {
		t.Errorf("unexpected error: %v", err)
	}
	if _, err := valerieOpenshiftClient.ImageStreamTags(metav1.NamespaceDefault).Get("name", "tag"); err == nil || !kapierror.IsForbidden(err) {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestBootstrapPolicySelfSubjectAccessReviews(t *testing.T) {
	masterConfig, clusterAdminKubeConfig, err := testserver.StartTestMasterAPI()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer testserver.CleanupMasterEtcd(t, masterConfig)

	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	valerieOpenshiftClient, valerieKubeClient, _, err := testutil.GetClientForUser(*clusterAdminClientConfig, "valerie")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	askCanICreatePolicyBindings := &authorizationapi.LocalSubjectAccessReview{
		Action: authorizationapi.Action{Verb: "create", Resource: "policybindings"},
	}
	subjectAccessReviewTest{
		description:       "can I get a subjectaccessreview on myself even if I have no rights to do it generally",
		localInterface:    valerieOpenshiftClient.LocalSubjectAccessReviews("openshift"),
		localReview:       askCanICreatePolicyBindings,
		kubeAuthInterface: valerieKubeClient.Authorization(),
		response: authorizationapi.SubjectAccessReviewResponse{
			Allowed:   false,
			Reason:    `User "valerie" cannot create policybindings in project "openshift"`,
			Namespace: "openshift",
		},
	}.run(t)

	askCanClusterAdminsCreateProject := &authorizationapi.LocalSubjectAccessReview{
		Groups: sets.NewString("system:cluster-admins"),
		Action: authorizationapi.Action{Verb: "create", Resource: "projects"},
	}
	subjectAccessReviewTest{
		description:       "I shouldn't be allowed to ask whether someone else can perform an action",
		localInterface:    valerieOpenshiftClient.LocalSubjectAccessReviews("openshift"),
		localReview:       askCanClusterAdminsCreateProject,
		kubeAuthInterface: valerieKubeClient.Authorization(),
		kubeNamespace:     "openshift",
		err:               `User "valerie" cannot create localsubjectaccessreviews in project "openshift"`,
		kubeErr:           `User "valerie" cannot create localsubjectaccessreviews.authorization.k8s.io in project "openshift"`,
	}.run(t)

}

func TestSelfSubjectAccessReviewsNonExistingNamespace(t *testing.T) {
	masterConfig, clusterAdminKubeConfig, err := testserver.StartTestMasterAPI()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer testserver.CleanupMasterEtcd(t, masterConfig)

	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	valerieOpenshiftClient, valerieKubeClient, _, err := testutil.GetClientForUser(*clusterAdminClientConfig, "valerie")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// ensure that a SAR for a non-exisitng namespace gives a SAR response and not a
	// namespace doesn't exist response from admisison.
	askCanICreatePodsInNonExistingNamespace := &authorizationapi.LocalSubjectAccessReview{
		Action: authorizationapi.Action{Namespace: "foo", Verb: "create", Resource: "pods"},
	}
	subjectAccessReviewTest{
		description:       "ensure SAR for non-existing namespace does not leak namespace info",
		localInterface:    valerieOpenshiftClient.LocalSubjectAccessReviews("foo"),
		localReview:       askCanICreatePodsInNonExistingNamespace,
		kubeAuthInterface: valerieKubeClient.Authorization(),
		response: authorizationapi.SubjectAccessReviewResponse{
			Allowed:   false,
			Reason:    `User "valerie" cannot create pods in project "foo"`,
			Namespace: "foo",
		},
	}.run(t)
}
