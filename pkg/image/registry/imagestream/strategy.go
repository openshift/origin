package imagestream

import (
	"fmt"
	"strings"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/registry/generic"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/fielderrors"
	"github.com/golang/glog"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/registry/subjectaccessreview"
	"github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/image/api/validation"
)

type ResourceGetter interface {
	Get(kapi.Context, string) (runtime.Object, error)
}

// Strategy implements behavior for ImageStreams.
type Strategy struct {
	runtime.ObjectTyper
	kapi.NameGenerator
	defaultRegistry   DefaultRegistry
	tagVerifier       *TagVerifier
	ImageStreamGetter ResourceGetter
}

// Strategy is the default logic that applies when creating and updating
// ImageStream objects via the REST API.
func NewStrategy(defaultRegistry DefaultRegistry, subjectAccessReviewClient subjectaccessreview.Registry) Strategy {
	return Strategy{
		ObjectTyper:     kapi.Scheme,
		NameGenerator:   kapi.SimpleNameGenerator,
		defaultRegistry: defaultRegistry,
		tagVerifier:     &TagVerifier{subjectAccessReviewClient},
	}
}

// NamespaceScoped is true for image streams.
func (s Strategy) NamespaceScoped() bool {
	return true
}

// PrepareForCreate clears fields that are not allowed to be set by end users on creation,
// and verifies the current user is authorized to access any image streams newly referenced
// in spec.tags.
func (s Strategy) PrepareForCreate(obj runtime.Object) {
	stream := obj.(*api.ImageStream)
	stream.Status = api.ImageStreamStatus{
		DockerImageRepository: s.dockerImageRepository(stream),
		Tags: make(map[string]api.TagEventList),
	}
}

// Validate validates a new image stream.
func (s Strategy) Validate(ctx kapi.Context, obj runtime.Object) fielderrors.ValidationErrorList {
	stream := obj.(*api.ImageStream)
	user, ok := kapi.UserFrom(ctx)
	if !ok {
		return fielderrors.ValidationErrorList{kerrors.NewForbidden("imageStream", stream.Name, fmt.Errorf("unable to update an ImageStream without a user on the context"))}
	}
	errs := s.tagVerifier.Verify(nil, stream, user.GetName())
	errs = append(errs, s.tagsChanged(nil, stream)...)
	errs = append(errs, validation.ValidateImageStream(stream)...)
	return errs
}

// AllowCreateOnUpdate is false for image streams.
func (s Strategy) AllowCreateOnUpdate() bool {
	return false
}

// dockerImageRepository determines the docker image stream for stream.
// If stream.DockerImageRepository is set, that value is returned. Otherwise,
// if a default registry exists, the value returned is of the form
// <default registry>/<namespace>/<stream name>.
func (s Strategy) dockerImageRepository(stream *api.ImageStream) string {
	if len(stream.Spec.DockerImageRepository) != 0 {
		return stream.Spec.DockerImageRepository
	}

	registry, ok := s.defaultRegistry.DefaultRegistry()
	if !ok {
		return ""
	}

	if len(stream.Namespace) == 0 {
		stream.Namespace = kapi.NamespaceDefault
	}
	ref := api.DockerImageReference{
		Registry:  registry,
		Namespace: stream.Namespace,
		Name:      stream.Name,
	}
	return ref.String()
}

func parseFromReference(stream *api.ImageStream, from *kapi.ObjectReference) (string, string, error) {
	splitChar := ""
	refType := ""

	switch from.Kind {
	case "ImageStreamTag":
		splitChar = ":"
		refType = "tag"
	case "ImageStreamImage":
		splitChar = "@"
		refType = "id"
	default:
		return "", "", fmt.Errorf("invalid from.kind %q - only ImageStreamTag and ImageStreamImage are allowed", from.Kind)
	}

	parts := strings.Split(from.Name, splitChar)
	switch len(parts) {
	case 1:
		// <tag> or <id>
		return stream.Name, from.Name, nil
	case 2:
		// <stream>:<tag> or <stream>@<id>
		return parts[0], parts[1], nil
	default:
		return "", "", fmt.Errorf("invalid from.name %q - it must be of the form <%s> or <stream>%s<%s>", from.Name, refType, splitChar, refType)
	}
}

