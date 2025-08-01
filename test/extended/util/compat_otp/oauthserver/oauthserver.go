package oauthserver

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"path"
	"time"

	"github.com/RangelReale/osincli"
	"github.com/davecgh/go-spew/spew"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	restclient "k8s.io/client-go/rest"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	configv1 "github.com/openshift/api/config/v1"
	legacyconfigv1 "github.com/openshift/api/legacyconfig/v1"
	oauthv1 "github.com/openshift/api/oauth/v1"
	osinv1 "github.com/openshift/api/osin/v1"
	configclient "github.com/openshift/client-go/config/clientset/versioned"
	"github.com/openshift/library-go/pkg/config/helpers"
	"github.com/openshift/library-go/pkg/crypto"

	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/compat_otp"
	"github.com/openshift/origin/test/extended/util/compat_otp/oauthserver/tokencmd"
	"github.com/openshift/origin/test/extended/util/compat_otp/testdata"
)

const (
	serviceURLFmt = "https://test-oauth-svc.%s.svc" // fill in the namespace

	servingCertDirPath  = "/var/config/system/secrets/serving-cert"
	servingCertPathCert = "/var/config/system/secrets/serving-cert/tls.crt"
	servingCertPathKey  = "/var/config/system/secrets/serving-cert/tls.key"

	routerCertsDirPath = "/var/config/system/secrets/router-certs"

	sessionSecretDirPath = "/var/config/system/secrets/session-secret"
	sessionSecretPath    = "/var/config/system/secrets/session-secret/session"

	oauthConfigPath  = "/var/config/system/configmaps/oauth-config"
	serviceCADirPath = "/var/config/system/configmaps/service-ca"

	configObjectsDir = "/var/oauth/configobjects/"

	RouteName = "test-oauth-route"
	SAName    = "e2e-oauth"
)

var (
	serviceCAPath = "/var/config/system/configmaps/service-ca/service-ca.crt" // has to be var so that we can use its address

	defaultProcMount         = corev1.DefaultProcMount
	volumesDefaultMode int32 = 420
)

type NewRequestTokenOptionsFunc func(username, password string) *tokencmd.RequestTokenOptions

