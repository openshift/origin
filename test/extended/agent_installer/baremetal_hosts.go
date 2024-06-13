package baremetal

import (
	"context"
	"fmt"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/openshift/origin/test/extended/baremetal"
	exutil "github.com/openshift/origin/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = g.Describe("[sig-installer][Feature:baremetal] Baremetal platform created by agent-installer should", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLI("baremetal")

	g.BeforeEach(func() {
		baremetal.SkipIfNotBaremetal(oc)
	})

	g.It("have baremetalhost resources", func() {
		dc := oc.AdminDynamicClient()
		bmc := baremetal.BaremetalClient(dc)

		hosts, err := bmc.List(context.Background(), metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(hosts.Items).ToNot(o.BeEmpty())
	})

	g.It("have provisioned hosts when provisioning network set to managed", func() {
		dc := oc.AdminDynamicClient()
		bmc := baremetal.BaremetalClient(dc)

		provisioningNetwork := baremetal.GetProvisioningNetwork(dc)
		if provisioningNetwork == "Managed" {
			hosts, _ := bmc.List(context.Background(), metav1.ListOptions{})

			for _, h := range hosts.Items {
				baremetal.ExpectStringField(h, "baremetalhost", "status.operationalStatus").To(o.BeEquivalentTo("OK"))
				baremetal.ExpectStringField(h, "baremetalhost", "status.provisioning.state").To(o.Or(o.BeEquivalentTo("provisioned"), o.BeEquivalentTo("externally provisioned")))
				baremetal.ExpectBoolField(h, "baremetalhost", "spec.online").To(o.BeTrue())

			}
		}
	})

	g.It("have hostfirmwaresetting resources when provisioning network set to managed", func() {
		dc := oc.AdminDynamicClient()

		bmc := baremetal.BaremetalClient(dc)
		hosts, _ := bmc.List(context.Background(), metav1.ListOptions{})

		hfsClient := baremetal.HostfirmwaresettingsClient(dc)

		provisioningNetwork := baremetal.GetProvisioningNetwork(dc)
		if provisioningNetwork == "Managed" {
			for _, h := range hosts.Items {
				hostName := baremetal.GetStringField(h, "baremetalhost", "metadata.name")

				g.By(fmt.Sprintf("check that baremetalhost %s has a corresponding hostfirmwaresettings", hostName))
				hfs, err := hfsClient.Get(context.Background(), hostName, metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(hfs).NotTo(o.Equal(nil))

				g.By("check that hostfirmwaresettings conditions show resource is valid")
				baremetal.CheckConditionStatus(*hfs, "Valid", "True")

				g.By("check that hostfirmwaresettings reference a schema")
				refName := baremetal.GetStringField(*hfs, "hostfirmwaresettings", "status.schema.name")
				refNS := baremetal.GetStringField(*hfs, "hostfirmwaresettings", "status.schema.namespace")

				schemaClient := dc.Resource(schema.GroupVersionResource{Group: "metal3.io", Resource: "firmwareschemas", Version: "v1alpha1"}).Namespace(refNS)
				schema, err := schemaClient.Get(context.Background(), refName, metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(schema).NotTo(o.Equal(nil))
			}
		}
	})
})
