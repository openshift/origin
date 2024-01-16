package tlsmetadatainterfaces

import (
	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphapi"
)

const UnknownOwner = "Unknown"

func AnnotationValue(whitelistedAnnotations []certgraphapi.AnnotationValue, key string) (string, bool) {
	for _, curr := range whitelistedAnnotations {
		if curr.Key == key {
			return curr.Value, true
		}
	}

	return "", false
}
