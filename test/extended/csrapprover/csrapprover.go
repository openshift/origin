package csrapprover

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"

	authenticationv1 "k8s.io/api/authentication/v1"
	certv1 "k8s.io/api/certificates/v1"
	kubeclient "k8s.io/client-go/kubernetes"
	certclientv1 "k8s.io/client-go/kubernetes/typed/certificates/v1"
	restclient "k8s.io/client-go/rest"
	admissionapi "k8s.io/pod-security-admission/api"

	configv1 "github.com/openshift/api/config/v1"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/image"
)

var _ = g.Describe("[sig-cluster-lifecycle]", func() {
	oc := exutil.NewCLIWithPodSecurityLevel("cluster-client-cert", admissionapi.LevelBaseline)
	defer g.GinkgoRecover()

	g.It("Pods cannot access the /config/master API endpoint", g.Label("Size:M"), func() {
		controlPlaneTopology, err := exutil.GetControlPlaneTopology(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		// https://issues.redhat.com/browse/CO-760
		if *controlPlaneTopology == configv1.ExternalTopologyMode {
			e2eskipper.Skipf("External clusters do not expose machine configuration externally because they don't use RHCOS workers. " +
				"Remove this skip when https://issues.redhat.com/browse/CO-760 (RHCOS support) is implemented")
		}

		// the /config/master API port+endpoint is only visible from inside the cluster
		// (-> we need to create a pod to try to reach it) and contains the token
		// of the node-bootstrapper SA, so no random pods should be able to see it
		pod, err := exutil.NewPodExecutor(oc, "get-bootstrap-creds", image.ShellImage())
		o.Expect(err).NotTo(o.HaveOccurred())

		// get the API server URL, mutate to internal API (use infra.Status.APIServerURLInternal) once API is bumped
		infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		internalAPI, err := url.Parse(infra.Status.APIServerURL)
		o.Expect(err).NotTo(o.HaveOccurred())
		internalAPI.Host = strings.Replace(internalAPI.Host, "api.", "api-int.", 1)

		host, _, err := net.SplitHostPort(internalAPI.Host)
		o.Expect(err).ToNot(o.HaveOccurred())

		internalAPI.Host = net.JoinHostPort(host, "22623")
		internalAPI.Path = "/config/master"

		// we should not be able to reach the endpoint
		curlOutput, err := pod.Exec(fmt.Sprintf("curl --connect-timeout 5 -k %s", internalAPI.String()))
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(curlOutput).To(o.Or(o.ContainSubstring("Connection refused"), o.ContainSubstring("Connection timed out")))
	})

	g.It("CSRs from machines that are not recognized by the cloud provider are not approved", g.Label("Size:S"), func() {
		controlPlaneTopology, err := exutil.GetControlPlaneTopology(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		if *controlPlaneTopology == configv1.ExternalTopologyMode {
			e2eskipper.Skipf("External clusters do not handle node bootstrapping in the cluster. The openshift-machine-config-operator/node-bootstrapper service account does not exist")
		}

		// we somehow were able to get the node-approver token, make sure we can't
		// create node certs with client auth with it
		priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		o.Expect(err).NotTo(o.HaveOccurred())

		certRequestTemplate := &x509.CertificateRequest{
			SignatureAlgorithm: x509.ECDSAWithSHA256,
			Subject: pkix.Name{
				CommonName:   "system:node:hacking-node.ec2.internal",
				Organization: []string{"system:nodes"},
			},
			PublicKey: priv.PublicKey,
		}

		csrbytes, err := x509.CreateCertificateRequest(rand.Reader, certRequestTemplate, priv)
		o.Expect(err).NotTo(o.HaveOccurred())

		// create a new token request for node-bootstrapper service account and use it to build a client for it
		tokenRequest := &authenticationv1.TokenRequest{
			Spec: authenticationv1.TokenRequestSpec{
				Audiences: []string{"https://kubernetes.default.svc"},
			},
		}

		bootstrapperToken, err := oc.AdminKubeClient().CoreV1().ServiceAccounts("openshift-machine-config-operator").CreateToken(context.TODO(), "node-bootstrapper", tokenRequest, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		saClientConfig := restclient.AnonymousClientConfig(oc.AdminConfig())
		saClientConfig.BearerToken = bootstrapperToken.Status.Token

		bootstrapperClient := kubeclient.NewForConfigOrDie(saClientConfig)

		csrName := "node-client-csr"
		_, err = bootstrapperClient.CertificatesV1().CertificateSigningRequests().Create(context.Background(), &certv1.CertificateSigningRequest{
			ObjectMeta: metav1.ObjectMeta{
				Name: csrName,
			},
			Spec: certv1.CertificateSigningRequestSpec{
				SignerName: "kubernetes.io/kubelet-serving",
				Request:    pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrbytes}),
				Usages: []certv1.KeyUsage{
					certv1.UsageDigitalSignature,
					certv1.UsageKeyEncipherment,
					certv1.UsageClientAuth,
				},
			},
		}, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		csrClient := oc.AdminKubeClient().CertificatesV1().CertificateSigningRequests()
		defer cleanupCSR(csrClient, csrName)

		err = waitCSRStatus(csrClient, csrName)
		// if status did not change in 30 sec, the CSR is still in pending
		// which is fine as the machine-approver does not deny
		if err != wait.ErrWaitTimeout {
			o.Expect(err).NotTo(o.HaveOccurred())
		}
	})
})

// waits for the CSR object to change status, checks that it did not get approved
func waitCSRStatus(csrAdminClient certclientv1.CertificateSigningRequestInterface, csrName string) error {
	return wait.Poll(1*time.Second, 30*time.Second, func() (bool, error) {
		csr, err := csrAdminClient.Get(context.Background(), csrName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		if len(csr.Status.Conditions) > 0 {
			for _, c := range csr.Status.Conditions {
				if c.Type == certv1.CertificateApproved {
					return true, fmt.Errorf("CSR for unknown node should not be approved")
				}
			}
		}
		return false, nil
	})
}

func cleanupCSR(csrAdminClient certclientv1.CertificateSigningRequestInterface, name string) {
	csrAdminClient.Delete(context.Background(), name, metav1.DeleteOptions{})
}
