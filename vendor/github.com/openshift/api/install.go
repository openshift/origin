package api

import (
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/openshift/api/apps"
	"github.com/openshift/api/authorization"
	"github.com/openshift/api/build"
	"github.com/openshift/api/config"
	"github.com/openshift/api/image"
	"github.com/openshift/api/network"
	"github.com/openshift/api/oauth"
	"github.com/openshift/api/operator"
	"github.com/openshift/api/project"
	"github.com/openshift/api/quota"
	"github.com/openshift/api/route"
	"github.com/openshift/api/security"
	"github.com/openshift/api/servicecertsigner"
	"github.com/openshift/api/template"
	"github.com/openshift/api/user"
	"github.com/openshift/api/webconsole"
)

var (
	schemeBuilder = runtime.NewSchemeBuilder(
		apps.Install,
		authorization.Install,
		build.Install,
		config.Install,
		image.Install,
		network.Install,
		oauth.Install,
		operator.Install,
		project.Install,
		quota.Install,
		route.Install,
		security.Install,
		servicecertsigner.Install,
		template.Install,
		user.Install,
		webconsole.Install,
	)
	// Install is a function which adds every version of every openshift group to a scheme
	Install = schemeBuilder.AddToScheme
)
