package imagestreamimport

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang/glog"
	gocontext "golang.org/x/net/context"

	corev1 "k8s.io/api/core/v1"
	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/diff"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	authorizationapi "k8s.io/kubernetes/pkg/apis/authorization"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kapihelper "k8s.io/kubernetes/pkg/apis/core/helper"
	authorizationclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/authorization/internalversion"

	imageapiv1 "github.com/openshift/api/image/v1"
	imageclientv1 "github.com/openshift/client-go/image/clientset/versioned/typed/image/v1"
	authorizationutil "github.com/openshift/origin/pkg/authorization/util"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	"github.com/openshift/origin/pkg/image/apis/image/validation/whitelist"
	"github.com/openshift/origin/pkg/image/importer"
	"github.com/openshift/origin/pkg/image/importer/dockerv1client"
	"github.com/openshift/origin/pkg/image/registry/imagestream"
	"github.com/openshift/origin/pkg/image/registryclient"
	"github.com/openshift/origin/pkg/image/util"
	quotautil "github.com/openshift/origin/pkg/quota/util"
)

// ImporterFunc returns an instance of the importer that should be used per invocation.
type ImporterFunc func(r importer.RepositoryRetriever) importer.Interface

// ImporterDockerRegistryFunc returns an instance of a docker client that should be used per invocation of import,
// may be nil if no legacy import capability is required.
type ImporterDockerRegistryFunc func() dockerv1client.Client

// REST implements the RESTStorage interface for ImageStreamImport
type REST struct {
	importFn          ImporterFunc
	streams           imagestream.Registry
	internalStreams   rest.CreaterUpdater
	images            rest.Creater
	isV1Client        imageclientv1.ImageStreamsGetter
	transport         http.RoundTripper
	insecureTransport http.RoundTripper
	clientFn          ImporterDockerRegistryFunc
	strategy          *strategy
	sarClient         authorizationclient.SubjectAccessReviewInterface
}

var _ rest.Creater = &REST{}

// NewREST returns a REST storage implementation that handles importing images. The clientFn argument is optional
// if v1 Docker Registry importing is not required. Insecure transport is optional, and both transports should not
// include client certs unless you wish to allow the entire cluster to import using those certs.
func NewREST(importFn ImporterFunc, streams imagestream.Registry, internalStreams rest.CreaterUpdater,
	images rest.Creater,
	isV1Client imageclientv1.ImageStreamsGetter,
	transport, insecureTransport http.RoundTripper,
	clientFn ImporterDockerRegistryFunc,
	registryWhitelister whitelist.RegistryWhitelister,
	sarClient authorizationclient.SubjectAccessReviewInterface,
) *REST {
	return &REST{
		importFn:          importFn,
		streams:           streams,
		internalStreams:   internalStreams,
		images:            images,
		isV1Client:        isV1Client,
		transport:         transport,
		insecureTransport: insecureTransport,
		clientFn:          clientFn,
		strategy:          NewStrategy(registryWhitelister),
		sarClient:         sarClient,
	}
}

// New is only implemented to make REST implement RESTStorage
func (r *REST) New() runtime.Object {
	return &imageapi.ImageStreamImport{}
}

