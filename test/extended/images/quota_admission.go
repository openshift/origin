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

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
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
					"openshift.io/imagesize":         resource.MustParse("5000"),
					"openshift.io/imagestreamsize":   resource.MustParse("10000"),
					"openshift.io/projectimagessize": resource.MustParse("15000"),
				},
			},
		}

		g.By("resource quota needs to be created")
		_, err := oc.AdminKubeREST().ResourceQuotas(oc.Namespace()).Create(rq)
		o.Expect(err).NotTo(o.HaveOccurred())
		g.By("trying to push image exceeding imagesize quota")
		buildImageOfSize(oc, "sized", "big", 8000, 5, true)

		g.By("trying to push image below imagesize quota")
		buildImageOfSize(oc, "sized", "middle", 4000, 5, false)
		waitForResourceQuotaSync(oc, "all-quota-set", kapi.ResourceList{"openshift.io/projectimagessize": resource.MustParse("4000")})

		g.By("trying to push image below imagesize quota")
		buildImageOfSize(oc, "sized", "middle2", 4000, 4, false)
		waitForResourceQuotaSync(oc, "all-quota-set", kapi.ResourceList{"openshift.io/projectimagessize": resource.MustParse("8000")})

		g.By("trying to push image exceeding imagestreamsize quota")
		buildImageOfSize(oc, "sized", "middle3", 4000, 4, true)

		g.By("trying to push image below imagestreamsize quota")
		buildImageOfSize(oc, "sized", "tiny", 1000, 3, false)
		waitForResourceQuotaSync(oc, "all-quota-set", kapi.ResourceList{"openshift.io/projectimagessize": resource.MustParse("9000")})

		g.By("trying to push image below projectimagessize quota")
		buildImageOfSize(oc, "other", "middle", 4000, 5, false)
		waitForResourceQuotaSync(oc, "all-quota-set", kapi.ResourceList{"openshift.io/projectimagessize": resource.MustParse("13000")})

		g.By("trying to push image exceeding projectimagessize quota")
		buildImageOfSize(oc, "other", "middle2", 4000, 5, true)

		g.By("removing image sized:middle")
		err = oc.REST().ImageStreamTags(oc.Namespace()).Delete("sized", "middle")
		o.Expect(err).NotTo(o.HaveOccurred())
		// expect usage decrement
		_, err = exutil.WaitForResourceQuotaSync(
			oc.KubeREST().ResourceQuotas(oc.Namespace()),
			"all-quota-set",
			kapi.ResourceList{"openshift.io/projectimagessize": resource.MustParse("9000")},
			true,
			time.Second*5,
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("trying to re-push the image")
		buildImageOfSize(oc, "sized", "reloaded", 4000, 4, false)
		waitForResourceQuotaSync(oc, "all-quota-set", kapi.ResourceList{"openshift.io/projectimagessize": resource.MustParse("13000")})
	})
})

// buildImageOfSize tries to build an image of particular size and number of layers.
// Built image is stored as an image stream tag <name>:<tag>. If shouldBeDenied is true,
// a build will be expected to fail with a denied error.
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

	lines := make([]string, numberOfLayers+1)
	lines[0] = "FROM scratch"
	for i := 1; i <= numberOfLayers; i++ {
		blobName := fmt.Sprintf("data%d", i)
		err := createRandomBlob(path.Join(tempDir, blobName), size/uint64(numberOfLayers))
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
		o.Expect(out).Should(o.MatchRegexp("(?i)Failed to push image:.*requested access to the resource is denied"))
	} else {
		o.Expect(err).NotTo(o.HaveOccurred())
	}
}

// waitForResourceQuotaSync waits until a usage of a quota reaches given limit with a short timeout
func waitForResourceQuotaSync(oc *exutil.CLI, name string, expectedResources kapi.ResourceList) error {
	g.By(fmt.Sprintf("waiting for resource quota %s to get updated", name))
	used, err := exutil.WaitForResourceQuotaSync(
		oc.KubeREST().ResourceQuotas(oc.Namespace()),
		"all-quota-set",
		expectedResources,
		false,
		time.Second*5,
	)
	if err != nil {
		return err
	}
	for k, v := range expectedResources {
		u := used[k]
		if v.Cmp(u) != 0 {
			return fmt.Errorf("Used value does not equal expected for resource %q: %s != %s", k, u.String(), v.String())
		}
	}
	return nil
}

// createRandomBlob creates a random data with bytes from `letters` in order
// to let docker take advantage of compression
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
