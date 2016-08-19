package imageapis

import (
	"fmt"
	"os"
	"strconv"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/resource"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	imageapi "github.com/openshift/origin/pkg/image/api"
	quotautil "github.com/openshift/origin/pkg/quota/util"
	imagesutil "github.com/openshift/origin/test/extended/images"
	exutil "github.com/openshift/origin/test/extended/util"
	testutil "github.com/openshift/origin/test/util"
)

const limitRangeName = "limits"

var _ = g.Describe("[imageapis] openshift limit range admission", func() {
	defer g.GinkgoRecover()
	var oc = exutil.NewCLI("limitrange-admission", exutil.KubeConfigPath())

	g.JustBeforeEach(func() {
		g.By("Waiting for builder service account")
		err := exutil.WaitForBuilderAccount(oc.KubeREST().ServiceAccounts(oc.Namespace()))
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	// needs to be run at the of of each It; cannot be run in AfterEach which is run after the project
	// is destroyed
	tearDown := func(oc *exutil.CLI) {
		g.By(fmt.Sprintf("Deleting limit range %s", limitRangeName))
		oc.AdminKubeREST().LimitRanges(oc.Namespace()).Delete(limitRangeName)

		deleteTestImagesAndStreams(oc)
	}

	g.It(fmt.Sprintf("should deny a push of built image exceeding %s limit", imageapi.LimitTypeImage), func() {
		oc.SetOutputDir(exutil.TestContext.OutputDir)
		defer tearDown(oc)

		dClient, err := testutil.NewDockerClient()
		o.Expect(err).NotTo(o.HaveOccurred())

		_, err = createLimitRangeOfType(oc, imageapi.LimitTypeImage, kapi.ResourceList{
			kapi.ResourceStorage: resource.MustParse("10Ki"),
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("trying to push an image exceeding size limit with just 1 layer"))
		err = imagesutil.BuildAndPushImageOfSizeWithBuilder(oc, dClient, oc.Namespace(), "sized", "middle", 16000, 1, false)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("trying to push an image exceeding size limit in total"))
		err = imagesutil.BuildAndPushImageOfSizeWithBuilder(oc, dClient, oc.Namespace(), "sized", "middle", 16000, 5, false)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("trying to push an image with one big layer below size limit"))
		err = imagesutil.BuildAndPushImageOfSizeWithBuilder(oc, dClient, oc.Namespace(), "sized", "small", 8000, 1, true)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("trying to push an image below size limit"))
		err = imagesutil.BuildAndPushImageOfSizeWithBuilder(oc, dClient, oc.Namespace(), "sized", "small", 8000, 2, true)
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It(fmt.Sprintf("should deny a push of built image exceeding limit on %s resource", imageapi.ResourceImageStreamImages), func() {
		oc.SetOutputDir(exutil.TestContext.OutputDir)
		defer tearDown(oc)

		limits := kapi.ResourceList{
			imageapi.ResourceImageStreamTags:   resource.MustParse("0"),
			imageapi.ResourceImageStreamImages: resource.MustParse("0"),
		}
		_, err := createLimitRangeOfType(oc, imageapi.LimitTypeImageStream, limits)
		o.Expect(err).NotTo(o.HaveOccurred())

		dClient, err := testutil.NewDockerClient()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("trying to push image exceeding limits %v", limits))
		err = imagesutil.BuildAndPushImageOfSizeWithBuilder(oc, dClient, oc.Namespace(), "sized", "refused", imageSize, 1, false)
		o.Expect(err).NotTo(o.HaveOccurred())

		limits, err = bumpLimit(oc, imageapi.ResourceImageStreamImages, "1")
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("trying to push image below limits %v", limits))
		err = imagesutil.BuildAndPushImageOfSizeWithBuilder(oc, dClient, oc.Namespace(), "sized", "first", imageSize, 2, true)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("trying to push image exceeding limits %v", limits))
		err = imagesutil.BuildAndPushImageOfSizeWithBuilder(oc, dClient, oc.Namespace(), "sized", "second", imageSize, 2, false)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("trying to push image below limits %v to another image stream", limits))
		err = imagesutil.BuildAndPushImageOfSizeWithBuilder(oc, dClient, oc.Namespace(), "another", "second", imageSize, 1, true)
		o.Expect(err).NotTo(o.HaveOccurred())

		limits, err = bumpLimit(oc, imageapi.ResourceImageStreamImages, "2")
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("trying to push image below limits %v", limits))
		err = imagesutil.BuildAndPushImageOfSizeWithBuilder(oc, dClient, oc.Namespace(), "another", "third", imageSize, 1, true)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("trying to push image exceeding limits %v", limits))
		err = imagesutil.BuildAndPushImageOfSizeWithBuilder(oc, dClient, oc.Namespace(), "another", "fourth", imageSize, 1, false)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By(`removing tag "second" from "another" image stream`)
		err = oc.REST().ImageStreamTags(oc.Namespace()).Delete("another", "second")
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("trying to push image below limits %v", limits))
		err = imagesutil.BuildAndPushImageOfSizeWithBuilder(oc, dClient, oc.Namespace(), "another", "replenish", imageSize, 1, true)
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It(fmt.Sprintf("should deny a docker image reference exceeding limit on %s resource", imageapi.ResourceImageStreamTags), func() {
		oc.SetOutputDir(exutil.TestContext.OutputDir)
		defer tearDown(oc)

		tag2Image, err := buildAndPushTestImagesTo(oc, "src", "tag", 2)
		o.Expect(err).NotTo(o.HaveOccurred())

		limit := kapi.ResourceList{imageapi.ResourceImageStreamTags: resource.MustParse("0")}
		_, err = createLimitRangeOfType(oc, imageapi.LimitTypeImageStream, limit)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("trying to tag a docker image exceeding limit %v", limit))
		out, err := oc.Run("import-image").Args("stream:dockerimage", "--confirm", "--insecure", "--from", tag2Image["tag1"].DockerImageReference).Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(out).Should(o.ContainSubstring("exceeds the maximum limit"))
		o.Expect(out).Should(o.ContainSubstring(string(imageapi.ResourceImageStreamTags)))

		limit, err = bumpLimit(oc, imageapi.ResourceImageStreamTags, "1")
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("trying to tag a docker image below limit %v", limit))
		err = oc.Run("import-image").Args("stream:dockerimage", "--confirm", "--insecure", "--from", tag2Image["tag1"].DockerImageReference).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = exutil.WaitForAnImageStreamTag(oc, oc.Namespace(), "stream", "dockerimage")
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("trying to tag a docker image exceeding limit %v", limit))
		is, err := oc.REST().ImageStreams(oc.Namespace()).Get("stream")
		o.Expect(err).NotTo(o.HaveOccurred())
		is.Spec.Tags["foo"] = imageapi.TagReference{
			Name: "foo",
			From: &kapi.ObjectReference{
				Kind: "DockerImage",
				Name: tag2Image["tag2"].DockerImageReference,
			},
			ImportPolicy: imageapi.TagImportPolicy{
				Insecure: true,
			},
		}
		_, err = oc.REST().ImageStreams(oc.Namespace()).Update(is)
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(quotautil.IsErrorQuotaExceeded(err)).Should(o.Equal(true))

		g.By("re-tagging the image under different tag")
		is, err = oc.REST().ImageStreams(oc.Namespace()).Get("stream")
		o.Expect(err).NotTo(o.HaveOccurred())
		is.Spec.Tags["duplicate"] = imageapi.TagReference{
			Name: "duplicate",
			From: &kapi.ObjectReference{
				Kind: "DockerImage",
				Name: tag2Image["tag1"].DockerImageReference,
			},
			ImportPolicy: imageapi.TagImportPolicy{
				Insecure: true,
			},
		}
		_, err = oc.REST().ImageStreams(oc.Namespace()).Update(is)
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It(fmt.Sprintf("should deny an import of a repository exceeding limit on %s resource", imageapi.ResourceImageStreamTags), func() {
		oc.SetOutputDir(exutil.TestContext.OutputDir)
		defer tearDown(oc)

		maxBulkImport, err := getMaxImagesBulkImportedPerRepository()
		o.Expect(err).NotTo(o.HaveOccurred())

		s1tag2Image, err := buildAndPushTestImagesTo(oc, "src1st", "tag", maxBulkImport+1)
		s2tag2Image, err := buildAndPushTestImagesTo(oc, "src2nd", "t", 2)
		o.Expect(err).NotTo(o.HaveOccurred())

		limit := kapi.ResourceList{
			imageapi.ResourceImageStreamTags:   *resource.NewQuantity(int64(maxBulkImport)+1, resource.DecimalSI),
			imageapi.ResourceImageStreamImages: *resource.NewQuantity(int64(maxBulkImport)+1, resource.DecimalSI),
		}
		_, err = createLimitRangeOfType(oc, imageapi.LimitTypeImageStream, limit)
		o.Expect(err).NotTo(o.HaveOccurred())

		s1ref, err := imageapi.ParseDockerImageReference(s1tag2Image["tag1"].DockerImageReference)
		o.Expect(err).NotTo(o.HaveOccurred())
		s1ref.Tag = ""
		s1ref.ID = ""
		s2ref, err := imageapi.ParseDockerImageReference(s2tag2Image["t1"].DockerImageReference)
		o.Expect(err).NotTo(o.HaveOccurred())
		s2ref.Tag = ""
		s2ref.ID = ""

		g.By(fmt.Sprintf("trying to import from repository %q below quota %v", s1ref.Exact(), limit))
		err = oc.Run("import-image").Args("bulkimport", "--confirm", "--insecure", "--all", "--from", s1ref.Exact()).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = exutil.WaitForAnImageStreamTag(oc, oc.Namespace(), "bulkimport", "tag1")
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("trying to import tags from repository %q exceeding quota %v", s2ref.Exact(), limit))
		out, err := oc.Run("import-image").Args("bulkimport", "--confirm", "--insecure", "--all", "--from", s2ref.Exact()).Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(out).Should(o.ContainSubstring("exceeds the maximum limit"))
		o.Expect(out).Should(o.ContainSubstring(string(imageapi.ResourceImageStreamTags)))
		o.Expect(out).Should(o.ContainSubstring(string(imageapi.ResourceImageStreamImages)))
	})
})

