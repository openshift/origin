package repository

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/distribution"
	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/manifest"
	repomw "github.com/docker/distribution/registry/middleware/repository"
	"github.com/docker/libtrust"
	"github.com/openshift/origin/pkg/client"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

func init() {
	repomw.Register("openshift", repomw.InitFunc(newRepository))
}

type repository struct {
	distribution.Repository

	// TODO cache this at the app level
	registryClient *client.Client
	registryAddr   string
	namespace      string
	name           string
}

// newRepository returns a new repository middleware.
func newRepository(repo distribution.Repository, options map[string]interface{}) (distribution.Repository, error) {
	openshiftAddr := os.Getenv("OPENSHIFT_MASTER")
	if len(openshiftAddr) == 0 {
		return nil, errors.New("OPENSHIFT_MASTER is required")
	}

	registryAddr := os.Getenv("REGISTRY_URL")
	if len(registryAddr) == 0 {
		return nil, errors.New("REGISTRY_URL is required")
	}

	insecure := os.Getenv("OPENSHIFT_INSECURE") == "true"
	var tlsClientConfig kclient.TLSClientConfig
	if !insecure {
		caData := os.Getenv("OPENSHIFT_CA_DATA")
		if len(caData) == 0 {
			return nil, errors.New("OPENSHIFT_CA_DATA is required")
		}
		certData := os.Getenv("OPENSHIFT_CERT_DATA")
		if len(certData) == 0 {
			return nil, errors.New("OPENSHIFT_CERT_DATA is required")
		}
		certKeyData := os.Getenv("OPENSHIFT_KEY_DATA")
		if len(certKeyData) == 0 {
			return nil, errors.New("OPENSHIFT_KEY_DATA is required")
		}
		tlsClientConfig = kclient.TLSClientConfig{
			CAData:   []byte(caData),
			CertData: []byte(certData),
			KeyData:  []byte(certKeyData),
		}
	}

	registryClientConfig := kclient.Config{
		Host:            openshiftAddr,
		TLSClientConfig: tlsClientConfig,
		Insecure:        insecure,
	}
	registryClient, err := client.New(&registryClientConfig)
	if err != nil {
		return nil, fmt.Errorf("Error creating OpenShift client: %s", err)
	}

	nameParts := strings.SplitN(repo.Name(), "/", 2)

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
	_, err := r.getImageStreamImage(dgst)
	if err != nil {
		return nil, err
	}

	image, err := r.getImage(dgst)
	if err != nil {
		return nil, err
	}

	return r.manifestFromImage(image)
}

// Get retrieves the named manifest, if it exists.
func (r *repository) GetByTag(tag string) (*manifest.SignedManifest, error) {
	image, err := r.getImageStreamTag(tag)
	if err != nil {
		// TODO remove when docker 1.6 is out
		// Since docker 1.5 doesn't support pull by id, we're simulating pull by id
		// against the v2 registry by using a pull spec of the form
		// <repo>:<hex portion of digest>, so once we verify we got a 404 from
		// getImageStreamTag, we construct a digest and attempt to get the
		// imageStreamImage using that digest.

		// TODO replace with kerrors.IsStatusError when it's rebased in
		if err, ok := err.(*kerrors.StatusError); !ok {
			log.Errorf("GetByTag: getImageStreamTag returned error: %s", err)
			return nil, err
		} else if err.ErrStatus.Code != http.StatusNotFound {
			log.Errorf("GetByTag: getImageStreamTag returned non-404: %s", err)
		}
		// let's try to get by id
		dgst, dgstErr := digest.ParseDigest("sha256:" + tag)
		if dgstErr != nil {
			log.Errorf("GetByTag: unable to parse digest: %s", dgstErr)
			return nil, err
		}
		image, err = r.getImageStreamImage(dgst)
		if err != nil {
			log.Errorf("GetByTag: getImageStreamImage returned error: %s", err)
			return nil, err
		}
	}

	dgst, err := digest.ParseDigest(image.Name)
	if err != nil {
		return nil, err
	}

	image, err = r.getImage(dgst)
	if err != nil {
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
	irm := imageapi.ImageStreamMapping{
		ObjectMeta: kapi.ObjectMeta{
			Namespace: r.namespace,
			Name:      r.name,
		},
		Tag: manifest.Tag,
		Image: imageapi.Image{
			ObjectMeta: kapi.ObjectMeta{
				Name: dgst.String(),
			},
			DockerImageReference: fmt.Sprintf("%s/%s/%s@%s", r.registryAddr, r.namespace, r.name, dgst.String()),
			DockerImageManifest:  string(payload),
		},
	}

	if err := r.registryClient.ImageStreamMappings(r.namespace).Create(&irm); err != nil {
		log.Errorf("Error creating ImageStreamMapping: %s", err)
		return err
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

// Delete deletes the manifest with digest `dgst`.
func (r *repository) Delete(dgst digest.Digest) error {
	return r.registryClient.Images().Delete(dgst.String())
}

// getImageStream retrieves the ImageStream for r.
func (r *repository) getImageStream() (*imageapi.ImageStream, error) {
	return r.registryClient.ImageStreams(r.namespace).Get(r.name)
}

// getImage retrieves the Image with digest `dgst`. This uses the registry's
// credentials and should ONLY
func (r *repository) getImage(dgst digest.Digest) (*imageapi.Image, error) {
	return r.registryClient.Images().Get(dgst.String())
}

// getImageStreamTag retrieves the Image with tag `tag` for the ImageStream
// associated with r.
func (r *repository) getImageStreamTag(tag string) (*imageapi.Image, error) {
	return r.registryClient.ImageStreamTags(r.namespace).Get(r.name, tag)
}

// getImageStreamImage retrieves the Image with digest `dgst` for the ImageStream
// associated with r. This ensures the user has access to the image.
func (r *repository) getImageStreamImage(dgst digest.Digest) (*imageapi.Image, error) {
	// TODO !!! use user credentials, not the registry's !!!
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
