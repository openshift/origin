package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/manifest"
	repomw "github.com/docker/distribution/registry/middleware/repository"
	"github.com/docker/libtrust"
	kapi "k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"

	"github.com/openshift/origin/pkg/client"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

var (
	// enumManifestKindToKeep filters images having DelectionTimestamp unset.
	enumManifestKindToKeep fields.Selector
	// enumManifestKindToKeep filters images having DelectionTimestamp set.
	enumManifestKindToDelete fields.Selector
	// enumManifestKindToKeep makes Enumerate method return all images found.
	enumManifestKindAll fields.Selector
)

func init() {
	repomw.Register("openshift", repomw.InitFunc(newRepository))

	var err error
	enumManifestKindToKeep, err = fields.ParseSelector("image.status.phase!=" + imageapi.ImagePurging)
	if err != nil {
		panic(err.Error())
	}

	enumManifestKindToDelete, err = fields.ParseSelector("image.status.phase==" + imageapi.ImagePurging)
	if err != nil {
		panic(err.Error())
	}

	enumManifestKindAll = fields.Everything()
}

type repository struct {
	distribution.Repository

	ctx            context.Context
	registryClient client.Interface
	registryAddr   string
	namespace      string
	name           string
	// getNoCheckImageStream prevents Get() function from performing an
	// existence check on image stream.
	getNoCheckImageStream bool
	imageFieldSelector    fields.Selector
}

// newRepository returns a new repository middleware.
func newRepository(ctx context.Context, repo distribution.Repository, options map[string]interface{}) (distribution.Repository, error) {
	registryAddr := os.Getenv("REGISTRY_URL")
	if len(registryAddr) == 0 {
		return nil, errors.New("REGISTRY_URL is required")
	}

	registryClient, err := NewRegistryOpenShiftClient()
	if err != nil {
		return nil, err
	}

	nameParts := strings.SplitN(repo.Name(), "/", 2)
	if len(nameParts) != 2 {
		return nil, fmt.Errorf("invalid repository name %q: it must be of the format <project>/<name>", repo.Name())
	}

	return &repository{
		Repository:     repo,
		registryClient: registryClient,
		registryAddr:   registryAddr,
		namespace:      nameParts[0],
		name:           nameParts[1],
	}, nil
}

// makeChangeEnumKindOption constructs a manifest service option causing the
// service to enumerate chosen kind of manifest revisions.
func makeChangeEnumKindOption(kind fields.Selector) distribution.ManifestServiceOption {
	return func(manServ distribution.ManifestService) error {
		repo, ok := manServ.(*repository)
		if !ok {
			return fmt.Errorf("unsupported type of manifest service (%T != %T)", manServ, &repository{})
		}
		repo.imageFieldSelector = enumManifestKindToKeep
		return nil
	}
}

// makeChangeEnumKindOption constructs a manifest service option causing the
// service to enumerate chosen kind of manifest revisions.
func makeGetCheckImageStreamOption(check bool) distribution.ManifestServiceOption {
	return func(manServ distribution.ManifestService) error {
		repo, ok := manServ.(*repository)
		if !ok {
			return fmt.Errorf("unsupported type of manifest service (%T != %T)", manServ, &repository{})
		}
		repo.getNoCheckImageStream = !check
		return nil
	}
}

// Manifests returns manifest service with given options applied.
func (r *repository) Manifests(ctx context.Context, options ...distribution.ManifestServiceOption) (distribution.ManifestService, error) {
	if r.ctx != ctx && len(options) == 0 {
		return r, nil
	}
	repo := repository(*r)
	repo.ctx = ctx

	for _, opt := range options {
		if err := opt(&repo); err != nil {
			return nil, err
		}
	}

	return &repo, nil
}

// Tags lists the tags under the named repository.
func (r *repository) Tags() ([]string, error) {
	imageStream, err := r.getImageStream()
	if err != nil {
		return []string{}, nil
	}
	tags := []string{}
	for tag := range imageStream.Status.Tags {
		tags = append(tags, tag)
	}

	return tags, nil
}

