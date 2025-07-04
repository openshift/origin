package oidc

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"
	routev1 "github.com/openshift/api/route/v1"
	exutil "github.com/openshift/origin/test/extended/util"
	appsv1 "k8s.io/api/apps/v1"
	authnv1 "k8s.io/api/authentication/v1"
	authzv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/pod-security-admission/api"
	"k8s.io/utils/ptr"
)

type kubeObject interface {
	runtime.Object
	metav1.Object
}

var _ = g.Describe("[sig-auth][Serial][Slow][OCPFeatureGate:ExternalOIDC]", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLI("external-oidc")
	oc.KubeFramework().NamespacePodSecurityLevel = api.LevelPrivileged

	var cleanups []removalFunc
	var keycloakCli *keycloakClient
	var username string
	var password string
	var group string

	g.BeforeEach(func() {
		var err error
		ctx := context.TODO()
		cleanups, err = deployKeycloak(ctx, oc)
		o.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error deploying keycloak")

		kcURL, err := admittedURLForRoute(ctx, oc, keycloakResourceName)
		o.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error getting keycloak route URL")

		keycloakCli, err = keycloakClientFor(kcURL)
		o.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error creating a keycloak client")

		// First authenticate as the admin keyloak user so we can add new groups and users
		err = keycloakCli.Authenticate("admin-cli", keycloakAdminUsername, keycloakAdminPassword)
		o.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error authenticating as keycloak admin")

		o.Expect(keycloakCli.ConfigureClient("admin-cli")).NotTo(o.HaveOccurred(), "should not encounter an error configuring the admin-cli client")

		username = rand.String(8)
		password = rand.String(8)
		group = fmt.Sprintf("ocp-test-%s-group", rand.String(8))

		o.Expect(keycloakCli.CreateGroup(group)).To(o.Succeed(), "should be able to create a new keycloak group")
		o.Expect(keycloakCli.CreateUser(username, password, group)).To(o.Succeed(), "should be able to create a new keycloak user")
	})

	g.AfterEach(func() {
		ctx := context.TODO()
		err := removeResources(ctx, cleanups...)
		o.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error cleaning up keycloak resources")
	})

	g.Describe("Configuring an external OIDC provider", func() {
		var originalAuth *configv1.Authentication

		g.BeforeEach(func() {
			ctx := context.TODO()

			original, _, err := configureOIDCAuthentication(ctx, oc, nil)
			o.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error configuring OIDC authentication")
			originalAuth = original
		})

		g.AfterEach(func() {
			ctx := context.TODO()
			err := resetAuthentication(ctx, oc, originalAuth)
			o.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error reverting authentication to original state")
		})

		g.It("should configure kube-apiserver", func() {
			o.Eventually(func(gomega o.Gomega) {
				kas, err := oc.AdminOperatorClient().OperatorV1().KubeAPIServers().Get(context.TODO(), "cluster", metav1.GetOptions{})
				gomega.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error getting the kubeapiservers.operator.openshift.io/cluster")

				observedConfig := map[string]interface{}{}
				err = json.Unmarshal(kas.Spec.ObservedConfig.Raw, &observedConfig)
				gomega.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error unmarshalling the KAS observed configuration")

				apiServerArgs := observedConfig["apiServerArguments"].(map[string]interface{})

				gomega.Expect(apiServerArgs["authentication-token-webhook-config-file"]).To(o.BeNil(), "authentication-token-webhook-config-file argument should not be specified when OIDC authentication is configured")
				gomega.Expect(apiServerArgs["authentication-token-webhook-version"]).To(o.BeNil(), "authentication-token-webhook-version argument should not be specified when OIDC authentication is configured")
				gomega.Expect(apiServerArgs["authConfig"]).To(o.BeNil(), "authConfig argument should not be specified when OIDC authentication is configured")

				gomega.Expect(apiServerArgs["authentication-config"]).NotTo(o.BeNil(), "authentication-config argument should be specified when OIDC authentication is configured")
				gomega.Expect(apiServerArgs["authentication-config"].([]interface{})[0].(string)).To(o.Equal("/etc/kubernetes/static-pod-resources/configmaps/auth-config/auth-config.json"))
			}).WithTimeout(5 * time.Minute).WithPolling(30 * time.Second).Should(o.Succeed())
		})

		g.It("should remove the OpenShift OAuth stack", func() {
			g.Skip("functionality not yet implemented")
			o.Eventually(func(gomega o.Gomega) {
				_, err := oc.AdminKubeClient().AppsV1().Deployments("openshift-authentication").Get(context.TODO(), "oauth-openshift", metav1.GetOptions{})
				gomega.Expect(err).NotTo(o.BeNil(), "should not be able to get the integrated oauth stack")
				gomega.Expect(apierrors.IsNotFound(err)).To(o.BeTrue(), "integrated oauth stack should not be present when OIDC authentication is configured")
			}).WithTimeout(20 * time.Minute).WithPolling(30 * time.Second).Should(o.Succeed())
		})

		g.It("should not accept tokens provided by the OAuth server", func() {
			o.Eventually(func(gomega o.Gomega) {
				_, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).List(context.TODO(), metav1.ListOptions{})
				gomega.Expect(err).ShouldNot(o.BeNil(), "should not be able to list Pods using OAuth client token")
				gomega.Expect(apierrors.IsUnauthorized(err)).To(o.BeTrue(), "should receive an unauthorized error when trying to list Pods using OAuth client token")
			}).WithTimeout(20 * time.Minute).WithPolling(30 * time.Second).Should(o.Succeed())
		})

		g.It("should accept tokens issued by the external IdP", func() {
			// should always be able to create an SSAR for yourself
			o.Eventually(func(gomega o.Gomega) {
				// always re-authenticate to get a new token
				err := keycloakCli.Authenticate("admin-cli", username, password)
				gomega.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error authenticating as keycloak user")

				tokenOC := oc.WithToken(keycloakCli.AccessToken())

				_, err = tokenOC.KubeClient().AuthorizationV1().SelfSubjectAccessReviews().Create(context.TODO(), &authzv1.SelfSubjectAccessReview{
					Spec: authzv1.SelfSubjectAccessReviewSpec{
						ResourceAttributes: &authzv1.ResourceAttributes{
							Resource: "pods",
							Verb:     "get",
						},
					},
				}, metav1.CreateOptions{})
				gomega.Expect(err).NotTo(o.HaveOccurred(), "should be able to create a SelfSubjectAccessReview")
			}).WithTimeout(20 * time.Minute).WithPolling(30 * time.Second).Should(o.Succeed())
		})

		g.It("should accept authentication via a certificate-based kubeconfig (break-glass)", func() {
			_, err := oc.AdminKubeClient().CoreV1().Pods(oc.Namespace()).List(context.TODO(), metav1.ListOptions{})
			o.Expect(err).NotTo(o.HaveOccurred(), "should be able to list pods using certificate-based authentication")
		})

		g.It("should map cluster identities correctly", func() {
			// should always be able to create an SSR for yourself
			o.Eventually(func(gomega o.Gomega) {
				err := keycloakCli.Authenticate("admin-cli", username, password)
				gomega.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error authenticating as keycloak user")

				tokenOC := oc.WithToken(keycloakCli.AccessToken())
				ssr, err := tokenOC.KubeClient().AuthenticationV1().SelfSubjectReviews().Create(context.TODO(), &authnv1.SelfSubjectReview{
					ObjectMeta: metav1.ObjectMeta{
						Name: fmt.Sprintf("%s-info", username),
					},
				}, metav1.CreateOptions{})
				gomega.Expect(err).NotTo(o.HaveOccurred(), "should be able to create a SelfSubjectReview")

				gomega.Expect(ssr.Status.UserInfo.Username).To(o.Equal(fmt.Sprintf("%s@payload.openshift.io", username)))
				gomega.Expect(ssr.Status.UserInfo.Groups).To(o.ContainElement(group))
			}).WithTimeout(20 * time.Minute).WithPolling(30 * time.Second).Should(o.Succeed())
		})
	})

	g.Describe("Switching back from OIDC", func() {
		g.BeforeEach(func() {
			ctx := context.TODO()
			original, _, err := configureOIDCAuthentication(ctx, oc, nil)
			o.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error configuring OIDC authentication")

			// Wait until we can authenticate using the configured external IdP
			o.Eventually(func(gomega o.Gomega) {
				// always re-authenticate to get a new token
				err := keycloakCli.Authenticate("admin-cli", username, password)
				gomega.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error authenticating as keycloak user")

				copiedOC := *oc
				tokenOC := copiedOC.WithToken(keycloakCli.AccessToken())

				_, err = tokenOC.KubeClient().AuthorizationV1().SelfSubjectAccessReviews().Create(context.TODO(), &authzv1.SelfSubjectAccessReview{
					Spec: authzv1.SelfSubjectAccessReviewSpec{
						ResourceAttributes: &authzv1.ResourceAttributes{
							Resource: "pods",
							Verb:     "get",
						},
					},
				}, metav1.CreateOptions{})
				gomega.Expect(err).NotTo(o.HaveOccurred(), "should be able to create a SelfSubjectAccessReview")
			}).WithTimeout(20 * time.Minute).WithPolling(30 * time.Second).Should(o.Succeed())

			err = resetAuthentication(ctx, oc, original)
			o.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error reverting authentication to original state")
		})

		g.It("should rollout configuration on the kube-apiserver successfully", func() {
			o.Eventually(func(gomega o.Gomega) {
				kas, err := oc.AdminOperatorClient().OperatorV1().KubeAPIServers().Get(context.TODO(), "cluster", metav1.GetOptions{})
				gomega.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error getting the kubeapiservers.operator.openshift.io/cluster")

				observedConfig := map[string]interface{}{}
				err = json.Unmarshal(kas.Spec.ObservedConfig.Raw, &observedConfig)
				gomega.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error unmarshalling the KAS observed configuration")

				apiServerArgs := observedConfig["apiServerArguments"].(map[string]interface{})

				gomega.Expect(apiServerArgs["authentication-token-webhook-config-file"]).NotTo(o.BeNil(), "authentication-token-webhook-config-file argument should be specified when OIDC authentication is not configured")
				gomega.Expect(apiServerArgs["authentication-token-webhook-version"]).NotTo(o.BeNil(), "authentication-token-webhook-version argument should be specified when OIDC authentication is not configured")
				gomega.Expect(apiServerArgs["authConfig"]).NotTo(o.BeNil(), "authConfig argument should be specified when OIDC authentication is not configured")

				gomega.Expect(apiServerArgs["authentication-config"]).To(o.BeNil(), "authentication-config argument should not be specified when OIDC authentication is not configured")
			}).WithTimeout(5 * time.Minute).WithPolling(30 * time.Second).Should(o.Succeed())
		})

		g.It("should rollout the OpenShift OAuth stack", func() {
			o.Eventually(func(gomega o.Gomega) {
				_, err := oc.AdminKubeClient().AppsV1().Deployments("openshift-authentication").Get(context.TODO(), "oauth-openshift", metav1.GetOptions{})
				gomega.Expect(err).Should(o.BeNil(), "should be able to get the integrated oauth stack")
			}).WithTimeout(20 * time.Minute).WithPolling(30 * time.Second).Should(o.Succeed())
		})

		g.It("should not accept tokens provided by an external IdP", func() {
			o.Eventually(func(gomega o.Gomega) {
				// always re-authenticate to get a new token
				err := keycloakCli.Authenticate("admin-cli", username, password)
				gomega.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error authenticating as keycloak user")

				copiedOC := *oc
				tokenOC := copiedOC.WithToken(keycloakCli.AccessToken())

				_, err = tokenOC.KubeClient().AuthorizationV1().SelfSubjectAccessReviews().Create(context.TODO(), &authzv1.SelfSubjectAccessReview{
					Spec: authzv1.SelfSubjectAccessReviewSpec{
						ResourceAttributes: &authzv1.ResourceAttributes{
							Resource: "pods",
							Verb:     "get",
						},
					},
				}, metav1.CreateOptions{})
				gomega.Expect(err).To(o.HaveOccurred(), "should not be able to create a SelfSubjectAccessReview")
				gomega.Expect(apierrors.IsUnauthorized(err)).To(o.BeTrue(), "external IdP token should be unauthorized")
			}).WithTimeout(20 * time.Minute).WithPolling(30 * time.Second).Should(o.Succeed())
		})

		g.It("should accept tokens provided by the OpenShift OAuth server", func() {
			o.Eventually(func(gomega o.Gomega) {
				_, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).List(context.TODO(), metav1.ListOptions{})
				gomega.Expect(err).Should(o.BeNil(), "should be able to list Pods using OAuth client token")
			}).WithTimeout(20 * time.Minute).WithPolling(30 * time.Second).Should(o.Succeed())
		})
	})
})

