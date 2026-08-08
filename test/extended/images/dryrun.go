package images

import (
	"context"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	admissionapi "k8s.io/pod-security-admission/api"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-imageregistry] Image --dry-run", func() {
	defer g.GinkgoRecover()

	var (
		oc = exutil.NewCLIWithPodSecurityLevel("image-dry-run", admissionapi.LevelBaseline)
	)

	g.It("should not delete resources [apigroup:image.openshift.io]", func() {
		g.By("preparing the image stream where the test image will be pushed")
		err := oc.Run("tag").Args("openshift/cli:latest", "test:latest").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		err = exutil.WaitForAnImageStreamTag(oc, oc.Namespace(), "test", "latest")
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("triggering delete operation of istag with --dry-run=server")
		err = oc.Run("delete").Args("istag/test:latest", "--dry-run=server").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("obtaining the test:latest image name")
		_, err = oc.Run("get").Args("istag", "test:latest", "-o", "jsonpath={.image.metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("triggering delete operation of imagestream with --dry-run=server")
		err = oc.Run("delete").Args("imagestream/test", "--dry-run=server").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("obtaining the test imagestream")
		_, err = oc.Run("get").Args("imagestream", "test", "-o", "jsonpath={.image.metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It("should not update resources [apigroup:image.openshift.io]", func() {

		g.By("actually creating imagestream for update dry-run test")
		err := oc.Run("create").Args("imagestream", "dryrun-test").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		defer func() {
			g.By("cleaning up imagestream")
			err := oc.Run("delete").Args("imagestream/dryrun-test", "--ignore-not-found=true").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
		}()

		g.By("verifying the imagestream was actually created")
		err = wait.PollUntilContextTimeout(context.Background(), 1*time.Second, 30*time.Second, true, func(ctx context.Context) (bool, error) {
			_, err := oc.Run("get").Args("imagestream", "dryrun-test", "-o", "jsonpath={.metadata.name}").Output()
			if err != nil {
				return false, nil // Continue polling
			}
			return true, nil // Success
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("triggering imagestream annotation update with --dry-run=server")
		err = oc.Run("annotate").Args("imagestream/dryrun-test", "test-annotation=value1", "--dry-run=server").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("verifying the annotation was not persisted")
		output, err := oc.Run("get").Args("imagestream", "dryrun-test", "-o", "jsonpath={.metadata.annotations.test-annotation}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.BeEmpty())
	})

	g.It("should not create resources [apigroup:image.openshift.io]", func() {
		g.By("triggering create operation of imagestream with --dry-run=server")
		err := oc.Run("create").Args("imagestream", "dryrun-test", "--dry-run=server").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		// wait for 10s to make sure we'll not miss any resource created accidentally
		err = wait.PollUntilContextTimeout(context.Background(), 1*time.Second, 10*time.Second, true, func(ctx context.Context) (bool, error) {
			_, err := oc.Run("get").Args("imagestream", "dryrun-test", "-o", "jsonpath={.metadata.name}").Output()
			if err == nil {
				return false, fmt.Errorf("imagestream was created when it shouldn't have been")
			}
			return false, nil
		})
		// polling must timeout otherwise it means we found the object
		o.Expect(err).To(o.Equal(context.DeadlineExceeded))

		g.By("the test imagestream must not exist")
		_, err = oc.Run("get").Args("imagestream", "dryrun-test", "-o", "jsonpath={.image.metadata.name}").Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(apierrors.IsNotFound(err)).To(o.BeTrue(), fmt.Sprintf("expected NotFound error, got: %v", err))
	})
})
