package v1

import (
	"k8s.io/kubernetes/pkg/api/unversioned"

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

// SchemeGroupVersion is group version used to register these objects
var SchemeGroupVersion = unversioned.GroupVersion{Group: "", Version: "v1"}
