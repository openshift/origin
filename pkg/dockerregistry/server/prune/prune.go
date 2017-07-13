package prune

import (
	"fmt"

	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/manifest/schema2"
	"github.com/docker/distribution/reference"
	"github.com/docker/distribution/registry/storage"
	"github.com/docker/distribution/registry/storage/driver"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openshift/origin/pkg/dockerregistry/server"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
)

func imageStreamHasManifestDigest(is *imageapi.ImageStream, dgst digest.Digest) bool {
	for _, tagEventList := range is.Status.Tags {
		for _, tagEvent := range tagEventList.Items {
			if tagEvent.Image == string(dgst) {
				return true
			}
		}
	}
	return false
}

// Summary is cumulative information about what was pruned.
type Summary struct {
	Blobs     int
	DiskSpace int64
}

// Prune removes blobs which are not used by Images in OpenShift.
//
// On error, the Summary will contain what was deleted so far.
//
// TODO(dmage): remove layer links to a blob if the blob is removed or it doesn't belong to the ImageStream.
// TODO(dmage): keep young blobs (docker/distribution#2297).
func Prune(ctx context.Context, storageDriver driver.StorageDriver, registry distribution.Namespace, registryClient server.RegistryClient, dryRun bool) (Summary, error) {
	logger := context.GetLogger(ctx)

	repositoryEnumerator, ok := registry.(distribution.RepositoryEnumerator)
	if !ok {
		return Summary{}, fmt.Errorf("unable to convert Namespace to RepositoryEnumerator")
	}

	oc, _, err := registryClient.Clients()
	if err != nil {
		return Summary{}, fmt.Errorf("error getting clients: %v", err)
	}

	imageList, err := oc.Images().List(metav1.ListOptions{})
	if err != nil {
		return Summary{}, fmt.Errorf("error listing images: %v", err)
	}

	inuse := make(map[string]string)
	for _, image := range imageList.Items {
		// Keep the manifest.
		inuse[image.Name] = image.DockerImageReference

		// Keep the config for a schema 2 manifest.
		if image.DockerImageManifestMediaType == schema2.MediaTypeManifest {
			inuse[image.DockerImageMetadata.ID] = image.DockerImageReference
		}

		// Keep image layers.
		for _, layer := range image.DockerImageLayers {
			inuse[layer.Name] = image.DockerImageReference
		}
	}

	var stats Summary

	var reposToDelete []string
	err = repositoryEnumerator.Enumerate(ctx, func(repoName string) error {
		logger.Debugln("Processing repository", repoName)

		named, err := reference.WithName(repoName)
		if err != nil {
			return fmt.Errorf("failed to parse the repo name %s: %v", repoName, err)
		}

		ref, err := imageapi.ParseDockerImageReference(repoName)
		if err != nil {
			return fmt.Errorf("failed to parse the image reference %s: %v", repoName, err)
		}

		is, err := oc.ImageStreams(ref.Namespace).Get(ref.Name, metav1.GetOptions{})
		if kerrors.IsNotFound(err) {
			logger.Printf("The image stream %s/%s is not found, will remove the whole repository", ref.Namespace, ref.Name)

			// We cannot delete the repository at this point, because it would break Enumerate.
			reposToDelete = append(reposToDelete, repoName)

			return nil
		} else if err != nil {
			return fmt.Errorf("failed to get the image stream %s: %v", repoName, err)
		}

		repository, err := registry.Repository(ctx, named)
		if err != nil {
			return err
		}

		manifestService, err := repository.Manifests(ctx)
		if err != nil {
			return err
		}

		manifestEnumerator, ok := manifestService.(distribution.ManifestEnumerator)
		if !ok {
			return fmt.Errorf("unable to convert ManifestService into ManifestEnumerator")
		}

		err = manifestEnumerator.Enumerate(ctx, func(dgst digest.Digest) error {
			if _, ok := inuse[string(dgst)]; ok && imageStreamHasManifestDigest(is, dgst) {
				logger.Debugf("Keeping the manifest link %s@%s", repoName, dgst)
				return nil
			}

			if dryRun {
				logger.Printf("Would delete manifest link: %s@%s", repoName, dgst)
				return nil
			}

			logger.Printf("Deleting manifest link: %s@%s", repoName, dgst)
			if err := manifestService.Delete(ctx, dgst); err != nil {
				return fmt.Errorf("failed to delete the manifest link %s@%s: %v", repoName, dgst, err)
			}

			return nil
		})
		if e, ok := err.(driver.PathNotFoundError); ok {
			logger.Printf("Skipped manifest link pruning for the repository %s: %v", repoName, e)
		} else if err != nil {
			return fmt.Errorf("failed to prune manifest links in the repository %s: %v", repoName, err)
		}

		return nil
	})
	if e, ok := err.(driver.PathNotFoundError); ok {
		logger.Warnf("No repositories found: %v", e)
		return stats, nil
	} else if err != nil {
		return stats, err
	}

	vacuum := storage.NewVacuum(ctx, storageDriver)

	logger.Debugln("Removing repositories")
	for _, repoName := range reposToDelete {
		if dryRun {
			logger.Printf("Would delete repository: %s", repoName)
			continue
		}

		if err = vacuum.RemoveRepository(repoName); err != nil {
			return stats, fmt.Errorf("unable to remove the repository %s: %v", repoName, err)
		}
	}

	logger.Debugln("Processing blobs")
	blobStatter := registry.BlobStatter()
	err = registry.Blobs().Enumerate(ctx, func(dgst digest.Digest) error {
		if imageReference, ok := inuse[string(dgst)]; ok {
			logger.Debugf("Keeping the blob %s (it belongs to the image %s)", dgst, imageReference)
			return nil
		}

		desc, err := blobStatter.Stat(ctx, dgst)
		if err != nil {
			return err
		}

		stats.Blobs++
		stats.DiskSpace += desc.Size

		if dryRun {
			logger.Printf("Would delete blob: %s", dgst)
			return nil
		}

		if err := vacuum.RemoveBlob(string(dgst)); err != nil {
			return fmt.Errorf("failed to delete the blob %s: %v", dgst, err)
		}

		return nil
	})
	if e, ok := err.(driver.PathNotFoundError); ok {
		logger.Warnf("No repositories found: %v", e)
		return stats, nil
	}
	return stats, err
}
