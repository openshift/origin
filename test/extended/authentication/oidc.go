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
	exutil "github.com/openshift/origin/test/extended/util"
	authnv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
			g.GinkgoLogr.Error(err,"updating authentication resource")
			return false, nil
		}

		return true, nil
	})

	return err, modified
}

func waitForRollout(ctx context.Context, client *exutil.CLI) {
	kasCli := client.AdminOperatorClient().OperatorV1().KubeAPIServers()

	// First wait for KAS to flip to progressing
	o.Eventually(func(gomega o.Gomega) {
		kas, err := kasCli.Get(ctx, "cluster", metav1.GetOptions{})
		gomega.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error fetching the KAS")

		found := false
		nipCond := operatorv1.OperatorCondition{}
		for _, cond := range kas.Status.Conditions {
			if cond.Type == condition.NodeInstallerProgressingConditionType {
				found = true
				nipCond = cond
				break
			}
		}

		gomega.Expect(found).To(o.BeTrue(), "should have found the NodeInstallerProgressing condition")
		gomega.Expect(nipCond.Status).To(o.Equal(operatorv1.ConditionTrue), "NodeInstallerProgressing condition should be True", nipCond)
	}).WithTimeout(10*time.Minute).WithPolling(20*time.Second).Should(o.Succeed(), "should eventually begin rolling out a new revision")

	// Then wait for it to flip back
	o.Eventually(func(gomega o.Gomega) {
		kas, err := kasCli.Get(ctx, "cluster", metav1.GetOptions{})
		gomega.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error fetching the KAS")

		found := false
		nipCond := operatorv1.OperatorCondition{}
		for _, cond := range kas.Status.Conditions {
			if cond.Type == condition.NodeInstallerProgressingConditionType {
				found = true
				nipCond = cond
				break
			}
		}

		gomega.Expect(found).To(o.BeTrue(), "should have found the NodeInstallerProgressing condition")
		gomega.Expect(nipCond.Status).To(o.Equal(operatorv1.ConditionFalse), "NodeInstallerProgressing condition should be False", nipCond)
	}).WithTimeout(30*time.Minute).WithPolling(30*time.Second).Should(o.Succeed(), "should eventually rollout out a new revision successfully")
}
