package authentication

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/operator"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/pod-security-admission/api"
)

// This test validates the behavior introduced by CNTRLPLANE-2991:
// When ExternalOIDCExternalClaimsSourcing feature gate is enabled and authentication
// type is OIDC, the kube-apiserver should NOT be configured with structured authentication
// (no auth-config ConfigMap, no --authentication-config flag). The oauth-metadata should
// be removed and the operator should remain healthy.
var _ = g.Describe("[sig-auth][Suite:openshift/auth/external-oidc][Serial][Slow][Disruptive]", g.Ordered, func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLIWithoutNamespace("oidc-claims-sourcing-e2e")
	oc.KubeFramework().NamespacePodSecurityLevel = api.LevelPrivileged
	oc.SetNamespace("oidc-claims-sourcing-e2e")
	ctx := context.TODO()

	var originalAuth *configv1.Authentication
	var keycloakNamespace string
	var cleanups []removalFunc
	var oidcClientSecret string

	g.BeforeAll(func() {
		var err error

		err = operator.WaitForOperatorsToSettle(ctx, oc.AdminConfigClient(), 30)
		o.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error waiting for the cluster operators to settle before starting test")

		testID := rand.String(8)
		keycloakNamespace = fmt.Sprintf("oidc-claims-sourcing-%s", testID)

		cleanups, err = deployKeycloak(ctx, oc, keycloakNamespace, g.GinkgoLogr)
		o.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error deploying keycloak")

		kcURL, err := admittedURLForRoute(ctx, oc, keycloakResourceName, keycloakNamespace)
		o.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error getting keycloak route URL")

		keycloakCli, err := keycloakClientFor(kcURL)
		o.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error creating a keycloak client")

		err = keycloakCli.Authenticate("admin-cli", keycloakAdminUsername, keycloakAdminPassword)
		o.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error authenticating as keycloak admin")

		o.Expect(keycloakCli.ConfigureClient("admin-cli")).NotTo(o.HaveOccurred(), "should not encounter an error configuring the admin-cli client")

		originalAuth, err = oc.AdminConfigClient().ConfigV1().Authentications().Get(ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "should not error getting authentications")

		oidcClientSecret = fmt.Sprintf("openshift-console-oidc-client-secret-%s", testID)
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      oidcClientSecret,
				Namespace: "openshift-config",
			},
			Data: map[string][]byte{
				"clientSecret": []byte(`a-secret-value`),
			},
		}
		_, err = oc.AdminKubeClient().CoreV1().Secrets("openshift-config").Create(ctx, secret, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error creating oidc client secret")
		cleanups = append(cleanups, func(ctx context.Context) error {
			return oc.AdminKubeClient().CoreV1().Secrets("openshift-config").Delete(ctx, secret.Name, metav1.DeleteOptions{})
		})
	})

	g.Describe("[OCPFeatureGate:ExternalOIDCExternalClaimsSourcing]", g.Ordered, func() {
		g.BeforeAll(func() {
			_, _, err := configureOIDCAuthentication(ctx, oc, keycloakNamespace, oidcClientSecret, nil)
			o.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error configuring OIDC authentication")

			waitForRollout(ctx, oc)
		})

		g.It("should not configure structured authentication when OIDC auth type is set", func() {
			kas, err := oc.AdminOperatorClient().OperatorV1().KubeAPIServers().Get(ctx, "cluster", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error getting the kubeapiservers.operator.openshift.io/cluster")

			observedConfig := map[string]interface{}{}
			err = json.Unmarshal(kas.Spec.ObservedConfig.Raw, &observedConfig)
			o.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error unmarshalling the KAS observed configuration")

			o.Expect(observedConfig["authConfig"]).To(o.BeNil(),
				"authConfig should not be specified when ExternalOIDCExternalClaimsSourcing is enabled with OIDC authentication")

			apiServerArgs, ok := observedConfig["apiServerArguments"].(map[string]interface{})
			o.Expect(ok).To(o.BeTrue(), "apiServerArguments should be present in observed config")

			o.Expect(apiServerArgs["authentication-config"]).To(o.BeNil(),
				"authentication-config argument should NOT be specified when ExternalOIDCExternalClaimsSourcing is enabled")
		})

		g.It("should not create the auth-config ConfigMap in openshift-kube-apiserver", func() {
			_, err := oc.AdminKubeClient().CoreV1().ConfigMaps("openshift-kube-apiserver").Get(ctx, "auth-config", metav1.GetOptions{})
			o.Expect(err).To(o.HaveOccurred(), "auth-config ConfigMap should not exist when ExternalOIDCExternalClaimsSourcing is enabled")
			o.Expect(apierrors.IsNotFound(err)).To(o.BeTrue(),
				"expected NotFound error for auth-config ConfigMap, got: %v", err)
		})

		g.It("should remove the oauth-metadata ConfigMap from openshift-kube-apiserver", func() {
			o.Eventually(func(gomega o.Gomega) {
				_, err := oc.AdminKubeClient().CoreV1().ConfigMaps("openshift-kube-apiserver").Get(ctx, "oauth-metadata", metav1.GetOptions{})
				gomega.Expect(err).To(o.HaveOccurred(), "oauth-metadata ConfigMap should not exist")
				gomega.Expect(apierrors.IsNotFound(err)).To(o.BeTrue(),
					"expected NotFound error for oauth-metadata ConfigMap")
			}).WithTimeout(5 * time.Minute).WithPolling(10 * time.Second).Should(o.Succeed(),
				"oauth-metadata should be removed when ExternalOIDCExternalClaimsSourcing is enabled with OIDC")
		})

		g.It("should keep the kube-apiserver operator healthy", func() {
			kas, err := oc.AdminOperatorClient().OperatorV1().KubeAPIServers().Get(ctx, "cluster", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error getting the kubeapiservers.operator.openshift.io/cluster")

			for _, cond := range kas.Status.Conditions {
				switch cond.Type {
				case "Available":
					o.Expect(cond.Status).To(o.Equal(operatorv1.ConditionTrue),
						"kube-apiserver operator should be Available")
				case "Degraded":
					o.Expect(cond.Status).To(o.Equal(operatorv1.ConditionFalse),
						"kube-apiserver operator should not be Degraded")
				}
			}
		})

		g.It("should remove the OpenShift OAuth stack", func() {
			o.Eventually(func(gomega o.Gomega) {
				_, err := oc.AdminKubeClient().AppsV1().Deployments("openshift-authentication").Get(ctx, "oauth-openshift", metav1.GetOptions{})
				gomega.Expect(err).NotTo(o.BeNil(), "should not be able to get the integrated oauth stack")
				gomega.Expect(apierrors.IsNotFound(err)).To(o.BeTrue(),
					"integrated oauth stack should not be present when OIDC authentication is configured")
			}).WithTimeout(5 * time.Minute).WithPolling(10 * time.Second).Should(o.Succeed())
		})
	})

	g.AfterAll(func() {
		err, modified := resetAuthentication(ctx, oc, originalAuth)
		o.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error reverting authentication to original state")

		if modified {
			waitForRollout(ctx, oc)
		}

		err = removeResources(ctx, cleanups...)
		o.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error cleaning up keycloak resources")
	})
})
