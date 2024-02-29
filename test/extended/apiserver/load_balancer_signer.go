package apiserver

import (
	"context"
	"fmt"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/cert"
)

var _ = g.Describe("[sig-api-machinery][Late][Feature:APIServer]", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithoutNamespace("lb-check")

	g.It("load-balancer signing certificate should be the installer created one", func() {
		ctx := context.Background()
		kubeClient := oc.AdminKubeClient()
		lbSignerSecret, err := kubeClient.CoreV1().Secrets("openshift-kube-apiserver-operator").Get(ctx, "loadbalancer-serving-signer", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "couldn't get signer secret")

		currSignerCert := lbSignerSecret.Data["tls.crt"]
		certificates, err := cert.ParseCertsPEM(currSignerCert)
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to parse public key")

		for i, currCert := range certificates {
			if currCert.Subject.CommonName != "kube-apiserver-lb-signer" {
				g.Fail(fmt.Sprintf("load balancer signing certificate %d was recreated as: %v", i, currCert.Subject.CommonName))
			}
		}
	})
})
