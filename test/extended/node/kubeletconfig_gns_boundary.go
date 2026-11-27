package node

import (
	"context"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/kubernetes/test/e2e/framework"

	machineconfigv1 "github.com/openshift/api/machineconfiguration/v1"
	mcclient "github.com/openshift/client-go/machineconfiguration/clientset/versioned"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-node][Suite:openshift/conformance/parallel] KubeletConfig graceful node shutdown boundaries", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithoutNamespace("kubeletconfig-gns-boundary")

	g.DescribeTable("should reject platform-managed graceful node shutdown configuration",
		func(ctx context.Context, config string) {
			SkipOnHyperShift(ctx, oc)

			mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
			o.Expect(err).NotTo(o.HaveOccurred(), "creating machine config client")

			selectorKey := "e2e.openshift.io/gns-boundary-" + strings.ToLower(utilrand.String(8))
			selectorValue := "no-match"
			pools, err := mcClient.MachineconfigurationV1().MachineConfigPools().List(ctx, metav1.ListOptions{})
			o.Expect(err).NotTo(o.HaveOccurred(), "listing MachineConfigPools")
			for _, pool := range pools.Items {
				o.Expect(pool.Labels[selectorKey]).NotTo(o.Equal(selectorValue), "selector must not match MachineConfigPool %q", pool.Name)
			}

			kcName := "gns-boundary-" + strings.ToLower(utilrand.String(12))
			kc := &machineconfigv1.KubeletConfig{
				ObjectMeta: metav1.ObjectMeta{Name: kcName},
				Spec: machineconfigv1.KubeletConfigSpec{
					MachineConfigPoolSelector: &metav1.LabelSelector{MatchLabels: map[string]string{selectorKey: selectorValue}},
					KubeletConfig:             &runtime.RawExtension{Raw: []byte(config)},
				},
			}
			g.DeferCleanup(func() {
				if err := CleanupKubeletConfig(context.Background(), mcClient, kcName, ""); err != nil {
					framework.Logf("cleanup failed for KubeletConfig %s: %v", kcName, err)
				}
			})

			_, err = CreateKubeletConfig(ctx, mcClient, kc)
			o.Expect(err).NotTo(o.HaveOccurred(), "creating KubeletConfig")

			err = wait.Poll(2*time.Second, 30*time.Second, func() (bool, error) {
				current, err := mcClient.MachineconfigurationV1().KubeletConfigs().Get(ctx, kcName, metav1.GetOptions{})
				if err != nil {
					return false, err
				}
				if current.Status.ObservedGeneration != current.Generation {
					return false, nil
				}
				for _, condition := range current.Status.Conditions {
					if condition.Type == machineconfigv1.KubeletConfigAccepted && condition.Status == corev1.ConditionFalse {
						return strings.Contains(condition.Message, "featureGates is not allowed to be set"), nil
					}
				}
				return false, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred(), "expected Accepted=False with a featureGates rejection")
		},
		g.Entry("shutdownGracePeriod with GracefulNodeShutdown", `{"shutdownGracePeriod":"30s","featureGates":{"GracefulNodeShutdown":true}}`),
		g.Entry("shutdownGracePeriodCriticalPods with GracefulNodeShutdown", `{"shutdownGracePeriodCriticalPods":"10s","featureGates":{"GracefulNodeShutdown":true}}`),
		g.Entry("shutdownGracePeriodByPodPriority with GracefulNodeShutdownBasedOnPodPriority", `{"shutdownGracePeriodByPodPriority":[{"priority":0,"shutdownGracePeriodSeconds":30}],"featureGates":{"GracefulNodeShutdownBasedOnPodPriority":true}}`),
		g.Entry("GracefulNodeShutdown feature gate", `{"featureGates":{"GracefulNodeShutdown":true}}`),
		g.Entry("GracefulNodeShutdownBasedOnPodPriority feature gate", `{"featureGates":{"GracefulNodeShutdownBasedOnPodPriority":true}}`),
	)
})
