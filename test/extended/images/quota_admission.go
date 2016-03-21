package images

import (
	cryptorand "crypto/rand"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/resource"
	kerrutil "k8s.io/kubernetes/pkg/util/errors"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	imageapi "github.com/openshift/origin/pkg/image/api"
	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	imageSize = 100

	quotaName = "all-quota-set"

	waitTimeout = time.Second * 5
)

var _ = g.Describe("[images] openshift image resource quota", func() {
	defer g.GinkgoRecover()
	var oc = exutil.NewCLI("imagequota-admission", exutil.KubeConfigPath())

	g.JustBeforeEach(func() {
		g.By("Waiting for builder service account")
		err := exutil.WaitForBuilderAccount(oc.KubeREST().ServiceAccounts(oc.Namespace()))
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.AfterEach(func() {
		g.By(fmt.Sprintf("Deleting quota %s", quotaName))
		oc.AdminKubeREST().ResourceQuotas(oc.Namespace()).Delete(quotaName)

		g.By("Deleting images")
		iss, err := oc.AdminREST().ImageStreams(oc.Namespace()).List(kapi.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		for _, is := range iss.Items {
			for _, history := range is.Status.Tags {
				for i := range history.Items {
					oc.AdminREST().Images().Delete(history.Items[i].Image)
				}
			}
			for _, tagRef := range is.Spec.Tags {
				switch tagRef.From.Kind {
				case "ImageStreamImage":
					nameParts := strings.Split(tagRef.From.Name, "@")
					if len(nameParts) != 2 {
						continue
					}
					imageName := nameParts[1]
					oc.AdminREST().Images().Delete(imageName)
				}
			}
			err := oc.AdminREST().ImageStreams(is.Namespace).Delete(is.Name)
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		g.By("Deleting shared project")
		oc.AdminREST().Projects().Delete(oc.Namespace() + "-shared")
	})

	g.It("should deny a push of image exceeding quota", func() {
		oc.SetOutputDir(exutil.TestContext.OutputDir)

		rq := &kapi.ResourceQuota{
			ObjectMeta: kapi.ObjectMeta{
				Name: quotaName,
			},
			Spec: kapi.ResourceQuotaSpec{
				Hard: kapi.ResourceList{
					imageapi.ResourceImages: resource.MustParse("0"),
				},
			},
		}

		g.By("resource quota needs to be created")
		_, err := oc.AdminKubeREST().ResourceQuotas(oc.Namespace()).Create(rq)
		o.Expect(err).NotTo(o.HaveOccurred())
		g.By(fmt.Sprintf("trying to push image exceeding %s=%d quota", imageapi.ResourceImages, 0))
		buildAndPushImage(oc, oc.Namespace(), "sized", "refused", true)

		g.By(fmt.Sprintf("bump the %q quota to %d", imageapi.ResourceImages, 1))
		rq, err = oc.AdminKubeREST().ResourceQuotas(oc.Namespace()).Get(quotaName)
		o.Expect(err).NotTo(o.HaveOccurred())
		rq.Spec.Hard[imageapi.ResourceImages] = resource.MustParse("1")
		_, err = oc.AdminKubeREST().ResourceQuotas(oc.Namespace()).Update(rq)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("trying to push image below %s=%d quota", imageapi.ResourceImages, 1))
		buildAndPushImage(oc, oc.Namespace(), "sized", "first", false)
		used, err := waitForResourceQuotaSync(oc, quotaName, rq.Spec.Hard)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(assertQuotasEqual(used, rq.Spec.Hard)).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("trying to push image exceeding %s=%d quota", imageapi.ResourceImages, 1))
		buildAndPushImage(oc, oc.Namespace(), "sized", "second", true)

		g.By(fmt.Sprintf("trying to push image exceeding %s=%q quota to another repository", imageapi.ResourceImages, 1))
		buildAndPushImage(oc, oc.Namespace(), "other", "third", true)

		g.By(fmt.Sprintf("bump the %q quota to %d", imageapi.ResourceImages, 2))
		rq, err = oc.AdminKubeREST().ResourceQuotas(oc.Namespace()).Get(quotaName)
		o.Expect(err).NotTo(o.HaveOccurred())
		rq.Spec.Hard[imageapi.ResourceImages] = resource.MustParse("2")
		_, err = oc.AdminKubeREST().ResourceQuotas(oc.Namespace()).Update(rq)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("trying to push image below %s=%d quota", imageapi.ResourceImages, 2))
		buildAndPushImage(oc, oc.Namespace(), "other", "second", false)
		used, err = waitForResourceQuotaSync(oc, quotaName, rq.Spec.Hard)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(assertQuotasEqual(used, rq.Spec.Hard)).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("trying to push image exceeding %s=%d quota", imageapi.ResourceImages, 2))
		buildAndPushImage(oc, oc.Namespace(), "other", "refused", true)

		g.By(fmt.Sprintf("trying to push image exceeding %s=%d quota to a new repository", imageapi.ResourceImages, 2))
		buildAndPushImage(oc, oc.Namespace(), "new", "refused", true)

		g.By("removing image sized:first")
		err = oc.REST().ImageStreamTags(oc.Namespace()).Delete("sized", "first")
		o.Expect(err).NotTo(o.HaveOccurred())
		// expect usage decrement
		used, err = exutil.WaitForResourceQuotaSync(
			oc.KubeREST().ResourceQuotas(oc.Namespace()),
			quotaName,
			kapi.ResourceList{imageapi.ResourceImages: resource.MustParse("1")},
			true,
			time.Second*5)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("trying to push image below %s=%d quota", imageapi.ResourceImages, 2))
		buildAndPushImage(oc, oc.Namespace(), "sized", "foo", false)
		used, err = waitForResourceQuotaSync(oc, quotaName, rq.Spec.Hard)
		o.Expect(assertQuotasEqual(used, rq.Spec.Hard)).NotTo(o.HaveOccurred())
	})

	g.It("should deny a tagging of an image exceeding quota", func() {
		oc.SetOutputDir(exutil.TestContext.OutputDir)

		projectName := oc.Namespace()
		sharedProjectName := projectName + "-shared"
		g.By(fmt.Sprintf("create a new project %s to store shared images", sharedProjectName))
		err := oc.Run("new-project").Args(sharedProjectName).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		oc.SetNamespace(sharedProjectName)

		tag2Image := make(map[string]imageapi.Image)
		for i := 1; i <= 5; i++ {
			tag := fmt.Sprintf("%d", i)
			buildAndPushImage(oc, sharedProjectName, "src", tag, false)
			ist, err := oc.REST().ImageStreamTags(sharedProjectName).Get("src", tag)
			o.Expect(err).NotTo(o.HaveOccurred())
			tag2Image[tag] = ist.Image
		}

		g.By(fmt.Sprintf("switch back to the original project %s", projectName))
		err = oc.Run("project").Args(projectName).Execute()
		oc.SetNamespace(projectName)
		o.Expect(err).NotTo(o.HaveOccurred())

		rq := &kapi.ResourceQuota{
			ObjectMeta: kapi.ObjectMeta{
				Name: quotaName,
			},
			Spec: kapi.ResourceQuotaSpec{Hard: kapi.ResourceList{imageapi.ResourceImages: resource.MustParse("0")}},
		}

		g.By(fmt.Sprintf("creating resource quota with a limit %s=%d", imageapi.ResourceImages, 0))
		_, err = oc.AdminKubeREST().ResourceQuotas(oc.Namespace()).Create(rq)
		o.Expect(err).NotTo(o.HaveOccurred())
		waitForLimitSync(oc, quotaName, rq.Spec.Hard)

		g.By("waiting for resource quota to get in sync")
		err = waitForLimitSync(oc, quotaName, rq.Spec.Hard)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("trying to tag an image exceeding %s=%d quota", imageapi.ResourceImages, 0))
		out, err := oc.Run("tag").Args(sharedProjectName+"/src:1", "is:1").Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(out).Should(o.MatchRegexp("(?i)exceeded quota"))
		o.Expect(out).Should(o.ContainSubstring(string(imageapi.ResourceImages)))

		g.By(fmt.Sprintf("bump the %s quota to %d", imageapi.ResourceImages, 1))
		rq, err = oc.AdminKubeREST().ResourceQuotas(oc.Namespace()).Get(quotaName)
		o.Expect(err).NotTo(o.HaveOccurred())
		rq.Spec.Hard[imageapi.ResourceImages] = resource.MustParse("1")
		_, err = oc.AdminKubeREST().ResourceQuotas(oc.Namespace()).Update(rq)
		o.Expect(err).NotTo(o.HaveOccurred())
		err = waitForLimitSync(oc, quotaName, rq.Spec.Hard)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("trying to tag an image below %s=%d quota", imageapi.ResourceImages, 1))
		out, err = oc.Run("tag").Args(sharedProjectName+"/src:1", "is:1").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		used, err := waitForResourceQuotaSync(oc, quotaName, rq.Spec.Hard)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(assertQuotasEqual(used, rq.Spec.Hard)).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("trying to tag an image exceeding %s=%d quota", imageapi.ResourceImages, 1))
		out, err = oc.Run("tag").Args(sharedProjectName+"/src:2", "is:2").Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(out).Should(o.MatchRegexp("(?i)exceeded quota"))
		o.Expect(out).Should(o.ContainSubstring(string(imageapi.ResourceImages)))

		g.By("re-tagging the image under different tag")
		out, err = oc.Run("tag").Args(sharedProjectName+"/src:1", "is:1again").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		used, err = waitForResourceQuotaSync(oc, quotaName, rq.Spec.Hard)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(assertQuotasEqual(used, rq.Spec.Hard)).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("bump the %s quota to %d", imageapi.ResourceImages, 2))
		rq, err = oc.AdminKubeREST().ResourceQuotas(oc.Namespace()).Get(quotaName)
		o.Expect(err).NotTo(o.HaveOccurred())
		rq.Spec.Hard[imageapi.ResourceImages] = resource.MustParse("2")
		_, err = oc.AdminKubeREST().ResourceQuotas(oc.Namespace()).Update(rq)
		o.Expect(err).NotTo(o.HaveOccurred())
		err = waitForLimitSync(oc, quotaName, rq.Spec.Hard)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("trying to alias tag a second image below %s=%d quota", imageapi.ResourceImages, 2))
		out, err = oc.Run("tag").Args("--alias", "--source=istag", sharedProjectName+"/src:2", "other:2").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		used, err = waitForResourceQuotaSync(oc, quotaName, rq.Spec.Hard)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(assertQuotasEqual(used, rq.Spec.Hard)).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("trying to alias tag a second image exceeding %s=%d quota", imageapi.ResourceImages, 2))
		out, err = oc.Run("tag").Args("--alias", "--source=istag", sharedProjectName+"/src:3", "other:3").Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(out).Should(o.MatchRegexp("(?i)exceeded quota"))
		o.Expect(out).Should(o.ContainSubstring(string(imageapi.ResourceImages)))

		g.By("re-tagging the image under different tag")
		out, err = oc.Run("tag").Args("--alias", "--source=istag", sharedProjectName+"/src:2", "another:2again").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		used, err = waitForResourceQuotaSync(oc, quotaName, rq.Spec.Hard)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(assertQuotasEqual(used, rq.Spec.Hard)).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("bump the %s quota to %d", imageapi.ResourceImages, 3))
		rq, err = oc.AdminKubeREST().ResourceQuotas(oc.Namespace()).Get(quotaName)
		o.Expect(err).NotTo(o.HaveOccurred())
		rq.Spec.Hard[imageapi.ResourceImages] = resource.MustParse("3")
		_, err = oc.AdminKubeREST().ResourceQuotas(oc.Namespace()).Update(rq)
		o.Expect(err).NotTo(o.HaveOccurred())
		err = waitForLimitSync(oc, quotaName, rq.Spec.Hard)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("trying to create ImageStreamTag referencing isimage below %s=%d limit", imageapi.ResourceImages, 3))
		ist := &imageapi.ImageStreamTag{
			ObjectMeta: kapi.ObjectMeta{
				Name: "dest:3",
			},
			Tag: &imageapi.TagReference{
				Name: "3",
				From: &kapi.ObjectReference{
					Kind:      "ImageStreamImage",
					Namespace: sharedProjectName,
					Name:      "src@" + tag2Image["3"].Name,
				},
			},
		}
		ist, err = oc.REST().ImageStreamTags(oc.Namespace()).Update(ist)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("trying to create ImageStreamTag referencing isimage exceeding %s=%d quota", imageapi.ResourceImages, 3))
		ist = &imageapi.ImageStreamTag{
			ObjectMeta: kapi.ObjectMeta{
				Name: "dest:4",
			},
			Tag: &imageapi.TagReference{
				Name: "4",
				From: &kapi.ObjectReference{
					Kind:      "ImageStreamImage",
					Namespace: sharedProjectName,
					Name:      "src@" + tag2Image["4"].Name,
				},
			},
		}
		_, err = oc.REST().ImageStreamTags(oc.Namespace()).Update(ist)
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(err.Error()).Should(o.MatchRegexp("(?i)exceeded quota"))

		g.By(fmt.Sprintf("bump the %s quota to %d", imageapi.ResourceImages, 4))
		rq, err = oc.AdminKubeREST().ResourceQuotas(oc.Namespace()).Get(quotaName)
		o.Expect(err).NotTo(o.HaveOccurred())
		rq.Spec.Hard[imageapi.ResourceImages] = resource.MustParse("4")
		_, err = oc.AdminKubeREST().ResourceQuotas(oc.Namespace()).Update(rq)
		o.Expect(err).NotTo(o.HaveOccurred())
		err = waitForLimitSync(oc, quotaName, rq.Spec.Hard)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("trying to create ImageStreamTag referencing istag below %s=%d limit", imageapi.ResourceImages, 4))
		ist = &imageapi.ImageStreamTag{
			ObjectMeta: kapi.ObjectMeta{
				Name: "dest:4",
			},
			Tag: &imageapi.TagReference{
				Name: "4",
				From: &kapi.ObjectReference{
					Kind:      "ImageStreamTag",
					Namespace: sharedProjectName,
					Name:      "src:4",
				},
			},
		}
		ist, err = oc.REST().ImageStreamTags(oc.Namespace()).Update(ist)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("trying to create ImageStreamTag referencing istag exceeding %s=%d quota", imageapi.ResourceImages, 4))
		ist = &imageapi.ImageStreamTag{
			ObjectMeta: kapi.ObjectMeta{
				Name: "dest:5",
			},
			Tag: &imageapi.TagReference{
				Name: "5",
				From: &kapi.ObjectReference{
					Kind:      "ImageStreamTag",
					Namespace: sharedProjectName,
					Name:      "src:5",
				},
			},
		}
		_, err = oc.REST().ImageStreamTags(oc.Namespace()).Update(ist)
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(err.Error()).Should(o.MatchRegexp("(?i)exceeded quota"))

		/** Test referencing using DockerImage.
		 *
		 * TODO: this is now failing during the image import with following status:
		 *   Reason:"Unauthorized", Message:"you may not have access to the Docker image \"172.30.163.69:5000/extended-test-imagequota-admission-6znr2-shared/src@sha256:bb08da7a0b99af72aebe0ead77009d28771a30dbe4d99f49bf3d29407c72e48f\""
		 * Uncomment once resolved (issue #7985).
		 * The import seems to be working when the source and destination namespaces are the same.

		g.By(fmt.Sprintf("trying to tag a docker image below %s=%d quota", imageapi.ResourceImages, 3))
		err = oc.Run("import-image").Args("stream:dockerimage", "--confirm", "--insecure", "--from", name2Image["3"].DockerImageReference).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		used, err = waitForResourceQuotaSync(oc, quotaName, rq.Spec.Hard)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(assertQuotasEqual(used, rq.Spec.Hard)).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("trying to tag a docker image exceeding %s=%d quota", imageapi.ResourceImages, 3))
		err = waitForAnImageStreamTag(oc, "stream", "dockerimage")
		o.Expect(err).NotTo(o.HaveOccurred())
		is, err := oc.REST().ImageStreams(oc.Namespace()).Get("stream")
		o.Expect(err).NotTo(o.HaveOccurred())
		is.Spec.Tags["foo"] = imageapi.TagReference{
			Name: "foo",
			From: &kapi.ObjectReference{
				Kind: "DockerImage",
				Name: name2Image["4"].DockerImageReference,
			},
			ImportPolicy: imageapi.TagImportPolicy{
				Insecure: true,
			},
		}
		_, err = oc.REST().ImageStreams(oc.Namespace()).Update(is)
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(kerrors.IsForbidden(err)).To(o.Equal(true))

		g.By("re-tagging the image under different tag")
		is, err = oc.REST().ImageStreams(oc.Namespace()).Get("stream")
		o.Expect(err).NotTo(o.HaveOccurred())
		is.Spec.Tags["duplicate"] = imageapi.TagReference{
			Name: "duplicate",
			From: &kapi.ObjectReference{
				Kind: "DockerImage",
				Name: name2Image["3"].DockerImageReference,
			},
			ImportPolicy: imageapi.TagImportPolicy{
				Insecure: true,
			},
		}
		_, err = oc.REST().ImageStreams(oc.Namespace()).Update(is)
		o.Expect(err).NotTo(o.HaveOccurred())
		err = waitForAnImageStreamTag(oc, "stream", "duplicate")
		o.Expect(err).NotTo(o.HaveOccurred())
		used, err = waitForResourceQuotaSync(oc, quotaName, rq.Spec.Hard)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(assertQuotasEqual(used, rq.Spec.Hard)).NotTo(o.HaveOccurred())
		*/
	})
})

