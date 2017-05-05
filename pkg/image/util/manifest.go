package util

import (
	"context"
	"net/http"
	"net/url"

	"github.com/docker/distribution/digest"

	"github.com/openshift/origin/pkg/image/importer"
)

// GetImageManifestByIDFromRegistry retrieves the image manifest from the registry using the basic
// authentication using the image ID.
func GetImageManifestByIDFromRegistry(registry *url.URL, repositoryName, imageID, username, password string) ([]byte, error) {
	ctx := context.Background()

	credentials := importer.NewBasicCredentials()
	credentials.Add(registry, username, password)

	repo, err := importer.NewContext(http.DefaultTransport, http.DefaultTransport).
		WithCredentials(credentials).
		Repository(ctx, registry, repositoryName, true)
	if err != nil {
		return nil, err
	}

	manifests, err := repo.Manifests(ctx, nil)
	if err != nil {
		return nil, err
	}

	manifest, err := manifests.Get(ctx, digest.Digest(imageID))
	if err != nil {
		return nil, err
	}
	_, manifestPayload, err := manifest.Payload()
	if err != nil {
		return nil, err
	}

	return manifestPayload, nil
}
