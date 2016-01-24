package controller

import (
	"github.com/golang/glog"

	kapi "k8s.io/kubernetes/pkg/api"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/image/api"
)

type ImportController struct {
	streams client.ImageStreamsNamespacer
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

// retryCount is the number of times to retry on a conflict when updating an image stream
const retryCount = 2

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
func (c *ImportController) Next(stream *api.ImageStream) error {
	ok, partial := needsImport(stream)
	if !ok {
		return nil
	}
	glog.V(3).Infof("Importing stream %s/%s partial=%t...", stream.Namespace, stream.Name, partial)

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
		glog.V(4).Infof("Import stream %s/%s partial=%t error: %v", stream.Namespace, stream.Name, partial, err)
	} else {
		glog.V(5).Infof("Import stream %s/%s partial=%t import: %#v", stream.Namespace, stream.Name, partial, result.Status.Import)
	}
	return err
}
