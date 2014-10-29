package v1beta1

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

	_ "github.com/openshift/origin/pkg/build/api/v1beta1"
	_ "github.com/openshift/origin/pkg/config/api/v1beta1"
	_ "github.com/openshift/origin/pkg/deploy/api/v1beta1"
	_ "github.com/openshift/origin/pkg/image/api/v1beta1"
	_ "github.com/openshift/origin/pkg/oauth/api/v1beta1"
	_ "github.com/openshift/origin/pkg/project/api/v1beta1"
	_ "github.com/openshift/origin/pkg/route/api/v1beta1"
	_ "github.com/openshift/origin/pkg/template/api/v1beta1"
	_ "github.com/openshift/origin/pkg/user/api/v1beta1"
)

// Codec encodes internal objects to the v1beta1 scheme
var Codec = runtime.CodecFor(api.Scheme, "v1beta1")

func init() {
	api.Scheme.AddKnownTypes("v1beta1")
}