// DeployOAuthServer - deployes an instance of an OpenShift OAuth server
// very simplified for now
// returns OAuth server url, cleanup function, error
func DeployOAuthServer(oc *exutil.CLI, idps []osinv1.IdentityProvider, configMaps []corev1.ConfigMap, secrets []corev1.Secret) (NewRequestTokenOptionsFunc, func(), error) {

	var cleanupFuncs []func()
	cleanupFunc := func() {
		for _, f := range cleanupFuncs {
			f()
		}
	}

	// create the CA bundle, Service, Route and SA
	oauthServerDataDir := exutil.FixturePath("testdata", "oauthserver")
	for _, res := range []string{"cabundle-cm.yaml", "oauth-sa.yaml", "oauth-network.yaml"} {
		if err := oc.AsAdmin().Run("create").Args("-f", path.Join(oauthServerDataDir, res)).Execute(); err != nil {
			return nil, cleanupFunc, err
		}
		e2e.Logf("Created resources defined in %v", res)
	}

	kubeClient := oc.AdminKubeClient()

	// the oauth server needs access to kube-system configmaps/extension-apiserver-authentication
	clusterRoleBinding, err := createClusterRoleBinding(oc)
	if err != nil {
		return nil, cleanupFunc, err
	}
	cleanupFuncs = append(cleanupFuncs, func() {
		_ = oc.AsAdmin().Run("delete").Args("clusterrolebindings.rbac.authorization.k8s.io", clusterRoleBinding.Name).Execute()
	})
	e2e.Logf("Created: %v %v", "ClusterRoleBinding", clusterRoleBinding.Name)

	// create the secrets and configmaps the OAuth server config requires to get the server going
	for _, cm := range configMaps {
		if _, err := kubeClient.CoreV1().ConfigMaps(oc.Namespace()).Create(context.Background(), &cm, metav1.CreateOptions{}); err != nil {
			return nil, cleanupFunc, err
		}
		e2e.Logf("Created: %v %v/%v", "ConfigMap", oc.Namespace(), cm.Name)
	}
	for _, secret := range secrets {
		if _, err := kubeClient.CoreV1().Secrets(oc.Namespace()).Create(context.Background(), &secret, metav1.CreateOptions{}); err != nil {
			return nil, cleanupFunc, err
		}
		e2e.Logf("Created: %v %v/%v", secret.Kind, secret.Namespace, secret.Name)
	}

	// generate a session secret for the oauth server
	sessionSecret, err := randomSessionSecret()
	if err != nil {
		return nil, cleanupFunc, err
	}
	if _, err := kubeClient.CoreV1().Secrets(oc.Namespace()).Create(context.Background(), sessionSecret, metav1.CreateOptions{}); err != nil {
		return nil, cleanupFunc, err
	}
	e2e.Logf("Created: %v %v/%v", "Secret", oc.Namespace(), sessionSecret.Name)

	// get the route of the future OAuth server (defined in the oauth-network.yaml fixture above)
	route, err := oc.AdminRouteClient().RouteV1().Routes(oc.Namespace()).Get(context.Background(), RouteName, metav1.GetOptions{})
	if err != nil {
		return nil, cleanupFunc, err
	}
	routeURL := fmt.Sprintf("https://%s", route.Spec.Host)

	// prepare the config, inject it with the route URL and the IdP config we got
	config, err := oauthServerConfig(oc, routeURL, idps)
	if err != nil {
		return nil, cleanupFunc, err
	}

	configBytes := encode(config)
	if configBytes == nil {
		return nil, cleanupFunc, fmt.Errorf("error encoding the OSIN config")
	}

	// store the config in a ConfigMap that's to be mounted into the server's pod
	_, err = kubeClient.CoreV1().ConfigMaps(oc.Namespace()).Create(context.Background(), &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: "oauth-config",
		},
		Data: map[string]string{
			"oauth.conf": string(configBytes),
		},
	}, metav1.CreateOptions{})
	if err != nil {
		return nil, cleanupFunc, err
	}
	e2e.Logf("Created: %v %v/%v", "ConfigMap", oc.Namespace(), "oauth-config")

	// get the OAuth server image that's used in the cluster
	image, err := getImage(oc)
	if err != nil {
		return nil, cleanupFunc, err
	}

	// prepare the pod def, create secrets and CMs
	oauthServerPod, err := oauthServerPod(configMaps, secrets, image)
	if err != nil {
		return nil, cleanupFunc, err
	}

	// finally create the oauth server, wait till it starts running
	if _, err := kubeClient.CoreV1().Pods(oc.Namespace()).Create(context.Background(), oauthServerPod, metav1.CreateOptions{}); err != nil {
		return nil, cleanupFunc, err
	}
	e2e.Logf("Created: %v %v/%v", "Pod", oc.Namespace(), oauthServerPod.Name)

	if err := waitForOAuthServerReady(oc); err != nil {
		return nil, cleanupFunc, err
	}
	e2e.Logf("OAuth server is ready")

	oauthClient, err := createOAuthClient(oc, routeURL)
	if err != nil {
		return nil, cleanupFunc, err
	}
	cleanupFuncs = append(cleanupFuncs, func() {
		_ = oc.AsAdmin().Run("delete").Args("oauthclients.oauth.openshift.io", oauthClient.Name).Execute()
	})
	e2e.Logf("Created: %v %v/%v", oauthClient.Kind, oauthClient.Namespace, oauthClient.Name)

	newRequestTokenOptionFunc := func(username, password string) *tokencmd.RequestTokenOptions {
		return newRequestTokenOptions(restclient.AnonymousClientConfig(oc.AdminConfig()), routeURL, oc.Namespace(), username, password)
	}

	return newRequestTokenOptionFunc, cleanupFunc, nil
}

func waitForOAuthServerReady(oc *exutil.CLI) error {
	if err := compat_otp.WaitForUserBeAuthorized(oc, "system:serviceaccount:"+oc.Namespace()+":e2e-oauth", "*", "*"); err != nil {
		return err
	}
	if err := waitForOAuthServerPodReady(oc); err != nil {
		return err
	}
	return waitForOAuthServerRouteReady(oc)
}

