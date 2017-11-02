package server

import (
	"time"

	"github.com/docker/distribution/context"
	"github.com/openshift/origin/pkg/dockerregistry/server/configuration"
)

type repositoryConfig struct {
	// registryAddr is the host name of the current registry. It may be of the
	// form "host:port" and is used to construct DockerImageReference.
	registryAddr string

	// acceptSchema2 allows to refuse the manifest schema version 2.
	acceptSchema2 bool

	// blobRepositoryCacheTTL is an eviction timeout for <blob belongs to
	// repository> entries of cachedLayers.
	blobRepositoryCacheTTL time.Duration

	// If true, the repository will check remote references in the image
	// stream to support pulling "through" from a remote repository.
	pullthrough bool

	// mirrorPullthrough will mirror remote blobs into the local repository if
	// set.
	mirrorPullthrough bool
}

func newRepositoryConfig(ctx context.Context, cfg *configuration.Configuration, options map[string]interface{}) (rc repositoryConfig, err error) {
	rc.registryAddr = cfg.Server.Addr
	rc.acceptSchema2 = cfg.Compatibility.AcceptSchema2
	rc.blobRepositoryCacheTTL = cfg.Cache.BlobRepositoryTTL
	rc.pullthrough = cfg.Pullthrough.Enabled
	rc.mirrorPullthrough = cfg.Pullthrough.Mirror

	return
}
