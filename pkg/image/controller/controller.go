package controller

import (
	"fmt"
	"time"

	"github.com/golang/glog"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/util"
	kerrors "k8s.io/kubernetes/pkg/util/errors"
	"k8s.io/kubernetes/pkg/util/sets"

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
	return stream.Annotations == nil || len(stream.Annotations[api.DockerImageRepositoryCheckAnnotation]) == 0
}

// retryCount is the number of times to retry on a conflict when updating an image stream
const retryCount = 2

// Next processes the given image stream, looking for streams that have DockerImageRepository
// set but have not yet been marked as "ready". If transient errors occur, err is returned but
// the image stream is not modified (so it will be tried again later). If a permanent
// failure occurs the image is marked with an annotation. The tags of the original spec image
// are left as is (those are updated through status).
// There are 3 use cases here:
// 1. spec.DockerImageRepository defined without any tags results in all tags being imported
//    from upstream image repository
// 2. spec.DockerImageRepository + tags defined - import all tags from upstream image repository,
//    and all the specified which (if name matches) will overwrite the default ones.
//    Additionally:
//    for kind == DockerImage import or reference underlying image, iow. exact tag (not provided means latest),
//    for kind != DockerImage reference tag from the same or other ImageStream
// 3. spec.DockerImageRepository not defined - import tags per its definition.
// Current behavior of the controller is to process import as far as possible, but
// we still want to keep backwards compatibility and retries, for that we'll return
// error in the following cases:
// 1. connection failure to upstream image repository
// 2. reading tags when error is different from RepositoryNotFound or RegistryNotFound
// 3. image retrieving when error is different from RepositoryNotFound, RegistryNotFound or ImageNotFound
// 4. ImageStreamMapping save error
// 5. error when marking ImageStream as imported

func (c *ImportController) Next(stream *api.ImageStream) error {
	if !needsImport(stream) {
		return nil
	}

	insecure := stream.Annotations[api.InsecureRepositoryAnnotation] == "true"
	client := c.client
	if client == nil {
		client = dockerregistry.NewClient()
	}

	toImport, err := getTags(stream, client, insecure)
	// return here, only if there is an error and nothing to import
	if err != nil && len(toImport) == 0 {
		return err
	}

	errs := c.importTags(stream, toImport, client, insecure)
	// one of retry-able error happened, we need to inform the RetryController
	// the import should be retried by returning error
	if len(errs) > 0 {
		return kerrors.NewAggregate(errs)
	}
	if err != nil {
		return err
	}

	return c.done(stream, "", retryCount)
}

// getTags returns tags from default upstream image repository and explicitly defined.
// Returns a map of tags to be imported and an error if one occurs.
// Tags explicitly defined will overwrite those from default upstream image repository.
func getTags(stream *api.ImageStream, client dockerregistry.Client, insecure bool) (map[string]api.DockerImageReference, error) {
	imports := make(map[string]api.DockerImageReference)
	references := sets.NewString()

	// read explicitly defined tags
	for tagName, specTag := range stream.Spec.Tags {
		if specTag.From == nil {
			continue
		}
		if specTag.From.Kind != "DockerImage" || specTag.Reference {
			references.Insert(tagName)
			continue
		}
		ref, err := api.ParseDockerImageReference(specTag.From.Name)
		if err != nil {
			glog.V(2).Infof("error parsing DockerImage %s: %v", specTag.From.Name, err)
			continue
		}
		imports[tagName] = ref.DockerClientDefaults()
	}

	if len(stream.Spec.DockerImageRepository) == 0 {
		return imports, nil
	}

	// read tags from default upstream image repository
	streamRef, err := api.ParseDockerImageReference(stream.Spec.DockerImageRepository)
	if err != nil {
		util.HandleError(fmt.Errorf("invalid docker image repository, cannot import data: %v", err))
		return imports, nil
	}
	conn, err := client.Connect(streamRef.Registry, insecure)
	if err != nil {
		// retry-able error no. 1
		return imports, err
	}
	tags, err := conn.ImageTags(streamRef.Namespace, streamRef.Name)
	switch {
	case dockerregistry.IsRepositoryNotFound(err), dockerregistry.IsRegistryNotFound(err):
		return imports, nil
	case err != nil:
		// retry-able error no. 2
		return imports, err
	}
	for tag, image := range tags {
		if _, ok := imports[tag]; ok || references.Has(tag) {
			continue
		}
		idTagPresent := false
		// this for loop is for backwards compatibility with v1 repo, where
		// there was no image id returned with tags, like v2 does right now.
		for t2, i2 := range tags {
			if i2 == image && t2 == image {
				idTagPresent = true
				break
			}
		}
		ref := streamRef
		if idTagPresent {
			ref.Tag = image
		} else {
			ref.Tag = tag
		}
		ref.ID = image
		imports[tag] = ref
	}

	return imports, nil
}

// importTags imports tags specified in a map from given ImageStream. Returns an error if one occurs.
func (c *ImportController) importTags(stream *api.ImageStream, imports map[string]api.DockerImageReference, client dockerregistry.Client, insecure bool) []error {
	retrieved := make(map[string]*dockerregistry.Image)
	var errlist []error
	for tag, ref := range imports {
		image, err := c.importTag(stream, tag, ref, retrieved[ref.ID], client, insecure)
		if err != nil {
			util.HandleError(err)
			errlist = append(errlist, err)
			continue
		}
		// save image object for next tag imports, this is to avoid re-downloading the default image registry
		if len(ref.ID) > 0 {
			retrieved[ref.ID] = image
		}
	}
	return errlist
}

// importTag import single tag from given ImageStream. Returns an error if one occurs.
func (c *ImportController) importTag(stream *api.ImageStream, tag string, ref api.DockerImageReference, dockerImage *dockerregistry.Image, client dockerregistry.Client, insecure bool) (*dockerregistry.Image, error) {
	if dockerImage == nil {
		// TODO insecure applies to the stream's spec.dockerImageRepository, not necessarily to an external one!
		conn, err := client.Connect(ref.Registry, insecure)
		if err != nil {
			return nil, err
		}
		if len(ref.ID) > 0 {
			dockerImage, err = conn.ImageByID(ref.Namespace, ref.Name, ref.ID)
		} else {
			dockerImage, err = conn.ImageByTag(ref.Namespace, ref.Name, ref.Tag)
		}
		switch {
		case dockerregistry.IsRepositoryNotFound(err), dockerregistry.IsRegistryNotFound(err), dockerregistry.IsImageNotFound(err):
			return nil, nil
		case err != nil:
			// retry-able error no. 3
			return nil, err
		}
	}
	var image api.DockerImage
	if err := kapi.Scheme.Convert(&dockerImage.Image, &image); err != nil {
		return nil, fmt.Errorf("could not convert image: %#v", err)
	}

	// prefer to pull by ID always
	if dockerImage.PullByID {
		// if the registry indicates the image is pullable by ID, clear the tag
		ref.Tag = ""
		ref.ID = dockerImage.ID
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
			DockerImageReference: ref.String(),
			DockerImageMetadata:  image,
		},
	}
	if err := c.mappings.ImageStreamMappings(stream.Namespace).Create(mapping); err != nil {
		// retry-able no. 4
		return nil, err
	}
	return dockerImage, nil
}

// done marks the stream as being processed due to an error or failure condition
func (c *ImportController) done(stream *api.ImageStream, reason string, retry int) error {
	if len(reason) == 0 {
		reason = unversioned.Now().UTC().Format(time.RFC3339)
	} else if len(reason) > 300 {
		// cut down the reason up to 300 characters max.
		reason = reason[:300]
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