func waitForOAuthServerPodReady(oc *exutil.CLI) error {
	e2e.Logf("Waiting for the OAuth server pod to be ready")
	return wait.PollImmediateInfinite(1*time.Second, func() (bool, error) {
		pod, err := oc.AdminKubeClient().CoreV1().Pods(oc.Namespace()).Get(context.Background(), "test-oauth-server", metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		if !exutil.CheckPodIsReady(*pod) {
			e2e.Logf("OAuth server pod is not ready: %s\nContainer statuses: %s", pod.Status.Message, spew.Sdump(pod.Status.ContainerStatuses))
			return false, nil
		}
		return true, nil
	})
}

func waitForOAuthServerRouteReady(oc *exutil.CLI) error {
	route, err := oc.AdminRouteClient().RouteV1().Routes(oc.Namespace()).Get(context.Background(), RouteName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	request, err := http.NewRequest(http.MethodHead, fmt.Sprintf("https://%s/healthz", route.Spec.Host), nil)
	if err != nil {
		return err
	}
	return wait.PollImmediate(time.Second, time.Minute, func() (done bool, err error) {
		e2e.Logf("Waiting for the OAuth server route to be ready")
		transport, err := restclient.TransportFor(restclient.AnonymousClientConfig(oc.AdminConfig()))
		if err != nil {
			e2e.Logf("Error getting transport: %v", err)
			return false, err
		}
		response, err := transport.RoundTrip(request)
		if response != nil && response.StatusCode == http.StatusOK {
			return true, nil
		}
		if response != nil {
			e2e.Logf("Waiting for the OAuth server route to be ready: %v", response.Status)
		}
		if err != nil {
			e2e.Logf("Waiting for the OAuth server route to be ready: %v", err)
		}
		return false, nil
	})
}

func oauthServerPod(configMaps []corev1.ConfigMap, secrets []corev1.Secret, image string) (*corev1.Pod, error) {
	oauthServerAsset := testdata.MustAsset("test/extended/testdata/oauthserver/oauth-pod.yaml")

	obj, err := helpers.ReadYAML(bytes.NewBuffer(oauthServerAsset), corev1.AddToScheme)
	if err != nil {
		return nil, err
	}

	oauthServerPod, ok := obj.(*corev1.Pod)
	if ok != true {
		return nil, err
	}

	volumes := oauthServerPod.Spec.Volumes
	volumeMounts := oauthServerPod.Spec.Containers[0].VolumeMounts

	for _, cm := range configMaps {
		volumes, volumeMounts = addCMMount(volumes, volumeMounts, &cm)
	}

	for _, sec := range secrets {
		volumes, volumeMounts = addSecretMount(volumes, volumeMounts, &sec)
	}

	oauthServerPod.Spec.Volumes = volumes
	oauthServerPod.Spec.Containers[0].VolumeMounts = volumeMounts
	oauthServerPod.Spec.Containers[0].Image = image

	return oauthServerPod, nil
}

func addCMMount(volumes []corev1.Volume, volumeMounts []corev1.VolumeMount, cm *corev1.ConfigMap) ([]corev1.Volume, []corev1.VolumeMount) {
	volumes = append(volumes, corev1.Volume{
		Name: cm.ObjectMeta.Name,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: cm.ObjectMeta.Name},
				DefaultMode:          &volumesDefaultMode,
			},
		},
	})

	volumeMounts = append(volumeMounts, corev1.VolumeMount{
		Name:      cm.ObjectMeta.Name,
		MountPath: GetDirPathFromConfigMapSecretName(cm.ObjectMeta.Name),
		ReadOnly:  true,
	})

	return volumes, volumeMounts
}

func addSecretMount(volumes []corev1.Volume, volumeMounts []corev1.VolumeMount, secret *corev1.Secret) ([]corev1.Volume, []corev1.VolumeMount) {
	volumes = append(volumes, corev1.Volume{
		Name: secret.ObjectMeta.Name,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName:  secret.ObjectMeta.Name,
				DefaultMode: &volumesDefaultMode,
			},
		},
	})

	volumeMounts = append(volumeMounts, corev1.VolumeMount{
		Name:      secret.ObjectMeta.Name,
		MountPath: GetDirPathFromConfigMapSecretName(secret.ObjectMeta.Name),
		ReadOnly:  true,
	})

	return volumes, volumeMounts
}

