package dr

import (
	"context"
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"math/rand"
	"strings"
	"time"
)

var _ = g.Describe("[sig-etcd][Feature:CertRotation][Suite:openshift/etcd/recovery] etcd", func() {
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

	g.It("can manually rotate signer certificates [Timeout:30m]", func() {
		kasSecretsClient := oc.AdminKubeClient().CoreV1().Secrets("openshift-kube-apiserver")
		etcdSecretsClient := oc.AdminKubeClient().CoreV1().Secrets("openshift-etcd")
		configSecretsClient := oc.AdminKubeClient().CoreV1().Secrets("openshift-config")

		currentKasClientCert, err := kasSecretsClient.Get(ctx, "etcd-client", v1.GetOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())
		currentEtcdLeafCerts, err := etcdSecretsClient.Get(ctx, "etcd-all-certs", v1.GetOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())

		// as of 4.16, the manual signer rotation is effectively a secret copy, similar to below OC command:
		// $ oc get secret etcd-signer -n openshift-etcd -ojson | \
		// jq 'del(.metadata["namespace","creationTimestamp","resourceVersion","selfLink","uid"])' | \
		// oc apply -n openshift-config -f -
		newSigner, err := etcdSecretsClient.Get(ctx, "etcd-signer", v1.GetOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())

		newSigner.ObjectMeta = v1.ObjectMeta{Name: "etcd-signer", Namespace: "openshift-config"}
		_, err = configSecretsClient.Update(ctx, newSigner, v1.UpdateOptions{})
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

	g.It("can manually rotate metrics signer certificates [Timeout:30m]", func() {
		etcdSecretsClient := oc.AdminKubeClient().CoreV1().Secrets("openshift-etcd")
		configSecretsClient := oc.AdminKubeClient().CoreV1().Secrets("openshift-config")

		currentEtcdMetricCert, err := etcdSecretsClient.Get(ctx, "etcd-metric-client", v1.GetOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())

		// as of 4.16, the manual signer rotation is effectively a secret copy, similar to below OC command:
		// $ oc get secret etcd-metrics-signer -n openshift-etcd -ojson | \
		// jq 'del(.metadata["namespace","creationTimestamp","resourceVersion","selfLink","uid"])' | \
		// oc apply -n openshift-config -f -
		newSigner, err := etcdSecretsClient.Get(ctx, "etcd-metric-signer", v1.GetOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())

		newSigner.ObjectMeta = v1.ObjectMeta{Name: "etcd-metric-signer", Namespace: "openshift-config"}
		_, err = configSecretsClient.Update(ctx, newSigner, v1.UpdateOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())

		g.GinkgoT().Log("waiting for etcd to stabilize on the same revision")
		// await all rollouts, then assert the leaf certs all successfully changed
		err = waitForEtcdToStabilizeOnTheSameRevisionLonger(g.GinkgoT(), oc)

		err = errors.Wrap(err, "timed out waiting for etcd pods to stabilize on the same revision")
		o.Expect(err).ToNot(o.HaveOccurred())

		rotatedEtcdMetricsCert, err := etcdSecretsClient.Get(ctx, "etcd-metric-client", v1.GetOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())
		o.Expect(rotatedEtcdMetricsCert.Data).ToNot(o.Equal(currentEtcdMetricCert.Data))
		// TODO check whether we still have prometheus metrics
	})

	g.It("can recreate dynamic certificates [Timeout:15m]", func() {
		etcdSecretsClient := oc.AdminKubeClient().CoreV1().Secrets("openshift-etcd")

		allEtcdSecrets, err := etcdSecretsClient.List(ctx, v1.ListOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())

		// we pick any peer cert at random and delete it
		rand.Shuffle(len(allEtcdSecrets.Items), func(i, j int) {
			allEtcdSecrets.Items[i], allEtcdSecrets.Items[j] = allEtcdSecrets.Items[j], allEtcdSecrets.Items[i]
		})

		var currentSecretName string
		var currentSecretData map[string][]byte
		for _, item := range allEtcdSecrets.Items {
			if strings.Contains(item.Name, "etcd-peer") {
				currentSecretName = item.Name
				currentSecretData = item.Data
				g.GinkgoT().Logf("Deleting secret %s...", currentSecretName)
				err = etcdSecretsClient.Delete(ctx, item.Name, v1.DeleteOptions{})
				o.Expect(err).ToNot(o.HaveOccurred())
				break
			}
		}

		o.Expect(currentSecretData).ToNot(o.BeNil())

		g.GinkgoT().Log("waiting for the secret to be recreated...")
		err = wait.Poll(30*time.Second, 5*time.Minute, func() (bool, error) {
			_, err := etcdSecretsClient.Get(ctx, currentSecretName, v1.GetOptions{})
			if err != nil {
				return !apierrors.IsNotFound(err), err
			}
			return true, nil
		})
		err = errors.Wrap(err, "timed out waiting for secret to be recreated by CEO")
		o.Expect(err).ToNot(o.HaveOccurred())

		g.GinkgoT().Log("waiting for etcd to stabilize on the same revision")
		// await all rollouts, then assert the leaf certs all successfully changed
		err = waitForEtcdToStabilizeOnTheSameRevisionLonger(g.GinkgoT(), oc)
		err = errors.Wrap(err, "timed out waiting for etcd pods to stabilize on the same revision")
		o.Expect(err).ToNot(o.HaveOccurred())
	})

	g.It("can recreate trust bundle [Timeout:15m]", func() {
		etcdConfigMapClient := oc.AdminKubeClient().CoreV1().ConfigMaps("openshift-etcd")
		bundleName := "etcd-ca-bundle"

		err := etcdConfigMapClient.Delete(ctx, bundleName, v1.DeleteOptions{})
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
		// await all rollouts, then assert the leaf certs all successfully changed
		err = waitForEtcdToStabilizeOnTheSameRevisionLonger(g.GinkgoT(), oc)
		err = errors.Wrap(err, "timed out waiting for etcd pods to stabilize on the same revision")
		o.Expect(err).ToNot(o.HaveOccurred())
	})
})
