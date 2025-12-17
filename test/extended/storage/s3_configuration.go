package storage

import (
	"context"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	imageregistryv1 "github.com/openshift/api/imageregistry/v1"
	imageregistry "github.com/openshift/client-go/imageregistry/clientset/versioned"
	exutil "github.com/openshift/origin/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

const (
	clusterConfigName = "cluster"
)

var _ = g.Describe("[sig-imageregistry][OCPFeatureGate:ChunkSizeMiB][Serial][apigroup:imageregistry.operator.openshift.io] Image Registry Config ChunkSizeMiB", func() {
	defer g.GinkgoRecover()
	var (
		ctx                = context.Background()
		oc                 = exutil.NewCLI("image-registry-config")
		originalConfigSpec *imageregistryv1.ImageRegistryConfigStorageS3
	)

	g.BeforeEach(func() {

		skipIfNotS3Storage(oc)

		imageRegistryConfigClient, err := imageregistry.NewForConfig(oc.AdminConfig())
		o.Expect(err).NotTo(o.HaveOccurred())
		imageRegistryConfig, err := imageRegistryConfigClient.ImageregistryV1().Configs().Get(ctx, clusterConfigName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		if imageRegistryConfig.Spec.Storage.S3 != nil {
			originalConfigSpec = imageRegistryConfig.Spec.Storage.S3.DeepCopy()
		} else {
			originalConfigSpec = &imageregistryv1.ImageRegistryConfigStorageS3{}
		}
		e2e.Logf("Storing original Image Registry Config")
	})

	g.AfterEach(func() {
		if originalConfigSpec == nil {
			return
		}

		e2e.Logf("Restoring original Image Registry Config")
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			imageRegistryConfigClient, err := imageregistry.NewForConfig(oc.AdminConfig())
			o.Expect(err).NotTo(o.HaveOccurred())
			imageRegistryConfig, err := imageRegistryConfigClient.ImageregistryV1().Configs().Get(ctx, clusterConfigName, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			imageRegistryConfig.Spec.Storage.S3 = originalConfigSpec

			_, err = imageRegistryConfigClient.ImageregistryV1().Configs().Update(ctx, imageRegistryConfig, metav1.UpdateOptions{})
			return err
		})
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It("should set ChunkSizeMiB value", g.Label("Size:S"), func() {
		g.By("Setting ChunkSizeMiB value")
		expectedChunkSize := int32(128) // 128MB

		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			imageRegistryConfigClient, err := imageregistry.NewForConfig(oc.AdminConfig())
			o.Expect(err).NotTo(o.HaveOccurred())
			imageRegistryConfig, err := imageRegistryConfigClient.ImageregistryV1().Configs().Get(ctx, clusterConfigName, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			imageRegistryConfig.Spec.Storage.S3.ChunkSizeMiB = expectedChunkSize

			_, err = imageRegistryConfigClient.ImageregistryV1().Configs().Update(ctx, imageRegistryConfig, metav1.UpdateOptions{})
			return err
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Validating ChunkSizeMiB value")
		imageRegistryConfigClient, err := imageregistry.NewForConfig(oc.AdminConfig())
		o.Expect(err).NotTo(o.HaveOccurred())
		imageRegistryConfig, err := imageRegistryConfigClient.ImageregistryV1().Configs().Get(ctx, clusterConfigName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(imageRegistryConfig.Spec.Storage.S3.ChunkSizeMiB).To(o.Equal(expectedChunkSize))
	})

	g.It("should not accept invalid ChunkSizeMiB value", g.Label("Size:S"), func() {
		g.By("Setting invalid ChunkSizeMiB value")
		invalidChunkSize := int32(-1)

		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			imageRegistryConfigClient, err := imageregistry.NewForConfig(oc.AdminConfig())
			o.Expect(err).NotTo(o.HaveOccurred())
			imageRegistryConfig, err := imageRegistryConfigClient.ImageregistryV1().Configs().Get(ctx, clusterConfigName, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			imageRegistryConfig.Spec.Storage.S3.ChunkSizeMiB = invalidChunkSize

			_, err = imageRegistryConfigClient.ImageregistryV1().Configs().Update(ctx, imageRegistryConfig, metav1.UpdateOptions{})
			return err
		})
		o.Expect(err).To(o.HaveOccurred())
	})

	g.It("should set minimum valid ChunkSizeMiB value", g.Label("Size:S"), func() {
		g.By("Setting minimum valid ChunkSizeMiB value")
		minValidChunkSize := int32(5) // 5MB

		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			imageRegistryConfigClient, err := imageregistry.NewForConfig(oc.AdminConfig())
			o.Expect(err).NotTo(o.HaveOccurred())
			imageRegistryConfig, err := imageRegistryConfigClient.ImageregistryV1().Configs().Get(ctx, clusterConfigName, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			imageRegistryConfig.Spec.Storage.S3.ChunkSizeMiB = minValidChunkSize

			_, err = imageRegistryConfigClient.ImageregistryV1().Configs().Update(ctx, imageRegistryConfig, metav1.UpdateOptions{})
			return err
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Validating minimum valid ChunkSizeMiB value")
		imageRegistryConfigClient, err := imageregistry.NewForConfig(oc.AdminConfig())
		o.Expect(err).NotTo(o.HaveOccurred())
		imageRegistryConfig, err := imageRegistryConfigClient.ImageregistryV1().Configs().Get(ctx, clusterConfigName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(imageRegistryConfig.Spec.Storage.S3.ChunkSizeMiB).To(o.Equal(minValidChunkSize))
	})

	g.It("should set maximum valid ChunkSizeMiB value", g.Label("Size:S"), func() {
		g.By("Setting maximum valid ChunkSizeMiB value")
		maxValidChunkSize := int32(5 * 1024) // 5GB

		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			imageRegistryConfigClient, err := imageregistry.NewForConfig(oc.AdminConfig())
			o.Expect(err).NotTo(o.HaveOccurred())
			imageRegistryConfig, err := imageRegistryConfigClient.ImageregistryV1().Configs().Get(ctx, clusterConfigName, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			imageRegistryConfig.Spec.Storage.S3.ChunkSizeMiB = maxValidChunkSize

			_, err = imageRegistryConfigClient.ImageregistryV1().Configs().Update(ctx, imageRegistryConfig, metav1.UpdateOptions{})
			return err
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Validating maximum valid ChunkSizeMiB value")
		imageRegistryConfigClient, err := imageregistry.NewForConfig(oc.AdminConfig())
		o.Expect(err).NotTo(o.HaveOccurred())
		imageRegistryConfig, err := imageRegistryConfigClient.ImageregistryV1().Configs().Get(ctx, clusterConfigName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(imageRegistryConfig.Spec.Storage.S3.ChunkSizeMiB).To(o.Equal(maxValidChunkSize))
	})

	g.It("should reject ChunkSizeMiB value greater than 5 GiB", g.Label("Size:S"), func() {
		g.By("Setting zero ChunkSizeMiB value")
		chunkSize := int32(6 * 1024)

		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			imageRegistryConfigClient, err := imageregistry.NewForConfig(oc.AdminConfig())
			o.Expect(err).NotTo(o.HaveOccurred())
			imageRegistryConfig, err := imageRegistryConfigClient.ImageregistryV1().Configs().Get(ctx, clusterConfigName, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			imageRegistryConfig.Spec.Storage.S3.ChunkSizeMiB = chunkSize

			_, err = imageRegistryConfigClient.ImageregistryV1().Configs().Update(ctx, imageRegistryConfig, metav1.UpdateOptions{})
			return err
		})
		o.Expect(err).To(o.HaveOccurred())

		g.By("Validating that ChunkSizeMiB value is rejected")
		imageRegistryConfigClient, err := imageregistry.NewForConfig(oc.AdminConfig())
		o.Expect(err).NotTo(o.HaveOccurred())
		imageRegistryConfig, err := imageRegistryConfigClient.ImageregistryV1().Configs().Get(ctx, clusterConfigName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(imageRegistryConfig.Spec.Storage.S3.ChunkSizeMiB).NotTo(o.Equal(chunkSize))
	})

})
