package authentication

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/api/features"
	operatorv1 "github.com/openshift/api/operator/v1"
	routev1 "github.com/openshift/api/route/v1"
	operatorv1client "github.com/openshift/client-go/operator/clientset/versioned/typed/operator/v1"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/operator"
	authnv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/util/errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/retry"
	"k8s.io/pod-security-admission/api"

	"github.com/openshift/library-go/pkg/operator/condition"
)

type kubeObject interface {
	runtime.Object
	metav1.Object
}

// Just note for readers: currently g.Ordered and g.BeforeAll don't take effect in openshift-tests. The tests are not run as the order they appear and the BeforeAll (with slow rollout) is run each test instead of only once
// We've looked into it and have some ideas on a solution but haven't finished (TODO)
// BTW this won't a problem in future once openshift/enhancements#1907 is done because this won't be disruptive nor need to wait for any rollout
var _ = g.Describe("[sig-auth][Suite:openshift/auth/external-oidc][Serial][Slow][Disruptive]", g.Ordered, func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLIWithoutNamespace("oidc-e2e")
	oc.KubeFramework().NamespacePodSecurityLevel = api.LevelPrivileged
	oc.SetNamespace("oidc-e2e")
	ctx := context.TODO()

	var cleanups []removalFunc
	var keycloakCli *keycloakClient
	var username string
	var password string
	var group string
	var originalAuth *configv1.Authentication
	var oauthUserConfig *rest.Config
	var oidcClientSecret string

	var keycloakNamespace string

	g.BeforeAll(func() {
		var err error

		// waitTime is in minutes - set to 30 minute wait for cluster operators to settle before starting tests.
		err = operator.WaitForOperatorsToSettle(ctx, oc.AdminConfigClient(), 30)
		o.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error waiting for the cluster operators to settle before starting test")

		testID := rand.String(8)
		keycloakNamespace = fmt.Sprintf("oidc-keycloak-%s", testID)

		cleanups, err = deployKeycloak(ctx, oc, keycloakNamespace, g.GinkgoLogr)
		o.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error deploying keycloak")

		kcURL, err := admittedURLForRoute(ctx, oc, keycloakResourceName, keycloakNamespace)
		o.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error getting keycloak route URL")

		keycloakCli, err = keycloakClientFor(kcURL)
		o.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error creating a keycloak client")

		// First authenticate as the admin keycloak user so we can add new groups and users
		err = keycloakCli.Authenticate("admin-cli", keycloakAdminUsername, keycloakAdminPassword)
		o.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error authenticating as keycloak admin")

		o.Expect(keycloakCli.ConfigureClient("admin-cli")).NotTo(o.HaveOccurred(), "should not encounter an error configuring the admin-cli client")

		username = fmt.Sprintf("user-%s", testID)
		password = fmt.Sprintf("password-%s", testID)
		group = fmt.Sprintf("ocp-test-%s-group", testID)

		o.Expect(keycloakCli.CreateGroup(group)).To(o.Succeed(), "should be able to create a new keycloak group")
		o.Expect(keycloakCli.CreateUser(username, password, group)).To(o.Succeed(), "should be able to create a new keycloak user")

		originalAuth, err = oc.AdminConfigClient().ConfigV1().Authentications().Get(ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "should not error getting authentications")

		oauthUserConfig = oc.GetClientConfigForUser("oidc-e2e-oauth-user")

		// create a dummy oidc client secret for the console to consume
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

	g.Describe("[OCPFeatureGate:ExternalOIDC]", g.Ordered, func() {
		g.BeforeAll(func() {
			_, _, err := configureOIDCAuthentication(ctx, oc, keycloakNamespace, oidcClientSecret, nil)
			o.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error configuring OIDC authentication")

			waitForRollout(ctx, oc)
			waitForHealthyOIDCClients(ctx, oc)
		})

		g.Describe("external IdP is configured", g.Ordered, func() {
			g.It("should configure kube-apiserver", func() {
				// Get the FeatureGates resource
				fgs, err := oc.AdminConfigClient().ConfigV1().FeatureGates().Get(context.TODO(), "cluster", metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred(), "error getting cluster FeatureGates.")

				// Loop through the feature gates to see if the desired one is enabled
				externalClaimsSourcingEnabled := false
				for _, fg := range fgs.Status.FeatureGates {
					for _, enabledFG := range fg.Enabled {
						if enabledFG.Name == features.FeatureGateExternalOIDCExternalClaimsSourcing {
							externalClaimsSourcingEnabled = true
							break
						}
					}
				}

				if externalClaimsSourcingEnabled {
					g.Skip("ExternalOIDCExternalClaimsSourcing is enabled which makes this test case obsolete, skipping.")
				}

				kas, err := oc.AdminOperatorClient().OperatorV1().KubeAPIServers().Get(ctx, "cluster", metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error getting the kubeapiservers.operator.openshift.io/cluster")

				observedConfig := map[string]interface{}{}
				err = json.Unmarshal(kas.Spec.ObservedConfig.Raw, &observedConfig)
				o.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error unmarshalling the KAS observed configuration")

				o.Expect(observedConfig["authConfig"]).To(o.BeNil(), "authConfig should not be specified when OIDC authentication is configured")

				apiServerArgs := observedConfig["apiServerArguments"].(map[string]interface{})

				o.Expect(apiServerArgs["authentication-token-webhook-config-file"]).To(o.BeNil(), "authentication-token-webhook-config-file argument should not be specified when OIDC authentication is configured")
				o.Expect(apiServerArgs["authentication-token-webhook-version"]).To(o.BeNil(), "authentication-token-webhook-version argument should not be specified when OIDC authentication is configured")

				o.Expect(apiServerArgs["authentication-config"]).NotTo(o.BeNil(), "authentication-config argument should be specified when OIDC authentication is configured")
				o.Expect(apiServerArgs["authentication-config"].([]interface{})[0].(string)).To(o.Equal("/etc/kubernetes/static-pod-resources/configmaps/auth-config/auth-config.json"))
			})

			g.It("should remove the OpenShift OAuth stack", func() {
				o.Eventually(func(gomega o.Gomega) {
					_, err := oc.AdminKubeClient().AppsV1().Deployments("openshift-authentication").Get(ctx, "oauth-openshift", metav1.GetOptions{})
					gomega.Expect(err).NotTo(o.BeNil(), "should not be able to get the integrated oauth stack")
					gomega.Expect(apierrors.IsNotFound(err)).To(o.BeTrue(), "integrated oauth stack should not be present when OIDC authentication is configured")
				}).WithTimeout(5 * time.Minute).WithPolling(10 * time.Second).Should(o.Succeed())
			})

			g.It("should not accept tokens provided by the OAuth server", func() {
				o.Eventually(func(gomega o.Gomega) {
					clientset, err := kubernetes.NewForConfig(oauthUserConfig)
					gomega.Expect(err).NotTo(o.HaveOccurred())

					_, err = clientset.AuthenticationV1().SelfSubjectReviews().Create(ctx, &authnv1.SelfSubjectReview{
						ObjectMeta: metav1.ObjectMeta{
							Name: fmt.Sprintf("%s-info", username),
						},
					}, metav1.CreateOptions{})
					gomega.Expect(err).ShouldNot(o.BeNil(), "should not be able to create SelfSubjectReview using OAuth client token")
					gomega.Expect(apierrors.IsUnauthorized(err)).To(o.BeTrue(), "should receive an unauthorized error when trying to create SelfSubjectReview using OAuth client token")
				}).WithTimeout(5 * time.Minute).WithPolling(10 * time.Second).Should(o.Succeed())
			})

			g.It("should accept authentication via a certificate-based kubeconfig (break-glass)", func() {
				_, err := oc.AdminKubeClient().CoreV1().Pods(oc.Namespace()).List(ctx, metav1.ListOptions{})
				o.Expect(err).NotTo(o.HaveOccurred(), "should be able to list pods using certificate-based authentication")
			})

			g.It("should map cluster identities correctly", func() {
				// should always be able to create an SSR for yourself
				o.Eventually(func(gomega o.Gomega) {
					err := keycloakCli.Authenticate("admin-cli", username, password)
					gomega.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error authenticating as keycloak user")

					copiedOC := *oc
					token := keycloakCli.AccessToken()
					tokenOC := copiedOC.WithToken(token)
					ssr, err := tokenOC.KubeClient().AuthenticationV1().SelfSubjectReviews().Create(ctx, &authnv1.SelfSubjectReview{
						ObjectMeta: metav1.ObjectMeta{
							Name: fmt.Sprintf("%s-info", username),
						},
					}, metav1.CreateOptions{})
					gomega.Expect(err).NotTo(o.HaveOccurred(), "should be able to create a SelfSubjectReview")

					gomega.Expect(ssr.Status.UserInfo.Username).To(o.Equal(fmt.Sprintf("%s@payload.openshift.io", username)))
					gomega.Expect(ssr.Status.UserInfo.Groups).To(o.ContainElement(group))
				}).WithTimeout(5 * time.Minute).WithPolling(10 * time.Second).Should(o.Succeed())
			})
		})

		g.Describe("reverting to IntegratedOAuth", g.Ordered, func() {
			g.BeforeAll(func() {
				// Wait until we can authenticate using the configured external IdP
				o.Eventually(func(gomega o.Gomega) {
					// always re-authenticate to get a new token
					err := keycloakCli.Authenticate("admin-cli", username, password)
					gomega.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error authenticating as keycloak user")

					copiedOC := *oc
					tokenOC := copiedOC.WithToken(keycloakCli.AccessToken())

					_, err = tokenOC.KubeClient().AuthenticationV1().SelfSubjectReviews().Create(ctx, &authnv1.SelfSubjectReview{
						ObjectMeta: metav1.ObjectMeta{
							Name: fmt.Sprintf("%s-info", username),
						},
					}, metav1.CreateOptions{})
					gomega.Expect(err).NotTo(o.HaveOccurred(), "should be able to create a SelfSubjectReview")
				}).WithTimeout(5 * time.Minute).WithPolling(10 * time.Second).Should(o.Succeed())

				err, modified := resetAuthentication(ctx, oc, originalAuth)
				o.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error reverting authentication to original state")

				if modified {
					waitForRollout(ctx, oc)
				}
			})

			g.It("should rollout configuration on the kube-apiserver successfully", func() {
				kas, err := oc.AdminOperatorClient().OperatorV1().KubeAPIServers().Get(ctx, "cluster", metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error getting the kubeapiservers.operator.openshift.io/cluster")

				observedConfig := map[string]interface{}{}
				err = json.Unmarshal(kas.Spec.ObservedConfig.Raw, &observedConfig)
				o.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error unmarshalling the KAS observed configuration")

				o.Expect(observedConfig["authConfig"]).ToNot(o.BeNil(), "authConfig should be specified when OIDC authentication is configured")

				apiServerArgs := observedConfig["apiServerArguments"].(map[string]interface{})

				o.Expect(apiServerArgs["authentication-token-webhook-config-file"]).NotTo(o.BeNil(), "authentication-token-webhook-config-file argument should be specified when OIDC authentication is not configured")
				o.Expect(apiServerArgs["authentication-token-webhook-version"]).NotTo(o.BeNil(), "authentication-token-webhook-version argument should be specified when OIDC authentication is not configured")

				o.Expect(apiServerArgs["authentication-config"]).To(o.BeNil(), "authentication-config argument should not be specified when OIDC authentication is not configured")
			})

			g.It("should rollout the OpenShift OAuth stack", func() {
				o.Eventually(func(gomega o.Gomega) {
					_, err := oc.AdminKubeClient().AppsV1().Deployments("openshift-authentication").Get(ctx, "oauth-openshift", metav1.GetOptions{})
					gomega.Expect(err).Should(o.BeNil(), "should be able to get the integrated oauth stack")
				}).WithTimeout(5 * time.Minute).WithPolling(10 * time.Second).Should(o.Succeed())
			})

			g.It("should not accept tokens provided by an external IdP", func() {
				o.Eventually(func(gomega o.Gomega) {
					// always re-authenticate to get a new token
					err := keycloakCli.Authenticate("admin-cli", username, password)
					gomega.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error authenticating as keycloak user")

					copiedOC := *oc
					tokenOC := copiedOC.WithToken(keycloakCli.AccessToken())

					_, err = tokenOC.KubeClient().AuthenticationV1().SelfSubjectReviews().Create(ctx, &authnv1.SelfSubjectReview{
						ObjectMeta: metav1.ObjectMeta{
							Name: fmt.Sprintf("%s-info", username),
						},
					}, metav1.CreateOptions{})
					gomega.Expect(err).To(o.HaveOccurred(), "should not be able to create a SelfSubjectReview")
					gomega.Expect(apierrors.IsUnauthorized(err)).To(o.BeTrue(), "external IdP token should be unauthorized")
				}).WithTimeout(5 * time.Minute).WithPolling(10 * time.Second).Should(o.Succeed())
			})

			g.It("should accept tokens provided by the OpenShift OAuth server", func() {
				o.Eventually(func(gomega o.Gomega) {
					clientset, err := kubernetes.NewForConfig(oauthUserConfig)
					gomega.Expect(err).NotTo(o.HaveOccurred())

					_, err = clientset.AuthenticationV1().SelfSubjectReviews().Create(ctx, &authnv1.SelfSubjectReview{
						ObjectMeta: metav1.ObjectMeta{
							Name: fmt.Sprintf("%s-info", username),
						},
					}, metav1.CreateOptions{})
					gomega.Expect(err).ShouldNot(o.HaveOccurred(), "should be able to create SelfSubjectReview using OAuth client token")
				}).WithTimeout(5 * time.Minute).WithPolling(10 * time.Second).Should(o.Succeed())
			})
		})
	})

	g.Describe("[OCPFeatureGate:ExternalOIDCWithUIDAndExtraClaimMappings]", g.Ordered, func() {
		g.Describe("external IdP is configured", func() {
			g.Describe("without specified UID or Extra claim mappings", func() {
				g.BeforeAll(func() {
					_, _, err := configureOIDCAuthentication(ctx, oc, keycloakNamespace, oidcClientSecret, nil)
					o.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error configuring OIDC authentication")

					waitForRollout(ctx, oc)
					waitForHealthyOIDCClients(ctx, oc)
				})

				g.It("should default UID to the 'sub' claim in the access token from the IdP", func() {
					// should always be able to create an SSR for yourself
					o.Eventually(func(gomega o.Gomega) {
						err := keycloakCli.Authenticate("admin-cli", username, password)
						gomega.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error authenticating as keycloak user")

						copiedOC := *oc
						tokenOC := copiedOC.WithToken(keycloakCli.AccessToken())
						ssr, err := tokenOC.KubeClient().AuthenticationV1().SelfSubjectReviews().Create(ctx, &authnv1.SelfSubjectReview{
							ObjectMeta: metav1.ObjectMeta{
								Name: fmt.Sprintf("%s-info", username),
							},
						}, metav1.CreateOptions{})
						gomega.Expect(err).NotTo(o.HaveOccurred(), "should be able to create a SelfSubjectReview")

						gomega.Expect(ssr.Status.UserInfo.UID).ToNot(o.BeEmpty())
					}).WithTimeout(5 * time.Minute).WithPolling(10 * time.Second).Should(o.Succeed())
				})
			})

			g.Describe("with valid specified UID or Extra claim mappings", func() {
				g.BeforeAll(func() {
					_, _, err := configureOIDCAuthentication(ctx, oc, keycloakNamespace, oidcClientSecret, func(o *configv1.OIDCProvider) {
						o.ClaimMappings.UID = &configv1.TokenClaimOrExpressionMapping{
							Expression: "claims.preferred_username.upperAscii()",
						}

						o.ClaimMappings.Extra = []configv1.ExtraMapping{
							{
								Key:             "payload/test",
								ValueExpression: "claims.email + 'extra'",
							},
						}
					})
					o.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error configuring OIDC authentication")

					waitForRollout(ctx, oc)
					waitForHealthyOIDCClients(ctx, oc)
				})

				g.Describe("checking cluster identity mapping", g.Ordered, func() {
					ssr := &authnv1.SelfSubjectReview{}
					g.BeforeAll(func() {
						o.Eventually(func(gomega o.Gomega) {
							err := keycloakCli.Authenticate("admin-cli", username, password)
							gomega.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error authenticating as keycloak user")

							copiedOC := *oc
							tokenOC := copiedOC.WithToken(keycloakCli.AccessToken())
							ssr, err = tokenOC.KubeClient().AuthenticationV1().SelfSubjectReviews().Create(ctx, &authnv1.SelfSubjectReview{
								ObjectMeta: metav1.ObjectMeta{
									Name: fmt.Sprintf("%s-info", username),
								},
							}, metav1.CreateOptions{})
							gomega.Expect(err).NotTo(o.HaveOccurred(), "should be able to create a SelfSubjectReview")
						}).WithTimeout(5 * time.Minute).WithPolling(10 * time.Second).Should(o.Succeed())
					})

					g.It("should map UID correctly", func() {
						o.Expect(ssr.UID).NotTo(o.Equal(strings.ToUpper(username)))
					})

					g.It("should map Extra correctly", func() {
						o.Expect(ssr.Status.UserInfo.Extra).To(o.HaveKey("payload/test"))
						o.Expect(ssr.Status.UserInfo.Extra["payload/test"]).To(o.HaveLen(1))
						o.Expect(ssr.Status.UserInfo.Extra["payload/test"][0]).To(o.Equal(fmt.Sprintf("%s@payload.openshift.ioextra", username)))
					})
				})
			})

			g.Describe("with invalid specified UID or Extra claim mappings", func() {
				g.It("should reject admission when UID claim expression is not compilable CEL", func() {
					_, _, err := configureOIDCAuthentication(ctx, oc, keycloakNamespace, oidcClientSecret, func(o *configv1.OIDCProvider) {
						o.ClaimMappings.UID = &configv1.TokenClaimOrExpressionMapping{
							Expression: "!@&*#^",
						}
					})
					o.Expect(err).To(o.HaveOccurred(), "should encounter an error configuring OIDC authentication")
				})

				g.It("should reject admission when Extra claim expression is not compilable CEL", func() {
					_, _, err := configureOIDCAuthentication(ctx, oc, keycloakNamespace, oidcClientSecret, func(o *configv1.OIDCProvider) {
						o.ClaimMappings.Extra = []configv1.ExtraMapping{
							{
								Key:             "payload/test",
								ValueExpression: "!@*&#^!@(*&^",
							},
						}
					})
					o.Expect(err).To(o.HaveOccurred(), "should encounter an error configuring OIDC authentication")
				})
			})
		})
	})

	g.Describe("[OCPFeatureGate:ExternalOIDCWithUpstreamParity]", g.Ordered, func() {
		var validUser, validUserPassword string
		var invalidUserValidation, invalidUserValidationPassword string
		var invalidClaimValidation, invalidClaimValidationPassword string

		g.Describe("with claim-based mappings, discoveryURL, userValidationRules, and CEL claimValidationRules", g.Ordered, func() {
			g.BeforeAll(func() {
				testID := rand.String(8)

				// Create multiple test users in Keycloak
				// Note: CreateUser automatically appends @payload.openshift.io to the username for the email
				validUser = fmt.Sprintf("user-valid-%s", testID)
				validUserPassword = fmt.Sprintf("password-valid-%s", testID)
				o.Expect(keycloakCli.CreateGroup(group)).To(o.Succeed(), "should be able to create/reuse keycloak group")
				o.Expect(keycloakCli.CreateUser(validUser, validUserPassword, group)).To(o.Succeed(), "should be able to create validUser")

				invalidUserValidation = fmt.Sprintf("noemail-%s", testID)
				invalidUserValidationPassword = fmt.Sprintf("password-invalid-user-%s", testID)
				otherGroup := "other-group"
				o.Expect(keycloakCli.CreateGroup(otherGroup)).To(o.Succeed(), "should be able to create other-group")
				o.Expect(keycloakCli.CreateUser(invalidUserValidation, invalidUserValidationPassword, otherGroup)).To(o.Succeed(), "should be able to create invalidUserValidation")

				invalidClaimValidation = fmt.Sprintf("user-invalid-%s", testID)
				invalidClaimValidationPassword = fmt.Sprintf("password-invalid-claim-%s", testID)
				// For this user, we need the email to end with @example.com to fail the CEL claim validation
				invalidClaimValidationEmail := fmt.Sprintf("%s@example.com", invalidClaimValidation)
				o.Expect(keycloakCli.CreateUserWithEmail(invalidClaimValidation, invalidClaimValidationEmail, invalidClaimValidationPassword, group)).To(o.Succeed(), "should be able to create invalidClaimValidation")

				// Configure OIDC provider with all new features
				_, _, err := configureOIDCAuthentication(ctx, oc, keycloakNamespace, oidcClientSecret, func(provider *configv1.OIDCProvider) {
					idpUrl, err := admittedURLForRoute(ctx, oc, keycloakResourceName, keycloakNamespace)
					o.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error getting keycloak route URL")

					// Set custom discoveryURL (different from issuerURL)
					provider.Issuer.DiscoveryURL = fmt.Sprintf("%s/realms/master/.well-known/openid-configuration", idpUrl)

					// Use claim-based mappings (explicit, not expression-based)
					provider.ClaimMappings.Username = configv1.UsernameClaimMapping{
						Claim: "email",
					}
					provider.ClaimMappings.Groups = configv1.PrefixedClaimMapping{
						TokenClaimMapping: configv1.TokenClaimMapping{
							Claim: "groups",
						},
					}

					// Set multiple UserValidationRules
					provider.UserValidationRules = []configv1.TokenUserValidationRule{
						{
							Expression: "user.username.contains('@')",
							Message:    "username must contain @ symbol",
						},
						{
							Expression: "user.groups.exists(g, g.startsWith('ocp-test-'))",
							Message:    "user must belong to ocp-test-* group",
						},
						{
							Expression: "user.username.size() > 5",
							Message:    "username must be longer than 5 characters",
						},
					}

					// Set ClaimValidationRules with CEL type
					provider.ClaimValidationRules = []configv1.TokenClaimValidationRule{
						{
							Type: configv1.TokenValidationRuleTypeCEL,
							CEL: configv1.TokenClaimValidationCELRule{
								Expression: "has(claims.email) && claims.email.endsWith('@payload.openshift.io')",
								Message:    "token must have email claim ending with @payload.openshift.io",
							},
						},
					}
				})
				o.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error configuring OIDC authentication")

				waitForRollout(ctx, oc)
				waitForHealthyOIDCClients(ctx, oc)
			})

			g.It("should authenticate successfully with custom discoveryURL, AND-logic userValidationRules, and CEL-type claimValidationRules", func() {
				o.Eventually(func(gomega o.Gomega) {
					err := keycloakCli.Authenticate("admin-cli", validUser, validUserPassword)
					gomega.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error authenticating as validUser")

					copiedOC := *oc
					tokenOC := copiedOC.WithToken(keycloakCli.AccessToken())
					ssr, err := tokenOC.KubeClient().AuthenticationV1().SelfSubjectReviews().Create(ctx, &authnv1.SelfSubjectReview{
						ObjectMeta: metav1.ObjectMeta{
							Name: fmt.Sprintf("%s-info", validUser),
						},
					}, metav1.CreateOptions{})
					gomega.Expect(err).NotTo(o.HaveOccurred(), "should be able to create a SelfSubjectReview")

					// Successful authentication implies custom discoveryURL worked

					// Verify userValidationRules (all 3 rules with AND logic passed)
					// Rule 1: user.username.contains('@')
					gomega.Expect(ssr.Status.UserInfo.Username).To(o.ContainSubstring("@"), "username should contain @ symbol")
					// Rule 2: user.groups.exists(g, g.startsWith('ocp-test-'))
					gomega.Expect(ssr.Status.UserInfo.Groups).To(o.ContainElement(group), "user should belong to ocp-test-* group")
					// Rule 3: user.username.size() > 5
					gomega.Expect(len(ssr.Status.UserInfo.Username)).To(o.BeNumerically(">", 5), "username should be longer than 5 characters")

					// Successful authentication implies all claimValidationRules passed (both RequiredClaim and CEL types)
				}).WithTimeout(5 * time.Minute).WithPolling(10 * time.Second).Should(o.Succeed())
			})

			g.It("should reject tokens when userValidationRules or CEL claimValidationRules evaluate to false", func() {
				g.By("checking that userValidationRule evaluating to 'false' causes authorization failure")
				o.Eventually(func(gomega o.Gomega) {
					err := keycloakCli.Authenticate("admin-cli", invalidUserValidation, invalidUserValidationPassword)
					gomega.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error authenticating as invalidUserValidation")

					copiedOC := *oc
					tokenOC := copiedOC.WithToken(keycloakCli.AccessToken())
					_, err = tokenOC.KubeClient().AuthenticationV1().SelfSubjectReviews().Create(ctx, &authnv1.SelfSubjectReview{
						ObjectMeta: metav1.ObjectMeta{
							Name: fmt.Sprintf("%s-info", invalidUserValidation),
						},
					}, metav1.CreateOptions{})
					gomega.Expect(err).To(o.HaveOccurred(), "should not be able to create a SelfSubjectReview")
					gomega.Expect(apierrors.IsUnauthorized(err)).To(o.BeTrue(), "should receive an unauthorized error")
				}).WithTimeout(5 * time.Minute).WithPolling(10 * time.Second).Should(o.Succeed())

				g.By("checking that CEL claimValidationRule evaluating to 'false' causes authorization failure")
				o.Eventually(func(gomega o.Gomega) {
					err := keycloakCli.Authenticate("admin-cli", invalidClaimValidation, invalidClaimValidationPassword)
					gomega.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error authenticating as invalidClaimValidation")

					copiedOC := *oc
					tokenOC := copiedOC.WithToken(keycloakCli.AccessToken())
					_, err = tokenOC.KubeClient().AuthenticationV1().SelfSubjectReviews().Create(ctx, &authnv1.SelfSubjectReview{
						ObjectMeta: metav1.ObjectMeta{
							Name: fmt.Sprintf("%s-info", invalidClaimValidation),
						},
					}, metav1.CreateOptions{})
					gomega.Expect(err).To(o.HaveOccurred(), "should not be able to create a SelfSubjectReview")
					gomega.Expect(apierrors.IsUnauthorized(err)).To(o.BeTrue(), "should receive an unauthorized error")
				}).WithTimeout(5 * time.Minute).WithPolling(10 * time.Second).Should(o.Succeed())
			})
		})

		g.Describe("with CEL expression-based claim mappings", g.Ordered, func() {
			var validExprUser, validExprUserPassword string
			var invalidExprUsername, invalidExprUsernamePassword string
			var invalidExprGroups, invalidExprGroupsPassword string

			g.BeforeAll(func() {
				testID := rand.String(8)

				// Create test users with specific claim values for expression mapping tests
				// validExprUser: email will be split to produce valid username, has BOTH 'ocp-' and non-'ocp-' groups to test filtering
				validExprUser = fmt.Sprintf("valid-expr-user-%s", testID)
				validExprUserPassword = fmt.Sprintf("password-valid-expr-%s", testID)
				validExprUserEmail := fmt.Sprintf("%s@example.com", validExprUser)
				otherGroup := "other-group"
				o.Expect(keycloakCli.CreateGroup(otherGroup)).To(o.Succeed(), "should be able to create other-group")
				// User belongs to BOTH 'ocp-test-*' group AND 'other-group' to verify filter works
				o.Expect(keycloakCli.CreateUserWithEmail(validExprUser, validExprUserEmail, validExprUserPassword, group, otherGroup)).To(o.Succeed(), "should be able to create validExprUser")

				// invalidExprUsername: email will be split to produce username that's too short (fails userValidationRules)
				invalidExprUsername = fmt.Sprintf("a%s", testID[:2])
				invalidExprUsernamePassword = fmt.Sprintf("password-invalid-expr-username-%s", testID)
				invalidExprUsernameEmail := fmt.Sprintf("%s@example.com", invalidExprUsername)
				o.Expect(keycloakCli.CreateUserWithEmail(invalidExprUsername, invalidExprUsernameEmail, invalidExprUsernamePassword, group)).To(o.Succeed(), "should be able to create invalidExprUsername")

				// invalidExprGroups: only in groups that don't start with 'ocp-', so filtered groups will be empty
				invalidExprGroups = fmt.Sprintf("invalid-groups-%s", testID)
				invalidExprGroupsPassword = fmt.Sprintf("password-invalid-expr-groups-%s", testID)
				invalidExprGroupsEmail := fmt.Sprintf("%s@example.com", invalidExprGroups)
				// User only belongs to 'other-group' (doesn't start with 'ocp-'), so filter produces empty list
				o.Expect(keycloakCli.CreateUserWithEmail(invalidExprGroups, invalidExprGroupsEmail, invalidExprGroupsPassword, otherGroup)).To(o.Succeed(), "should be able to create invalidExprGroups")

				// Configure OIDC provider with CEL expression-based claim mappings
				_, _, err := configureOIDCAuthentication(ctx, oc, keycloakNamespace, oidcClientSecret, func(provider *configv1.OIDCProvider) {
					idpUrl, err := admittedURLForRoute(ctx, oc, keycloakResourceName, keycloakNamespace)
					o.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error getting keycloak route URL")

					// Set custom discoveryURL
					provider.Issuer.DiscoveryURL = fmt.Sprintf("%s/realms/master/.well-known/openid-configuration", idpUrl)

					// Use CEL expressions for claim mappings
					// Note: Omitting prefixPolicy for username.expression and prefix for groups.expression
					// validates that these are allowed (per https://github.com/openshift/api/pull/2771)
					provider.ClaimMappings.Username = configv1.UsernameClaimMapping{
						Expression: "claims.email.split('@')[0]", // Extract username part from email
					}
					provider.ClaimMappings.Groups = configv1.PrefixedClaimMapping{
						TokenClaimMapping: configv1.TokenClaimMapping{
							Expression: "claims.groups.orValue([]).filter(g, g.startsWith('ocp-'))", // Only keep groups starting with 'ocp-'
						},
					}

					// Set UserValidationRules that validate the CEL-mapped username and groups
					provider.UserValidationRules = []configv1.TokenUserValidationRule{
						{
							Expression: "user.username.size() > 5",
							Message:    "username must be longer than 5 characters",
						},
						{
							Expression: "user.groups.size() > 0",
							Message:    "user must belong to at least one group after filtering",
						},
					}

					// Set ClaimValidationRules to validate claims before mapping
					provider.ClaimValidationRules = []configv1.TokenClaimValidationRule{
						{
							Type: configv1.TokenValidationRuleTypeCEL,
							CEL: configv1.TokenClaimValidationCELRule{
								Expression: "has(claims.email) && claims.email.contains('@')",
								Message:    "token must have valid email claim",
							},
						},
						{
							Type: configv1.TokenValidationRuleTypeCEL,
							CEL: configv1.TokenClaimValidationCELRule{
								Expression: "claims.email_verified == true",
								Message:    "email must be verified",
							},
						},
					}
				})
				o.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error configuring OIDC authentication with CEL expression mappings")

				waitForRollout(ctx, oc)
				waitForHealthyOIDCClients(ctx, oc)
			})

			g.It("should authenticate with CEL expression claim mappings (with omitted prefix configurations), userValidationRules, and claimValidationRules", func() {
				o.Eventually(func(gomega o.Gomega) {
					err := keycloakCli.Authenticate("admin-cli", validExprUser, validExprUserPassword)
					gomega.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error authenticating as validExprUser")

					copiedOC := *oc
					tokenOC := copiedOC.WithToken(keycloakCli.AccessToken())
					ssr, err := tokenOC.KubeClient().AuthenticationV1().SelfSubjectReviews().Create(ctx, &authnv1.SelfSubjectReview{
						ObjectMeta: metav1.ObjectMeta{
							Name: fmt.Sprintf("%s-info", validExprUser),
						},
					}, metav1.CreateOptions{})
					gomega.Expect(err).NotTo(o.HaveOccurred(), "should be able to create a SelfSubjectReview")

					// Verify CEL expression-based username mapping worked
					// Expression: claims.email.split('@')[0] should extract "valid-expr-user-<testID>" from email
					gomega.Expect(ssr.Status.UserInfo.Username).To(o.Equal(validExprUser), "username should be extracted from email via CEL expression")

					// Verify CEL expression-based groups mapping worked
					// The groups expression should keep ONLY groups starting with 'ocp-'
					gomega.Expect(ssr.Status.UserInfo.Groups).To(o.ContainElement(group), "groups should include 'ocp-test-*' group")
					gomega.Expect(ssr.Status.UserInfo.Groups).NotTo(o.ContainElement("other-group"), "groups should NOT include 'other-group' (filtered out)")
					// This test's configuration omits prefixPolicy for username.expression and prefix for groups.expression
					// Success for far implies that these are allowed (per https://github.com/openshift/api/pull/2771)

					// Verify userValidationRules (applied after CEL mapping)
					// Rule 1: user.username.size() > 5
					gomega.Expect(len(ssr.Status.UserInfo.Username)).To(o.BeNumerically(">", 5), "mapped username should be longer than 5 characters")
					// Rule 2: user.groups.size() > 0
					gomega.Expect(len(ssr.Status.UserInfo.Groups)).To(o.BeNumerically(">", 0), "user should have at least one group after filtering")

					// Successful authentication implies claimValidationRules passed (before mapping)
				}).WithTimeout(5 * time.Minute).WithPolling(10 * time.Second).Should(o.Succeed())
			})

			g.It("should reject when username or groups expression produces value failing userValidationRules", func() {
				g.By("checking that username expression producing value failing userValidationRules causes authorization failure")
				o.Eventually(func(gomega o.Gomega) {
					err := keycloakCli.Authenticate("admin-cli", invalidExprUsername, invalidExprUsernamePassword)
					gomega.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error authenticating as invalidExprUsername")

					copiedOC := *oc
					tokenOC := copiedOC.WithToken(keycloakCli.AccessToken())
					_, err = tokenOC.KubeClient().AuthenticationV1().SelfSubjectReviews().Create(ctx, &authnv1.SelfSubjectReview{
						ObjectMeta: metav1.ObjectMeta{
							Name: fmt.Sprintf("%s-info", invalidExprUsername),
						},
					}, metav1.CreateOptions{})
					gomega.Expect(err).To(o.HaveOccurred(), "should not be able to create a SelfSubjectReview")
					gomega.Expect(apierrors.IsUnauthorized(err)).To(o.BeTrue(), "should receive an unauthorized error due to username length validation failure")
					// Username expression produces "axx" (length 3), which fails userValidationRules: username.size() > 5
				}).WithTimeout(5 * time.Minute).WithPolling(10 * time.Second).Should(o.Succeed())

				g.By("checking that groups expression producing value failing userValidationRules causes authorization failure")
				o.Eventually(func(gomega o.Gomega) {
					err := keycloakCli.Authenticate("admin-cli", invalidExprGroups, invalidExprGroupsPassword)
					gomega.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error authenticating as invalidExprGroups")

					copiedOC := *oc
					tokenOC := copiedOC.WithToken(keycloakCli.AccessToken())
					_, err = tokenOC.KubeClient().AuthenticationV1().SelfSubjectReviews().Create(ctx, &authnv1.SelfSubjectReview{
						ObjectMeta: metav1.ObjectMeta{
							Name: fmt.Sprintf("%s-info", invalidExprGroups),
						},
					}, metav1.CreateOptions{})
					gomega.Expect(err).To(o.HaveOccurred(), "should not be able to create a SelfSubjectReview")
					gomega.Expect(apierrors.IsUnauthorized(err)).To(o.BeTrue(), "should receive an unauthorized error due to groups validation failure")
					// Groups expression filters to only 'ocp-*' groups, producing empty list, which fails userValidationRules: groups.size() > 0
				}).WithTimeout(5 * time.Minute).WithPolling(10 * time.Second).Should(o.Succeed())
			})
		})

		g.It("should reject invalid CEL expressions in admission", func() {
			// Test invalid CEL expression in userValidationRules
			_, _, err := configureOIDCAuthentication(ctx, oc, keycloakNamespace, oidcClientSecret, func(provider *configv1.OIDCProvider) {
				provider.UserValidationRules = []configv1.TokenUserValidationRule{
					{
						Expression: "!@#$%^&*()",
						Message:    "invalid expression",
					},
				}
			})
			o.Expect(err).To(o.HaveOccurred(), "should encounter an error with invalid CEL expression")

			// Test non-boolean CEL expression in userValidationRules
			_, _, err = configureOIDCAuthentication(ctx, oc, keycloakNamespace, oidcClientSecret, func(provider *configv1.OIDCProvider) {
				provider.UserValidationRules = []configv1.TokenUserValidationRule{
					{
						Expression: "user.username",
						Message:    "non-boolean expression",
					},
				}
			})
			o.Expect(err).To(o.HaveOccurred(), "should encounter an error with non-boolean CEL expression")

			// Test invalid CEL expression in claimValidationRules
			_, _, err = configureOIDCAuthentication(ctx, oc, keycloakNamespace, oidcClientSecret, func(provider *configv1.OIDCProvider) {
				provider.ClaimValidationRules = []configv1.TokenClaimValidationRule{
					{
						Type: configv1.TokenValidationRuleTypeCEL,
						CEL: configv1.TokenClaimValidationCELRule{
							Expression: "invalid syntax",
							Message:    "invalid expression",
						},
					},
				}
			})
			o.Expect(err).To(o.HaveOccurred(), "should encounter an error with invalid CEL expression in claimValidationRules")

			// Test invalid CEL expression in claimMappings.username.expression
			_, _, err = configureOIDCAuthentication(ctx, oc, keycloakNamespace, oidcClientSecret, func(provider *configv1.OIDCProvider) {
				provider.ClaimMappings.Username.Expression = "!@#$%^&*()"
			})
			o.Expect(err).To(o.HaveOccurred(), "should encounter an error with invalid CEL expression in username mapping")

			// Test invalid CEL expression in claimMappings.groups.expression
			_, _, err = configureOIDCAuthentication(ctx, oc, keycloakNamespace, oidcClientSecret, func(provider *configv1.OIDCProvider) {
				provider.ClaimMappings.Groups.TokenClaimMapping.Expression = "!@#$%^&*()"
			})
			o.Expect(err).To(o.HaveOccurred(), "should encounter an error with invalid CEL expression in groups mapping")
		})
	})

	g.Describe("[OCPFeatureGate:ExternalOIDCExternalClaimsSourcing]", g.Ordered, func() {
		var externalClaimsUser, externalClaimsUserPassword string
		var externalGroups []string

		g.BeforeAll(func() {
			// Dedicated client with groups only in /userinfo (not in access/id tokens),
			// so these tests prove groups were sourced externally rather than from the JWT.
			o.Expect(keycloakCli.ConfigureClientForExternalClaims(externalClaimsKeycloakClientID)).To(o.Succeed(),
				"should configure a Keycloak client for external claims testing")
		})

		g.Describe("with single external claims source", g.Ordered, func() {
			g.BeforeAll(func() {
				testID := rand.String(8)

				// Create test user with groups in Keycloak; the external-claims client
				// will not put those groups in the access token.
				externalClaimsUser = fmt.Sprintf("ext-claims-user-%s", testID)
				externalClaimsUserPassword = fmt.Sprintf("password-ext-claims-%s", testID)

				externalGroups = []string{
					fmt.Sprintf("ext-group-1-%s", testID),
					fmt.Sprintf("ext-group-2-%s", testID),
					fmt.Sprintf("ext-group-3-%s", testID),
				}

				for _, grp := range externalGroups {
					o.Expect(keycloakCli.CreateGroup(grp)).To(o.Succeed(), "should be able to create external groups")
				}
				o.Expect(keycloakCli.CreateUser(externalClaimsUser, externalClaimsUserPassword, externalGroups...)).To(o.Succeed(), "should be able to create user with external groups")

				// Configure OIDC with external claims sourcing (CNTRLPLANE-3475)
				_, _, err := configureOIDCAuthentication(ctx, oc, keycloakNamespace, oidcClientSecret, func(provider *configv1.OIDCProvider) {
					idpUrl, err := admittedURLForRoute(ctx, oc, keycloakResourceName, keycloakNamespace)
					o.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error getting keycloak route URL")

					hostname := strings.TrimPrefix(idpUrl, "https://")

					// Configure external claims source to fetch groups from /userinfo endpoint
					provider.ExternalClaimsSources = []configv1.ExternalClaimsSource{
						{
							Authentication: configv1.ExternalSourceAuthentication{
								Type: configv1.ExternalSourceAuthenticationTypeRequestProvidedToken,
							},
							URL: configv1.SourceURL{
								Hostname:       hostname,
								PathExpression: "['realms', 'master', 'protocol', 'openid-connect', 'userinfo']",
							},
							TLS: configv1.ExternalSourceTLS{
								CertificateAuthority: configv1.ExternalSourceCertificateAuthorityConfigMapReference{
									Name: keycloakCAConfigMapName(),
								},
							},
							Mappings: []configv1.SourcedClaimMapping{
								{
									Name:       "groups",
									Expression: "response.body.groups.join(',')",
								},
							},
						},
					}

					// Update groups claim mapping to parse comma-separated groups from external source
					provider.ClaimMappings.Groups = configv1.PrefixedClaimMapping{
						TokenClaimMapping: configv1.TokenClaimMapping{
							Expression: "claims.?groups.orValue('').split(',').filter(g, size(g) > 0)",
						},
					}
				})
				o.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error configuring OIDC with external claims sourcing")

				waitForRollout(ctx, oc)
				waitForHealthyOIDCClients(ctx, oc)
			})

			g.It("should source groups from external claims endpoint", func() {
				// CNTRLPLANE-3475: Single source simple mapping
				o.Eventually(func(gomega o.Gomega) {
					err := keycloakCli.Authenticate(externalClaimsKeycloakClientID, externalClaimsUser, externalClaimsUserPassword)
					gomega.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error authenticating as external claims user")

					copiedOC := *oc
					tokenOC := copiedOC.WithToken(keycloakCli.AccessToken())
					ssr, err := tokenOC.KubeClient().AuthenticationV1().SelfSubjectReviews().Create(ctx, &authnv1.SelfSubjectReview{
						ObjectMeta: metav1.ObjectMeta{
							Name: fmt.Sprintf("%s-info", externalClaimsUser),
						},
					}, metav1.CreateOptions{})
					gomega.Expect(err).NotTo(o.HaveOccurred(), "should be able to create a SelfSubjectReview")

					// Verify all external groups are present
					for _, expectedGroup := range externalGroups {
						gomega.Expect(ssr.Status.UserInfo.Groups).To(o.ContainElement(expectedGroup),
							fmt.Sprintf("should contain external group %s", expectedGroup))
					}

					// Verify we have at least the expected number of groups (may have system groups too)
					gomega.Expect(len(ssr.Status.UserInfo.Groups)).To(o.BeNumerically(">=", len(externalGroups)),
						"should have at least all external groups")
				}).WithTimeout(5 * time.Minute).WithPolling(10 * time.Second).Should(o.Succeed())
			})
		})

		// Keycloak exposes a single /userinfo endpoint, so this covers multiple claim
		// mappings against that endpoint (merge behavior), not two distinct IdP hosts.
		g.Describe("with multiple mappings from the same external claims endpoint", g.Ordered, func() {
			var multiSourceUser, multiSourceUserPassword string
			var source1Groups, source2Groups []string

			g.BeforeAll(func() {
				testID := rand.String(8)

				multiSourceUser = fmt.Sprintf("multi-source-user-%s", testID)
				multiSourceUserPassword = fmt.Sprintf("password-multi-source-%s", testID)

				// Create groups for two different mappings from the same /userinfo payload
				source1Groups = []string{fmt.Sprintf("source1-group-%s", testID)}
				source2Groups = []string{fmt.Sprintf("source2-group-%s", testID)}

				for _, grp := range append(source1Groups, source2Groups...) {
					o.Expect(keycloakCli.CreateGroup(grp)).To(o.Succeed(), "should be able to create groups")
				}
				o.Expect(keycloakCli.CreateUser(multiSourceUser, multiSourceUserPassword, append(source1Groups, source2Groups...)...)).To(o.Succeed())

				// Configure OIDC with two mappings from the same /userinfo source (CNTRLPLANE-3474/3481)
				_, _, err := configureOIDCAuthentication(ctx, oc, keycloakNamespace, oidcClientSecret, func(provider *configv1.OIDCProvider) {
					idpUrl, err := admittedURLForRoute(ctx, oc, keycloakResourceName, keycloakNamespace)
					o.Expect(err).NotTo(o.HaveOccurred())

					hostname := strings.TrimPrefix(idpUrl, "https://")

					provider.ExternalClaimsSources = []configv1.ExternalClaimsSource{
						{
							Authentication: configv1.ExternalSourceAuthentication{
								Type: configv1.ExternalSourceAuthenticationTypeRequestProvidedToken,
							},
							URL: configv1.SourceURL{
								Hostname:       hostname,
								PathExpression: "['realms', 'master', 'protocol', 'openid-connect', 'userinfo']",
							},
							TLS: configv1.ExternalSourceTLS{
								CertificateAuthority: configv1.ExternalSourceCertificateAuthorityConfigMapReference{
									Name: keycloakCAConfigMapName(),
								},
							},
							Mappings: []configv1.SourcedClaimMapping{
								{
									Name:       "groups_srcone",
									Expression: "response.body.groups.filter(g, g.startsWith('source1-')).join(',')",
								},
							},
						},
						{
							Authentication: configv1.ExternalSourceAuthentication{
								Type: configv1.ExternalSourceAuthenticationTypeRequestProvidedToken,
							},
							URL: configv1.SourceURL{
								Hostname:       hostname,
								PathExpression: "['realms', 'master', 'protocol', 'openid-connect', 'userinfo']",
							},
							TLS: configv1.ExternalSourceTLS{
								CertificateAuthority: configv1.ExternalSourceCertificateAuthorityConfigMapReference{
									Name: keycloakCAConfigMapName(),
								},
							},
							Mappings: []configv1.SourcedClaimMapping{
								{
									Name:       "groups_srctwo",
									Expression: "response.body.groups.filter(g, g.startsWith('source2-')).join(',')",
								},
							},
						},
					}

					provider.ClaimMappings.Groups = configv1.PrefixedClaimMapping{
						TokenClaimMapping: configv1.TokenClaimMapping{
							Expression: "(claims.?groups_srcone.orValue('') + ',' + claims.?groups_srctwo.orValue('')).split(',').filter(g, size(g) > 0)",
						},
					}
				})
				o.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error configuring multi-mapping external claims")

				waitForRollout(ctx, oc)
				waitForHealthyOIDCClients(ctx, oc)
			})

			g.It("should merge groups from multiple mappings of the same external claims endpoint", func() {
				// CNTRLPLANE-3474/3481: Multi-mapping claims merge
				o.Eventually(func(gomega o.Gomega) {
					err := keycloakCli.Authenticate(externalClaimsKeycloakClientID, multiSourceUser, multiSourceUserPassword)
					gomega.Expect(err).NotTo(o.HaveOccurred())

					copiedOC := *oc
					tokenOC := copiedOC.WithToken(keycloakCli.AccessToken())
					ssr, err := tokenOC.KubeClient().AuthenticationV1().SelfSubjectReviews().Create(ctx, &authnv1.SelfSubjectReview{
						ObjectMeta: metav1.ObjectMeta{
							Name: fmt.Sprintf("%s-info", multiSourceUser),
						},
					}, metav1.CreateOptions{})
					gomega.Expect(err).NotTo(o.HaveOccurred())

					// Verify groups from BOTH mappings are present
					for _, grp := range source1Groups {
						gomega.Expect(ssr.Status.UserInfo.Groups).To(o.ContainElement(grp),
							"should contain groups from mapping 1")
					}
					for _, grp := range source2Groups {
						gomega.Expect(ssr.Status.UserInfo.Groups).To(o.ContainElement(grp),
							"should contain groups from mapping 2")
					}
				}).WithTimeout(5 * time.Minute).WithPolling(10 * time.Second).Should(o.Succeed())
			})
		})

		// CNTRLPLANE-3481: sourcing from >1 ExternalClaimsSources entries.
		// Dual Keycloak hosts are not available in CI, so distinct sources are two
		// ExternalClaimsSources against the same /userinfo with different mapping names.
		// Setup lives in each It because openshift-tests does not reliably honor Ordered/BeforeAll
		// when running a single test name.
		g.Describe("with multiple external claims sources", func() {
			g.It("should merge claims from two reachable external claims sources", func() {
				testID := rand.String(8)
				o.Expect(keycloakCli.ConfigureClientForExternalClaims(externalClaimsKeycloakClientID)).To(o.Succeed(),
					"should configure a Keycloak client for external claims testing")

				user := fmt.Sprintf("two-src-user-%s", testID)
				userPassword := fmt.Sprintf("password-two-src-%s", testID)
				sourceAGroup := fmt.Sprintf("msrca-group-%s", testID)
				sourceBGroup := fmt.Sprintf("msrcb-group-%s", testID)

				for _, grp := range []string{sourceAGroup, sourceBGroup} {
					o.Expect(keycloakCli.CreateGroup(grp)).To(o.Succeed(), "should create two-source test group")
				}
				o.Expect(keycloakCli.CreateUser(user, userPassword, sourceAGroup, sourceBGroup)).To(o.Succeed(),
					"should create user with groups for both reachable sources")

				hostname := keycloakHostname(ctx, oc, keycloakNamespace)
				ensureOIDCConfigured(ctx, oc, keycloakNamespace, oidcClientSecret)
				baselineRevision := kubeAPIServerLatestRevision(ctx, oc)

				hotReloadExternalClaimsAndWait(ctx, oc, baselineRevision, "groups_src_a", "", func(provider *configv1.OIDCProvider) {
					provider.ExternalClaimsSources = []configv1.ExternalClaimsSource{
						keycloakUserInfoClaimsSource(hostname, configv1.SourcedClaimMapping{
							Name:       "groups_src_a",
							Expression: "response.body.groups.filter(g, g.startsWith('msrca-')).join(',')",
						}),
						keycloakUserInfoClaimsSource(hostname, configv1.SourcedClaimMapping{
							Name:       "groups_src_b",
							Expression: "response.body.groups.filter(g, g.startsWith('msrcb-')).join(',')",
						}),
					}
					provider.ClaimMappings.Groups = configv1.PrefixedClaimMapping{
						TokenClaimMapping: configv1.TokenClaimMapping{
							Expression: "(claims.?groups_src_a.orValue('') + ',' + claims.?groups_src_b.orValue('')).split(',').filter(g, size(g) > 0)",
						},
					}
				})

				o.Eventually(func(gomega o.Gomega) {
					groups, err := selfSubjectGroups(ctx, oc, keycloakCli, externalClaimsKeycloakClientID, user, userPassword)
					gomega.Expect(err).NotTo(o.HaveOccurred(), "two reachable sources: should authenticate")
					gomega.Expect(groups).To(o.ContainElement(sourceAGroup), "two reachable sources: should include claims from source A")
					gomega.Expect(groups).To(o.ContainElement(sourceBGroup), "two reachable sources: should include claims from source B")
				}).WithTimeout(5 * time.Minute).WithPolling(10 * time.Second).Should(o.Succeed())
			})

			g.It("should honor predicates when deciding whether to fetch an external claims source", func() {
				testID := rand.String(8)
				o.Expect(keycloakCli.ConfigureClientForExternalClaims(externalClaimsKeycloakClientID)).To(o.Succeed(),
					"should configure a Keycloak client for external claims testing")

				matchUser := fmt.Sprintf("pred-match-%s", testID)
				skipUser := fmt.Sprintf("pred-skip-%s", testID)
				userPassword := fmt.Sprintf("password-pred-%s", testID)
				predGroup := fmt.Sprintf("pred-group-%s", testID)

				o.Expect(keycloakCli.CreateGroup(predGroup)).To(o.Succeed(), "should create predicate test group")
				o.Expect(keycloakCli.CreateUser(matchUser, userPassword, predGroup)).To(o.Succeed(),
					"should create predicate-matching user")
				o.Expect(keycloakCli.CreateUser(skipUser, userPassword, predGroup)).To(o.Succeed(),
					"should create predicate-skipping user")

				hostname := keycloakHostname(ctx, oc, keycloakNamespace)
				ensureOIDCConfigured(ctx, oc, keycloakNamespace, oidcClientSecret)
				baselineRevision := kubeAPIServerLatestRevision(ctx, oc)

				predicateNeedle := fmt.Sprintf("pred-match-%s", testID)
				predSource := keycloakUserInfoClaimsSource(hostname, configv1.SourcedClaimMapping{
					Name:       "groups_predicate",
					Expression: "response.body.groups.filter(g, g.startsWith('pred-group-')).join(',')",
				})
				predSource.Predicates = []configv1.ExternalSourcePredicate{
					{
						// preferred_username is present on Keycloak access tokens; email is
						// the username claim but is not always copied onto the access token.
						Expression: fmt.Sprintf("has(claims.preferred_username) && claims.preferred_username == '%s'", matchUser),
					},
				}

				hotReloadExternalClaimsAndWait(ctx, oc, baselineRevision, predicateNeedle, "", func(provider *configv1.OIDCProvider) {
					provider.ExternalClaimsSources = []configv1.ExternalClaimsSource{predSource}
					provider.ClaimMappings.Groups = configv1.PrefixedClaimMapping{
						TokenClaimMapping: configv1.TokenClaimMapping{
							Expression: "claims.?groups_predicate.orValue('').split(',').filter(g, size(g) > 0)",
						},
					}
				})

				o.Eventually(func(gomega o.Gomega) {
					groups, err := selfSubjectGroups(ctx, oc, keycloakCli, externalClaimsKeycloakClientID, matchUser, userPassword)
					gomega.Expect(err).NotTo(o.HaveOccurred(), "predicate true: should authenticate and fetch the source")
					gomega.Expect(groups).To(o.ContainElement(predGroup), "predicate true: should include groups from the source")
				}).WithTimeout(5 * time.Minute).WithPolling(10 * time.Second).Should(o.Succeed())

				o.Eventually(func(gomega o.Gomega) {
					groups, err := selfSubjectGroups(ctx, oc, keycloakCli, externalClaimsKeycloakClientID, skipUser, userPassword)
					gomega.Expect(err).NotTo(o.HaveOccurred(), "predicate false: authentication should still succeed")
					gomega.Expect(groups).NotTo(o.ContainElement(predGroup), "predicate false: source must not be fetched")
				}).WithTimeout(5 * time.Minute).WithPolling(10 * time.Second).Should(o.Succeed())
			})

			g.It("should omit claims from an unreachable source while keeping claims from a reachable one", func() {
				testID := rand.String(8)
				o.Expect(keycloakCli.ConfigureClientForExternalClaims(externalClaimsKeycloakClientID)).To(o.Succeed(),
					"should configure a Keycloak client for external claims testing")

				user := fmt.Sprintf("partial-user-%s", testID)
				userPassword := fmt.Sprintf("password-partial-%s", testID)
				okGroup := fmt.Sprintf("msrcok-group-%s", testID)
				badGroup := fmt.Sprintf("msrcbad-group-%s", testID)

				for _, grp := range []string{okGroup, badGroup} {
					o.Expect(keycloakCli.CreateGroup(grp)).To(o.Succeed(), "should create partial-failure test group")
				}
				o.Expect(keycloakCli.CreateUser(user, userPassword, okGroup, badGroup)).To(o.Succeed(),
					"should create user with groups for reachable and omitted sources")

				hostname := keycloakHostname(ctx, oc, keycloakNamespace)
				ensureOIDCConfigured(ctx, oc, keycloakNamespace, oidcClientSecret)
				baselineRevision := kubeAPIServerLatestRevision(ctx, oc)

				const unreachableHost = "unreachable-host-that-does-not-exist.example.com"
				hotReloadExternalClaimsAndWait(ctx, oc, baselineRevision, unreachableHost, "", func(provider *configv1.OIDCProvider) {
					provider.ExternalClaimsSources = []configv1.ExternalClaimsSource{
						keycloakUserInfoClaimsSource(hostname, configv1.SourcedClaimMapping{
							Name:       "groups_ok",
							Expression: "response.body.groups.filter(g, g.startsWith('msrcok-')).join(',')",
						}),
						{
							Authentication: configv1.ExternalSourceAuthentication{
								Type: configv1.ExternalSourceAuthenticationTypeRequestProvidedToken,
							},
							URL: configv1.SourceURL{
								Hostname:       unreachableHost,
								PathExpression: "['userinfo']",
							},
							Mappings: []configv1.SourcedClaimMapping{
								{
									Name:       "groups_bad",
									Expression: "response.body.groups.filter(g, g.startsWith('msrcbad-')).join(',')",
								},
							},
						},
					}
					provider.ClaimMappings.Groups = configv1.PrefixedClaimMapping{
						TokenClaimMapping: configv1.TokenClaimMapping{
							Expression: "(claims.?groups_ok.orValue('') + ',' + claims.?groups_bad.orValue('')).split(',').filter(g, size(g) > 0)",
						},
					}
				})

				o.Eventually(func(gomega o.Gomega) {
					groups, err := selfSubjectGroups(ctx, oc, keycloakCli, externalClaimsKeycloakClientID, user, userPassword)
					gomega.Expect(err).NotTo(o.HaveOccurred(), "partial failure: reachable source should still allow authentication")
					gomega.Expect(groups).To(o.ContainElement(okGroup), "partial failure: groups from the reachable source should be present")
					gomega.Expect(groups).NotTo(o.ContainElement(badGroup), "partial failure: groups from the unreachable source should be omitted")
				}).WithTimeout(5 * time.Minute).WithPolling(10 * time.Second).Should(o.Succeed())
			})

			g.It("should evaluate dynamic pathExpression using claims.sub", func() {
				testID := rand.String(8)
				o.Expect(keycloakCli.ConfigureClientForExternalClaims(externalClaimsKeycloakClientID)).To(o.Succeed(),
					"should configure a Keycloak client for external claims testing")

				user := fmt.Sprintf("dynpath-user-%s", testID)
				userPassword := fmt.Sprintf("password-dynpath-%s", testID)
				dynGroup := fmt.Sprintf("dynpath-group-%s", testID)

				o.Expect(keycloakCli.CreateGroup(dynGroup)).To(o.Succeed(), "should create dynamic-path test group")
				o.Expect(keycloakCli.CreateUser(user, userPassword, dynGroup)).To(o.Succeed(),
					"should create user for dynamic pathExpression test")

				hostname := keycloakHostname(ctx, oc, keycloakNamespace)
				ensureOIDCConfigured(ctx, oc, keycloakNamespace, oidcClientSecret)
				baselineRevision := kubeAPIServerLatestRevision(ctx, oc)

				const missingPathMarker = "does-not-exist"
				hotReloadExternalClaimsAndWait(ctx, oc, baselineRevision, missingPathMarker, "", func(provider *configv1.OIDCProvider) {
					dynSource := keycloakUserInfoClaimsSource(hostname, configv1.SourcedClaimMapping{
						Name:       "groups_dyn",
						Expression: "response.body.groups.filter(g, g.startsWith('dynpath-')).join(',')",
					})
					// Keycloak userinfo is a fixed path; claims.sub selects that path vs a 404.
					dynSource.URL.PathExpression = fmt.Sprintf("has(claims.sub) && size(claims.sub) > 0 ? ['realms', 'master', 'protocol', 'openid-connect', 'userinfo'] : ['%s']", missingPathMarker)
					provider.ExternalClaimsSources = []configv1.ExternalClaimsSource{dynSource}
					provider.ClaimMappings.Groups = configv1.PrefixedClaimMapping{
						TokenClaimMapping: configv1.TokenClaimMapping{
							Expression: "claims.?groups_dyn.orValue('').split(',').filter(g, size(g) > 0)",
						},
					}
				})

				o.Eventually(func(gomega o.Gomega) {
					groups, err := selfSubjectGroups(ctx, oc, keycloakCli, externalClaimsKeycloakClientID, user, userPassword)
					gomega.Expect(err).NotTo(o.HaveOccurred(), "dynamic path: should authenticate when pathExpression uses claims.sub")
					gomega.Expect(groups).To(o.ContainElement(dynGroup), "dynamic path: should source groups from the claims.sub-selected userinfo path")
				}).WithTimeout(5 * time.Minute).WithPolling(10 * time.Second).Should(o.Succeed())
			})
		})

		g.Describe("with invalid external claims source", g.Ordered, func() {
			var errorHandlingUser, errorHandlingUserPassword string

			g.BeforeAll(func() {
				testID := rand.String(8)

				errorHandlingUser = fmt.Sprintf("error-user-%s", testID)
				errorHandlingUserPassword = fmt.Sprintf("password-error-%s", testID)

				// User has groups in Keycloak, but the external-claims client will not
				// put them in the access token. With an unreachable external source,
				// sourced groups are omitted and UserValidationRule fails.
				o.Expect(keycloakCli.CreateUser(errorHandlingUser, errorHandlingUserPassword, group)).To(o.Succeed())

				// Configure OIDC with INVALID external claims source (CNTRLPLANE-3482)
				_, _, err := configureOIDCAuthentication(ctx, oc, keycloakNamespace, oidcClientSecret, func(provider *configv1.OIDCProvider) {
					// Configure external claims source with unreachable host
					provider.ExternalClaimsSources = []configv1.ExternalClaimsSource{
						{
							Authentication: configv1.ExternalSourceAuthentication{
								Type: configv1.ExternalSourceAuthenticationTypeRequestProvidedToken,
							},
							URL: configv1.SourceURL{
								Hostname:       "unreachable-host-that-does-not-exist.example.com",
								PathExpression: "['userinfo']",
							},
							Mappings: []configv1.SourcedClaimMapping{
								{
									Name:       "groups",
									Expression: "response.body.groups.join(',')",
								},
							},
						},
					}

					provider.ClaimMappings.Groups = configv1.PrefixedClaimMapping{
						TokenClaimMapping: configv1.TokenClaimMapping{
							// Filter empty segments: ''.split(',') yields [""] which would
							// otherwise make user.groups.size() > 0 succeed incorrectly.
							Expression: "claims.?groups.orValue('').split(',').filter(g, size(g) > 0)",
						},
					}

					// Require a real Keycloak group that can only appear if external claims
					// sourcing succeeded. size()>0 alone is insufficient because an empty
					// split still produces a one-element list of "".
					provider.UserValidationRules = []configv1.TokenUserValidationRule{
						{
							Expression: "user.groups.exists(g, size(g) > 0 && g.startsWith('ocp-test-'))",
							Message:    "user must have at least one ocp-test-* group from external claims",
						},
					}
				})
				o.Expect(err).NotTo(o.HaveOccurred())

				waitForRollout(ctx, oc)
				waitForHealthyOIDCClients(ctx, oc)
			})

			g.It("should reject authentication when external claims source is unreachable and validation requires sourced claims", func() {
				// CNTRLPLANE-3482: Error handling - bad host
				// When an external claims source is unreachable, sourced claims are omitted.
				// With a UserValidationRule requiring ocp-test-* groups, authentication fails
				// because the presented token has no groups and the omitted external claims
				// leave no qualifying groups.
				o.Eventually(func(gomega o.Gomega) {
					err := keycloakCli.Authenticate(externalClaimsKeycloakClientID, errorHandlingUser, errorHandlingUserPassword)
					gomega.Expect(err).NotTo(o.HaveOccurred())

					copiedOC := *oc
					tokenOC := copiedOC.WithToken(keycloakCli.AccessToken())
					_, err = tokenOC.KubeClient().AuthenticationV1().SelfSubjectReviews().Create(ctx, &authnv1.SelfSubjectReview{
						ObjectMeta: metav1.ObjectMeta{
							Name: fmt.Sprintf("%s-info", errorHandlingUser),
						},
					}, metav1.CreateOptions{})

					// Authentication should fail because:
					// 1. Access token has no groups (external-claims client)
					// 2. External source is unreachable → claims omitted
					// 3. UserValidationRule requires an ocp-test-* group
					gomega.Expect(err).To(o.HaveOccurred(),
						"should receive an error when external claims are omitted and validation requires groups")
					gomega.Expect(apierrors.IsUnauthorized(err)).To(o.BeTrue(),
						"should receive Unauthorized error due to UserValidationRule failure")
				}).WithTimeout(5 * time.Minute).WithPolling(10 * time.Second).Should(o.Succeed())
			})
		})

		// CNTRLPLANE-3483: hot reload of ExternalClaimsSources on oauth-apiserver.
		// Implemented as a single It because g.Ordered/BeforeAll are not reliably honored
		// by openshift-tests today; sequential mutate→sync→assert steps must stay together.
		//
		// Once the cluster is already OIDC, ExternalClaimsSources changes must be picked up
		// without a kube-apiserver static-pod revision (oauth-apiserver auth-config hot reload).
		g.Describe("hot reloading external claims source configuration", g.Ordered, func() {
			var hotReloadUser, hotReloadPassword string
			var keepGroup, dropGroup string

			g.It("should pick up ExternalClaimsSources changes without a kube-apiserver revision rollout", func() {
				testID := rand.String(8)
				hotReloadUser = fmt.Sprintf("hot-reload-user-%s", testID)
				hotReloadPassword = fmt.Sprintf("password-hot-reload-%s", testID)
				keepGroup = fmt.Sprintf("hotreload-keep-%s", testID)
				dropGroup = fmt.Sprintf("hotreload-drop-%s", testID)
				// Unique auth-config markers (must not rely on issuer hostname — that is already present).
				phase1Marker := fmt.Sprintf("hotreload-phase1-%s", testID)
				phase4Marker := fmt.Sprintf("hotreload-phase4-%s", testID)

				for _, grp := range []string{keepGroup, dropGroup} {
					o.Expect(keycloakCli.CreateGroup(grp)).To(o.Succeed(), "should create hot-reload test group")
				}
				o.Expect(keycloakCli.CreateUser(hotReloadUser, hotReloadPassword, keepGroup, dropGroup)).To(o.Succeed(),
					"should create hot-reload test user")

				hostname := keycloakHostname(ctx, oc, keycloakNamespace)

				// Bootstrap OIDC if needed (openshift-tests may run this It first). Enabling OIDC
				// may roll kube-apiserver; subsequent ExternalClaimsSources edits must not.
				ensureOIDCConfigured(ctx, oc, keycloakNamespace, oidcClientSecret)
				baselineRevision := kubeAPIServerLatestRevision(ctx, oc)

				hotReloadExternalClaimsAndWait(ctx, oc, baselineRevision, phase1Marker, "", func(provider *configv1.OIDCProvider) {
					provider.ExternalClaimsSources = []configv1.ExternalClaimsSource{
						keycloakUserInfoClaimsSource(hostname, configv1.SourcedClaimMapping{
							Name:       "groups",
							Expression: "response.body.groups.join(',')",
						}),
					}
					// No-op filter embeds phase1Marker so auth-config sync is observable.
					provider.ClaimMappings.Groups = configv1.PrefixedClaimMapping{
						TokenClaimMapping: configv1.TokenClaimMapping{
							Expression: fmt.Sprintf("claims.?groups.orValue('').split(',').filter(g, size(g) > 0 && !g.startsWith('%s'))", phase1Marker),
						},
					}
				})

				// Phase 1: initial config sources all Keycloak groups from /userinfo.
				o.Eventually(func(gomega o.Gomega) {
					groups, err := selfSubjectGroups(ctx, oc, keycloakCli, externalClaimsKeycloakClientID, hotReloadUser, hotReloadPassword)
					gomega.Expect(err).NotTo(o.HaveOccurred(), "phase1: should authenticate with initial external claims config")
					gomega.Expect(groups).To(o.ContainElement(keepGroup), "phase1: should contain keep group")
					gomega.Expect(groups).To(o.ContainElement(dropGroup), "phase1: should contain drop group")
				}).WithTimeout(5 * time.Minute).WithPolling(10 * time.Second).Should(o.Succeed())
				expectKubeAPIServerRevisionUnchanged(ctx, oc, baselineRevision, "after phase1 auth success")

				// Phase 2: narrow the mapping expression; only keep-* groups should remain.
				hotReloadExternalClaimsAndWait(ctx, oc, baselineRevision, "hotreload-keep-", "", func(provider *configv1.OIDCProvider) {
					provider.ExternalClaimsSources = []configv1.ExternalClaimsSource{
						keycloakUserInfoClaimsSource(hostname, configv1.SourcedClaimMapping{
							Name:       "groups",
							Expression: "response.body.groups.filter(g, g.startsWith('hotreload-keep-')).join(',')",
						}),
					}
					provider.ClaimMappings.Groups = configv1.PrefixedClaimMapping{
						TokenClaimMapping: configv1.TokenClaimMapping{
							Expression: "claims.?groups.orValue('').split(',').filter(g, size(g) > 0)",
						},
					}
				})

				o.Eventually(func(gomega o.Gomega) {
					groups, err := selfSubjectGroups(ctx, oc, keycloakCli, externalClaimsKeycloakClientID, hotReloadUser, hotReloadPassword)
					gomega.Expect(err).NotTo(o.HaveOccurred(), "phase2: should authenticate after mapping change")
					gomega.Expect(groups).To(o.ContainElement(keepGroup), "phase2: filtered mapping should still include keep group")
					gomega.Expect(groups).NotTo(o.ContainElement(dropGroup), "phase2: filtered mapping should drop non-matching groups")
				}).WithTimeout(5 * time.Minute).WithPolling(10 * time.Second).Should(o.Succeed())
				expectKubeAPIServerRevisionUnchanged(ctx, oc, baselineRevision, "after phase2 auth success")

				const unreachableHost = "unreachable-host-that-does-not-exist.example.com"

				// Phase 3: replace source with an unreachable host and require sourced groups.
				hotReloadExternalClaimsAndWait(ctx, oc, baselineRevision, unreachableHost, "", func(provider *configv1.OIDCProvider) {
					provider.ExternalClaimsSources = []configv1.ExternalClaimsSource{
						{
							Authentication: configv1.ExternalSourceAuthentication{
								Type: configv1.ExternalSourceAuthenticationTypeRequestProvidedToken,
							},
							URL: configv1.SourceURL{
								Hostname:       unreachableHost,
								PathExpression: "['userinfo']",
							},
							Mappings: []configv1.SourcedClaimMapping{
								{
									Name:       "groups",
									Expression: "response.body.groups.join(',')",
								},
							},
						},
					}
					provider.ClaimMappings.Groups = configv1.PrefixedClaimMapping{
						TokenClaimMapping: configv1.TokenClaimMapping{
							Expression: "claims.?groups.orValue('').split(',').filter(g, size(g) > 0)",
						},
					}
					provider.UserValidationRules = []configv1.TokenUserValidationRule{
						{
							Expression: fmt.Sprintf("user.groups.exists(g, g == '%s')", keepGroup),
							Message:    "user must retain hot-reload keep group from external claims",
						},
					}
				})

				o.Eventually(func(gomega o.Gomega) {
					err := keycloakCli.Authenticate(externalClaimsKeycloakClientID, hotReloadUser, hotReloadPassword)
					gomega.Expect(err).NotTo(o.HaveOccurred(), "phase3: should still obtain a Keycloak token")

					copiedOC := *oc
					tokenOC := copiedOC.WithToken(keycloakCli.AccessToken())
					_, err = tokenOC.KubeClient().AuthenticationV1().SelfSubjectReviews().Create(ctx, &authnv1.SelfSubjectReview{
						ObjectMeta: metav1.ObjectMeta{
							Name: fmt.Sprintf("%s-info", hotReloadUser),
						},
					}, metav1.CreateOptions{})
					gomega.Expect(err).To(o.HaveOccurred(), "phase3: unreachable source should omit claims and fail validation")
					gomega.Expect(apierrors.IsUnauthorized(err)).To(o.BeTrue(), "phase3: should be Unauthorized")
				}).WithTimeout(5 * time.Minute).WithPolling(10 * time.Second).Should(o.Succeed())
				expectKubeAPIServerRevisionUnchanged(ctx, oc, baselineRevision, "after phase3 auth failure")

				// Phase 4: restore a working source; authentication and groups recover.
				hotReloadExternalClaimsAndWait(ctx, oc, baselineRevision, phase4Marker, unreachableHost, func(provider *configv1.OIDCProvider) {
					provider.ExternalClaimsSources = []configv1.ExternalClaimsSource{
						keycloakUserInfoClaimsSource(hostname, configv1.SourcedClaimMapping{
							Name:       "groups",
							Expression: "response.body.groups.join(',')",
						}),
					}
					provider.ClaimMappings.Groups = configv1.PrefixedClaimMapping{
						TokenClaimMapping: configv1.TokenClaimMapping{
							Expression: fmt.Sprintf("claims.?groups.orValue('').split(',').filter(g, size(g) > 0 && !g.startsWith('%s'))", phase4Marker),
						},
					}
				})

				o.Eventually(func(gomega o.Gomega) {
					groups, err := selfSubjectGroups(ctx, oc, keycloakCli, externalClaimsKeycloakClientID, hotReloadUser, hotReloadPassword)
					gomega.Expect(err).NotTo(o.HaveOccurred(), "phase4: should authenticate after restoring external claims source")
					gomega.Expect(groups).To(o.ContainElement(keepGroup), "phase4: restored source should include keep group")
					gomega.Expect(groups).To(o.ContainElement(dropGroup), "phase4: restored source should include drop group")
				}).WithTimeout(5 * time.Minute).WithPolling(10 * time.Second).Should(o.Succeed())
				expectKubeAPIServerRevisionUnchanged(ctx, oc, baselineRevision, "after phase4 auth success")
			})
		})

		// CNTRLPLANE-3473: functional coverage for sourcing arbitrarily large
		// numbers of groups. IDPs cap both what tokens carry (an identity token
		// holds at most ~200 groups) and what a single response returns (Entra
		// ID's Graph API caps at 999 groups per response). The default count
		// crosses both thresholds so the claim set must be assembled from an
		// external response far larger than what any token carries. Override
		// EXTERNAL_CLAIMS_GROUP_COUNT for on-demand runs with larger sets.
		g.Describe("with a large number of sourced groups", func() {
			g.It("should source an arbitrarily large number of groups from an external claims source", func() {
				testID := rand.String(8)
				o.Expect(keycloakCli.ConfigureClientForExternalClaims(externalClaimsKeycloakClientID)).To(o.Succeed(),
					"should configure a Keycloak client for external claims testing")

				groupCount := envPositiveInt("EXTERNAL_CLAIMS_GROUP_COUNT", 1200)
				groupPrefix := fmt.Sprintf("lg-%s", testID)
				expectedGroups := indexedGroupNames(groupPrefix, groupCount)

				user := fmt.Sprintf("large-groups-user-%s", testID)
				userPassword := fmt.Sprintf("password-large-groups-%s", testID)

				reauthAdmin := func() error {
					return keycloakCli.Authenticate("admin-cli", keycloakAdminUsername, keycloakAdminPassword)
				}
				o.Expect(keycloakCli.CreateGroups(expectedGroups, reauthAdmin)).To(o.Succeed(),
					fmt.Sprintf("should create %d groups for large-group sourcing", groupCount))
				o.Expect(keycloakCli.CreateUser(user, userPassword, expectedGroups...)).To(o.Succeed(),
					"should create user with a large number of groups")

				hostname := keycloakHostname(ctx, oc, keycloakNamespace)
				ensureOIDCConfigured(ctx, oc, keycloakNamespace, oidcClientSecret)
				baselineRevision := kubeAPIServerLatestRevision(ctx, oc)

				hotReloadExternalClaimsAndWait(ctx, oc, baselineRevision, groupPrefix, "", func(provider *configv1.OIDCProvider) {
					provider.ExternalClaimsSources = []configv1.ExternalClaimsSource{
						keycloakUserInfoClaimsSource(hostname, configv1.SourcedClaimMapping{
							Name:       "groups_large",
							Expression: "response.body.groups.join(',')",
						}),
					}
					provider.ClaimMappings.Groups = configv1.PrefixedClaimMapping{
						TokenClaimMapping: configv1.TokenClaimMapping{
							Expression: fmt.Sprintf("claims.?groups_large.orValue('').split(',').filter(g, g.startsWith('%s'))", groupPrefix),
						},
					}
				})

				o.Eventually(func(gomega o.Gomega) {
					groups, err := selfSubjectGroups(ctx, oc, keycloakCli, externalClaimsKeycloakClientID, user, userPassword)
					gomega.Expect(err).NotTo(o.HaveOccurred(), "large group count: should authenticate with externally sourced groups")
					gomega.Expect(groupsMatchingPrefix(groups, groupPrefix)).To(o.ConsistOf(expectedGroups),
						fmt.Sprintf("should source all %d groups from the external claims source", groupCount))
				}).WithTimeout(15 * time.Minute).WithPolling(15 * time.Second).Should(o.Succeed())
			})

			// CNTRLPLANE-3476: performance coverage for an external claims source
			// returning increasing numbers of groups. Counts default to a
			// CI-friendly ladder; set EXTERNAL_CLAIMS_GROUP_COUNTS (comma-separated,
			// e.g. 200,500,1000,2000,5000,10000) for on-demand runs and collect the
			// SelfSubjectReview round-trip timings printed by this test.
			g.It("should source increasing numbers of groups from an external claims source", func() {
				testID := rand.String(8)
				o.Expect(keycloakCli.ConfigureClientForExternalClaims(externalClaimsKeycloakClientID)).To(o.Succeed(),
					"should configure a Keycloak client for external claims testing")

				counts := envPositiveInts("EXTERNAL_CLAIMS_GROUP_COUNTS", []int{200, 500, 1000, 2000})
				sort.Ints(counts)
				deduplicated := counts[:0]
				for i, count := range counts {
					if i == 0 || count != counts[i-1] {
						deduplicated = append(deduplicated, count)
					}
				}
				counts = deduplicated

				groupPrefix := fmt.Sprintf("pg-%s", testID)
				pool := indexedGroupNames(groupPrefix, counts[len(counts)-1])

				type ladderUser struct {
					count          int
					name, password string
				}
				users := make([]ladderUser, 0, len(counts))
				for _, count := range counts {
					users = append(users, ladderUser{
						count:    count,
						name:     fmt.Sprintf("perf-user-%05d-%s", count, testID),
						password: fmt.Sprintf("password-perf-%05d-%s", count, testID),
					})
				}

				reauthAdmin := func() error {
					return keycloakCli.Authenticate("admin-cli", keycloakAdminUsername, keycloakAdminPassword)
				}
				o.Expect(keycloakCli.CreateGroups(pool, reauthAdmin)).To(o.Succeed(),
					fmt.Sprintf("should create %d groups for the group-count ladder", len(pool)))
				for _, ladder := range users {
					o.Expect(keycloakCli.CreateUser(ladder.name, ladder.password, pool[:ladder.count]...)).To(o.Succeed(),
						fmt.Sprintf("should create user with %d groups", ladder.count))
				}

				hostname := keycloakHostname(ctx, oc, keycloakNamespace)
				ensureOIDCConfigured(ctx, oc, keycloakNamespace, oidcClientSecret)
				baselineRevision := kubeAPIServerLatestRevision(ctx, oc)

				// A single source serves every ladder user because /userinfo
				// returns exactly the groups of the authenticating user.
				hotReloadExternalClaimsAndWait(ctx, oc, baselineRevision, groupPrefix, "", func(provider *configv1.OIDCProvider) {
					provider.ExternalClaimsSources = []configv1.ExternalClaimsSource{
						keycloakUserInfoClaimsSource(hostname, configv1.SourcedClaimMapping{
							Name:       "groups_perf",
							Expression: "response.body.groups.join(',')",
						}),
					}
					provider.ClaimMappings.Groups = configv1.PrefixedClaimMapping{
						TokenClaimMapping: configv1.TokenClaimMapping{
							Expression: fmt.Sprintf("claims.?groups_perf.orValue('').split(',').filter(g, g.startsWith('%s'))", groupPrefix),
						},
					}
				})

				for _, ladder := range users {
					expectedGroups := pool[:ladder.count]
					// elapsed holds the round-trip of the last (successful) attempt.
					var elapsed time.Duration
					o.Eventually(func(gomega o.Gomega) {
						start := time.Now()
						groups, err := selfSubjectGroups(ctx, oc, keycloakCli, externalClaimsKeycloakClientID, ladder.name, ladder.password)
						elapsed = time.Since(start)
						gomega.Expect(err).NotTo(o.HaveOccurred(),
							fmt.Sprintf("group-count ladder (%d groups): should authenticate", ladder.count))
						gomega.Expect(groupsMatchingPrefix(groups, groupPrefix)).To(o.ConsistOf(expectedGroups),
							fmt.Sprintf("group-count ladder (%d groups): should source all groups", ladder.count))
					}).WithTimeout(15 * time.Minute).WithPolling(15 * time.Second).Should(o.Succeed())

					g.GinkgoWriter.Printf("CNTRLPLANE-3476: external claims source returned %d groups; SelfSubjectReview round-trip: %s\n", ladder.count, elapsed)
				}
			})
		})

		// CNTRLPLANE-3477: performance coverage for complex CEL expressions on
		// source responses. Every stage of the external claims pipeline that takes
		// CEL (predicate, pathExpression, mapping, groups claim mapping, user
		// validation rule) is exercised with a chained expression, and the
		// SelfSubjectReview round-trip is logged so runs double as data points for
		// the impact of complex CEL on the auth flow.
		g.Describe("with complex CEL expressions on source responses", func() {
			g.It("should authenticate when mappings, predicates, and claim expressions are complex", func() {
				testID := rand.String(8)
				o.Expect(keycloakCli.ConfigureClientForExternalClaims(externalClaimsKeycloakClientID)).To(o.Succeed(),
					"should configure a Keycloak client for external claims testing")

				const groupCount = 20
				groupPrefix := fmt.Sprintf("celg-%s", testID)
				expectedGroups := indexedGroupNames(groupPrefix, groupCount)

				user := fmt.Sprintf("cel-user-%s", testID)
				userPassword := fmt.Sprintf("password-cel-%s", testID)

				o.Expect(keycloakCli.CreateGroups(expectedGroups, nil)).To(o.Succeed(),
					"should create groups for complex CEL expression testing")
				o.Expect(keycloakCli.CreateUser(user, userPassword, expectedGroups...)).To(o.Succeed(),
					"should create user for complex CEL expression testing")

				hostname := keycloakHostname(ctx, oc, keycloakNamespace)
				ensureOIDCConfigured(ctx, oc, keycloakNamespace, oidcClientSecret)
				baselineRevision := kubeAPIServerLatestRevision(ctx, oc)

				lastGroup := expectedGroups[groupCount-1]
				hotReloadExternalClaimsAndWait(ctx, oc, baselineRevision, groupPrefix, "", func(provider *configv1.OIDCProvider) {
					source := keycloakUserInfoClaimsSource(hostname,
						configv1.SourcedClaimMapping{
							Name: "groups_cel_upper",
							Expression: fmt.Sprintf(
								"response.body.groups.filter(g, g.startsWith('%s')).map(g, g.upperAscii()).filter(g, size(g) > 8).map(g, g.lowerAscii()).join(',')",
								groupPrefix),
						},
						configv1.SourcedClaimMapping{
							Name: "groups_cel_nested",
							Expression: fmt.Sprintf(
								"response.body.groups.filter(g, g.startsWith('%s')).map(g, g.matches('^%s-[0-9]+$') ? g : g + '-fallback').join(',')",
								groupPrefix, groupPrefix),
						},
					)
					// Ternary path expression chaining list concatenation.
					source.URL.PathExpression = "has(claims.sub) && size(claims.sub) > 0 ? (['realms', 'master'] + ['protocol', 'openid-connect', 'userinfo']) : ['does-not-exist']"
					source.Predicates = []configv1.ExternalSourcePredicate{
						{
							Expression: "has(claims.preferred_username) && claims.preferred_username.startsWith('cel-user-') && size(claims.sub) > 0",
						},
					}
					provider.ExternalClaimsSources = []configv1.ExternalClaimsSource{source}

					// Intersect both mappings: keep the round-tripped names that
					// also survive the nested ternary mapping.
					provider.ClaimMappings.Groups = configv1.PrefixedClaimMapping{
						TokenClaimMapping: configv1.TokenClaimMapping{
							Expression: "claims.?groups_cel_upper.orValue('').split(',').filter(g, size(g) > 0 && claims.?groups_cel_nested.orValue('').split(',').exists(n, n == g)).map(g, g.lowerAscii())",
						},
					}
					provider.UserValidationRules = []configv1.TokenUserValidationRule{
						{
							Expression: fmt.Sprintf("user.groups.filter(g, g.startsWith('%s')).size() == %d && user.groups.exists(g, g == '%s') && user.username.contains('@')",
								groupPrefix, groupCount, lastGroup),
							Message: "user must authenticate with the full complex-CEL sourced group set",
						},
					}
				})

				var elapsed time.Duration
				o.Eventually(func(gomega o.Gomega) {
					start := time.Now()
					groups, err := selfSubjectGroups(ctx, oc, keycloakCli, externalClaimsKeycloakClientID, user, userPassword)
					elapsed = time.Since(start)
					gomega.Expect(err).NotTo(o.HaveOccurred(), "complex CEL: should authenticate")
					gomega.Expect(groupsMatchingPrefix(groups, groupPrefix)).To(o.ConsistOf(expectedGroups),
						"complex CEL: should source the full group set through chained expressions")
				}).WithTimeout(10 * time.Minute).WithPolling(15 * time.Second).Should(o.Succeed())

				g.GinkgoWriter.Printf("CNTRLPLANE-3477: complex CEL expressions across %d sourced groups; SelfSubjectReview round-trip: %s\n", groupCount, elapsed)
			})
		})

		// CNTRLPLANE-3479: functional coverage for sourcing groups from Entra
		// ID's Graph API. A real Entra tenant is not reachable from CI, so this
		// test reproduces the Graph integration pattern against Keycloak:
		//   - the source authenticates with the OAuth2 client credentials grant
		//     (authentication.type=ClientCredential), which is how a separate app
		//     credential is used to access Graph;
		//   - the source path is derived per subject (Graph:
		//     /v1.0/users/{sub}/transitiveMemberOf; here:
		//     /admin/realms/master/users/{sub}/groups);
		//   - the response is an array of objects (Graph: {"value":[{"id":...}]};
		//     here: [{"name":...}]) unwrapped by a CEL mapping expression.
		g.Describe("with a Graph-like external claims source using client credentials", func() {
			g.It("should source groups using client credentials against a Graph-like endpoint", func() {
				testID := rand.String(8)
				o.Expect(keycloakCli.ConfigureClientForExternalClaims(externalClaimsKeycloakClientID)).To(o.Succeed(),
					"should configure a Keycloak client for external claims testing")

				groupPrefix := fmt.Sprintf("entra-%s", testID)
				expectedGroups := indexedGroupNames(groupPrefix, 3)

				user := fmt.Sprintf("entra-user-%s", testID)
				userPassword := fmt.Sprintf("password-entra-%s", testID)

				o.Expect(keycloakCli.CreateGroups(expectedGroups, nil)).To(o.Succeed(),
					"should create groups for Graph-like sourcing")
				o.Expect(keycloakCli.CreateUser(user, userPassword, expectedGroups...)).To(o.Succeed(),
					"should create user with groups for Graph-like sourcing")

				// Confidential client with a service account: the apiserver
				// obtains the source token via the client credentials grant with
				// this client, mirroring the app credentials used with Graph.
				const graphSimClientID = "external-claims-client-credentials"
				// The service account needs read-only roles to look up user
				// group memberships through the Admin REST API. Both operations
				// are idempotent, and each attempt re-authenticates because the
				// master realm's access tokens only live for 60 seconds (see
				// Keycloak's ApplianceBootstrap#createMasterRealm), so a token
				// obtained earlier can expire mid-retry.
				var clientSecret string
				o.Eventually(func(gomega o.Gomega) {
					gomega.Expect(keycloakCli.Authenticate("admin-cli", keycloakAdminUsername, keycloakAdminPassword)).To(o.Succeed(),
						"should authenticate as the keycloak admin")
					var err error
					clientSecret, err = keycloakCli.EnsureConfidentialServiceAccountClient(graphSimClientID)
					gomega.Expect(err).NotTo(o.HaveOccurred(), "should ensure a confidential client for the client credentials grant")
					gomega.Expect(keycloakCli.GrantRealmManagementRoles(graphSimClientID, "view-users", "query-users")).To(o.Succeed(),
						"should grant the service account read-only user lookup roles")
				}).WithTimeout(2 * time.Minute).WithPolling(10 * time.Second).Should(o.Succeed())

				secretName := fmt.Sprintf("external-claims-cc-%s", testID)
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      secretName,
						Namespace: "openshift-config",
					},
					Data: map[string][]byte{
						// The key the apiserver looks up for ClientCredential sources.
						"client-secret": []byte(clientSecret),
					},
				}
				_, err := oc.AdminKubeClient().CoreV1().Secrets("openshift-config").Create(ctx, secret, metav1.CreateOptions{})
				o.Expect(err).NotTo(o.HaveOccurred(), "should create the client credentials secret in openshift-config")
				defer func() {
					_ = oc.AdminKubeClient().CoreV1().Secrets("openshift-config").Delete(ctx, secretName, metav1.DeleteOptions{})
				}()

				hostname := keycloakHostname(ctx, oc, keycloakNamespace)
				tokenEndpoint := fmt.Sprintf("https://%s/realms/master/protocol/openid-connect/token", hostname)

				ensureOIDCConfigured(ctx, oc, keycloakNamespace, oidcClientSecret)
				baselineRevision := kubeAPIServerLatestRevision(ctx, oc)

				hotReloadExternalClaimsAndWait(ctx, oc, baselineRevision, graphSimClientID, "", func(provider *configv1.OIDCProvider) {
					provider.ExternalClaimsSources = []configv1.ExternalClaimsSource{
						{
							Authentication: configv1.ExternalSourceAuthentication{
								Type: configv1.ExternalSourceAuthenticationTypeClientCredential,
								ClientCredential: configv1.ClientCredentialConfig{
									ClientID:      graphSimClientID,
									ClientSecret:  configv1.ClientSecretSecretReference{Name: secretName},
									TokenEndpoint: tokenEndpoint,
									TLS: configv1.ExternalSourceTLS{
										CertificateAuthority: configv1.ExternalSourceCertificateAuthorityConfigMapReference{
											Name: keycloakCAConfigMapName(),
										},
									},
								},
							},
							URL: configv1.SourceURL{
								Hostname: hostname,
								// Graph-like per-subject resource path.
								PathExpression: "['admin', 'realms', 'master', 'users'] + [claims.sub] + ['groups']",
							},
							TLS: configv1.ExternalSourceTLS{
								CertificateAuthority: configv1.ExternalSourceCertificateAuthorityConfigMapReference{
									Name: keycloakCAConfigMapName(),
								},
							},
							Mappings: []configv1.SourcedClaimMapping{
								{
									Name: "groups_cc",
									// Both the Keycloak Admin API ([{"name":...}]) and
									// Graph ({"value":[{"id":...}]}) return arrays of
									// objects that are unwrapped the same way.
									Expression: fmt.Sprintf("response.body.map(g, g.name).filter(n, n.startsWith('%s')).join(',')", groupPrefix),
								},
							},
						},
					}
					provider.ClaimMappings.Groups = configv1.PrefixedClaimMapping{
						TokenClaimMapping: configv1.TokenClaimMapping{
							Expression: "claims.?groups_cc.orValue('').split(',').filter(g, size(g) > 0)",
						},
					}
				})

				o.Eventually(func(gomega o.Gomega) {
					groups, err := selfSubjectGroups(ctx, oc, keycloakCli, externalClaimsKeycloakClientID, user, userPassword)
					gomega.Expect(err).NotTo(o.HaveOccurred(), "client credentials: should authenticate with Graph-like sourced groups")
					gomega.Expect(groupsMatchingPrefix(groups, groupPrefix)).To(o.ConsistOf(expectedGroups),
						"client credentials: should source all groups from the Graph-like endpoint")
				}).WithTimeout(10 * time.Minute).WithPolling(15 * time.Second).Should(o.Succeed())
			})
		})
	})

	g.AfterAll(func() {
		err, modified := resetAuthentication(ctx, oc, originalAuth)
		o.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error reverting authentication to original state")

		// Only if we modified the Authentication resource during the reset should we wait for a rollout
		if modified {
			waitForRollout(ctx, oc)
		}

		err = removeResources(ctx, cleanups...)
		o.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error cleaning up keycloak resources")
	})
})

type removalFunc func(context.Context) error

func removeResources(ctx context.Context, removalFuncs ...removalFunc) error {
	errs := []error{}

	for _, removal := range removalFuncs {
		if removal == nil {
			continue
		}
		err := removal(ctx)
		errs = append(errs, err)
	}

	return errors.FilterOut(errors.NewAggregate(errs), apierrors.IsNotFound)
}

func configureOIDCAuthentication(ctx context.Context, client *exutil.CLI, keycloakNS, oidcClientSecret string, modifier func(*configv1.OIDCProvider)) (*configv1.Authentication, *configv1.Authentication, error) {
	// Ensure the issuer CA and console client secret exist before flipping Authentication
	// to OIDC. Otherwise console-operator can race on a missing keycloak-ca ConfigMap and
	// leave status.oidcClients Available=Unknown for a long time.
	if err := ensureOIDCConsolePrerequisites(ctx, client, oidcClientSecret); err != nil {
		return nil, nil, err
	}

	authConfig, err := client.AdminConfigClient().ConfigV1().Authentications().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("getting authentications.config.openshift.io/cluster: %w", err)
	}

	original := authConfig.DeepCopy()
	modified := authConfig.DeepCopy()

	oidcProvider, err := generateOIDCProvider(ctx, client, keycloakNS, oidcClientSecret)
	if err != nil {
		return nil, nil, fmt.Errorf("generating OIDC provider: %w", err)
	}

	if modifier != nil {
		modifier(oidcProvider)
	}

	modified.Spec.Type = configv1.AuthenticationTypeOIDC
	modified.Spec.WebhookTokenAuthenticator = nil
	modified.Spec.OIDCProviders = []configv1.OIDCProvider{*oidcProvider}

	modified, err = client.AdminConfigClient().ConfigV1().Authentications().Update(ctx, modified, metav1.UpdateOptions{})
	if err != nil {
		return nil, nil, err
	}

	return original, modified, nil
}

