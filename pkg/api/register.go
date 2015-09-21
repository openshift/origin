package api

import (
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/runtime"

	_ "github.com/openshift/origin/pkg/authorization/api"
	_ "github.com/openshift/origin/pkg/build/api"
	_ "github.com/openshift/origin/pkg/deploy/api"
	_ "github.com/openshift/origin/pkg/image/api"
	_ "github.com/openshift/origin/pkg/oauth/api"
	_ "github.com/openshift/origin/pkg/project/api"
	_ "github.com/openshift/origin/pkg/route/api"
	_ "github.com/openshift/origin/pkg/sdn/api"
	_ "github.com/openshift/origin/pkg/security/policy/api"
	_ "github.com/openshift/origin/pkg/template/api"
	_ "github.com/openshift/origin/pkg/user/api"
)

// Codec is the identity codec for this package - it can only convert itself
// to itself.
var Codec = runtime.CodecFor(api.Scheme, "")

func init() {
	api.Scheme.AddKnownTypes("")
}
