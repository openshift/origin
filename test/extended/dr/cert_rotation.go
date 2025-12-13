package dr

import (
	"context"
	"fmt"
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/openshift/origin/test/extended/prometheus/client"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/pkg/errors"
	"github.com/prometheus/common/model"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"strings"
	"time"
)

var _ = g.Describe("[sig-etcd][Feature:CertRotation][Suite:openshift/etcd/certrotation] etcd", func() {
	defer g.GinkgoRecover()

	ctx := context.TODO()
	oc := exutil.NewCLIWithoutNamespace("etcd-certs").AsAdmin()

	g.BeforeEach(func() {
		// we need to ensure this test always begins with a stable revision for api and etcd
		g.GinkgoT().Log("waiting for api servers to stabilize on the same revision")
		err := waitForApiServerToStabilizeOnTheSameRevisionLonger(g.GinkgoT(), oc)
		err = errors.Wrap(err, "cleanup timed out waiting for APIServer pods to stabilize on the same revision")
		o.Expect(err).ToNot(o.HaveOccurred())

		g.GinkgoT().Log("waiting for etcd to stabilize on the same revision")
		err = waitForEtcdToStabilizeOnTheSameRevisionLonger(g.GinkgoT(), oc)
		err = errors.Wrap(err, "cleanup timed out waiting for etcd pods to stabilize on the same revision")
		o.Expect(err).ToNot(o.HaveOccurred())
	})

	g.It("can manually rotate signer certificates [Timeout:30m]", g.Label("Size:L"), func() {
		kasSecretsClient := oc.AdminKubeClient().CoreV1().Secrets("openshift-kube-apiserver")
		etcdSecretsClient := oc.AdminKubeClient().CoreV1().Secrets("openshift-etcd")

		currentKasClientCert, err := kasSecretsClient.Get(ctx, "etcd-client", v1.GetOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())
		currentEtcdLeafCerts, err := etcdSecretsClient.Get(ctx, "etcd-all-certs", v1.GetOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())

		// as of 4.17, the manual signer rotation is effectively a secret deletion
		// the operator will automatically recreate, rollout and regenerate all leaf certificates
		err = etcdSecretsClient.Delete(ctx, "etcd-signer", v1.DeleteOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())

		g.GinkgoT().Log("waiting for etcd/apiserver to stabilize on the same revision")
		// await all rollouts, then assert the leaf certs all successfully changed
		err = waitForEtcdToStabilizeOnTheSameRevisionLonger(g.GinkgoT(), oc)
		err = errors.Wrap(err, "timed out waiting for etcd pods to stabilize on the same revision")
		o.Expect(err).ToNot(o.HaveOccurred())

		err = waitForApiServerToStabilizeOnTheSameRevisionLonger(g.GinkgoT(), oc)
		err = errors.Wrap(err, "timed out waiting for APIServer pods to stabilize on the same revision")
		o.Expect(err).ToNot(o.HaveOccurred())

		rotatedKasClientCert, err := kasSecretsClient.Get(ctx, "etcd-client", v1.GetOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())
		o.Expect(rotatedKasClientCert.Data).ToNot(o.Equal(currentKasClientCert.Data))

		rotatedEtcdLeafCerts, err := etcdSecretsClient.Get(ctx, "etcd-all-certs", v1.GetOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())
		o.Expect(rotatedEtcdLeafCerts.Data).ToNot(o.Equal(currentEtcdLeafCerts.Data))
	})

	g.It("can manually rotate metrics signer certificates [Timeout:45m]", g.Label("Size:L"), func() {
		etcdSecretsClient := oc.AdminKubeClient().CoreV1().Secrets("openshift-etcd")
		prometheus, err := client.NewE2EPrometheusRouterClient(ctx, oc)
		o.Expect(err).ToNot(o.HaveOccurred())

		currentEtcdMetricCert, err := etcdSecretsClient.Get(ctx, "etcd-metric-client", v1.GetOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())

		// as of 4.17, the manual signer rotation is effectively a secret deletion
		// the operator will automatically recreate, rollout and regenerate all leaf certificates
		err = etcdSecretsClient.Delete(ctx, "etcd-metric-signer", v1.DeleteOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())

		g.GinkgoT().Log("waiting for etcd to stabilize on the same revision")
		// await all rollouts, then assert the leaf client cert has successfully changed
		err = waitForEtcdToStabilizeOnTheSameRevisionLonger(g.GinkgoT(), oc)

		err = errors.Wrap(err, "timed out waiting for etcd pods to stabilize on the same revision")
		o.Expect(err).ToNot(o.HaveOccurred())

		rotatedEtcdMetricsCert, err := etcdSecretsClient.Get(ctx, "etcd-metric-client", v1.GetOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())
		o.Expect(rotatedEtcdMetricsCert.Data).ToNot(o.Equal(currentEtcdMetricCert.Data))

		err = wait.Poll(30*time.Second, 40*time.Minute, func() (bool, error) {
			// ensure that prometheus has etcd metrics again
			result, _, err := prometheus.Query(context.Background(), "sum(up{job=\"etcd\"})", time.Now())
			if err != nil {
				return false, err
			}

			vec, ok := result.(model.Vector)
			if !ok {
				o.Expect(fmt.Errorf("expecting Prometheus query to return a vector, got %s instead", vec.Type())).ToNot(o.HaveOccurred())
			}

			if len(vec) == 0 {
				o.Expect(fmt.Errorf("expecting Prometheus query to return at least one item, got 0 instead")).ToNot(o.HaveOccurred())
			}

			numUpEtcds := int(vec[0].Value)
			g.GinkgoT().Logf("Found [%d] etcds that are up as reported by prometheus...", numUpEtcds)
			return numUpEtcds > 0, nil
		})
		err = errors.Wrap(err, "timed out waiting for metrics to appear again")
		o.Expect(err).ToNot(o.HaveOccurred())
	})

	g.It("can recreate dynamic certificates [Timeout:30m]", g.Label("Size:L"), func() {
		nodes := masterNodes(oc)
		etcdSecretsClient := oc.AdminKubeClient().CoreV1().Secrets("openshift-etcd")

		var currentSecretName string
		var currentSecretData map[string][]byte
		for _, node := range nodes {
			s, err := etcdSecretsClient.Get(context.Background(), fmt.Sprintf("etcd-peer-%s", node.Name), v1.GetOptions{})
			o.Expect(err).ToNot(o.HaveOccurred())
			currentSecretName = s.Name
			currentSecretData = s.Data
			break
		}

		o.Expect(currentSecretData).ToNot(o.BeNil())
		g.GinkgoT().Logf("Deleting secret [%s]...", currentSecretName)
		err := etcdSecretsClient.Delete(ctx, currentSecretName, v1.DeleteOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())

		g.GinkgoT().Log("waiting for the secret to be recreated...")
		err = wait.Poll(30*time.Second, 25*time.Minute, func() (bool, error) {
			_, err := etcdSecretsClient.Get(ctx, currentSecretName, v1.GetOptions{})
			if err != nil {
				return !apierrors.IsNotFound(err), err
			}
			return true, nil
		})
		err = errors.Wrap(err, fmt.Sprintf("timed out waiting for secret [%s] to be recreated by CEO", currentSecretName))
		o.Expect(err).ToNot(o.HaveOccurred())

		g.GinkgoT().Log("waiting for etcd to stabilize on the same revision")
		err = waitForEtcdToStabilizeOnTheSameRevisionLonger(g.GinkgoT(), oc)
		err = errors.Wrap(err, "timed out waiting for etcd pods to stabilize on the same revision")
		o.Expect(err).ToNot(o.HaveOccurred())
	})

	g.It("can recreate trust bundle [Timeout:15m]", g.Label("Size:L"), func() {
		etcdConfigMapClient := oc.AdminKubeClient().CoreV1().ConfigMaps("openshift-etcd")
		bundleName := "etcd-ca-bundle"

		get, err := etcdConfigMapClient.Get(ctx, bundleName, v1.GetOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())

		// it makes little sense to recreate a bundle that only has a single CA in it, so we skip this test when the
		// randomization will run this before any signer rotation.
		// TODO(thomas): we should serialize this with the signer rotation somehow
		if strings.Count(get.Data["ca-bundle.crt"], "BEGIN CERTIFICATE") <= 1 {
			g.Skip("only one CA in the bundle detected, skipping test")
		}

		err = etcdConfigMapClient.Delete(ctx, bundleName, v1.DeleteOptions{})
		err = errors.Wrap(err, "error while deleting etcd CA bundle")
		o.Expect(err).ToNot(o.HaveOccurred())

		g.GinkgoT().Log("waiting for the bundle to be recreated...")
		err = wait.Poll(30*time.Second, 5*time.Minute, func() (bool, error) {
			_, err := etcdConfigMapClient.Get(ctx, bundleName, v1.GetOptions{})
			if err != nil {
				return !apierrors.IsNotFound(err), err
			}
			return true, nil
		})
		err = errors.Wrap(err, "timed out waiting for bundle to be recreated by CEO")
		o.Expect(err).ToNot(o.HaveOccurred())

		g.GinkgoT().Log("waiting for etcd to stabilize on the same revision")
		err = waitForEtcdToStabilizeOnTheSameRevisionLonger(g.GinkgoT(), oc)
		err = errors.Wrap(err, "timed out waiting for etcd pods to stabilize on the same revision")
		o.Expect(err).ToNot(o.HaveOccurred())
	})
})