// buildAndPushTestImagesTo builds a given number of test images. The images are pushed to a new image stream
// of given name under <tagPrefix><X> where X is a number of image starting from 1.
func buildAndPushTestImagesTo(oc *exutil.CLI, isName string, tagPrefix string, numberOfImages int) (tag2Image map[string]imageapi.Image, err error) {
	dClient, err := testutil.NewDockerClient()
	if err != nil {
		return
	}
	tag2Image = make(map[string]imageapi.Image)

	for i := 1; i <= numberOfImages; i++ {
		tag := fmt.Sprintf("%s%d", tagPrefix, i)
		dgst, err := imagesutil.BuildAndPushImageOfSizeWithDocker(oc, dClient, isName, tag, imageSize, 2, g.GinkgoWriter, true)
		if err != nil {
			return nil, err
		}
		ist, err := oc.REST().ImageStreamTags(oc.Namespace()).Get(isName, tag)
		if err != nil {
			return nil, err
		}
		if dgst != ist.Image.Name {
			return nil, fmt.Errorf("digest of built image does not match stored: %s != %s", dgst, ist.Image.Name)
		}
		tag2Image[tag] = ist.Image
	}

	return
}

// createLimitRangeOfType creates a new limit range object with given limits for given limit type in current namespace
func createLimitRangeOfType(oc *exutil.CLI, limitType kapi.LimitType, maxLimits kapi.ResourceList) (*kapi.LimitRange, error) {
	lr := &kapi.LimitRange{
		ObjectMeta: kapi.ObjectMeta{
			Name: limitRangeName,
		},
		Spec: kapi.LimitRangeSpec{
			Limits: []kapi.LimitRangeItem{
				{
					Type: limitType,
					Max:  maxLimits,
				},
			},
		},
	}

	g.By(fmt.Sprintf("creating limit range object %q with %s limited to: %v", limitRangeName, limitType, maxLimits))
	lr, err := oc.AdminKubeREST().LimitRanges(oc.Namespace()).Create(lr)
	return lr, err
}

