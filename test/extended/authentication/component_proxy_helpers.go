package authentication

import (
	"context"
	"fmt"
	"net"
	"reflect"
	"strconv"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	libcrypto "github.com/openshift/library-go/pkg/crypto"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	watchtools "k8s.io/client-go/tools/watch"
	"k8s.io/client-go/util/retry"

	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/image"
)

const (
	squidImage       = "registry.redhat.io/rhel10/squid:10.2-1784702318"
	squidHTTPPort    = int32(3128)
	squidHTTPSPort   = int32(3129)
	squidServiceName = "squid-proxy"

	componentProxyCAConfigMapName = "v4-0-config-system-auth-proxy-ca"
)

func componentProxyTestLabels() map[string]string {
	return map[string]string{
		"e2e-test": "openshift-authentication-operator",
	}
}

// saveAndRestoreAuthState snapshots the Authentication operator CR and
// oauth/cluster, returning a cleanup function that restores both.
// If either resource was modified, it waits for the operator to stabilize.
func saveAndRestoreAuthState(ctx context.Context, oc *exutil.CLI) (removalFunc, error) {
	operatorClient := oc.AdminOperatorClient()
	oauthClient := oc.AdminConfigClient().ConfigV1().OAuths()

	auth, err := operatorClient.OperatorV1().Authentications().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting authentication/cluster: %w", err)
	}
	originalAuthSpec := auth.Spec.DeepCopy()

	oauth, err := oauthClient.Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting oauth/cluster: %w", err)
	}
	originalOAuthSpec := oauth.Spec.DeepCopy()

	return func(ctx context.Context) error {
		var changed bool

		g.GinkgoWriter.Println("cleanup: restoring authentication/cluster")
		if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			fresh, err := operatorClient.OperatorV1().Authentications().Get(ctx, "cluster", metav1.GetOptions{})
			if err != nil {
				return err
			}
			if reflect.DeepEqual(fresh.Spec, *originalAuthSpec) {
				return nil
			}
			changed = true
			fresh.Spec = *originalAuthSpec
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
			if reflect.DeepEqual(fresh.Spec, *originalOAuthSpec) {
				return nil
			}
			changed = true
			fresh.Spec = *originalOAuthSpec
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
			Labels: componentProxyTestLabels(),
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

// deploySquidProxy deploys a Squid forward proxy listening on HTTP (3128) and
// HTTPS (3129) with a self-signed CA and serving certificate.
func deploySquidProxy(ctx context.Context, oc *exutil.CLI) (httpProxyURL, httpsProxyURL string, caCertPEM []byte, namespace string, cleanup removalFunc, err error) {
	kubeClient := oc.AdminKubeClient()

	nsLabels := componentProxyTestLabels()
	nsLabels["pod-security.kubernetes.io/enforce"] = "baseline"
	nsLabels["security.openshift.io/scc.podSecurityLabelSync"] = "false"
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "e2e-proxy-",
			Labels:       nsLabels,
		},
	}
	created, err := kubeClient.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	if err != nil {
		return "", "", nil, "", nil, fmt.Errorf("creating Squid proxy namespace: %w", err)
	}
	namespace = created.Name
	cleanup = func(ctx context.Context) error {
		g.GinkgoWriter.Println("cleanup: removing Squid proxy namespace")
		err := kubeClient.CoreV1().Namespaces().Delete(ctx, namespace, metav1.DeleteOptions{})
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	caConfig, err := libcrypto.MakeSelfSignedCAConfigForDuration("squid-proxy-ca", 2*time.Hour)
	if err != nil {
		return "", "", nil, "", cleanup, fmt.Errorf("creating proxy CA: %w", err)
	}
	ca := &libcrypto.CA{Config: caConfig, SerialGenerator: &libcrypto.RandomSerialGenerator{}}

	serviceHost := fmt.Sprintf("%s.%s.svc.cluster.local", squidServiceName, namespace)
	serverCertConfig, err := ca.MakeServerCert(sets.New(serviceHost), 2*time.Hour)
	if err != nil {
		return "", "", nil, "", cleanup, fmt.Errorf("creating proxy server cert: %w", err)
	}

	caCertPEM, _, err = caConfig.GetPEMBytes()
	if err != nil {
		return "", "", nil, "", cleanup, fmt.Errorf("encoding proxy CA cert: %w", err)
	}
	serverCertPEM, serverKeyPEM, err := serverCertConfig.GetPEMBytes()
	if err != nil {
		return "", "", nil, "", cleanup, fmt.Errorf("encoding proxy server cert: %w", err)
	}

	// logformat fields: method url sourceIP squidStatus httpCode — parsed by waitForProxyTrafficFrom
	squidConfig := fmt.Sprintf(`http_port %d
https_port %d tls-cert=/etc/squid/tls/tls.crt tls-key=/etc/squid/tls/tls.key
pid_filename /tmp/squid.pid
acl all src all
http_access allow all
logformat proxytest %%rm %%ru %%>a %%Ss %%>Hs
access_log stdio:/dev/stdout proxytest
cache_log stdio:/dev/stderr
cache deny all
buffered_logs off
`, squidHTTPPort, squidHTTPSPort)

	_, err = kubeClient.CoreV1().ConfigMaps(namespace).Create(ctx, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "squid-config"},
		Data:       map[string]string{"squid.conf": squidConfig},
	}, metav1.CreateOptions{})
	if err != nil {
		return "", "", nil, "", cleanup, fmt.Errorf("creating Squid config: %w", err)
	}

	_, err = kubeClient.CoreV1().Secrets(namespace).Create(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "squid-tls"},
		Data: map[string][]byte{
			"tls.crt": serverCertPEM,
			"tls.key": serverKeyPEM,
		},
	}, metav1.CreateOptions{})
	if err != nil {
		return "", "", nil, "", cleanup, fmt.Errorf("creating Squid TLS secret: %w", err)
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:   squidServiceName,
			Labels: map[string]string{"app": squidServiceName},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: new(int32(1)),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": squidServiceName},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": squidServiceName},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "squid",
							Image: image.LocationFor(squidImage),
							Ports: []corev1.ContainerPort{
								{ContainerPort: squidHTTPPort, Protocol: corev1.ProtocolTCP},
								{ContainerPort: squidHTTPSPort, Protocol: corev1.ProtocolTCP},
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "squid-config", MountPath: "/etc/squid/squid.conf", SubPath: "squid.conf"},
								{Name: "squid-tls", MountPath: "/etc/squid/tls", ReadOnly: true},
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									TCPSocket: &corev1.TCPSocketAction{
										Port: intstr.FromInt32(squidHTTPPort),
									},
								},
								InitialDelaySeconds: 5,
								PeriodSeconds:       5,
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "squid-config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{Name: "squid-config"},
								},
							},
						},
						{
							Name: "squid-tls",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{SecretName: "squid-tls"},
							},
						},
					},
				},
			},
		},
	}

	_, err = kubeClient.AppsV1().Deployments(namespace).Create(ctx, deployment, metav1.CreateOptions{})
	if err != nil {
		return "", "", nil, "", cleanup, fmt.Errorf("creating Squid deployment: %w", err)
	}

	_, err = kubeClient.CoreV1().Services(namespace).Create(ctx, &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:   squidServiceName,
			Labels: map[string]string{"app": squidServiceName},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": squidServiceName},
			Ports: []corev1.ServicePort{
				{Name: "http", Port: squidHTTPPort, TargetPort: intstr.FromInt32(squidHTTPPort), Protocol: corev1.ProtocolTCP},
				{Name: "https", Port: squidHTTPSPort, TargetPort: intstr.FromInt32(squidHTTPSPort), Protocol: corev1.ProtocolTCP},
			},
		},
	}, metav1.CreateOptions{})
	if err != nil {
		return "", "", nil, "", cleanup, fmt.Errorf("creating Squid service: %w", err)
	}

	g.GinkgoWriter.Printf("waiting for Squid proxy deployment in %s to be ready\n", namespace)
	timeLimitedCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	_, err = watchtools.UntilWithSync(timeLimitedCtx,
		cache.NewListWatchFromClient(
			kubeClient.AppsV1().RESTClient(), "deployments", namespace,
			fields.OneTermEqualSelector("metadata.name", squidServiceName)),
		&appsv1.Deployment{},
		nil,
		func(event watch.Event) (bool, error) {
			if event.Type == watch.Error {
				return false, fmt.Errorf("Squid deployment watch error: %v", event.Object)
			}
			if event.Type == watch.Bookmark {
				return false, nil
			}
			d, ok := event.Object.(*appsv1.Deployment)
			if !ok {
				return false, nil
			}
			return d.Status.ReadyReplicas > 0, nil
		},
	)
	if err != nil {
		return "", "", nil, "", cleanup, fmt.Errorf("Squid proxy deployment did not become ready: %w", err)
	}

	httpProxyURL = "http://" + net.JoinHostPort(serviceHost, strconv.Itoa(int(squidHTTPPort)))
	httpsProxyURL = "https://" + net.JoinHostPort(serviceHost, strconv.Itoa(int(squidHTTPSPort)))
	g.GinkgoWriter.Printf("Squid proxy deployed: http=%s https=%s\n", httpProxyURL, httpsProxyURL)
	return httpProxyURL, httpsProxyURL, caCertPEM, namespace, cleanup, nil
}

