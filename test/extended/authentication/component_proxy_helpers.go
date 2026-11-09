package authentication

import (
	"context"
	"fmt"
	"reflect"
	"time"

	g "github.com/onsi/ginkgo/v2"
	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"

	"github.com/openshift/origin/test/extended/authentication/auth_proxy"
	exutil "github.com/openshift/origin/test/extended/util"
)

const componentProxyCAConfigMapName = "v4-0-config-system-auth-proxy-ca"

// saveAndRestoreAuthState snapshots the proxy and identity providers changed by
// these tests, returning a cleanup function that restores them without replacing
// operator-owned configuration. If either changed, it waits for stabilization.
func saveAndRestoreAuthState(ctx context.Context, oc *exutil.CLI) (removalFunc, error) {
	operatorClient := oc.AdminOperatorClient()
	oauthClient := oc.AdminConfigClient().ConfigV1().OAuths()

	auth, err := operatorClient.OperatorV1().Authentications().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting authentication/cluster: %w", err)
	}
	originalProxy := auth.Spec.DeepCopy().Proxy

	oauth, err := oauthClient.Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting oauth/cluster: %w", err)
	}
	originalIdentityProviders := oauth.Spec.DeepCopy().IdentityProviders

	return func(ctx context.Context) error {
		var changed bool

		g.GinkgoWriter.Println("cleanup: restoring authentication/cluster")
		if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			fresh, err := operatorClient.OperatorV1().Authentications().Get(ctx, "cluster", metav1.GetOptions{})
			if err != nil {
				return err
			}
			if reflect.DeepEqual(fresh.Spec.Proxy, originalProxy) {
				return nil
			}
			changed = true
			fresh.Spec.Proxy = originalProxy
			_, err = operatorClient.OperatorV1().Authentications().Update(ctx, fresh, metav1.UpdateOptions{})
			return err
		}); err != nil {
			g.GinkgoWriter.Printf("cleanup: failed to restore authentication/cluster: %v\n", err)
		}

		g.GinkgoWriter.Println("cleanup: restoring oauth/cluster")
		if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			fresh, err := oauthClient.Get(ctx, "cluster", metav1.GetOptions{})
			if err != nil {
				return err
			}
			if reflect.DeepEqual(fresh.Spec.IdentityProviders, originalIdentityProviders) {
				return nil
			}
			changed = true
			fresh.Spec.IdentityProviders = originalIdentityProviders
			_, err = oauthClient.Update(ctx, fresh, metav1.UpdateOptions{})
			return err
		}); err != nil {
			g.GinkgoWriter.Printf("cleanup: failed to restore oauth/cluster: %v\n", err)
		}

		if changed {
			g.GinkgoWriter.Println("cleanup: waiting for operator to stabilize")
			if err := waitForOperatorToPickUpChanges(ctx, oc, "authentication"); err != nil {
				g.GinkgoWriter.Printf("cleanup: operator did not recover: %v\n", err)
			}
		}
		return nil
	}, nil
}

func createTrustedCAConfigMap(ctx context.Context, oc *exutil.CLI, caCertPEM []byte) (string, removalFunc, error) {
	const configMapName = "e2e-proxy-trusted-ca"
	kubeClient := oc.AdminKubeClient()
	_, err := kubeClient.CoreV1().ConfigMaps("openshift-config").Create(ctx, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:   configMapName,
			Labels: auth_proxy.ComponentProxyTestLabels(),
		},
		Data: map[string]string{
			"ca-bundle.crt": string(caCertPEM),
		},
	}, metav1.CreateOptions{})
	if err != nil {
		return "", nil, fmt.Errorf("failed to create trusted CA configmap \"openshift-config/%s\": %w", configMapName, err)
	}

	return configMapName, func(ctx context.Context) error {
		g.GinkgoWriter.Println("cleanup: removing trustedCA configmap")
		if err := kubeClient.CoreV1().ConfigMaps("openshift-config").Delete(ctx, configMapName, metav1.DeleteOptions{}); err != nil {
			g.GinkgoWriter.Printf("failed to clean up configmap \"openshift-config/%s\": %v\n", configMapName, err)
		}
		return nil
	}, nil
}