// ensureOIDCConfigured bootstraps OIDC authentication when the cluster is not already OIDC.
// Enabling OIDC may roll kube-apiserver; callers that hot-reload ExternalClaimsSources should
// record LatestAvailableRevision only after this returns.
func ensureOIDCConfigured(ctx context.Context, client *exutil.CLI, keycloakNS, oidcClientSecret string) {
	authConfig, err := client.AdminConfigClient().ConfigV1().Authentications().Get(ctx, "cluster", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "should get authentications.config.openshift.io/cluster")

	if authConfig.Spec.Type == configv1.AuthenticationTypeOIDC && len(authConfig.Spec.OIDCProviders) > 0 {
		return
	}

	_, _, err = configureOIDCAuthentication(ctx, client, keycloakNS, oidcClientSecret, nil)
	o.Expect(err).NotTo(o.HaveOccurred(), "should bootstrap OIDC authentication for hot-reload testing")
	waitForRollout(ctx, client)
	waitForHealthyOIDCClients(ctx, client)
}

// hotReloadExternalClaimsAndWait patches ExternalClaimsSources on the existing OIDC provider
// and waits for oauth-apiserver auth-config sync. Per product expectation, this must not
// advance kube-apiserver LatestAvailableRevision.
func hotReloadExternalClaimsAndWait(ctx context.Context, client *exutil.CLI, expectedRevision int32, mustContain, mustNotContain string, modifier func(*configv1.OIDCProvider)) {
	err := updateExistingOIDCProvider(ctx, client, modifier)
	o.Expect(err).NotTo(o.HaveOccurred(), "should update existing OIDC provider external claims configuration")
	waitForAuthConfigSync(ctx, client, mustContain, mustNotContain)
	expectKubeAPIServerRevisionUnchanged(ctx, client, expectedRevision, "after external claims hot reload")
	waitForHealthyOIDCClients(ctx, client)
}