func oauthServerConfig(oc *exutil.CLI, routeURL string, idps []osinv1.IdentityProvider) (*osinv1.OsinServerConfig, error) {
	adminConfigClient := configclient.NewForConfigOrDie(oc.AdminConfig()).ConfigV1()

	infrastructure, err := adminConfigClient.Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	console, err := adminConfigClient.Consoles().Get(context.Background(), "cluster", metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	namedRouterCerts, err := routerCertsToSNIConfig(oc)
	if err != nil {
		return nil, err
	}

	return &osinv1.OsinServerConfig{
		GenericAPIServerConfig: configv1.GenericAPIServerConfig{
			ServingInfo: configv1.HTTPServingInfo{
				ServingInfo: configv1.ServingInfo{
					BindAddress: "0.0.0.0:6443",
					BindNetwork: "tcp4",
					// we have valid serving certs provided by service-ca
					// this is our main server cert which is used if SNI does not match
					CertInfo: configv1.CertInfo{
						CertFile: servingCertPathCert,
						KeyFile:  servingCertPathKey,
					},
					ClientCA:          "",
					NamedCertificates: namedRouterCerts,
					MinTLSVersion:     crypto.TLSVersionToNameOrDie(crypto.DefaultTLSVersion()),
					CipherSuites:      crypto.CipherSuitesToNamesOrDie(crypto.DefaultCiphers()),
				},
				MaxRequestsInFlight:   1000,
				RequestTimeoutSeconds: 5 * 60, // 5 minutes
			},
			AuditConfig: configv1.AuditConfig{},
			KubeClientConfig: configv1.KubeClientConfig{
				KubeConfig: "",
				ConnectionOverrides: configv1.ClientConnectionOverrides{
					QPS:   400,
					Burst: 400,
				},
			},
		},
		OAuthConfig: osinv1.OAuthConfig{
			MasterCA:                    &serviceCAPath, // we have valid serving certs provided by service-ca so we can use the service for loopback
			MasterURL:                   fmt.Sprintf(serviceURLFmt, oc.Namespace()),
			MasterPublicURL:             routeURL,
			LoginURL:                    infrastructure.Status.APIServerURL,
			AssetPublicURL:              console.Status.ConsoleURL, // set console route as valid 302 redirect for logout
			AlwaysShowProviderSelection: false,
			IdentityProviders:           idps,
			GrantConfig: osinv1.GrantConfig{
				Method:               osinv1.GrantHandlerDeny, // force denial as this field must be set per OAuth client
				ServiceAccountMethod: osinv1.GrantHandlerPrompt,
			},
			SessionConfig: &osinv1.SessionConfig{
				SessionSecretsFile:   sessionSecretPath,
				SessionMaxAgeSeconds: 5 * 60, // 5 minutes
				SessionName:          "ssn",
			},
			TokenConfig: osinv1.TokenConfig{
				AuthorizeTokenMaxAgeSeconds: 5 * 60,       // 5 minutes
				AccessTokenMaxAgeSeconds:    24 * 60 * 60, // 1 day
			},
		},
	}, nil
}

func routerCertsToSNIConfig(oc *exutil.CLI) ([]configv1.NamedCertificate, error) {
	routerSecret, err := oc.AdminKubeClient().CoreV1().Secrets("openshift-config-managed").Get(context.Background(), "router-certs", metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	localRouterSecret := routerSecret.DeepCopy()
	localRouterSecret.ResourceVersion = ""
	localRouterSecret.Namespace = oc.Namespace()
	if _, err := oc.AdminKubeClient().CoreV1().Secrets(oc.Namespace()).Create(context.Background(), localRouterSecret, metav1.CreateOptions{}); err != nil {
		return nil, err
	}

	var out []configv1.NamedCertificate
	for domain := range localRouterSecret.Data {
		out = append(out, configv1.NamedCertificate{
			Names: []string{"*." + domain}, // ingress domain is always a wildcard
			CertInfo: configv1.CertInfo{ // the cert and key are appended together
				CertFile: routerCertsDirPath + "/" + domain,
				KeyFile:  routerCertsDirPath + "/" + domain,
			},
		})
	}
	return out, nil
}

func randomSessionSecret() (*corev1.Secret, error) {
	skey, err := newSessionSecretsJSON()
	if err != nil {
		return nil, err
	}
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "session-secret",
			Labels: map[string]string{
				"app": "test-oauth-server",
			},
		},
		Data: map[string][]byte{
			"session": skey,
		},
	}, nil
}