func getSquidProxyLogs(ctx context.Context, oc *exutil.CLI, namespace string) (string, error) {
	return getSquidProxyLogsSince(ctx, oc, namespace, time.Time{})
}

func getSquidProxyLogsSince(ctx context.Context, oc *exutil.CLI, namespace string, since time.Time) (string, error) {
	kubeClient := oc.AdminKubeClient()

	pods, err := kubeClient.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("app=%s", squidServiceName),
	})
	if err != nil {
		return "", fmt.Errorf("listing squid pods in %s: %w", namespace, err)
	}
	if len(pods.Items) == 0 {
		return "", fmt.Errorf("no squid proxy pods found in namespace %s", namespace)
	}

	logOpts := &corev1.PodLogOptions{Container: "squid"}
	if !since.IsZero() {
		t := metav1.NewTime(since)
		logOpts.SinceTime = &t
	}
	logBytes, err := kubeClient.CoreV1().Pods(namespace).GetLogs(pods.Items[0].Name, logOpts).DoRaw(ctx)
	if err != nil {
		return "", fmt.Errorf("getting logs from squid container: %w", err)
	}

	return string(logBytes), nil
}

// waitForProxyTrafficFrom polls the squid access log until a CONNECT entry from
// the given source IP appears, confirming that the source is routing traffic
// through the proxy. The log format is set by the proxytest logformat directive
// in the squid config: "method url sourceIP status httpCode".
func waitForProxyTrafficFrom(ctx context.Context, oc *exutil.CLI, proxyNamespace, sourceIP string, since time.Time, timeout time.Duration) error {
	g.GinkgoWriter.Printf("waiting up to %s for proxy traffic from %s\n", timeout, sourceIP)
	return wait.PollUntilContextTimeout(ctx, 10*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		logs, err := getSquidProxyLogsSince(ctx, oc, proxyNamespace, since)
		if err != nil {
			g.GinkgoWriter.Printf("failed to read squid logs: %v\n", err)
			return false, nil
		}
		for line := range strings.SplitSeq(logs, "\n") {
			parts := strings.Fields(line)
			if len(parts) >= 4 && parts[0] == "CONNECT" && parts[2] == sourceIP && parts[3] == "TCP_TUNNEL" {
				g.GinkgoWriter.Printf("confirmed proxy traffic from %s: %s\n", sourceIP, line)
				return true, nil
			}
		}
		return false, nil
	})
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
			Labels: componentProxyTestLabels(),
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
			Labels: componentProxyTestLabels(),
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