// updateExistingOIDCProvider applies modifier to the first configured OIDC provider in place.
// Unlike configureOIDCAuthentication, this does not regenerate the whole provider from scratch.
// Retries on conflict: CAO/status writers can update Authentication between Get and Update.
func updateExistingOIDCProvider(ctx context.Context, client *exutil.CLI, modifier func(*configv1.OIDCProvider)) error {
	cli := client.AdminConfigClient().ConfigV1().Authentications()
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		authConfig, err := cli.Get(ctx, "cluster", metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("getting authentications.config.openshift.io/cluster: %w", err)
		}
		if authConfig.Spec.Type != configv1.AuthenticationTypeOIDC {
			return fmt.Errorf("expected Authentication type OIDC, got %q", authConfig.Spec.Type)
		}
		if len(authConfig.Spec.OIDCProviders) == 0 {
			return fmt.Errorf("expected at least one oidcProvider on Authentication/cluster")
		}

		provider := authConfig.Spec.OIDCProviders[0].DeepCopy()
		// Clear fields each phase fully replaces so stale UserValidationRules/sources do not linger.
		provider.ExternalClaimsSources = nil
		provider.UserValidationRules = nil
		if modifier != nil {
			modifier(provider)
		}
		authConfig.Spec.OIDCProviders[0] = *provider

		_, err = cli.Update(ctx, authConfig, metav1.UpdateOptions{})
		return err
	})
}