// keycloakProxySetup holds the results of deploying Keycloak for proxy tests,
// before the IdP is registered in OpenShift.
type keycloakProxySetup struct {
	client       *keycloakClient
	idpName      string
	namespace    string
	clientID     string
	clientSecret string
	issuerURL    string
}

func deployKeycloakForProxy(ctx context.Context, oc *exutil.CLI) (*keycloakProxySetup, []removalFunc, error) {
	namespace := fmt.Sprintf("e2e-proxy-kc-%s", rand.String(8))
	cleanups, err := deployKeycloak(ctx, oc, namespace, g.GinkgoLogr)
	if err != nil {
		return nil, cleanups, fmt.Errorf("deploying keycloak: %w", err)
	}

	setup := &keycloakProxySetup{
		idpName:   fmt.Sprintf("keycloak-proxy-test-%s", namespace),
		namespace: namespace,
	}

	// Use the route for admin API calls (the test runner may be external).
	routeURL, err := admittedURLForRoute(ctx, oc, keycloakResourceName, namespace)
	if err != nil {
		return nil, cleanups, fmt.Errorf("getting keycloak route URL: %w", err)
	}

	kcClient, err := keycloakClientFor(routeURL)
	if err != nil {
		return nil, cleanups, fmt.Errorf("creating keycloak client: %w", err)
	}
	setup.client = kcClient
	setup.issuerURL = routeURL + "/realms/master"

	err = wait.PollUntilContextTimeout(ctx, 5*time.Second, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
		err := kcClient.Authenticate("admin-cli", keycloakAdminUsername, keycloakAdminPassword)
		if err != nil {
			g.GinkgoWriter.Printf("failed to authenticate to Keycloak: %v\n", err)
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		return nil, cleanups, fmt.Errorf("authenticating to keycloak: %w", err)
	}

	clientList, err := kcClient.ListClients()
	if err != nil {
		return nil, cleanups, fmt.Errorf("listing keycloak clients: %w", err)
	}

	var adminClientID, passwdClientID string
	for _, c := range clientList {
		if c.ClientID == "admin-cli" {
			adminClientID = c.ID
		} else if len(c.RedirectURIs) > 0 {
			passwdClientID = c.ID
			setup.clientID = c.ClientID
		}
		if len(passwdClientID) > 0 && len(adminClientID) > 0 {
			break
		}
	}

	if adminClientID == "" {
		return nil, cleanups, fmt.Errorf("admin-cli client not found in keycloak")
	}
	if passwdClientID == "" {
		return nil, cleanups, fmt.Errorf("password-grant client (with redirectUris) not found in keycloak")
	}

	// Extend admin-cli token lifetime to 30 minutes so the token doesn't
	// expire during subsequent Keycloak API calls.
	err = wait.PollUntilContextTimeout(ctx, 5*time.Second, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
		err := kcClient.UpdateClientAccessTokenTimeout(adminClientID, 60*30)
		if err != nil {
			g.GinkgoWriter.Printf("failed to update client access token timeout: %v, retrying\n", err)
			if authErr := kcClient.Authenticate("admin-cli", keycloakAdminUsername, keycloakAdminPassword); authErr != nil {
				g.GinkgoWriter.Printf("failed to re-authenticate: %v\n", authErr)
			}
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		return nil, cleanups, fmt.Errorf("updating admin-cli access token timeout: %w", err)
	}

	err = kcClient.Authenticate("admin-cli", keycloakAdminUsername, keycloakAdminPassword)
	if err != nil {
		return nil, cleanups, fmt.Errorf("re-authenticating to keycloak: %w", err)
	}

	// Regenerate the client secret so we have a known value to pass to the
	// OAuth IdP configuration — the initial secret is auto-generated by Keycloak.
	err = wait.PollUntilContextTimeout(ctx, 5*time.Second, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
		var err error
		setup.clientSecret, err = kcClient.RegenerateClientSecret(passwdClientID)
		if err != nil {
			g.GinkgoWriter.Printf("failed to regenerate client secret: %v, retrying\n", err)
			if authErr := kcClient.Authenticate("admin-cli", keycloakAdminUsername, keycloakAdminPassword); authErr != nil {
				g.GinkgoWriter.Printf("failed to re-authenticate: %v\n", authErr)
			}
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		return nil, cleanups, fmt.Errorf("regenerating client secret: %w", err)
	}

	err = wait.PollUntilContextTimeout(ctx, 5*time.Second, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
		err := kcClient.CreateClientGroupMapper(passwdClientID, "test-groups-mapper", "groups")
		if err != nil {
			g.GinkgoWriter.Printf("failed to create client group mapper: %v, retrying\n", err)
			if authErr := kcClient.Authenticate("admin-cli", keycloakAdminUsername, keycloakAdminPassword); authErr != nil {
				g.GinkgoWriter.Printf("failed to re-authenticate: %v\n", authErr)
			}
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		return nil, cleanups, fmt.Errorf("creating client group mapper: %w", err)
	}

	return setup, cleanups, nil
}

func addKeycloakOIDCIdPForProxy(ctx context.Context, oc *exutil.CLI, setup *keycloakProxySetup) ([]removalFunc, error) {
	var cleanups []removalFunc
	kubeClient := oc.AdminKubeClient()

	secretName := setup.idpName + "-secret"
	_, err := kubeClient.CoreV1().Secrets("openshift-config").Create(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:   secretName,
			Labels: auth_proxy.ComponentProxyTestLabels(),
		},
		Data: map[string][]byte{
			"clientSecret": []byte(setup.clientSecret),
		},
	}, metav1.CreateOptions{})
	if err != nil {
		return cleanups, fmt.Errorf("creating keycloak client secret: %w", err)
	}
	cleanups = append(cleanups, func(ctx context.Context) error {
		return kubeClient.CoreV1().Secrets("openshift-config").Delete(ctx, secretName, metav1.DeleteOptions{})
	})

	caCMName := setup.idpName + "-ca"
	caCleanup, err := syncDefaultIngressCAToConfig(ctx, oc, caCMName)
	if err != nil {
		return cleanups, fmt.Errorf("syncing default ingress CA: %w", err)
	}
	cleanups = append(cleanups, caCleanup)

	err = addIdentityProvider(ctx, oc, configv1.IdentityProvider{
		Name:          setup.idpName,
		MappingMethod: configv1.MappingMethodClaim,
		IdentityProviderConfig: configv1.IdentityProviderConfig{
			Type: configv1.IdentityProviderTypeOpenID,
			OpenID: &configv1.OpenIDIdentityProvider{
				ClientID: setup.clientID,
				ClientSecret: configv1.SecretNameReference{
					Name: secretName,
				},
				ExtraScopes: []string{"profile", "email"},
				Claims: configv1.OpenIDClaims{
					PreferredUsername: []string{"preferred_username"},
					Groups:            []configv1.OpenIDClaim{"groups"},
				},
				Issuer: setup.issuerURL,
				CA: configv1.ConfigMapNameReference{
					Name: caCMName,
				},
			},
		},
	})
	if err != nil {
		return cleanups, fmt.Errorf("adding identity provider: %w", err)
	}

	return cleanups, nil
}

// syncDefaultIngressCAToConfig copies the default ingress CA into a new
// ConfigMap in openshift-config so it can be referenced by an IdP's CA field.
func syncDefaultIngressCAToConfig(ctx context.Context, oc *exutil.CLI, name string) (removalFunc, error) {
	kubeClient := oc.AdminKubeClient()

	ca, err := kubeClient.CoreV1().ConfigMaps("openshift-config-managed").Get(ctx, "default-ingress-cert", metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting openshift-config-managed/default-ingress-cert: %w", err)
	}

	_, err = kubeClient.CoreV1().ConfigMaps("openshift-config").Create(ctx, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: auth_proxy.ComponentProxyTestLabels(),
		},
		Data: map[string]string{
			"ca.crt": ca.Data["ca-bundle.crt"],
		},
	}, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("creating configmap openshift-config/%s: %w", name, err)
	}

	return func(ctx context.Context) error {
		return kubeClient.CoreV1().ConfigMaps("openshift-config").Delete(ctx, name, metav1.DeleteOptions{})
	}, nil
}

