package etcd

import (
	"context"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/pkg/errors"

	v1 "github.com/openshift/api/config/v1"
	"github.com/openshift/library-go/test/library"
	exutil "github.com/openshift/origin/test/extended/util"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = g.Describe("[sig-etcd][OCPFeatureGate:HardwareSpeed][Serial] etcd", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLIWithoutNamespace("etcd-hardware-speed").AsAdmin()

	g.BeforeEach(func() {
		isSingleNode, err := exutil.IsSingleNode(context.Background(), oc.AdminConfigClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if isSingleNode {
			g.Skip("the test is for etcd peer communication which is not valid for single node")
		}
	})

	g.AfterEach(func(ctx context.Context) {
		var err error

		// Reset the hardware speed back to default to leave the cluster in the same state.
		g.GinkgoT().Log("resetting hardware speed back to default")
		data := fmt.Sprintf(`{"spec": {"controlPlaneHardwareSpeed": ""}}`)
		_, err = oc.AdminOperatorClient().OperatorV1().Etcds().Patch(ctx, "cluster", types.MergePatchType, []byte(data), metav1.PatchOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())

		g.GinkgoT().Log("waiting for etcd to stabilize on the same revision")
		err = waitForEtcdToStabilizeOnTheSameRevision(g.GinkgoT(), oc)
		err = errors.Wrap(err, "timed out waiting for etcd pods to stabilize on the same revision")
		o.Expect(err).ToNot(o.HaveOccurred())

		g.GinkgoT().Log("ensuring all etcd servers are running with default hardware speed values")
		err = ensureHardwareSpeedForAllEtcds(ctx, oc, "")
		o.Expect(err).ToNot(o.HaveOccurred())
	})

	// The following test covers a hardware speed change to the Standard profile.
	// It starts by changing the hardware speed to Standard.
	// next it ensures that the etcd servers come back up and are healthy.
	// next it validates that the etcd servers were started with the expected environment variables.
	g.It("is able to set the hardware speed to Standard [Timeout:30m][apigroup:machine.openshift.io]", g.Label("Size:L"), func(ctx context.Context) {
		// Set the hardware speed to Standard from default ""
		g.GinkgoT().Log("setting hardware speed to Standard")
		data := fmt.Sprintf(`{"spec": {"controlPlaneHardwareSpeed": "Standard"}}`)
		_, err := oc.AdminOperatorClient().OperatorV1().Etcds().Patch(ctx, "cluster", types.MergePatchType, []byte(data), metav1.PatchOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())

		g.GinkgoT().Log("waiting for api servers to stabilize on the same revision")
		err = waitForEtcdToStabilizeOnTheSameRevision(g.GinkgoT(), oc)
		err = errors.Wrap(err, "timed out waiting for etcd pods to stabilize on the same revision")
		o.Expect(err).ToNot(o.HaveOccurred())

		g.GinkgoT().Log("ensuring all etcd servers are running with standard hardware speed values")
		err = ensureHardwareSpeedForAllEtcds(ctx, oc, "Standard")
		o.Expect(err).ToNot(o.HaveOccurred())
	})

	// The following test covers a hardware speed change to the Slower profile.
	// It starts by changing the hardware speed to Profile.
	// next it ensures that the etcd servers come back up and are healthy.
	// next it validates that the etcd servers were started with the expected environment variables.
	g.It("is able to set the hardware speed to Slower [Timeout:30m][apigroup:machine.openshift.io]", g.Label("Size:L"), func(ctx context.Context) {
		g.GinkgoT().Log("setting hardware speed to Slower")
		data := fmt.Sprintf(`{"spec": {"controlPlaneHardwareSpeed": "Slower"}}`)
		_, err := oc.AdminOperatorClient().OperatorV1().Etcds().Patch(ctx, "cluster", types.MergePatchType, []byte(data), metav1.PatchOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())

		g.GinkgoT().Log("waiting for etcd to stabilize on the same revision")
		err = waitForEtcdToStabilizeOnTheSameRevision(g.GinkgoT(), oc)
		err = errors.Wrap(err, "timed out waiting for etcd pods to stabilize on the same revision")
		o.Expect(err).ToNot(o.HaveOccurred())

		g.GinkgoT().Log("ensuring all etcd servers are running with slower hardware speed values")
		err = ensureHardwareSpeedForAllEtcds(ctx, oc, "Slower")
		o.Expect(err).ToNot(o.HaveOccurred())
	})

	// The following test covers a hardware speed change to the default profile.
	// It starts by changing the hardware speed to "".
	// next it ensures that the etcd servers come back up and are healthy.
	// next it validates that the etcd servers were started with the expected environment variables.
	g.It("is able to set the hardware speed to \"\" [Timeout:30m][apigroup:machine.openshift.io]", g.Label("Size:L"), func(ctx context.Context) {
		g.GinkgoT().Log("setting hardware speed to \"\"")
		data := fmt.Sprintf(`{"spec": {"controlPlaneHardwareSpeed": ""}}`)
		_, err := oc.AdminOperatorClient().OperatorV1().Etcds().Patch(ctx, "cluster", types.MergePatchType, []byte(data), metav1.PatchOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())

		g.GinkgoT().Log("waiting for etcd to stabilize on the same revision")
		err = waitForEtcdToStabilizeOnTheSameRevision(g.GinkgoT(), oc)
		err = errors.Wrap(err, "timed out waiting for etcd pods to stabilize on the same revision")
		o.Expect(err).ToNot(o.HaveOccurred())

		g.GinkgoT().Log("ensuring all etcd servers are running with default hardware speed values")
		err = ensureHardwareSpeedForAllEtcds(ctx, oc, "")
		o.Expect(err).ToNot(o.HaveOccurred())
	})
})

func waitForEtcdToStabilizeOnTheSameRevision(t library.LoggingT, oc *exutil.CLI) error {
	podClient := oc.AdminKubeClient().CoreV1().Pods("openshift-etcd")
	return library.WaitForPodsToStabilizeOnTheSameRevision(t, podClient, "app=etcd", 5, 24*time.Second, 5*time.Second, 30*time.Minute)
}

func expectedStandardHardwareSpeed() map[string]string {
	return map[string]string{
		"ETCD_HEARTBEAT_INTERVAL": "100",
		"ETCD_ELECTION_TIMEOUT":   "1000",
	}
}

func expectedSlowerHardwareSpeed() map[string]string {
	return map[string]string{
		"ETCD_HEARTBEAT_INTERVAL": "500",
		"ETCD_ELECTION_TIMEOUT":   "2500",
	}
}

func ensureHardwareSpeedForAllEtcds(ctx context.Context, oc *exutil.CLI, expectedHwSpeed string) error {
	var m map[string]string

	switch expectedHwSpeed {
	case "Standard":
		m = expectedStandardHardwareSpeed()
	case "Slower":
		m = expectedSlowerHardwareSpeed()
	case "":
		m = expectedStandardHardwareSpeed()
		// Some platforms have different default hardware speed for backward compatibility.
		infrastructure, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(ctx, "cluster", metav1.GetOptions{})
		if err != nil {
			err = errors.Wrap(err, "failed to get infrastructure")
			return err
		}
		if status := infrastructure.Status.PlatformStatus; status != nil {
			switch {
			case status.Azure != nil:
				m = expectedSlowerHardwareSpeed()
			case status.IBMCloud != nil:
				if infrastructure.Status.PlatformStatus.IBMCloud.ProviderType == v1.IBMCloudProviderTypeVPC {
					m = expectedSlowerHardwareSpeed()
				}
			}
		}
	default:
		return fmt.Errorf("invalid hardware speed %v", expectedHwSpeed)
	}

	etcds, err := oc.AdminKubeClient().CoreV1().Pods("openshift-etcd").List(ctx, metav1.ListOptions{})
	if err != nil {
		err = errors.Wrap(err, "failed to list etcd pods")
		return err
	}
	for _, etcd := range etcds.Items {
		for _, c := range etcd.Spec.Containers {
			if c.Name != "etcd" {
				continue
			}
			if err := ensureEnvVarValues(c.Env, m); err != nil {
				err = errors.Wrapf(err, "in etcd pod %s", etcd.Name)
				return err
			}
		}
	}
	return nil
}

func ensureEnvVarValues(env []corev1.EnvVar, expected map[string]string) error {
	for _, envVar := range env {
		if expectedValue, has := expected[envVar.Name]; has {
			if envVar.Value != expectedValue {
				return fmt.Errorf("expected %s to be %s, got %s", envVar.Name, expectedValue, envVar.Value)
			}
			delete(expected, envVar.Name)
		}
	}

	if len(expected) > 0 {
		missing := make([]string, 0, len(expected))
		for k, v := range expected {
			missing = append(missing, fmt.Sprintf("%s=%s", k, v))
		}
		return fmt.Errorf("missing expected env vars: [%s]", strings.Join(missing, ", "))
	}

	return nil
}
