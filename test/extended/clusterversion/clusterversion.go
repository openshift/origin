package clusterversion

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"
	configclient "github.com/openshift/client-go/config/clientset/versioned"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-cluster-lifecycle][OCPFeatureGate:ImageStreamImportMode] ClusterVersion API", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLIWithoutNamespace("")

	g.It("desired.architecture field in the CV should be multi or empty depending on the 'release.openshift.io/architecture' field being set in the release payload's metadata [apigroup:config.openshift.io]", func() {
		TestClusterVersionDesiredArchitecture(g.GinkgoT(), oc)
	})

})

func TestClusterVersionDesiredArchitecture(t g.GinkgoTInterface, oc *exutil.CLI) {
	ctx := context.Background()

	archMetadata, _, err := oc.AsAdmin().Run("adm", "release", "info", `-ojsonpath={.metadata.metadata.release\.openshift\.io\/architecture}`).Outputs()
	o.Expect(err).NotTo(o.HaveOccurred())

	// Check desired.Architecture in the CV
	configClient, err := configclient.NewForConfig(oc.AdminConfig())
	o.Expect(err).NotTo(o.HaveOccurred())
	clusterVersion, err := configClient.ConfigV1().ClusterVersions().Get(ctx, "version", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	if archMetadata == "multi" {
		o.Expect(clusterVersion.Status.Desired.Architecture).To(o.Equal(configv1.ClusterVersionArchitectureMulti))
	} else {
		o.Expect(clusterVersion.Status.Desired.Architecture).To(o.BeEmpty())
	}
}
