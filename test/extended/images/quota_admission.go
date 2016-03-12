package images

import (
	cryptorand "crypto/rand"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/resource"
	"k8s.io/kubernetes/pkg/quota"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	imageapi "github.com/openshift/origin/pkg/image/api"
	exutil "github.com/openshift/origin/test/extended/util"
	testutil "github.com/openshift/origin/test/util"
)

const (
	// There are coefficients used to multiply layer data size to get a rough size of uploaded blob.
	layerSizeMultiplierForDocker18     = 2.0
	layerSizeMultiplierForLatestDocker = 0.8

	imageSizeHardQuota        = "10000"
	imageStreamSizeHardQuota  = "20000"
	projectImageSizeHardQuota = "30000"

	bigImageSize    = 14000
	middleImageSize = 8000
	tinyImageSize   = 1000
)

var _ = g.Describe("[images] openshift image resource quota", func() {
	defer g.GinkgoRecover()
	var oc = exutil.NewCLI("imagequota-admission", exutil.KubeConfigPath())

	g.JustBeforeEach(func() {
		g.By("Waiting for builder service account")
		err := exutil.WaitForBuilderAccount(oc.KubeREST().ServiceAccounts(oc.Namespace()))
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It(fmt.Sprintf("should deny a push of built image exceeding quota"), func() {
		oc.SetOutputDir(exutil.TestContext.OutputDir)

		rq := &kapi.ResourceQuota{
			ObjectMeta: kapi.ObjectMeta{
				Name: "all-quota-set",
			},
			Spec: kapi.ResourceQuotaSpec{
				Hard: kapi.ResourceList{
					imageapi.ResourceImageSize:         resource.MustParse(imageSizeHardQuota),
					imageapi.ResourceImageStreamSize:   resource.MustParse(imageStreamSizeHardQuota),
					imageapi.ResourceProjectImagesSize: resource.MustParse(projectImageSizeHardQuota),
				},
			},
		}

		g.By("resource quota needs to be created")
		_, err := oc.AdminKubeREST().ResourceQuotas(oc.Namespace()).Create(rq)
		o.Expect(err).NotTo(o.HaveOccurred())
		g.By("trying to push image exceeding imagesize quota")
		buildImageOfSize(oc, "sized", "big", bigImageSize, 3, true)

		g.By("trying to push image below imagesize quota")
		buildImageOfSize(oc, "sized", "middle", middleImageSize, 2, false)
		expected := kapi.ResourceList{imageapi.ResourceProjectImagesSize: *resource.NewQuantity(middleImageSize/2, resource.BinarySI)}
		used, err := waitForResourceQuotaSync(oc, "all-quota-set", expected)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("trying to push image below imagesize quota")
		buildImageOfSize(oc, "sized", "middle2", middleImageSize, 2, false)
		expected = quota.Add(used, kapi.ResourceList{imageapi.ResourceProjectImagesSize: *resource.NewQuantity(middleImageSize/2, resource.BinarySI)})
		used, err = waitForResourceQuotaSync(oc, "all-quota-set", expected)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("trying to push image exceeding imagestreamsize quota")
		buildImageOfSize(oc, "sized", "middle3", middleImageSize, 2, true)

		g.By("trying to push image below imagestreamsize quota")
		buildImageOfSize(oc, "sized", "tiny", tinyImageSize, 1, false)
		expected = quota.Add(used, kapi.ResourceList{imageapi.ResourceProjectImagesSize: *resource.NewQuantity(tinyImageSize/2, resource.BinarySI)})
		used, err = waitForResourceQuotaSync(oc, "all-quota-set", expected)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("trying to push image below projectimagessize quota")
		buildImageOfSize(oc, "other", "middle", middleImageSize, 2, false)
		expected = quota.Add(used, kapi.ResourceList{imageapi.ResourceProjectImagesSize: *resource.NewQuantity(middleImageSize/2, resource.BinarySI)})
		used, err = waitForResourceQuotaSync(oc, "all-quota-set", expected)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("trying to push image exceeding projectimagessize quota")
		buildImageOfSize(oc, "other", "middle2", middleImageSize, 2, true)

		expected = quota.Subtract(used, kapi.ResourceList{imageapi.ResourceProjectImagesSize: *resource.NewQuantity(middleImageSize/2, resource.BinarySI)})
		g.By("removing image sized:middle")
		err = oc.REST().ImageStreamTags(oc.Namespace()).Delete("sized", "middle")
		o.Expect(err).NotTo(o.HaveOccurred())
		// expect usage decrement
		used, err = exutil.WaitForResourceQuotaSync(
			oc.KubeREST().ResourceQuotas(oc.Namespace()),
			"all-quota-set",
			expected,
			true,
			time.Second*5,
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("trying to re-push the image")
		buildImageOfSize(oc, "sized", "reloaded", middleImageSize, 2, false)
		expected = quota.Add(used, kapi.ResourceList{imageapi.ResourceProjectImagesSize: *resource.NewQuantity(middleImageSize/2, resource.BinarySI)})
		_, err = waitForResourceQuotaSync(oc, "all-quota-set", expected)
		o.Expect(err).NotTo(o.HaveOccurred())
	})
})

// buildImageOfSize tries to build an image of wanted size and number of layers. Built image is stored as an
// image stream tag <name>:<tag>. If shouldBeDenied is true, a build will be expected to fail with a denied
// error. Note the size is only approximate. Resulting image size will be different depending on used
// compression algorithm and metadata overhead.
func buildImageOfSize(oc *exutil.CLI, name, tag string, size uint64, numberOfLayers int, shouldBeDenied bool) {
	istName := name
	if tag != "" {
		istName += ":" + tag
	}
	g.By(fmt.Sprintf("building an image %q of size %d", istName, size))

	bc, err := oc.REST().BuildConfigs(oc.Namespace()).Get(name)
	if err == nil {
		g.By(fmt.Sprintf("changing build config %s to store result into %s", name, istName))
		o.Expect(bc.Spec.BuildSpec.Output.To.Kind).To(o.Equal("ImageStreamTag"))
		bc.Spec.BuildSpec.Output.To.Name = istName
		_, err := oc.REST().BuildConfigs(oc.Namespace()).Update(bc)
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

	dataSize := calculateRoughDataSize(size, numberOfLayers)

	lines := make([]string, numberOfLayers+1)
	lines[0] = "FROM scratch"
	for i := 1; i <= numberOfLayers; i++ {
		blobName := fmt.Sprintf("data%d", i)
		err := createRandomBlob(path.Join(tempDir, blobName), dataSize)
		o.Expect(err).NotTo(o.HaveOccurred())
		lines[i] = fmt.Sprintf("COPY %s /%s", blobName, blobName)
	}
	err = ioutil.WriteFile(path.Join(tempDir, "Dockerfile"), []byte(strings.Join(lines, "\n")+"\n"), 0644)
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

// waitForResourceQuotaSync waits until a usage of a quota reaches given limit with a short timeout
func waitForResourceQuotaSync(oc *exutil.CLI, name string, expectedResources kapi.ResourceList) (kapi.ResourceList, error) {
	g.By(fmt.Sprintf("waiting for resource quota %s to get updated", name))
	used, err := exutil.WaitForResourceQuotaSync(
		oc.KubeREST().ResourceQuotas(oc.Namespace()),
		"all-quota-set",
		expectedResources,
		false,
		time.Second*5,
	)
	if err != nil {
		return nil, err
	}
	return used, nil
}

// createRandomBlob creates a random data with bytes from `letters` in order to let docker take advantage of
// compression
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

// dockerVersion is a cached version string of Docker daemon
var dockerVersion = ""

// getDockerVersion returns a version of running Docker daemon which is questioned only during the first
// invocation.
func getDockerVersion() (major, minor int, version string, err error) {
	reVersion := regexp.MustCompile(`^(\d+)\.(\d+)`)

	if dockerVersion == "" {
		client, err2 := testutil.NewDockerClient()
		if err = err2; err != nil {
			return
		}
		env, err2 := client.Version()
		if err = err2; err != nil {
			return
		}
		dockerVersion = env.Get("Version")
		g.By(fmt.Sprintf("using docker version %s", version))
	}
	version = dockerVersion

	matches := reVersion.FindStringSubmatch(version)
	if len(matches) < 3 {
		return 0, 0, "", fmt.Errorf("failed to parse version string %s", version)
	}
	major, _ = strconv.Atoi(matches[1])
	minor, _ = strconv.Atoi(matches[2])
	return
}

// calculateRoughDataSize returns a rough size of data blob to generate in order to build an image of wanted
// size. Image is comprised of numberOfLayers layers of the same size.
func calculateRoughDataSize(wantedImageSize uint64, numberOfLayers int) uint64 {
	major, minor, version, err := getDockerVersion()
	if err != nil {
		g.By(fmt.Sprintf("failed to get docker version: %s", version))
	}
	if (major >= 1 && minor >= 9) || version == "" {
		// running Docker version 1.9+
		return uint64(float64(wantedImageSize) / (float64(numberOfLayers) * layerSizeMultiplierForLatestDocker))
	}

	// running Docker daemon < 1.9
	return uint64(float64(wantedImageSize) / (float64(numberOfLayers) * layerSizeMultiplierForDocker18))
}
