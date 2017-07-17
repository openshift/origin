package imagestream

import (
	"fmt"
	"strings"

	"github.com/golang/glog"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/authentication/user"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	kstorage "k8s.io/apiserver/pkg/storage"
	"k8s.io/apiserver/pkg/storage/names"
	kapi "k8s.io/kubernetes/pkg/api"
	kapihelper "k8s.io/kubernetes/pkg/api/helper"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	"github.com/openshift/origin/pkg/authorization/registry/subjectaccessreview"
	imageadmission "github.com/openshift/origin/pkg/image/admission"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	"github.com/openshift/origin/pkg/image/apis/image/validation"
)

type ResourceGetter interface {
	Get(apirequest.Context, string, *metav1.GetOptions) (runtime.Object, error)
}

// Strategy implements behavior for ImageStreams.
type Strategy struct {
	runtime.ObjectTyper
	names.NameGenerator
	defaultRegistry   imageapi.DefaultRegistry
	tagVerifier       *TagVerifier
	limitVerifier     imageadmission.LimitVerifier
	imageStreamGetter ResourceGetter
}

// NewStrategy is the default logic that applies when creating and updating
// ImageStream objects via the REST API.
func NewStrategy(defaultRegistry imageapi.DefaultRegistry, subjectAccessReviewClient subjectaccessreview.Registry, limitVerifier imageadmission.LimitVerifier, imageStreamGetter ResourceGetter) Strategy {
	return Strategy{
		ObjectTyper:       kapi.Scheme,
		NameGenerator:     names.SimpleNameGenerator,
		defaultRegistry:   defaultRegistry,
		tagVerifier:       &TagVerifier{subjectAccessReviewClient},
		limitVerifier:     limitVerifier,
		imageStreamGetter: imageStreamGetter,
	}
}

// NamespaceScoped is true for image streams.
func (s Strategy) NamespaceScoped() bool {
	return true
}

// PrepareForCreate clears fields that are not allowed to be set by end users on creation.
func (s Strategy) PrepareForCreate(ctx apirequest.Context, obj runtime.Object) {
	stream := obj.(*imageapi.ImageStream)
	stream.Status = imageapi.ImageStreamStatus{
		DockerImageRepository: s.dockerImageRepository(stream),
		Tags: make(map[string]imageapi.TagEventList),
	}
	stream.Generation = 1
	for tag, ref := range stream.Spec.Tags {
		ref.Generation = &stream.Generation
		stream.Spec.Tags[tag] = ref
	}
}

// Validate validates a new image stream and verifies the current user is
// authorized to access any image streams newly referenced in spec.tags.
func (s Strategy) Validate(ctx apirequest.Context, obj runtime.Object) field.ErrorList {
	stream := obj.(*imageapi.ImageStream)
	var errs field.ErrorList
	if err := s.validateTagsAndLimits(ctx, nil, stream); err != nil {
		errs = append(errs, field.InternalError(field.NewPath(""), err))
	}
	errs = append(errs, validation.ValidateImageStream(stream)...)
	return errs
}

func (s Strategy) validateTagsAndLimits(ctx apirequest.Context, oldStream, newStream *imageapi.ImageStream) error {
	user, ok := apirequest.UserFrom(ctx)
	if !ok {
		return kerrors.NewForbidden(schema.GroupResource{Resource: "imagestreams"}, newStream.Name, fmt.Errorf("no user context available"))
	}

	errs := s.tagVerifier.Verify(oldStream, newStream, user)
	errs = append(errs, s.tagsChanged(oldStream, newStream)...)
	if len(errs) > 0 {
		return kerrors.NewInvalid(schema.GroupKind{Kind: "imagestreams"}, newStream.Name, errs)
	}

	ns, ok := apirequest.NamespaceFrom(ctx)
	if !ok {
		ns = newStream.Namespace
	}
	return s.limitVerifier.VerifyLimits(ns, newStream)
}

// AllowCreateOnUpdate is false for image streams.
func (s Strategy) AllowCreateOnUpdate() bool {
	return false
}

func (Strategy) AllowUnconditionalUpdate() bool {
	return false
}