// buildAndPushImage tries to build an image. The image is stored as an image stream tag <name>:<tag>. If
// shouldBeDenied is true, a build will be expected to fail with a denied error.
func buildAndPushImage(oc *exutil.CLI, namespace, name, tag string, shouldBeDenied bool) {
	istName := name
	if tag != "" {
		istName += ":" + tag
	}
	g.By(fmt.Sprintf("building an image %q", istName))

	bc, err := oc.REST().BuildConfigs(namespace).Get(name)
	if err == nil {
		g.By(fmt.Sprintf("changing build config %s to store result into %s", name, istName))
		o.Expect(bc.Spec.BuildSpec.Output.To.Kind).To(o.Equal("ImageStreamTag"))
		bc.Spec.BuildSpec.Output.To.Name = istName
		_, err := oc.REST().BuildConfigs(namespace).Update(bc)
		o.Expect(err).NotTo(o.HaveOccurred())
	} else {
		g.By(fmt.Sprintf("creating a new build config %s with output to %s ", name, istName))
		err = oc.Run("new-build").Args(
			"--binary",
			"--name", name,
			"--to", istName).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
	}

	tempDir, err := ioutil.TempDir("", "name-build")
	o.Expect(err).NotTo(o.HaveOccurred())

	err = createRandomBlob(path.Join(tempDir, "data"), imageSize)
	o.Expect(err).NotTo(o.HaveOccurred())
	err = ioutil.WriteFile(path.Join(tempDir, "Dockerfile"), []byte("FROM scratch\nCOPY data /data\n"), 0644)
	o.Expect(err).NotTo(o.HaveOccurred())

	err = oc.Run("start-build").Args(name, "--from-dir", tempDir, "--wait").Execute()
	if shouldBeDenied {
		o.Expect(err).To(o.HaveOccurred())
		out, err := oc.Run("logs").Args("bc/" + name).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).Should(o.MatchRegexp("(?i)Failed to push image:.*denied"))
	} else {
		o.Expect(err).NotTo(o.HaveOccurred())
	}
}

