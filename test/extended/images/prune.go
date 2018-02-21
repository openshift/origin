package images

import (
	"fmt"
	"sort"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	"github.com/docker/distribution/manifest/schema1"
	"github.com/docker/distribution/manifest/schema2"
	godigest "github.com/opencontainers/go-digest"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	exutil "github.com/openshift/origin/test/extended/util"
	testutil "github.com/openshift/origin/test/util"
)

const (
	testImageSize     = 1024
	mirrorBlobTimeout = time.Second * 30
	// this image has a high number of relatively small blobs
	externalImageReference = "docker.io/openshift/origin-release:golang-1.4"
)

var _ = g.Describe("[Feature:ImagePrune][registry][Serial] Image prune", func() {
	defer g.GinkgoRecover()
	var oc = exutil.NewCLI("prune-images", exutil.KubeConfigPath())

	var originalAcceptSchema2 *bool

	g.JustBeforeEach(func() {
		if originalAcceptSchema2 == nil {
			accepts, err := DoesRegistryAcceptSchema2(oc)
			o.Expect(err).NotTo(o.HaveOccurred())
			originalAcceptSchema2 = &accepts
		}

		err := exutil.WaitForBuilderAccount(oc.KubeClient().Core().ServiceAccounts(oc.Namespace()))
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("give a user %s a right to prune images with %s role", oc.Username(), "system:image-pruner"))
		err = oc.AsAdmin().WithoutNamespace().Run("adm").Args("policy", "add-cluster-role-to-user", "system:image-pruner", oc.Username()).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.Describe("of schema 1", func() {
		g.JustBeforeEach(func() {
			var err error
			isRedeployed := false
			if *originalAcceptSchema2 {
				g.By("ensure the registry does not accept schema 2")
				isRedeployed, err = EnsureRegistryAcceptsSchema2(oc, false)
				o.Expect(err).NotTo(o.HaveOccurred())
			}
			if !isRedeployed {
				_, err = RedeployRegistry(oc)
				o.Expect(err).NotTo(o.HaveOccurred())
			}
		})

		g.AfterEach(func() {
			if *originalAcceptSchema2 {
				_, err := EnsureRegistryAcceptsSchema2(oc, true)
				o.Expect(err).NotTo(o.HaveOccurred())
			}
		})

		g.It("should prune old image", func() { testPruneImages(oc, 1) })
	})

	g.Describe("of schema 2", func() {
		g.JustBeforeEach(func() {
			var err error
			isRedeployed := false
			if !*originalAcceptSchema2 {
				g.By("ensure the registry accepts schema 2")
				isRedeployed, err = EnsureRegistryAcceptsSchema2(oc, true)
				o.Expect(err).NotTo(o.HaveOccurred())
			}
			if !isRedeployed {
				_, err = RedeployRegistry(oc)
				o.Expect(err).NotTo(o.HaveOccurred())
			}
		})

		g.AfterEach(func() {
			if !*originalAcceptSchema2 {
				_, err := EnsureRegistryAcceptsSchema2(oc, false)
				o.Expect(err).NotTo(o.HaveOccurred())
			}
		})

		g.It("should prune old image with config", func() { testPruneImages(oc, 2) })
	})

	g.Describe("with --prune-registry==false", func() {
		g.JustBeforeEach(func() {
			var err error
			isRedeployed := false
			if !*originalAcceptSchema2 {
				g.By("ensure the registry accepts schema 2")
				isRedeployed, err = EnsureRegistryAcceptsSchema2(oc, true)
				o.Expect(err).NotTo(o.HaveOccurred())
			}
			if !isRedeployed {
				_, err = RedeployRegistry(oc)
				o.Expect(err).NotTo(o.HaveOccurred())
			}
		})

		g.AfterEach(func() {
			if !*originalAcceptSchema2 {
				_, err := EnsureRegistryAcceptsSchema2(oc, false)
				o.Expect(err).NotTo(o.HaveOccurred())
			}
		})

		g.It("should prune old image but skip registry", func() { testSoftPruneImages(oc) })
	})

	g.Describe("with default --all flag", func() {
		g.JustBeforeEach(func() {
			var err error
			isRedeployed := false
			if !*originalAcceptSchema2 {
				g.By("ensure the registry accepts schema 2")
				isRedeployed, err = EnsureRegistryAcceptsSchema2(oc, true)
				o.Expect(err).NotTo(o.HaveOccurred())
			}
			if !isRedeployed {
				_, err = RedeployRegistry(oc)
				o.Expect(err).NotTo(o.HaveOccurred())
			}
		})

		g.AfterEach(func() {
			if !*originalAcceptSchema2 {
				_, err := EnsureRegistryAcceptsSchema2(oc, false)
				o.Expect(err).NotTo(o.HaveOccurred())
			}
		})

		g.It("should prune both internally managed and external images", func() { testPruneAllImages(oc, true, 2) })
	})

	g.Describe("with --all=false flag", func() {
		g.JustBeforeEach(func() {
			var err error
			isRedeployed := false
			if !*originalAcceptSchema2 {
				g.By("ensure the registry accepts schema 2")
				isRedeployed, err = EnsureRegistryAcceptsSchema2(oc, true)
				o.Expect(err).NotTo(o.HaveOccurred())
			}
			if !isRedeployed {
				_, err = RedeployRegistry(oc)
				o.Expect(err).NotTo(o.HaveOccurred())
			}
		})

		g.AfterEach(func() {
			if !*originalAcceptSchema2 {
				_, err := EnsureRegistryAcceptsSchema2(oc, false)
				o.Expect(err).NotTo(o.HaveOccurred())
			}
		})

		g.It("should prune only internally managed images", func() { testPruneAllImages(oc, false, 2) })
	})
})

func testPruneImages(oc *exutil.CLI, schemaVersion int) {
	var mediaType string
	switch schemaVersion {
	case 1:
		mediaType = schema1.MediaTypeManifest
	case 2:
		mediaType = schema2.MediaTypeManifest
	default:
		g.Fail(fmt.Sprintf("unexpected schema version %d", schemaVersion))
	}

	isName := "prune"
	repoName := oc.Namespace() + "/" + isName

	oc.SetOutputDir(exutil.TestContext.OutputDir)
	outSink := g.GinkgoWriter

	cleanUp := NewCleanUpContainer(oc)
	defer cleanUp.Run()

	dClient, err := testutil.NewDockerClient()
	o.Expect(err).NotTo(o.HaveOccurred())

	g.By(fmt.Sprintf("build two images using Docker and push them as schema %d", schemaVersion))
	imgPruneName, _, err := BuildAndPushImageOfSizeWithDocker(oc, dClient, isName, "latest", testImageSize, 2, outSink, true, true)
	o.Expect(err).NotTo(o.HaveOccurred())
	cleanUp.AddImage(imgPruneName, "", "")
	cleanUp.AddImageStream(isName)
	pruneSize, err := GetRegistryStorageSize(oc)
	o.Expect(err).NotTo(o.HaveOccurred())
	imgKeepName, _, err := BuildAndPushImageOfSizeWithDocker(oc, dClient, isName, "latest", testImageSize, 2, outSink, true, true)
	o.Expect(err).NotTo(o.HaveOccurred())
	cleanUp.AddImage(imgKeepName, "", "")
	keepSize, err := GetRegistryStorageSize(oc)
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(pruneSize < keepSize).To(o.BeTrue())

	g.By(fmt.Sprintf("ensure uploaded image is of schema %d", schemaVersion))
	imgPrune, err := oc.AsAdmin().ImageClient().Image().Images().Get(imgPruneName, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(imgPrune.DockerImageManifestMediaType).To(o.Equal(mediaType))
	imgKeep, err := oc.AsAdmin().ImageClient().Image().Images().Get(imgKeepName, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(imgKeep.DockerImageManifestMediaType).To(o.Equal(mediaType))

	g.By("prune the first image uploaded (dry-run)")
	output, err := oc.WithoutNamespace().Run("adm").Args("prune", "images", "--keep-tag-revisions=1", "--keep-younger-than=0").Output()

	g.By("verify images, layers and configs about to be pruned")
	o.Expect(output).To(o.ContainSubstring(imgPruneName))
	if schemaVersion == 1 {
		o.Expect(output).NotTo(o.ContainSubstring(imgPrune.DockerImageMetadata.ID))
	} else {
		o.Expect(output).To(o.ContainSubstring(imgPrune.DockerImageMetadata.ID))
	}
	for _, layer := range imgPrune.DockerImageLayers {
		if !strings.Contains(output, layer.Name) {
			o.Expect(output).To(o.ContainSubstring(layer.Name))
		}
	}

	o.Expect(output).NotTo(o.ContainSubstring(imgKeepName))
	o.Expect(output).NotTo(o.ContainSubstring(imgKeep.DockerImageMetadata.ID))
	for _, layer := range imgKeep.DockerImageLayers {
		if !strings.Contains(output, layer.Name) {
			o.Expect(output).NotTo(o.ContainSubstring(layer.Name))
		}
	}

	noConfirmSize, err := GetRegistryStorageSize(oc)
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(noConfirmSize).To(o.Equal(keepSize))

	g.By("prune the first image uploaded (confirm)")
	output, err = oc.WithoutNamespace().Run("adm").Args("prune", "images", "--keep-tag-revisions=1", "--keep-younger-than=0", "--confirm").Output()

	g.By("verify images, layers and configs about to be pruned")
	o.Expect(output).To(o.ContainSubstring(imgPruneName))
	if schemaVersion == 1 {
		o.Expect(output).NotTo(o.ContainSubstring(imgPrune.DockerImageMetadata.ID))
	} else {
		o.Expect(output).To(o.ContainSubstring(imgPrune.DockerImageMetadata.ID))
	}
	for _, layer := range imgPrune.DockerImageLayers {
		if !strings.Contains(output, layer.Name) {
			o.Expect(output).To(o.ContainSubstring(layer.Name))
		}
		globally, inRepository, err := IsBlobStoredInRegistry(oc, godigest.Digest(layer.Name), repoName)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(globally).To(o.BeFalse())
		o.Expect(inRepository).To(o.BeFalse())
	}

	o.Expect(output).NotTo(o.ContainSubstring(imgKeepName))
	o.Expect(output).NotTo(o.ContainSubstring(imgKeep.DockerImageMetadata.ID))
	for _, layer := range imgKeep.DockerImageLayers {
		if !strings.Contains(output, layer.Name) {
			o.Expect(output).NotTo(o.ContainSubstring(layer.Name))
		}
		globally, inRepository, err := IsBlobStoredInRegistry(oc, godigest.Digest(layer.Name), repoName)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(globally).To(o.BeTrue())
		o.Expect(inRepository).To(o.BeTrue())
	}

	confirmSize, err := GetRegistryStorageSize(oc)
	o.Expect(err).NotTo(o.HaveOccurred())
	g.By(fmt.Sprintf("confirming storage size: sizeOfKeepImage=%d <= sizeAfterPrune=%d < beforePruneSize=%d", imgKeep.DockerImageMetadata.Size, confirmSize, keepSize))
	o.Expect(confirmSize >= imgKeep.DockerImageMetadata.Size).To(o.BeTrue())
	o.Expect(confirmSize < keepSize).To(o.BeTrue())
	g.By(fmt.Sprintf("confirming pruned size: sizeOfPruneImage=%d <= (sizeAfterPrune=%d - sizeBeforePrune=%d)", imgPrune.DockerImageMetadata.Size, keepSize, confirmSize))
	o.Expect(imgPrune.DockerImageMetadata.Size <= keepSize-confirmSize).To(o.BeTrue())
}

func testSoftPruneImages(oc *exutil.CLI) {
	isName := "prune"

	oc.SetOutputDir(exutil.TestContext.OutputDir)
	outSink := g.GinkgoWriter

	cleanUp := NewCleanUpContainer(oc)
	defer cleanUp.Run()

	dClient, err := testutil.NewDockerClient()
	o.Expect(err).NotTo(o.HaveOccurred())

	g.By("build two images using Docker and push them")
	imgPruneName, _, err := BuildAndPushImageOfSizeWithDocker(oc, dClient, isName, "latest", testImageSize, 2, outSink, true, true)
	o.Expect(err).NotTo(o.HaveOccurred())
	cleanUp.AddImage(imgPruneName, "", "")
	cleanUp.AddImageStream(isName)
	pruneSize, err := GetRegistryStorageSize(oc)
	o.Expect(err).NotTo(o.HaveOccurred())

	g.By("prune the first image uploaded (confirm, skipping registry)")
	output, err := oc.WithoutNamespace().Run("adm").Args("prune", "images", "--keep-tag-revisions=1", "--keep-younger-than=0", "--confirm", "--prune-registry=false").Output()

	g.By("verify images, layers and configs about to be pruned")
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(output).NotTo(o.ContainSubstring(imgPruneName))
	o.Expect(output).To(o.ContainSubstring("Only API objects will be removed"))

	skipRegistrySize, err := GetRegistryStorageSize(oc)
	o.Expect(err).NotTo(o.HaveOccurred())
	g.By(fmt.Sprintf("confirming storage size: sizeAfterPrune=%d == beforePruneSize=%d", skipRegistrySize, pruneSize))
	o.Expect(skipRegistrySize == pruneSize).To(o.BeTrue())
}

func testPruneAllImages(oc *exutil.CLI, setAllImagesToFalse bool, schemaVersion int) {
	isName := fmt.Sprintf("prune-schema%d-all-images-%t", schemaVersion, setAllImagesToFalse)
	repository := oc.Namespace() + "/" + isName

	oc.SetOutputDir(exutil.TestContext.OutputDir)
	outSink := g.GinkgoWriter

	cleanUp := NewCleanUpContainer(oc)
	defer cleanUp.Run()

	dClient, err := testutil.NewDockerClient()
	o.Expect(err).NotTo(o.HaveOccurred())

	g.By("build one image using Docker and push it")
	managedImageName, _, err := BuildAndPushImageOfSizeWithDocker(oc, dClient, isName, "latest", testImageSize, 2, outSink, true, true)
	o.Expect(err).NotTo(o.HaveOccurred())
	cleanUp.AddImage(managedImageName, "", "")
	cleanUp.AddImageStream(isName)
	o.Expect(err).NotTo(o.HaveOccurred())

	managedImage, err := oc.AsAdmin().ImageClient().Image().Images().Get(managedImageName, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	externalImage, blobdgst, err := importImageAndMirrorItsSmallestBlob(oc, externalImageReference, "origin-release:latest")
	o.Expect(err).NotTo(o.HaveOccurred())
	cleanUp.AddImage(externalImage.Name, "", "")
	cleanUp.AddImageStream("origin-release")

	checkAdminPruneOutput := func(output string, dryRun bool) {
		o.Expect(output).To(o.ContainSubstring(managedImage.Name))
		for _, layer := range managedImage.DockerImageLayers {
			o.Expect(output).To(o.ContainSubstring(layer.Name))
		}

		for _, layer := range managedImage.DockerImageLayers {
			o.Expect(output).To(o.ContainSubstring(layer.Name))
			globally, inRepository, err := IsBlobStoredInRegistry(oc, godigest.Digest(layer.Name), repository)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(globally).To(o.Equal(dryRun))
			o.Expect(inRepository).To(o.Equal(dryRun))
		}

		if setAllImagesToFalse {
			o.Expect(output).NotTo(o.ContainSubstring(externalImage.Name))
		} else {
			o.Expect(output).To(o.ContainSubstring(externalImage.Name))
		}

		for _, layer := range externalImage.DockerImageLayers {
			if setAllImagesToFalse {
				o.Expect(output).NotTo(o.ContainSubstring(layer.Name))
			} else {
				o.Expect(output).To(o.ContainSubstring(layer.Name))
			}
			// check for a presence of blob that we chose to mirror, not any other
			if blobdgst.String() != layer.Name {
				continue
			}
			globally, inRepository, err := IsBlobStoredInRegistry(oc, godigest.Digest(layer.Name), repository)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(globally).To(o.Equal(dryRun || setAllImagesToFalse))
			// mirrored blobs are not linked into any repository/_layers directory
			o.Expect(inRepository).To(o.BeFalse())
		}
	}

	args := []string{"prune", "images", "--keep-tag-revisions=0", "--keep-younger-than=0"}
	if setAllImagesToFalse {
		args = append(args, "--all=false")
	}

	g.By(fmt.Sprintf("dry-running oc adm %s", strings.Join(args, " ")))
	output, err := oc.WithoutNamespace().Run("adm").Args(args...).Output()

	g.By("verify images, layers and configs about to be pruned")
	checkAdminPruneOutput(output, true)

	args = append(args, "--confirm")
	g.By(fmt.Sprintf("running oc adm %s", strings.Join(args, " ")))
	output, err = oc.WithoutNamespace().Run("adm").Args(args...).Output()

	g.By("verify that blobs have been pruned")
	checkAdminPruneOutput(output, false)
}

type byLayerSize []imageapi.ImageLayer

func (bls byLayerSize) Len() int      { return len(bls) }
func (bls byLayerSize) Swap(i, j int) { bls[i], bls[j] = bls[j], bls[i] }
func (bls byLayerSize) Less(i, j int) bool {
	if bls[i].LayerSize < bls[j].LayerSize {
		return true
	}
	if bls[i].LayerSize == bls[j].LayerSize && bls[i].Name < bls[j].Name {
		return true
	}
	return false
}

func importImageAndMirrorItsSmallestBlob(oc *exutil.CLI, imageReference, destISTag string) (*imageapi.Image, godigest.Digest, error) {
	g.By(fmt.Sprintf("importing external image %q", imageReference))
	err := oc.Run("tag").Args("--source=docker", imageReference, destISTag).Execute()
	if err != nil {
		return nil, "", err
	}
	isName, tag, ok := imageapi.SplitImageStreamTag(destISTag)
	if !ok {
		return nil, "", fmt.Errorf("failed to parse image stream tag %q", destISTag)
	}
	err = exutil.WaitForAnImageStreamTag(oc, oc.Namespace(), isName, tag)
	if err != nil {
		return nil, "", err
	}
	istag, err := oc.ImageClient().Image().ImageStreamTags(oc.Namespace()).Get(destISTag, metav1.GetOptions{})
	if err != nil {
		return nil, "", err
	}

	tmpLayers := make([]imageapi.ImageLayer, 0, len(istag.Image.DockerImageLayers))
	for i := range istag.Image.DockerImageLayers {
		layer := istag.Image.DockerImageLayers[i]
		// skip empty blobs
		if IsEmptyDigest(godigest.Digest(layer.Name)) {
			continue
		}
		tmpLayers = append(tmpLayers, layer)
	}
	sort.Sort(byLayerSize(tmpLayers))
	if len(tmpLayers) == 0 {
		return nil, "", fmt.Errorf("failed to find any non-empty blob in image %q", imageReference)
	}

	layer := tmpLayers[0]
	g.By(fmt.Sprintf("mirroring image's blob of size=%d in repository %q", layer.LayerSize, isName))
	err = MirrorBlobInRegistry(oc, godigest.Digest(layer.Name), oc.Namespace()+"/"+isName, mirrorBlobTimeout)
	if err != nil {
		return nil, "", err
	}

	return &istag.Image, godigest.Digest(tmpLayers[0].Name), nil
}
