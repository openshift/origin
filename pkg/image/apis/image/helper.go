package image

import (
	"errors"
)

const (
	// DockerDefaultNamespace is the value for namespace when a single segment name is provided.
	DockerDefaultNamespace = "library"
	// DockerDefaultRegistry is the value for the registry when none was provided.
	DockerDefaultRegistry = "docker.io"
	// DockerDefaultV1Registry is the host name of the default v1 registry
	DockerDefaultV1Registry = "index." + DockerDefaultRegistry
	// DockerDefaultV2Registry is the host name of the default v2 registry
	DockerDefaultV2Registry = "registry-1." + DockerDefaultRegistry

	// TagReferenceAnnotationTagHidden indicates that a given TagReference is hidden from search results
	TagReferenceAnnotationTagHidden = "hidden"
)

// ErrImageStreamImportUnsupported is an error client receive when the import
// failed.
var ErrImageStreamImportUnsupported = errors.New("the server does not support directly importing images - create an image stream with tags or the dockerImageRepository field set")

// ErrCircularReference is an error when reference tag is circular.
var ErrCircularReference = errors.New("reference tag is circular")

// ErrNotFoundReference is an error when reference tag is not found.
var ErrNotFoundReference = errors.New("reference tag is not found")

// ErrCrossImageStreamReference is an error when reference tag points to another imagestream.
var ErrCrossImageStreamReference = errors.New("reference tag points to another imagestream")

// ErrInvalidReference is an error when reference tag is invalid.
var ErrInvalidReference = errors.New("reference tag is invalid")