// dockerImageRepository determines the docker image stream for stream.
// If stream.DockerImageRepository is set, that value is returned. Otherwise,
// if a default registry exists, the value returned is of the form
// <default registry>/<namespace>/<stream name>.
func (s Strategy) dockerImageRepository(stream *imageapi.ImageStream) string {
	registry, ok := s.defaultRegistry.DefaultRegistry()
	if !ok {
		return stream.Spec.DockerImageRepository
	}

	if len(stream.Namespace) == 0 {
		stream.Namespace = metav1.NamespaceDefault
	}
	ref := imageapi.DockerImageReference{
		Registry:  registry,
		Namespace: stream.Namespace,
		Name:      stream.Name,
	}
	return ref.String()
}

func parseFromReference(stream *imageapi.ImageStream, from *kapi.ObjectReference) (string, string, error) {
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
func (s Strategy) tagsChanged(old, stream *imageapi.ImageStream) field.ErrorList {
	internalRegistry, hasInternalRegistry := s.defaultRegistry.DefaultRegistry()

	var errs field.ErrorList

	oldTags := map[string]imageapi.TagReference{}
	if old != nil && old.Spec.Tags != nil {
		oldTags = old.Spec.Tags
	}

	for tag, tagRef := range stream.Spec.Tags {
		if oldRef, ok := oldTags[tag]; ok && !tagRefChanged(oldRef, tagRef, stream.Namespace) {
			continue
		}

		if tagRef.From == nil {
			continue
		}

		glog.V(5).Infof("Detected changed tag %s in %s/%s", tag, stream.Namespace, stream.Name)

		generation := stream.Generation
		tagRef.Generation = &generation

		fromPath := field.NewPath("spec", "tags").Key(tag).Child("from")
		if tagRef.From.Kind == "DockerImage" && len(tagRef.From.Name) > 0 {
			if tagRef.Reference {
				event, err := tagReferenceToTagEvent(stream, tagRef, "")
				if err != nil {
					errs = append(errs, field.Invalid(fromPath, tagRef.From, err.Error()))
					continue
				}
				stream.Spec.Tags[tag] = tagRef
				imageapi.AddTagEventToImageStream(stream, tag, *event)
			}
			continue
		}

		tagRefStreamName, tagOrID, err := parseFromReference(stream, tagRef.From)
		if err != nil {
			errs = append(errs, field.Invalid(fromPath.Child("name"), tagRef.From.Name, "must be of the form <tag>, <repo>:<tag>, <id>, or <repo>@<id>"))
			continue
		}

		streamRef := stream
		streamRefNamespace := tagRef.From.Namespace
		if len(streamRefNamespace) == 0 {
			streamRefNamespace = stream.Namespace
		}
		if streamRefNamespace != stream.Namespace || tagRefStreamName != stream.Name {
			obj, err := s.imageStreamGetter.Get(apirequest.WithNamespace(apirequest.NewContext(), streamRefNamespace), tagRefStreamName, &metav1.GetOptions{})
			if err != nil {
				if kerrors.IsNotFound(err) {
					errs = append(errs, field.NotFound(fromPath.Child("name"), tagRef.From.Name))
				} else {
					errs = append(errs, field.Invalid(fromPath.Child("name"), tagRef.From.Name, fmt.Sprintf("unable to retrieve image stream: %v", err)))
				}
				continue
			}

			streamRef = obj.(*imageapi.ImageStream)
		}

		event, err := tagReferenceToTagEvent(streamRef, tagRef, tagOrID)
		if err != nil {
			errs = append(errs, field.Invalid(fromPath.Child("name"), tagRef.From.Name, fmt.Sprintf("error generating tag event: %v", err)))
			continue
		}
		if event == nil {
			// referenced tag or ID doesn't exist, which is ok
			continue
		}

		// if this is not a reference tag, and the tag points to the internal registry for the other namespace, alter it to
		// point to this stream so that pulls happen from this stream in the future.
		if !tagRef.Reference {
			if ref, err := imageapi.ParseDockerImageReference(event.DockerImageReference); err == nil {
				if hasInternalRegistry && ref.Registry == internalRegistry && ref.Namespace == streamRef.Namespace && ref.Name == streamRef.Name {
					ref.Namespace = stream.Namespace
					ref.Name = stream.Name
					event.DockerImageReference = ref.Exact()
				}
			}
		}

		stream.Spec.Tags[tag] = tagRef
		imageapi.AddTagEventToImageStream(stream, tag, *event)
	}

	imageapi.UpdateChangedTrackingTags(stream, old)

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

func tagReferenceToTagEvent(stream *imageapi.ImageStream, tagRef imageapi.TagReference, tagOrID string) (*imageapi.TagEvent, error) {
	var (
		event *imageapi.TagEvent
		err   error
	)
	switch tagRef.From.Kind {
	case "DockerImage":
		event = &imageapi.TagEvent{
			Created:              metav1.Now(),
			DockerImageReference: tagRef.From.Name,
		}

	case "ImageStreamImage":
		event, err = imageapi.ResolveImageID(stream, tagOrID)
	case "ImageStreamTag":
		event, err = imageapi.LatestTaggedImage(stream, tagOrID), nil
	default:
		err = fmt.Errorf("invalid from.kind %q: it must be DockerImage, ImageStreamImage or ImageStreamTag", tagRef.From.Kind)
	}
	if err != nil {
		return nil, err
	}
	if event != nil && tagRef.Generation != nil {
		event.Generation = *tagRef.Generation
	}
	return event, nil
}

// tagRefChanged returns true if the tag ref changed between two spec updates.
func tagRefChanged(old, next imageapi.TagReference, streamNamespace string) bool {
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
	return tagRefGenerationChanged(old, next)
}

// tagRefGenerationChanged returns true if and only the values were set and the new generation
// is at zero.
func tagRefGenerationChanged(old, next imageapi.TagReference) bool {
	switch {
	case old.Generation != nil && next.Generation != nil:
		if *old.Generation == *next.Generation {
			return false
		}
		if *next.Generation == 0 {
			return true
		}
		return false
	default:
		return false
	}
}

func tagEventChanged(old, next imageapi.TagEvent) bool {
	return old.Image != next.Image || old.DockerImageReference != next.DockerImageReference || old.Generation > next.Generation
}

// updateSpecTagGenerationsForUpdate ensures that new spec updates always have a generation set, and that the value
// cannot be updated by an end user (except by setting generation 0).
func updateSpecTagGenerationsForUpdate(stream, oldStream *imageapi.ImageStream) {
	for tag, ref := range stream.Spec.Tags {
		if ref.Generation != nil && *ref.Generation == 0 {
			continue
		}
		if oldRef, ok := oldStream.Spec.Tags[tag]; ok {
			ref.Generation = oldRef.Generation
			stream.Spec.Tags[tag] = ref
		}
	}
}

// ensureSpecTagGenerationsAreSet ensures that all spec tags have a generation set to either 0 or the
// current stream value.
func ensureSpecTagGenerationsAreSet(stream, oldStream *imageapi.ImageStream) {
	oldTags := map[string]imageapi.TagReference{}
	if oldStream != nil && oldStream.Spec.Tags != nil {
		oldTags = oldStream.Spec.Tags
	}

	// set the generation for any spec tags that have changed, are nil, or are zero
	for tag, ref := range stream.Spec.Tags {
		if oldRef, ok := oldTags[tag]; !ok || tagRefChanged(oldRef, ref, stream.Namespace) {
			ref.Generation = nil
		}

		if ref.Generation != nil && *ref.Generation != 0 {
			continue
		}
		ref.Generation = &stream.Generation
		stream.Spec.Tags[tag] = ref
	}
}

// updateObservedGenerationForStatusUpdate ensures every status item has a generation set.
func updateObservedGenerationForStatusUpdate(stream, oldStream *imageapi.ImageStream) {
	for tag, newer := range stream.Status.Tags {
		if len(newer.Items) == 0 || newer.Items[0].Generation != 0 {
			// generation is set, continue
			continue
		}

		older := oldStream.Status.Tags[tag]
		if len(older.Items) == 0 || !tagEventChanged(older.Items[0], newer.Items[0]) {
			// if the tag wasn't changed by the status update
			newer.Items[0].Generation = stream.Generation
			stream.Status.Tags[tag] = newer
			continue
		}

		spec, ok := stream.Spec.Tags[tag]
		if !ok || spec.Generation == nil {
			// if the spec tag has no generation
			newer.Items[0].Generation = stream.Generation
			stream.Status.Tags[tag] = newer
			continue
		}

		// set the status tag from the spec tag generation
		newer.Items[0].Generation = *spec.Generation
		stream.Status.Tags[tag] = newer
	}
}

type TagVerifier struct {
	subjectAccessReviewClient subjectaccessreview.Registry
}

func (v *TagVerifier) Verify(old, stream *imageapi.ImageStream, user user.Info) field.ErrorList {
	var errors field.ErrorList
	oldTags := map[string]imageapi.TagReference{}
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

		streamName, _, err := parseFromReference(stream, tagRef.From)
		fromPath := field.NewPath("spec", "tags").Key(tag).Child("from")
		if err != nil {
			errors = append(errors, field.Invalid(fromPath.Child("name"), tagRef.From.Name, "must be of the form <tag>, <repo>:<tag>, <id>, or <repo>@<id>"))
			continue
		}

		// Make sure this user can pull the specified image before allowing them to tag it into another imagestream
		subjectAccessReview := authorizationapi.AddUserToSAR(user, &authorizationapi.SubjectAccessReview{
			Action: authorizationapi.Action{
				Verb:         "get",
				Group:        imageapi.LegacyGroupName,
				Resource:     "imagestreams/layers",
				ResourceName: streamName,
			},
		})
		ctx := apirequest.WithNamespace(apirequest.WithUser(apirequest.NewContext(), user), tagRef.From.Namespace)
		glog.V(4).Infof("Performing SubjectAccessReview for user=%s, groups=%v to %s/%s", user.GetName(), user.GetGroups(), tagRef.From.Namespace, streamName)
		resp, err := v.subjectAccessReviewClient.CreateSubjectAccessReview(ctx, subjectAccessReview)
		if err != nil || resp == nil || (resp != nil && !resp.Allowed) {
			message := fmt.Sprintf("%s/%s", tagRef.From.Namespace, streamName)
			if resp != nil {
				message = message + fmt.Sprintf(": %q %q", resp.Reason, resp.EvaluationError)
			}
			if err != nil {
				message = message + fmt.Sprintf("- %v", err)
			}
			errors = append(errors, field.Forbidden(fromPath, message))
			continue
		}
	}
	return errors
}

// Canonicalize normalizes the object after validation.
func (Strategy) Canonicalize(obj runtime.Object) {
}

func (s Strategy) prepareForUpdate(obj, old runtime.Object, resetStatus bool) {
	oldStream := old.(*imageapi.ImageStream)
	stream := obj.(*imageapi.ImageStream)

	stream.Generation = oldStream.Generation
	if resetStatus {
		stream.Status = oldStream.Status
	}
	stream.Status.DockerImageRepository = s.dockerImageRepository(stream)

	// ensure that users cannot change spec tag generation to any value except 0
	updateSpecTagGenerationsForUpdate(stream, oldStream)

	// Any changes to the spec increment the generation number.
	//
	// TODO: Any changes to a part of the object that represents desired state (labels,
	// annotations etc) should also increment the generation.
	if !kapihelper.Semantic.DeepEqual(oldStream.Spec, stream.Spec) || stream.Generation == 0 {
		stream.Generation = oldStream.Generation + 1
	}

	// default spec tag generations afterwards (to avoid updating the generation for legacy objects)
	ensureSpecTagGenerationsAreSet(stream, oldStream)
}

func (s Strategy) PrepareForUpdate(ctx apirequest.Context, obj, old runtime.Object) {
	s.prepareForUpdate(obj, old, true)
}

// ValidateUpdate is the default update validation for an end user.
func (s Strategy) ValidateUpdate(ctx apirequest.Context, obj, old runtime.Object) field.ErrorList {
	stream := obj.(*imageapi.ImageStream)
	oldStream := old.(*imageapi.ImageStream)
	var errs field.ErrorList
	if err := s.validateTagsAndLimits(ctx, oldStream, stream); err != nil {
		errs = append(errs, field.InternalError(field.NewPath(""), err))
	}
	errs = append(errs, validation.ValidateImageStreamUpdate(stream, oldStream)...)
	return errs
}

// Decorate decorates stream.Status.DockerImageRepository using the logic from
// dockerImageRepository().
func (s Strategy) Decorate(obj runtime.Object) error {
	switch t := obj.(type) {
	case *imageapi.ImageStream:
		t.Status.DockerImageRepository = s.dockerImageRepository(t)
	case *imageapi.ImageStreamList:
		for i := range t.Items {
			is := &t.Items[i]
			is.Status.DockerImageRepository = s.dockerImageRepository(is)
		}
	default:
		return kerrors.NewBadRequest(fmt.Sprintf("not an ImageStream nor ImageStreamList: %v", obj))
	}
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

// Canonicalize normalizes the object after validation.
func (StatusStrategy) Canonicalize(obj runtime.Object) {
}

func (StatusStrategy) PrepareForUpdate(ctx apirequest.Context, obj, old runtime.Object) {
	oldStream := old.(*imageapi.ImageStream)
	stream := obj.(*imageapi.ImageStream)

	stream.Spec.Tags = oldStream.Spec.Tags
	stream.Spec.DockerImageRepository = oldStream.Spec.DockerImageRepository

	updateObservedGenerationForStatusUpdate(stream, oldStream)
}

func (s StatusStrategy) ValidateUpdate(ctx apirequest.Context, obj, old runtime.Object) field.ErrorList {
	newIS := obj.(*imageapi.ImageStream)
	errs := field.ErrorList{}

	ns, ok := apirequest.NamespaceFrom(ctx)
	if !ok {
		ns = newIS.Namespace
	}
	err := s.limitVerifier.VerifyLimits(ns, newIS)
	if err != nil {
		errs = append(errs, field.Forbidden(field.NewPath("imageStream"), err.Error()))
	}

	// TODO: merge valid fields after update
	errs = append(errs, validation.ValidateImageStreamStatusUpdate(newIS, old.(*imageapi.ImageStream))...)
	return errs
}

// GetAttrs returns labels and fields of a given object for filtering purposes
func GetAttrs(o runtime.Object) (labels.Set, fields.Set, bool, error) {
	obj, ok := o.(*imageapi.ImageStream)
	if !ok {
		return nil, nil, false, fmt.Errorf("not an ImageStream")
	}
	return labels.Set(obj.Labels), SelectableFields(obj), obj.Initializers != nil, nil
}

// Matcher returns a generic matcher for a given label and field selector.
func Matcher(label labels.Selector, field fields.Selector) kstorage.SelectionPredicate {
	return kstorage.SelectionPredicate{
		Label:    label,
		Field:    field,
		GetAttrs: GetAttrs,
	}
}

// SelectableFields returns a field set that can be used for filter selection
func SelectableFields(obj *imageapi.ImageStream) fields.Set {
	return imageapi.ImageStreamToSelectableFields(obj)
}

// InternalStrategy implements behavior for updating both the spec and status
// of an image stream
type InternalStrategy struct {
	Strategy
}

// NewInternalStrategy creates an update strategy around an existing stream
// strategy.
func NewInternalStrategy(strategy Strategy) InternalStrategy {
	return InternalStrategy{strategy}
}

// Canonicalize normalizes the object after validation.
func (InternalStrategy) Canonicalize(obj runtime.Object) {
}

func (s InternalStrategy) PrepareForCreate(ctx apirequest.Context, obj runtime.Object) {
	stream := obj.(*imageapi.ImageStream)

	stream.Status.DockerImageRepository = s.dockerImageRepository(stream)
	stream.Generation = 1
	for tag, ref := range stream.Spec.Tags {
		ref.Generation = &stream.Generation
		stream.Spec.Tags[tag] = ref
	}
}

func (s InternalStrategy) PrepareForUpdate(ctx apirequest.Context, obj, old runtime.Object) {
	s.prepareForUpdate(obj, old, false)
}