// Exists returns true if the manifest specified by dgst exists.
func (r *repository) Exists(dgst digest.Digest) (bool, error) {
	image, err := r.getImage(dgst)
	if err != nil {
		return false, err
	}
	return image != nil, nil
}

// ExistsByTag returns true if the manifest with tag `tag` exists.
func (r *repository) ExistsByTag(tag string) (bool, error) {
	imageStream, err := r.getImageStream()
	if err != nil {
		return false, err
	}
	_, found := imageStream.Status.Tags[tag]
	return found, nil
}

// Get retrieves the manifest with digest `dgst`.
func (r *repository) Get(dgst digest.Digest) (*manifest.SignedManifest, error) {
	if !r.getNoCheckImageStream {
		if _, err := r.getImageStreamImage(dgst); err != nil {
			log.Errorf("Error retrieving ImageStreamImage %s/%s@%s: %v", r.namespace, r.name, dgst.String(), err)
			return nil, err
		}
	}

	image, err := r.getImage(dgst)
	if err != nil {
		log.Errorf("Error retrieving image %s: %v", dgst.String(), err)
		return nil, err
	}

	return r.manifestFromImage(image)
}

// Enumerate retrieves digests of manifest revisions in this repository.
func (r *repository) Enumerate() ([]digest.Digest, error) {
	images, err := r.getImages()
	if err != nil {
		log.Errorf("Error enumerating images: %v", err)
		return nil, err
	}

	res := make([]digest.Digest, 0, len(images.Items))
	for _, img := range images.Items {
		dgst, err := digest.ParseDigest(img.Image.Name)
		if err != nil {
			log.Warnf("Failed to parse image name \"%s\" into digest: %v", img.Name, err)
		} else {
			res = append(res, dgst)
		}
	}

	return res, nil
}

// GetByTag retrieves the named manifest with the provided tag
func (r *repository) GetByTag(tag string, options ...distribution.ManifestServiceOption) (*manifest.SignedManifest, error) {
	for _, opt := range options {
		if err := opt(r); err != nil {
			return nil, err
		}
	}
	imageStreamTag, err := r.getImageStreamTag(tag)
	if err != nil {
		log.Errorf("Error getting ImageStreamTag %q: %v", tag, err)
		return nil, err
	}
	image := &imageStreamTag.Image

	dgst, err := digest.ParseDigest(imageStreamTag.Image.Name)
	if err != nil {
		log.Errorf("Error parsing digest %q: %v", imageStreamTag.Image.Name, err)
		return nil, err
	}

	image, err = r.getImage(dgst)
	if err != nil {
		log.Errorf("Error getting image %q: %v", dgst.String(), err)
		return nil, err
	}

	return r.manifestFromImage(image)
}

