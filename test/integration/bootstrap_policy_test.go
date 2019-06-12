package integration

import (
	"testing"

	authorizationv1 "github.com/openshift/api/authorization/v1"
	authorizationv1typedclient "github.com/openshift/client-go/authorization/clientset/versioned/typed/authorization/v1"
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
	valerieAuthorizationClient := authorizationv1typedclient.NewForConfigOrDie(valerieClientConfig)

	askCanICreatePolicyBindings := &authorizationv1.LocalSubjectAccessReview{
		Action: authorizationv1.Action{Verb: "create", Resource: "policybindings"},
	}
	subjectAccessReviewTest{
		description:       "can I get a subjectaccessreview on myself even if I have no rights to do it generally",
		localInterface:    valerieAuthorizationClient.LocalSubjectAccessReviews("openshift"),
		localReview:       askCanICreatePolicyBindings,
		kubeAuthInterface: valerieKubeClient.AuthorizationV1(),
		response: authorizationv1.SubjectAccessReviewResponse{
			Allowed:   false,
			Reason:    ``,
			Namespace: "openshift",
		},
	}.run(t)

	askCanClusterAdminsCreateProject := &authorizationv1.LocalSubjectAccessReview{
		GroupsSlice: []string{"system:cluster-admins"},
		Action:      authorizationv1.Action{Verb: "create", Resource: "projects"},
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
	askCanICreatePodsInNonExistingNamespace := &authorizationv1.LocalSubjectAccessReview{
		Action: authorizationv1.Action{Namespace: "foo", Verb: "create", Resource: "pods"},
	}
	subjectAccessReviewTest{
		description:       "ensure SAR for non-existing namespace does not leak namespace info",
		localInterface:    authorizationv1typedclient.NewForConfigOrDie(valerieClientConfig).LocalSubjectAccessReviews("foo"),
		localReview:       askCanICreatePodsInNonExistingNamespace,
		kubeAuthInterface: valerieKubeClient.AuthorizationV1(),
		response: authorizationv1.SubjectAccessReviewResponse{
			Allowed:   false,
			Reason:    ``,
			Namespace: "foo",
		},
	}.run(t)
}
