package controller

import (
	"fmt"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/util"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/dockerregistry"
	"github.com/openshift/origin/pkg/image/api"
)

type ImportController struct {
	streams  client.ImageStreamsNamespacer
	mappings client.ImageStreamMappingsNamespacer
	// injected for testing
	client dockerregistry.Client
}

// needsImport returns true if the provided image stream should have its tags imported.
func needsImport(stream *api.ImageStream) bool {
	if len(stream.Spec.DockerImageRepository) == 0 {
		return false
	}
	if stream.Annotations != nil && len(stream.Annotations[api.DockerImageRepositoryCheckAnnotation]) != 0 {
		return false
	}
	return true
}

// retryCount is the number of times to retry on a conflict when updating an image stream
const retryCount = 2

// Next processes the given image stream, looking for streams that have DockerImageRepository
// set but have not yet been marked as "ready". If transient errors occur, err is returned but
// the image stream is not modified (so it will be tried again later). If a permanent
// failure occurs the image is marked with an annotation. The tags of the original spec image
// are left as is (those are updated through status).
func (c *ImportController) Next(stream *api.ImageStream) error {
	if !needsImport(stream) {
		return nil
	}
	name := stream.Spec.DockerImageRepository

	ref, err := api.ParseDockerImageReference(name)
	if err != nil {
		err = fmt.Errorf("invalid docker image repository, cannot import data: %v", err)
		util.HandleError(err)
		return c.done(stream, err.Error(), retryCount)
	}

	insecure := stream.Annotations != nil && stream.Annotations[api.InsecureRepositoryAnnotation] == "true"

	client := c.client
	if client == nil {
		client = dockerregistry.NewClient()
	}
	conn, err := client.Connect(ref.Registry, insecure)
	if err != nil {
		return err
	}
	tags, err := conn.ImageTags(ref.Namespace, ref.Name)
	switch {
	case dockerregistry.IsRepositoryNotFound(err), dockerregistry.IsRegistryNotFound(err):
		return c.done(stream, err.Error(), retryCount)
	case err != nil:
		return err
	}

	imageToTag := make(map[string][]string)
	for tag, image := range tags {
		if specTag, ok := stream.Spec.Tags[tag]; ok && specTag.From != nil {
			// spec tag is set to track another tag - do not import
			continue
		}

		imageToTag[image] = append(imageToTag[image], tag)
	}

	// no tags to import
	if len(imageToTag) == 0 {
		return c.done(stream, "", retryCount)
	}

	for id, tags := range imageToTag {
		dockerImage, err := conn.ImageByID(ref.Namespace, ref.Name, id)
		switch {
		case dockerregistry.IsRepositoryNotFound(err), dockerregistry.IsRegistryNotFound(err):
			return c.done(stream, err.Error(), retryCount)
		case dockerregistry.IsImageNotFound(err):
			continue
		case err != nil:
			return err
		}
		var image api.DockerImage
		if err := kapi.Scheme.Convert(dockerImage, &image); err != nil {
			err = fmt.Errorf("could not convert image: %#v", err)
			util.HandleError(err)
			return c.done(stream, err.Error(), retryCount)
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
					Name:      stream.Name,
					Namespace: stream.Namespace,
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
			if err := c.mappings.ImageStreamMappings(stream.Namespace).Create(mapping); err != nil {
				if errors.IsNotFound(err) {
					return c.done(stream, err.Error(), retryCount)
				}
				return err
			}
		}
	}

	// we've completed our updates
	return c.done(stream, "", retryCount)
}

// done marks the stream as being processed due to an error or failure condition
func (c *ImportController) done(stream *api.ImageStream, reason string, retry int) error {
	if len(reason) == 0 {
		reason = util.Now().UTC().Format(time.RFC3339)
	}
	if stream.Annotations == nil {
		stream.Annotations = make(map[string]string)
	}
	stream.Annotations[api.DockerImageRepositoryCheckAnnotation] = reason
	if _, err := c.streams.ImageStreams(stream.Namespace).Update(stream); err != nil && !errors.IsNotFound(err) {
		if errors.IsConflict(err) && retry > 0 {
			if stream, err = c.streams.ImageStreams(stream.Namespace).Get(stream.Name); err == nil {
				return c.done(stream, reason, retry-1)
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
