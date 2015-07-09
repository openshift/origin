package api

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/docker/distribution/digest"
	"github.com/golang/glog"
)

// DockerDefaultNamespace is the value for namespace when a single segment name is provided.
const DockerDefaultNamespace = "library"

// TODO remove (base, tag, id)
func parseRepositoryTag(repos string) (string, string, string) {
	n := strings.Index(repos, "@")
	if n >= 0 {
		parts := strings.Split(repos, "@")
		return parts[0], "", parts[1]
	}
	n = strings.LastIndex(repos, ":")
	if n < 0 {
		return repos, "", ""
	}
	if tag := repos[n+1:]; !strings.Contains(tag, "/") {
		return repos[:n], tag, ""
	}
	return repos, "", ""
}

func isRegistryName(str string) bool {
	switch {
	case strings.Contains(str, ":"),
		strings.Contains(str, "."),
		str == "localhost":
		return true
	}
	return false
}

// ParseDockerImageReference parses a Docker pull spec string into a
// DockerImageReference.
func ParseDockerImageReference(spec string) (DockerImageReference, error) {
	var (
		ref DockerImageReference
	)
	// TODO replace with docker version once docker/docker PR11109 is merged upstream
	stream, tag, id := parseRepositoryTag(spec)

	repoParts := strings.Split(stream, "/")
	switch len(repoParts) {
	case 2:
		if isRegistryName(repoParts[0]) {
			// registry/name
			ref.Registry = repoParts[0]
			// TODO: default this in all cases where Namespace ends up as ""?
			ref.Namespace = DockerDefaultNamespace
			if len(repoParts[1]) == 0 {
				return ref, fmt.Errorf("the docker pull spec %q must be two or three segments separated by slashes", spec)
			}
			ref.Name = repoParts[1]
			ref.Tag = tag
			ref.ID = id
			break
		}
		// namespace/name
		ref.Namespace = repoParts[0]
		if len(repoParts[1]) == 0 {
			return ref, fmt.Errorf("the docker pull spec %q must be two or three segments separated by slashes", spec)
		}
		ref.Name = repoParts[1]
		ref.Tag = tag
		ref.ID = id
		break
	case 3:
		// registry/namespace/name
		ref.Registry = repoParts[0]
		ref.Namespace = repoParts[1]
		if len(repoParts[2]) == 0 {
			return ref, fmt.Errorf("the docker pull spec %q must be two or three segments separated by slashes", spec)
		}
		ref.Name = repoParts[2]
		ref.Tag = tag
		ref.ID = id
		break
	case 1:
		// name
		if len(repoParts[0]) == 0 {
			return ref, fmt.Errorf("the docker pull spec %q must be two or three segments separated by slashes", spec)
		}
		ref.Name = repoParts[0]
		ref.Tag = tag
		ref.ID = id
		break
	default:
		return ref, fmt.Errorf("the docker pull spec %q must be two or three segments separated by slashes", spec)
	}

	// TODO: validate repository name here?
	//
	// repo := ref.Name
	// if len(ref.Namespace) > 0 {
	// 	repo = ref.Namespace + "/" + ref.Name
	// }

	// if err := v2.ValidateRepositoryName(repo); err != nil {
	// 	return DockerImageReference{}, err
	// }

	return ref, nil
}

// DockerClientDefaults sets the default values used by the Docker client.
func (r DockerImageReference) DockerClientDefaults() DockerImageReference {
	if len(r.Namespace) == 0 {
		r.Namespace = "library"
	}
	if len(r.Registry) == 0 {
		r.Registry = "docker.io"
	}
	if len(r.Tag) == 0 {
		r.Tag = DefaultImageTag
	}
	return r
}

// Minimal reduces a DockerImageReference to its minimalist form.
func (r DockerImageReference) Minimal() DockerImageReference {
	if r.Tag == DefaultImageTag {
		r.Tag = ""
	}
	return r
}

// String converts a DockerImageReference to a Docker pull spec.
func (r DockerImageReference) String() string {
	registry := r.Registry
	if len(registry) > 0 {
		registry += "/"
	}

	if len(r.Namespace) == 0 {
		r.Namespace = DockerDefaultNamespace
	}
	r.Namespace += "/"

	var ref string
	if len(r.Tag) > 0 {
		ref = ":" + r.Tag
	} else if len(r.ID) > 0 {
		if _, err := digest.ParseDigest(r.ID); err == nil {
			// if it parses as a digest, it's v2 pull by id
			ref = "@" + r.ID
		} else {
			// if it doesn't parse as a digest, it's presumably a v1 registry by-id tag
			ref = ":" + r.ID
		}
	}

	return fmt.Sprintf("%s%s%s%s", registry, r.Namespace, r.Name, ref)
}