// tagsChanged updates stream.Status.Tags based on the old and new image stream.
// if the old stream is nil, all tags are considered additions.
func (s Strategy) tagsChanged(old, stream *api.ImageStream) fielderrors.ValidationErrorList {
	var errs fielderrors.ValidationErrorList

	oldTags := map[string]api.TagReference{}
	if old != nil && old.Spec.Tags != nil {
		oldTags = old.Spec.Tags
	}

	for tag, tagRef := range stream.Spec.Tags {
		if oldRef, ok := oldTags[tag]; ok && !tagRefChanged(oldRef, tagRef, stream.Namespace) {
			continue
		}
		if len(tagRef.DockerImageReference) > 0 {
			event, err := tagReferenceToTagEvent(stream, tagRef, "")
			if err != nil {
				errs = append(errs, fielderrors.NewFieldInvalid(fmt.Sprintf("spec.tags[%s].dockerImageReference", tag), tagRef.DockerImageReference, err.Error()))
				continue
			}
			api.AddTagEventToImageStream(stream, tag, *event)
			continue
		}

		if tagRef.From == nil {
			continue
		}

		tagRefStreamName, tagOrID, err := parseFromReference(stream, tagRef.From)
		if err != nil {
			errs = append(errs, fielderrors.NewFieldInvalid(fmt.Sprintf("spec.tags[%s].from.name", tag), tagRef.From.Name, "must be of the form <tag>, <repo>:<tag>, <id>, or <repo>@<id>"))
			continue
		}

		streamRef := stream
		streamRefNamespace := tagRef.From.Namespace
		if len(streamRefNamespace) == 0 {
			streamRefNamespace = stream.Namespace
		}
		if streamRefNamespace != stream.Namespace || tagRefStreamName != stream.Name {
			obj, err := s.ImageStreamGetter.Get(kapi.WithNamespace(kapi.NewContext(), streamRefNamespace), tagRefStreamName)
			if err != nil {
				errs = append(errs, fielderrors.NewFieldInvalid(fmt.Sprintf("spec.tags[%s].from.name", tag), tagRef.From.Name, fmt.Sprintf("error retrieving ImageStream %s/%s: %v", streamRefNamespace, tagRefStreamName, err)))
				continue
			}

			streamRef = obj.(*api.ImageStream)
		}

		event, err := tagReferenceToTagEvent(streamRef, tagRef, tagOrID)
		if err != nil {
			errs = append(errs, fielderrors.NewFieldInvalid(fmt.Sprintf("spec.tags[%s].from.name", tag), tagRef.From.Name, fmt.Sprintf("error generating tag event: %v", err)))
			continue
		}

		if event == nil {
			glog.Errorf("unable to find tag event for %#v", tagRef.From)
			continue
		}

		api.AddTagEventToImageStream(stream, tag, *event)
	}

	// use a consistent timestamp on creation
	if old == nil && !stream.CreationTimestamp.IsZero() {
		for tag, list := range stream.Status.Tags {
			for _, event := range list.Items {
				event.Created = stream.CreationTimestamp
			}
			stream.Status.Tags[tag] = list
		}
	}

	return errs
}

func tagReferenceToTagEvent(stream *api.ImageStream, tagRef api.TagReference, tagOrID string) (*api.TagEvent, error) {
	if len(tagRef.DockerImageReference) > 0 {
		return &api.TagEvent{
			Created:              util.Now(),
			DockerImageReference: tagRef.DockerImageReference,
		}, nil
	}

	switch tagRef.From.Kind {
	case "ImageStreamImage":
		ref, err := api.DockerImageReferenceForStream(stream)
		if err != nil {
			return nil, err
		}
		ref.ID = tagOrID
		return &api.TagEvent{
			Created:              util.Now(),
			DockerImageReference: ref.String(),
			Image:                ref.ID,
		}, nil
	case "ImageStreamTag":
		return api.LatestTaggedImage(stream, tagOrID), nil
	default:
		return nil, fmt.Errorf("invalid from.kind %q: it must be ImageStreamImage or ImageStreamTag", tagRef.From.Kind)
	}
}

func tagRefChanged(old, next api.TagReference, streamNamespace string) bool {
	if len(next.DockerImageReference) > 0 {
		// DockerImageReference possibly changed
		return next.DockerImageReference != old.DockerImageReference
	}
	if next.From == nil {
		// both fields in next are empty
		return false
	}
	if len(next.From.Kind) == 0 && len(next.From.Name) == 0 {
		// invalid
		return false
	}
	oldFrom := old.From
	if oldFrom == nil {
		oldFrom = &kapi.ObjectReference{}
	}
	oldNamespace := oldFrom.Namespace
	if len(oldNamespace) == 0 {
		oldNamespace = streamNamespace
	}
	nextNamespace := next.From.Namespace
	if len(nextNamespace) == 0 {
		nextNamespace = streamNamespace
	}
	if oldNamespace != nextNamespace {
		return true
	}
	if oldFrom.Name != next.From.Name {
		return true
	}
	return false
}

type TagVerifier struct {
	subjectAccessReviewClient subjectaccessreview.Registry
}

