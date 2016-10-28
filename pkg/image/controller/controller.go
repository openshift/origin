package controller

import (
	"errors"

	"github.com/golang/glog"

	kapi "k8s.io/kubernetes/pkg/api"
	apierrs "k8s.io/kubernetes/pkg/api/errors"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/image/api"
)

var ErrNotImportable = errors.New("the specified stream cannot be imported")

type ImportController struct {
	streams client.ImageStreamsNamespacer
}

// Notifier provides information about when the controller makes a decision
type Notifier interface {
	// Importing is invoked when the controller is going to import an image stream
	Importing(stream *api.ImageStream)
}

// NotifierFunc implements Notifier
type NotifierFunc func(stream *api.ImageStream)

// Importing adapts NotifierFunc to Notifier
func (fn NotifierFunc) Importing(stream *api.ImageStream) {
	fn(stream)
}

// tagImportable is true if the given TagReference is importable by this controller
func tagImportable(tagRef api.TagReference) bool {
	if tagRef.From == nil {
		return false
	}
	if tagRef.From.Kind != "DockerImage" || tagRef.Reference {
		return false
	}
	return true
}

// tagNeedsImport is true if the observed tag generation for this tag is older than the
// specified tag generation (if no tag generation is specified, the controller does not
// need to import this tag).
func tagNeedsImport(stream *api.ImageStream, tag string, tagRef api.TagReference, importWhenGenerationNil bool) bool {
	if !tagImportable(tagRef) {
		return false
	}
	if tagRef.Generation == nil {
		return importWhenGenerationNil
	}
	return *tagRef.Generation > api.LatestObservedTagGeneration(stream, tag)
}

// needsImport returns true if the provided image stream should have tags imported. Partial is returned
// as true if the spec.dockerImageRepository does not need to be refreshed (if only tags have to be imported).
func needsImport(stream *api.ImageStream) (ok bool, partial bool) {
	if stream.Annotations == nil || len(stream.Annotations[api.DockerImageRepositoryCheckAnnotation]) == 0 {
		if len(stream.Spec.DockerImageRepository) > 0 {
			return true, false
		}
		// for backwards compatibility, if any of our tags are importable, trigger a partial import when the
		// annotation is cleared.
		for _, tagRef := range stream.Spec.Tags {
			if tagImportable(tagRef) {
				return true, true
			}
		}
	}
	// find any tags with a newer generation than their status
	for tag, tagRef := range stream.Spec.Tags {
		if tagNeedsImport(stream, tag, tagRef, false) {
			return true, true
		}
	}
	return false, false
}

// needsScheduling returns true if this image stream has any scheduled tags
func needsScheduling(stream *api.ImageStream) bool {
	for _, tagRef := range stream.Spec.Tags {
		if tagImportable(tagRef) && tagRef.ImportPolicy.Scheduled {
			return true
		}
	}
	return false
}

// resetScheduledTags artificially increments the generation on the tags that should be imported.
func resetScheduledTags(stream *api.ImageStream) {
	next := stream.Generation + 1
	for tag, tagRef := range stream.Spec.Tags {
		if tagImportable(tagRef) && tagRef.ImportPolicy.Scheduled {
			tagRef.Generation = &next
			stream.Spec.Tags[tag] = tagRef
		}
	}
}

// Next processes the given image stream, looking for streams that have DockerImageRepository
// set but have not yet been marked as "ready". If transient errors occur, err is returned but
// the image stream is not modified (so it will be tried again later). If a permanent
// failure occurs the image is marked with an annotation and conditions are set on the status
// tags. The tags of the original spec image are left as is (those are updated through status).
//
// There are 3 scenarios:
//
// 1. spec.DockerImageRepository defined without any tags results in all tags being imported
//    from upstream image repository
//
// 2. spec.DockerImageRepository + tags defined - import all tags from upstream image repository,
//    and all the specified which (if name matches) will overwrite the default ones.
//    Additionally:
//    for kind == DockerImage import or reference underlying image, exact tag (not provided means latest),
//    for kind != DockerImage reference tag from the same or other ImageStream
//
// 3. spec.DockerImageRepository not defined - import tags per each definition.
//
// Notifier, if passed, will be invoked if the stream is going to be imported.
func (c *ImportController) Next(stream *api.ImageStream, notifier Notifier) error {
	ok, partial := needsImport(stream)
	if !ok {
		return nil
	}
	glog.V(3).Infof("Importing stream %s/%s partial=%t...", stream.Namespace, stream.Name, partial)

	if notifier != nil {
		notifier.Importing(stream)
	}

	isi := &api.ImageStreamImport{
		ObjectMeta: kapi.ObjectMeta{
			Name:            stream.Name,
			Namespace:       stream.Namespace,
			ResourceVersion: stream.ResourceVersion,
			UID:             stream.UID,
		},
		Spec: api.ImageStreamImportSpec{Import: true},
	}
	for tag, tagRef := range stream.Spec.Tags {
		if !(partial && tagImportable(tagRef)) && !tagNeedsImport(stream, tag, tagRef, true) {
			continue
		}
		isi.Spec.Images = append(isi.Spec.Images, api.ImageImportSpec{
			From:         kapi.ObjectReference{Kind: "DockerImage", Name: tagRef.From.Name},
			To:           &kapi.LocalObjectReference{Name: tag},
			ImportPolicy: tagRef.ImportPolicy,
		})
	}
	if repo := stream.Spec.DockerImageRepository; !partial && len(repo) > 0 {
		insecure := stream.Annotations[api.InsecureRepositoryAnnotation] == "true"
		isi.Spec.Repository = &api.RepositoryImportSpec{
			From:         kapi.ObjectReference{Kind: "DockerImage", Name: repo},
			ImportPolicy: api.TagImportPolicy{Insecure: insecure},
		}
	}
	result, err := c.streams.ImageStreams(stream.Namespace).Import(isi)
	if err != nil {
		if apierrs.IsNotFound(err) && client.IsStatusErrorKind(err, "imageStream") {
			return ErrNotImportable
		}
		glog.V(4).Infof("Import stream %s/%s partial=%t error: %v", stream.Namespace, stream.Name, partial, err)
	} else {
		glog.V(5).Infof("Import stream %s/%s partial=%t import: %#v", stream.Namespace, stream.Name, partial, result.Status.Import)
	}
	return err
}

func (c *ImportController) NextTimedByName(namespace, name string) error {
	stream, err := c.streams.ImageStreams(namespace).Get(name)
	if err != nil {
		if apierrs.IsNotFound(err) {
			return ErrNotImportable
		}
		return err
	}
	return c.NextTimed(stream)
}

func (c *ImportController) NextTimed(stream *api.ImageStream) error {
	if !needsScheduling(stream) {
		return ErrNotImportable
	}
	resetScheduledTags(stream)

	glog.V(3).Infof("Scheduled import of stream %s/%s...", stream.Namespace, stream.Name)

	return c.Next(stream, nil)
}