// SplitImageStreamTag turns the name of an ImageStreamTag into Name and Tag.
// It returns false if the tag was not properly specified in the name.
func SplitImageStreamTag(nameAndTag string) (name string, tag string, ok bool) {
	parts := strings.SplitN(nameAndTag, ":", 2)
	name = parts[0]
	if len(parts) > 1 {
		tag = parts[1]
	}
	if len(tag) == 0 {
		tag = DefaultImageTag
	}
	return name, tag, len(parts) == 2
}

// JoinImageStreamTag turns a name and tag into the name of an ImageStreamTag
func JoinImageStreamTag(name, tag string) string {
	if len(tag) == 0 {
		tag = DefaultImageTag
	}
	return fmt.Sprintf("%s:%s", name, tag)
}

// ImageWithMetadata returns a copy of image with the DockerImageMetadata filled in
// from the raw DockerImageManifest data stored in the image.
func ImageWithMetadata(image Image) (*Image, error) {
	if len(image.DockerImageManifest) == 0 {
		return &image, nil
	}

	manifestData := image.DockerImageManifest

	image.DockerImageManifest = ""

	manifest := DockerImageManifest{}
	if err := json.Unmarshal([]byte(manifestData), &manifest); err != nil {
		return nil, err
	}

	if len(manifest.History) == 0 {
		// should never have an empty history, but just in case...
		return &image, nil
	}

	v1Metadata := DockerV1CompatibilityImage{}
	if err := json.Unmarshal([]byte(manifest.History[0].DockerV1Compatibility), &v1Metadata); err != nil {
		return nil, err
	}

	image.DockerImageMetadata.ID = v1Metadata.ID
	image.DockerImageMetadata.Parent = v1Metadata.Parent
	image.DockerImageMetadata.Comment = v1Metadata.Comment
	image.DockerImageMetadata.Created = v1Metadata.Created
	image.DockerImageMetadata.Container = v1Metadata.Container
	image.DockerImageMetadata.ContainerConfig = v1Metadata.ContainerConfig
	image.DockerImageMetadata.DockerVersion = v1Metadata.DockerVersion
	image.DockerImageMetadata.Author = v1Metadata.Author
	image.DockerImageMetadata.Config = v1Metadata.Config
	image.DockerImageMetadata.Architecture = v1Metadata.Architecture
	image.DockerImageMetadata.Size = v1Metadata.Size

	return &image, nil
}

// DockerImageReferenceForStream returns a DockerImageReference that represents
// the ImageStream or false, if no valid reference exists.
func DockerImageReferenceForStream(stream *ImageStream) (DockerImageReference, error) {
	spec := stream.Status.DockerImageRepository
	if len(spec) == 0 {
		spec = stream.Spec.DockerImageRepository
	}
	if len(spec) == 0 {
		return DockerImageReference{}, fmt.Errorf("no possible pull spec for %s/%s", stream.Namespace, stream.Name)
	}
	return ParseDockerImageReference(spec)
}

// LatestTaggedImage returns the most recent TagEvent for the specified image
// repository and tag. Will resolve lookups for the empty tag. Returns nil
// if tag isn't present in stream.status.tags.
func LatestTaggedImage(stream *ImageStream, tag string) *TagEvent {
	if len(tag) == 0 {
		tag = DefaultImageTag
	}
	// find the most recent tag event with an image reference
	if stream.Status.Tags != nil {
		if history, ok := stream.Status.Tags[tag]; ok {
			return &history.Items[0]
		}
	}

	return nil
}