// assertQuotasEqual compares two quota sets and returns an error with proper description when they don't match
func assertQuotasEqual(a, b kapi.ResourceList) error {
	errs := []error{}
	if len(a) != len(b) {
		errs = append(errs, fmt.Errorf("number of items does not match (%d != %d)", len(a), len(b)))
	}

	for k, av := range a {
		if bv, exists := b[k]; exists {
			if av.Cmp(bv) != 0 {
				errs = append(errs, fmt.Errorf("a[%s] != b[%s] (%s != %s)", k, k, av.String(), bv.String()))
			}
		} else {
			errs = append(errs, fmt.Errorf("resource %q not present in b", k))
		}
	}

	for k := range b {
		if _, exists := a[k]; !exists {
			errs = append(errs, fmt.Errorf("resource %q not present in a", k))
		}
	}

	return kerrutil.NewAggregate(errs)
}

// waitForResourceQuotaSync waits until a usage of a quota reaches given limit with a short timeout
func waitForResourceQuotaSync(oc *exutil.CLI, name string, expectedResources kapi.ResourceList) (kapi.ResourceList, error) {
	g.By(fmt.Sprintf("waiting for resource quota %s to get updated", name))
	used, err := exutil.WaitForResourceQuotaSync(
		oc.KubeREST().ResourceQuotas(oc.Namespace()),
		quotaName,
		expectedResources,
		false,
		waitTimeout,
	)
	if err != nil {
		return nil, err
	}
	return used, nil
}

