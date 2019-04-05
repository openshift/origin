package integration

import (
	"testing"

	"k8s.io/apimachinery/pkg/util/sets"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	authorizationclient "github.com/openshift/origin/pkg/authorization/generated/internalclientset"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

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

	valerieKubeClient, valerieClientConfig, err := testutil.GetClientForUser(clusterAdminClientConfig, "valerie")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	valerieAuthorizationClient := authorizationclient.NewForConfigOrDie(valerieClientConfig).Authorization()

	askCanICreatePolicyBindings := &authorizationapi.LocalSubjectAccessReview{
		Action: authorizationapi.Action{Verb: "create", Resource: "policybindings"},
	}
	subjectAccessReviewTest{
		description:       "can I get a subjectaccessreview on myself even if I have no rights to do it generally",
		localInterface:    valerieAuthorizationClient.LocalSubjectAccessReviews("openshift"),
		localReview:       askCanICreatePolicyBindings,
		kubeAuthInterface: valerieKubeClient.AuthorizationV1(),
		response: authorizationapi.SubjectAccessReviewResponse{
			Allowed:   false,
			Reason:    ``,
			Namespace: "openshift",
		},
	}.run(t)

	askCanClusterAdminsCreateProject := &authorizationapi.LocalSubjectAccessReview{
		Groups: sets.NewString("system:cluster-admins"),
		Action: authorizationapi.Action{Verb: "create", Resource: "projects"},
	}
	subjectAccessReviewTest{
		description:       "I shouldn't be allowed to ask whether someone else can perform an action",
		localInterface:    valerieAuthorizationClient.LocalSubjectAccessReviews("openshift"),
		localReview:       askCanClusterAdminsCreateProject,
		kubeAuthInterface: valerieKubeClient.AuthorizationV1(),
		kubeNamespace:     "openshift",
		err:               `localsubjectaccessreviews.authorization.openshift.io is forbidden: User "valerie" cannot create resource "localsubjectaccessreviews" in API group "authorization.openshift.io" in the namespace "openshift"`,
		kubeErr:           `localsubjectaccessreviews.authorization.k8s.io is forbidden: User "valerie" cannot create resource "localsubjectaccessreviews" in API group "authorization.k8s.io" in the namespace "openshift"`,
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

	valerieKubeClient, valerieClientConfig, err := testutil.GetClientForUser(clusterAdminClientConfig, "valerie")
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
		localInterface:    authorizationclient.NewForConfigOrDie(valerieClientConfig).Authorization().LocalSubjectAccessReviews("foo"),
		localReview:       askCanICreatePodsInNonExistingNamespace,
		kubeAuthInterface: valerieKubeClient.AuthorizationV1(),
		response: authorizationapi.SubjectAccessReviewResponse{
			Allowed:   false,
			Reason:    ``,
			Namespace: "foo",
		},
	}.run(t)
}