// waitForAuthConfigSync polls auth-config ConfigMaps until contents include mustContain
// and, when mustNotContain is non-empty, no longer include that substring.
func waitForAuthConfigSync(ctx context.Context, client *exutil.CLI, mustContain, mustNotContain string) {
	namespaces := []string{"openshift-config-managed", "openshift-oauth-apiserver"}
	o.Eventually(func(gomega o.Gomega) {
		foundInAny := false
		var lastErr error
		for _, ns := range namespaces {
			cm, err := client.AdminKubeClient().CoreV1().ConfigMaps(ns).Get(ctx, "auth-config", metav1.GetOptions{})
			if err != nil {
				if apierrors.IsNotFound(err) {
					lastErr = err
					continue
				}
				gomega.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("getting configmap %s/auth-config", ns))
			}
			content := authConfigMapContent(cm)
			gomega.Expect(content).To(o.ContainSubstring(mustContain),
				fmt.Sprintf("configmap %s/auth-config should contain %q after external claims update", ns, mustContain))
			if mustNotContain != "" {
				gomega.Expect(content).NotTo(o.ContainSubstring(mustNotContain),
					fmt.Sprintf("configmap %s/auth-config should not contain stale %q", ns, mustNotContain))
			}
			foundInAny = true
		}
		if !foundInAny {
			gomega.Expect(lastErr).NotTo(o.HaveOccurred(),
				"expected auth-config ConfigMap in openshift-config-managed or openshift-oauth-apiserver")
		}
	}).WithTimeout(5*time.Minute).WithPolling(5*time.Second).Should(o.Succeed(),
		fmt.Sprintf("auth-config should eventually contain %q", mustContain))
}

