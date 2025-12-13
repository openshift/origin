package authorization

import (
	"fmt"

	"k8s.io/client-go/kubernetes"

	g "github.com/onsi/ginkgo/v2"
	authorizationv1 "github.com/openshift/api/authorization/v1"
	authorizationv1typedclient "github.com/openshift/client-go/authorization/clientset/versioned/typed/authorization/v1"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-auth][Feature:OpenShiftAuthorization] self-SAR compatibility", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLI("bootstrap-policy")

	g.Context("", func() {
		g.Describe("TestBootstrapPolicySelfSubjectAccessReviews", func() {
			g.It(fmt.Sprintf("should succeed [apigroup:user.openshift.io][apigroup:authorization.openshift.io]"), g.Label("Size:S"), func() {
				t := g.GinkgoT()

				valerieName := oc.CreateUser("valerie-").Name
				valerieClientConfig := oc.GetClientConfigForUser(valerieName)

				askCanICreatePolicyBindings := &authorizationv1.LocalSubjectAccessReview{
					Action: authorizationv1.Action{Verb: "create", Resource: "policybindings"},
				}
				subjectAccessReviewTest{
					description:       "can I get a subjectaccessreview on myself even if I have no rights to do it generally",
					localInterface:    authorizationv1typedclient.NewForConfigOrDie(valerieClientConfig).LocalSubjectAccessReviews("openshift"),
					localReview:       askCanICreatePolicyBindings,
					kubeAuthInterface: kubernetes.NewForConfigOrDie(valerieClientConfig).AuthorizationV1(),
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
					localInterface:    authorizationv1typedclient.NewForConfigOrDie(valerieClientConfig).LocalSubjectAccessReviews("openshift"),
					localReview:       askCanClusterAdminsCreateProject,
					kubeAuthInterface: kubernetes.NewForConfigOrDie(valerieClientConfig).AuthorizationV1(),
					kubeNamespace:     "openshift",
					err:               `localsubjectaccessreviews.authorization.openshift.io is forbidden: User "` + valerieName + `" cannot create resource "localsubjectaccessreviews" in API group "authorization.openshift.io" in the namespace "openshift"`,
					kubeErr:           `localsubjectaccessreviews.authorization.k8s.io is forbidden: User "` + valerieName + `" cannot create resource "localsubjectaccessreviews" in API group "authorization.k8s.io" in the namespace "openshift"`,
				}.run(t)

			})
		})

		g.Describe("TestSelfSubjectAccessReviewsNonExistingNamespace", func() {
			g.It(fmt.Sprintf("should succeed [apigroup:user.openshift.io][apigroup:authorization.openshift.io]"), g.Label("Size:S"), func() {
				t := g.GinkgoT()

				valerieName := oc.CreateUser("valerie-").Name
				valerieClientConfig := oc.GetClientConfigForUser(valerieName)

				// ensure that a SAR for a non-exisitng namespace gives a SAR response and not a
				// namespace doesn't exist response from admisison.
				askCanICreatePodsInNonExistingNamespace := &authorizationv1.LocalSubjectAccessReview{
					Action: authorizationv1.Action{Namespace: "foo", Verb: "create", Resource: "pods"},
				}
				subjectAccessReviewTest{
					description:       "ensure SAR for non-existing namespace does not leak namespace info",
					localInterface:    authorizationv1typedclient.NewForConfigOrDie(valerieClientConfig).LocalSubjectAccessReviews("foo"),
					localReview:       askCanICreatePodsInNonExistingNamespace,
					kubeAuthInterface: kubernetes.NewForConfigOrDie(valerieClientConfig).AuthorizationV1(),
					response: authorizationv1.SubjectAccessReviewResponse{
						Allowed:   false,
						Reason:    ``,
						Namespace: "foo",
					},
				}.run(t)
			})
		})
	})
})
