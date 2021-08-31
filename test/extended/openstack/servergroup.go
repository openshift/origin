package openstack

import (
	"context"

	"github.com/blang/semver"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/servergroups"
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-installer][Feature:openstack] OpenStack platform should", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLI("openstack")

	g.It("create Control plane nodes in a server group", func() {
		ctx := context.TODO()

		SkipUnlessOpenStack(ctx, oc)
		SkipUnlessVersion(ctx, oc, semver.Version{Major: 4, Minor: 8})

		containsAll := func(slice []string, elements ...string) bool {
			var acc int
			for _, e := range slice {
				for _, element := range elements {
					if e == element {
						acc++
						break
					}
				}
			}
			return acc == len(elements)
		}

		g.By("getting the OpenStack IDs of the Control plane instances")
		masterInstances := make([]string, 0, 3)
		{
			clientSet, err := e2e.LoadClientset()
			o.Expect(err).NotTo(o.HaveOccurred())

			nodeList, err := clientSet.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			for _, item := range nodeList.Items {
				if _, ok := item.GetLabels()["node-role.kubernetes.io/master"]; ok {
					masterInstances = append(masterInstances, item.Status.NodeInfo.SystemUUID)
				}
			}
		}

		g.By("getting information on the OpenStack Server groups")
		var groups []servergroups.ServerGroup
		{
			computeClient, err := client(serviceCompute)
			o.Expect(err).NotTo(o.HaveOccurred())
			allPages, err := servergroups.List(computeClient).AllPages()
			o.Expect(err).NotTo(o.HaveOccurred())
			groups, err = servergroups.ExtractServerGroups(allPages)
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		g.By("ensuring that at least one group contains all Control plane instances")
		var found bool
		for _, group := range groups {
			if containsAll(group.Members, masterInstances...) {
				found = true
				break
			}
		}
		o.Expect(found).To(o.BeTrue(), "No server group was found holding all masters together")
	})
})