func authConfigMapContent(cm *corev1.ConfigMap) string {
	if cm == nil {
		return ""
	}
	if v, ok := cm.Data["auth-config.json"]; ok && v != "" {
		return v
	}
	var b strings.Builder
	for _, v := range cm.Data {
		b.WriteString(v)
	}
	return b.String()
}

func kubeAPIServerLatestRevision(ctx context.Context, client *exutil.CLI) int32 {
	kas, err := client.AdminOperatorClient().OperatorV1().KubeAPIServers().Get(ctx, "cluster", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "should get kubeapiservers.operator.openshift.io/cluster")
	return kas.Status.LatestAvailableRevision
}

func expectKubeAPIServerRevisionUnchanged(ctx context.Context, client *exutil.CLI, expected int32, when string) {
	kas, err := client.AdminOperatorClient().OperatorV1().KubeAPIServers().Get(ctx, "cluster", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "should get kubeapiservers.operator.openshift.io/cluster")
	o.Expect(kas.Status.LatestAvailableRevision).To(o.Equal(expected),
		fmt.Sprintf("ExternalClaimsSources hot reload must not create a new kube-apiserver revision %s (want %d)", when, expected))

	err = checkKubeAPIServerCondition(ctx, client.AdminOperatorClient().OperatorV1().KubeAPIServers(),
		condition.NodeInstallerProgressingConditionType, operatorv1.ConditionFalse)
	o.Expect(err).NotTo(o.HaveOccurred(),
		fmt.Sprintf("NodeInstallerProgressing should remain False (AllNodesAtLatestRevision) %s", when))
}