// bumpLimit changes the limit value for given resource for all the limit types of limit range object
func bumpLimit(oc *exutil.CLI, resourceName kapi.ResourceName, limit string) (kapi.ResourceList, error) {
	g.By(fmt.Sprintf("bump a limit on resource %q to %s", resourceName, limit))
	lr, err := oc.AdminKubeREST().LimitRanges(oc.Namespace()).Get(limitRangeName)
	if err != nil {
		return nil, err
	}
	res := kapi.ResourceList{}

	change := false
	for i := range lr.Spec.Limits {
		item := &lr.Spec.Limits[i]
		if old, exists := item.Max[resourceName]; exists {
			for k, v := range item.Max {
				res[k] = v
			}
			parsed := resource.MustParse(limit)
			if old.Cmp(parsed) != 0 {
				item.Max[resourceName] = parsed
				change = true
			}
		}
	}

	if !change {
		return res, nil
	}
	_, err = oc.AdminKubeREST().LimitRanges(oc.Namespace()).Update(lr)
	return res, err
}

// getMaxImagesBulkImportedPerRepository returns a maximum numbers of images that can be imported from
// repository at once. The value is obtained from environment variable which must be set.
func getMaxImagesBulkImportedPerRepository() (int, error) {
	max := os.Getenv("MAX_IMAGES_BULK_IMPORTED_PER_REPOSITORY")
	if len(max) == 0 {
		return 0, fmt.Errorf("MAX_IMAGES_BULK_IMAGES_IMPORTED_PER_REPOSITORY needs to be set")
	}
	return strconv.Atoi(max)
}
