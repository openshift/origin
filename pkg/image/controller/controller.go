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

const dockerImageRepositoryCheckAnnotation = "openshift.io/image.dockerRepositoryCheck"

type ImportController struct {
	repositories client.ImageRepositoriesNamespacer
	mappings     client.ImageRepositoryMappingsNamespacer
	client       dockerregistry.Client
}

// Next processes the given image repository, looking for repos that have DockerImageRepository
// set but have not yet been marked as "ready". If transient errors occur, err is returned but
// the image repository is not modified (so it will be tried again later). If a permanent
// failure occurs the image is marked with an annotation.
func (c *ImportController) Next(repo *api.ImageRepository) error {
	name := repo.DockerImageRepository
	if len(name) == 0 {
		return nil
	}
	if repo.Annotations == nil {
		repo.Annotations = make(map[string]string)
	}
	if len(repo.Annotations[dockerImageRepositoryCheckAnnotation]) != 0 {
		return nil
	}

	registry, namespace, name, _, err := api.SplitDockerPullSpec(name)
	if err != nil {
		err = fmt.Errorf("invalid docker image repository, cannot import data: %v", err)
		util.HandleError(err)
		return c.done(repo, err.Error())
	}

	conn, err := c.client.Connect(registry)
	if err != nil {
		return err
	}
	tags, err := conn.ImageTags(namespace, name)
	switch {
	case dockerregistry.IsRepositoryNotFound(err), dockerregistry.IsRegistryNotFound(err):
		return c.done(repo, err.Error())
	case err != nil:
		return err
	}

	newTags := make(map[string]string, len(repo.Tags))
	imageToTag := make(map[string][]string)
	switch {
	case len(repo.Tags) == 0:
		// copy all tags
		for tag, _ := range tags {
			// TODO: once pull by image is implemented, use tag = imageid
			newTags[tag] = tag
		}
		for tag, image := range tags {
			imageToTag[image] = append(imageToTag[image], tag)
		}
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
			// TODO: once pull by image is implemented, use tag = imageid
			newTags[tag] = tag
		}
	}

	// whether we ignore or succeed, ensure the most recent mappings are recorded
	repo.Tags = newTags

	// nothing to tag - no images in the upstream repo, or we're in sync
	if len(imageToTag) == 0 {
		return c.done(repo, "")
	}

	for id, tags := range imageToTag {
		dockerImage, err := conn.ImageByID(namespace, name, id)
		switch {
		case dockerregistry.IsRepositoryNotFound(err), dockerregistry.IsRegistryNotFound(err):
			return c.done(repo, err.Error())
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
			return c.done(repo, err.Error())
		}
		mapping := &api.ImageRepositoryMapping{
			ObjectMeta: kapi.ObjectMeta{
				Name:      repo.Name,
				Namespace: repo.Namespace,
			},
			Tag: tags[0],
			Image: api.Image{
				DockerImageMetadata: image,
			},
		}
		if err := c.mappings.ImageRepositoryMappings(repo.Namespace).Create(mapping); err != nil {
			if errors.IsNotFound(err) {
				return c.done(repo, err.Error())
			}
			return err
		}
	}

	// we've completed our updates
	return c.done(repo, "")
}

// ignore marks the repository as being processed due to an error or failure condition
func (c *ImportController) done(repo *api.ImageRepository, reason string) error {
	if len(reason) == 0 {
		reason = util.Now().UTC().Format(time.RFC3339)
	}
	repo.Annotations[dockerImageRepositoryCheckAnnotation] = reason
	if _, err := c.repositories.ImageRepositories(repo.Namespace).Update(repo); err != nil && !errors.IsNotFound(err) {
		return err
	}
	return nil
}