func keycloakHostname(ctx context.Context, client *exutil.CLI, keycloakNS string) string {
	idpUrl, err := admittedURLForRoute(ctx, client, keycloakResourceName, keycloakNS)
	o.Expect(err).NotTo(o.HaveOccurred(), "should get keycloak route URL")
	return strings.TrimPrefix(idpUrl, "https://")
}

func keycloakUserInfoClaimsSource(hostname string, mappings ...configv1.SourcedClaimMapping) configv1.ExternalClaimsSource {
	return configv1.ExternalClaimsSource{
		Authentication: configv1.ExternalSourceAuthentication{
			Type: configv1.ExternalSourceAuthenticationTypeRequestProvidedToken,
		},
		URL: configv1.SourceURL{
			Hostname:       hostname,
			PathExpression: "['realms', 'master', 'protocol', 'openid-connect', 'userinfo']",
		},
		TLS: configv1.ExternalSourceTLS{
			CertificateAuthority: configv1.ExternalSourceCertificateAuthorityConfigMapReference{
				Name: keycloakCAConfigMapName(),
			},
		},
		Mappings: mappings,
	}
}

// selfSubjectGroups authenticates via Keycloak and returns groups from SelfSubjectReview.
func selfSubjectGroups(ctx context.Context, client *exutil.CLI, keycloakCli *keycloakClient, clientID, user, password string) ([]string, error) {
	if err := keycloakCli.Authenticate(clientID, user, password); err != nil {
		return nil, err
	}

	copiedOC := *client
	tokenOC := copiedOC.WithToken(keycloakCli.AccessToken())
	ssr, err := tokenOC.KubeClient().AuthenticationV1().SelfSubjectReviews().Create(ctx, &authnv1.SelfSubjectReview{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-info", user),
		},
	}, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	return ssr.Status.UserInfo.Groups, nil
}

