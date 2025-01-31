package router

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	admissionapi "k8s.io/pod-security-admission/api"

	routev1 "github.com/openshift/api/route/v1"
	"github.com/openshift/origin/test/extended/router/certgen"
	exutil "github.com/openshift/origin/test/extended/util"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/kubernetes/test/e2e/framework/pod"
)

const (
	SECRET_READER_ROLE               = "secret-reader-role"
	SECRET_READER_ROLE_BINDING       = "secret-reader-role-binding"
	CUSTOM_HOST_UPDATER_ROLE         = "custom-host-updater-role"
	CUSTOM_HOST_UPDATER_ROLE_BINDING = "custom-host-updater-role-binding"
)

var _ = g.Describe("[sig-network][OCPFeatureGate:RouteExternalCertificate][Feature:Router][apigroup:route.openshift.io]", func() {
	defer g.GinkgoRecover()
	var (
		oc            = exutil.NewCLIWithPodSecurityLevel("router-external-certificate", admissionapi.LevelBaseline)
		helloPodPath  = exutil.FixturePath("..", "..", "examples", "hello-openshift", "hello-pod.json")
		helloPodName  = "hello-openshift"
		helloPodSvc   = "hello-openshift"
		defaultDomain string
		err           error
	)

	g.BeforeEach(func() {
		defaultDomain, err = getDefaultIngressClusterDomainName(oc, time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to find default domain name")

		g.By("creating pod")
		err = oc.Run("create").Args("-f", helloPodPath, "-n", oc.Namespace()).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("waiting for the pod to be running")
		err = pod.WaitForPodNameRunningInNamespace(context.TODO(), oc.KubeClient(), helloPodName, oc.Namespace())
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("creating service")
		err = oc.Run("expose").Args("pod", helloPodName, "-n", oc.Namespace()).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("waiting for the service to become available")
		err = exutil.WaitForEndpoint(oc.KubeClient(), oc.Namespace(), helloPodSvc)
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.Describe("The router", func() {
		g.BeforeEach(func() {
		})
		g.It("should support external certificate", func() {

			host := oc.Namespace() + "." + defaultDomain

			g.By("Creating a secret with crt/key")
			secret, rootDerBytes, err := createSecret(oc.Namespace(), "my-tls-secret", corev1.SecretTypeTLS, host)
			o.Expect(err).NotTo(o.HaveOccurred())
			_, err = oc.KubeClient().CoreV1().Secrets(oc.Namespace()).Create(context.Background(), secret, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Providing router service account permissions to get,list,watch the secret")
			_, err = oc.KubeClient().RbacV1().Roles(oc.Namespace()).Create(context.Background(),
				createRole(oc.Namespace(), "my-tls-secret"), metav1.CreateOptions{},
			)
			o.Expect(err).NotTo(o.HaveOccurred())
			_, err = oc.KubeClient().RbacV1().RoleBindings(oc.Namespace()).Create(context.Background(),
				createRoleBinding(oc.Namespace()), metav1.CreateOptions{},
			)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Providing the user custom-host update permission (FIXME)")
			_, err = oc.AdminKubeClient().RbacV1().Roles(oc.Namespace()).Create(context.Background(),
				customHostRole(oc.Namespace()), metav1.CreateOptions{},
			)
			o.Expect(err).NotTo(o.HaveOccurred())
			_, err = oc.AdminKubeClient().RbacV1().RoleBindings(oc.Namespace()).Create(context.Background(),
				customHostRoleBinding(oc.Namespace(), oc.Username()), metav1.CreateOptions{},
			)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Creating a route with edge termination")
			edgeRoute := createRouteWithEC(oc.Namespace(), "edge-route", secret.Name, helloPodSvc, host, routev1.TLSTerminationEdge)
			_, err = oc.RouteClient().RouteV1().Routes(oc.Namespace()).Create(context.Background(), edgeRoute, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Sending https request")
			resp, err := httpsGetCall(host, rootDerBytes)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(resp).Should(o.ContainSubstring("Hello OpenShift"))

			// if stuck here, then no error upto here
			for {
			}
		})
	})
})

// Create Role for 'routes/custom-host' subresource update permissions
// Remove me later
func customHostRole(namespace string) *rbacv1.Role {
	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      CUSTOM_HOST_UPDATER_ROLE,
			Namespace: namespace,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{routev1.GroupName},
				Resources: []string{"routes/custom-host"},
				Verbs:     []string{"update"},
			},
		},
	}
}

// Remove me later
func customHostRoleBinding(namespace, user string) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      CUSTOM_HOST_UPDATER_ROLE_BINDING,
			Namespace: namespace,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind: rbacv1.UserKind,
				Name: user,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     CUSTOM_HOST_UPDATER_ROLE,
		},
	}
}

