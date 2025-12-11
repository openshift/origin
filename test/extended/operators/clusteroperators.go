package operators

import (
	"context"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	s "github.com/onsi/gomega/gstruct"
	t "github.com/onsi/gomega/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kube-openapi/pkg/util/sets"

	config "github.com/openshift/api/config/v1"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-arch] ClusterOperators [apigroup:config.openshift.io]", func() {
	defer g.GinkgoRecover()

	var clusterOperators []config.ClusterOperator
	whitelistNoNamespace := sets.NewString(
		"cloud-credential",
		"image-registry",
		"machine-api",
		"marketplace",
		"network",
		"operator-lifecycle-manager",
		"operator-lifecycle-manager-catalog",
		"support",
	)
	whitelistNoOperatorConfig := sets.NewString(
		"cloud-credential",
		"cluster-autoscaler",
		"machine-api",
		"machine-config",
		"marketplace",
		"network",
		"operator-lifecycle-manager",
		"operator-lifecycle-manager-catalog",
		"support",
	)

	oc := exutil.NewCLIWithoutNamespace("clusteroperators")

	g.BeforeEach(func() {
		clusterOperatorsList, err := oc.AdminConfigClient().ConfigV1().ClusterOperators().List(context.Background(), metav1.ListOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())
		clusterOperators = clusterOperatorsList.Items
	})

	g.Context("should define", func() {
		g.Specify("at least one namespace in their lists of related objects", g.Label("Size:S"), func() {
			for _, clusterOperator := range clusterOperators {
				if !whitelistNoNamespace.Has(clusterOperator.Name) {
					o.Expect(clusterOperator.Status.RelatedObjects).To(o.ContainElement(isNamespace()), "ClusterOperator: %s", clusterOperator.Name)
				}
			}

		})

		oc := exutil.NewCLI("clusteroperators")
		g.Specify("at least one related object that is not a namespace", g.Label("Size:S"), func() {
			controlplaneTopology, err := exutil.GetControlPlaneTopology(oc)
			o.Expect(err).NotTo(o.HaveOccurred())

			if *controlplaneTopology == config.ExternalTopologyMode {
				// The packageserver runs in a different cluster along the other controlplane components
				// when the controlplane is external.
				whitelistNoOperatorConfig.Insert("operator-lifecycle-manager-packageserver")
			}
			for _, clusterOperator := range clusterOperators {
				if !whitelistNoOperatorConfig.Has(clusterOperator.Name) {
					o.Expect(clusterOperator.Status.RelatedObjects).To(o.ContainElement(o.Not(isNamespace())), "ClusterOperator: %s", clusterOperator.Name)
				}
			}
		})

		g.Specify("valid related objects", g.Label("Size:S"), func() {
			controlplaneTopology, err := exutil.GetControlPlaneTopology(oc)
			o.Expect(err).NotTo(o.HaveOccurred())
			if *controlplaneTopology == config.ExternalTopologyMode {
				// The packageserver runs in a different cluster along the other controlplane components
				// when the controlplane is external.
				whitelistNoOperatorConfig.Insert("operator-lifecycle-manager-packageserver")
			}
			restMapper := oc.RESTMapper()

			for _, clusterOperator := range clusterOperators {
				if !whitelistNoOperatorConfig.Has(clusterOperator.Name) {
					for _, relatedObj := range clusterOperator.Status.RelatedObjects {

						// any uppercase immediately signals an improper mapping:
						o.Expect(relatedObj.Resource).To(o.Equal(strings.ToLower(relatedObj.Resource)),
							"Cluster operator %s should be using lowercase resource references: %s",
							clusterOperator.Name, relatedObj.Resource)

						// also ensure we find a valid rest mapping:

						// "storage" ClusterOperator has some relatedResource refs to CRDs that only exists if
						// TechPreviewNoUpgrade is enabled. This is acceptable and not a bug, but needs a special case here.
						if clusterOperator.Name == "storage" && (relatedObj.Resource == "sharedconfigmaps" ||
							relatedObj.Resource == "sharedsecrets") {
							continue
						}

						resourceMatches, err := restMapper.ResourcesFor(schema.GroupVersionResource{
							Group:    relatedObj.Group,
							Resource: relatedObj.Resource,
						})
						o.Expect(err).ToNot(o.HaveOccurred())
						o.Expect(len(resourceMatches)).To(o.BeNumerically(">", 0),
							"No valid rest mapping found for cluster operator %s related object %s",
							clusterOperator.Name, relatedObj.Resource)
					}
				}
			}
		})

	})
})

func isNamespace() t.GomegaMatcher {
	return s.MatchFields(s.IgnoreExtras|s.IgnoreMissing, s.Fields{
		"Resource": o.Equal("namespaces"),
		"Group":    o.Equal(""),
	})
}
