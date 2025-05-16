package images

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"
	imagev1 "github.com/openshift/api/image/v1"
	configclient "github.com/openshift/client-go/config/clientset/versioned"
	"github.com/openshift/library-go/pkg/image/imageutil"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/operator"
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

func changeImportModeAndWaitForApiServer(ctx context.Context, t g.GinkgoTInterface, oc *exutil.CLI, importMode string) error {
	data := fmt.Sprintf(`{"spec":{"imageStreamImportMode":null}}`)
	if len(importMode) > 0 {
		data = fmt.Sprintf(`{"spec":{"imageStreamImportMode":"%s"}}`, importMode)
	}
	_, err := oc.AdminConfigClient().ConfigV1().Images().Patch(context.Background(), "cluster", types.MergePatchType, []byte(data), metav1.PatchOptions{})
	if err != nil {
		return err
	}

	// when pods are pending it means the config has started to propagate
	t.Log("waiting for image controller configuration to propagate")
	_, err = exutil.WaitForPods(oc.AdminKubeClient().CoreV1().Pods("openshift-apiserver"), labels.Everything(), exutil.CheckPodIsPending, 1, 5*time.Minute)
	if err != nil {
		return fmt.Errorf("error waiting for new apiserver pods: %s", err)
	}

	t.Log("waiting for new openshift-apiserver operator to settle")
	if err := operator.WaitForOperatorsToSettle(ctx, oc.AdminConfigClient(), 10); err != nil {
		return err
	}
	t.Log("image controller configuration propagated")
	return nil
}

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

func TestImageConfigImageStreamImportModeLegacy(t g.GinkgoTInterface, oc *exutil.CLI) {
	ctx := context.Background()
	if !exutil.IsTechPreviewNoUpgrade(ctx, oc.AdminConfigClient()) {
		g.Skip("skipping, this feature is only supported on TechPreviewNoUpgrade clusters")
	}
	if isSNO, err := exutil.IsSingleNode(ctx, oc.AdminConfigClient()); err == nil && isSNO {
		g.Skip("skipping this test for SNO as it involves an openshift-apiserver disruption")
	}

	clusterAdminConfigClient := oc.AdminConfigClient().ConfigV1()
	imageConfig, err := clusterAdminConfigClient.Images().Get(ctx, "cluster", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	importModeNoChange := false
	if imageConfig.Status.ImageStreamImportMode == configv1.ImportModeLegacy {
		importModeNoChange = true
	}

	// If import mode is actually the same as the intended one, skip changing it
	if !importModeNoChange {
		err = changeImportModeAndWaitForApiServer(ctx, t, oc, string(configv1.ImportModeLegacy))
		o.Expect(err).NotTo(o.HaveOccurred())
	}

	g.By("creating an imagestream and imagestream tag")
	stream := &imagev1.ImageStream{ObjectMeta: metav1.ObjectMeta{Name: "test-importmode"}}

	clusterAdminImageClient := oc.AdminImageClient().ImageV1()
	expected, err := clusterAdminImageClient.ImageStreams(oc.Namespace()).Create(ctx, stream, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(expected.Name).NotTo(o.BeEmpty())

	_, err = clusterAdminImageClient.ImageStreamTags(oc.Namespace()).Create(ctx, &imagev1.ImageStreamTag{
		ObjectMeta: metav1.ObjectMeta{Name: "test-importmode:2"},
		Tag: &imagev1.TagReference{
			From: &corev1.ObjectReference{Kind: "ImageStreamTag", Namespace: "openshift", Name: "tools:latest"},
		},
	}, metav1.CreateOptions{})
	o.Expect(err).ToNot(o.HaveOccurred())

	is, err := clusterAdminImageClient.ImageStreams(oc.Namespace()).Get(ctx, stream.Name, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	tag, ok := imageutil.SpecHasTag(is, "2")
	o.Expect(ok).To(o.BeTrue())
	o.Expect(tag.ImportPolicy.ImportMode).To(o.Equal(imagev1.ImportModeLegacy))

	// Revert back to original
	if !importModeNoChange {
		err = changeImportModeAndWaitForApiServer(ctx, t, oc, "")
		o.Expect(err).NotTo(o.HaveOccurred())
	}
}

func TestImageConfigImageStreamImportModePreserveOriginal(t g.GinkgoTInterface, oc *exutil.CLI) {
	ctx := context.Background()
	if !exutil.IsTechPreviewNoUpgrade(ctx, oc.AdminConfigClient()) {
		g.Skip("skipping, this feature is only supported on TechPreviewNoUpgrade clusters")
	}
	if isSNO, err := exutil.IsSingleNode(ctx, oc.AdminConfigClient()); err == nil && isSNO {
		g.Skip("skipping this test for SNO as it involves an openshift-apiserver disruption")
	}

	clusterAdminConfigClient := oc.AdminConfigClient().ConfigV1()
	imageConfig, err := clusterAdminConfigClient.Images().Get(ctx, "cluster", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	importModeNoChange := false
	if imageConfig.Status.ImageStreamImportMode == configv1.ImportModePreserveOriginal {
		importModeNoChange = true
	}

	// If import mode is actually the same as the intended one, skip changing it
	if !importModeNoChange {
		err = changeImportModeAndWaitForApiServer(ctx, t, oc, string(configv1.ImportModePreserveOriginal))
		o.Expect(err).NotTo(o.HaveOccurred())
	}

	g.By("creating an imagestream and imagestream tag")
	stream := &imagev1.ImageStream{ObjectMeta: metav1.ObjectMeta{Name: "test-importmode"}}

	clusterAdminImageClient := oc.AdminImageClient().ImageV1()
	expected, err := clusterAdminImageClient.ImageStreams(oc.Namespace()).Create(ctx, stream, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(expected.Name).NotTo(o.BeEmpty())

	_, err = clusterAdminImageClient.ImageStreamTags(oc.Namespace()).Create(ctx, &imagev1.ImageStreamTag{
		ObjectMeta: metav1.ObjectMeta{Name: "test-importmode:3"},
		Tag: &imagev1.TagReference{
			From: &corev1.ObjectReference{Kind: "ImageStreamTag", Namespace: "openshift", Name: "tools:latest"},
		},
	}, metav1.CreateOptions{})
	o.Expect(err).ToNot(o.HaveOccurred())

	is, err := clusterAdminImageClient.ImageStreams(oc.Namespace()).Get(ctx, stream.Name, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	tag, ok := imageutil.SpecHasTag(is, "3")
	o.Expect(ok).To(o.BeTrue())
	o.Expect(tag.ImportPolicy.ImportMode).To(o.Equal(imagev1.ImportModePreserveOriginal))

	// Revert back to original
	if !importModeNoChange {
		err = changeImportModeAndWaitForApiServer(ctx, t, oc, "")
		o.Expect(err).NotTo(o.HaveOccurred())
	}
}
