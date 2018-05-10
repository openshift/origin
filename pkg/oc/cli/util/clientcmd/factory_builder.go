package clientcmd

import (
	"k8s.io/apimachinery/pkg/api/meta"
	scaleclient "k8s.io/client-go/scale"
	"k8s.io/kubernetes/pkg/kubectl"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/plugins"
	"k8s.io/kubernetes/pkg/kubectl/resource"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	authorizationclientinternal "github.com/openshift/origin/pkg/authorization/generated/internalclientset"
	oauthclientinternal "github.com/openshift/origin/pkg/oauth/generated/internalclientset"
	"github.com/openshift/origin/pkg/oc/admin/prune/authprune"
	securityclientinternal "github.com/openshift/origin/pkg/security/generated/internalclientset"
	userapi "github.com/openshift/origin/pkg/user/apis/user"
	userclientinternal "github.com/openshift/origin/pkg/user/generated/internalclientset"
)

type ring2Factory struct {
	clientAccessFactory  kcmdutil.ClientAccessFactory
	objectMappingFactory kcmdutil.ObjectMappingFactory
	kubeBuilderFactory   kcmdutil.BuilderFactory
}

func NewBuilderFactory(clientAccessFactory kcmdutil.ClientAccessFactory, objectMappingFactory kcmdutil.ObjectMappingFactory) kcmdutil.BuilderFactory {
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
	clientConfig, err := f.clientAccessFactory.ClientConfig()
	if err != nil {
		return nil, err
	}

	gk := mapping.GroupVersionKind.GroupKind()
	switch gk {
	case authorizationapi.Kind("Role"):
		kubeClient, err := f.clientAccessFactory.KubernetesClientSet()
		if err != nil {
			return nil, err
		}
		return authprune.NewRoleReaper(kubeClient.RbacV1(), kubeClient.RbacV1()), nil
	case authorizationapi.Kind("ClusterRole"):
		kubeClient, err := f.clientAccessFactory.KubernetesClientSet()
		if err != nil {
			return nil, err
		}
		return authprune.NewClusterRoleReaper(kubeClient.RbacV1(), kubeClient.RbacV1(), kubeClient.RbacV1()), nil
	case userapi.Kind("User"):
		userClient, err := userclientinternal.NewForConfig(clientConfig)
		if err != nil {
			return nil, err
		}
		authClient, err := authorizationclientinternal.NewForConfig(clientConfig)
		if err != nil {
			return nil, err
		}
		oauthClient, err := oauthclientinternal.NewForConfig(clientConfig)
		if err != nil {
			return nil, err
		}
		securityClient, err := securityclientinternal.NewForConfig(clientConfig)
		if err != nil {
			return nil, err
		}
		return authprune.NewUserReaper(
			userClient,
			authClient,
			oauthClient,
			securityClient.Security().SecurityContextConstraints(),
		), nil
	case userapi.Kind("Group"):
		userClient, err := userclientinternal.NewForConfig(clientConfig)
		if err != nil {
			return nil, err
		}
		authClient, err := authorizationclientinternal.NewForConfig(clientConfig)
		if err != nil {
			return nil, err
		}
		securityClient, err := securityclientinternal.NewForConfig(clientConfig)
		if err != nil {
			return nil, err
		}
		return authprune.NewGroupReaper(
			userClient,
			authClient,
			securityClient.Security().SecurityContextConstraints(),
		), nil
	}
	return f.kubeBuilderFactory.Reaper(mapping)
}