func verifyOAuthServerDeploymentProxyConfig(ctx context.Context, oc *exutil.CLI, expectedHTTPProxy, expectedHTTPSProxy, expectedNoProxy string, expectTrustedCAVolume bool) error {
	kubeClient := oc.AdminKubeClient()

	return wait.PollUntilContextTimeout(ctx, 10*time.Second, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
		deployment, err := kubeClient.AppsV1().Deployments("openshift-authentication").Get(ctx, "oauth-openshift", metav1.GetOptions{})
		if err != nil {
			g.GinkgoWriter.Printf("failed to get oauth-openshift deployment: %v\n", err)
			return false, nil
		}

		envVars := make(map[string]string)
		for _, container := range deployment.Spec.Template.Spec.Containers {
			for _, env := range container.Env {
				switch env.Name {
				case "HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY":
					envVars[env.Name] = env.Value
				}
			}
		}

		if envVars["HTTP_PROXY"] != expectedHTTPProxy || envVars["HTTPS_PROXY"] != expectedHTTPSProxy {
			g.GinkgoWriter.Printf("proxy env mismatch: HTTP_PROXY=%q (want %q), HTTPS_PROXY=%q (want %q)\n",
				envVars["HTTP_PROXY"], expectedHTTPProxy, envVars["HTTPS_PROXY"], expectedHTTPSProxy)
			return false, nil
		}
		if expectedNoProxy == "" {
			if envVars["NO_PROXY"] != "" {
				g.GinkgoWriter.Printf("proxy env mismatch: NO_PROXY=%q (want empty)\n", envVars["NO_PROXY"])
				return false, nil
			}
		} else {
			// Use IsSuperset rather than exact match because the operator appends
			// the apiserver IP to NO_PROXY beyond the entries we configure.
			actualNoProxy := sets.New[string](strings.Split(envVars["NO_PROXY"], ",")...)
			expectedNoProxyEntries := sets.New[string](strings.Split(expectedNoProxy, ",")...)
			if !actualNoProxy.IsSuperset(expectedNoProxyEntries) {
				g.GinkgoWriter.Printf("proxy env mismatch: NO_PROXY=%q does not contain all of %q\n", envVars["NO_PROXY"], expectedNoProxy)
				return false, nil
			}
		}

		foundVolume, foundMount := trustedCAVolumeState(deployment)
		if expectTrustedCAVolume != (foundVolume && foundMount) {
			g.GinkgoWriter.Printf("trustedCA volume=%v mount=%v (want present=%v)\n", foundVolume, foundMount, expectTrustedCAVolume)
			return false, nil
		}
		return true, nil
	})
}

