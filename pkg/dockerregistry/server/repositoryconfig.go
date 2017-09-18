package server

import (
	"fmt"
	"os"
	"time"

	"github.com/docker/distribution/context"
)

// Environment variables.
const (
	// DockerRegistryURLEnvVar is a mandatory environment variable name specifying url of internal docker
	// registry. All references to pushed images will be prefixed with its value.
	// DEPRECATED: Use the OPENSHIFT_DEFAULT_REGISTRY instead.
	DockerRegistryURLEnvVar = "DOCKER_REGISTRY_URL"

	// DockerRegistryURLEnvVarOption is an optional environment that overrides the
	// DOCKER_REGISTRY_URL.
	DockerRegistryURLEnvVarOption = "REGISTRY_MIDDLEWARE_REPOSITORY_OPENSHIFT_DOCKERREGISTRYURL"

	// OpenShiftDefaultRegistry overrides the DockerRegistryURLEnvVar as in OpenShift the
	// default registry URL is controller by this environment variable.
	OpenShiftDefaultRegistryEnvVar = "OPENSHIFT_DEFAULT_REGISTRY"

	// EnforceQuotaEnvVar is a boolean environment variable that allows to turn quota enforcement on or off.
	// By default, quota enforcement is off. It overrides openshift middleware configuration option.
	// Recognized values are "true" and "false".
	EnforceQuotaEnvVar = "REGISTRY_MIDDLEWARE_REPOSITORY_OPENSHIFT_ENFORCEQUOTA"

	// ProjectCacheTTLEnvVar is an environment variable specifying an eviction timeout for project quota
	// objects. It takes a valid time duration string (e.g. "2m"). If empty, you get the default timeout. If
	// zero (e.g. "0m"), caching is disabled.
	ProjectCacheTTLEnvVar = "REGISTRY_MIDDLEWARE_REPOSITORY_OPENSHIFT_PROJECTCACHETTL"

	// AcceptSchema2EnvVar is a boolean environment variable that allows to accept manifest schema v2
	// on manifest put requests.
	AcceptSchema2EnvVar = "REGISTRY_MIDDLEWARE_REPOSITORY_OPENSHIFT_ACCEPTSCHEMA2"

	// BlobRepositoryCacheTTLEnvVar  is an environment variable specifying an eviction timeout for <blob
	// belongs to repository> entries. The higher the value, the faster queries but also a higher risk of
	// leaking a blob that is no longer tagged in given repository.
	BlobRepositoryCacheTTLEnvVar = "REGISTRY_MIDDLEWARE_REPOSITORY_OPENSHIFT_BLOBREPOSITORYCACHETTL"

	// Pullthrough is a boolean environment variable that controls whether pullthrough is enabled.
	PullthroughEnvVar = "REGISTRY_MIDDLEWARE_REPOSITORY_OPENSHIFT_PULLTHROUGH"

	// MirrorPullthrough is a boolean environment variable that controls mirroring of blobs on pullthrough.
	MirrorPullthroughEnvVar = "REGISTRY_MIDDLEWARE_REPOSITORY_OPENSHIFT_MIRRORPULLTHROUGH"
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

func newRepositoryConfig(ctx context.Context, options map[string]interface{}) (rc repositoryConfig, err error) {
	// TODO: Deprecate this environment variable.
	rc.registryAddr = os.Getenv(DockerRegistryURLEnvVar)
	if len(rc.registryAddr) == 0 {
		rc.registryAddr = os.Getenv(OpenShiftDefaultRegistryEnvVar)
	} else {
		context.GetLogger(ctx).Infof("DEPRECATED: %q is deprecated, use the %q instead", DockerRegistryURLEnvVar, OpenShiftDefaultRegistryEnvVar)
	}
	if len(rc.registryAddr) == 0 {
		rc.registryAddr, err = getStringOption(DockerRegistryURLEnvVarOption, "dockerregistryurl", rc.registryAddr, options)
		if err != nil {
			return
		}
	}

	// TODO: This is a fallback to assuming there is a service named 'docker-registry'. This
	// might change in the future and we should make this configurable.
	if len(rc.registryAddr) == 0 {
		if len(os.Getenv("DOCKER_REGISTRY_SERVICE_HOST")) > 0 && len(os.Getenv("DOCKER_REGISTRY_SERVICE_PORT")) > 0 {
			rc.registryAddr = os.Getenv("DOCKER_REGISTRY_SERVICE_HOST") + ":" + os.Getenv("DOCKER_REGISTRY_SERVICE_PORT")
		} else {
			return rc, fmt.Errorf("%s variable must be set when running outside of Kubernetes cluster", DockerRegistryURLEnvVar)
		}
	}

	rc.acceptSchema2, err = getBoolOption(AcceptSchema2EnvVar, "acceptschema2", true, options)
	if err != nil {
		return
	}
	rc.blobRepositoryCacheTTL, err = getDurationOption(BlobRepositoryCacheTTLEnvVar, "blobrepositorycachettl", defaultBlobRepositoryCacheTTL, options)
	if err != nil {
		return
	}
	rc.pullthrough, err = getBoolOption(PullthroughEnvVar, "pullthrough", true, options)
	if err != nil {
		return
	}
	rc.mirrorPullthrough, err = getBoolOption(MirrorPullthroughEnvVar, "mirrorpullthrough", true, options)
	return
}
