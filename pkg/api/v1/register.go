package v1

import (
	_ "github.com/openshift/origin/pkg/authorization/api/v1"
	_ "github.com/openshift/origin/pkg/build/api/v1"
	_ "github.com/openshift/origin/pkg/deploy/api/v1"
	_ "github.com/openshift/origin/pkg/image/api/v1"
	_ "github.com/openshift/origin/pkg/oauth/api/v1"
	_ "github.com/openshift/origin/pkg/project/api/v1"
	_ "github.com/openshift/origin/pkg/route/api/v1"
	_ "github.com/openshift/origin/pkg/sdn/api/v1"
	_ "github.com/openshift/origin/pkg/security/api/v1"
	_ "github.com/openshift/origin/pkg/template/api/v1"
	_ "github.com/openshift/origin/pkg/user/api/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// SchemeGroupVersion is group version used to register these objects
var SchemeGroupVersion = schema.GroupVersion{Group: "", Version: "v1"}
