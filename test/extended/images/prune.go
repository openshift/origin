package images

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	"github.com/docker/distribution/manifest/schema1"
	"github.com/docker/distribution/manifest/schema2"

	kapi "k8s.io/kubernetes/pkg/api"

	dockerregistryserver "github.com/openshift/origin/pkg/dockerregistry/server"
	exutil "github.com/openshift/origin/test/extended/util"
	testutil "github.com/openshift/origin/test/util"
)

const testImageSize = 1024

type cleanUpContainer struct {
	imageNames []string
}

var _ = g.Describe("[images] prune images", func() {
	defer g.GinkgoRecover()
	var oc = exutil.NewCLI("prune-images", exutil.KubeConfigPath())
	var originalAcceptSchema2 *bool

	g.JustBeforeEach(func() {
		if originalAcceptSchema2 == nil {
			accepts, err := doesRegistryAcceptSchema2(oc)
			o.Expect(err).NotTo(o.HaveOccurred())
			originalAcceptSchema2 = &accepts
		}

		err := exutil.WaitForBuilderAccount(oc.KubeREST().ServiceAccounts(oc.Namespace()))
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("give a user %s a right to prune images with %s role", oc.Username(), "system:image-pruner"))
		err = oc.AsAdmin().WithoutNamespace().Run("adm").Args("policy", "add-cluster-role-to-user", "system:image-pruner", oc.Username()).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.Describe("of schema 1", func() {
		g.JustBeforeEach(func() {
			if *originalAcceptSchema2 {
				g.By("ensure the registry does not accept schema 2")
				err := ensureRegistryAcceptsSchema2(oc, false)
				o.Expect(err).NotTo(o.HaveOccurred())
			}
		})

		g.AfterEach(func() {
			if *originalAcceptSchema2 {
				err := ensureRegistryAcceptsSchema2(oc, true)
				o.Expect(err).NotTo(o.HaveOccurred())
			}
		})

		g.It("should prune old image", func() { testPruneImages(oc, 1) })
	})

	g.Describe("of schema 2", func() {
		g.JustBeforeEach(func() {
			if !*originalAcceptSchema2 {
				g.By("ensure the registry accepts schema 2")
				err := ensureRegistryAcceptsSchema2(oc, true)
				o.Expect(err).NotTo(o.HaveOccurred())
			}
		})

		g.AfterEach(func() {
			if !*originalAcceptSchema2 {
				err := ensureRegistryAcceptsSchema2(oc, false)
				o.Expect(err).NotTo(o.HaveOccurred())
			}
		})

		g.It("should prune old image with config", func() { testPruneImages(oc, 2) })
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

	oc.SetOutputDir(exutil.TestContext.OutputDir)
	outSink := g.GinkgoWriter

	cleanUp := cleanUpContainer{}
	defer tearDownPruneImagesTest(oc, &cleanUp)

	dClient, err := testutil.NewDockerClient()
	o.Expect(err).NotTo(o.HaveOccurred())

	g.By(fmt.Sprintf("build two images using Docker and push them as schema %d", schemaVersion))
	imgPruneName, err := BuildAndPushImageOfSizeWithDocker(oc, dClient, "prune", "latest", testImageSize, 2, outSink, true)
	o.Expect(err).NotTo(o.HaveOccurred())
	cleanUp.imageNames = append(cleanUp.imageNames, imgPruneName)
	pruneSize, err := getRegistryStorageSize(oc)
	o.Expect(err).NotTo(o.HaveOccurred())
	imgKeepName, err := BuildAndPushImageOfSizeWithDocker(oc, dClient, "prune", "latest", testImageSize, 2, outSink, true)
	o.Expect(err).NotTo(o.HaveOccurred())
	cleanUp.imageNames = append(cleanUp.imageNames, imgKeepName)
	keepSize, err := getRegistryStorageSize(oc)
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(pruneSize < keepSize).To(o.BeTrue())

	g.By(fmt.Sprintf("ensure uploaded image is of schema %d", schemaVersion))
	imgPrune, err := oc.AsAdmin().REST().Images().Get(imgPruneName)
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(imgPrune.DockerImageManifestMediaType).To(o.Equal(mediaType))
	imgKeep, err := oc.AsAdmin().REST().Images().Get(imgKeepName)
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

	noConfirmSize, err := getRegistryStorageSize(oc)
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
	}

	o.Expect(output).NotTo(o.ContainSubstring(imgKeepName))
	o.Expect(output).NotTo(o.ContainSubstring(imgKeep.DockerImageMetadata.ID))
	for _, layer := range imgKeep.DockerImageLayers {
		if !strings.Contains(output, layer.Name) {
			o.Expect(output).NotTo(o.ContainSubstring(layer.Name))
		}
	}

	confirmSize, err := getRegistryStorageSize(oc)
	o.Expect(err).NotTo(o.HaveOccurred())
	g.By(fmt.Sprintf("confirming storage size: sizeOfKeepImage=%d <= sizeAfterPrune=%d < beforePruneSize=%d", imgKeep.DockerImageMetadata.Size, confirmSize, keepSize))
	o.Expect(confirmSize >= imgKeep.DockerImageMetadata.Size).To(o.BeTrue())
	o.Expect(confirmSize < keepSize).To(o.BeTrue())
	g.By(fmt.Sprintf("confirming pruned size: sizeOfPruneImage=%d <= (sizeAfterPrune=%d - sizeBeforePrune=%d)", imgPrune, keepSize, confirmSize))
	o.Expect(imgPrune.DockerImageMetadata.Size <= keepSize-confirmSize).To(o.BeTrue())
}

func tearDownPruneImagesTest(oc *exutil.CLI, cleanUp *cleanUpContainer) {
	for _, image := range cleanUp.imageNames {
		err := oc.AsAdmin().REST().Images().Delete(image)
		if err != nil {
			fmt.Fprintf(g.GinkgoWriter, "clean up of image %q failed: %v\n", image, err)
		}
	}
}

func getRegistryStorageSize(oc *exutil.CLI) (int64, error) {
	ns := oc.Namespace()
	defer oc.SetNamespace(ns)
	out, err := oc.SetNamespace(kapi.NamespaceDefault).AsAdmin().Run("rsh").Args("dc/docker-registry", "du", "--bytes", "--summarize", "/registry/docker/registry").Output()
	if err != nil {
		return 0, err
	}
	m := regexp.MustCompile(`^\d+`).FindString(out)
	if len(m) == 0 {
		return 0, fmt.Errorf("failed to parse du output: %s", out)
	}

	size, err := strconv.ParseInt(m, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse du output: %s", m)
	}

	return size, nil
}

func doesRegistryAcceptSchema2(oc *exutil.CLI) (bool, error) {
	ns := oc.Namespace()
	defer oc.SetNamespace(ns)
	env, err := oc.SetNamespace(kapi.NamespaceDefault).AsAdmin().Run("env").Args("dc/docker-registry", "--list").Output()
	if err != nil {
		return false, err
	}

	return strings.Contains(env, fmt.Sprintf("%s=true", dockerregistryserver.AcceptSchema2EnvVar)), nil
}

// ensureRegistryAcceptsSchema2 checks whether the registry is configured to accept manifests V2 schema 2 or
// not. If the result doesn't match given accept argument, registry's deployment config is updated accordingly
// and the function blocks until the registry is re-deployed and ready for new requests.
func ensureRegistryAcceptsSchema2(oc *exutil.CLI, accept bool) error {
	ns := oc.Namespace()
	oc = oc.SetNamespace(kapi.NamespaceDefault).AsAdmin()
	defer oc.SetNamespace(ns)
	env, err := oc.Run("env").Args("dc/docker-registry", "--list").Output()
	if err != nil {
		return err
	}

	value := fmt.Sprintf("%s=%t", dockerregistryserver.AcceptSchema2EnvVar, accept)
	if strings.Contains(env, value) {
		if accept {
			g.By("docker-registry is already configured to accept schema 2")
		} else {
			g.By("docker-registry is already configured to refuse schema 2")
		}
		return nil
	}

	dc, err := oc.REST().DeploymentConfigs(kapi.NamespaceDefault).Get("docker-registry")
	if err != nil {
		return err
	}
	waitForVersion := dc.Status.LatestVersion + 1

	g.By("configuring Docker registry to accept schema 2")
	err = oc.Run("env").Args("dc/docker-registry", value).Execute()
	if err != nil {
		return fmt.Errorf("failed to update registry's environment with %s: %v", &waitForVersion, err)
	}
	return exutil.WaitForRegistry(oc.AdminREST(), oc.AdminKubeREST(), &waitForVersion, oc)
}
