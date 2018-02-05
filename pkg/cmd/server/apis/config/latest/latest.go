package latest

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"

	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
)

// HACK TO ELIMINATE CYCLE UNTIL WE KILL THIS PACKAGE

// Version is the string that represents the current external default version.
var Version = schema.GroupVersion{Group: "", Version: "v1"}

// OldestVersion is the string that represents the oldest server version supported,
// for client code that wants to hardcode the lowest common denominator.
var OldestVersion = schema.GroupVersion{Group: "", Version: "v1"}

// Versions is the list of versions that are recognized in code. The order provided
// may be assumed to be least feature rich to most feature rich, and clients may
// choose to prefer the latter items in the list over the former items when presented
// with a set of versions to choose.
var Versions = []schema.GroupVersion{{Group: "", Version: "v1"}}

var Codec = serializer.NewCodecFactory(configapi.Scheme).LegacyCodec(
	schema.GroupVersion{Group: "", Version: "v1"},
	schema.GroupVersion{Group: "apiserver.k8s.io", Version: "v1alpha1"},
	schema.GroupVersion{Group: "audit.k8s.io", Version: "v1alpha1"},
	schema.GroupVersion{Group: "admission.config.openshift.io", Version: "v1"},
)