// waitForAnImageStreamTag waits until an image stream with given name has non-empty history for given tag
func waitForAnImageStreamTag(oc *exutil.CLI, name, tag string) error {
	g.By(fmt.Sprintf("waiting for an is importer to import a tag %s into a stream %s", tag, name))
	start := time.Now()
	c := make(chan error)
	go func() {
		err := exutil.WaitForAnImageStream(
			oc.REST().ImageStreams(oc.Namespace()),
			name,
			func(is *imageapi.ImageStream) bool {
				if history, exists := is.Status.Tags[tag]; !exists || len(history.Items) == 0 {
					return false
				}
				return true
			},
			func(is *imageapi.ImageStream) bool {
				return time.Now().After(start.Add(waitTimeout))
			})
		c <- err
	}()

	select {
	case e := <-c:
		return e
	case <-time.After(waitTimeout):
		return fmt.Errorf("timed out while waiting of an image stream tag %s/%s:%s", oc.Namespace(), name, tag)
	}
}

// waitForResourceQuotaSync waits until a usage of a quota reaches given limit with a short timeout
func waitForLimitSync(oc *exutil.CLI, name string, hardLimit kapi.ResourceList) error {
	g.By(fmt.Sprintf("waiting for resource quota %s to get updated", name))
	return exutil.WaitForResourceQuotaLimitSync(
		oc.KubeREST().ResourceQuotas(oc.Namespace()),
		quotaName,
		hardLimit,
		waitTimeout)
}

// createRandomBlob creates a random data with bytes from `letters` in order to let docker take advantage of
// compression. Resulting layer size will be different due to file metadata overhead and compression.
func createRandomBlob(dest string, size uint64) error {
	var letters = []byte("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()

	data := make([]byte, size)
	if _, err = cryptorand.Read(data); err != nil {
		return err
	}

	for i := range data {
		data[i] = letters[uint(data[i])%uint(len(letters))]
	}

	f.Write(data)
	return nil
}