// Put creates or updates the named manifest.
func (r *repository) Put(manifest *manifest.SignedManifest) error {
	// Resolve the payload in the manifest.
	payload, err := manifest.Payload()
	if err != nil {
		return err
	}

	// Calculate digest
	dgst, err := digest.FromBytes(payload)
	if err != nil {
		return err
	}

	// Upload to openshift
	ism := imageapi.ImageStreamMapping{
		ObjectMeta: kapi.ObjectMeta{
			Namespace: r.namespace,
			Name:      r.name,
		},
		Tag: manifest.Tag,
		Image: imageapi.Image{
			ObjectMeta: kapi.ObjectMeta{
				Name: dgst.String(),
				Annotations: map[string]string{
					imageapi.ManagedByOpenShiftAnnotation: "true",
				},
			},
			DockerImageReference: fmt.Sprintf("%s/%s/%s@%s", r.registryAddr, r.namespace, r.name, dgst.String()),
			DockerImageManifest:  string(payload),
		},
	}

	if err := r.registryClient.ImageStreamMappings(r.namespace).Create(&ism); err != nil {
		// if the error was that the image stream wasn't found, try to auto provision it
		statusErr, ok := err.(*kerrors.StatusError)
		if !ok {
			log.Errorf("Error creating ImageStreamMapping: %s", err)
			return err
		}

		status := statusErr.ErrStatus
		if status.Code != http.StatusNotFound || status.Details.Kind != "imageStream" || status.Details.Name != r.name {
			log.Errorf("Error creating ImageStreamMapping: %s", err)
			return err
		}

		stream := imageapi.ImageStream{
			ObjectMeta: kapi.ObjectMeta{
				Name: r.name,
			},
		}

		client, ok := UserClientFrom(r.ctx)
		if !ok {
			log.Errorf("Error creating user client to auto provision image stream: Origin user client unavailable")
			return statusErr
		}

		if _, err := client.ImageStreams(r.namespace).Create(&stream); err != nil {
			log.Errorf("Error auto provisioning image stream: %s", err)
			return statusErr
		}

		// try to create the ISM again
		if err := r.registryClient.ImageStreamMappings(r.namespace).Create(&ism); err != nil {
			log.Errorf("Error creating image stream mapping: %s", err)
			return err
		}
	}

	// Grab each json signature and store them.
	signatures, err := manifest.Signatures()
	if err != nil {
		return err
	}

	for _, signature := range signatures {
		if err := r.Signatures().Put(dgst, signature); err != nil {
			log.Errorf("Error storing signature: %s", err)
			return err
		}
	}

	return nil
}

// Delete deletes the manifest with digest `dgst`. Note: Image resources
// in OpenShift are deleted via 'oadm prune images'. This function deletes
// the content related to the manifest in the registry's storage (signatures).
func (r *repository) Delete(dgst digest.Digest) error {
	manServ, err := r.Repository.Manifests(r.ctx)
	if err != nil {
		return err
	}
	// TODO: run finalize on image object
	return manServ.Delete(dgst)
}

// getImageStream retrieves the ImageStream for r.
func (r *repository) getImageStream() (*imageapi.ImageStream, error) {
	return r.registryClient.ImageStreams(r.namespace).Get(r.name)
}

// getImage retrieves the Image with digest `dgst`.
func (r *repository) getImage(dgst digest.Digest) (*imageapi.Image, error) {
	return r.registryClient.Images().Get(dgst.String())
}

// getImages retrieves repository's ImageStreamImageList.
func (r *repository) getImages() (*imageapi.ImageStreamImageList, error) {
	return r.registryClient.ImageStreamImages(r.namespace).List(labels.Everything(), r.imageFieldSelector)
}

// getImageStreamTag retrieves the Image with tag `tag` for the ImageStream
// associated with r.
func (r *repository) getImageStreamTag(tag string) (*imageapi.ImageStreamTag, error) {
	return r.registryClient.ImageStreamTags(r.namespace).Get(r.name, tag)
}

// getImageStreamImage retrieves the Image with digest `dgst` for the ImageStream
// associated with r. This ensures the image belongs to the image stream.
func (r *repository) getImageStreamImage(dgst digest.Digest) (*imageapi.ImageStreamImage, error) {
	return r.registryClient.ImageStreamImages(r.namespace).Get(r.name, dgst.String())
}

// manifestFromImage converts an Image to a SignedManifest.
func (r *repository) manifestFromImage(image *imageapi.Image) (*manifest.SignedManifest, error) {
	dgst, err := digest.ParseDigest(image.Name)
	if err != nil {
		return nil, err
	}

	// Fetch the signatures for the manifest
	signatures, err := r.Signatures().Get(dgst)
	if err != nil {
		return nil, err
	}

	jsig, err := libtrust.NewJSONSignature([]byte(image.DockerImageManifest), signatures...)
	if err != nil {
		return nil, err
	}

	// Extract the pretty JWS
	raw, err := jsig.PrettySignature("signatures")
	if err != nil {
		return nil, err
	}

	var sm manifest.SignedManifest
	if err := json.Unmarshal(raw, &sm); err != nil {
		return nil, err
	}
	return &sm, err
}