// this is less random than the actual secret generated in cluster-authentication-operator
func newSessionSecretsJSON() ([]byte, error) {
	const (
		sha256KeyLenBytes = sha256.BlockSize // max key size with HMAC SHA256
		aes256KeyLenBytes = 32               // max key size with AES (AES-256)
	)

	secrets := &legacyconfigv1.SessionSecrets{
		TypeMeta: metav1.TypeMeta{
			Kind:       "SessionSecrets",
			APIVersion: "v1",
		},
		Secrets: []legacyconfigv1.SessionSecret{
			{
				Authentication: randomString(sha256KeyLenBytes), // 64 chars
				Encryption:     randomString(aes256KeyLenBytes), // 32 chars
			},
		},
	}
	secretsBytes, err := json.Marshal(secrets)
	if err != nil {
		return nil, fmt.Errorf("error marshalling the session secret: %v", err) // should never happen
	}

	return secretsBytes, nil
}

// randomString - random string of A-Z chars with len size
func randomString(size int) string {
	buffer := make([]byte, size)
	for i := 0; i < size; i++ {
		buffer[i] = byte(65 + rand.Intn(25))
	}
	return base64.RawURLEncoding.EncodeToString(buffer)
}

// getImage will grab the hypershift image version from openshift-authentication ns
func getImage(oc *exutil.CLI) (string, error) {
	selector, _ := labels.Parse("app=oauth-openshift")
	pods, err := oc.AdminKubeClient().CoreV1().Pods("openshift-authentication").List(context.Background(), metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return "", err
	}
	return pods.Items[0].Spec.Containers[0].Image, nil
}

func newRequestTokenOptions(config *restclient.Config, oauthServerURL, oauthClientName, username, password string) *tokencmd.RequestTokenOptions {
	options := tokencmd.NewRequestTokenOptions(config, nil, username, password, false)
	// supply the info the client would otherwise ask from .well-known/oauth-authorization-server
	oauthClientConfig := &osincli.ClientConfig{
		ClientId:     oauthClientName,
		AuthorizeUrl: fmt.Sprintf("%s/oauth/authorize", oauthServerURL), // TODO: the endpoints are defined in vendor/github.com/openshift/library-go/pkg/oauth/oauthdiscovery/urls.go
		TokenUrl:     fmt.Sprintf("%s/oauth/token", oauthServerURL),
		RedirectUrl:  fmt.Sprintf("%s/oauth/token/implicit", oauthServerURL),
	}
	if err := osincli.PopulatePKCE(oauthClientConfig); err != nil {
		panic(err)
	}
	options.OsinConfig = oauthClientConfig
	options.Issuer = oauthServerURL
	return options
}

func createClusterRoleBinding(oc *exutil.CLI) (*rbacv1.ClusterRoleBinding, error) {
	return oc.AdminKubeClient().RbacV1().ClusterRoleBindings().Create(context.Background(), &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: oc.Namespace(),
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     "cluster-admin",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      SAName,
				Namespace: oc.Namespace(),
			},
		},
	}, metav1.CreateOptions{})
}

func createOAuthClient(oc *exutil.CLI, routeURL string) (*oauthv1.OAuthClient, error) {
	return oc.AdminOAuthClient().OauthV1().OAuthClients().
		Create(context.Background(), &oauthv1.OAuthClient{
			ObjectMeta: metav1.ObjectMeta{
				Name: oc.Namespace(),
			},
			GrantMethod:           oauthv1.GrantHandlerAuto,
			RedirectURIs:          []string{fmt.Sprintf("%s/oauth/token/implicit", routeURL)},
			RespondWithChallenges: true,
		}, metav1.CreateOptions{})
}