var _ = g.Describe("[sig-auth][Serial][Slow][OCPFeatureGate:ExternalOIDCWithUIDAndExtraClaimMappings]", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLI("external-oidc-with-uid-and-extra")
	oc.KubeFramework().NamespacePodSecurityLevel = api.LevelPrivileged

	var cleanups []removalFunc
	var keycloakCli *keycloakClient
	var username string
	var password string
	var group string

	g.BeforeEach(func() {
		var err error
		ctx := context.TODO()
		cleanups, err = deployKeycloak(ctx, oc)
		o.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error deploying keycloak")

		kcURL, err := admittedURLForRoute(ctx, oc, keycloakResourceName)
		o.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error getting keycloak route URL")

		keycloakCli, err = keycloakClientFor(kcURL)
		o.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error creating a keycloak client")

		// First authenticate as the admin keyloak user so we can add new groups and users
		err = keycloakCli.Authenticate("admin-cli", keycloakAdminUsername, keycloakAdminPassword)
		o.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error authenticating as keycloak admin")

		o.Expect(keycloakCli.ConfigureClient("admin-cli")).NotTo(o.HaveOccurred(), "should not encounter an error configuring the admin-cli client")

		username = rand.String(8)
		password = rand.String(8)
		group = fmt.Sprintf("ocp-test-%s-group", rand.String(8))

		o.Expect(keycloakCli.CreateGroup(group)).To(o.Succeed(), "should be able to create a new keycloak group")
		o.Expect(keycloakCli.CreateUser(username, password, group)).To(o.Succeed(), "should be able to create a new keycloak user")
	})

	g.AfterEach(func() {
		ctx := context.TODO()
		err := removeResources(ctx, cleanups...)
		o.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error cleaning up keycloak resources")
	})

	g.Describe("Configuring an external OIDC provider", func() {
		g.Describe("Without specified UID or Extra claim mappings", func() {
			var originalAuth *configv1.Authentication
			g.BeforeEach(func() {
				ctx := context.TODO()

				original, _, err := configureOIDCAuthentication(ctx, oc, nil)
				o.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error configuring OIDC authentication")
				originalAuth = original
			})

			g.AfterEach(func() {
				ctx := context.TODO()
				err := resetAuthentication(ctx, oc, originalAuth)
				o.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error reverting authentication to original state")
			})

			g.It("should default UID to the 'sub' claim in the access token from the IdP", func() {
				g.Fail("not implemented")
			})
		})

		g.Describe("With valid specified UID or Extra claim mappings", func() {
			var originalAuth *configv1.Authentication
			g.BeforeEach(func() {
				ctx := context.TODO()

				original, _, err := configureOIDCAuthentication(ctx, oc, func(o *configv1.OIDCProvider) {
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
				originalAuth = original
			})

			g.AfterEach(func() {
				ctx := context.TODO()
				err := resetAuthentication(ctx, oc, originalAuth)
				o.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error reverting authentication to original state")
			})

			g.It("should map the UID of the cluster identity correctly", func() {
				// should always be able to create an SSR for yourself
				o.Eventually(func(gomega o.Gomega) {
					err := keycloakCli.Authenticate("admin-cli", username, password)
					gomega.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error authenticating as keycloak user")

					tokenOC := oc.WithToken(keycloakCli.AccessToken())
					ssr, err := tokenOC.KubeClient().AuthenticationV1().SelfSubjectReviews().Create(context.TODO(), &authnv1.SelfSubjectReview{
						ObjectMeta: metav1.ObjectMeta{
							Name: fmt.Sprintf("%s-info", username),
						},
					}, metav1.CreateOptions{})
					gomega.Expect(err).NotTo(o.HaveOccurred(), "should be able to create a SelfSubjectReview")

					gomega.Expect(ssr.Status.UserInfo.UID).To(o.Equal(strings.ToUpper(username)))
				}).WithTimeout(20 * time.Minute).WithPolling(30 * time.Second).Should(o.Succeed())
			})

			g.It("should map the Extra of the cluster identity correctly", func() {
				// should always be able to create an SSR for yourself
				o.Eventually(func(gomega o.Gomega) {
					err := keycloakCli.Authenticate("admin-cli", username, password)
					gomega.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error authenticating as keycloak user")

					tokenOC := oc.WithToken(keycloakCli.AccessToken())
					ssr, err := tokenOC.KubeClient().AuthenticationV1().SelfSubjectReviews().Create(context.TODO(), &authnv1.SelfSubjectReview{
						ObjectMeta: metav1.ObjectMeta{
							Name: fmt.Sprintf("%s-info", username),
						},
					}, metav1.CreateOptions{})
					gomega.Expect(err).NotTo(o.HaveOccurred(), "should be able to create a SelfSubjectReview")

					gomega.Expect(ssr.Status.UserInfo.Extra).To(o.HaveKeyWithValue("payload/test", []string{fmt.Sprintf("%s@payload.openshift.ioextra", username)}))
				}).WithTimeout(20 * time.Minute).WithPolling(30 * time.Second).Should(o.Succeed())
			})
		})

		g.Describe("With invalid specified UID or Extra claim mappings", func() {
			ctx := context.TODO()

			g.It("should reject admission when UID claim expression is not compilable CEL", func() {
				g.Skip("functionality not yet implemented")
				_, _, err := configureOIDCAuthentication(ctx, oc, func(o *configv1.OIDCProvider) {
					o.ClaimMappings.UID = &configv1.TokenClaimOrExpressionMapping{
						Expression: "!@&*#^",
					}
				})
				o.Expect(err).To(o.HaveOccurred(), "should encounter an error configuring OIDC authentication")
			})

			g.It("should reject admission when Extra claim expression is not compilable CEL", func() {
				g.Skip("functionality not yet implemented")
				_, _, err := configureOIDCAuthentication(ctx, oc, func(o *configv1.OIDCProvider) {
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

const (
	keycloakResourceName          = "keycloak"
	keycloakServingCertSecretName = "keycloak-serving-cert"
	keycloakLabelKey              = "app"
	keycloakLabelValue            = "keycloak"
	keycloakHTTPSPort             = 8443

	// TODO: should this be an openshift image?
	keycloakImage          = "quay.io/keycloak/keycloak:25.0"
	keycloakAdminUsername  = "admin"
	keycloakAdminPassword  = "password"
	keycloakCertVolumeName = "certkeypair"
	keycloakCertMountPath  = "/etc/x509/https"
	keycloakCertFile       = "tls.crt"
	keycloakKeyFile        = "tls.key"
)

func deployKeycloak(ctx context.Context, client *exutil.CLI) ([]removalFunc, error) {
	cleanups := []removalFunc{}

	cleanup, err := createKeycloakServiceAccount(ctx, client)
	if err != nil {
		return cleanups, fmt.Errorf("creating serviceaccount for keycloak: %w", err)
	}
	cleanups = append(cleanups, cleanup)

	service, cleanup, err := createKeycloakService(ctx, client)
	if err != nil {
		return cleanups, fmt.Errorf("creating service for keycloak: %w", err)
	}
	cleanups = append(cleanups, cleanup)

	cleanup, err = createKeycloakDeployment(ctx, client)
	if err != nil {
		return cleanups, fmt.Errorf("creating deployment for keycloak: %w", err)
	}
	cleanups = append(cleanups, cleanup)

	cleanup, err = createKeycloakRoute(ctx, service, client)
	if err != nil {
		return cleanups, fmt.Errorf("creating route for keycloak: %w", err)
	}
	cleanups = append(cleanups, cleanup)

	cleanup, err = createKeycloakCAConfigMap(ctx, client)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return cleanups, fmt.Errorf("creating CA configmap for keycloak: %w", err)
	}
	cleanups = append(cleanups, cleanup)

	return cleanups, waitForKeycloakAvailable(ctx, client)
}

func createKeycloakServiceAccount(ctx context.Context, client *exutil.CLI) (removalFunc, error) {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name: keycloakResourceName,
		},
	}
	sa.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("ServiceAccount"))

	_, err := client.AdminKubeClient().CoreV1().ServiceAccounts(client.Namespace()).Create(ctx, sa, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("creating serviceaccount: %w", err)
	}

	return func(ctx context.Context) error {
		return client.AdminKubeClient().CoreV1().ServiceAccounts(client.Namespace()).Delete(ctx, sa.Name, metav1.DeleteOptions{})
	}, nil
}

func createKeycloakService(ctx context.Context, client *exutil.CLI) (*corev1.Service, removalFunc, error) {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: keycloakResourceName,
			Annotations: map[string]string{
				"service.beta.openshift.io/serving-cert-secret-name": keycloakServingCertSecretName,
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: keycloakLabels(),
			Ports: []corev1.ServicePort{
				{
					Name: "https",
					Port: keycloakHTTPSPort,
				},
			},
		},
	}
	service.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Service"))

	_, err := client.AdminKubeClient().CoreV1().Services(client.Namespace()).Create(ctx, service, metav1.CreateOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("creating service: %w", err)
	}

	return service, func(ctx context.Context) error {
		return client.AdminKubeClient().CoreV1().Services(client.Namespace()).Delete(ctx, service.Name, metav1.DeleteOptions{})
	}, nil
}

func createKeycloakCAConfigMap(ctx context.Context, client *exutil.CLI) (removalFunc, error) {
	defaultIngressCACM, err := client.AdminKubeClient().CoreV1().ConfigMaps("openshift-config-managed").Get(ctx, "default-ingress-cert", metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting configmap openshift-config-managed/default-ingress-cert: %w", err)
	}

	data := defaultIngressCACM.Data["ca-bundle.crt"]

	keycloakCACM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-ca", keycloakResourceName),
		},
		Data: map[string]string{
			"ca-bundle.crt": data,
		},
	}
	keycloakCACM.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("ConfigMap"))

	_, err = client.AdminKubeClient().CoreV1().ConfigMaps("openshift-config").Create(ctx, keycloakCACM, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("creating configmap: %w", err)
	}

	return func(ctx context.Context) error {
		return client.AdminKubeClient().CoreV1().ConfigMaps("openshift-config").Delete(ctx, keycloakCACM.Name, metav1.DeleteOptions{})
	}, nil
}

func createKeycloakDeployment(ctx context.Context, client *exutil.CLI) (removalFunc, error) {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:   keycloakResourceName,
			Labels: keycloakLabels(),
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: keycloakLabels(),
			},
			Replicas: ptr.To(int32(1)),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:   keycloakResourceName,
					Labels: keycloakLabels(),
				},
				Spec: corev1.PodSpec{
					Containers: keycloakContainers(),
					Volumes:    keycloakVolumes(),
				},
			},
		},
	}
	deployment.SetGroupVersionKind(appsv1.SchemeGroupVersion.WithKind("Deployment"))

	_, err := client.AdminKubeClient().AppsV1().Deployments(client.Namespace()).Create(ctx, deployment, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("creating deployment: %w", err)
	}

	return func(ctx context.Context) error {
		return client.AdminKubeClient().AppsV1().Deployments(client.Namespace()).Delete(ctx, deployment.Name, metav1.DeleteOptions{})
	}, nil
}

