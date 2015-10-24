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
	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/manifest"
	repomw "github.com/docker/distribution/registry/middleware/repository"
	"github.com/docker/libtrust"
	"github.com/openshift/origin/pkg/client"
	imageapi "github.com/openshift/origin/pkg/image/api"
	"golang.org/x/net/context"
	kapi "k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/client/unversioned"
)

func init() {
	repomw.Register("openshift", repomw.InitFunc(newRepository))
}

type repository struct {
	distribution.Repository

	registryClient *client.Client
	registryAddr   string
	namespace      string
	name           string
}

// newRepository returns a new repository middleware.
func newRepository(repo distribution.Repository, options map[string]interface{}) (distribution.Repository, error) {
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

// Manifests returns r, which implements distribution.ManifestService.
func (r *repository) Manifests() distribution.ManifestService {
	return r
}

// Tags lists the tags under the named repository.
func (r *repository) Tags(ctx context.Context) ([]string, error) {
	imageStream, err := r.getImageStream(ctx)
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
func (r *repository) Exists(ctx context.Context, dgst digest.Digest) (bool, error) {
	image, err := r.getImage(dgst)
	if err != nil {
		return false, err
	}
	return image != nil, nil
}

// ExistsByTag returns true if the manifest with tag `tag` exists.
func (r *repository) ExistsByTag(ctx context.Context, tag string) (bool, error) {
	imageStream, err := r.getImageStream(ctx)
	if err != nil {
		return false, err
	}
	_, found := imageStream.Status.Tags[tag]
	return found, nil
}

// Get retrieves the manifest with digest `dgst`.
func (r *repository) Get(ctx context.Context, dgst digest.Digest) (*manifest.SignedManifest, error) {
	if _, err := r.getImageStreamImage(ctx, dgst); err != nil {
		log.Errorf("Error retrieving ImageStreamImage %s/%s@%s: %v", r.namespace, r.name, dgst.String(), err)
		return nil, err
	}

	image, err := r.getImage(dgst)
	if err != nil {
		log.Errorf("Error retrieving image %s: %v", dgst.String(), err)
		return nil, err
	}

	return r.manifestFromImage(image)
}

// GetByTag retrieves the named manifest with the provided tag
func (r *repository) GetByTag(ctx context.Context, tag string) (*manifest.SignedManifest, error) {
	imageStreamTag, err := r.getImageStreamTag(ctx, tag)
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
func (r *repository) Put(ctx context.Context, manifest *manifest.SignedManifest) error {
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

		client, ok := UserClientFrom(ctx)
		if !ok {
			log.Errorf("Error creating user client to auto provision image stream: Origin user client unavailable")
			return statusErr
		}

		if _, err := client.ImageStreams(r.namespace).Create(&stream); err != nil {
			log.Errorf("Error auto provisioning image stream: %s", err)
			return statusErr
		}

		// try to create the ISM again
		if err := unversioned.RetryOnConflict(unversioned.DefaultRetry, func() error {
			return r.registryClient.ImageStreamMappings(r.namespace).Create(&ism)
		}); err != nil {
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
func (r *repository) Delete(ctx context.Context, dgst digest.Digest) error {
	return r.Repository.Manifests().Delete(ctx, dgst)
}

// getImageStream retrieves the ImageStream for r.
func (r *repository) getImageStream(ctx context.Context) (*imageapi.ImageStream, error) {
	return r.registryClient.ImageStreams(r.namespace).Get(r.name)
}

// getImage retrieves the Image with digest `dgst`.
func (r *repository) getImage(dgst digest.Digest) (*imageapi.Image, error) {
	return r.registryClient.Images().Get(dgst.String())
}

// getImageStreamTag retrieves the Image with tag `tag` for the ImageStream
// associated with r.
func (r *repository) getImageStreamTag(ctx context.Context, tag string) (*imageapi.ImageStreamTag, error) {
	return r.registryClient.ImageStreamTags(r.namespace).Get(r.name, tag)
}

// getImageStreamImage retrieves the Image with digest `dgst` for the ImageStream
// associated with r. This ensures the image belongs to the image stream.
func (r *repository) getImageStreamImage(ctx context.Context, dgst digest.Digest) (*imageapi.ImageStreamImage, error) {
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
