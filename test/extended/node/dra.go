package node

import (
	"context"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
	admissionapi "k8s.io/pod-security-admission/api"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-node][DRA][OCPFeatureGate:DynamicResourceAllocation]", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithPodSecurityLevel("dra-scheduling", admissionapi.LevelPrivileged)

	g.Context("Dynamic Resource Allocation", func() {

		g.It("should verify beta and alpha DRA APIs are disabled [apigroup:resource.k8s.io]", func(ctx context.Context) {
			g.By("discovering available API versions for resource.k8s.io group")
			discoveryClient := oc.AdminKubeClient().Discovery()
			apiGroup, err := discoveryClient.ServerResourcesForGroupVersion("resource.k8s.io/v1")
			o.Expect(err).NotTo(o.HaveOccurred(), "v1 API should be available when DRA feature gate is enabled")
			o.Expect(apiGroup).NotTo(o.BeNil())
			framework.Logf("Found resource.k8s.io/v1 API with %d resources", len(apiGroup.APIResources))

			g.By("listing all available versions for resource.k8s.io group")
			apiGroupList, err := discoveryClient.ServerGroups()
			o.Expect(err).NotTo(o.HaveOccurred(), "should be able to list API groups")

			var resourceAPIGroup *metav1.APIGroup
			for _, group := range apiGroupList.Groups {
				if group.Name == "resource.k8s.io" {
					resourceAPIGroup = &group
					break
				}
			}
			o.Expect(resourceAPIGroup).NotTo(o.BeNil(), "resource.k8s.io group should exist")

			framework.Logf("Available versions for resource.k8s.io: %v", resourceAPIGroup.Versions)
			// Verify only v1 is in the list
			expectedVersions := []metav1.GroupVersionForDiscovery{
				{
					GroupVersion: "resource.k8s.io/v1",
					Version:      "v1",
				},
			}
			o.Expect(resourceAPIGroup.Versions).To(o.Equal(expectedVersions), "only v1 should be available")
			o.Expect(resourceAPIGroup.PreferredVersion.Version).To(o.Equal("v1"), "v1 should be the preferred version")
		})

	})
})
