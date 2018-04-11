package clientcmd

import (
	"k8s.io/apimachinery/pkg/api/meta"
	scaleclient "k8s.io/client-go/scale"
	"k8s.io/kubernetes/pkg/kubectl"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/plugins"
	"k8s.io/kubernetes/pkg/kubectl/resource"

	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	appsclient "github.com/openshift/origin/pkg/apps/generated/internalclientset"
	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	authorizationreaper "github.com/openshift/origin/pkg/authorization/reaper"
	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	buildclient "github.com/openshift/origin/pkg/build/generated/internalclientset"
	buildcmd "github.com/openshift/origin/pkg/oc/cli/builds"
	deploymentcmd "github.com/openshift/origin/pkg/oc/cli/deploymentconfigs"
	userapi "github.com/openshift/origin/pkg/user/apis/user"
	authenticationreaper "github.com/openshift/origin/pkg/user/reaper"
)

type ring2Factory struct {
	clientAccessFactory  ClientAccessFactory
	objectMappingFactory kcmdutil.ObjectMappingFactory
	kubeBuilderFactory   kcmdutil.BuilderFactory
}

func NewBuilderFactory(clientAccessFactory ClientAccessFactory, objectMappingFactory kcmdutil.ObjectMappingFactory) kcmdutil.BuilderFactory {
	return &ring2Factory{
		clientAccessFactory:  clientAccessFactory,
		objectMappingFactory: objectMappingFactory,
		kubeBuilderFactory:   kcmdutil.NewBuilderFactory(clientAccessFactory, objectMappingFactory),
	}
}

// NewBuilder returns a new resource builder for structured api objects.
func (f *ring2Factory) NewBuilder() *resource.Builder {
	return f.kubeBuilderFactory.NewBuilder()
}

// PluginLoader loads plugins from a path set by the KUBECTL_PLUGINS_PATH env var.
// If this env var is not set, it defaults to
//   "~/.kube/plugins", plus
//  "./kubectl/plugins" directory under the "data dir" directory specified by the XDG
// system directory structure spec for the given platform.
func (f *ring2Factory) PluginLoader() plugins.PluginLoader {
	return f.kubeBuilderFactory.PluginLoader()
}

func (f *ring2Factory) PluginRunner() plugins.PluginRunner {
	return f.kubeBuilderFactory.PluginRunner()
}

func (f *ring2Factory) ScaleClient() (scaleclient.ScalesGetter, error) {
	return f.kubeBuilderFactory.ScaleClient()
}

func (f *ring2Factory) Scaler() (kubectl.Scaler, error) {
	return f.kubeBuilderFactory.Scaler()
}

func (f *ring2Factory) Reaper(mapping *meta.RESTMapping) (kubectl.Reaper, error) {
	gk := mapping.GroupVersionKind.GroupKind()
	switch {
	case appsapi.IsKindOrLegacy("DeploymentConfig", gk):
		kc, err := f.clientAccessFactory.ClientSet()
		if err != nil {
			return nil, err
		}
		config, err := f.clientAccessFactory.OpenShiftClientConfig().ClientConfig()
		if err != nil {
			return nil, err
		}
		scaleClient, err := f.ScaleClient()
		if err != nil {
			return nil, err
		}
		return deploymentcmd.NewDeploymentConfigReaper(appsclient.NewForConfigOrDie(config), kc, scaleClient), nil
	case authorizationapi.IsKindOrLegacy("Role", gk):
		authClient, err := f.clientAccessFactory.OpenshiftInternalAuthorizationClient()
		if err != nil {
			return nil, err
		}
		return authorizationreaper.NewRoleReaper(authClient.Authorization(), authClient.Authorization()), nil
	case authorizationapi.IsKindOrLegacy("ClusterRole", gk):
		authClient, err := f.clientAccessFactory.OpenshiftInternalAuthorizationClient()
		if err != nil {
			return nil, err
		}
		return authorizationreaper.NewClusterRoleReaper(authClient.Authorization(), authClient.Authorization(), authClient.Authorization()), nil
	case userapi.IsKindOrLegacy("User", gk):
		userClient, err := f.clientAccessFactory.OpenshiftInternalUserClient()
		if err != nil {
			return nil, err
		}
		authClient, err := f.clientAccessFactory.OpenshiftInternalAuthorizationClient()
		if err != nil {
			return nil, err
		}
		oauthClient, err := f.clientAccessFactory.OpenshiftInternalOAuthClient()
		if err != nil {
			return nil, err
		}
		securityClient, err := f.clientAccessFactory.OpenshiftInternalSecurityClient()
		if err != nil {
			return nil, err
		}
		return authenticationreaper.NewUserReaper(
			userClient,
			userClient,
			authClient,
			authClient,
			oauthClient,
			securityClient.Security().SecurityContextConstraints(),
		), nil
	case userapi.IsKindOrLegacy("Group", gk):
		userClient, err := f.clientAccessFactory.OpenshiftInternalUserClient()
		if err != nil {
			return nil, err
		}
		authClient, err := f.clientAccessFactory.OpenshiftInternalAuthorizationClient()
		if err != nil {
			return nil, err
		}
		securityClient, err := f.clientAccessFactory.OpenshiftInternalSecurityClient()
		if err != nil {
			return nil, err
		}
		return authenticationreaper.NewGroupReaper(
			userClient,
			authClient,
			authClient,
			securityClient.Security().SecurityContextConstraints(),
		), nil
	case buildapi.IsKindOrLegacy("BuildConfig", gk):
		config, err := f.clientAccessFactory.OpenShiftClientConfig().ClientConfig()
		if err != nil {
			return nil, err
		}
		return buildcmd.NewBuildConfigReaper(buildclient.NewForConfigOrDie(config)), nil
	}
	return f.kubeBuilderFactory.Reaper(mapping)
}