func trustedCAVolumeState(deployment *appsv1.Deployment) (foundVolume, foundMount bool) {
	for _, vol := range deployment.Spec.Template.Spec.Volumes {
		if vol.ConfigMap != nil && vol.ConfigMap.Name == componentProxyCAConfigMapName {
			foundVolume = true
			break
		}
	}

	for _, container := range deployment.Spec.Template.Spec.Containers {
		for _, mount := range container.VolumeMounts {
			if mount.Name == componentProxyCAConfigMapName {
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
		cm, err := kubeClient.CoreV1().ConfigMaps("openshift-authentication").Get(ctx, componentProxyCAConfigMapName, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}
		return len(cm.Data) > 0, nil
	})
}

// deleteNamespaceSync deletes a namespace and polls until it is fully removed.
// Foreground delete propagation is being used, so all namespace resources are deleted by the time this function unblocks.
func deleteNamespaceSync(ctx context.Context, oc *exutil.CLI, namespace string, timeout time.Duration) error {
	kubeClient := oc.AdminKubeClient()
	if err := kubeClient.CoreV1().Namespaces().Delete(ctx, namespace, metav1.DeleteOptions{
		PropagationPolicy: new(metav1.DeletePropagationForeground),
	}); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("deleting namespace %s: %w", namespace, err)
	}

	g.GinkgoWriter.Printf("waiting up to %s for namespace %s to be fully deleted\n", timeout, namespace)
	return wait.PollUntilContextTimeout(ctx, 5*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		_, err := kubeClient.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		if err != nil {
			g.GinkgoWriter.Printf("error checking namespace %s: %v\n", namespace, err)
		}
		return false, nil
	})
}
