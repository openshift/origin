package oauth

import (
	"context"
	"fmt"
	"io"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/library-go/pkg/oauth/tokenrequest"
	"github.com/openshift/library-go/pkg/oauth/tokenrequest/challengehandlers"
	exauth "github.com/openshift/origin/test/extended/authentication"
	exutil "github.com/openshift/origin/test/extended/util"
	exoperator "github.com/openshift/origin/test/extended/util/operator"
	authnv1 "k8s.io/api/authentication/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/util/rand"
)

var _ = g.Describe("[sig-auth][Feature:AuthenticationComponentProxy] OIDC login component proxy", g.Ordered, func() {
	var cleanups []exauth.RemovalFunc
	var authConfig *operatorv1.Authentication
	var original *operatorv1.Authentication

	var keycloakCli *exauth.KeycloakClient
	var username string
	var password string
	var group string

	squidServiceName := "squid-proxy"

	networkPolicyCopyList := map[string]networkingv1.NetworkPolicy{}

	keycloakNS := "keycloakNamespace"
	// This value will need to be saved somewhere to be accessed.
	proxyNS := "e2e-proxy-abc123"

	networkPolicyMap := []NetworkPolicyMap{
		{"proxy-e2e-deny-direct-openshift-authentication", "openshift-authentication"},
		{"proxy-e2e-deny-direct-openshift-authentication-operator", "openshift-authentication-operator"},
		{"proxy-e2e-allow-only-from-proxy", keycloakNS},
	}

	oc := exutil.NewCLIWithoutNamespace(keycloakNS)
	ctx := context.TODO()

	g.BeforeAll(func() {
		featureGates, err := oc.AdminConfigClient().ConfigV1().FeatureGates().Get(ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		if featureGates.Spec.FeatureSet != configv1.TechPreviewNoUpgrade {
			g.Skip(fmt.Sprintf("tests should only run if behind %s featureset", configv1.TechPreviewNoUpgrade))
		}

		authConfig, err = oc.AdminOperatorClient().OperatorV1().Authentications().Get(ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "should be able to get authentications.operator.openshift.io/cluster")

		original = authConfig.DeepCopy()

		for _, np := range networkPolicyMap {
			networkPolicy, err := oc.AdminKubeClient().NetworkingV1().NetworkPolicies(np.namespace).Get(ctx, np.name, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred(), "should be able to get NetworkPolicy")

			networkPolicyCopyList[np.name] = *networkPolicy.DeepCopy()
		}

		// We assume keycloak has been deployed and tests done previously that it works fine.
		kcUrl, err := exauth.AdmittedURLForRoute(ctx, oc, exauth.KeycloakResourceName, keycloakNS)
		o.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error getting keycloak route URL")

		keycloakCli, err = exauth.KeycloakClientFor(kcUrl)
		o.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error creating a keycloak client")

		// First authenticate as the admin keycloak user so we can add new groups and users
		err = keycloakCli.Authenticate("admin-cli", exauth.KeycloakAdminUsername, exauth.KeycloakAdminPassword)
		o.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error authenticating as keycloak admin")

		o.Expect(keycloakCli.ConfigureClient("admin-cli")).NotTo(o.HaveOccurred(), "should not encounter an error configuring the admin-cli client")

		testID := rand.String(8)

		username = fmt.Sprintf("user-%s", testID)
		password = fmt.Sprintf("password-%s", testID)
		group = fmt.Sprintf("ocp-test-%s-group", testID)

		o.Expect(keycloakCli.CreateGroup(group)).To(o.Succeed(), "should be able to create a new keycloak group")
		o.Expect(keycloakCli.CreateUser(username, password, group)).To(o.Succeed(), "should be able to create a new keycloak user")
	})

	g.AfterEach(func() {
		current, err := oc.AdminOperatorClient().OperatorV1().Authentications().Get(ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		current.Spec.Proxy = original.Spec.Proxy
		_, err = oc.AdminOperatorClient().OperatorV1().Authentications().Update(ctx, current, metav1.UpdateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		o.Eventually(func(gomega o.Gomega) {
			authn, err := oc.AdminOperatorClient().OperatorV1().Authentications().Get(ctx, "cluster", metav1.GetOptions{})
			gomega.Expect(err).NotTo(o.HaveOccurred())

			progressing := false
			for _, cond := range authn.Status.Conditions {
				if cond.Type == "Progressing" && cond.Status == operatorv1.ConditionFalse {
					progressing = true
					break
				}
			}
			gomega.Expect(progressing).To(o.BeTrue(), "authentication operator should finish progressing")

			available := false
			for _, cond := range authn.Status.Conditions {
				if cond.Type == "Available" && cond.Status == operatorv1.ConditionTrue {
					available = true
					break
				}
			}
			gomega.Expect(available).To(o.BeTrue(), "authentication operator should be available")
		}).WithTimeout(5*time.Minute).WithPolling(10*time.Second).Should(o.Succeed(), "authentication operator should reconcile without proxy config")

		err = exoperator.WaitForOperatorsToSettle(ctx, oc.AdminConfigClient(), 10)
		o.Expect(err).NotTo(o.HaveOccurred(), "cluster operators should settle after removing proxy config")

		for name, saved := range networkPolicyCopyList {
			restored := saved.DeepCopy()
			existing, err := oc.AdminKubeClient().NetworkingV1().NetworkPolicies(saved.Namespace).Get(ctx, name, metav1.GetOptions{})
			if apierrors.IsNotFound(err) {
				restored.ResourceVersion = ""
				_, err = oc.AdminKubeClient().NetworkingV1().NetworkPolicies(saved.Namespace).Create(ctx, restored, metav1.CreateOptions{})
				o.Expect(err).NotTo(o.HaveOccurred(), "network policy should be created")
				continue
			}
			o.Expect(err).NotTo(o.HaveOccurred(), "should be able to get network policy")

			restored.ResourceVersion = existing.ResourceVersion
			_, err = oc.AdminKubeClient().NetworkingV1().NetworkPolicies(saved.Namespace).Update(ctx, restored, metav1.UpdateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred(), "network policy should be updated")
		}

		err = exauth.RemoveResources(ctx, cleanups...)
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	assertOIDCLogin := func(msg string) {
		g.GinkgoHelper()

		route, err := oc.AdminRouteClient().RouteV1().Routes("openshift-authentication").Get(ctx, "oauth-openshift", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "should be able to get the OAuth server route")
		oauthServerURL := fmt.Sprintf("https://%s", route.Spec.Host)

		o.Eventually(func(gomega o.Gomega) {
			tokenOpts := tokenrequest.NewRequestTokenOptions(oc.AdminConfig(), false)
			tokenOpts, err := tokenOpts.WithChallengeHandlers(
				challengehandlers.NewBasicChallengeHandler(oauthServerURL, "", nil, io.Discard, nil, username, password),
			)
			gomega.Expect(err).NotTo(o.HaveOccurred())

			err = tokenOpts.SetDefaultOsinConfig("openshift-challenging-client", nil)
			gomega.Expect(err).NotTo(o.HaveOccurred(), "should discover OAuth metadata")

			token, err := tokenOpts.RequestToken()
			gomega.Expect(err).NotTo(o.HaveOccurred(), msg)
			gomega.Expect(token).NotTo(o.BeEmpty())

			copiedOC := *oc
			tokenOC := copiedOC.WithToken(token)
			ssr, err := tokenOC.KubeClient().AuthenticationV1().SelfSubjectReviews().Create(ctx, &authnv1.SelfSubjectReview{}, metav1.CreateOptions{})
			gomega.Expect(err).NotTo(o.HaveOccurred(), "should be able to create a SelfSubjectReview")

			gomega.Expect(ssr.Status.UserInfo.Username).NotTo(o.BeEmpty())
			gomega.Expect(ssr.Status.UserInfo.Groups).To(o.ContainElement(group))
		}).WithTimeout(5 * time.Minute).WithPolling(10 * time.Second).Should(o.Succeed())
	}

	g.It("should apply proxy configuration to the authentication operator [apigroup:operator.openshift.io] and perform full oidc login flow", func() {
		modified := authConfig.DeepCopy()

		// actual proxy values will be set at a later stage
		modified.Spec.Proxy = operatorv1.AuthenticationProxyConfig{
			HTTPProxy:  "httpProxyURL",
			HTTPSProxy: "httpsProxyURL",
			NoProxy:    []string{"dontGoHere"},
		}

		_, err := oc.AdminOperatorClient().OperatorV1().Authentications().Update(ctx, modified, metav1.UpdateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "should be able to update the authentication operator proxy config")

		o.Eventually(func(gomega o.Gomega) {
			authn, err := oc.AdminOperatorClient().OperatorV1().Authentications().Get(ctx, "cluster", metav1.GetOptions{})
			gomega.Expect(err).NotTo(o.HaveOccurred())

			progressing := false
			for _, cond := range authn.Status.Conditions {
				if cond.Type == "Progressing" && cond.Status == operatorv1.ConditionFalse {
					progressing = true
					break
				}
			}
			gomega.Expect(progressing).To(o.BeTrue(), "authentication operator should finish progressing")

			available := false
			for _, cond := range authn.Status.Conditions {
				if cond.Type == "Available" && cond.Status == operatorv1.ConditionTrue {
					available = true
					break
				}
			}
			gomega.Expect(available).To(o.BeTrue(), "authentication operator should be available")
		}).WithTimeout(5*time.Minute).WithPolling(10*time.Second).Should(o.Succeed(), "authentication operator should reconcile proxy config")

		err = exoperator.WaitForOperatorsToSettle(ctx, oc.AdminConfigClient(), 10)
		o.Expect(err).NotTo(o.HaveOccurred(), "cluster operators should settle after proxy config change")

		assertOIDCLogin("should get an OpenShift token via OAuth flow through the component proxy")
	})

	g.It("should fall back to direct IdP connectivity for oidc login", func() {
		// spec.proxy has been removed in g.AfterEach(), need to remove NetworkPolicies
		// to test direct IdP connectivity

		for name, np := range networkPolicyCopyList {
			err := oc.AdminKubeClient().NetworkingV1().NetworkPolicies(np.Namespace).Delete(ctx, name, metav1.DeleteOptions{})
			o.Expect(err).NotTo(o.HaveOccurred(), "should be able to delete NetworkPolicy")
			_, err = oc.AdminKubeClient().NetworkingV1().NetworkPolicies(np.Namespace).Get(ctx, name, metav1.GetOptions{})
			o.Expect(apierrors.IsNotFound(err)).To(o.BeTrue())
		}

		// add a network policy to deny all egress from squid
		denyAllSquid := &networkingv1.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "block-squid-to-keycloak",
				Namespace: proxyNS,
			},
			Spec: networkingv1.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{"app": squidServiceName},
				},
				PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
				Egress:      []networkingv1.NetworkPolicyEgressRule{},
			},
		}

		// applying this policy ensures proxy cannot be used during oauth flow - meaning
		// only direct IdP would have taken place.
		_, err := oc.AdminKubeClient().NetworkingV1().NetworkPolicies(proxyNS).Create(ctx, denyAllSquid, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "network policy should create successfully")

		g.DeferCleanup(func() {
			err := oc.AdminKubeClient().NetworkingV1().NetworkPolicies(proxyNS).Delete(ctx, denyAllSquid.Name, metav1.DeleteOptions{})
			if err != nil && !apierrors.IsNotFound(err) {
				g.GinkgoWriter.Printf("failed to clean up NetworkPolicy %s: %v\n", denyAllSquid.Name, err)
			}
		})

		assertOIDCLogin("should get an OpenShift token via direct IdP connectivity")
	})
})

type NetworkPolicyMap struct {
	name      string
	namespace string
}

type NetworkPolicyCopy struct {
	name          string
	networkPolicy *networkingv1.NetworkPolicy
}
