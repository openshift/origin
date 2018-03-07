package images

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	"github.com/docker/distribution/manifest/schema2"

	kapiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	kcoreclient "k8s.io/client-go/kubernetes/typed/core/v1"

	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	exutil "github.com/openshift/origin/test/extended/util"
	testutil "github.com/openshift/origin/test/util"
)

var _ = g.Describe("[Feature:ImagePrune][registry][Serial] Image hard prune", func() {
	defer g.GinkgoRecover()
	var oc = exutil.NewCLI("prune-images", exutil.KubeConfigPath())
	var originalAcceptSchema2 *bool

	g.JustBeforeEach(func() {
		if originalAcceptSchema2 == nil {
			accepts, err := DoesRegistryAcceptSchema2(oc)
			o.Expect(err).NotTo(o.HaveOccurred())
			originalAcceptSchema2 = &accepts
		}

		readOnly := false
		acceptSchema2 := true
		_, err := ConfigureRegistry(oc,
			RegistryConfiguration{
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
		_, err := ConfigureRegistry(oc,
			RegistryConfiguration{
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
		registryURL, err := GetDockerRegistryURL(oc)
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
			img, err := oc.AsAdmin().ImageClient().Image().Images().Get(imgName, metav1.GetOptions{})
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
		restarted, err = registryutil.ConfigureRegistry(oc, registryutil.RegistryConfiguration{ReadOnly: &readOnly})
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

		err = oc.AsAdmin().ImageClient().Image().ImageStreamTags(oc.Namespace()).Delete("a:latest", nil)
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

		err = oc.AsAdmin().ImageClient().Image().Images().Delete(childImg1, nil)
		o.Expect(err).NotTo(o.HaveOccurred())
		// The repository a-tagged will not be removed even though it has no tags anymore.
		// For the repository to be removed, the image stream itself needs to be deleted.
		err = oc.AsAdmin().ImageClient().Image().ImageStreamTags(oc.Namespace()).Delete("a-tagged:latest", nil)
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

		err = oc.AsAdmin().ImageClient().Image().Images().Delete(baseImg1, nil)
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

		err = oc.AsAdmin().ImageClient().Image().Images().Delete(childImg2, nil)
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
		err = oc.AsAdmin().ImageClient().Image().ImageStreams(oc.Namespace()).Delete("b", nil)
		o.Expect(err).NotTo(o.HaveOccurred())
		delete(expectedDeletions.ManifestLinks, oc.Namespace()+"/b")
		err = oc.AsAdmin().ImageClient().Image().Images().Delete(baseImg2, nil)
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

		err = oc.AsAdmin().ImageClient().Image().Images().Delete(childImg3, nil)
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

const (
	AcceptSchema2EnvVar  = "REGISTRY_MIDDLEWARE_REPOSITORY_OPENSHIFT_ACCEPTSCHEMA2"
	readOnlyEnvVar       = "REGISTRY_STORAGE_MAINTENANCE_READONLY"
	defaultAcceptSchema2 = true
)

// GetDockerRegistryURL returns a cluster URL of internal docker registry if available.
func GetDockerRegistryURL(oc *exutil.CLI) (string, error) {
	svc, err := oc.AdminKubeClient().Core().Services("default").Get("docker-registry", metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	url := svc.Spec.ClusterIP
	for _, p := range svc.Spec.Ports {
		url = fmt.Sprintf("%s:%d", url, p.Port)
		break
	}
	return url, nil
}

// RegistriConfiguration holds desired configuration options for the integrated registry. *nil* stands for
// "no change".
type RegistryConfiguration struct {
	ReadOnly      *bool
	AcceptSchema2 *bool
}

type byAgeDesc []kapiv1.Pod

func (ba byAgeDesc) Len() int      { return len(ba) }
func (ba byAgeDesc) Swap(i, j int) { ba[i], ba[j] = ba[j], ba[i] }
func (ba byAgeDesc) Less(i, j int) bool {
	return ba[j].CreationTimestamp.Before(&ba[i].CreationTimestamp)
}

// GetRegistryPod returns the youngest registry pod deployed.
func GetRegistryPod(podsGetter kcoreclient.PodsGetter) (*kapiv1.Pod, error) {
	podList, err := podsGetter.Pods(metav1.NamespaceDefault).List(metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(labels.Set{"deploymentconfig": "docker-registry"}).String(),
	})
	if err != nil {
		return nil, err
	}
	if len(podList.Items) == 0 {
		return nil, fmt.Errorf("failed to find any docker-registry pod")
	}

	sort.Sort(byAgeDesc(podList.Items))

	return &podList.Items[0], nil
}

// LogRegistryPod attempts to write registry log to a file in artifacts directory.
func LogRegistryPod(oc *exutil.CLI) error {
	pod, err := GetRegistryPod(oc.KubeClient().Core())
	if err != nil {
		return fmt.Errorf("failed to get registry pod: %v", err)
	}

	ocLocal := *oc
	ocLocal.SetOutputDir(exutil.ArtifactDirPath())
	path, err := ocLocal.Run("logs").Args("dc/docker-registry").OutputToFile("pod-" + pod.Name + ".log")
	if err == nil {
		fmt.Fprintf(g.GinkgoWriter, "written registry pod log to %s\n", path)
	}
	return err
}

// ConfigureRegistry re-deploys the registry pod if its configuration doesn't match the desiredState. The
// function blocks until the registry is ready.
func ConfigureRegistry(oc *exutil.CLI, desiredState RegistryConfiguration) (bool, error) {
	defer func(ns string) { oc.SetNamespace(ns) }(oc.Namespace())
	oc = oc.SetNamespace(metav1.NamespaceDefault).AsAdmin()
	env, err := oc.Run("env").Args("dc/docker-registry", "--list").Output()
	if err != nil {
		return false, err
	}

	envOverrides := []string{}

	if desiredState.AcceptSchema2 != nil {
		current := !strings.Contains(env, fmt.Sprintf("%s=%t", AcceptSchema2EnvVar, false))
		if current != *desiredState.AcceptSchema2 {
			new := fmt.Sprintf("%s=%t", AcceptSchema2EnvVar, *desiredState.AcceptSchema2)
			envOverrides = append(envOverrides, new)
		}
	}
	if desiredState.ReadOnly != nil {
		value := fmt.Sprintf("%s=%s", readOnlyEnvVar, makeReadonlyEnvValue(*desiredState.ReadOnly))
		if !strings.Contains(env, value) {
			envOverrides = append(envOverrides, value)
		}
	}
	if len(envOverrides) == 0 {
		g.By("docker-registry is already in the desired state of configuration")
		return false, nil
	}

	if err := oc.Run("env").Args(append([]string{"dc/docker-registry"}, envOverrides...)...).Execute(); err != nil {
		return false, fmt.Errorf("failed to update registry's environment: %v", err)
	}

	if err := oc.Run("rollout").Args("status", "dc/docker-registry").Execute(); err != nil {
		return false, fmt.Errorf("unable to get registry rollout status: %v", err)
	}

	return true, nil
}

func RedeployRegistry(oc *exutil.CLI) (bool, error) {
	defer func(ns string) { oc.SetNamespace(ns) }(oc.Namespace())

	oc = oc.SetNamespace(metav1.NamespaceDefault).AsAdmin()

	if err := oc.Run("rollout").Args("latest", "dc/docker-registry").Execute(); err != nil {
		return false, fmt.Errorf("failed to rollout registry: %v", err)
	}

	if err := oc.Run("rollout").Args("status", "dc/docker-registry").Execute(); err != nil {
		return false, fmt.Errorf("unable to get registry rollout status: %v", err)
	}

	return true, nil
}

// EnsureRegistryAcceptsSchema2 checks whether the registry is configured to accept manifests V2 schema 2 or
// not. If the result doesn't match given accept argument, registry's deployment config will be updated
// accordingly and the function will block until the registry have been re-deployed and ready for new
// requests.
func EnsureRegistryAcceptsSchema2(oc *exutil.CLI, accept bool) (bool, error) {
	return ConfigureRegistry(oc, RegistryConfiguration{AcceptSchema2: &accept})
}

func makeReadonlyEnvValue(on bool) string {
	return fmt.Sprintf(`{"enabled":%t}`, on)
}

// GetRegistryStorageSize returns a number of bytes occupied by registry's data on its filesystem.
func GetRegistryStorageSize(oc *exutil.CLI) (int64, error) {
	defer func(ns string) { oc.SetNamespace(ns) }(oc.Namespace())
	out, err := oc.SetNamespace(metav1.NamespaceDefault).AsAdmin().Run("rsh").Args(
		"dc/docker-registry", "du", "--bytes", "--summarize", "/registry/docker/registry").Output()
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

// DoesRegistryAcceptSchema2 returns true if the integrated registry is configured to accept manifest V2
// schema 2.
func DoesRegistryAcceptSchema2(oc *exutil.CLI) (bool, error) {
	defer func(ns string) { oc.SetNamespace(ns) }(oc.Namespace())
	env, err := oc.SetNamespace(metav1.NamespaceDefault).AsAdmin().Run("env").Args("dc/docker-registry", "--list").Output()
	if err != nil {
		return defaultAcceptSchema2, err
	}

	if strings.Contains(env, fmt.Sprintf("%s=", AcceptSchema2EnvVar)) {
		return strings.Contains(env, fmt.Sprintf("%s=true", AcceptSchema2EnvVar)), nil
	}

	return defaultAcceptSchema2, nil
}
