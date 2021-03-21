package baremetal

import (
	"context"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	configv1 "github.com/openshift/api/config/v1"
	exutil "github.com/openshift/origin/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
)

func skipIfNotBaremetal(oc *exutil.CLI) {
	g.By("checking platform type")

	infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	if infra.Status.Platform != configv1.BareMetalPlatformType {
		e2eskipper.Skipf("No baremetal platform detected")
	}
}

func baremetalClient(dc dynamic.Interface) dynamic.ResourceInterface {
	baremetalClient := dc.Resource(schema.GroupVersionResource{Group: "metal3.io", Resource: "baremetalhosts", Version: "v1alpha1"})
	return baremetalClient.Namespace("openshift-machine-api")
}

var _ = g.Describe("[sig-installer][Feature:baremetal] Baremetal platform should", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLI("baremetal")

	g.It("have a metal3 deployment", func() {
		skipIfNotBaremetal(oc)

		c, err := e2e.LoadClientset()
		o.Expect(err).ToNot(o.HaveOccurred())

		metal3, err := c.AppsV1().Deployments("openshift-machine-api").Get(context.Background(), "metal3", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(metal3.Status.AvailableReplicas).To(o.BeEquivalentTo(1))
	})

	g.It("have baremetalhost resources", func() {
		skipIfNotBaremetal(oc)

		dc := oc.AdminDynamicClient()
		bmc := baremetalClient(dc)

		hosts, err := bmc.List(context.Background(), v1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(hosts.Items).ToNot(o.BeEmpty())

		for _, h := range hosts.Items {
			status, found, err := unstructured.NestedString(h.Object, "status", "operationalStatus")
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(found).To(o.BeTrue(), "baremetalhost not ready")
			o.Expect(status).To(o.BeEquivalentTo("OK"))
		}
	})
})
