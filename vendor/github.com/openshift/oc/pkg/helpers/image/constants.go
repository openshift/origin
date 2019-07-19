package image

import (
	"errors"
)

const (

	// DockerDefaultNamespace is the value for namespace when a single segment name is provided.
	DockerDefaultNamespace = "library"

	// TagReferenceAnnotationTagHidden indicates that a given TagReference is hidden from search results
	TagReferenceAnnotationTagHidden = "hidden"
)

var (
	// ErrImageStreamImportUnsupported is an error client receive when the import
	// failed.
	ErrImageStreamImportUnsupported = errors.New("the server does not support directly importing images - create an image stream with tags or the dockerImageRepository field set")

	// ErrCircularReference is an error when reference tag is circular.
	ErrCircularReference = errors.New("reference tag is circular")

	// ErrNotFoundReference is an error when reference tag is not found.
	ErrNotFoundReference = errors.New("reference tag is not found")

	// ErrCrossImageStreamReference is an error when reference tag points to another imagestream.
	ErrCrossImageStreamReference = errors.New("reference tag points to another imagestream")

	// ErrInvalidReference is an error when reference tag is invalid.
	ErrInvalidReference = errors.New("reference tag is invalid")
)