func createSecret(namespace, secretName string, secretType corev1.SecretType, hosts ...string) (*corev1.Secret, []byte, error) {
	// certificate start and end time are very
	// lenient to avoid any clock drift between
	// the test machine and the cluster under
	// test.
	notBefore := time.Now().Add(-24 * time.Hour)
	notAfter := time.Now().Add(24 * time.Hour)

	// Generate crt/key for secret
	rootDerBytes, tlsCrtData, tlsPrivateKey, err := certgen.GenerateKeyPair(notBefore, notAfter, hosts...)
	if err != nil {
		return nil, nil, err
	}

	derKey, err := certgen.MarshalPrivateKeyToDERFormat(tlsPrivateKey)
	if err != nil {
		return nil, nil, err
	}

	pemCrt, err := certgen.MarshalCertToPEMString(tlsCrtData)
	if err != nil {
		return nil, nil, err
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      secretName,
		},
		StringData: map[string]string{
			"tls.crt": pemCrt,
			"tls.key": derKey,
		},
		Type: secretType,
	}, rootDerBytes, nil
}

func createRouteWithEC(namespace, routeName, secretName, serviceName, host string, termination routev1.TLSTerminationType) *routev1.Route {

	return &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:      routeName,
			Namespace: namespace,
		},
		Spec: routev1.RouteSpec{
			Host: host,
			To: routev1.RouteTargetReference{
				Kind: "Service",
				Name: serviceName,
			},
			TLS: &routev1.TLSConfig{
				Termination: termination,
				ExternalCertificate: &routev1.LocalObjectReference{
					Name: secretName,
				},
			},
		},
	}
}

func createRole(namespace, secretName string) *rbacv1.Role {
	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      SECRET_READER_ROLE,
			Namespace: namespace,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups:     []string{""},
				Resources:     []string{"secrets"},
				ResourceNames: []string{secretName},
				Verbs:         []string{"get", "list", "watch"},
			},
		},
	}
}

func createRoleBinding(namespace string) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      SECRET_READER_ROLE_BINDING,
			Namespace: namespace,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      "router",
				Namespace: "openshift-ingress",
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     SECRET_READER_ROLE,
		},
	}
}

func httpsGetCall(hostname string, rootDerBytes []byte) (string, error) {
	if len(rootDerBytes) == 0 {
		return "", fmt.Errorf("root CA is empty; certificate generation likely failed")
	}
	// Convert DER to PEM
	rootCertPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: rootDerBytes,
	})

	// Add root CA to trust pool
	certPool := x509.NewCertPool()
	if ok := certPool.AppendCertsFromPEM(rootCertPEM); !ok {
		return "", fmt.Errorf("failed to add root CA certificate to cert pool")
	}
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: certPool,
			},
		},
	}

	url := fmt.Sprintf("https://%s", hostname)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Check if the status code is 200 OK
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("http status code %v", resp.StatusCode)
	}
	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	return string(body), nil
}

// hostName, err := getHostnameForRoute(oc, edgeRoute.Name)
// o.Expect(err).NotTo(o.HaveOccurred())
// e2e.Logf("got the host name : %s", hostName)
