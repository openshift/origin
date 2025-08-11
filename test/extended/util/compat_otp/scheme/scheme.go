package scheme

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

// Scheme is the default instance of runtime.Scheme to which types in the Kubernetes API are already registered.
var Scheme = runtime.NewScheme()

// Codecs provides access to encoding and decoding for the scheme
var Codecs = serializer.NewCodecFactory(Scheme)

// DefaultJSONEncoder returns a default encoder for our scheme
func DefaultJSONEncoder() runtime.Encoder {
	return unstructured.NewJSONFallbackEncoder(Codecs.LegacyCodec(Scheme.PrioritizedVersionsAllGroups()...))
}
