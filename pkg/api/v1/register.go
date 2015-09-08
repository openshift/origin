package v1

import (
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/runtime"

	_ "github.com/openshift/origin/pkg/authorization/api/v1"
	_ "github.com/openshift/origin/pkg/build/api/v1"
	_ "github.com/openshift/origin/pkg/deploy/api/v1"
	_ "github.com/openshift/origin/pkg/image/api/v1"
	_ "github.com/openshift/origin/pkg/oauth/api/v1"
	_ "github.com/openshift/origin/pkg/project/api/v1"
	_ "github.com/openshift/origin/pkg/route/api/v1"
	_ "github.com/openshift/origin/pkg/sdn/api/v1"
	_ "github.com/openshift/origin/pkg/template/api/v1"
	_ "github.com/openshift/origin/pkg/user/api/v1"
)

// Codec encodes internal objects to the v1 scheme
var Codec = runtime.CodecFor(api.Scheme, "v1")

func init() {
}
