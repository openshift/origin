package networking

import (
	"context"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"
	exutil "github.com/openshift/origin/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	admissionapi "k8s.io/pod-security-admission/api"
)

var _ = g.Describe("[sig-network] load balancer", func() {
	defer g.GinkgoRecover()

	var (
		oc = exutil.NewCLIWithPodSecurityLevel("load-balancer", admissionapi.LevelPrivileged)
	)

	g.It("should be managed by OpenShift", g.Label("Size:S"), func() {
		infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to get cluster-wide infrastructure")

		if infra.Status.PlatformStatus == nil {
			e2e.Logf("Load balancer tests are not supported on this platform, skipping test")
			return
		}

		if !platformSupportLB(infra.Status.PlatformStatus.Type) {
			e2e.Logf("Load balancer tests are not supported on this platform, skipping test")
			return
		}

		if !isOpenShiftManagedDefaultLB(*infra) {
			e2e.Logf("Load balancer is not managed by OpenShift, skipping test")
			return
		}

		g.By("checking that Keepalived and HAproxy pods are deployed")
		o.Expect(foundLBPods(infra, oc)).To(o.BeTrue(), "Keepalived and HAproxy pods are not deployed and should be")
	})

	g.It("should not be managed by OpenShift", g.Label("Size:S"), func() {
		infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to get cluster-wide infrastructure")

		if infra.Status.PlatformStatus == nil {
			e2e.Logf("Load balancer tests are not supported on this platform, skipping test")
			return
		}

		if !platformSupportLB(infra.Status.PlatformStatus.Type) {
			e2e.Logf("Load balancer tests are not supported on this platform, skipping test")
			return
		}

		if isOpenShiftManagedDefaultLB(*infra) {
			e2e.Logf("Load balancer is managed by OpenShift, skipping test")
			return
		}

		g.By("checking that Keepalived and HAproxy pods are not deployed")
		o.Expect(foundLBPods(infra, oc)).NotTo(o.BeTrue(), "Keepalived and HAproxy pods are deployed and should not be")
	})
})

func foundLBPods(infra *configv1.Infrastructure, oc *exutil.CLI) bool {
	namespace := "openshift-" + onPremPlatformShortName(*infra) + "-infra"
	pods, err := getPodsByNamespace(oc, namespace)
	expectNoError(err)
	foundPod := false
	for _, pod := range pods {
		keepalivedAppLabel := onPremPlatformShortName(*infra) + "-infra-vrrp"
		if pod.Labels["app"] == keepalivedAppLabel {
			foundPod = true
		}

		haproxyAppLabel := onPremPlatformShortName(*infra) + "-infra-api-lb"
		if pod.Labels["app"] == haproxyAppLabel {
			foundPod = true
		}
	}
	return foundPod
}

func getPodsByNamespace(oc *exutil.CLI, namespace string) ([]corev1.Pod, error) {
	pods, err := oc.AdminKubeClient().CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return pods.Items, nil
}

func platformSupportLB(platformType configv1.PlatformType) bool {
	switch platformType {
	case configv1.BareMetalPlatformType, configv1.NutanixPlatformType, configv1.OpenStackPlatformType, configv1.OvirtPlatformType, configv1.VSpherePlatformType:
		return true
	default:
		return false
	}
}

func isOpenShiftManagedDefaultLB(infra configv1.Infrastructure) bool {
	platformType := infra.Status.PlatformStatus.Type
	lbType := configv1.LoadBalancerTypeOpenShiftManagedDefault
	switch platformType {
	case configv1.BareMetalPlatformType:
		if infra.Status.PlatformStatus.BareMetal != nil {
			if infra.Status.PlatformStatus.BareMetal.LoadBalancer != nil {
				lbType = infra.Status.PlatformStatus.BareMetal.LoadBalancer.Type
			}
			return lbType == configv1.LoadBalancerTypeOpenShiftManagedDefault
		}
	case configv1.OvirtPlatformType:
		if infra.Status.PlatformStatus.Ovirt != nil {
			if infra.Status.PlatformStatus.Ovirt.LoadBalancer != nil {
				lbType = infra.Status.PlatformStatus.Ovirt.LoadBalancer.Type
			}
			return lbType == configv1.LoadBalancerTypeOpenShiftManagedDefault
		}
	case configv1.OpenStackPlatformType:
		if infra.Status.PlatformStatus.OpenStack != nil {
			if infra.Status.PlatformStatus.OpenStack.LoadBalancer != nil {
				lbType = infra.Status.PlatformStatus.OpenStack.LoadBalancer.Type
			}
			return lbType == configv1.LoadBalancerTypeOpenShiftManagedDefault
		}
	case configv1.VSpherePlatformType:
		if infra.Status.PlatformStatus.VSphere != nil {
			// vSphere allows to use a user managed load balancer by not setting the VIPs in PlatformStatus.
			// We will maintain backward compatibility by checking if the VIPs are not set, we will
			// not deploy HAproxy, Keepalived and CoreDNS.
			if len(infra.Status.PlatformStatus.VSphere.APIServerInternalIPs) == 0 {
				return false
			}
			if infra.Status.PlatformStatus.VSphere.LoadBalancer != nil {
				lbType = infra.Status.PlatformStatus.VSphere.LoadBalancer.Type
			}
			return lbType == configv1.LoadBalancerTypeOpenShiftManagedDefault
		}
		return false
	case configv1.NutanixPlatformType:
		if infra.Status.PlatformStatus.Nutanix != nil {
			if infra.Status.PlatformStatus.Nutanix.LoadBalancer != nil {
				lbType = infra.Status.PlatformStatus.Nutanix.LoadBalancer.Type
			}
			return lbType == configv1.LoadBalancerTypeOpenShiftManagedDefault
		}
	default:
		// If a new on-prem platform is newly supported, the default value of LoadBalancerType is internal.
		return false
	}
	return false
}

func onPremPlatformShortName(infra configv1.Infrastructure) string {
	if infra.Status.PlatformStatus != nil {
		switch infra.Status.PlatformStatus.Type {
		case configv1.BareMetalPlatformType:
			return "kni"
		case configv1.OvirtPlatformType:
			return "ovirt"
		case configv1.OpenStackPlatformType:
			return "openstack"
		case configv1.VSpherePlatformType:
			return "vsphere"
		case configv1.NutanixPlatformType:
			return "nutanix"
		default:
			return ""
		}
	} else {
		return ""
	}
}
