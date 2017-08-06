package imagestreamtag

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/containers/image/manifest"
	"github.com/docker/distribution"
	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/manifest/schema1"
	"github.com/docker/distribution/manifest/schema2"
	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/endpoints/handlers/negotiation"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	kapi "k8s.io/kubernetes/pkg/api"
	authorizationapi "k8s.io/kubernetes/pkg/apis/authorization"
	authorizationclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/authorization/internalversion"

	authorizationutil "github.com/openshift/origin/pkg/authorization/util"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	"github.com/openshift/origin/pkg/image/apis/image/validation"
	"github.com/openshift/origin/pkg/image/dockerlayer"
	"github.com/openshift/origin/pkg/image/importer"
	"github.com/openshift/origin/pkg/image/registry/image"
	"github.com/openshift/origin/pkg/image/registry/imagestream"
)

type instantiateStrategy struct {
	runtime.ObjectTyper
}

var InstantiateStrategy = &instantiateStrategy{ObjectTyper: kapi.Scheme}

func (instantiateStrategy) NamespaceScoped() bool                                            { return true }
func (instantiateStrategy) GenerateName(base string) string                                  { return base }
func (instantiateStrategy) AllowCreateOnUpdate() bool                                        { return false }
func (instantiateStrategy) AllowUnconditionalUpdate() bool                                   { return false }
func (instantiateStrategy) Canonicalize(obj runtime.Object)                                  {}
func (instantiateStrategy) PrepareForCreate(ctx apirequest.Context, obj runtime.Object)      {}
func (instantiateStrategy) PrepareForUpdate(ctx apirequest.Context, obj, old runtime.Object) {}
func (instantiateStrategy) Validate(ctx apirequest.Context, obj runtime.Object) field.ErrorList {
	instantiate := obj.(*imageapi.ImageStreamTagInstantiate)
	return validation.ValidateImageStreamTagInstantiate(instantiate)
}
func (instantiateStrategy) ValidateUpdate(ctx apirequest.Context, obj, old runtime.Object) field.ErrorList {
	instantiate := obj.(*imageapi.ImageStreamTagInstantiate)
	return validation.ValidateImageStreamTagInstantiate(instantiate)
}

type InstantiateREST struct {
	imageRegistry       image.Registry
	imageStreamRegistry imagestream.Registry
	sarClient           authorizationclient.SubjectAccessReviewInterface
	defaultRegistry     imageapi.RegistryHostnameRetriever
	gr                  schema.GroupResource
	repository          importer.RepositoryRetriever
	expired             RegistryAuthExpired
}

var _ rest.Creater = &InstantiateREST{}

// New creates a new build generation request
func (r *InstantiateREST) New() runtime.Object {
	return &imageapi.ImageStreamTagInstantiate{}
}

func (r *InstantiateREST) Create(ctx apirequest.Context, obj runtime.Object, includeUninitialized bool) (runtime.Object, error) {
	imageInstantiate, ok := obj.(*imageapi.ImageStreamTagInstantiate)
	if !ok {
		return nil, errors.NewBadRequest(fmt.Sprintf("obj is not an ImageStreamTagInstantiate: %#v", obj))
	}
	uid := imageInstantiate.UID
	imageInstantiate.UID = ""
	if err := rest.BeforeCreate(InstantiateStrategy, ctx, obj); err != nil {
		return nil, err
	}

	// TODO: decide whether specifying UID precondition like this is acceptable
	var preconditionUID *types.UID
	if len(uid) > 0 {
		preconditionUID = &imageInstantiate.UID
	}

	id := imageInstantiate.Name
	imageStreamName, _, ok := imageapi.SplitImageStreamTag(id)
	if !ok {
		return nil, fmt.Errorf("%q must be of the form <stream_name>:<tag>", id)
	}
	namespace, ok := apirequest.NamespaceFrom(ctx)
	if !ok {
		return nil, errors.NewBadRequest("a namespace must be specified to instantiate images")
	}

	target, tag, err := imageStreamForInstantiate(r.imageStreamRegistry, ctx, id, r.gr, preconditionUID)
	if err != nil {
		if !errors.IsNotFound(err) {
			return nil, err
		}

		// try to create the target if it doesn't exist
		target = &imageapi.ImageStream{
			ObjectMeta: metav1.ObjectMeta{
				Name:      imageStreamName,
				Namespace: namespace,
			},
		}
	}

	return r.completeInstantiate(ctx, tag, target, imageInstantiate, nil, "")
}

