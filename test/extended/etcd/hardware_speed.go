package etcd

import (
	"context"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/pkg/errors"

	"github.com/openshift/library-go/test/library"
	exutil "github.com/openshift/origin/test/extended/util"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = g.Describe("[sig-etcd][Feature:HardwareSpeed][Serial] etcd", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLIWithoutNamespace("etcd-hardware-speed").AsAdmin()

	g.BeforeEach(func() {
		//TODO remove this check once https://github.com/openshift/api/pull/1844 has merged
		if !exutil.IsTechPreviewNoUpgrade(oc) {
			g.Skip("the test is not expected to work within Tech Preview disabled clusters")
		}
	})

	g.AfterEach(func() {
		var err error
		g.GinkgoT().Log("waiting for etcd to stabilize on the same revision")
		err = waitForEtcdToStabilizeOnTheSameRevision(g.GinkgoT(), oc)
		err = errors.Wrap(err, "timed out waiting for etcd pods to stabilize on the same revision")
		o.Expect(err).ToNot(o.HaveOccurred())

		g.GinkgoT().Log("waiting for api servers to stabilize on the same revision")
		err = waitForEtcdToStabilizeOnTheSameRevision(g.GinkgoT(), oc)
		err = errors.Wrap(err, "timed out waiting for api server pods to stabilize on the same revision")
		o.Expect(err).ToNot(o.HaveOccurred())
	})

	// The following test covers a hardware speed change.
	// It starts by changing the hardware speed to Standard.
	// next it validates that the etcd and api servers come back up and are healthy.
	// next it sets the hardware speed to Slower.
	// and validates that the etcd and api servers come back up and are healthy.
	// The test ends by resetting the hardware speed to default.
	// and validates that the etcd and api servers come back up and are healthy.
	g.It("is able to update the hardware speed [Timeout:30m][apigroup:machine.openshift.io]", func(ctx context.Context) {
		// Set the hardware speed to Standard from default ""
		g.GinkgoT().Log("setting hardware speed to Standard")
		data := fmt.Sprintf(`{"spec": {"controlPlaneHardwareSpeed": "Standard"}}`)
		_, err := oc.AdminOperatorClient().OperatorV1().Etcds().Patch(ctx, "cluster", types.MergePatchType, []byte(data), metav1.PatchOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())

		g.GinkgoT().Log("waiting for api servers to stabilize on the same revision")
		err = waitForEtcdToStabilizeOnTheSameRevision(g.GinkgoT(), oc)
		err = errors.Wrap(err, "timed out waiting for etcd pods to stabilize on the same revision")
		o.Expect(err).ToNot(o.HaveOccurred())

		g.GinkgoT().Log("waiting for api servers to stabilize on the same revision")
		err = waitForEtcdToStabilizeOnTheSameRevision(g.GinkgoT(), oc)
		err = errors.Wrap(err, "timed out waiting for api server pods to stabilize on the same revision")
		o.Expect(err).ToNot(o.HaveOccurred())

		// Set the hardware speed to Slower from Standard
		g.GinkgoT().Log("setting hardware speed to Slower")
		data = fmt.Sprintf(`{"spec": {"controlPlaneHardwareSpeed": "Slower"}}`)
		_, err = oc.AdminOperatorClient().OperatorV1().Etcds().Patch(ctx, "cluster", types.MergePatchType, []byte(data), metav1.PatchOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())

		g.GinkgoT().Log("waiting for etcd to stabilize on the same revision")
		err = waitForEtcdToStabilizeOnTheSameRevision(g.GinkgoT(), oc)
		err = errors.Wrap(err, "timed out waiting for etcd pods to stabilize on the same revision")
		o.Expect(err).ToNot(o.HaveOccurred())

		g.GinkgoT().Log("waiting for api servers to stabilize on the same revision")
		err = waitForEtcdToStabilizeOnTheSameRevision(g.GinkgoT(), oc)
		err = errors.Wrap(err, "timed out waiting for api server pods to stabilize on the same revision")
		o.Expect(err).ToNot(o.HaveOccurred())

		// Reset the hardware speed back to default to leave the cluster in the same state.
		g.GinkgoT().Log("resetting hardware speed back to default")
		data = fmt.Sprintf(`{"spec": {"controlPlaneHardwareSpeed": ""}}`)
		_, err = oc.AdminOperatorClient().OperatorV1().Etcds().Patch(ctx, "cluster", types.MergePatchType, []byte(data), metav1.PatchOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())

		g.GinkgoT().Log("waiting for etcd to stabilize on the same revision")
		err = waitForEtcdToStabilizeOnTheSameRevision(g.GinkgoT(), oc)
		err = errors.Wrap(err, "timed out waiting for etcd pods to stabilize on the same revision")
		o.Expect(err).ToNot(o.HaveOccurred())

		g.GinkgoT().Log("waiting for api servers to stabilize on the same revision")
		err = waitForEtcdToStabilizeOnTheSameRevision(g.GinkgoT(), oc)
		err = errors.Wrap(err, "timed out waiting for api server pods to stabilize on the same revision")
		o.Expect(err).ToNot(o.HaveOccurred())
	})
})

func waitForEtcdToStabilizeOnTheSameRevision(t library.LoggingT, oc *exutil.CLI) error {
	podClient := oc.AdminKubeClient().CoreV1().Pods("openshift-etcd")
	return library.WaitForPodsToStabilizeOnTheSameRevision(t, podClient, "app=etcd", 3, 10*time.Second, 5*time.Second, 30*time.Minute)
}

func waitForApiServerToStabilizeOnTheSameRevision(t library.LoggingT, oc *exutil.CLI) error {
	podClient := oc.AdminKubeClient().CoreV1().Pods("openshift-kube-apiserver")
	return library.WaitForPodsToStabilizeOnTheSameRevision(t, podClient, "apiserver=true", 3, 10*time.Second, 5*time.Second, 30*time.Minute)
}
