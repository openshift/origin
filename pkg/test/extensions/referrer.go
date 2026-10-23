package extensions

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/registry"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/credentials"
)

// Keep these values in sync with the OCI referrer contract in
// openshift-eng/openshift-tests-extension/docs/oci-referrers.md.
const (
	testExtensionArtifactType = "application/vnd.openshift.tests-extension.v1+gzip"
	testExtensionLayerType    = "application/gzip"
	ociTitleAnnotation        = "org.opencontainers.image.title"
)

type extensionReferrer struct {
	repository *remote.Repository
	manifest   ocispec.Descriptor
	layer      ocispec.Descriptor
}

// registryOnlyClient keeps registry-supplied pagination URLs on the registry
// host. The auth client can still contact the registry's token service.
type registryOnlyClient struct {
	host   string
	client *auth.Client
}

func (c registryOnlyClient) Do(req *http.Request) (*http.Response, error) {
	if req.URL.Scheme != "https" || req.URL.Host != c.host {
		return nil, fmt.Errorf("registry request outside https://%s: %s", c.host, req.URL.Redacted())
	}
	return c.client.Do(req)
}

// findExtensionReferrer returns nil only when there is no artifact matching
// the requested gzip filename. Registry and malformed artifact errors are fatal.
func findExtensionReferrer(imageRef, filename, authFile string) (*extensionReferrer, error) {
	ref, err := registry.ParseReference(imageRef)
	if err != nil {
		return nil, err
	}
	repo, err := remote.NewRepository(ref.Registry + "/" + ref.Repository)
	if err != nil {
		return nil, err
	}
	authClient := &auth.Client{Cache: auth.NewCache()}
	if authFile != "" {
		store, err := credentials.NewStore(authFile, credentials.StoreOptions{})
		if err != nil {
			return nil, err
		}
		authClient.Credential = credentials.Credential(store)
	}
	repo.Client = registryOnlyClient{host: ref.Registry, client: authClient}
	repo.ReferrerListMaxPages = 20

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	subject, err := repo.Resolve(ctx, ref.Reference)
	if err != nil {
		return nil, err
	}
	var found *extensionReferrer
	err = repo.Referrers(ctx, subject, testExtensionArtifactType, func(page []ocispec.Descriptor) error {
		for _, candidate := range page {
			if candidate.ArtifactType != "" && candidate.ArtifactType != testExtensionArtifactType {
				continue
			}
			if candidate.Size < 0 || candidate.Size > 1024*1024 {
				return fmt.Errorf("referrer manifest %s has invalid size %d", candidate.Digest, candidate.Size)
			}
			rc, err := repo.Fetch(ctx, candidate)
			if err != nil {
				return err
			}
			data, readErr := content.ReadAll(rc, candidate)
			closeErr := rc.Close()
			if readErr != nil {
				return readErr
			}
			if closeErr != nil {
				return closeErr
			}
			var manifest ocispec.Manifest
			if err := json.Unmarshal(data, &manifest); err != nil {
				return err
			}
			// Some registries fall back to a synthetic index. Verify the subject
			// in the artifact itself before accepting a listed descriptor.
			layer, ok := extensionLayer(manifest, subject, filename)
			if !ok {
				continue
			}
			if found != nil {
				return fmt.Errorf("multiple test extension referrers for %s on %s", filename, subject.Digest)
			}
			found = &extensionReferrer{repository: repo, manifest: candidate, layer: layer}
		}
		return nil
	})
	return found, err
}

func extensionLayer(manifest ocispec.Manifest, subject ocispec.Descriptor, filename string) (ocispec.Descriptor, bool) {
	if manifest.Subject == nil || manifest.Subject.Digest != subject.Digest || manifest.ArtifactType != testExtensionArtifactType || len(manifest.Layers) != 1 {
		return ocispec.Descriptor{}, false
	}
	layer := manifest.Layers[0]
	return layer, layer.MediaType == testExtensionLayerType && layer.Annotations[ociTitleAnnotation] == filename
}

func (referrer *extensionReferrer) download(destination string) (err error) {
	if referrer.layer.Size < 0 || referrer.layer.Size > 1024*1024*1024 {
		return fmt.Errorf("test extension blob has invalid size %d", referrer.layer.Size)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	rc, err := referrer.repository.Fetch(ctx, referrer.layer)
	if err != nil {
		return err
	}
	defer rc.Close()
	tmp, err := os.CreateTemp(filepath.Dir(destination), ".test-extension-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	if _, err := io.Copy(tmp, content.NewVerifyReader(rc, referrer.layer)); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), destination)
}
