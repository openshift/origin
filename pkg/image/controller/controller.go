package controller

import (
	"fmt"
	"strings"
	"time"

	"github.com/golang/glog"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/util"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/dockerregistry"
	"github.com/openshift/origin/pkg/image/api"
	imageutil "github.com/openshift/origin/pkg/image/util"
)

var (
	availableSelector       fields.Selector
	availableStreamSelector fields.Selector
)

func init() {
	var err error
	availableSelector, err = fields.ParseSelector("image.status.phase!=" + api.ImagePurging)
	if err != nil {
		glog.Fatal(err.Error())
	}
	availableStreamSelector, err = fields.ParseSelector("status.phase==" + api.ImageStreamAvailable)
	if err != nil {
		glog.Fatal(err.Error())
	}
}

type ImageStreamController struct {
	streams         client.ImageStreamsNamespacer
	streamDeletions client.ImageStreamDeletionsInterfacer
	streamImages    client.ImageStreamImagesNamespacer
	images          client.ImagesInterfacer
	mappings        client.ImageStreamMappingsNamespacer
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
func (c *ImageStreamController) Next(stream *api.ImageStream) error {

	if stream.Status.Phase == api.ImageStreamTerminating {
		return c.terminateImageStream(stream)
	}

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
		if err := kapi.Scheme.Convert(&dockerImage.Image, &image); err != nil {
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

			pullRef := api.DockerImageReference{
				Registry:  ref.Registry,
				Namespace: ref.Namespace,
				Name:      ref.Name,
				Tag:       tag,
			}
			// prefer to pull by ID always
			if dockerImage.PullByID {
				// if the registry indicates the image is pullable by ID, clear the tag
				pullRef.Tag = ""
				pullRef.ID = dockerImage.ID
			} else if idTagPresent {
				// if there is a tag for the image by its id (tag=tag), we can pull by id
				pullRef.Tag = id
			}

			mapping := &api.ImageStreamMapping{
				ObjectMeta: kapi.ObjectMeta{
					Name:      stream.Name,
					Namespace: stream.Namespace,
				},
				Tag: tag,
				Image: api.Image{
					ObjectMeta: kapi.ObjectMeta{
						Name: dockerImage.ID,
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
func (c *ImageStreamController) done(stream *api.ImageStream, reason string, retry int) error {
	if len(reason) == 0 {
		reason = util.Now().UTC().Format(time.RFC3339)
	}
	if stream.Annotations == nil {
		stream.Annotations = make(map[string]string)
	}
	stream.Annotations[api.DockerImageRepositoryCheckAnnotation] = reason
	if _, err := c.streams.ImageStreams(stream.Namespace).Update(stream); err != nil && !errors.IsNotFound(err) {
		if errors.IsConflict(err) && retry > 0 {
			if stream, err := c.streams.ImageStreams(stream.Namespace).Get(stream.Name); err == nil {
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

// terminateImageStream handles terminating image stream. It does following:
//
//    1. marks for deletion all its images that aren't referred by any other
//       available image streams
//    2. creates an instance of ImageStreamDeletion
//    3. removes origin finalizer from its finalizers
//    4. deletes it if there are no other finalizers left
func (c *ImageStreamController) terminateImageStream(stream *api.ImageStream) (err error) {
	glog.V(4).Infof("Handling termination of image stream %s/%s (%s)", stream.Namespace, stream.Name, stream.Status.Phase)
	// if stream is not terminating, ignore it
	if stream.Status.Phase != api.ImageStreamTerminating {
		return nil
	}

	if !imageutil.ImageStreamFinalized(stream) {
		// finalize image stream
		if err := c.markOrphanedImagesForDeletion(stream); err != nil {
			return err
		}

		glog.V(4).Infof("Creating image stream deletion for stream %s/%s", stream.Namespace, stream.Name)
		isd, err := api.NewDeletionForImageStream(stream)
		if err != nil {
			return err
		}
		_, err = c.streamDeletions.ImageStreamDeletions().Create(isd)
		if err != nil && !errors.IsAlreadyExists(err) {
			return err
		}

		glog.V(4).Infof("Finalizing image stream %s/%s", stream.Namespace, stream.Name)
		stream, err = imageutil.FinalizeImageStream(c.streams, stream)
		if err != nil {
			return err
		}
	}

	if len(stream.Spec.Finalizers) == 0 {
		glog.V(4).Infof("Deleting image stream %s/%s", stream.Namespace, stream.Name)
		return c.streams.ImageStreams(stream.Namespace).Delete(stream.Name)
	}

	return nil
}

// markOrphanedImagesForDeletion marks for deletion each image if given image
// stream if it meets following conditions
//
//   1. it's an internally managed image (lives in internal registry)
//   2. it's available (not marked for deletion already)
//   3. it's referenced only by terminating image streams
func (c *ImageStreamController) markOrphanedImagesForDeletion(stream *api.ImageStream) (err error) {
	glog.V(4).Infof("Marking images of image stream %s/%s for deletion", stream.Namespace, stream.Name)

	candidates := make(map[string]*api.Image)
	images, err := c.streamImages.ImageStreamImages(stream.Namespace).List(labels.Everything(), availableSelector)
	if err != nil {
		return fmt.Errorf("Failed to list image stream images: %v", err)
	}
	for _, isi := range images.Items {
		nameParts := strings.Split(isi.Name, "@")
		imageStreamName := nameParts[0]

		if isi.Image.Annotations == nil || isi.Image.Annotations[api.ManagedByOpenShiftAnnotation] != "true" {
			// skip images not managed internally
			continue
		}
		if isi.Image.Status.Phase == api.ImagePurging {
			// shouldn't happen
			glog.V(4).Infof("Received image %q belonging to stream %s/%s which is already being purged", isi.Image.Name, stream.Namespace, imageStreamName)
			continue
		}

		if imageStreamName == stream.Name {
			candidates[isi.Image.Name] = &isi.Image
		}
	}

	if len(candidates) == 0 {
		return nil
	}

	// remove all candidates referred by available image streams
	availableStreams, err := c.streams.ImageStreams("").List(labels.Everything(), availableStreamSelector)
	if err != nil {
		return fmt.Errorf("Failed to list available image streams: %v", err)
	}

	for _, stream := range availableStreams.Items {
		if stream.Status.Phase == api.ImageStreamTerminating {
			// shouldn't happen
			glog.V(4).Infof("Skipping available image stream %s/%s", stream.Namespace, stream.Name)
			continue
		}
		images, err := c.streamImages.ImageStreamImages(stream.Namespace).List(labels.Everything(), availableSelector)
		if err != nil {
			return fmt.Errorf("Failed to list images of image stream %s/%s: %v", stream.Namespace, stream.Name, err)
		}

		for _, isi := range images.Items {
			delete(candidates, isi.Image.Name)
		}
	}

	for _, img := range candidates {
		glog.V(4).Infof("Marking image %q for deletion", img.Name)
		if err := c.images.Images().Delete(img.Name); err != nil {
			return fmt.Errorf("Failed to delete image %s: %v", img.Name, err)
		}
	}

	return nil
}
