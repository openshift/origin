package baremetal

import (
	"context"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/origin/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = g.Describe("[sig-installer][Feature:baremetal] Baremetal/OpenStack/vSphere/None platforms ", func() {
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

var _ = g.Describe("[sig-installer][Feature:baremetal] Baremetal platform should", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLI("baremetal")

	g.It("have baremetalhost resources", func() {
		skipIfNotBaremetal(oc)

		dc := oc.AdminDynamicClient()
		bmc := baremetalClient(dc)

		hosts, err := bmc.List(context.Background(), metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(hosts.Items).ToNot(o.BeEmpty())

		for _, h := range hosts.Items {
			expectStringField(h, "baremetalhost", "status.operationalStatus").To(o.BeEquivalentTo("OK"))
			expectStringField(h, "baremetalhost", "status.provisioning.state").To(o.Or(o.BeEquivalentTo("provisioned"), o.BeEquivalentTo("externally provisioned")))
			expectBoolField(h, "baremetalhost", "spec.online").To(o.BeTrue())

		}
	})

	g.It("have preprovisioning images for workers", func() {
		skipIfNotBaremetal(oc)

		dc := oc.AdminDynamicClient()
		bmc := baremetalClient(dc)
		ppiClient := preprovisioningImagesClient(dc)

		hosts, err := bmc.List(context.Background(), metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		for _, h := range hosts.Items {
			state := getStringField(h, "baremetalhost", "status.provisioning.state")
			if state != "externally provisioned" {
				hostName := getStringField(h, "baremetalhost", "metadata.name")
				_, err := ppiClient.Get(context.Background(), hostName, metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
			}
		}
	})

	g.It("have hostfirmwaresetting resources", func() {
		skipIfNotBaremetal(oc)

		dc := oc.AdminDynamicClient()

		bmc := baremetalClient(dc)
		hosts, err := bmc.List(context.Background(), metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(hosts.Items).ToNot(o.BeEmpty())

		hfsClient := hostfirmwaresettingsClient(dc)

		for _, h := range hosts.Items {
			hostName := getStringField(h, "baremetalhost", "metadata.name")

			g.By(fmt.Sprintf("check that baremetalhost %s has a corresponding hostfirmwaresettings", hostName))
			hfs, err := hfsClient.Get(context.Background(), hostName, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(hfs).NotTo(o.Equal(nil))

			// Reenable this when add check that host is using redfish driver
			// g.By("check that hostfirmwaresettings settings have been populated")
			// expectStringMapField(*hfs, "hostfirmwaresettings", "status.settings").ToNot(o.BeEmpty())

			g.By("check that hostfirmwaresettings conditions show resource is valid")
			checkConditionStatus(*hfs, "Valid", "True")

			g.By("check that hostfirmwaresettings reference a schema")
			refName := getStringField(*hfs, "hostfirmwaresettings", "status.schema.name")
			refNS := getStringField(*hfs, "hostfirmwaresettings", "status.schema.namespace")

			schemaClient := dc.Resource(schema.GroupVersionResource{Group: "metal3.io", Resource: "firmwareschemas", Version: "v1alpha1"}).Namespace(refNS)
			schema, err := schemaClient.Get(context.Background(), refName, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(schema).NotTo(o.Equal(nil))
		}
	})

	g.It("not allow updating BootMacAddress", func() {
		skipIfNotBaremetal(oc)

		dc := oc.AdminDynamicClient()
		bmc := baremetalClient(dc)

		hosts, err := bmc.List(context.Background(), metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(hosts.Items).ToNot(o.BeEmpty())

		host := hosts.Items[0]
		expectStringField(host, "baremetalhost", "spec.bootMACAddress").ShouldNot(o.BeNil())
		// Already verified that bootMACAddress exists
		bootMACAddress, _, _ := unstructured.NestedString(host.Object, "spec", "bootMACAddress")
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
		check, _, _ := unstructured.NestedString(h.Object, "spec", "bootMACAddress")
		o.Expect(check).To(o.Equal(bootMACAddress))
	})
})

// This block must be used for the serial tests. Any eventual extra worker deployed will be
// automatically deleted during the AfterEach
var _ = g.Describe("[sig-installer][Feature:baremetal][Serial] Baremetal platform should", func() {
	defer g.GinkgoRecover()

	var (
		oc     = exutil.NewCLI("baremetal")
		helper *BaremetalTestHelper
	)

	g.BeforeEach(func() {
		helper = NewBaremetalTestHelper(oc.AdminDynamicClient())
		helper.Setup()
	})

	g.AfterEach(func() {
		helper.DeleteAllExtraWorkers()
	})

	g.It("skip inspection when disabled by annotation", func() {
		skipIfNotBaremetal(oc)

		// Get extra worker info
		hostData, secretData := helper.GetExtraWorkerData(0)

		// Set inspection annotation as disabled
		unstructured.SetNestedField(hostData.Object, "disabled", "metadata", "annotations", "inspect.metal3.io")

		// Deploy extra worker and wait
		host, _ := helper.CreateExtraWorker(hostData, secretData)
		host = helper.WaitForProvisioningState(host, "available")

		g.By("Check that hardware field in status is empty")
		_, found, err := unstructured.NestedString(host.Object, "status", "hardware")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(found).To(o.BeFalse())
	})

	g.It("configure BIOS settings during cleaning", func() {
		var procTurboMode = "ProcTurboMode"

		skipIfNotBaremetal(oc)

		dc := oc.AdminDynamicClient()

		// Deploy extra worker and wait for it to be available
		host, _ := helper.DeployExtraWorker(0)
		hostName := getStringField(*host, "baremetalhost", "metadata.name")

		hfsClient := hostfirmwaresettingsClient(dc)
		hfs := getHostFirmwareSettings(hfsClient, hostName)

		status, _, err := unstructured.NestedStringMap(hfs.Object, "status", "settings")
		o.Expect(err).NotTo(o.HaveOccurred())
		spec, _, err := unstructured.NestedStringMap(hfs.Object, "spec", "settings")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(spec).NotTo(o.Equal(nil))

		// Change HostFirmwareSetting to an invalid value
		spec[procTurboMode] = "Foo"
		g.By(fmt.Sprintf("setting firmwaresetting %s invalid value for host %s", procTurboMode, hostName))
		err = unstructured.SetNestedStringMap(hfs.Object, spec, "spec", "settings")
		o.Expect(err).NotTo(o.HaveOccurred())
		_, err = hfsClient.Update(context.Background(), hfs, metav1.UpdateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		time.Sleep(2 * time.Second)
		hfs = getHostFirmwareSettings(hfsClient, hostName)
		g.By(fmt.Sprintf("verifying Condition Valid is set to false"))
		checkConditionStatus(*hfs, "Valid", "False")

		// clear settings
		err = unstructured.SetNestedStringMap(hfs.Object, map[string]string{}, "spec", "settings")
		o.Expect(err).NotTo(o.HaveOccurred())
		_, err = hfsClient.Update(context.Background(), hfs, metav1.UpdateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		time.Sleep(2 * time.Second)
		hfs = getHostFirmwareSettings(hfsClient, hostName)
		checkConditionStatus(*hfs, "Valid", "True")

		// Change HostFirmwareSetting to valid value different than current
		status, _, _ = unstructured.NestedStringMap(hfs.Object, "status", "settings")
		v, _ := status[procTurboMode]
		newValue := "Enabled"
		if v == "Enabled" {
			newValue = "Disabled"
		}
		spec[procTurboMode] = newValue
		g.By(fmt.Sprintf("setting firmwaresetting %s to %s for host %s", procTurboMode, newValue, hostName))

		err = unstructured.SetNestedStringMap(hfs.Object, spec, "spec", "settings")

		time.Sleep(2 * time.Second)
		status, _, _ = unstructured.NestedStringMap(hfs.Object, "status", "settings")
		_, ok := status[procTurboMode]
		o.Expect(ok).To(o.BeTrue(), "setting not available for host %s", hostName)

		o.Expect(err).NotTo(o.HaveOccurred())
		_, err = hfsClient.Update(context.Background(), hfs, metav1.UpdateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		// Host should transition to preparing to go through cleaning
		host = helper.WaitForProvisioningState(host, "preparing")

		// after it returns to available check that setting has changed
		host = helper.WaitForProvisioningState(host, "available")
		hfs = getHostFirmwareSettings(hfsClient, hostName)

		status, _, err = unstructured.NestedStringMap(hfs.Object, "status", "settings")
		o.Expect(err).NotTo(o.HaveOccurred())
		v, ok = status[procTurboMode]
		o.Expect(ok).To(o.BeTrue(), "setting not available for host %s", hostName)
		o.Expect(v).To(o.Equal(newValue), "host status not updated to %s", newValue)

	})

})

var _ = g.Describe("[sig-installer][Feature:baremetal][Serial] A baremetal deployment without a provisioning network should", func() {
	defer g.GinkgoRecover()

	var (
		oc = exutil.NewCLI("baremetal")
	)

	g.It("show the Provisioning Network as 'Disabled'", func() {
		skipIfNotBaremetal(oc)

		dc := oc.AdminDynamicClient()

		skipIfProvisioningNetworkSet(dc)

		o.Expect(getProvisioningNetwork(dc)).To((o.BeEquivalentTo("Disabled")))

		g.By("Not allow setting the ProvisioningNetwork to 'Managed' with invalid values")
		invalidProvisioningNetworkCIDR := "172.22.0.0/33"
		provisioningClient := provisioningClient(dc)

		provisionings, err := provisioningClient.List(context.Background(), metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(provisionings.Items).ToNot(o.BeEmpty())

		provisioning := provisionings.Items[0]

		err = unstructured.SetNestedField(provisioning.Object, invalidProvisioningNetworkCIDR, "spec", "provisioningNetworkCIDR")
		o.Expect(err).NotTo(o.HaveOccurred())

		_, err = provisioningClient.Update(context.Background(), &provisioning, metav1.UpdateOptions{})
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(err.Error()).To(o.ContainSubstring("could not parse provisioningNetworkCIDR"))

		o.Expect(getProvisioningNetwork(dc)).To((o.BeEquivalentTo("Disabled")))
	})

	g.It("allow setting the ProvisioningNetwork to 'Managed' with valid settings", func() {
		skipIfNotBaremetal(oc)

		dc := oc.AdminDynamicClient()

		skipIfProvisioningNetworkSet(dc)

		validProvisioningNetworkCIDR := "172.22.0.0/24"
		validProvisioningIP := "172.22.0.3"

		provisioningClient := provisioningClient(dc)

		provisionings, err := provisioningClient.List(context.Background(), metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(provisionings.Items).ToNot(o.BeEmpty())

		provisioning := provisionings.Items[0]

		err = unstructured.SetNestedField(provisioning.Object, validProvisioningNetworkCIDR, "spec", "provisioningNetworkCIDR")
		o.Expect(err).NotTo(o.HaveOccurred())

		err = unstructured.SetNestedField(provisioning.Object, validProvisioningIP, "spec", "provisioningIP")
		o.Expect(err).NotTo(o.HaveOccurred())

		err = unstructured.SetNestedField(provisioning.Object, "Managed", "spec", "provisioningNetwork")
		o.Expect(err).NotTo(o.HaveOccurred())

		_, err = provisioningClient.Update(context.Background(), &provisioning, metav1.UpdateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Move the ProvisioningNetwork back to 'Disabled'")
		err = unstructured.SetNestedField(provisioning.Object, "Disabled", "spec", "provisioningNetwork")
		o.Expect(err).NotTo(o.HaveOccurred())

		_, err = provisioningClient.Update(context.Background(), &provisioning, metav1.UpdateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		o.Expect(getProvisioningNetwork(dc)).To((o.BeEquivalentTo("Disabled")))
	})

})
