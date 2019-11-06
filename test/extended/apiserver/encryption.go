package apiserver

import (
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Feature:EtcdEncrytion] etcd encryption", func() {
	var (
		oc = exutil.NewCLI("cluster-basic-auth", exutil.KubeConfigPath())
	)
	defer g.GinkgoRecover()

	g.It("should be possible to enable etcd encryption without hickups for parallel and later tests", func() {
		apiServerClient := oc.AdminConfigClient().ConfigV1().APIServers()

		err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			apiServer, err := apiServerClient.Get("cluster", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			apiServer.Spec.Encryption.Type = "aescbc"

			_, err = apiServerClient.Update(apiServer)
			return err
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		// intentionally do not wait for any condition. This test does not test the encryption state
		// machine, but makes sure only that encryption has no influence on other tests and system
		// stability.
	})
})