func keycloakLabels() map[string]string {
	return map[string]string{
		keycloakLabelKey: keycloakLabelValue,
	}
}

func keycloakReadinessProbe() *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path:   "/health/ready",
				Port:   intstr.FromInt(9000),
				Scheme: corev1.URISchemeHTTPS,
			},
		},
		InitialDelaySeconds: 10,
	}
}

func keycloakLivenessProbe() *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path:   "/health/live",
				Port:   intstr.FromInt(9000),
				Scheme: corev1.URISchemeHTTPS,
			},
		},
		InitialDelaySeconds: 10,
	}
}

func keycloakEnvVars() []corev1.EnvVar {
	return []corev1.EnvVar{
		{
			Name:  "KEYCLOAK_ADMIN",
			Value: keycloakAdminUsername,
		},
		{
			Name:  "KEYCLOAK_ADMIN_PASSWORD",
			Value: keycloakAdminPassword,
		},
		{
			Name:  "KC_HEALTH_ENABLED",
			Value: "true",
		},
		{
			Name:  "KC_HOSTNAME_STRICT",
			Value: "false",
		},
		{
			Name:  "KC_PROXY",
			Value: "reencrypt",
		},
		{
			Name:  "KC_HTTPS_CERTIFICATE_FILE",
			Value: path.Join(keycloakCertMountPath, keycloakCertFile),
		},
		{
			Name:  "KC_HTTPS_CERTIFICATE_KEY_FILE",
			Value: path.Join(keycloakCertMountPath, keycloakKeyFile),
		},
	}
}

