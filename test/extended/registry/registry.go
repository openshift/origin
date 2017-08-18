package registry

import (
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	dockerClient "github.com/fsouza/go-dockerclient"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	regclient "github.com/openshift/origin/pkg/dockerregistry"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	imagesutil "github.com/openshift/origin/test/extended/images"
	registryutil "github.com/openshift/origin/test/extended/registry/util"
	exutil "github.com/openshift/origin/test/extended/util"
	testutil "github.com/openshift/origin/test/util"
)

const (
	repoName  = "app"
	tagName   = "latest"
	imageSize = 1024
)

var _ = g.Describe("[Conformance][registry][migration] manifest migration from etcd to registry storage", func() {
	defer g.GinkgoRecover()
	var oc = exutil.NewCLI("registry-migration", exutil.KubeConfigPath())

	g.It("registry can get access to manifest [local]", func() {
		oc.SetOutputDir(exutil.TestContext.OutputDir)
		cleanUp := imagesutil.NewCleanUpContainer(oc)
		defer cleanUp.Run()

		g.By("set up policy for registry to have anonymous access to images")
		err := oc.Run("policy").Args("add-role-to-user", "registry-viewer", "system:anonymous").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		dClient, err := testutil.NewDockerClient()
		o.Expect(err).NotTo(o.HaveOccurred())

		registryURL, err := registryutil.GetDockerRegistryURL(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("pushing image...")
		imageDigest, _, err := imagesutil.BuildAndPushImageOfSizeWithDocker(oc, dClient, repoName, tagName, imageSize, 1, g.GinkgoWriter, true, true)
		o.Expect(err).NotTo(o.HaveOccurred())
		cleanUp.AddImage(imageDigest, "", "")

		g.By("checking that the image converted...")
		image, err := oc.AsAdmin().Client().Images().Get(imageDigest, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(len(image.DockerImageManifest)).Should(o.Equal(0))
		imageMetadataNotEmpty(image)

		g.By("getting image manifest from docker-registry...")
		conn, err := regclient.NewClient(10*time.Second, true).Connect(registryURL, true)
		o.Expect(err).NotTo(o.HaveOccurred())

		_, manifest, err := conn.ImageManifest(oc.Namespace(), repoName, tagName)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(len(manifest)).Should(o.BeNumerically(">", 0))

		g.By("restoring manifest...")
		image, err = oc.AsAdmin().Client().Images().Get(imageDigest, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		imageMetadataNotEmpty(image)

		image.DockerImageManifest = string(manifest)

		newImage, err := oc.AsAdmin().Client().Images().Update(image)
		o.Expect(err).NotTo(o.HaveOccurred())
		imageMetadataNotEmpty(newImage)

		g.By("checking that the manifest is present in the image...")
		image, err = oc.AsAdmin().Client().Images().Get(imageDigest, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(len(image.DockerImageManifest)).Should(o.BeNumerically(">", 0))
		o.Expect(image.DockerImageManifest).Should(o.Equal(string(manifest)))
		imageMetadataNotEmpty(image)

		g.By("getting image manifest from docker-registry one more time...")
		_, manifest, err = conn.ImageManifest(oc.Namespace(), repoName, tagName)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(len(manifest)).Should(o.BeNumerically(">", 0))

		g.By("waiting until image is updated...")
		err = waitForImageUpdate(oc, image)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("checking that the manifest was removed from the image...")
		image, err = oc.AsAdmin().Client().Images().Get(imageDigest, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(len(image.DockerImageManifest)).Should(o.Equal(0))
		imageMetadataNotEmpty(image)

		g.By("getting image manifest from docker-registry to check if he's available...")
		_, manifest, err = conn.ImageManifest(oc.Namespace(), repoName, tagName)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(len(manifest)).Should(o.BeNumerically(">", 0))

		g.By("pulling image...")
		authCfg, err := exutil.BuildAuthConfiguration(registryURL, oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		opts := dockerClient.PullImageOptions{
			Repository: authCfg.ServerAddress + "/" + oc.Namespace() + "/" + repoName,
			Tag:        tagName,
		}
		err = dClient.PullImage(opts, *authCfg)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("removing image...")
		err = dClient.RemoveImage(opts.Repository)
		o.Expect(err).NotTo(o.HaveOccurred())
	})
})

func imageMetadataNotEmpty(image *imageapi.Image) {
	o.Expect(len(image.DockerImageMetadata.ID)).Should(o.BeNumerically(">", 0))
	o.Expect(len(image.DockerImageMetadata.Container)).Should(o.BeNumerically(">", 0))
	o.Expect(len(image.DockerImageMetadata.DockerVersion)).Should(o.BeNumerically(">", 0))
	o.Expect(len(image.DockerImageMetadata.Architecture)).Should(o.BeNumerically(">", 0))
}

func waitForImageUpdate(oc *exutil.CLI, image *imageapi.Image) error {
	return wait.Poll(200*time.Millisecond, 2*time.Minute, func() (bool, error) {
		newImage, err := oc.AsAdmin().Client().Images().Get(image.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		return (image.ResourceVersion < newImage.ResourceVersion), nil
	})
}