func (r *REST) Create(ctx apirequest.Context, obj runtime.Object, createValidation rest.ValidateObjectFunc, _ bool) (runtime.Object, error) {
	isi, ok := obj.(*imageapi.ImageStreamImport)
	if !ok {
		return nil, kapierrors.NewBadRequest(fmt.Sprintf("obj is not an ImageStreamImport: %#v", obj))
	}

	inputMeta := isi.ObjectMeta

	if err := rest.BeforeCreate(r.strategy, ctx, obj); err != nil {
		return nil, err
	}
	if err := createValidation(obj.DeepCopyObject()); err != nil {
		return nil, err
	}

	// Check if the user is allowed to create Images or ImageStreamMappings.
	// In case the user is allowed to create them, do not validate the ImageStreamImport
	// registry location against the registry whitelist, but instead allow to create any
	// image from any registry.
	user, ok := apirequest.UserFrom(ctx)
	if !ok {
		return nil, kapierrors.NewBadRequest("unable to get user from context")
	}
	createImageSAR := authorizationutil.AddUserToSAR(user, &authorizationapi.SubjectAccessReview{
		Spec: authorizationapi.SubjectAccessReviewSpec{
			ResourceAttributes: &authorizationapi.ResourceAttributes{
				Verb:     "create",
				Group:    imageapi.GroupName,
				Resource: "images",
			},
		},
	})
	isCreateImage, err := r.sarClient.Create(createImageSAR)
	if err != nil {
		return nil, err
	}

	createImageStreamMappingSAR := authorizationutil.AddUserToSAR(user, &authorizationapi.SubjectAccessReview{
		Spec: authorizationapi.SubjectAccessReviewSpec{
			ResourceAttributes: &authorizationapi.ResourceAttributes{
				Verb:     "create",
				Group:    imageapi.GroupName,
				Resource: "imagestreammapping",
			},
		},
	})
	isCreateImageStreamMapping, err := r.sarClient.Create(createImageStreamMappingSAR)
	if err != nil {
		return nil, err
	}

	if !isCreateImage.Status.Allowed && !isCreateImageStreamMapping.Status.Allowed {
		if errs := r.strategy.ValidateAllowedRegistries(isi); len(errs) != 0 {
			return nil, kapierrors.NewInvalid(imageapi.Kind("ImageStreamImport"), isi.Name, errs)
		}
	}

	namespace, ok := apirequest.NamespaceFrom(ctx)
	if !ok {
		return nil, kapierrors.NewBadRequest("a namespace must be specified to import images")
	}

	if r.clientFn != nil {
		if client := r.clientFn(); client != nil {
			ctx = apirequest.WithValue(ctx, importer.ContextKeyV1RegistryClient, client)
		}
	}

	create := false
	stream, err := r.streams.GetImageStream(ctx, isi.Name, &metav1.GetOptions{})
	if err != nil {
		if !kapierrors.IsNotFound(err) {
			return nil, err
		}
		// consistency check, stream must exist
		if len(inputMeta.ResourceVersion) > 0 || len(inputMeta.UID) > 0 {
			return nil, err
		}
		create = true
		stream = &imageapi.ImageStream{
			ObjectMeta: metav1.ObjectMeta{
				Name:       isi.Name,
				Namespace:  namespace,
				Generation: 0,
			},
		}
	} else {
		if len(inputMeta.ResourceVersion) > 0 && inputMeta.ResourceVersion != stream.ResourceVersion {
			glog.V(4).Infof("DEBUG: mismatch between requested ResourceVersion %s and located ResourceVersion %s", inputMeta.ResourceVersion, stream.ResourceVersion)
			return nil, kapierrors.NewConflict(imageapi.Resource("imagestream"), inputMeta.Name, fmt.Errorf("the image stream was updated from %q to %q", inputMeta.ResourceVersion, stream.ResourceVersion))
		}
		if len(inputMeta.UID) > 0 && inputMeta.UID != stream.UID {
			glog.V(4).Infof("DEBUG: mismatch between requested UID %s and located UID %s", inputMeta.UID, stream.UID)
			return nil, kapierrors.NewNotFound(imageapi.Resource("imagestream"), inputMeta.Name)
		}
	}

	// only load secrets if we need them
	credentials := importer.NewLazyCredentialsForSecrets(func() ([]corev1.Secret, error) {
		secrets, err := r.isV1Client.ImageStreams(namespace).Secrets(isi.Name, metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
		return secrets.Items, nil
	})
	importCtx := registryclient.NewContext(r.transport, r.insecureTransport).WithCredentials(credentials)
	imports := r.importFn(importCtx)
	if err := imports.Import(ctx.(gocontext.Context), isi, stream); err != nil {
		return nil, kapierrors.NewInternalError(err)
	}

	// check imported images status. If we get authentication error (401), try import same image without authentication.
	// Docker registry gives 401 on public images if you have wrong secret in your secret list.
	// this block was introduced by PR #18012
	// TODO: remove this blocks when smarter auth client gets done with retries
	var imageStatus []metav1.Status
	importFailed := false
	for _, image := range isi.Status.Images {
		//cache all imports status
		imageStatus = append(imageStatus, image.Status)
		if image.Status.Reason == metav1.StatusReasonUnauthorized && strings.Contains(strings.ToLower(image.Status.Message), "username or password") {
			importFailed = true
		}
	}
	// try import IS without auth if it failed before
	if importFailed {
		importCtx := registryclient.NewContext(r.transport, r.insecureTransport).WithCredentials(nil)
		imports := r.importFn(importCtx)
		if err := imports.Import(ctx.(gocontext.Context), isi, stream); err != nil {
			return nil, kapierrors.NewInternalError(err)
		}
	}
	//cycle through status and set old messages so not to confuse users
	for key, image := range isi.Status.Images {
		if image.Status.Reason == metav1.StatusReasonUnauthorized {
			isi.Status.Images[key].Status = imageStatus[key]
		}
	}

	// if we encountered an error loading credentials and any images could not be retrieved with an access
	// related error, modify the message.
	// TODO: set a status cause
	if err := credentials.Err(); err != nil {
		for i, image := range isi.Status.Images {
			switch image.Status.Reason {
			case metav1.StatusReasonUnauthorized, metav1.StatusReasonForbidden:
				isi.Status.Images[i].Status.Message = fmt.Sprintf("Unable to load secrets for this image: %v; (%s)", err, image.Status.Message)
			}
		}
		if r := isi.Status.Repository; r != nil {
			switch r.Status.Reason {
			case metav1.StatusReasonUnauthorized, metav1.StatusReasonForbidden:
				r.Status.Message = fmt.Sprintf("Unable to load secrets for this repository: %v; (%s)", err, r.Status.Message)
			}
		}
	}

	// TODO: perform the transformation of the image stream and return it with the ISI if import is false
	//   so that clients can see what the resulting object would look like.
	if !isi.Spec.Import {
		clearManifests(isi)
		return isi, nil
	}

	if stream.Annotations == nil {
		stream.Annotations = make(map[string]string)
	}
	now := metav1.Now()
	_, hasAnnotation := stream.Annotations[imageapi.DockerImageRepositoryCheckAnnotation]
	nextGeneration := stream.Generation + 1

	original := stream.DeepCopy()

	// walk the retrieved images, ensuring each one exists in etcd
	importedImages := make(map[string]error)
	updatedImages := make(map[string]*imageapi.Image)

	if spec := isi.Spec.Repository; spec != nil {
		for i, status := range isi.Status.Repository.Images {
			if checkImportFailure(status, stream, status.Tag, nextGeneration, now) {
				continue
			}

			image := status.Image
			ref, err := imageapi.ParseDockerImageReference(image.DockerImageReference)
			if err != nil {
				utilruntime.HandleError(fmt.Errorf("unable to parse image reference during import: %v", err))
				continue
			}
			from, err := imageapi.ParseDockerImageReference(spec.From.Name)
			if err != nil {
				utilruntime.HandleError(fmt.Errorf("unable to parse from reference during import: %v", err))
				continue
			}

			tag := ref.Tag
			if len(status.Tag) > 0 {
				tag = status.Tag
			}
			// we've imported a set of tags, ensure spec tag will point to this for later imports
			from.ID, from.Tag = "", tag

			if updated, ok := r.importSuccessful(ctx, image, stream, tag, from.Exact(), nextGeneration,
				now, spec.ImportPolicy, spec.ReferencePolicy, importedImages, updatedImages); ok {
				isi.Status.Repository.Images[i].Image = updated
			}
		}
	}

	for i, spec := range isi.Spec.Images {
		if spec.To == nil {
			continue
		}
		tag := spec.To.Name

		// record a failure condition
		status := isi.Status.Images[i]
		if checkImportFailure(status, stream, tag, nextGeneration, now) {
			// ensure that we have a spec tag set
			ensureSpecTag(stream, tag, spec.From.Name, spec.ImportPolicy, spec.ReferencePolicy, false)
			continue
		}

		// record success
		image := status.Image
		if updated, ok := r.importSuccessful(ctx, image, stream, tag, spec.From.Name, nextGeneration,
			now, spec.ImportPolicy, spec.ReferencePolicy, importedImages, updatedImages); ok {
			isi.Status.Images[i].Image = updated
		}
	}

	// TODO: should we allow partial failure?
	for _, err := range importedImages {
		if err != nil {
			return nil, err
		}
	}

	clearManifests(isi)

	// ensure defaulting is applied by round trip converting
	// TODO: convert to using versioned types.
	external, err := legacyscheme.Scheme.ConvertToVersion(stream, imageapiv1.SchemeGroupVersion)
	if err != nil {
		return nil, err
	}
	legacyscheme.Scheme.Default(external)
	internal, err := legacyscheme.Scheme.ConvertToVersion(external, imageapi.SchemeGroupVersion)
	if err != nil {
		return nil, err
	}
	stream = internal.(*imageapi.ImageStream)

	// if and only if we have changes between the original and the imported stream, trigger
	// an import
	hasChanges := !kapihelper.Semantic.DeepEqual(original, stream)
	if create {
		stream.Annotations[imageapi.DockerImageRepositoryCheckAnnotation] = now.UTC().Format(time.RFC3339)
		glog.V(4).Infof("create new stream: %#v", stream)
		obj, err = r.internalStreams.Create(ctx, stream, rest.ValidateAllObjectFunc, false)
	} else {
		if hasAnnotation && !hasChanges {
			glog.V(4).Infof("stream did not change: %#v", stream)
			obj, err = original, nil
		} else {
			if glog.V(4) {
				glog.V(4).Infof("updating stream %s", diff.ObjectDiff(original, stream))
			}
			stream.Annotations[imageapi.DockerImageRepositoryCheckAnnotation] = now.UTC().Format(time.RFC3339)
			obj, _, err = r.internalStreams.Update(ctx, stream.Name, rest.DefaultUpdatedObjectInfo(stream), rest.ValidateAllObjectFunc, rest.ValidateAllObjectUpdateFunc)
		}
	}

	if err != nil {
		// if we have am admission limit error then record the conditions on the original stream.  Quota errors
		// will be recorded by the importer.
		if quotautil.IsErrorLimitExceeded(err) {
			originalStream := original
			recordLimitExceededStatus(originalStream, stream, err, now, nextGeneration)
			var limitErr error
			obj, _, limitErr = r.internalStreams.Update(ctx, stream.Name, rest.DefaultUpdatedObjectInfo(originalStream), rest.ValidateAllObjectFunc, rest.ValidateAllObjectUpdateFunc)
			if limitErr != nil {
				utilruntime.HandleError(fmt.Errorf("failed to record limit exceeded status in image stream %s/%s: %v", stream.Namespace, stream.Name, limitErr))
			}
		}

		return nil, err
	}
	isi.Status.Import = obj.(*imageapi.ImageStream)
	return isi, nil
}

// recordLimitExceededStatus adds the limit err to any new tag.
func recordLimitExceededStatus(originalStream *imageapi.ImageStream, newStream *imageapi.ImageStream, err error, now metav1.Time, nextGeneration int64) {
	for tag := range newStream.Status.Tags {
		if _, ok := originalStream.Status.Tags[tag]; !ok {
			imageapi.SetTagConditions(originalStream, tag, newImportFailedCondition(err, nextGeneration, now))
		}
	}
}

func checkImportFailure(status imageapi.ImageImportStatus, stream *imageapi.ImageStream, tag string, nextGeneration int64, now metav1.Time) bool {
	if status.Image != nil && status.Status.Status == metav1.StatusSuccess {
		return false
	}
	message := status.Status.Message
	if len(message) == 0 {
		message = "unknown error prevented import"
	}
	condition := imageapi.TagEventCondition{
		Type:       imageapi.ImportSuccess,
		Status:     kapi.ConditionFalse,
		Message:    message,
		Reason:     string(status.Status.Reason),
		Generation: nextGeneration,

		LastTransitionTime: now,
	}

	if tag == "" {
		if len(status.Tag) > 0 {
			tag = status.Tag
		} else if status.Image != nil {
			if ref, err := imageapi.ParseDockerImageReference(status.Image.DockerImageReference); err == nil {
				tag = ref.Tag
			}
		}
	}

	if !imageapi.HasTagCondition(stream, tag, condition) {
		imageapi.SetTagConditions(stream, tag, condition)
		if tagRef, ok := stream.Spec.Tags[tag]; ok {
			zero := int64(0)
			tagRef.Generation = &zero
			stream.Spec.Tags[tag] = tagRef
		}
	}
	return true
}

// ensureSpecTag guarantees that the spec tag is set with the provided from, importPolicy and referencePolicy.
// If reset is passed, the tag will be overwritten.
func ensureSpecTag(stream *imageapi.ImageStream, tag, from string, importPolicy imageapi.TagImportPolicy,
	referencePolicy imageapi.TagReferencePolicy, reset bool) imageapi.TagReference {
	if stream.Spec.Tags == nil {
		stream.Spec.Tags = make(map[string]imageapi.TagReference)
	}
	specTag, ok := stream.Spec.Tags[tag]
	if ok && !reset {
		return specTag
	}
	specTag.From = &kapi.ObjectReference{
		Kind: "DockerImage",
		Name: from,
	}

	zero := int64(0)
	specTag.Generation = &zero
	specTag.ImportPolicy = importPolicy
	specTag.ReferencePolicy = referencePolicy
	stream.Spec.Tags[tag] = specTag
	return specTag
}

// importSuccessful records a successful import into an image stream, setting the spec tag, status tag or conditions, and ensuring
// the image is created in etcd. Images are cached so they are not created multiple times in a row (when multiple tags point to the
// same image), and a failure to persist the image will be summarized before we update the stream. If an image was imported by this
// operation, it *replaces* the imported image (from the remote repository) with the updated image.
func (r *REST) importSuccessful(
	ctx apirequest.Context,
	image *imageapi.Image, stream *imageapi.ImageStream, tag string, from string, nextGeneration int64, now metav1.Time,
	importPolicy imageapi.TagImportPolicy, referencePolicy imageapi.TagReferencePolicy,
	importedImages map[string]error, updatedImages map[string]*imageapi.Image,
) (*imageapi.Image, bool) {
	r.strategy.PrepareImageForCreate(image)

	pullSpec, _ := imageapi.MostAccuratePullSpec(image.DockerImageReference, image.Name, "")
	tagEvent := imageapi.TagEvent{
		Created:              now,
		DockerImageReference: pullSpec,
		Image:                image.Name,
		Generation:           nextGeneration,
	}

	if stream.Spec.Tags == nil {
		stream.Spec.Tags = make(map[string]imageapi.TagReference)
	}

	// ensure the spec and status tag match the imported image
	changed := imageapi.DifferentTagEvent(stream, tag, tagEvent) || imageapi.DifferentTagGeneration(stream, tag)
	specTag, ok := stream.Spec.Tags[tag]
	if changed || !ok {
		specTag = ensureSpecTag(stream, tag, from, importPolicy, referencePolicy, true)
		imageapi.AddTagEventToImageStream(stream, tag, tagEvent)
	}
	// always reset the import policy
	specTag.ImportPolicy = importPolicy
	stream.Spec.Tags[tag] = specTag

	// import or reuse the image, and ensure tag conditions are set
	importErr, alreadyImported := importedImages[image.Name]
	if importErr != nil {
		imageapi.SetTagConditions(stream, tag, newImportFailedCondition(importErr, nextGeneration, now))
	} else {
		imageapi.SetTagConditions(stream, tag)
	}

	// create the image if it does not exist, otherwise cache the updated status from the store for use by other tags
	if alreadyImported {
		if updatedImage, ok := updatedImages[image.Name]; ok {
			return updatedImage, true
		}
		return nil, false
	}

	updated, err := r.images.Create(ctx, image, rest.ValidateAllObjectFunc, false)
	switch {
	case kapierrors.IsAlreadyExists(err):
		if err := util.ImageWithMetadata(image); err != nil {
			glog.V(4).Infof("Unable to update image metadata during image import when image already exists %q: %v", image.Name, err)
		}
		updated = image
		fallthrough
	case err == nil:
		updatedImage := updated.(*imageapi.Image)
		updatedImages[image.Name] = updatedImage
		//isi.Status.Repository.Images[i].Image = updatedImage
		importedImages[image.Name] = nil
		return updatedImage, true
	default:
		importedImages[image.Name] = err
	}
	return nil, false
}

// clearManifests unsets the manifest for each object that does not request it
func clearManifests(isi *imageapi.ImageStreamImport) {
	for i := range isi.Status.Images {
		if !isi.Spec.Images[i].IncludeManifest {
			if isi.Status.Images[i].Image != nil {
				isi.Status.Images[i].Image.DockerImageManifest = ""
				isi.Status.Images[i].Image.DockerImageConfig = ""
			}
		}
	}
	if isi.Spec.Repository != nil && !isi.Spec.Repository.IncludeManifest {
		for i := range isi.Status.Repository.Images {
			if isi.Status.Repository.Images[i].Image != nil {
				isi.Status.Repository.Images[i].Image.DockerImageManifest = ""
				isi.Status.Repository.Images[i].Image.DockerImageConfig = ""
			}
		}
	}
}

func newImportFailedCondition(err error, gen int64, now metav1.Time) imageapi.TagEventCondition {
	c := imageapi.TagEventCondition{
		Type:       imageapi.ImportSuccess,
		Status:     kapi.ConditionFalse,
		Message:    err.Error(),
		Generation: gen,

		LastTransitionTime: now,
	}
	if status, ok := err.(kapierrors.APIStatus); ok {
		s := status.Status()
		c.Reason, c.Message = string(s.Reason), s.Message
	}
	return c
}
