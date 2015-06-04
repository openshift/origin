package controller

import (
	"fmt"
	"time"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/dockerregistry"
	"github.com/openshift/origin/pkg/image/api"
)

type ImportController struct {
	repositories client.ImageStreamsNamespacer
	mappings     client.ImageStreamMappingsNamespacer
	client       dockerregistry.Client
}

// needsImport returns true if the provided repository should have its tags imported.
func needsImport(repo *api.ImageStream) bool {
	if len(repo.Spec.DockerImageRepository) == 0 {
		return false
	}
	if repo.Annotations != nil && len(repo.Annotations[api.DockerImageRepositoryCheckAnnotation]) != 0 {
		return false
	}
	return true
	/*
		if len(repo.Spec.Tags) == 0 {
			return true
		}
		emptyTags := 0
		for _, v := range repo.Spec.Tags {
			if len(v.DockerImageReference) == 0 {
				emptyTags++
			}
		}
		return emptyTags > 0
	*/
}

// retryCount is the number of times to retry on a conflict when updating an image stream
const retryCount = 2

// Next processes the given image repository, looking for repos that have DockerImageRepository
// set but have not yet been marked as "ready". If transient errors occur, err is returned but
// the image repository is not modified (so it will be tried again later). If a permanent
// failure occurs the image is marked with an annotation. The tags of the original spec image
// are left as is (those are updated through status).
func (c *ImportController) Next(repo *api.ImageStream) error {
	if !needsImport(repo) {
		return nil
	}
	name := repo.Spec.DockerImageRepository

	ref, err := api.ParseDockerImageReference(name)
	if err != nil {
		err = fmt.Errorf("invalid docker image repository, cannot import data: %v", err)
		util.HandleError(err)
		return c.done(repo, err.Error(), retryCount)
	}

	insecure := repo.Annotations != nil && repo.Annotations[api.InsecureRepositoryAnnotation] == "true"

	conn, err := c.client.Connect(ref.Registry, insecure)
	if err != nil {
		return err
	}
	tags, err := conn.ImageTags(ref.Namespace, ref.Name)
	switch {
	case dockerregistry.IsRepositoryNotFound(err), dockerregistry.IsRegistryNotFound(err):
		return c.done(repo, err.Error(), retryCount)
	case err != nil:
		return err
	}

	newTags := make(map[string]string) //, len(repo.Spec.Tags))
	imageToTag := make(map[string][]string)
	//switch {
	//case len(repo.Tags) == 0:
	// copy all tags
	for tag := range tags {
		// TODO: switch to image when pull by ID is automatic
		newTags[tag] = tag
	}
	for tag, image := range tags {
		imageToTag[image] = append(imageToTag[image], tag)
	}
	/*
		default:
			for tag, v := range repo.Tags {
				if len(v) != 0 {
					newTags[tag] = v
					continue
				}
				image, ok := tags[tag]
				if !ok {
					// tag not found, set empty
					continue
				}
				imageToTag[image] = append(imageToTag[image], tag)
				// TODO: switch to image when pull by ID is automatic
				newTags[tag] = tag
			}
		}
	*/

	// nothing to tag - no images in the upstream repo, or we're in sync
	if len(imageToTag) == 0 {
		return c.done(repo, "", retryCount)
	}

	for id, tags := range imageToTag {
		dockerImage, err := conn.ImageByID(ref.Namespace, ref.Name, id)
		switch {
		case dockerregistry.IsRepositoryNotFound(err), dockerregistry.IsRegistryNotFound(err):
			return c.done(repo, err.Error(), retryCount)
		case dockerregistry.IsImageNotFound(err):
			for _, tag := range tags {
				delete(newTags, tag)
			}
			continue
		case err != nil:
			return err
		}
		var image api.DockerImage
		if err := kapi.Scheme.Convert(dockerImage, &image); err != nil {
			err = fmt.Errorf("could not convert image: %#v", err)
			util.HandleError(err)
			return c.done(repo, err.Error(), retryCount)
		}

		idTagPresent := false
		if len(tags) > 1 && hasTag(tags, id) {
			// only set to true if we have at least 1 tag that isn't the image id
			idTagPresent = true
		}
		for _, tag := range tags {
			if idTagPresent && id == tag {
				continue
			}
			pullRefTag := tag
			if idTagPresent {
				// if there is a tag for the image by its id (tag=tag), we can pull by id
				pullRefTag = id
			}
			pullRef := api.DockerImageReference{
				Registry:  ref.Registry,
				Namespace: ref.Namespace,
				Name:      ref.Name,
				Tag:       pullRefTag,
			}

			mapping := &api.ImageStreamMapping{
				ObjectMeta: kapi.ObjectMeta{
					Name:      repo.Name,
					Namespace: repo.Namespace,
				},
				Tag: tag,
				Image: api.Image{
					ObjectMeta: kapi.ObjectMeta{
						Name: id,
					},
					DockerImageReference: pullRef.String(),
					DockerImageMetadata:  image,
				},
			}
			if err := c.mappings.ImageStreamMappings(repo.Namespace).Create(mapping); err != nil {
				if errors.IsNotFound(err) {
					return c.done(repo, err.Error(), retryCount)
				}
				return err
			}
		}
	}

	// we've completed our updates
	return c.done(repo, "", retryCount)
}

// done marks the repository as being processed due to an error or failure condition
func (c *ImportController) done(repo *api.ImageStream, reason string, retry int) error {
	if len(reason) == 0 {
		reason = util.Now().UTC().Format(time.RFC3339)
	}
	if repo.Annotations == nil {
		repo.Annotations = make(map[string]string)
	}
	repo.Annotations[api.DockerImageRepositoryCheckAnnotation] = reason
	if _, err := c.repositories.ImageStreams(repo.Namespace).Update(repo); err != nil && !errors.IsNotFound(err) {
		if errors.IsConflict(err) && retry > 0 {
			if repo, err := c.repositories.ImageStreams(repo.Namespace).Get(repo.Name); err == nil {
				return c.done(repo, reason, retry-1)
			}
		}
		return err
	}
	return nil
}

func hasTag(tags []string, tag string) bool {
	for _, s := range tags {
		if s == tag {
			return true
		}
	}
	return false
}
