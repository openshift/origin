package imagestreamimport

import (
	"fmt"
	"net/http"
	"time"

	"github.com/golang/glog"
	gocontext "golang.org/x/net/context"

	kapi "k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/rest"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/diff"
	utilruntime "k8s.io/kubernetes/pkg/util/runtime"
	"k8s.io/kubernetes/pkg/util/validation/field"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/dockerregistry"
	"github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/image/importer"
	"github.com/openshift/origin/pkg/image/registry/imagestream"
	quotautil "github.com/openshift/origin/pkg/quota/util"
)

// ImporterFunc returns an instance of the importer that should be used per invocation.
type ImporterFunc func(r importer.RepositoryRetriever) importer.Interface

// ImporterDockerRegistryFunc returns an instance of a docker client that should be used per invocation of import,
// may be nil if no legacy import capability is required.
type ImporterDockerRegistryFunc func() dockerregistry.Client

// REST implements the RESTStorage interface for ImageStreamImport
type REST struct {
	importFn          ImporterFunc
	streams           imagestream.Registry
	internalStreams   rest.CreaterUpdater
	images            rest.Creater
	secrets           client.ImageStreamSecretsNamespacer
	transport         http.RoundTripper
	insecureTransport http.RoundTripper
	clientFn          ImporterDockerRegistryFunc
}

// NewREST returns a REST storage implementation that handles importing images. The clientFn argument is optional
// if v1 Docker Registry importing is not required. Insecure transport is optional, and both transports should not
// include client certs unless you wish to allow the entire cluster to import using those certs.
func NewREST(importFn ImporterFunc, streams imagestream.Registry, internalStreams rest.CreaterUpdater,
	images rest.Creater, secrets client.ImageStreamSecretsNamespacer,
	transport, insecureTransport http.RoundTripper,
	clientFn ImporterDockerRegistryFunc,
) *REST {
	return &REST{
		importFn:          importFn,
		streams:           streams,
		internalStreams:   internalStreams,
		images:            images,
		secrets:           secrets,
		transport:         transport,
		insecureTransport: insecureTransport,
		clientFn:          clientFn,
	}
}

// New is only implemented to make REST implement RESTStorage
func (r *REST) New() runtime.Object {
	return &api.ImageStreamImport{}
}