func (r *InstantiateREST) completeInstantiate(ctx apirequest.Context, tag string, target *imageapi.ImageStream, imageInstantiate *imageapi.ImageStreamTagInstantiate, layerBody io.Reader, mediaType string) (runtime.Object, error) {
	// TODO: load this from the default registry function
	insecure := true

	ref, u, err := registryTarget(target, r.defaultRegistry)
	if err != nil {
		return nil, err
	}

	// verify the user has access to the From image, if any is specified
	baseImageName, baseImageRepository, err := r.resolveTagInstantiateToImage(ctx, target, imageInstantiate)
	if err != nil {
		return nil, err
	}

	// no layer, so we load our base image (if necessary)
	var created time.Time
	var baseImage *imageapi.Image
	var sourceRepo distribution.Repository
	if len(baseImageName) > 0 {
		image, err := r.imageRegistry.GetImage(ctx, baseImageName, &metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		baseImage = image
		sourceRepo, err = r.repository.Repository(ctx, u, baseImageRepository, insecure)
		if err != nil {
			return nil, errors.NewInternalError(fmt.Errorf("could not contact integrated registry: %v", err))
		}
		glog.V(4).Infof("Using base image for instantiate of tag %s: %s from %s", imageInstantiate.Name, baseImageName, baseImageRepository)
		created = image.DockerImageMetadata.Created.Time
	}

	imageRepository := imageapi.DockerImageReference{Namespace: ref.Namespace, Name: ref.Name}.Exact()
	repo, err := r.repository.Repository(ctx, u, imageRepository, insecure)
	if err != nil {
		return nil, errors.NewInternalError(fmt.Errorf("could not contact integrated registry: %v", err))
	}

	var imageLayer *imageapi.ImageLayer
	var imageLayerDiffID digest.Digest
	if layerBody != nil {
		desc, diffID, modTime, err := uploadLayer(ctx, layerBody, repo, mediaType)
		if err != nil {
			return nil, errors.NewInternalError(fmt.Errorf("unable to upload new image layer: %v", err))
		}
		imageLayer = &imageapi.ImageLayer{
			Name:      desc.Digest.String(),
			LayerSize: desc.Size,
			MediaType: mediaType,
		}
		imageLayerDiffID = diffID

		if modTime != nil && created.Before(*modTime) {
			created = *modTime
		}
	}

	target, image, err := instantiateImage(
		ctx, r.gr,
		repo, sourceRepo, r.imageStreamRegistry, r.imageRegistry,
		target, baseImage, imageInstantiate, created,
		imageLayer, imageLayerDiffID,
		*ref,
	)
	if err != nil {
		glog.V(4).Infof("Failed cloning into tag %s: %v", imageInstantiate.Name, err)
		return nil, err
	}

	return newISTag(tag, target, image, false)
}

// resolveTagInstantiateToImage checks for a base image reference, attempts to resolve it and check for error, and then
// returns an image name or error. It always returns the source repository name, relative to the current registry.
func (r *InstantiateREST) resolveTagInstantiateToImage(ctx apirequest.Context, target *imageapi.ImageStream, imageInstantiate *imageapi.ImageStreamTagInstantiate) (string, string, error) {
	from := imageInstantiate.From
	if from == nil {
		return "", "", nil
	}

	namespace := target.Namespace
	if len(from.Namespace) > 0 {
		namespace = from.Namespace
	}

	user, ok := apirequest.UserFrom(ctx)
	if !ok {
		return "", "", errors.NewForbidden(r.gr, imageInstantiate.Name, fmt.Errorf("no user context available"))
	}

	switch from.Kind {
	case "ImageStreamTag":
		name, tag, ok := imageapi.SplitImageStreamTag(from.Name)
		if !ok {
			return "", "", errors.NewBadRequest("from.name not accepted")
		}
		if len(name) == 0 {
			name = target.Name
		}
		ok, err := hasAccessToStream(ctx, r.sarClient, user, namespace, name)
		if err != nil {
			return "", "", err
		}
		if !ok {
			return "", "", errors.NewInvalid(schema.GroupKind{Group: r.gr.Group, Kind: "ImageStreamTagInstantiate"}, imageInstantiate.Name, field.ErrorList{field.Forbidden(field.NewPath("from"), "not allowed to access that image stream")})
		}
		source, err := r.imageStreamRegistry.GetImageStream(apirequest.WithNamespace(ctx, namespace), name, &metav1.GetOptions{})
		if err != nil {
			return "", "", err
		}
		event := imageapi.LatestTaggedImage(source, tag)
		if event == nil {
			return "", "", errors.NewNotFound(r.gr, imageapi.JoinImageStreamTag(name, tag))
		}
		if len(event.Image) == 0 {
			return "", "", errors.NewInvalid(schema.GroupKind{Group: r.gr.Group, Kind: "ImageStreamTagInstantiate"}, imageInstantiate.Name, field.ErrorList{field.Invalid(field.NewPath("from.name"), from.Name, "does not point to an image stream tag that resolves to a known image")})
		}
		return event.Image, imageapi.DockerImageReference{Namespace: namespace, Name: name}.Exact(), nil

	case "ImageStreamImage":
		name, imageName, ok := imageapi.SplitImageStreamImage(from.Name)
		if !ok {
			return "", "", errors.NewBadRequest("from.name not accepted")
		}
		if len(name) == 0 {
			name = target.Name
		}
		ok, err := hasAccessToStream(ctx, r.sarClient, user, namespace, name)
		if err != nil {
			return "", "", err
		}
		if !ok {
			return "", "", errors.NewInvalid(schema.GroupKind{Group: r.gr.Group, Kind: "ImageStreamTagInstantiate"}, imageInstantiate.Name, field.ErrorList{field.Forbidden(field.NewPath("from"), "not allowed to access that image stream")})
		}
		source, err := r.imageStreamRegistry.GetImageStream(apirequest.WithNamespace(ctx, namespace), name, &metav1.GetOptions{})
		if err != nil {
			return "", "", err
		}
		_, event := imageapi.LatestImageTagEvent(source, imageName)
		if event == nil {
			return "", "", errors.NewNotFound(schema.GroupResource{Group: r.gr.Group, Resource: "imagestreamtags"}, imageapi.JoinImageStreamTag(name, imageName))
		}
		return imageName, imageapi.DockerImageReference{Namespace: namespace, Name: name}.Exact(), nil

	default:
		return "", "", errors.NewBadRequest("from.kind not accepted")
	}
}

type InstantiateLayerREST struct {
	rest *InstantiateREST
}

// Connect returns a ConnectHandler that will handle the request/response for a request
func (r *InstantiateLayerREST) Connect(ctx apirequest.Context, name string, options runtime.Object, responder rest.Responder) (http.Handler, error) {
	return &instantiateLayerHandler{
		r:         r,
		responder: responder,
		ctx:       ctx,
		name:      name,
		options:   options.(*imageapi.ImageStreamTagInstantiateOptions),
	}, nil
}

// New provides the primary object type
func (r *InstantiateLayerREST) New() runtime.Object {
	return &imageapi.ImageStreamTagInstantiateOptions{}
}

// NewConnectOptions describes the layer upload process.
func (r *InstantiateLayerREST) NewConnectOptions() (runtime.Object, bool, string) {
	return &imageapi.ImageStreamTagInstantiateOptions{}, false, ""
}

// ConnectMethods returns POST, the only supported binary method.
func (r *InstantiateLayerREST) ConnectMethods() []string {
	return []string{"POST"}
}

// instantiateLayerHandler responds to upload requests
type instantiateLayerHandler struct {
	r *InstantiateLayerREST

	responder rest.Responder
	ctx       apirequest.Context
	name      string
	options   *imageapi.ImageStreamTagInstantiateOptions
}

var _ http.Handler = &instantiateLayerHandler{}

func (h *instantiateLayerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	build, err := h.handle(r.Body, r.Header.Get("Content-Type"))
	if err != nil {
		h.responder.Error(err)
		return
	}
	h.responder.Object(http.StatusCreated, build)
}

const formDataContentType = "multipart/form-data"

func (h *instantiateLayerHandler) handle(r io.Reader, contentType string) (runtime.Object, error) {
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil || !strings.HasPrefix(mediaType, "multipart/") {
		err := errors.NewGenericServerResponse(http.StatusNotAcceptable, "post", h.r.rest.gr, h.name, "", 0, false)
		err.ErrStatus.Message = fmt.Sprintf("The only accepted media type is '%s' with boundary specified. The first part must have content type application/json and contain an ImageStreamTagInstantiate object. The second part must be a Docker file layer with content type '%s'.", formDataContentType, manifest.DockerV2Schema2LayerMediaType)
		return nil, err
	}
	glog.V(5).Infof("Reader: %s %#v", mediaType, params)
	mr := multipart.NewReader(r, params["boundary"])
	part, err := mr.NextPart()
	if err == io.EOF {
		return nil, errors.NewBadRequest(fmt.Sprintf("You must provide two parts - an application/json or application/vnd.kubernetes.protobuf encoded ImageStreamTagInstantiate object and the binary contents of the layer."))
	}
	if err != nil {
		return nil, errors.NewBadRequest(err.Error())
	}
	info, err := negotiation.NegotiateInputSerializerForMediaType(part.Header.Get("Content-Type"), kapi.Codecs)
	if err != nil {
		return nil, err
	}
	targetVersion := imageapi.SchemeGroupVersion
	decoder := kapi.Codecs.DecoderToVersion(info.Serializer, targetVersion)

	data, err := ioutil.ReadAll(part)
	if err != nil {
		return nil, errors.NewBadRequest(err.Error())
	}
	obj, err := runtime.Decode(decoder, data)
	if err != nil {
		return nil, errors.NewBadRequest(err.Error())
	}
	imageInstantiate, ok := obj.(*imageapi.ImageStreamTagInstantiate)
	if !ok {
		return nil, errors.NewBadRequest(fmt.Sprintf("This endpoint requires an ImageStreamTagInstantiate object as the first part of the multipart request body. %T", obj))
	}
	glog.V(5).Infof("User requested instantiate %#v\n%s", imageInstantiate, data)
	if imageInstantiate.Name != h.name {
		return nil, errors.NewBadRequest(fmt.Sprintf("The name field on the provided ImageStreamTagInstantiate must match the name in the URL."))
	}

	if err := rest.BeforeCreate(InstantiateStrategy, h.ctx, imageInstantiate); err != nil {
		return nil, err
	}

	namespace, ok := apirequest.NamespaceFrom(h.ctx)
	if !ok {
		return nil, errors.NewBadRequest("a namespace must be specified to instantiate images")
	}
	if imageInstantiate.Namespace != namespace {
		return nil, errors.NewBadRequest(fmt.Sprintf("The namespace field on the provided ImageStreamTagInstantiate must match the namespace in the URL."))
	}

	part, err = mr.NextPart()
	if err == io.EOF {
		return nil, errors.NewBadRequest(fmt.Sprintf("You must provide two parts - an application/json or application/vnd.kubernetes.protobuf encoded ImageStreamTagInstantiate object and the binary contents of the layer."))
	}
	if err != nil {
		return nil, errors.NewBadRequest(err.Error())
	}

	layerContentType := part.Header.Get("Content-Type")
	if layerContentType != manifest.DockerV2Schema2LayerMediaType {
		// TODO: in the future support application/zip?
		err := errors.NewGenericServerResponse(http.StatusNotAcceptable, "post", h.r.rest.gr, h.name, "", 0, false)
		err.ErrStatus.Message = fmt.Sprintf("The only accepted media type for the layer part is a valid Docker layer (filesystem diff tar+gz) '%s'", manifest.DockerV2Schema2LayerMediaType)
		return nil, err
	}

	target, tag, err := imageStreamForInstantiate(h.r.rest.imageStreamRegistry, h.ctx, h.name, h.r.rest.gr, nil)
	if err != nil {
		if !errors.IsNotFound(err) {
			return nil, err
		}

		name, _, ok := imageapi.SplitImageStreamTag(h.name)
		if !ok {
			return nil, errors.NewBadRequest(fmt.Sprintf("The name field on the provided ImageStreamTagInstantiate is not a valid image stream tag name."))
		}

		// try to create the target if it doesn't exist
		target = &imageapi.ImageStream{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
		}
	}

	// if we specified a precondition, we must find a match.
	if len(h.options.PreconditionUID) > 0 && h.options.PreconditionUID != imageInstantiate.UID {
		return nil, errors.NewConflict(h.r.rest.gr, h.name, fmt.Errorf("the precondition UID does not match the image stream tag instantiate UID"))
	}

	return h.r.rest.completeInstantiate(h.ctx, tag, target, toImageStreamTagInstantiate(h.name, imageInstantiate, target), part, layerContentType)
}

func uploadLayer(ctx apirequest.Context, r io.Reader, repo distribution.Repository, mediaType string) (distribution.Descriptor, digest.Digest, *time.Time, error) {
	if len(mediaType) == 0 {
		return distribution.Descriptor{}, "", nil, fmt.Errorf("no content type set for layer")
	}
	blobs := repo.Blobs(ctx)
	bw, err := blobs.Create(ctx)
	if err != nil {
		return distribution.Descriptor{}, "", nil, fmt.Errorf("unable to begin uploading blob: %v", err)
	}
	defer bw.Cancel(ctx)

	algo := digest.Canonical
	// calculate the blob digest as the sha256 sum of the uploaded contents
	blobhash := algo.New().Hash()
	// calculate the diffID as the sha256 sum of the layer contents
	pr, pw := io.Pipe()
	layerhash := algo.New().Hash()
	ch := make(chan error)
	var modTime *time.Time
	go func() {
		defer close(ch)
		gr, err := gzip.NewReader(pr)
		if err != nil {
			ch <- fmt.Errorf("unable to create gzip reader layer upload: %v", err)
			return
		}
		if !gr.Header.ModTime.IsZero() {
			modTime = &gr.Header.ModTime
		}
		_, err = io.Copy(layerhash, gr)
		ch <- err
	}()

	// upload the bytes
	n, err := bw.ReadFrom(io.TeeReader(r, io.MultiWriter(blobhash, pw)))
	if err != nil {
		return distribution.Descriptor{}, "", nil, fmt.Errorf("unable to upload new layer (%d): %v", n, err)
	}
	if err := pw.Close(); err != nil {
		return distribution.Descriptor{}, "", nil, fmt.Errorf("unable to complete writing diffID: %v", err)
	}
	if err := <-ch; err != nil {
		return distribution.Descriptor{}, "", nil, fmt.Errorf("unable to calculate layer diffID: %v", err)
	}

	// TODO: use configurable digests
	layerdigest := digest.NewDigestFromBytes(algo, layerhash.Sum(make([]byte, 0, layerhash.Size())))
	blobdigest := digest.NewDigestFromBytes(algo, blobhash.Sum(make([]byte, 0, blobhash.Size())))

	glog.V(4).Infof("Wrote blob of %d bytes with digests layer=%s blob=%s", n, layerdigest, blobdigest)

	desc, err := bw.Commit(ctx, distribution.Descriptor{
		MediaType: mediaType,
		Size:      n,
		Digest:    blobdigest,
	})
	if err != nil {
		return distribution.Descriptor{}, "", nil, fmt.Errorf("unable to commit new layer: %v", err)
	}
	return desc, layerdigest, modTime, nil
}

// instantiateImage assembles the new image, saves it to the registry, then saves an image and tags the
// image stream.
func instantiateImage(
	ctx apirequest.Context, gr schema.GroupResource,
	repo, sourceRepo distribution.Repository, streams imagestream.Registry, images image.Registry,
	stream *imageapi.ImageStream, base *imageapi.Image, imageInstantiate *imageapi.ImageStreamTagInstantiate, created time.Time,
	layer *imageapi.ImageLayer, diffID digest.Digest,
	imageReference imageapi.DockerImageReference,
) (*imageapi.ImageStream, *imageapi.Image, error) {

	if imageInstantiate.Image == nil && base == nil {
		return nil, nil, fmt.Errorf("no image metadata available")
	}
	_, tag, ok := imageapi.SplitImageStreamTag(imageInstantiate.Name)
	if !ok {
		return nil, nil, fmt.Errorf("%q must be of the form <stream_name>:<tag>", imageInstantiate.Name)
	}

	// create a new config.json representing the image
	var meta imageapi.DockerImage
	if imageInstantiate.Image != nil {
		meta = imageInstantiate.Image.DockerImageMetadata
	} else {
		meta = base.DockerImageMetadata
	}
	imageConfig := imageapi.DockerImageConfig{
		OS:            meta.OS,
		Architecture:  meta.Architecture,
		Author:        meta.Author,
		Comment:       meta.Comment,
		Config:        meta.Config,
		Created:       metav1.NewTime(created),
		DockerVersion: meta.DockerVersion,

		Size:   0,
		RootFS: &imageapi.DockerConfigRootFS{Type: "layers"},

		// TODO: resolve
		// History         []DockerConfigHistory
		// OSVersion       string
		// OSFeatures      []string
	}
	layers, err := calculateUpdatedImageConfig(ctx, &imageConfig, base, layer, diffID, sourceRepo)
	if err != nil {
		return nil, nil, errors.NewInternalError(fmt.Errorf("unable to generate a new image configuration: %v", err))
	}
	configJSON, err := json.Marshal(&imageConfig)
	if err != nil {
		return nil, nil, errors.NewInternalError(fmt.Errorf("unable to marshal the new image config.json: %v", err))
	}

	// generate a manifest for that config.json
	glog.V(5).Infof("Saving layer %s onto %q with configJSON:\n%s", diffID, imageInstantiate.Name, configJSON)
	blobs := repo.Blobs(ctx)
	image, err := importer.SerializeImageAsSchema2Manifest(ctx, blobs, configJSON, layers)
	if err != nil {
		return nil, nil, errors.NewInternalError(fmt.Errorf("unable to generate a new image manifest: %v", err))
	}

	// create the manifest as an image
	imageReference.ID = image.Name
	image.DockerImageReference = imageReference.Exact()
	if err := images.CreateImage(ctx, image); err != nil && !errors.IsAlreadyExists(err) {
		return nil, nil, err
	}

	// record the image into the tag
	stream, err = saveTagToImageStream(ctx, stream, streams, gr, imageReference, tag, imageInstantiate.UID)
	return stream, image, err
}

// calculateUpdatedImageConfig generates a new image config.json with the provided info.
func calculateUpdatedImageConfig(
	ctx apirequest.Context,
	imageConfig *imageapi.DockerImageConfig,
	base *imageapi.Image,
	layer *imageapi.ImageLayer,
	diffID digest.Digest,
	sourceRepo distribution.Repository,
) ([]imageapi.ImageLayer, error) {
	var layers []imageapi.ImageLayer

	// initialize with the base
	if base != nil {
		layers = append(layers, base.DockerImageLayers...)
		for i := range layers {
			imageConfig.Size += layers[i].LayerSize
		}

		// need to look up the rootFS
		manifests, err := sourceRepo.Manifests(ctx)
		if err != nil {
			return nil, err
		}
		m, err := manifests.Get(ctx, digest.Digest(base.Name))
		if err != nil {
			return nil, err
		}
		var contents []byte
		switch t := m.(type) {
		case *schema2.DeserializedManifest:
			if t.Config.MediaType != manifest.DockerV2Schema2ConfigMediaType {
				return nil, fmt.Errorf("unrecognized config: %s", t.Config.MediaType)
			}
			contents, err = sourceRepo.Blobs(ctx).Get(ctx, t.Config.Digest)
			if err != nil {
				return nil, fmt.Errorf("unreadable config %s: %v", t.Config.Digest, err)
			}

			existingImageConfig := &imageapi.DockerImageConfig{}
			if err := json.Unmarshal(contents, existingImageConfig); err != nil {
				return nil, fmt.Errorf("manifest unreadable %s: %v", base.Name, err)
			}
			if existingImageConfig.RootFS == nil || existingImageConfig.RootFS.Type != "layers" {
				return nil, fmt.Errorf("unable to find rootFs description from base image %s", base.Name)
			}
			imageConfig.OS = existingImageConfig.OS
			imageConfig.Architecture = existingImageConfig.Architecture
			imageConfig.OSFeatures = existingImageConfig.OSFeatures
			imageConfig.OSVersion = existingImageConfig.OSVersion
			imageConfig.RootFS.DiffIDs = existingImageConfig.RootFS.DiffIDs

		case *schema1.SignedManifest:
			digest := digest.FromBytes(t.Canonical)
			contents, err = sourceRepo.Blobs(ctx).Get(ctx, digest)
			if err != nil {
				return nil, fmt.Errorf("unreadable config %s: %v", digest, err)
			}
			for _, layer := range t.FSLayers {
				imageConfig.RootFS.DiffIDs = append(imageConfig.RootFS.DiffIDs, layer.BlobSum.String())
			}
		default:
			return nil, fmt.Errorf("unrecognized manifest: %T", m)
		}
	}

	// add the optional layer if provided
	if layer != nil {
		// the layer goes at the front - the most recent image is always first
		layers = append(layers, *layer)
		imageConfig.Size += layer.LayerSize
		imageConfig.RootFS.DiffIDs = append(imageConfig.RootFS.DiffIDs, diffID.String())
	}

	// add the scratch layer in if no other layers exist
	if len(layers) == 0 {
		layers = append(layers, imageapi.ImageLayer{
			Name:      dockerlayer.GzippedEmptyLayerDigest.String(),
			LayerSize: int64(len(dockerlayer.GzippedEmptyLayer)),
			MediaType: manifest.DockerV2Schema2LayerMediaType,
		})
		imageConfig.RootFS.DiffIDs = append(imageConfig.RootFS.DiffIDs, dockerlayer.EmptyLayerDiffID.String())
		imageConfig.Size += layers[0].LayerSize
	}

	// the metav1 serialization of zero is not parseable by the Docker daemon, therefore
	// we must store a zero+1 value
	if imageConfig.Created.IsZero() {
		imageConfig.Created = metav1.Time{imageConfig.Created.Add(1 * time.Second)}
	}

	return layers, nil
}

// saveTagToImageStream attempts to update the image stream with the newly tagged image,
// retrying on conflict.
func saveTagToImageStream(
	ctx apirequest.Context,
	stream *imageapi.ImageStream,
	streams imagestream.Registry,
	gr schema.GroupResource,
	imageReference imageapi.DockerImageReference,
	tag string,
	imageInstantiateUID types.UID,
) (*imageapi.ImageStream, error) {

	var err error
	// add a status event pointing to the new image
	imageStreamName := stream.Name
	uid := stream.UID

	for {
		if stream == nil {
			stream, err = streams.GetImageStream(ctx, imageStreamName, &metav1.GetOptions{})
			if err != nil {
				return nil, err
			}
		}

		if stream.UID != uid {
			return nil, errors.NewConflict(schema.GroupResource{Group: gr.Group, Resource: "imagestreams"}, imageStreamName, fmt.Errorf("image stream was deleted and recreated during update"))
		}

		// TODO: follow rules on any spec tags w.r.t. variant
		ref := imageReference.Exact()

		event := imageapi.TagEvent{
			Created:              metav1.Now(),
			Image:                imageReference.ID,
			Generation:           stream.Generation,
			DockerImageReference: ref,
		}
		if !imageapi.AddTagEventToImageStream(stream, tag, event) {
			// already tagged, no-op
			return stream, nil
		}
		imageapi.UpdateTrackingTags(stream, tag, event)

		if stream.CreationTimestamp.IsZero() {
			stream, err = streams.CreateImageStreamInternal(ctx, stream)
		} else {
			stream, err = streams.UpdateImageStream(ctx, stream)
		}
		glog.V(4).Infof("Persisted image stream: %#v", stream)
		if err != nil {
			if errors.IsConflict(err) {
				continue
			}
			return nil, err
		}
		return stream, nil
	}
}

func toImageStreamTagInstantiate(name string, imageInstantiate *imageapi.ImageStreamTagInstantiate, stream *imageapi.ImageStream) *imageapi.ImageStreamTagInstantiate {
	imageInstantiate.ObjectMeta = metav1.ObjectMeta{
		Name:              name,
		Namespace:         stream.Namespace,
		UID:               imageInstantiate.UID,
		ResourceVersion:   stream.ResourceVersion,
		CreationTimestamp: imageInstantiate.CreationTimestamp,
	}
	return imageInstantiate
}

func imageStreamForInstantiate(registry imagestream.Registry, ctx apirequest.Context, id string, gr schema.GroupResource, preconditionUID *types.UID) (*imageapi.ImageStream, string, error) {
	imageStreamName, tag, ok := imageapi.SplitImageStreamTag(id)
	if !ok {
		return nil, "", fmt.Errorf("%q must be of the form <stream_name>:<tag>", id)
	}
	target, err := registry.GetImageStream(ctx, imageStreamName, &metav1.GetOptions{})
	if err != nil {
		return nil, tag, err
	}
	if preconditionUID != nil && *preconditionUID != target.UID {
		return nil, tag, errors.NewConflict(gr, id, fmt.Errorf("the precondition UID does not match the imagestream UID"))
	}
	return target, tag, nil
}

func hasAccessToStream(ctx apirequest.Context, client authorizationclient.SubjectAccessReviewInterface, user user.Info, namespace, name string) (bool, error) {
	review := authorizationutil.AddUserToSAR(user, &authorizationapi.SubjectAccessReview{
		Spec: authorizationapi.SubjectAccessReviewSpec{
			ResourceAttributes: &authorizationapi.ResourceAttributes{
				Verb:     "get",
				Group:    imageapi.GroupName,
				Resource: "imagestreams/layers",
			},
		},
	})
	resp, err := client.Create(review)
	if err != nil {
		return false, err
	}
	return resp.Status.Allowed, nil
}

func registryTarget(target *imageapi.ImageStream, defaultRegistry imageapi.RegistryHostnameRetriever) (*imageapi.DockerImageReference, *url.URL, error) {
	repositoryValue := target.Status.DockerImageRepository
	if len(repositoryValue) == 0 {
		if name, ok := defaultRegistry.InternalRegistryHostname(); ok {
			repositoryValue = imageapi.DockerImageReference{Registry: name, Namespace: target.Namespace, Name: target.Name}.Exact()
		}
	}
	if len(repositoryValue) == 0 {
		return nil, nil, errors.NewBadRequest("cannot instantiate image stream tags, no default registry has been configured")
	}
	ref, err := imageapi.ParseDockerImageReference(repositoryValue)
	if err != nil {
		return nil, nil, err
	}

	// TODO: insecure registry handling
	u, err := url.Parse("https://" + ref.Registry)
	if err != nil {
		return nil, nil, errors.NewBadRequest(fmt.Sprintf("cannot instantiate image stream tags, no default registry has been defined: %v", err))
	}
	return &ref, u, nil
}
