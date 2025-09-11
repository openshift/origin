package images

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"
	imagev1 "github.com/openshift/api/image/v1"
	configclient "github.com/openshift/client-go/config/clientset/versioned"
	"github.com/openshift/library-go/pkg/image/imageutil"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-imageregistry][OCPFeatureGate:ImageStreamImportMode][Serial] ImageStream API", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLI("imagestream-api")

	g.It("import mode should be PreserveOriginal or Legacy depending on desired.architecture field in the CV [apigroup:image.openshift.io]", func() {
		TestImageStreamImportMode(g.GinkgoT(), oc)
	})

	g.It("import mode should be Legacy if the import mode specified in image.config.openshift.io config is Legacy [apigroup:image.openshift.io]", func() {
		TestImageConfigImageStreamImportModeLegacy(g.GinkgoT(), oc)
	})

	g.It("import mode should be PreserveOriginal if the import mode specified in image.config.openshift.io config is PreserveOriginal [apigroup:image.openshift.io]", func() {
		TestImageConfigImageStreamImportModePreserveOriginal(g.GinkgoT(), oc)
	})
})

func TestImageStreamImportMode(t g.GinkgoTInterface, oc *exutil.CLI) {
	ctx := context.Background()
	if !exutil.IsTechPreviewNoUpgrade(ctx, oc.AdminConfigClient()) {
		g.Skip("skipping, this feature is only supported on TechPreviewNoUpgrade clusters")
	}

	// Check desired.Architecture in the CV
	configClient, err := configclient.NewForConfig(oc.AdminConfig())
	o.Expect(err).NotTo(o.HaveOccurred())
	clusterVersion, err := configClient.ConfigV1().ClusterVersions().Get(ctx, "version", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	importmode := imagev1.ImportModeLegacy
	if clusterVersion.Status.Desired.Architecture == configv1.ClusterVersionArchitectureMulti {
		importmode = imagev1.ImportModePreserveOriginal
	}

	clusterAdminImageClient := oc.AdminImageClient().ImageV1()
	g.By("creating an imagestream and imagestream tag")
	stream := &imagev1.ImageStream{ObjectMeta: metav1.ObjectMeta{Name: "test-importmode"}}

	expected, err := clusterAdminImageClient.ImageStreams(oc.Namespace()).Create(ctx, stream, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(expected.Name).NotTo(o.BeEmpty())

	_, err = clusterAdminImageClient.ImageStreamTags(oc.Namespace()).Create(ctx, &imagev1.ImageStreamTag{
		ObjectMeta: metav1.ObjectMeta{Name: "test-importmode:1"},
		Tag: &imagev1.TagReference{
			From: &corev1.ObjectReference{Kind: "ImageStreamTag", Namespace: "openshift", Name: "tools:latest"},
		},
	}, metav1.CreateOptions{})
	o.Expect(err).ToNot(o.HaveOccurred())

	is, err := clusterAdminImageClient.ImageStreams(oc.Namespace()).Get(ctx, stream.Name, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	tag, ok := imageutil.SpecHasTag(is, "1")
	o.Expect(ok).To(o.BeTrue())
	o.Expect(tag.ImportPolicy.ImportMode).To(o.Equal(importmode))
}