func (v *TagVerifier) Verify(old, stream *api.ImageStream, user string) fielderrors.ValidationErrorList {
	var errors fielderrors.ValidationErrorList
	oldTags := map[string]api.TagReference{}
	if old != nil && old.Spec.Tags != nil {
		oldTags = old.Spec.Tags
	}
	for tag, tagRef := range stream.Spec.Tags {
		if tagRef.From == nil {
			continue
		}
		if len(tagRef.From.Namespace) == 0 {
			continue
		}
		if stream.Namespace == tagRef.From.Namespace {
			continue
		}
		if oldRef, ok := oldTags[tag]; ok && !tagRefChanged(oldRef, tagRef, stream.Namespace) {
			continue
		}

		glog.Infof("validating access for %s to %v", user, tagRef.From)
		streamName, _, err := parseFromReference(stream, tagRef.From)
		if err != nil {
			errors = append(errors, fielderrors.NewFieldInvalid(fmt.Sprintf("spec.tags[%s].from.name", tag), tagRef.From.Name, "must be of the form <tag>, <repo>:<tag>, <id>, or <repo>@<id>"))
			continue
		}

		subjectAccessReview := authorizationapi.SubjectAccessReview{
			Verb:         "get",
			Resource:     "imageStream",
			User:         user,
			ResourceName: streamName,
		}
		ctx := kapi.WithNamespace(kapi.NewContext(), tagRef.From.Namespace)
		glog.V(1).Infof("Performing SubjectAccessReview for user %s to %s/%s", user, tagRef.From.Namespace, streamName)
		resp, err := v.subjectAccessReviewClient.CreateSubjectAccessReview(ctx, &subjectAccessReview)
		if err != nil || resp == nil || (resp != nil && !resp.Allowed) {
			errors = append(errors, fielderrors.NewFieldForbidden(fmt.Sprintf("spec.tags[%s].from", tag), fmt.Sprintf("%s/%s", tagRef.From.Namespace, streamName)))
			continue
		}
	}
	return errors
}

func (s Strategy) PrepareForUpdate(obj, old runtime.Object) {
	oldStream := old.(*api.ImageStream)
	stream := obj.(*api.ImageStream)

	stream.Status = oldStream.Status
	stream.Status.DockerImageRepository = s.dockerImageRepository(stream)
}

// ValidateUpdate is the default update validation for an end user.
func (s Strategy) ValidateUpdate(ctx kapi.Context, obj, old runtime.Object) fielderrors.ValidationErrorList {
	stream := obj.(*api.ImageStream)

	user, ok := kapi.UserFrom(ctx)
	if !ok {
		return fielderrors.ValidationErrorList{kerrors.NewForbidden("imageStream", stream.Name, fmt.Errorf("unable to update an ImageStream without a user on the context"))}
	}

	oldStream := old.(*api.ImageStream)

	errs := s.tagVerifier.Verify(oldStream, stream, user.GetName())
	errs = append(errs, s.tagsChanged(oldStream, stream)...)
	errs = append(errs, validation.ValidateImageStreamUpdate(stream, oldStream)...)
	return errs
}

// Decorate decorates stream.Status.DockerImageRepository using the logic from
// dockerImageRepository().
func (s Strategy) Decorate(obj runtime.Object) error {
	ir := obj.(*api.ImageStream)
	ir.Status.DockerImageRepository = s.dockerImageRepository(ir)
	return nil
}

type StatusStrategy struct {
	Strategy
}

// NewStatusStrategy creates a status update strategy around an existing stream
// strategy.
func NewStatusStrategy(strategy Strategy) StatusStrategy {
	return StatusStrategy{strategy}
}

func (StatusStrategy) PrepareForUpdate(obj, old runtime.Object) {
}

func (StatusStrategy) ValidateUpdate(ctx kapi.Context, obj, old runtime.Object) fielderrors.ValidationErrorList {
	// TODO: merge valid fields after update
	return validation.ValidateImageStreamStatusUpdate(obj.(*api.ImageStream), old.(*api.ImageStream))
}

// MatchImageStream returns a generic matcher for a given label and field selector.
func MatchImageStream(label labels.Selector, field fields.Selector) generic.Matcher {
	return generic.MatcherFunc(func(obj runtime.Object) (bool, error) {
		ir, ok := obj.(*api.ImageStream)
		if !ok {
			return false, fmt.Errorf("not an ImageStream")
		}
		fields := ImageStreamToSelectableFields(ir)
		return label.Matches(labels.Set(ir.Labels)) && field.Matches(fields), nil
	})
}

// ImageStreamToSelectableFields returns a label set that represents the object.
func ImageStreamToSelectableFields(ir *api.ImageStream) labels.Set {
	return labels.Set{
		"metadata.name":                ir.Name,
		"spec.dockerImageRepository":   ir.Spec.DockerImageRepository,
		"status.dockerImageRepository": ir.Status.DockerImageRepository,
	}
}

// DefaultRegistry returns the default Docker registry (host or host:port), or false if it is not available.
type DefaultRegistry interface {
	DefaultRegistry() (string, bool)
}

// DefaultRegistryFunc implements DefaultRegistry for a simple function.
type DefaultRegistryFunc func() (string, bool)

// DefaultRegistry implements the DefaultRegistry interface for a function.
func (fn DefaultRegistryFunc) DefaultRegistry() (string, bool) {
	return fn()
}
