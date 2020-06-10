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

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	certv1beta1 "k8s.io/api/certificates/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"

	kubeclient "k8s.io/client-go/kubernetes"
	certclientv1beta1 "k8s.io/client-go/kubernetes/typed/certificates/v1beta1"
	restclient "k8s.io/client-go/rest"

	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/ibmcloud"
)

var _ = g.Describe("[sig-cluster-lifecycle]", func() {
	oc := exutil.NewCLI("cluster-client-cert")
	defer g.GinkgoRecover()

	g.It("Pods cannot access the /config/master API endpoint", func() {
		// https://issues.redhat.com/browse/CO-760
		if e2e.TestContext.Provider == ibmcloud.ProviderName {
			e2eskipper.Skipf("IBM ROKS clusters do not expose machine configuration externally because they don't use RHCOS workers. " +
				"Remove this skip when https://issues.redhat.com/browse/CO-760 (RHCOS support) is implemented")
		}

		// the /config/master API port+endpoint is only visible from inside the cluster
		// (-> we need to create a pod to try to reach it) and contains the token
		// of the node-bootstrapper SA, so no random pods should be able to see it
		pod, err := exutil.NewPodExecutor(oc, "get-bootstrap-creds", "quay.io/fedora/fedora:32-x86_64")
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
		curlOutput, err := pod.Exec(fmt.Sprintf("curl -k %s", internalAPI.String()))
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(curlOutput).To(o.ContainSubstring("Connection refused"))
	})

	g.It("CSRs from machines that are not recognized by the cloud provider are not approved", func() {
		if e2e.TestContext.Provider == ibmcloud.ProviderName {
			e2eskipper.Skipf("IBM ROKS clusters do not handle node bootstrapping in the cluster. The openshift-machine-config-operator/node-bootstrapper service account does not exist")
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

		// get the token of the node-bootstrapper and use it to build a client for it
		bootstrapperToken, err := oc.AsAdmin().Run("sa").Args("get-token", "node-bootstrapper", "-n", "openshift-machine-config-operator").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		saClientConfig := restclient.AnonymousClientConfig(oc.AdminConfig())
		saClientConfig.BearerToken = bootstrapperToken

		bootstrapperClient := kubeclient.NewForConfigOrDie(saClientConfig)

		csrName := "node-client-csr"
		bootstrapperClient.CertificatesV1beta1().CertificateSigningRequests().Create(context.Background(), &certv1beta1.CertificateSigningRequest{
			ObjectMeta: metav1.ObjectMeta{
				Name: csrName,
			},
			Spec: certv1beta1.CertificateSigningRequestSpec{
				Request: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrbytes}),
				Usages: []certv1beta1.KeyUsage{
					certv1beta1.UsageDigitalSignature,
					certv1beta1.UsageKeyEncipherment,
					certv1beta1.UsageClientAuth,
				},
			},
		}, metav1.CreateOptions{})

		csrClient := oc.AdminKubeClient().CertificatesV1beta1().CertificateSigningRequests()
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
func waitCSRStatus(csrAdminClient certclientv1beta1.CertificateSigningRequestInterface, csrName string) error {
	return wait.Poll(1*time.Second, 30*time.Second, func() (bool, error) {
		csr, err := csrAdminClient.Get(context.Background(), csrName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		if len(csr.Status.Conditions) > 0 {
			for _, c := range csr.Status.Conditions {
				if c.Type == certv1beta1.CertificateApproved {
					return true, fmt.Errorf("CSR for unknown node should not be approved")
				}
			}
		}
		return false, nil
	})
}

func cleanupCSR(csrAdminClient certclientv1beta1.CertificateSigningRequestInterface, name string) {
	csrAdminClient.Delete(context.Background(), name, metav1.DeleteOptions{})
}
