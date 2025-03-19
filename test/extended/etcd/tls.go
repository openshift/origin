package etcd

import (
	"context"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	exutil "github.com/openshift/origin/test/extended/util"
)

const tlsTestDuration = 10 * time.Minute
const tlsWaitForCleanupDuration = 10 * time.Minute

var _ = g.Describe("[sig-etcd][Serial] etcd", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLIWithoutNamespace("etcd-tls").AsAdmin()

	g.It("Test TLS 1.3 Configuration", func() {
		ctx, ctxCancelFn := context.WithTimeout(context.Background(), tlsTestDuration)
		defer ctxCancelFn()

		operatorClient := oc.AdminOperatorClient()

		originalEtcd, err := operatorClient.OperatorV1().Etcds().Get(ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		defer func() {
			g.By("Cleanup - removing TLS configuration")

			cleanupCtx, cancelCleanup := context.WithTimeout(context.Background(), tlsWaitForCleanupDuration)
			defer cancelCleanup()

			_, err := operatorClient.OperatorV1().Etcds().Update(cleanupCtx, originalEtcd, metav1.UpdateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			err = waitForEtcdToStabilizeOnTheSameRevision(g.GinkgoT(), oc)
			err = errors.Wrap(err, "timed out waiting for etcd pods to stabilize after cleaning up TLS 1.3")
			o.Expect(err).ToNot(o.HaveOccurred())
		}()

		g.By("Applying a JSON Patch to use TLS 1.3")

		addTLS13Patch := `[{"op":"replace","path":"/spec/observedConfig/servingInfo","value":{"cipherSuites":["TLS_AES_128_GCM_SHA256","TLS_AES_256_GCM_SHA384","TLS_CHACHA20_POLY1305_SHA256"],"minTLSVersion":"VersionTLS13"}}]`

		_, err = operatorClient.OperatorV1().Etcds().Patch(ctx, "cluster", types.JSONPatchType, []byte(addTLS13Patch), metav1.PatchOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Expecting etcd to stabilize without errors")

		err = waitForEtcdToStabilizeOnTheSameRevision(g.GinkgoT(), oc)
		err = errors.Wrap(err, "timed out waiting for etcd pods to stabilize using TLS 1.3")
		o.Expect(err).ToNot(o.HaveOccurred())
	})
})
