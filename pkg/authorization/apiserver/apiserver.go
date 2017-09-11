package apiserver

import (
	"fmt"
	"sync"

	"k8s.io/apimachinery/pkg/apimachinery/registered"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"
	restclient "k8s.io/client-go/rest"
	rbacclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/rbac/internalversion"
	kinternalinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion"
	rbacregistryvalidation "k8s.io/kubernetes/pkg/registry/rbac/validation"

	authorizationapiv1 "github.com/openshift/origin/pkg/authorization/apis/authorization/v1"
	"github.com/openshift/origin/pkg/authorization/authorizer"
	"github.com/openshift/origin/pkg/authorization/registry/clusterrole"
	"github.com/openshift/origin/pkg/authorization/registry/clusterrolebinding"
	"github.com/openshift/origin/pkg/authorization/registry/localresourceaccessreview"
	"github.com/openshift/origin/pkg/authorization/registry/localsubjectaccessreview"
	"github.com/openshift/origin/pkg/authorization/registry/resourceaccessreview"
	"github.com/openshift/origin/pkg/authorization/registry/role"
	"github.com/openshift/origin/pkg/authorization/registry/rolebinding"
	rolebindingrestrictionetcd "github.com/openshift/origin/pkg/authorization/registry/rolebindingrestriction/etcd"
	"github.com/openshift/origin/pkg/authorization/registry/selfsubjectrulesreview"
	"github.com/openshift/origin/pkg/authorization/registry/subjectaccessreview"
	"github.com/openshift/origin/pkg/authorization/registry/subjectrulesreview"
)

type AuthorizationAPIServerConfig struct {
	GenericConfig *genericapiserver.Config

	CoreAPIServerClientConfig *restclient.Config
	KubeInternalInformers     kinternalinformers.SharedInformerFactory
	RuleResolver              rbacregistryvalidation.AuthorizationRuleResolver
	SubjectLocator            authorizer.SubjectLocator

	// TODO these should all become local eventually
	Scheme   *runtime.Scheme
	Registry *registered.APIRegistrationManager
	Codecs   serializer.CodecFactory

	makeV1Storage sync.Once
	v1Storage     map[string]rest.Storage
	v1StorageErr  error
}

type AuthorizationAPIServer struct {
	GenericAPIServer *genericapiserver.GenericAPIServer
}

type completedConfig struct {
	*AuthorizationAPIServerConfig
}

// Complete fills in any fields not set that are required to have valid data. It's mutating the receiver.
func (c *AuthorizationAPIServerConfig) Complete() completedConfig {
	c.GenericConfig.Complete()

	return completedConfig{c}
}

// SkipComplete provides a way to construct a server instance without config completion.
func (c *AuthorizationAPIServerConfig) SkipComplete() completedConfig {
	return completedConfig{c}
}

// New returns a new instance of AuthorizationAPIServer from the given config.
func (c completedConfig) New(delegationTarget genericapiserver.DelegationTarget) (*AuthorizationAPIServer, error) {
	genericServer, err := c.AuthorizationAPIServerConfig.GenericConfig.SkipComplete().New("authorization.openshift.io-apiserver", delegationTarget) // completion is done in Complete, no need for a second time
	if err != nil {
		return nil, err
	}

	s := &AuthorizationAPIServer{
		GenericAPIServer: genericServer,
	}

	v1Storage, err := c.V1RESTStorage()
	if err != nil {
		return nil, err
	}

	apiGroupInfo := genericapiserver.NewDefaultAPIGroupInfo(authorizationapiv1.GroupName, c.Registry, c.Scheme, metav1.ParameterCodec, c.Codecs)
	apiGroupInfo.GroupMeta.GroupVersion = authorizationapiv1.SchemeGroupVersion
	apiGroupInfo.VersionedResourcesStorageMap[authorizationapiv1.SchemeGroupVersion.Version] = v1Storage
	if err := s.GenericAPIServer.InstallAPIGroup(&apiGroupInfo); err != nil {
		return nil, err
	}

	return s, nil
}

func (c *AuthorizationAPIServerConfig) V1RESTStorage() (map[string]rest.Storage, error) {
	c.makeV1Storage.Do(func() {
		c.v1Storage, c.v1StorageErr = c.newV1RESTStorage()
	})

	return c.v1Storage, c.v1StorageErr
}

func (c *AuthorizationAPIServerConfig) newV1RESTStorage() (map[string]rest.Storage, error) {
	rbacClient, err := rbacclient.NewForConfig(c.GenericConfig.LoopbackClientConfig)
	if err != nil {
		return nil, err
	}

	selfSubjectRulesReviewStorage := selfsubjectrulesreview.NewREST(c.RuleResolver, c.KubeInternalInformers.Rbac().InternalVersion().ClusterRoles().Lister())
	subjectRulesReviewStorage := subjectrulesreview.NewREST(c.RuleResolver, c.KubeInternalInformers.Rbac().InternalVersion().ClusterRoles().Lister())
	subjectAccessReviewStorage := subjectaccessreview.NewREST(c.GenericConfig.Authorizer)
	subjectAccessReviewRegistry := subjectaccessreview.NewRegistry(subjectAccessReviewStorage)
	localSubjectAccessReviewStorage := localsubjectaccessreview.NewREST(subjectAccessReviewRegistry)
	resourceAccessReviewStorage := resourceaccessreview.NewREST(c.GenericConfig.Authorizer, c.SubjectLocator)
	resourceAccessReviewRegistry := resourceaccessreview.NewRegistry(resourceAccessReviewStorage)
	localResourceAccessReviewStorage := localresourceaccessreview.NewREST(resourceAccessReviewRegistry)
	roleBindingRestrictionStorage, err := rolebindingrestrictionetcd.NewREST(c.GenericConfig.RESTOptionsGetter)
	if err != nil {
		return nil, fmt.Errorf("error building REST storage: %v", err)
	}

	v1Storage := map[string]rest.Storage{}
	v1Storage["resourceAccessReviews"] = resourceAccessReviewStorage
	v1Storage["subjectAccessReviews"] = subjectAccessReviewStorage
	v1Storage["localSubjectAccessReviews"] = localSubjectAccessReviewStorage
	v1Storage["localResourceAccessReviews"] = localResourceAccessReviewStorage
	v1Storage["selfSubjectRulesReviews"] = selfSubjectRulesReviewStorage
	v1Storage["subjectRulesReviews"] = subjectRulesReviewStorage
	v1Storage["roles"] = role.NewREST(rbacClient.RESTClient())
	v1Storage["roleBindings"] = rolebinding.NewREST(rbacClient.RESTClient())
	v1Storage["clusterRoles"] = clusterrole.NewREST(rbacClient.RESTClient())
	v1Storage["clusterRoleBindings"] = clusterrolebinding.NewREST(rbacClient.RESTClient())
	v1Storage["roleBindingRestrictions"] = roleBindingRestrictionStorage
	return v1Storage, nil
}