func keycloakVolumes() []corev1.Volume {
	return []corev1.Volume{
		{
			Name: keycloakCertVolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: keycloakServingCertSecretName,
				},
			},
		},
	}
}

func keycloakVolumeMounts() []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{
			Name:      keycloakCertVolumeName,
			MountPath: keycloakCertMountPath,
			ReadOnly:  true,
		},
	}
}

func keycloakContainers() []corev1.Container {
	return []corev1.Container{
		{
			Name:         "keycloak",
			Image:        keycloakImage,
			Env:          keycloakEnvVars(),
			VolumeMounts: keycloakVolumeMounts(),
			Ports: []corev1.ContainerPort{
				{
					ContainerPort: keycloakHTTPSPort,
				},
			},
			LivenessProbe:  keycloakLivenessProbe(),
			ReadinessProbe: keycloakReadinessProbe(),
			Command: []string{
				"/opt/keycloak/bin/kc.sh",
				"start-dev",
			},
		},
	}
}

func createKeycloakRoute(ctx context.Context, service *corev1.Service, client *exutil.CLI) (removalFunc, error) {
	route := &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name: keycloakResourceName,
		},
		Spec: routev1.RouteSpec{
			TLS: &routev1.TLSConfig{
				Termination:                   routev1.TLSTerminationReencrypt,
				InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
			},
			To: routev1.RouteTargetReference{
				Kind: "Service",
				Name: service.Name,
			},
			Port: &routev1.RoutePort{
				TargetPort: intstr.FromString("https"),
			},
		},
	}
	route.SetGroupVersionKind(routev1.SchemeGroupVersion.WithKind("Route"))

	_, err := client.RouteClient().RouteV1().Routes(client.Namespace()).Create(ctx, route, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("creating route: %w", err)
	}

	return func(ctx context.Context) error {
		return client.AdminRouteClient().RouteV1().Routes(client.Namespace()).Delete(ctx, route.Name, metav1.DeleteOptions{})
	}, nil
}