func updateAuthenticationProxy(ctx context.Context, oc *exutil.CLI, proxy operatorv1.AuthenticationProxyConfig) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		auth, err := oc.AdminOperatorClient().OperatorV1().Authentications().Get(ctx, "cluster", metav1.GetOptions{})
		if err != nil {
			return err
		}
		auth.Spec.Proxy = proxy
		_, err = oc.AdminOperatorClient().OperatorV1().Authentications().Update(ctx, auth, metav1.UpdateOptions{})
		return err
	})
}

func addIdentityProvider(ctx context.Context, oc *exutil.CLI, idp configv1.IdentityProvider) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		oauth, err := oc.AdminConfigClient().ConfigV1().OAuths().Get(ctx, "cluster", metav1.GetOptions{})
		if err != nil {
			return err
		}
		oauth.Spec.IdentityProviders = append(oauth.Spec.IdentityProviders, idp)
		_, err = oc.AdminConfigClient().ConfigV1().OAuths().Update(ctx, oauth, metav1.UpdateOptions{})
		return err
	})
}

func verifyOAuthServerDeploymentProxyConfig(
	ctx context.Context,
	oc *exutil.CLI,
	expectedAuthProxyConfig auth_proxy.ProxyConfig,
) error {
	kubeClient := oc.AdminKubeClient()
	hasCACert := len(expectedAuthProxyConfig.CACertPEM) > 0

	return wait.PollUntilContextTimeout(ctx, 10*time.Second, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
		deployment, err := kubeClient.AppsV1().Deployments("openshift-authentication").Get(ctx, "oauth-openshift", metav1.GetOptions{})
		if err != nil {
			g.GinkgoWriter.Printf("failed to get oauth-openshift deployment: %v\n", err)
			return false, nil
		}

		actualOauthServerProxyConfig, err := auth_proxy.DeploymentProxyConfig(ctx,
			"openshift-authentication", "oauth-openshift", "oauth-openshift",
			corev1.ConfigMapKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: componentProxyCAConfigMapName},
				Key:                  "ca-bundle.crt",
			}, kubeClient.AppsV1(), kubeClient.CoreV1())
		if err != nil {
			g.GinkgoWriter.Printf("failed to get actual oauth server proxy config: %v\n", err)
			return false, nil
		}

		if !actualOauthServerProxyConfig.Matches(expectedAuthProxyConfig) {
			g.GinkgoWriter.Printf("proxy env mismatch: found=%q (want %q)\n", actualOauthServerProxyConfig, expectedAuthProxyConfig)
			return false, nil
		}

		foundVolume, foundMount := trustedCAVolumeState(deployment)
		if hasCACert != (foundVolume && foundMount) {
			g.GinkgoWriter.Printf("trustedCA volume=%v mount=%v (want present=%v)\n", foundVolume, foundMount, hasCACert)
			return false, nil
		}
		return true, nil
	})
}

func trustedCAVolumeState(deployment *appsv1.Deployment) (foundVolume, foundMount bool) {
	var volumeName string
	for _, vol := range deployment.Spec.Template.Spec.Volumes {
		if vol.ConfigMap != nil && vol.ConfigMap.Name == componentProxyCAConfigMapName {
			foundVolume = true
			volumeName = vol.Name
			break
		}
	}
	if !foundVolume {
		return false, false
	}

	for _, container := range deployment.Spec.Template.Spec.Containers {
		for _, mount := range container.VolumeMounts {
			if mount.Name == volumeName {
				foundMount = true
				break
			}
		}
	}

	return foundVolume, foundMount
}

func verifyTrustedCAConfigMapSynced(ctx context.Context, oc *exutil.CLI) error {
	kubeClient := oc.AdminKubeClient()

	return wait.PollUntilContextTimeout(ctx, 10*time.Second, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
		cm, err := kubeClient.CoreV1().
			ConfigMaps("openshift-authentication").
			Get(ctx, componentProxyCAConfigMapName, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}
		return len(cm.Data) > 0, nil
	})
}
