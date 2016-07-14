package server

import (
	"encoding/json"
	"strings"

	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/manifest/schema2"

	imageapi "github.com/openshift/origin/pkg/image/api"
)

// Context keys
const (
	// repositoryKey serves to store/retrieve repository object to/from context.
	repositoryKey = "openshift.repository"
)

func WithRepository(parent context.Context, repo *repository) context.Context {
	return context.WithValue(parent, repositoryKey, repo)
}
func RepositoryFrom(ctx context.Context) (repo *repository, found bool) {
	repo, found = ctx.Value(repositoryKey).(*repository)
	return
}

func getBoolOption(name string, defval bool, options map[string]interface{}) bool {
	if value, ok := options[name]; ok {
		var b bool
		if b, ok = value.(bool); ok {
			return b
		}
	}
	return defval
}

// deserializedManifestFromImage converts an Image to a DeserializedManifest.
func deserializedManifestFromImage(image *imageapi.Image) (*schema2.DeserializedManifest, error) {
	var manifest schema2.DeserializedManifest
	if err := json.Unmarshal([]byte(image.DockerImageManifest), &manifest); err != nil {
		return nil, err
	}
	return &manifest, nil
}

func getNamespaceName(resourceName string) (string, string, error) {
	repoParts := strings.SplitN(resourceName, "/", 2)
	if len(repoParts) != 2 {
		return "", "", ErrNamespaceRequired
	}
	ns := repoParts[0]
	if len(ns) == 0 {
		return "", "", ErrNamespaceRequired
	}
	name := repoParts[1]
	if len(name) == 0 {
		return "", "", ErrNamespaceRequired
	}
	return ns, name, nil
}

// effectiveCreateOptions find out what the blob creation options are going to do by dry-running them.
func effectiveCreateOptions(options []distribution.BlobCreateOption) (*distribution.CreateOptions, error) {
	opts := &distribution.CreateOptions{}
	for _, createOptions := range options {
		err := createOptions.Apply(opts)
		if err != nil {
			return nil, err
		}
	}
	return opts, nil
}