type removalFunc func(context.Context) error

func removeResources(ctx context.Context, removalFuncs ...removalFunc) error {
	errs := []error{}

	for _, removal := range removalFuncs {
		if removal == nil {
			continue
		}
		err := removal(ctx)
		if err != nil && !apierrors.IsNotFound(err) {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

func configureOIDCAuthentication(ctx context.Context, client *exutil.CLI, modifier func(*configv1.OIDCProvider)) (*configv1.Authentication, *configv1.Authentication, error) {
	authConfig, err := client.AdminConfigClient().ConfigV1().Authentications().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("getting authentications.config.openshift.io/cluster: %w", err)
	}

	original := authConfig.DeepCopy()
	modified := authConfig.DeepCopy()

	oidcProvider, err := generateOIDCProvider(ctx, client)
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

func generateOIDCProvider(ctx context.Context, client *exutil.CLI) (*configv1.OIDCProvider, error) {
	idpName := "keycloak"
	caBundle := "keycloak-ca"
	audiences := []configv1.TokenAudience{
		"admin-cli",
	}
	usernameClaim := "email"
	groupsClaim := "groups"

	idpUrl, err := admittedURLForRoute(ctx, client, keycloakResourceName)
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
				TokenClaimMapping: configv1.TokenClaimMapping{
					Claim: usernameClaim,
				},
			},
			Groups: configv1.PrefixedClaimMapping{
				TokenClaimMapping: configv1.TokenClaimMapping{
					Claim: groupsClaim,
				},
			},
		},
	}, nil
}

