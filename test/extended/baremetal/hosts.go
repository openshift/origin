package baremetal

import (
	"context"
	"fmt"

	metal3v1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/origin/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = g.Describe("[sig-installer][Feature:baremetal][apigroup:metal3.io] Baremetal/OpenStack/vSphere/None/AWS/Azure/GCP platforms", func() {
	defer g.GinkgoRecover()

	var (
		oc = exutil.NewCLI("baremetal")
	)

	g.It("have a metal3 deployment", func() {
		dc := oc.AdminDynamicClient()
		skipIfUnsupportedPlatformOrConfig(oc, dc)

		c, err := e2e.LoadClientset()
		o.Expect(err).ToNot(o.HaveOccurred())

		metal3, err := c.AppsV1().Deployments("openshift-machine-api").Get(context.Background(), "metal3", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(metal3.Status.AvailableReplicas).To(o.BeEquivalentTo(1))

		o.Expect(metal3.Annotations).Should(o.HaveKey("baremetal.openshift.io/owned"))
		o.Expect(metal3.Labels).Should(o.HaveKeyWithValue("baremetal.openshift.io/cluster-baremetal-operator", "metal3-state"))
	})
})

var _ = g.Describe("[sig-installer][Feature:baremetal][apigroup:metal3.io][apigroup:config.openshift.io] Baremetal platform should", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLI("baremetal")
	isTNFDeployment := false

	g.BeforeEach(func() {
		skipIfNotBaremetal(oc)
		isTNFDeployment = exutil.IsTwoNodeFencing(context.TODO(), oc.AdminConfigClient())
	})

	g.It("have baremetalhost resources", func() {
		dc := oc.AdminDynamicClient()
		bmc := baremetalClient(dc)

		expectedOperationalStatus := metal3v1alpha1.OperationalStatusOK
		if isTNFDeployment {
			expectedOperationalStatus = metal3v1alpha1.OperationalStatusDetached
		}

		hosts, err := bmc.List(context.Background(), metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(hosts.Items).ToNot(o.BeEmpty())

		for _, h := range hosts.Items {
			bmh := toBMH(h)
			o.Expect(bmh.Status.OperationalStatus).To(o.Equal(expectedOperationalStatus), "host %s", bmh.Name)
			o.Expect(bmh.Status.Provisioning.State).To(o.Or(
				o.Equal(metal3v1alpha1.StateProvisioned),
				o.Equal(metal3v1alpha1.StateExternallyProvisioned),
			), "host %s", bmh.Name)
			o.Expect(bmh.Spec.Online).To(o.BeTrue(), "host %s", bmh.Name)
		}
	})

	g.It("have preprovisioning images for workers", func() {
		dc := oc.AdminDynamicClient()
		bmc := baremetalClient(dc)
		ppiClient := preprovisioningImagesClient(dc)

		hosts, err := bmc.List(context.Background(), metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		for _, h := range hosts.Items {
			bmh := toBMH(h)
			if bmh.Status.Provisioning.State != metal3v1alpha1.StateExternallyProvisioned {
				_, err := ppiClient.Get(context.Background(), bmh.Name, metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred(), "missing PreprovisioningImage for host %s", bmh.Name)
			}
		}
	})

	g.It("have hostfirmwaresetting resources", func() {
		dc := oc.AdminDynamicClient()

		bmc := baremetalClient(dc)
		hosts, err := bmc.List(context.Background(), metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(hosts.Items).ToNot(o.BeEmpty())

		hfsClient := hostfirmwaresettingsClient(dc)

		for _, h := range hosts.Items {
			bmh := toBMH(h)

			g.By(fmt.Sprintf("check that baremetalhost %s has a corresponding hostfirmwaresettings", bmh.Name))
			hfsUnstructured, err := hfsClient.Get(context.Background(), bmh.Name, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			hfs := toHFS(*hfsUnstructured)

			g.By("check that hostfirmwaresettings settings have been populated")
			o.Expect(hfs.Status.Settings).ToNot(o.BeEmpty())

			g.By("check that hostfirmwaresettings conditions show resource is valid")
			o.Expect(hfs.Status.Conditions).ToNot(o.BeEmpty())
			for _, cond := range hfs.Status.Conditions {
				if cond.Type == string(metal3v1alpha1.FirmwareSettingsValid) {
					o.Expect(cond.Status).To(o.Equal(metav1.ConditionTrue))
				}
			}

			g.By("check that hostfirmwaresettings reference a schema")
			o.Expect(hfs.Status.FirmwareSchema).ToNot(o.BeNil())
			o.Expect(hfs.Status.FirmwareSchema.Name).ToNot(o.BeEmpty())

			fsClient := firmwareSchemaClient(dc, hfs.Status.FirmwareSchema.Namespace)
			fs, err := fsClient.Get(context.Background(), hfs.Status.FirmwareSchema.Name, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(fs).NotTo(o.BeNil())
		}
	})

	g.It("not allow updating BootMacAddress", func() {
		dc := oc.AdminDynamicClient()
		bmc := baremetalClient(dc)

		hosts, err := bmc.List(context.Background(), metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(hosts.Items).ToNot(o.BeEmpty())

		host := hosts.Items[0]
		bmh := toBMH(host)
		o.Expect(bmh.Spec.BootMACAddress).ToNot(o.BeEmpty())

		testMACAddress := "11:11:11:11:11:11"

		g.By("updating bootMACAddress which is not allowed")
		err = unstructured.SetNestedField(host.Object, testMACAddress, "spec", "bootMACAddress")
		o.Expect(err).NotTo(o.HaveOccurred())
		_, err = bmc.Update(context.Background(), &host, metav1.UpdateOptions{})
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(err.Error()).To(o.ContainSubstring("bootMACAddress can not be changed once it is set"))

		g.By("verify bootMACAddress is not updated")
		h, err := bmc.Get(context.Background(), host.GetName(), metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		updated := toBMH(*h)
		o.Expect(updated.Spec.BootMACAddress).To(o.Equal(bmh.Spec.BootMACAddress))
	})

	g.It("have a valid provisioning configuration", func() {
		dc := oc.AdminDynamicClient()
		provisioningClient := dc.Resource(provisioningGVR())

		provisioning, err := provisioningClient.Get(context.Background(), "provisioning-configuration", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("checking provisioning network is set")
		provisioningNetwork, found, err := unstructured.NestedString(provisioning.Object, "spec", "provisioningNetwork")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(found).To(o.BeTrue(), "spec.provisioningNetwork not found")
		o.Expect(provisioningNetwork).ToNot(o.BeEmpty())
	})

	g.It("have all metal3 pod containers running", func() {
		c, err := e2e.LoadClientset()
		o.Expect(err).ToNot(o.HaveOccurred())

		pods, err := c.CoreV1().Pods("openshift-machine-api").List(context.Background(), metav1.ListOptions{
			LabelSelector: "baremetal.openshift.io/cluster-baremetal-operator=metal3-state",
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(pods.Items).ToNot(o.BeEmpty())

		for _, pod := range pods.Items {
			g.By(fmt.Sprintf("checking containers in pod %s", pod.Name))
			for _, cs := range pod.Status.ContainerStatuses {
				o.Expect(cs.Ready).To(o.BeTrue(), fmt.Sprintf("container %s in pod %s is not ready", cs.Name, pod.Name))
				o.Expect(cs.RestartCount).To(o.BeNumerically("<", 5), fmt.Sprintf("container %s in pod %s has restarted %d times", cs.Name, pod.Name, cs.RestartCount))
			}
		}
	})

	g.It("have a metal3-image-customization deployment", func() {
		c, err := e2e.LoadClientset()
		o.Expect(err).ToNot(o.HaveOccurred())

		icc, err := c.AppsV1().Deployments("openshift-machine-api").Get(context.Background(), "metal3-image-customization", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(icc.Status.AvailableReplicas).To(o.BeEquivalentTo(1))

		o.Expect(icc.Annotations).Should(o.HaveKey("baremetal.openshift.io/owned"))
		o.Expect(icc.Labels).Should(o.HaveKeyWithValue("baremetal.openshift.io/cluster-baremetal-operator", "metal3-image-customization-service"))
	})
})

// This block must be used for the serial tests. Any eventual extra worker deployed will be
// automatically deleted during the AfterEach
var _ = g.Describe("[sig-installer][Feature:baremetal][Serial][apigroup:metal3.io] Baremetal platform should", func() {
	defer g.GinkgoRecover()

	var (
		oc     = exutil.NewCLI("baremetal")
		helper *BaremetalTestHelper
	)

	g.BeforeEach(func() {
		skipIfNotBaremetal(oc)
		helper = NewBaremetalTestHelper(oc.AdminDynamicClient())
		helper.Setup()
	})

	g.AfterEach(func() {
		helper.DeleteAllExtraWorkers()
	})

	g.It("skip inspection when disabled by annotation", func() {

		// Get extra worker info
		hostData, secretData := helper.GetExtraWorkerData(0)

		// Set inspection annotation as disabled
		unstructured.SetNestedField(hostData.Object, "disabled", "metadata", "annotations", "inspect.metal3.io")

		// Deploy extra worker and wait
		host, _ := helper.CreateExtraWorker(hostData, secretData)
		host = helper.WaitForProvisioningState(host, "available")

		g.By("Check that hardware field in status is empty")
		bmh := toBMH(*host)
		o.Expect(bmh.Status.HardwareDetails).To(o.BeNil())
	})
})
