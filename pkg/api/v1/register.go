package v1

import (
	_ "github.com/openshift/origin/pkg/apps/apis/apps/v1"
	_ "github.com/openshift/origin/pkg/authorization/apis/authorization/v1"
	_ "github.com/openshift/origin/pkg/build/apis/build/v1"
	_ "github.com/openshift/origin/pkg/image/apis/image/v1"
	_ "github.com/openshift/origin/pkg/network/apis/network/v1"
	_ "github.com/openshift/origin/pkg/oauth/apis/oauth/v1"
	_ "github.com/openshift/origin/pkg/project/apis/project/v1"
	_ "github.com/openshift/origin/pkg/route/apis/route/v1"
	_ "github.com/openshift/origin/pkg/security/apis/security/v1"
	_ "github.com/openshift/origin/pkg/template/apis/template/v1"
	_ "github.com/openshift/origin/pkg/user/apis/user/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// SchemeGroupVersion is group version used to register these objects
var SchemeGroupVersion = schema.GroupVersion{Group: "", Version: "v1"}
