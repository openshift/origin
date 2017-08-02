package images

import (
	"fmt"
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	"github.com/docker/distribution/manifest/schema2"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	registryutil "github.com/openshift/origin/test/extended/registry/util"
	exutil "github.com/openshift/origin/test/extended/util"
	testutil "github.com/openshift/origin/test/util"
)

var _ = g.Describe("[Feature:ImagePrune] Image hard prune", func() {
	defer g.GinkgoRecover()
	var oc = exutil.NewCLI("prune-images", exutil.KubeConfigPath())
	var originalAcceptSchema2 *bool

	g.JustBeforeEach(func() {
		if originalAcceptSchema2 == nil {
			accepts, err := registryutil.DoesRegistryAcceptSchema2(oc)
			o.Expect(err).NotTo(o.HaveOccurred())
			originalAcceptSchema2 = &accepts
		}

		readOnly := false
		acceptSchema2 := true
		err := registryutil.ConfigureRegistry(oc,
			registryutil.RegistryConfiguration{
				ReadOnly:      &readOnly,
				AcceptSchema2: &acceptSchema2,
			})
		o.Expect(err).NotTo(o.HaveOccurred())

		err = exutil.WaitForBuilderAccount(oc.KubeClient().Core().ServiceAccounts(oc.Namespace()))
		o.Expect(err).NotTo(o.HaveOccurred())

		defer func(ns string) { oc.SetNamespace(ns) }(oc.Namespace())
		g.By(fmt.Sprintf("give a user %s a right to prune images with %s role", oc.Username(), "system:image-pruner"))
		err = oc.AsAdmin().WithoutNamespace().Run("adm").Args("policy", "add-cluster-role-to-user", "system:image-pruner", oc.Username()).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().SetNamespace(metav1.NamespaceDefault).Run("adm").
			Args("policy", "add-cluster-role-to-user", "system:image-pruner",
				fmt.Sprintf("system:serviceaccount:%s:registry", metav1.NamespaceDefault)).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		// make sure to remove all unreferenced blobs from the storage
		_, err = RunHardPrune(oc, false)
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.AfterEach(func() {
		readOnly := false
		err := registryutil.ConfigureRegistry(oc,
			registryutil.RegistryConfiguration{
				ReadOnly:      &readOnly,
				AcceptSchema2: originalAcceptSchema2,
			})
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	mergeOrSetExpectedDeletions := func(expected, new *RegistryStorageFiles, merge bool) *RegistryStorageFiles {
		if !merge {
			return new
		}
		for _, repo := range new.Repos {
			expected.Repos = append(expected.Repos, repo)
		}
		for name, links := range new.ManifestLinks {
			expected.ManifestLinks.Add(name, links...)
		}
		for name, links := range new.LayerLinks {
			expected.LayerLinks.Add(name, links...)
		}
		for _, blob := range new.Blobs {
			expected.Blobs = append(expected.Blobs, blob)
		}
		return expected
	}

	testHardPrune := func(dryRun bool) {
		oc.SetOutputDir(exutil.TestContext.OutputDir)
		outSink := g.GinkgoWriter
		registryURL, err := registryutil.GetDockerRegistryURL(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		cleanUp := NewCleanUpContainer(oc)
		defer cleanUp.Run()

		dClient, err := testutil.NewDockerClient()
		o.Expect(err).NotTo(o.HaveOccurred())

		baseImg1, imageId, err := BuildAndPushImageOfSizeWithDocker(oc, dClient, "a", "latest", testImageSize, 2, outSink, true, false)
		o.Expect(err).NotTo(o.HaveOccurred())
		cleanUp.AddImage(baseImg1, imageId, "")
		baseImg1Spec := fmt.Sprintf("%s/%s/a:latest", registryURL, oc.Namespace())

		baseImg2, imageId, err := BuildAndPushImageOfSizeWithDocker(oc, dClient, "b", "latest", testImageSize, 2, outSink, true, false)
		o.Expect(err).NotTo(o.HaveOccurred())
		cleanUp.AddImage(baseImg2, imageId, "")
		baseImg2Spec := fmt.Sprintf("%s/%s/b:latest", registryURL, oc.Namespace())

		baseImg3, imageId, err := BuildAndPushImageOfSizeWithDocker(oc, dClient, "c", "latest", testImageSize, 2, outSink, true, false)
		o.Expect(err).NotTo(o.HaveOccurred())
		cleanUp.AddImage(baseImg3, imageId, "")
		baseImg3Spec := fmt.Sprintf("%s/%s/c:latest", registryURL, oc.Namespace())

		baseImg4, imageId, err := BuildAndPushImageOfSizeWithDocker(oc, dClient, "a", "img4", testImageSize, 2, outSink, true, false)
		o.Expect(err).NotTo(o.HaveOccurred())
		cleanUp.AddImage(baseImg4, imageId, "")

		childImg1, imageId, err := BuildAndPushChildImage(oc, dClient, baseImg1Spec, "c", "latest", 1, outSink, true)
		o.Expect(err).NotTo(o.HaveOccurred())
		cleanUp.AddImage(childImg1, "", "")
		childImg2, imageId, err := BuildAndPushChildImage(oc, dClient, baseImg2Spec, "b", "latest", 1, outSink, true)
		o.Expect(err).NotTo(o.HaveOccurred())
		cleanUp.AddImage(childImg2, "", "")
		childImg3, imageId, err := BuildAndPushChildImage(oc, dClient, baseImg3Spec, "c", "latest", 1, outSink, true)
		o.Expect(err).NotTo(o.HaveOccurred())
		cleanUp.AddImage(childImg3, "", "")

		err = oc.Run("tag").Args("--source=istag", "a:latest", "a-tagged:latest").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		imgs := map[string]*imageapi.Image{}
		for _, imgName := range []string{baseImg1, baseImg2, baseImg3, baseImg4, childImg1, childImg2, childImg3} {
			img, err := oc.AsAdmin().Client().Images().Get(imgName, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			imgs[imgName] = img
			o.Expect(img.DockerImageManifestMediaType).To(o.Equal(schema2.MediaTypeManifest))
		}

		// this shouldn't delete anything
		deleted, err := RunHardPrune(oc, dryRun)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(deleted.Len()).To(o.Equal(0))

		/* TODO: use a persistent storage for the registry to preserve data across re-deployments
		readOnly := true
		err = registryutil.ConfigureRegistry(oc, registryutil.RegistryConfiguration{ReadOnly: &readOnly})
		o.Expect(err).NotTo(o.HaveOccurred())
		*/

		/* imageName  | parent   | layers | imagestreams
		 * ---------- | -------- | ------ | ------------
		 *  baseImg1  |          | 1 2    | a a-tagged
		 *  baseImg2  |          | 4 5    | b
		 *  baseImg3  |          | 7 8    | c
		 *  baseImg4  |          | 11 12  | a
		 *  childImg1 | baseImg1 | 1 2 3  | c
		 *  childImg2 | baseImg2 | 4 5 6  | b
		 *  childImg3 | baseImg3 | 7 8 9  | c
		 */

		err = oc.AsAdmin().Client().ImageStreamTags(oc.Namespace()).Delete("a", "latest")
		o.Expect(err).NotTo(o.HaveOccurred())
		deleted, err = RunHardPrune(oc, dryRun)
		o.Expect(err).NotTo(o.HaveOccurred())
		expectedDeletions := &RegistryStorageFiles{
			/* TODO: reenable once we delete layer links as well
			LayerLinks: RepoLinks{oc.Namespace()+"/a": []string{
				imgs[baseImg1].DockerImageMetadata.ID,
				imgs[baseImg1].DockerImageLayers[0].Name,
				imgs[baseImg1].DockerImageLayers[1].Name,
			}},
			*/
			ManifestLinks: RepoLinks{oc.Namespace() + "/a": []string{baseImg1}},
		}
		err = AssertDeletedStorageFiles(deleted, expectedDeletions)
		o.Expect(err).NotTo(o.HaveOccurred())

		err = oc.AsAdmin().Client().Images().Delete(childImg1)
		o.Expect(err).NotTo(o.HaveOccurred())
		// The repository a-tagged will not be removed even though it has no tags anymore.
		// For the repository to be removed, the image stream itself needs to be deleted.
		err = oc.AsAdmin().Client().ImageStreamTags(oc.Namespace()).Delete("a-tagged", "latest")
		o.Expect(err).NotTo(o.HaveOccurred())
		deleted, err = RunHardPrune(oc, dryRun)
		o.Expect(err).NotTo(o.HaveOccurred())
		expectedDeletions = mergeOrSetExpectedDeletions(expectedDeletions,
			&RegistryStorageFiles{
				/* TODO: reenable once we delete layer links as well
				LayerLinks: RepoLinks{oc.Namespace()+"/c": []string{
					imgs[childImg1].DockerImageMetadata.ID,
					imgs[childImg1].DockerImageLayers[0].Name,
				}},
				*/
				ManifestLinks: RepoLinks{oc.Namespace() + "/c": []string{childImg1}},
				Blobs: []string{
					childImg1, // manifest blob
					imgs[childImg1].DockerImageMetadata.ID, // manifest config
					imgs[childImg1].DockerImageLayers[0].Name,
				},
			},
			dryRun)
		err = AssertDeletedStorageFiles(deleted, expectedDeletions)
		o.Expect(err).NotTo(o.HaveOccurred())

		err = oc.AsAdmin().Client().Images().Delete(baseImg1)
		o.Expect(err).NotTo(o.HaveOccurred())
		deleted, err = RunHardPrune(oc, dryRun)
		o.Expect(err).NotTo(o.HaveOccurred())
		expectedDeletions = mergeOrSetExpectedDeletions(expectedDeletions,
			&RegistryStorageFiles{
				Blobs: []string{
					baseImg1, // manifest blob
					imgs[baseImg1].DockerImageMetadata.ID, // manifest config
					imgs[baseImg1].DockerImageLayers[0].Name,
					imgs[baseImg1].DockerImageLayers[1].Name,
				},
			},
			dryRun)
		err = AssertDeletedStorageFiles(deleted, expectedDeletions)
		o.Expect(err).NotTo(o.HaveOccurred())

		err = oc.AsAdmin().Client().Images().Delete(childImg2)
		o.Expect(err).NotTo(o.HaveOccurred())
		deleted, err = RunHardPrune(oc, dryRun)
		o.Expect(err).NotTo(o.HaveOccurred())
		expectedDeletions = mergeOrSetExpectedDeletions(expectedDeletions,
			&RegistryStorageFiles{
				/* TODO: reenable once we delete layer links as well
				LayerLinks: RepoLinks{oc.Namespace()+"/b": []string{
					imgs[childImg2].DockerImageMetadata.ID,
					imgs[childImg2].DockerImageLayers[0].Name,
				}},
				*/
				ManifestLinks: RepoLinks{oc.Namespace() + "/b": []string{childImg2}},
				Blobs: []string{
					childImg2, // manifest blob
					imgs[childImg2].DockerImageMetadata.ID, // manifest config
					imgs[childImg2].DockerImageLayers[0].Name,
				},
			},
			dryRun)
		err = AssertDeletedStorageFiles(deleted, expectedDeletions)
		o.Expect(err).NotTo(o.HaveOccurred())

		// untag both baseImg2 and childImg2
		err = oc.AsAdmin().Client().ImageStreams(oc.Namespace()).Delete("b")
		o.Expect(err).NotTo(o.HaveOccurred())
		delete(expectedDeletions.ManifestLinks, oc.Namespace()+"/b")
		err = oc.AsAdmin().Client().Images().Delete(baseImg2)
		o.Expect(err).NotTo(o.HaveOccurred())
		deleted, err = RunHardPrune(oc, dryRun)
		o.Expect(err).NotTo(o.HaveOccurred())
		expectedDeletions = mergeOrSetExpectedDeletions(expectedDeletions,
			&RegistryStorageFiles{
				/* TODO: reenable once we delete layer links as well
				LayerLinks: RepoLinks{oc.Namespace()+"/b": []string{
					imgs[baseImg2].DockerImageMetadata.ID,
					imgs[baseImg2].DockerImageLayers[0].Name,
					imgs[baseImg2].DockerImageLayers[1].Name,
				}},
				*/
				Repos: []string{oc.Namespace() + "/b"},
				Blobs: []string{
					baseImg2, // manifest blob
					imgs[baseImg2].DockerImageMetadata.ID, // manifest config
					imgs[baseImg2].DockerImageLayers[0].Name,
					imgs[baseImg2].DockerImageLayers[1].Name,
				},
			},
			dryRun)
		err = AssertDeletedStorageFiles(deleted, expectedDeletions)
		o.Expect(err).NotTo(o.HaveOccurred())

		/* updated is/image table
		 * imageName  | parent   | layers | imagestreams
		 * ---------- | -------- | ------ | ------------
		 *  baseImg3  |          | 7 8    | c
		 *  baseImg4  |          | 11 12  | a
		 *  childImg3 | baseImg3 | 7 8 9  | c
		 */

		// delete baseImg3 using soft prune
		output, err := oc.WithoutNamespace().Run("adm").Args(
			"prune", "images", "--keep-tag-revisions=1", "--keep-younger-than=0").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring(baseImg3))
		o.Expect(output).To(o.ContainSubstring(imgs[baseImg3].DockerImageMetadata.ID))
		for _, layer := range imgs[baseImg3].DockerImageLayers {
			o.Expect(output).To(o.ContainSubstring(layer.Name))
		}
		o.Expect(output).NotTo(o.ContainSubstring(baseImg4))
		o.Expect(output).NotTo(o.ContainSubstring(childImg3))

		// there should be nothing left for hard pruner to delete
		deleted, err = RunHardPrune(oc, dryRun)
		o.Expect(err).NotTo(o.HaveOccurred())
		if !dryRun {
			expectedDeletions = &RegistryStorageFiles{}
		}
		err = AssertDeletedStorageFiles(deleted, expectedDeletions)
		o.Expect(err).NotTo(o.HaveOccurred())

		err = oc.AsAdmin().Client().Images().Delete(childImg3)
		o.Expect(err).NotTo(o.HaveOccurred())
		deleted, err = RunHardPrune(oc, dryRun)
		o.Expect(err).NotTo(o.HaveOccurred())
		expectedDeletions = mergeOrSetExpectedDeletions(expectedDeletions,
			&RegistryStorageFiles{
				/* TODO: reenable once we delete layer links as well
				LayerLinks: RepoLinks{oc.Namespace()+"/b": []string{
					imgs[baseImg2].DockerImageMetadata.ID,
					imgs[baseImg2].DockerImageLayers[0].Name,
					imgs[baseImg2].DockerImageLayers[1].Name,
				}},
				*/
				ManifestLinks: RepoLinks{oc.Namespace() + "/c": []string{childImg3}},
				Blobs: []string{
					childImg3,
					imgs[childImg3].DockerImageMetadata.ID, // manifest config
					imgs[childImg3].DockerImageLayers[0].Name,
				},
			},
			dryRun)
		err = AssertDeletedStorageFiles(deleted, expectedDeletions)
		o.Expect(err).NotTo(o.HaveOccurred())

		/* updated is/image table
		 * imageName  | parent   | layers | imagestreams
		 * ---------- | -------- | ------ | ------------
		 *  baseImg3  |          | 7 8    | c
		 *  baseImg4  |          | 11 12  | a
		 */

		assertImageBlobsPresent := func(present bool, img *imageapi.Image) {
			for _, layer := range img.DockerImageLayers {
				o.Expect(pathExistsInRegistry(oc, strings.Split(blobToPath("", layer.Name), "/")...)).
					To(o.Equal(present))
			}
			o.Expect(pathExistsInRegistry(oc, strings.Split(blobToPath("", img.DockerImageMetadata.ID), "/")...)).
				To(o.Equal(present))
			o.Expect(pathExistsInRegistry(oc, strings.Split(blobToPath("", img.Name), "/")...)).
				To(o.Equal(present))
		}

		for _, img := range []string{baseImg1, childImg1, baseImg2, childImg2} {
			assertImageBlobsPresent(dryRun, imgs[img])
		}
		for _, img := range []string{baseImg3, baseImg4} {
			assertImageBlobsPresent(true, imgs[img])
		}
	}

	g.It("should show orphaned blob deletions in dry-run mode", func() {
		testHardPrune(true)
	})

	g.It("should delete orphaned blobs", func() {
		testHardPrune(false)
	})

})
