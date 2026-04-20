package authentication

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"
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

					// Set multiple ClaimValidationRules mixing RequiredClaim and CEL types
					provider.ClaimValidationRules = []configv1.TokenClaimValidationRule{
						{
							Type: configv1.TokenValidationRuleTypeRequiredClaim,
							RequiredClaim: &configv1.TokenRequiredClaim{
								Claim:         "aud",
								RequiredValue: "admin-cli",
							},
						},
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

			g.It("should authenticate successfully with custom discoveryURL, AND-logic userValidationRules, and mixed-type claimValidationRules", func() {
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
				testAPassed := false
				testBPassed := false
				var testAFailure, testBFailure string

				// Test A: userValidationRules false evaluation
				func() {
					defer func() {
						if r := recover(); r != nil {
							testAFailure = fmt.Sprintf("%v", r)
						}
					}()

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

					testAPassed = true
				}()

				// Test B: CEL claimValidationRules false evaluation
				func() {
					defer func() {
						if r := recover(); r != nil {
							testBFailure = fmt.Sprintf("%v", r)
						}
					}()

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

					testBPassed = true
				}()

				// Final assertion: both tests should pass
				failureMsg := fmt.Sprintf("userValidationRules false evaluation test failed: %t; reason: %s\nCEL claimValidationRules false evaluation test failed: %t; reason: %s",
					!testAPassed, testAFailure, !testBPassed, testBFailure)

				o.Expect(testAPassed && testBPassed).To(o.BeTrue(), failureMsg)
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
				testAPassed := false
				testBPassed := false
				var testAFailure, testBFailure string

				// Test A: username expression producing value failing userValidationRules
				func() {
					defer func() {
						if r := recover(); r != nil {
							testAFailure = fmt.Sprintf("%v", r)
						}
					}()

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

					testAPassed = true
				}()

				// Test B: groups expression producing value failing userValidationRules
				func() {
					defer func() {
						if r := recover(); r != nil {
							testBFailure = fmt.Sprintf("%v", r)
						}
					}()

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

					testBPassed = true
				}()

				// Final assertion: both tests should pass
				failureMsg := fmt.Sprintf("username expression test failed: %t; reason: %s\ngroups expression test failed: %t; reason: %s",
					!testAPassed, testAFailure, !testBPassed, testBFailure)

				o.Expect(testAPassed && testBPassed).To(o.BeTrue(), failureMsg)
			})
		})

		// TODO: this test relies on https://github.com/openshift/kubernetes/pull/2627
		/*
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
		*/
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

func generateOIDCProvider(ctx context.Context, client *exutil.CLI, namespace, oidcClientSecret string) (*configv1.OIDCProvider, error) {
	idpName := "keycloak"
	caBundle := "keycloak-ca"
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
				Name: caBundle,
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

func waitForHealthyOIDCClients(ctx context.Context, client *exutil.CLI) {
	o.Eventually(func(gomega o.Gomega) {
		authn, err := client.AdminConfigClient().ConfigV1().Authentications().Get(ctx, "cluster", metav1.GetOptions{})
		gomega.Expect(err).NotTo(o.HaveOccurred())

		for _, client := range authn.Status.OIDCClients {
			// ignore clients that aren't OpenShift default clients
			if client.ComponentNamespace != "openshift-console" && !(client.ComponentName == "console" || client.ComponentName == "cli") {
				continue
			}

			availableCondition := meta.FindStatusCondition(client.Conditions, "Available")
			gomega.Expect(availableCondition).NotTo(o.BeNil(), fmt.Sprintf("oidc client %s/%s should have an Available condition", client.ComponentNamespace, client.ComponentName))
			gomega.Expect(availableCondition.Status).To(o.Equal(metav1.ConditionTrue), fmt.Sprintf("oidc client %s/%s should be available but was not", client.ComponentNamespace, client.ComponentName), availableCondition)
		}
	}).WithTimeout(10*time.Minute).WithPolling(20*time.Second).Should(o.Succeed(), "should eventually have healthy OIDC client configurations")
}