// AddTagEventToImageStream attempts to update the given image stream with a tag event. It will
// collapse duplicate entries - returning true if a change was made or false if no change
// occurred.
func AddTagEventToImageStream(stream *ImageStream, tag string, next TagEvent) bool {
	if stream.Status.Tags == nil {
		stream.Status.Tags = make(map[string]TagEventList)
	}

	tags, ok := stream.Status.Tags[tag]
	if !ok || len(tags.Items) == 0 {
		stream.Status.Tags[tag] = TagEventList{Items: []TagEvent{next}}
		return true
	}

	previous := &tags.Items[0]

	// image reference has not changed
	if previous.DockerImageReference == next.DockerImageReference {
		if next.Image == previous.Image {
			return false
		}
		previous.Image = next.Image
		stream.Status.Tags[tag] = tags
		return true
	}

	// image has not changed, but image reference has
	if next.Image == previous.Image {
		previous.DockerImageReference = next.DockerImageReference
		stream.Status.Tags[tag] = tags
		return true
	}

	tags.Items = append([]TagEvent{next}, tags.Items...)
	stream.Status.Tags[tag] = tags
	return true
}

// UpdateTrackingTags sets updatedImage as the most recent TagEvent for all tags
// in stream.spec.tags that have from.kind = "ImageStreamTag" and the tag in from.name
// = updatedTag. from.name may be either <tag> or <stream name>:<tag>. For now, only
// references to tags in the current stream are supported.
//
// For example, if stream.spec.tags[latest].from.name = 2.0, whenever an image is pushed
// to this stream with the tag 2.0, status.tags[latest].items[0] will also be updated
// to point at the same image that was just pushed for 2.0.
func UpdateTrackingTags(stream *ImageStream, updatedTag string, updatedImage TagEvent) {
	glog.V(5).Infof("UpdateTrackingTags: stream=%s/%s, updatedTag=%s, updatedImage.dockerImageReference=%s, updatedImage.image=%s", stream.Namespace, stream.Name, updatedTag, updatedImage.DockerImageReference, updatedImage.Image)
	for specTag, tagRef := range stream.Spec.Tags {
		glog.V(5).Infof("Examining spec tag %q, tagRef=%#v", specTag, tagRef)

		// no from
		if tagRef.From == nil {
			glog.V(5).Infof("tagRef.From is nil, skipping")
			continue
		}

		// wrong kind
		if tagRef.From.Kind != "ImageStreamTag" {
			glog.V(5).Infof("tagRef.Kind %q isn't ImageStreamTag, skipping", tagRef.From.Kind)
			continue
		}

		tagRefNamespace := tagRef.From.Namespace
		if len(tagRefNamespace) == 0 {
			tagRefNamespace = stream.Namespace
		}

		// different namespace
		if tagRefNamespace != stream.Namespace {
			glog.V(5).Infof("tagRefNamespace %q doesn't match stream namespace %q - skipping", tagRefNamespace, stream.Namespace)
			continue
		}

		tagRefName := tagRef.From.Name
		parts := strings.Split(tagRefName, ":")
		tag := ""
		switch len(parts) {
		case 2:
			// <stream>:<tag>
			tagRefName = parts[0]
			tag = parts[1]
		default:
			// <tag> (this stream)
			tag = tagRefName
			tagRefName = stream.Name
		}

		glog.V(5).Infof("tagRefName=%q, tag=%q", tagRefName, tag)

		// different stream
		if tagRefName != stream.Name {
			glog.V(5).Infof("tagRefName %q doesn't match stream name %q - skipping", tagRefName, stream.Name)
			continue
		}

		// different tag
		if tag != updatedTag {
			glog.V(5).Infof("tag %q doesn't match updated tag %q - skipping", tag, updatedTag)
			continue
		}

		updated := AddTagEventToImageStream(stream, specTag, updatedImage)
		glog.V(5).Infof("stream updated? %t", updated)
	}
}

// NameAndTag returns an image name with a tag. If the tag is blank then
// DefaultImageTag is used.
func NameAndTag(name, tag string) string {
	if len(tag) == 0 {
		tag = DefaultImageTag
	}
	return fmt.Sprintf("%s:%s", name, tag)
}

// ResolveImageID returns a StringSet of all the image IDs in stream that start with imageID.
func ResolveImageID(stream *ImageStream, imageID string) util.StringSet {
	set := util.NewStringSet()
	for _, history := range stream.Status.Tags {
		for _, tagging := range history.Items {
			if d, err := digest.ParseDigest(tagging.Image); err == nil {
				if strings.HasPrefix(d.Hex(), imageID) || strings.HasPrefix(tagging.Image, imageID) {
					set.Insert(tagging.Image)
				}
				continue
			}
			if strings.HasPrefix(tagging.Image, imageID) {
				set.Insert(tagging.Image)
			}
		}
	}
	return set
}