// envPositiveInt returns the positive integer configured via name, falling
// back to def when unset or invalid.
func envPositiveInt(name string, def int) int {
	if v := strings.TrimSpace(os.Getenv(name)); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}

// envPositiveInts returns the comma-separated positive integers configured via
// name, falling back to def when unset or invalid.
func envPositiveInts(name string, def []int) []int {
	v := strings.TrimSpace(os.Getenv(name))
	if v == "" {
		return def
	}

	parts := strings.Split(v, ",")
	out := make([]int, 0, len(parts))
	for _, part := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || n <= 0 {
			return def
		}
		out = append(out, n)
	}
	if len(out) == 0 {
		return def
	}
	return out
}

// indexedGroupNames returns count unique group names sharing prefix, e.g.
// indexedGroupNames("lg-abc12345", 2) -> ["lg-abc12345-00001", "lg-abc12345-00002"].
func indexedGroupNames(prefix string, count int) []string {
	names := make([]string, count)
	for i := range names {
		names[i] = fmt.Sprintf("%s-%05d", prefix, i+1)
	}
	return names
}

// groupsMatchingPrefix returns the subset of groups sharing prefix. Keycloak
// group mappers only emit the groups of the authenticating user, so for the
// users created by these tests the result is exactly their group set.
func groupsMatchingPrefix(groups []string, prefix string) []string {
	matched := []string{}
	for _, grp := range groups {
		if strings.HasPrefix(grp, prefix) {
			matched = append(matched, grp)
		}
	}
	return matched
}

