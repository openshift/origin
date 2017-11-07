package image

import (
	"context"
	"net/http"
	"net/url"

	"github.com/docker/distribution/digest"

	"k8s.io/client-go/rest"

	"github.com/openshift/origin/pkg/image/importer"
)

// getImageManifestByIDFromRegistry retrieves the image manifest from the registry using the basic
// authentication using the image ID.
func getImageManifestByIDFromRegistry(registry *url.URL, repositoryName, imageID, username, password string, insecure bool) ([]byte, error) {
	ctx := context.Background()

	credentials := importer.NewBasicCredentials()
	credentials.Add(registry, username, password)

	insecureRT, err := rest.TransportFor(&rest.Config{TLSClientConfig: rest.TLSClientConfig{Insecure: true}})
	if err != nil {
		return nil, err
	}

	repo, err := importer.NewContext(http.DefaultTransport, insecureRT).
		WithCredentials(credentials).
		Repository(ctx, registry, repositoryName, insecure)
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
