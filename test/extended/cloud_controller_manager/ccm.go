package cloud_controller_manager

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ghodss/yaml"
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"
	exutil "github.com/openshift/origin/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

const cloudControllerNamespace = "openshift-cloud-controller-manager"
const kuberControllerNamespace = "openshift-kube-controller-manager"

type SimpleSystemdUnit struct {
	Contents string
	Name     string
}

var _ = g.Describe("[sig-cloud-provider][Feature:OpenShiftCloudControllerManager][Late]", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLI("ccm")

	g.It("Deploy an external cloud provider [apigroup:machineconfiguration.openshift.io]", g.Label("Size:M"), func() {
		infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		if infra.Status.ControlPlaneTopology == configv1.ExternalTopologyMode {
			g.Skip("Control plane is external")
		}

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

		var systemdUnits []SimpleSystemdUnit

		g.By("Getting masters MachineConfig")
		masterkubelets, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("machineconfig/01-master-kubelet", "-o=jsonpath={.spec.config.systemd.units}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		g.By("Expecting masters MachineConfig to contain cloud-provider as external for kubelet")
		err = json.Unmarshal([]byte(masterkubelets), &systemdUnits)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(systemdUnits).To(
			o.ContainElement(o.SatisfyAll(
				o.HaveField("Name", o.Equal("kubelet.service")),
				o.HaveField("Contents", o.ContainSubstring("cloud-provider=external")),
			)))

		g.By("Getting workers MachineConfig")
		workerkubelets, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("machineconfig/01-worker-kubelet", "-o=jsonpath={.spec.config.systemd.units}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		g.By("Expecting workers MachineConfig to contain cloud-provider as external for kubelet")
		err = json.Unmarshal([]byte(workerkubelets), &systemdUnits)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(systemdUnits).To(
			o.ContainElement(o.SatisfyAll(
				o.HaveField("Name", o.Equal("kubelet.service")),
				o.HaveField("Contents", o.ContainSubstring("cloud-provider=external")),
			)))
	})

	g.It("Cluster scoped load balancer healthcheck port and path should be 10256/healthz", g.Label("Size:M"), func() {
		exutil.SkipIfNotPlatform(oc, "AWS")
		if strings.HasPrefix(exutil.GetClusterRegion(oc), "us-iso") {
			g.Skip("Skipped: There is no public subnet on AWS C2S/SC2S disconnected clusters!")
		}

		g.By("Create a cluster scope load balancer")
		svcName := "test-lb"
		defer oc.WithoutNamespace().AsAdmin().Run("delete").Args("-n", oc.Namespace(), "service", "loadbalancer", svcName, "--ignore-not-found").Execute()
		out, err := oc.AsAdmin().WithoutNamespace().Run("create").Args("-n", oc.Namespace(), "service", "loadbalancer", svcName, "--tcp=80:8080").Output()
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to create lb service")
		o.Expect(out).To(o.ContainSubstring("service/" + svcName + " created"))

		g.By("Check External-IP assigned")
		svcExternalIP := getLoadBalancerExternalIP(oc, oc.Namespace(), svcName)
		e2e.Logf("External IP assigned: %s", svcExternalIP)
		o.Expect(svcExternalIP).NotTo(o.BeEmpty(), "externalIP should not be empty")
		lbName := strings.Split(svcExternalIP, "-")[0]

		g.By("Check healthcheck port and path should be 10256/healthz")
		healthCheckPort := "10256"
		healthCheckPath := "/healthz"
		exutil.GetAwsCredentialFromCluster(oc)
		region := exutil.GetClusterRegion(oc)
		sess := exutil.InitAwsSession(region)
		elbClient := exutil.NewELBClient(sess)
		healthCheck, err := elbClient.GetLBHealthCheckPortPath(lbName)
		o.Expect(err).NotTo(o.HaveOccurred(), "unable to get health check port and path")
		e2e.Logf("Health check port and path: %v", healthCheck)
		o.Expect(healthCheck).To(o.Equal(fmt.Sprintf("HTTP:%s%s", healthCheckPort, healthCheckPath)))
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

// getLoadBalancerExternalIP get IP address of LB service
func getLoadBalancerExternalIP(oc *exutil.CLI, namespace string, svcName string) string {
	var svcExternalIP string
	var cmdErr error
	checkErr := wait.Poll(5*time.Second, 300*time.Second, func() (bool, error) {
		svcExternalIP, cmdErr = oc.AsAdmin().WithoutNamespace().Run("get").Args("service", "-n", namespace, svcName, "-o=jsonpath={.status.loadBalancer.ingress[0].hostname}").Output()
		if svcExternalIP == "" || cmdErr != nil {
			e2e.Logf("Waiting for lb service IP assignment. Trying again...")
			return false, nil
		}
		return true, nil
	})
	o.Expect(checkErr).NotTo(o.HaveOccurred())
	return svcExternalIP
}
