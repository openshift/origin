package v1

import (
	_ "github.com/openshift/api/apps/v1"
	_ "github.com/openshift/api/authorization/v1"
	_ "github.com/openshift/api/build/v1"
	_ "github.com/openshift/api/image/v1"
	_ "github.com/openshift/api/network/v1"
	_ "github.com/openshift/api/oauth/v1"
	_ "github.com/openshift/api/project/v1"
	_ "github.com/openshift/api/route/v1"
	_ "github.com/openshift/api/security/v1"
	_ "github.com/openshift/api/template/v1"
	_ "github.com/openshift/api/user/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// SchemeGroupVersion is group version used to register these objects
var SchemeGroupVersion = schema.GroupVersion{Group: "", Version: "v1"}