func (r *REST) Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error) {
	isi, ok := obj.(*api.ImageStreamImport)
	if !ok {
		return nil, kapierrors.NewBadRequest(fmt.Sprintf("obj is not an ImageStreamImport: %#v", obj))
	}

	inputMeta := isi.ObjectMeta

	if err := rest.BeforeCreate(Strategy, ctx, obj); err != nil {
		return nil, err
	}

	namespace, ok := kapi.NamespaceFrom(ctx)
	if !ok {
		return nil, kapierrors.NewBadRequest("a namespace must be specified to import images")
	}

	if r.clientFn != nil {
		if client := r.clientFn(); client != nil {
			ctx = kapi.WithValue(ctx, importer.ContextKeyV1RegistryClient, client)
		}
	}

	// only load secrets if we need them
	credentials := importer.NewLazyCredentialsForSecrets(func() ([]kapi.Secret, error) {
		secrets, err := r.secrets.ImageStreamSecrets(namespace).Secrets(isi.Name, kapi.ListOptions{})
		if err != nil {
			return nil, err
		}
		return secrets.Items, nil
	})
	importCtx := importer.NewContext(r.transport, r.insecureTransport).WithCredentials(credentials)
	imports := r.importFn(importCtx)
	if err := imports.Import(ctx.(gocontext.Context), isi); err != nil {
		return nil, kapierrors.NewInternalError(err)
	}

	// if we encountered an error loading credentials and any images could not be retrieved with an access
	// related error, modify the message.
	// TODO: set a status cause
	if err := credentials.Err(); err != nil {
		for i, image := range isi.Status.Images {
			switch image.Status.Reason {
			case unversioned.StatusReasonUnauthorized, unversioned.StatusReasonForbidden:
				isi.Status.Images[i].Status.Message = fmt.Sprintf("Unable to load secrets for this image: %v; (%s)", err, image.Status.Message)
			}
		}
		if r := isi.Status.Repository; r != nil {
			switch r.Status.Reason {
			case unversioned.StatusReasonUnauthorized, unversioned.StatusReasonForbidden:
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

	create := false
	stream, err := r.streams.GetImageStream(ctx, isi.Name)
	if err != nil {
		if !kapierrors.IsNotFound(err) {
			return nil, err
		}
		// consistency check, stream must exist
		if len(inputMeta.ResourceVersion) > 0 || len(inputMeta.UID) > 0 {
			return nil, err
		}
		create = true
		stream = &api.ImageStream{
			ObjectMeta: kapi.ObjectMeta{
				Name:       isi.Name,
				Namespace:  namespace,
				Generation: 0,
			},
		}
	} else {
		if len(inputMeta.ResourceVersion) > 0 && inputMeta.ResourceVersion != stream.ResourceVersion {
			glog.V(4).Infof("DEBUG: mismatch between requested ResourceVersion %s and located ResourceVersion %s", inputMeta.ResourceVersion, stream.ResourceVersion)
			return nil, kapierrors.NewConflict(api.Resource("imagestream"), inputMeta.Name, fmt.Errorf("the image stream was updated from %q to %q", inputMeta.ResourceVersion, stream.ResourceVersion))
		}
		if len(inputMeta.UID) > 0 && inputMeta.UID != stream.UID {
			glog.V(4).Infof("DEBUG: mismatch between requested UID %s and located UID %s", inputMeta.UID, stream.UID)
			return nil, kapierrors.NewNotFound(api.Resource("imagestream"), inputMeta.Name)
		}
	}

	if stream.Annotations == nil {
		stream.Annotations = make(map[string]string)
	}
	now := unversioned.Now()
	_, hasAnnotation := stream.Annotations[api.DockerImageRepositoryCheckAnnotation]
	nextGeneration := stream.Generation + 1

	original, err := kapi.Scheme.DeepCopy(stream)
	if err != nil {
		return nil, err
	}

	// walk the retrieved images, ensuring each one exists in etcd
	importedImages := make(map[string]error)
	updatedImages := make(map[string]*api.Image)

	if spec := isi.Spec.Repository; spec != nil {
		for i, status := range isi.Status.Repository.Images {
			if checkImportFailure(status, stream, status.Tag, nextGeneration, now) {
				continue
			}

			image := status.Image
			ref, err := api.ParseDockerImageReference(image.DockerImageReference)
			if err != nil {
				utilruntime.HandleError(fmt.Errorf("unable to parse image reference during import: %v", err))
				continue
			}
			from, err := api.ParseDockerImageReference(spec.From.Name)
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

			if updated, ok := r.importSuccessful(ctx, image, stream, tag, from.Exact(), nextGeneration, now, spec.ImportPolicy, importedImages, updatedImages); ok {
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
			ensureSpecTag(stream, tag, spec.From.Name, spec.ImportPolicy, false)
			continue
		}

		// record success
		image := status.Image
		if updated, ok := r.importSuccessful(ctx, image, stream, tag, spec.From.Name, nextGeneration, now, spec.ImportPolicy, importedImages, updatedImages); ok {
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

	hasChanges := !kapi.Semantic.DeepEqual(original, stream)
	if create {
		stream.Annotations[api.DockerImageRepositoryCheckAnnotation] = now.UTC().Format(time.RFC3339)
		glog.V(4).Infof("create new stream: %#v", stream)
		obj, err = r.internalStreams.Create(ctx, stream)
	} else {
		if hasAnnotation && !hasChanges {
			glog.V(4).Infof("stream did not change: %#v", stream)
			obj, err = original.(*api.ImageStream), nil
		} else {
			if glog.V(4) {
				glog.V(4).Infof("updating stream %s", diff.ObjectDiff(original, stream))
			}
			stream.Annotations[api.DockerImageRepositoryCheckAnnotation] = now.UTC().Format(time.RFC3339)
			obj, _, err = r.internalStreams.Update(ctx, stream.Name, rest.DefaultUpdatedObjectInfo(stream, kapi.Scheme))
		}
	}

	if err != nil {
		// if we have am admission limit error then record the conditions on the original stream.  Quota errors
		// will be recorded by the importer.
		if quotautil.IsErrorLimitExceeded(err) {
			originalStream := original.(*api.ImageStream)
			recordLimitExceededStatus(originalStream, stream, err, now, nextGeneration)
			var limitErr error
			obj, _, limitErr = r.internalStreams.Update(ctx, stream.Name, rest.DefaultUpdatedObjectInfo(originalStream, kapi.Scheme))
			if limitErr != nil {
				utilruntime.HandleError(fmt.Errorf("failed to record limit exceeded status in image stream %s/%s: %v", stream.Namespace, stream.Name, limitErr))
			}
		}

		return nil, err
	}
	isi.Status.Import = obj.(*api.ImageStream)
	return isi, nil
}

// recordLimitExceededStatus adds the limit err to any new tag.
func recordLimitExceededStatus(originalStream *api.ImageStream, newStream *api.ImageStream, err error, now unversioned.Time, nextGeneration int64) {
	for tag := range newStream.Status.Tags {
		if _, ok := originalStream.Status.Tags[tag]; !ok {
			api.SetTagConditions(originalStream, tag, newImportFailedCondition(err, nextGeneration, now))
		}
	}
}

func checkImportFailure(status api.ImageImportStatus, stream *api.ImageStream, tag string, nextGeneration int64, now unversioned.Time) bool {
	if status.Image != nil && status.Status.Status == unversioned.StatusSuccess {
		return false
	}
	message := status.Status.Message
	if len(message) == 0 {
		message = "unknown error prevented import"
	}
	condition := api.TagEventCondition{
		Type:       api.ImportSuccess,
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
			if ref, err := api.ParseDockerImageReference(status.Image.DockerImageReference); err == nil {
				tag = ref.Tag
			}
		}
	}

	if !api.HasTagCondition(stream, tag, condition) {
		api.SetTagConditions(stream, tag, condition)
		if tagRef, ok := stream.Spec.Tags[tag]; ok {
			zero := int64(0)
			tagRef.Generation = &zero
			stream.Spec.Tags[tag] = tagRef
		}
	}
	return true
}

// ensureSpecTag guarantees that the spec tag is set with the provided from and importPolicy. If reset is passed,
// the tag will be overwritten.
func ensureSpecTag(stream *api.ImageStream, tag, from string, importPolicy api.TagImportPolicy, reset bool) api.TagReference {
	if stream.Spec.Tags == nil {
		stream.Spec.Tags = make(map[string]api.TagReference)
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
	stream.Spec.Tags[tag] = specTag
	return specTag
}

// importSuccessful records a successful import into an image stream, setting the spec tag, status tag or conditions, and ensuring
// the image is created in etcd. Images are cached so they are not created multiple times in a row (when multiple tags point to the
// same image), and a failure to persist the image will be summarized before we update the stream. If an image was imported by this
// operation, it *replaces* the imported image (from the remote repository) with the updated image.
func (r *REST) importSuccessful(
	ctx kapi.Context,
	image *api.Image, stream *api.ImageStream, tag string, from string, nextGeneration int64, now unversioned.Time, importPolicy api.TagImportPolicy,
	importedImages map[string]error, updatedImages map[string]*api.Image,
) (*api.Image, bool) {
	Strategy.PrepareImageForCreate(image)

	pullSpec, _ := api.MostAccuratePullSpec(image.DockerImageReference, image.Name, "")
	tagEvent := api.TagEvent{
		Created:              now,
		DockerImageReference: pullSpec,
		Image:                image.Name,
		Generation:           nextGeneration,
	}

	if stream.Spec.Tags == nil {
		stream.Spec.Tags = make(map[string]api.TagReference)
	}

	// ensure the spec and status tag match the imported image
	changed := api.DifferentTagEvent(stream, tag, tagEvent) || api.DifferentTagGeneration(stream, tag)
	specTag, ok := stream.Spec.Tags[tag]
	if changed || !ok {
		specTag = ensureSpecTag(stream, tag, from, importPolicy, true)
		api.AddTagEventToImageStream(stream, tag, tagEvent)
	}
	// always reset the import policy
	specTag.ImportPolicy = importPolicy
	stream.Spec.Tags[tag] = specTag

	// import or reuse the image, and ensure tag conditions are set
	importErr, alreadyImported := importedImages[image.Name]
	if importErr != nil {
		api.SetTagConditions(stream, tag, newImportFailedCondition(importErr, nextGeneration, now))
	} else {
		api.SetTagConditions(stream, tag)
	}

	// create the image if it does not exist, otherwise cache the updated status from the store for use by other tags
	if alreadyImported {
		if updatedImage, ok := updatedImages[image.Name]; ok {
			return updatedImage, true
		}
		return nil, false
	}

	updated, err := r.images.Create(ctx, image)
	switch {
	case kapierrors.IsAlreadyExists(err):
		if err := api.ImageWithMetadata(image); err != nil {
			glog.V(4).Infof("Unable to update image metadata during image import when image already exists %q: err", image.Name, err)
		}
		updated = image
		fallthrough
	case err == nil:
		updatedImage := updated.(*api.Image)
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
func clearManifests(isi *api.ImageStreamImport) {
	for i := range isi.Status.Images {
		if !isi.Spec.Images[i].IncludeManifest {
			if isi.Status.Images[i].Image != nil {
				isi.Status.Images[i].Image.DockerImageManifest = ""
			}
		}
	}
	if isi.Spec.Repository != nil && !isi.Spec.Repository.IncludeManifest {
		for i := range isi.Status.Repository.Images {
			if isi.Status.Repository.Images[i].Image != nil {
				isi.Status.Repository.Images[i].Image.DockerImageManifest = ""
			}
		}
	}
}

func newImportFailedCondition(err error, gen int64, now unversioned.Time) api.TagEventCondition {
	c := api.TagEventCondition{
		Type:       api.ImportSuccess,
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

func invalidStatus(kind, position string, errs ...*field.Error) unversioned.Status {
	return kapierrors.NewInvalid(api.Kind(kind), position, errs).ErrStatus
}
