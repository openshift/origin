package apiserver

import (
	"sync"

	"k8s.io/apimachinery/pkg/apimachinery/registered"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"
	restclient "k8s.io/client-go/rest"
	kclientsetinternal "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	kinternalinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion"

	securityapiv1 "github.com/openshift/api/security/v1"
	securityinformer "github.com/openshift/origin/pkg/security/generated/informers/internalversion"
	"github.com/openshift/origin/pkg/security/registry/podsecuritypolicyreview"
	"github.com/openshift/origin/pkg/security/registry/podsecuritypolicyselfsubjectreview"
	"github.com/openshift/origin/pkg/security/registry/podsecuritypolicysubjectreview"
	"github.com/openshift/origin/pkg/security/registry/rangeallocations"
	sccstorage "github.com/openshift/origin/pkg/security/registry/securitycontextconstraints/etcd"
	oscc "github.com/openshift/origin/pkg/security/securitycontextconstraints"
)

type ExtraConfig struct {
	KubeAPIServerClientConfig *restclient.Config
	// SCCStorage is actually created with a kubernetes restmapper options to have the correct prefix,
	// so we have to have it special cased here to point to the right spot.
	SCCStorage            *sccstorage.REST
	SecurityInformers     securityinformer.SharedInformerFactory
	KubeInternalInformers kinternalinformers.SharedInformerFactory
	Authorizer            authorizer.Authorizer

	// TODO these should all become local eventually
	Scheme   *runtime.Scheme
	Registry *registered.APIRegistrationManager
	Codecs   serializer.CodecFactory

	makeV1Storage sync.Once
	v1Storage     map[string]rest.Storage
	v1StorageErr  error
}

type SecurityAPIServerConfig struct {
	GenericConfig *genericapiserver.RecommendedConfig
	ExtraConfig   ExtraConfig
}

type SecurityAPIServer struct {
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
func (c *SecurityAPIServerConfig) Complete() completedConfig {
	cfg := completedConfig{
		c.GenericConfig.Complete(),
		&c.ExtraConfig,
	}

	return cfg
}

// New returns a new instance of SecurityAPIServer from the given config.
func (c completedConfig) New(delegationTarget genericapiserver.DelegationTarget) (*SecurityAPIServer, error) {
	genericServer, err := c.GenericConfig.New("security.openshift.io-apiserver", delegationTarget)
	if err != nil {
		return nil, err
	}

	s := &SecurityAPIServer{
		GenericAPIServer: genericServer,
	}

	v1Storage, err := c.V1RESTStorage()
	if err != nil {
		return nil, err
	}

	apiGroupInfo := genericapiserver.NewDefaultAPIGroupInfo(securityapiv1.GroupName, c.ExtraConfig.Registry, c.ExtraConfig.Scheme, metav1.ParameterCodec, c.ExtraConfig.Codecs)
	apiGroupInfo.GroupMeta.GroupVersion = securityapiv1.SchemeGroupVersion
	apiGroupInfo.VersionedResourcesStorageMap[securityapiv1.SchemeGroupVersion.Version] = v1Storage
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
	kubeInternalClient, err := kclientsetinternal.NewForConfig(c.ExtraConfig.KubeAPIServerClientConfig)
	if err != nil {
		return nil, err
	}

	sccStorage := c.ExtraConfig.SCCStorage
	// TODO allow this when we're sure that its storing correctly and we want to allow starting up without embedding kube
	if false && sccStorage == nil {
		sccStorage = sccstorage.NewREST(c.GenericConfig.RESTOptionsGetter)
	}
	sccMatcher := oscc.NewDefaultSCCMatcher(c.ExtraConfig.SecurityInformers.Security().InternalVersion().SecurityContextConstraints().Lister(), c.ExtraConfig.Authorizer)
	podSecurityPolicyReviewStorage := podsecuritypolicyreview.NewREST(
		sccMatcher,
		c.ExtraConfig.KubeInternalInformers.Core().InternalVersion().ServiceAccounts().Lister(),
		kubeInternalClient,
	)
	podSecurityPolicySubjectStorage := podsecuritypolicysubjectreview.NewREST(
		sccMatcher,
		kubeInternalClient,
	)
	podSecurityPolicySelfSubjectReviewStorage := podsecuritypolicyselfsubjectreview.NewREST(
		sccMatcher,
		kubeInternalClient,
	)
	uidRangeStorage := rangeallocations.NewREST(c.GenericConfig.RESTOptionsGetter)

	v1Storage := map[string]rest.Storage{}
	v1Storage["securityContextConstraints"] = sccStorage
	v1Storage["podSecurityPolicyReviews"] = podSecurityPolicyReviewStorage
	v1Storage["podSecurityPolicySubjectReviews"] = podSecurityPolicySubjectStorage
	v1Storage["podSecurityPolicySelfSubjectReviews"] = podSecurityPolicySelfSubjectReviewStorage
	v1Storage["rangeallocations"] = uidRangeStorage
	return v1Storage, nil
}
