package newapp

import (
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/openshift/api"
	"github.com/openshift/api/apps"
	"github.com/openshift/api/authorization"
	"github.com/openshift/api/build"
	"github.com/openshift/api/image"
	"github.com/openshift/api/network"
	"github.com/openshift/api/oauth"
	"github.com/openshift/api/project"
	"github.com/openshift/api/quota"
	"github.com/openshift/api/route"
	"github.com/openshift/api/security"
	"github.com/openshift/api/template"
	"github.com/openshift/api/user"
)

// we need a scheme that contains ONLY groupped APIs for newapp
var newAppScheme = runtime.NewScheme()

func init() {
	utilruntime.Must(api.InstallKube(newAppScheme))

	utilruntime.Must(apps.Install(newAppScheme))
	utilruntime.Must(authorization.Install(newAppScheme))
	utilruntime.Must(build.Install(newAppScheme))
	utilruntime.Must(image.Install(newAppScheme))
	utilruntime.Must(network.Install(newAppScheme))
	utilruntime.Must(oauth.Install(newAppScheme))
	utilruntime.Must(project.Install(newAppScheme))
	utilruntime.Must(quota.Install(newAppScheme))
	utilruntime.Must(route.Install(newAppScheme))
	utilruntime.Must(security.Install(newAppScheme))
	utilruntime.Must(template.Install(newAppScheme))
	utilruntime.Must(user.Install(newAppScheme))
}
