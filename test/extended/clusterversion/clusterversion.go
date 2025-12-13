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

// The desired architecture field was introduced as part of a feature that provides a toggle for an imagestream's importMode: https://github.com/openshift/api/pull/2024
var _ = g.Describe("[sig-cluster-lifecycle][OCPFeatureGate:ImageStreamImportMode] ClusterVersion API", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLIWithoutNamespace("")

	g.It("desired architecture should be valid when architecture is set in release payload metadata [apigroup:config.openshift.io]", g.Label("Size:S"), func() {
		TestClusterVersionDesiredArchitecture(g.GinkgoT(), oc)
	})

})

func TestClusterVersionDesiredArchitecture(t g.GinkgoTInterface, oc *exutil.CLI) {
	ctx := context.Background()

	archMetadata, _, err := oc.AsAdmin().Run("adm", "release", "info", `-ojsonpath={.metadata.metadata.release\.openshift\.io\/architecture}`).Outputs()
	if err != nil {
		cleanup, cmdArgs, err := exutil.PrepareImagePullSecretAndCABundle(oc)
		if cleanup != nil {
			defer cleanup()
		}
		o.Expect(err).NotTo(o.HaveOccurred())
		archMetadata, _, err = oc.AsAdmin().Run("adm", "release", "info", `-ojsonpath={.metadata.metadata.release\.openshift\.io\/architecture}`).Args(cmdArgs...).Outputs()
		o.Expect(err).NotTo(o.HaveOccurred())
	} else {
		o.Expect(err).NotTo(o.HaveOccurred())
	}

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