func keycloakCAConfigMapName() string {
	return fmt.Sprintf("%s-ca", keycloakResourceName)
}

// ensureOIDCConsolePrerequisites waits for the issuer CA ConfigMap and console OIDC client
// secret in openshift-config. Console-operator reads both as soon as Authentication is OIDC.
func ensureOIDCConsolePrerequisites(ctx context.Context, client *exutil.CLI, oidcClientSecret string) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	caName := keycloakCAConfigMapName()
	return wait.PollUntilContextCancel(timeoutCtx, 2*time.Second, true, func(ctx context.Context) (bool, error) {
		cm, err := client.AdminKubeClient().CoreV1().ConfigMaps("openshift-config").Get(ctx, caName, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			return false, fmt.Errorf("getting configmap openshift-config/%s: %w", caName, err)
		}
		if len(cm.Data["ca-bundle.crt"]) == 0 {
			return false, nil
		}

		_, err = client.AdminKubeClient().CoreV1().Secrets("openshift-config").Get(ctx, oidcClientSecret, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			return false, fmt.Errorf("getting secret openshift-config/%s: %w", oidcClientSecret, err)
		}
		return true, nil
	})
}

func generateOIDCProvider(ctx context.Context, client *exutil.CLI, namespace, oidcClientSecret string) (*configv1.OIDCProvider, error) {
	idpName := "keycloak"
	audiences := []configv1.TokenAudience{
		"admin-cli",
	}
	usernameClaim := "email"
	groupsClaim := "groups"

	idpUrl, err := admittedURLForRoute(ctx, client, keycloakResourceName, namespace)
	if err != nil {
		return nil, fmt.Errorf("getting issuer URL: %w", err)
	}

	return &configv1.OIDCProvider{
		Name: idpName,
		Issuer: configv1.TokenIssuer{
			URL: fmt.Sprintf("%s/realms/master", idpUrl),
			CertificateAuthority: configv1.ConfigMapNameReference{
				Name: keycloakCAConfigMapName(),
			},
			Audiences: audiences,
		},
		ClaimMappings: configv1.TokenClaimMappings{
			Username: configv1.UsernameClaimMapping{
				Claim: usernameClaim,
			},
			Groups: configv1.PrefixedClaimMapping{
				TokenClaimMapping: configv1.TokenClaimMapping{
					Claim: groupsClaim,
				},
			},
		},
		// while this config is not required for the tests in this suite, if omitted
		// the console-operator will go Degraded; since we're currently running these
		// tests in clusters where the Console is installed, we provide this config
		// to avoid breaking cluster operator monitor tests
		OIDCClients: []configv1.OIDCClientConfig{
			{
				ComponentName:      "console",
				ComponentNamespace: "openshift-console",
				ClientID:           "openshift-console-oidc-client",
				ClientSecret: configv1.SecretNameReference{
					Name: oidcClientSecret,
				},
			},
			{
				ComponentName:      "cli",
				ComponentNamespace: "openshift-console",
				ClientID:           "openshift-cli-oidc-client",
				ClientSecret: configv1.SecretNameReference{
					Name: oidcClientSecret,
				},
			},
		},
	}, nil
}

func admittedURLForRoute(ctx context.Context, client *exutil.CLI, routeName, namespace string) (string, error) {
	var admittedURL string

	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	err := wait.PollUntilContextCancel(timeoutCtx, 1*time.Second, true, func(ctx context.Context) (bool, error) {
		route, err := client.AdminRouteClient().RouteV1().Routes(namespace).Get(ctx, routeName, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}

		for _, ingress := range route.Status.Ingress {
			for _, condition := range ingress.Conditions {
				if condition.Type == routev1.RouteAdmitted && condition.Status == corev1.ConditionTrue {
					admittedURL = ingress.Host
					return true, nil
				}
			}
		}

		return false, fmt.Errorf("no admitted ingress for route %q", route.Name)
	})
	return fmt.Sprintf("https://%s", admittedURL), err
}

func resetAuthentication(ctx context.Context, client *exutil.CLI, original *configv1.Authentication) (error, bool) {
	if original == nil {
		return nil, false
	}

	modified := false
	timeoutCtx, cancel := context.WithDeadline(ctx, time.Now().Add(5*time.Minute))
	defer cancel()
	cli := client.AdminConfigClient().ConfigV1().Authentications()
	err := wait.PollUntilContextCancel(timeoutCtx, 10*time.Second, true, func(ctx context.Context) (done bool, err error) {
		current, err := cli.Get(ctx, "cluster", metav1.GetOptions{})
		if err != nil {
			return false, fmt.Errorf("getting the current authentications.config.openshift.io/cluster: %w", err)
		}

		if equality.Semantic.DeepEqual(current.Spec, original.Spec) {
			return true, nil
		}

		current.Spec = original.Spec
		modified = true

		_, err = cli.Update(ctx, current, metav1.UpdateOptions{})
		if err != nil {
			// Only log the error so we continue to retry until the context has timed out
			g.GinkgoLogr.Error(err, "updating authentication resource")
			return false, nil
		}

		return true, nil
	})

	return err, modified
}

func waitForRollout(ctx context.Context, client *exutil.CLI) {
	kasCli := client.AdminOperatorClient().OperatorV1().KubeAPIServers()

	// First wait for KAS NodeInstallerProgressing condition to flip to "True".
	// This means that the KAS-O has successfully started being configured
	// with our auth resource changes.
	o.Eventually(func(gomega o.Gomega) {
		err := checkKubeAPIServerCondition(ctx, kasCli, condition.NodeInstallerProgressingConditionType, operatorv1.ConditionTrue)
		gomega.Expect(err).NotTo(o.HaveOccurred())
	}).WithTimeout(10*time.Minute).WithPolling(20*time.Second).Should(o.Succeed(), "should eventually begin rolling out a new revision")

	// waitTime is in minutes - set to 50 minute wait for cluster operators to settle
	// Usually, it doesn't take nearly an hour for cluster operators to settle
	// but due to the disruptive nature of how we are testing here means we _may_
	// encounter scenarios where the KAS is undergoing multiple revision rollouts
	// in succession. The worst case we've seen is 2 back-to-back revision rollouts
	// which lead to the cluster-authentication-operator being unavailable for ~35-45
	// minutes as it waits for the KAS to finish rolling out so it can begin
	// doing whatever configurations it needs to.
	err := operator.WaitForOperatorsToSettle(ctx, client.AdminConfigClient(), 50)
	o.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error waiting for the cluster operators to settle")
}

// checkKubeAPIServerCondition is a utility function to check that the KubeAPIServer
// resource on the cluster has a status condition type set with the expected
// condition status. If it does not, it returns an error. If it does, it returns <nil>.
func checkKubeAPIServerCondition(ctx context.Context, kasCli operatorv1client.KubeAPIServerInterface, conditionType string, conditionStatus operatorv1.ConditionStatus) error {
	kas, err := kasCli.Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("getting KAS: %w", err)
	}

	found := false
	nipCond := operatorv1.OperatorCondition{}
	for _, cond := range kas.Status.Conditions {
		if cond.Type == condition.NodeInstallerProgressingConditionType {
			found = true
			nipCond = cond
			break
		}
	}

	if !found {
		return fmt.Errorf("no condition %q found in KAS status conditions", conditionType)
	}

	if nipCond.Status != conditionStatus {
		return fmt.Errorf("condition %q expected to have status %q, but has status %q instead. Full condition: %v", conditionType, conditionStatus, nipCond.Status, nipCond)
	}

	return nil
}

func isDefaultPlatformOIDCClient(c configv1.OIDCClientStatus) bool {
	return c.ComponentNamespace == "openshift-console" && (c.ComponentName == "console" || c.ComponentName == "cli")
}

// waitForHealthyOIDCClients waits until platform OIDC clients are healthy.
// Prefer status.Available=True. If console-operator lags and leaves Available=Unknown
// even after successfully rolling OIDC into the console Deployment (seen in CI),
// accept console OIDC readiness as healthy as long as no client is Degraded=True.
func waitForHealthyOIDCClients(ctx context.Context, client *exutil.CLI) {
	o.Eventually(func(gomega o.Gomega) {
		authn, err := client.AdminConfigClient().ConfigV1().Authentications().Get(ctx, "cluster", metav1.GetOptions{})
		gomega.Expect(err).NotTo(o.HaveOccurred())

		statusHealthy := true
		var statusDetails []string
		for _, oidcClient := range authn.Status.OIDCClients {
			if !isDefaultPlatformOIDCClient(oidcClient) {
				continue
			}

			degraded := meta.FindStatusCondition(oidcClient.Conditions, "Degraded")
			if degraded != nil && degraded.Status == metav1.ConditionTrue {
				gomega.Expect(degraded.Status).NotTo(o.Equal(metav1.ConditionTrue),
					fmt.Sprintf("oidc client %s/%s is degraded: reason=%s message=%s",
						oidcClient.ComponentNamespace, oidcClient.ComponentName, degraded.Reason, degraded.Message))
			}

			available := meta.FindStatusCondition(oidcClient.Conditions, "Available")
			if available != nil && available.Status == metav1.ConditionTrue {
				continue
			}

			statusHealthy = false
			if available == nil {
				statusDetails = append(statusDetails, fmt.Sprintf("%s/%s: Available condition missing", oidcClient.ComponentNamespace, oidcClient.ComponentName))
				continue
			}
			progressing := meta.FindStatusCondition(oidcClient.Conditions, "Progressing")
			progressingDetail := "Progressing=<nil>"
			if progressing != nil {
				progressingDetail = fmt.Sprintf("Progressing=%s reason=%s message=%s", progressing.Status, progressing.Reason, progressing.Message)
			}
			statusDetails = append(statusDetails,
				fmt.Sprintf("%s/%s: Available=%s reason=%s message=%s; %s",
					oidcClient.ComponentNamespace, oidcClient.ComponentName,
					available.Status, available.Reason, available.Message, progressingDetail))
		}

		if statusHealthy {
			return
		}

		err = verifyConsoleOIDCConfigured(ctx, client)
		gomega.Expect(err).NotTo(o.HaveOccurred(),
			fmt.Sprintf("OIDC client status not Available yet (%s); console OIDC readiness check also failed", strings.Join(statusDetails, "; ")))
	}).WithTimeout(15*time.Minute).WithPolling(20*time.Second).Should(o.Succeed(), "should eventually have healthy OIDC client configurations")
}

// verifyConsoleOIDCConfigured confirms console-operator applied OIDC config and the
// console Deployment is available. Used when status.oidcClients Available lags.
func verifyConsoleOIDCConfigured(ctx context.Context, client *exutil.CLI) error {
	cm, err := client.AdminKubeClient().CoreV1().ConfigMaps("openshift-console").Get(ctx, "console-config", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("getting openshift-console/console-config: %w", err)
	}
	cfg, ok := cm.Data["console-config.yaml"]
	if !ok || !strings.Contains(cfg, "authType: oidc") {
		return fmt.Errorf("console-config does not have authType: oidc yet")
	}

	// OIDCSetupController syncs the issuer CA into the console namespace.
	if _, err := client.AdminKubeClient().CoreV1().ConfigMaps("openshift-console").Get(ctx, keycloakCAConfigMapName(), metav1.GetOptions{}); err != nil {
		return fmt.Errorf("issuer CA %q not synced to openshift-console: %w", keycloakCAConfigMapName(), err)
	}

	deploy, err := client.AdminKubeClient().AppsV1().Deployments("openshift-console").Get(ctx, "console", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("getting openshift-console/console deployment: %w", err)
	}
	if deploy.Status.UnavailableReplicas > 0 || deploy.Status.AvailableReplicas < 1 {
		return fmt.Errorf("console deployment not available: ready=%d updated=%d unavailable=%d replicas=%d",
			deploy.Status.ReadyReplicas, deploy.Status.UpdatedReplicas, deploy.Status.UnavailableReplicas, deploy.Status.Replicas)
	}
	if deploy.Status.UpdatedReplicas < deploy.Status.Replicas {
		return fmt.Errorf("console deployment not fully updated: updated=%d replicas=%d",
			deploy.Status.UpdatedReplicas, deploy.Status.Replicas)
	}

	return nil
}
