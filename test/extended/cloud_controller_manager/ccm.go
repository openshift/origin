package cloud_controller_manager

import (
	"context"
	"fmt"

	"github.com/ghodss/yaml"
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"
	exutil "github.com/openshift/origin/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const cloudControllerNamespace = "openshift-cloud-controller-manager"
const kuberControllerNamespace = "openshift-kube-controller-manager"

var _ = g.Describe("[sig-cloud-provider][Feature:OpenShiftCloudControllerManager][Late]", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLI("ccm")

	g.It("Deploy an external cloud provider [apigroup:machineconfiguration.openshift.io]", func() {
		infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		if !isPlatformExternal(infra.Status.PlatformStatus.Type) {
			g.Skip("Platform does not use external cloud provider")
		}

		g.By("Listing Pods on the openshift-cloud-controller-manager Namespace")
		ccmPods, err := oc.AdminKubeClient().CoreV1().Pods(cloudControllerNamespace).List(context.Background(), metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		g.By("Checking existance of any CCM Pod on the openshift-cloud-controller-manager Namespace")
		o.Expect(len(ccmPods.Items) > 0).Should(o.BeTrue())

		g.By("Getting configMap on the openshift-kube-controller-manager Namespace")
		cm, err := oc.AdminKubeClient().CoreV1().ConfigMaps(kuberControllerNamespace).Get(context.TODO(), "config", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		var observedConfig map[string]interface{}
		err = yaml.Unmarshal([]byte(cm.Data["config.yaml"]), &observedConfig)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Getting the value for the cloud-provider setting in the configMap")
		cloudProvider, found, err := unstructured.NestedSlice(observedConfig, "extendedArguments", "cloud-provider")
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Expecting Kube Controller Manager to not own cloud controller")
		// When cloud-provider setting has "external" as value or it's just empty, KCM does not own Cloud Controllers
		if found && (len(cloudProvider) != 1 || (cloudProvider[0] != "external" && cloudProvider[0] != "")) {
			g.Fail(fmt.Sprintf("Expected cloud-provider %v setting to indicate KCM relinquished cloud ownership", cloudProvider))
		}

		g.By("Getting masters MachineConfig")
		masterkubelet, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("machineconfig/01-master-kubelet", "-o=jsonpath={.spec.config.systemd.units[0].contents}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		g.By("Expecting masters MachineConfig to contain cloud-provider as external for kubelet")
		o.Expect(masterkubelet).To(o.ContainSubstring("cloud-provider=external"))

		g.By("Getting workers MachineConfig")
		workerkubelet, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("machineconfig/01-worker-kubelet", "-o=jsonpath={.spec.config.systemd.units[0].contents}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		g.By("Expecting workers MachineConfig to contain cloud-provider as external for kubelet")
		o.Expect(workerkubelet).To(o.ContainSubstring("cloud-provider=external"))
	})
})

// isPlatformExternal returns true when the platform has an in-tree provider,
// but the platform is expected to use the external provider.
func isPlatformExternal(platformType configv1.PlatformType) bool {
	switch platformType {
	case configv1.AWSPlatformType,
		configv1.AzurePlatformType,
		configv1.OpenStackPlatformType,
		configv1.VSpherePlatformType:
		return true
	default:
		return false
	}
}
