package latest

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"

	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
)

// HACK TO ELIMINATE CYCLE UNTIL WE KILL THIS PACKAGE

var Codec = serializer.NewCodecFactory(configapi.Scheme).LegacyCodec(
	schema.GroupVersion{Group: "", Version: "v1"},
	schema.GroupVersion{Group: "apiserver.k8s.io", Version: "v1alpha1"},
	schema.GroupVersion{Group: "audit.k8s.io", Version: "v1alpha1"},
	schema.GroupVersion{Group: "admission.config.openshift.io", Version: "v1"},
)