func admittedURLForRoute(ctx context.Context, client *exutil.CLI, routeName string) (string, error) {
	var admittedURL string

	// TODO: should probably create a new context that has a timeout to pass into this
	err := wait.PollUntilContextCancel(ctx, 1*time.Second, true, func(ctx context.Context) (bool, error) {
		route, err := client.RouteClient().RouteV1().Routes(client.Namespace()).Get(ctx, routeName, metav1.GetOptions{})
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

type keycloakClient struct {
	realm    string
	client   *http.Client
	adminURL *url.URL

	accessToken string
	idToken     string
}

func keycloakClientFor(keycloakURL string) (*keycloakClient, error) {
	baseURL, err := url.Parse(keycloakURL)
	if err != nil {
		return nil, fmt.Errorf("parsing url: %w", err)
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	return &keycloakClient{
		realm: "master",
		client: &http.Client{
			Transport: transport,
		},
		adminURL: baseURL.JoinPath("admin", "realms", "master"),
	}, nil
}

func (kc *keycloakClient) CreateGroup(name string) error {
	groupURL := kc.adminURL.JoinPath("groups")

	group := map[string]interface{}{
		"name": name,
	}

	groupBytes, err := json.Marshal(group)
	if err != nil {
		return fmt.Errorf("marshalling group configuration %v", group)
	}

	resp, err := kc.DoRequest(http.MethodPost, groupURL.String(), runtime.ContentTypeJSON, true, bytes.NewBuffer(groupBytes))
	if err != nil {
		return fmt.Errorf("sending POST request to %q to create group %s", groupURL.String(), name)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed creating group %q: %s - %s", name, resp.Status, respBytes)
	}

	return nil
}

func (kc *keycloakClient) CreateUser(username, password string, groups ...string) error {
	userURL := kc.adminURL.JoinPath("users")

	user := map[string]interface{}{
		"username":      username,
		"email":         fmt.Sprintf("%s@payload.openshift.io", username),
		"enabled":       true,
		"emailVerified": true,
		"groups":        groups,
		"credentials": []map[string]interface{}{
			{
				"temporary": false,
				"type":      "password",
				"value":     password,
			},
		},
	}

	userBytes, err := json.Marshal(user)
	if err != nil {
		return fmt.Errorf("marshalling user configuration %v", user)
	}

	resp, err := kc.DoRequest(http.MethodPost, userURL.String(), runtime.ContentTypeJSON, true, bytes.NewBuffer(userBytes))
	if err != nil {
		return fmt.Errorf("sending POST request to %q to create user %v", userURL.String(), user)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed creating user %v: %s - %s", user, resp.Status, respBytes)
	}

	return nil
}

func (kc *keycloakClient) Authenticate(clientID, username, password string) error {
	data := url.Values{
		"username":   []string{username},
		"password":   []string{password},
		"grant_type": []string{"password"},
		"client_id":  []string{clientID},
		"scope":      []string{"openid"},
	}

	tokenURL := *kc.adminURL
	tokenURL.Path = fmt.Sprintf("/realms/%s/protocol/openid-connect/token", kc.realm)

	resp, err := kc.DoRequest(http.MethodPost, tokenURL.String(), "application/x-www-form-urlencoded", false, bytes.NewBuffer([]byte(data.Encode())))
	if err != nil {
		return fmt.Errorf("authenticating as user %q: %w", username, err)
	}
	defer resp.Body.Close()

	respBody := map[string]interface{}{}
	respBodyData, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response data: %w", err)
	}

	err = json.Unmarshal(respBodyData, &respBody)
	if err != nil {
		return fmt.Errorf("unmarshalling response body %s: %w", respBodyData, err)
	}

	accessTokenData, ok := respBody["access_token"]
	if !ok {
		return errors.New("unable to extract access token from the response body: access_token field is missing")
	}

	accessToken, ok := accessTokenData.(string)
	if !ok {
		return fmt.Errorf("expected accessToken to be of type string but was %T", accessTokenData)
	}
	kc.accessToken = accessToken

	idTokenData, ok := respBody["id_token"]
	if !ok {
		return errors.New("unable to extract id token from the response body: id_token field is missing")
	}

	idToken, ok := idTokenData.(string)
	if !ok {
		return fmt.Errorf("expected idToken to be of type string but was %T", idTokenData)
	}
	kc.idToken = idToken

	return nil
}

func (kc *keycloakClient) DoRequest(method, url, contentType string, authenticated bool, body io.Reader) (*http.Response, error) {
	if len(kc.accessToken) == 0 && authenticated {
		panic("must authenticate before calling keycloakClient.DoRequest")
	}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", kc.accessToken))
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Accept", runtime.ContentTypeJSON)

	return kc.client.Do(req)
}

func (kc *keycloakClient) AccessToken() string {
	return kc.accessToken
}

func (kc *keycloakClient) IdToken() string {
	return kc.idToken
}

func (kc *keycloakClient) ConfigureClient(clientId string) error {
	client, err := kc.GetClientByClientID(clientId)
	if err != nil {
		return fmt.Errorf("getting client %q: %w", clientId, err)
	}

	id, ok := client["id"]
	if !ok {
		return fmt.Errorf("client %q doesn't have 'id'", clientId)
	}

	idStr, ok := id.(string)
	if !ok {
		return fmt.Errorf("client %q 'id' is not of type string: %T", clientId, id)
	}

	if err := kc.CreateClientGroupMapper(idStr, "test-groups-mapper", "groups"); err != nil {
		return fmt.Errorf("creating group mapper for client %q: %w", clientId, err)
	}

	if err := kc.CreateClientAudienceMapper(idStr, "test-aud-mapper"); err != nil {
		return fmt.Errorf("creating audience mapper for client %q: %w", clientId, err)
	}

	return nil
}

func (kc *keycloakClient) CreateClientGroupMapper(clientId, name, claim string) error {
	mappersURL := *kc.adminURL
	mappersURL.Path += fmt.Sprintf("/clients/%s/protocol-mappers/models", clientId)

	mapper := map[string]interface{}{
		"name":           name,
		"protocol":       "openid-connect",
		"protocolMapper": "oidc-group-membership-mapper", // protocol-mapper type provided by Keycloak
		"config": map[string]string{
			"full.path":            "false",
			"id.token.claim":       "true",
			"access.token.claim":   "true",
			"userinfo.token.claim": "true",
			"claim.name":           claim,
		},
	}

	mapperBytes, err := json.Marshal(mapper)
	if err != nil {
		return err
	}

	// Keycloak does not return the object on successful create so there's no need to attempt to retrieve it from the response
	resp, err := kc.DoRequest(http.MethodPost, mappersURL.String(), runtime.ContentTypeJSON, true, bytes.NewBuffer(mapperBytes))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed creating mapper %q: %s %s", name, resp.Status, respBytes)
	}

	return nil
}

func (kc *keycloakClient) CreateClientAudienceMapper(clientId, name string) error {
	mappersURL := *kc.adminURL
	mappersURL.Path += fmt.Sprintf("/clients/%s/protocol-mappers/models", clientId)

	mapper := map[string]interface{}{
		"name":           name,
		"protocol":       "openid-connect",
		"protocolMapper": "oidc-audience-mapper", // protocol-mapper type provided by Keycloak
		"config": map[string]string{
			"id.token.claim":            "false",
			"access.token.claim":        "true",
			"introspection.token.claim": "true",
			"included.client.audience":  "admin-cli",
			"included.custom.audience":  "",
			"lightweight.claim":         "false",
		},
	}

	mapperBytes, err := json.Marshal(mapper)
	if err != nil {
		return err
	}

	// Keycloak does not return the object on successful create so there's no need to attempt to retrieve it from the response
	resp, err := kc.DoRequest(http.MethodPost, mappersURL.String(), runtime.ContentTypeJSON, true, bytes.NewBuffer(mapperBytes))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed creating mapper %q: %s %s", name, resp.Status, respBytes)
	}

	return nil
}

// ListClients retrieves all clients
func (kc *keycloakClient) ListClients() ([]map[string]interface{}, error) {
	clientsURL := *kc.adminURL
	clientsURL.Path += "/clients"

	resp, err := kc.DoRequest(http.MethodGet, clientsURL.String(), runtime.ContentTypeJSON, true, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("listing clients failed: %s: %s", resp.Status, respBytes)
	}

	clients := []map[string]interface{}{}
	err = json.Unmarshal(respBytes, &clients)

	return clients, err
}

func (kc *keycloakClient) GetClientByClientID(clientID string) (map[string]interface{}, error) {
	clients, err := kc.ListClients()
	if err != nil {
		return nil, err
	}

	for _, c := range clients {
		if c["clientId"].(string) == clientID {
			return c, nil
		}
	}

	return nil, fmt.Errorf("client with clientID %q not found", clientID)
}

func resetAuthentication(ctx context.Context, client *exutil.CLI, original *configv1.Authentication) error {
	if original == nil {
		return nil
	}

	current, err := client.AdminConfigClient().ConfigV1().Authentications().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("getting the current authentications.config.openshift.io/cluster: %w", err)
	}

	current.Spec = original.Spec

	_, err = client.AdminConfigClient().ConfigV1().Authentications().Update(ctx, current, metav1.UpdateOptions{})
	return err
}

func waitForKeycloakAvailable(ctx context.Context, client *exutil.CLI) error {
	timeoutCtx, cancel := context.WithDeadline(ctx, time.Now().Add(2*time.Minute))
	defer cancel()
	err := wait.PollUntilContextCancel(timeoutCtx, 10*time.Second, true, func(ctx context.Context) (done bool, err error) {
		deploy, err := client.KubeClient().AppsV1().Deployments(client.Namespace()).Get(ctx, keycloakResourceName, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}

		for _, condition := range deploy.Status.Conditions {
			if condition.Type == appsv1.DeploymentAvailable && condition.Status == corev1.ConditionTrue {
				return true, nil
			}
		}

		return false, nil
	})

	return err
}
