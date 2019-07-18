package apiserver

import (
	"fmt"
	"strings"
	"sync"

	"k8s.io/klog"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	authorizationclient "k8s.io/client-go/kubernetes/typed/authorization/v1"
	restclient "k8s.io/client-go/rest"

	projectapiv1 "github.com/openshift/api/project/v1"
	projectv1client "github.com/openshift/client-go/project/clientset/versioned"
	templatev1client "github.com/openshift/client-go/template/clientset/versioned"
	projectproxy "github.com/openshift/openshift-apiserver/pkg/project/apiserver/registry/project/proxy"
	projectrequeststorage "github.com/openshift/openshift-apiserver/pkg/project/apiserver/registry/projectrequest/delegated"
	projectauth "github.com/openshift/openshift-apiserver/pkg/project/auth"
	projectcache "github.com/openshift/openshift-apiserver/pkg/project/cache"
)

type ExtraConfig struct {
	KubeAPIServerClientConfig *restclient.Config
	ProjectAuthorizationCache *projectauth.AuthorizationCache
	ProjectCache              *projectcache.ProjectCache
	ProjectRequestTemplate    string
	ProjectRequestMessage     string
	RESTMapper                meta.RESTMapper

	// TODO these should all become local eventually
	Scheme *runtime.Scheme
	Codecs serializer.CodecFactory

	makeV1Storage sync.Once
	v1Storage     map[string]rest.Storage
	v1StorageErr  error
}

type ProjectAPIServerConfig struct {
	GenericConfig *genericapiserver.RecommendedConfig
	ExtraConfig   ExtraConfig
}

// ProjectAPIServer contains state for a Kubernetes cluster master/api server.
type ProjectAPIServer struct {
	GenericAPIServer *genericapiserver.GenericAPIServer
}

type completedConfig struct {
	GenericConfig genericapiserver.CompletedConfig
	ExtraConfig   *ExtraConfig
}

type CompletedConfig struct {
	// Embed a private pointer that cannot be instantiated outside of this package.
	*completedConfig
}

// Complete fills in any fields not set that are required to have valid data. It's mutating the receiver.
func (c *ProjectAPIServerConfig) Complete() completedConfig {
	cfg := completedConfig{
		c.GenericConfig.Complete(),
		&c.ExtraConfig,
	}

	return cfg
}

// New returns a new instance of ProjectAPIServer from the given config.
func (c completedConfig) New(delegationTarget genericapiserver.DelegationTarget) (*ProjectAPIServer, error) {
	genericServer, err := c.GenericConfig.New("project.openshift.io-apiserver", delegationTarget)
	if err != nil {
		return nil, err
	}

	s := &ProjectAPIServer{
		GenericAPIServer: genericServer,
	}

	v1Storage, err := c.V1RESTStorage()
	if err != nil {
		return nil, err
	}

	apiGroupInfo := genericapiserver.NewDefaultAPIGroupInfo(projectapiv1.GroupName, c.ExtraConfig.Scheme, metav1.ParameterCodec, c.ExtraConfig.Codecs)
	apiGroupInfo.VersionedResourcesStorageMap[projectapiv1.SchemeGroupVersion.Version] = v1Storage
	if err := s.GenericAPIServer.InstallAPIGroup(&apiGroupInfo); err != nil {
		return nil, err
	}

	return s, nil
}

func (c *completedConfig) V1RESTStorage() (map[string]rest.Storage, error) {
	c.ExtraConfig.makeV1Storage.Do(func() {
		c.ExtraConfig.v1Storage, c.ExtraConfig.v1StorageErr = c.newV1RESTStorage()
	})

	return c.ExtraConfig.v1Storage, c.ExtraConfig.v1StorageErr
}

func (c *completedConfig) newV1RESTStorage() (map[string]rest.Storage, error) {
	kubeClient, err := kubernetes.NewForConfig(c.ExtraConfig.KubeAPIServerClientConfig)
	if err != nil {
		return nil, err
	}
	projectClient, err := projectv1client.NewForConfig(c.ExtraConfig.KubeAPIServerClientConfig)
	if err != nil {
		return nil, err
	}
	templateClient, err := templatev1client.NewForConfig(c.ExtraConfig.KubeAPIServerClientConfig)
	if err != nil {
		return nil, err
	}
	authorizationClient, err := authorizationclient.NewForConfig(c.ExtraConfig.KubeAPIServerClientConfig)
	if err != nil {
		return nil, err
	}
	dynamicClient, err := dynamic.NewForConfig(c.ExtraConfig.KubeAPIServerClientConfig)
	if err != nil {
		return nil, err
	}

	projectStorage := projectproxy.NewREST(kubeClient.CoreV1().Namespaces(), c.ExtraConfig.ProjectAuthorizationCache, c.ExtraConfig.ProjectAuthorizationCache, c.ExtraConfig.ProjectCache)

	namespace, templateName, err := parseNamespaceAndName(c.ExtraConfig.ProjectRequestTemplate)
	if err != nil {
		klog.Errorf("Error parsing project request template value: %v", err)
		// we can continue on, the storage that gets created will be valid, it simply won't work properly.  There's no reason to kill the master
	}

	projectRequestStorage := projectrequeststorage.NewREST(
		c.ExtraConfig.ProjectRequestMessage,
		namespace, templateName,
		projectClient.ProjectV1(),
		templateClient,
		authorizationClient.SubjectAccessReviews(),
		dynamicClient,
		c.ExtraConfig.RESTMapper,
		c.GenericConfig.SharedInformerFactory.Rbac().V1().RoleBindings().Lister(),
	)

	v1Storage := map[string]rest.Storage{}
	v1Storage["projects"] = projectStorage
	v1Storage["projectRequests"] = projectRequestStorage
	return v1Storage, nil
}

// parseNamespaceAndName returns back the namespace and name (empty if something goes wrong), for a given string.
// This is useful when pointing to a particular resource inside of our config.
func parseNamespaceAndName(in string) (string, string, error) {
	if len(in) == 0 {
		return "", "", nil
	}

	tokens := strings.Split(in, "/")
	if len(tokens) != 2 {
		return "", "", fmt.Errorf("expected input in the form <namespace>/<resource-name>, not: %v", in)
	}

	return tokens[0], tokens[1], nil
}
