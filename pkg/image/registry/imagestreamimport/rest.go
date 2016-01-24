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
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/dockerregistry"
	"github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/image/importer"
	"github.com/openshift/origin/pkg/image/registry/imagestream"
)

// ImporterFunc returns an instance of the importer that should be used per invocation.
type ImporterFunc func(r importer.RepositoryRetriever) importer.Interface

// ImporterDockerRegistryFunc returns an instance of a docker client that should be used per invocation of import,
// may be nil if no legacy import capability is required.
type ImporterDockerRegistryFunc func() dockerregistry.Client

// REST implements the RESTStorage interface for ImageStreamImport
type REST struct {
	importFn        ImporterFunc
	streams         imagestream.Registry
	internalStreams rest.CreaterUpdater
	images          rest.Creater
	secrets         client.ImageStreamSecretsNamespacer
	transport       http.RoundTripper
	clientFn        ImporterDockerRegistryFunc
}

// NewREST returns a REST storage implementation that handles importing images. The clientFn argument is optional
// if v1 Docker Registry importing is not required
func NewREST(importFn ImporterFunc, streams imagestream.Registry, internalStreams rest.CreaterUpdater,
	images rest.Creater, secrets client.ImageStreamSecretsNamespacer, clientFn ImporterDockerRegistryFunc) *REST {
	rt, err := kclient.TransportFor(&kclient.Config{})
	// TODO: will be refactored upstream, or take this as input?
	if err != nil {
		panic(err)
	}
	return &REST{
		importFn:        importFn,
		streams:         streams,
		internalStreams: internalStreams,
		images:          images,
		secrets:         secrets,
		transport:       rt,
		clientFn:        clientFn,
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

	secrets, err := r.secrets.ImageStreamSecrets(namespace).Secrets(isi.Name, kapi.ListOptions{})
	if err != nil {
		util.HandleError(fmt.Errorf("unable to load secrets for namespace %q: %v", namespace, err))
		secrets = &kapi.SecretList{}
	}

	if r.clientFn != nil {
		if client := r.clientFn(); client != nil {
			ctx = kapi.WithValue(ctx, importer.ContextKeyV1RegistryClient, client)
		}
	}
	credentials := importer.NewCredentialsForSecrets(secrets.Items)
	importCtx := importer.NewContext(r.transport).WithCredentials(credentials)

	imports := r.importFn(importCtx)
	if err := imports.Import(ctx.(gocontext.Context), isi); err != nil {
		return nil, kapierrors.NewInternalError(err)
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
			glog.V(4).Infof("DEBUG: mismatch between requested UID %s and located UID %s", inputMeta.UID, stream.UID)
			return nil, kapierrors.NewConflict("imageStream", inputMeta.Name, fmt.Errorf("the image stream was updated from %q to %q", inputMeta.ResourceVersion, stream.ResourceVersion))
		}
		if len(inputMeta.UID) > 0 && inputMeta.UID != stream.UID {
			glog.V(4).Infof("DEBUG: mismatch between requested UID %s and located UID %s", inputMeta.UID, stream.UID)
			return nil, kapierrors.NewNotFound("imageStream", inputMeta.Name)
		}
	}

	if stream.Annotations == nil {
		stream.Annotations = make(map[string]string)
	}
	now := unversioned.Now()
	stream.Annotations[api.DockerImageRepositoryCheckAnnotation] = now.UTC().Format(time.RFC3339)
	gen := stream.Generation + 1
	zero := int64(0)

	importedImages := make(map[string]error)
	updatedImages := make(map[string]*api.Image)
	if spec := isi.Spec.Repository; spec != nil {
		for i, imageStatus := range isi.Status.Repository.Images {
			image := imageStatus.Image
			if image == nil {
				continue
			}

			// update the spec tag
			ref, err := api.ParseDockerImageReference(image.DockerImageReference)
			if err != nil {
				// ???
				continue
			}
			tag := ref.Tag
			if len(imageStatus.Tag) > 0 {
				tag = imageStatus.Tag
			}
			if _, ok := stream.Spec.Tags[tag]; !ok {
				if stream.Spec.Tags == nil {
					stream.Spec.Tags = make(map[string]api.TagReference)
				}
				stream.Spec.Tags[tag] = api.TagReference{
					From: &kapi.ObjectReference{
						Kind: "DockerImage",
						Name: image.DockerImageReference,
					},
					Generation:   &gen,
					ImportPolicy: api.TagImportPolicy{Insecure: spec.ImportPolicy.Insecure},
				}
			}

			// import or reuse the image
			importErr, imported := importedImages[image.Name]
			if importErr != nil {
				api.SetTagConditions(stream, tag, newImportFailedCondition(err, gen, now))
			}

			pullSpec, _ := api.MostAccuratePullSpec(image.DockerImageReference, image.Name, "")
			api.AddTagEventToImageStream(stream, tag, api.TagEvent{
				Created:              now,
				DockerImageReference: pullSpec,
				Image:                image.Name,
				Generation:           gen,
			})

			if imported {
				if updatedImage, ok := updatedImages[image.Name]; ok {
					isi.Status.Repository.Images[i].Image = updatedImage
				}
				continue
			}

			// establish the image into the store
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
				isi.Status.Repository.Images[i].Image = updatedImage
				importedImages[image.Name] = nil
			default:
				importedImages[image.Name] = err
			}
		}
	}

	for i, spec := range isi.Spec.Images {
		if spec.To == nil {
			continue
		}
		tag := spec.To.Name

		if stream.Spec.Tags == nil {
			stream.Spec.Tags = make(map[string]api.TagReference)
		}
		specTag := stream.Spec.Tags[tag]
		from := spec.From
		specTag.From = &from
		specTag.Generation = &zero
		specTag.ImportPolicy.Insecure = spec.ImportPolicy.Insecure
		stream.Spec.Tags[tag] = specTag

		status := isi.Status.Images[i]
		if status.Image == nil || status.Status.Status == unversioned.StatusFailure {
			message := status.Status.Message
			if len(message) == 0 {
				message = "unknown error prevented import"
			}
			api.SetTagConditions(stream, tag, api.TagEventCondition{
				Type:       api.ImportSuccess,
				Status:     kapi.ConditionFalse,
				Message:    message,
				Reason:     string(status.Status.Reason),
				Generation: gen,

				LastTransitionTime: now,
			})
			continue
		}
		image := status.Image

		importErr, imported := importedImages[image.Name]
		if importErr != nil {
			api.SetTagConditions(stream, tag, newImportFailedCondition(err, gen, now))
		}

		pullSpec, _ := api.MostAccuratePullSpec(image.DockerImageReference, image.Name, "")
		api.AddTagEventToImageStream(stream, tag, api.TagEvent{
			Created:              now,
			DockerImageReference: pullSpec,
			Image:                image.Name,
			Generation:           gen,
		})

		if imported {
			continue
		}
		_, err = r.images.Create(ctx, image)
		if kapierrors.IsAlreadyExists(err) {
			err = nil
		}
		importedImages[image.Name] = err
	}

	// TODO: should we allow partial failure?
	for _, err := range importedImages {
		if err != nil {
			return nil, err
		}
	}

	clearManifests(isi)

	if create {
		obj, err = r.internalStreams.Create(ctx, stream)
	} else {
		obj, _, err = r.internalStreams.Update(ctx, stream)
	}
	if err != nil {
		return nil, err
	}
	isi.Status.Import = obj.(*api.ImageStream)
	return isi, nil
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
	if status, ok := err.(kclient.APIStatus); ok {
		s := status.Status()
		c.Reason, c.Message = string(s.Reason), s.Message
	}
	return c
}

func invalidStatus(kind, position string, errs ...error) unversioned.Status {
	return kapierrors.NewInvalid(kind, position, errs).(kclient.APIStatus).Status()
}
