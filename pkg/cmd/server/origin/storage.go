package origin

import (
	"fmt"
	"time"

	"github.com/golang/glog"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/util/flowcontrol"
	kapi "k8s.io/kubernetes/pkg/api"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	kubeletclient "k8s.io/kubernetes/pkg/kubelet/client"

	authzapiv1 "github.com/openshift/origin/pkg/authorization/apis/authorization/v1"
	"github.com/openshift/origin/pkg/authorization/util"
	buildapiv1 "github.com/openshift/origin/pkg/build/apis/build/v1"
	buildclient "github.com/openshift/origin/pkg/build/client"
	buildgenerator "github.com/openshift/origin/pkg/build/generator"
	buildregistry "github.com/openshift/origin/pkg/build/registry/build"
	buildetcd "github.com/openshift/origin/pkg/build/registry/build/etcd"
	buildconfigregistry "github.com/openshift/origin/pkg/build/registry/buildconfig"
	buildconfigetcd "github.com/openshift/origin/pkg/build/registry/buildconfig/etcd"
	buildlogregistry "github.com/openshift/origin/pkg/build/registry/buildlog"
	"github.com/openshift/origin/pkg/build/webhook"
	"github.com/openshift/origin/pkg/build/webhook/bitbucket"
	"github.com/openshift/origin/pkg/build/webhook/generic"
	"github.com/openshift/origin/pkg/build/webhook/github"
	"github.com/openshift/origin/pkg/build/webhook/gitlab"
	deployapiv1 "github.com/openshift/origin/pkg/deploy/apis/apps/v1"
	deployconfigregistry "github.com/openshift/origin/pkg/deploy/registry/deployconfig"
	deployconfigetcd "github.com/openshift/origin/pkg/deploy/registry/deployconfig/etcd"
	deploylogregistry "github.com/openshift/origin/pkg/deploy/registry/deploylog"
	deployconfiggenerator "github.com/openshift/origin/pkg/deploy/registry/generator"
	deployconfiginstantiate "github.com/openshift/origin/pkg/deploy/registry/instantiate"
	deployrollback "github.com/openshift/origin/pkg/deploy/registry/rollback"
	"github.com/openshift/origin/pkg/dockerregistry"
	imageapiv1 "github.com/openshift/origin/pkg/image/apis/image/v1"
	"github.com/openshift/origin/pkg/image/importer"
	imageimporter "github.com/openshift/origin/pkg/image/importer"
	"github.com/openshift/origin/pkg/image/registry/image"
	imageetcd "github.com/openshift/origin/pkg/image/registry/image/etcd"
	"github.com/openshift/origin/pkg/image/registry/imagesecret"
	"github.com/openshift/origin/pkg/image/registry/imagesignature"
	"github.com/openshift/origin/pkg/image/registry/imagestream"
	imagestreametcd "github.com/openshift/origin/pkg/image/registry/imagestream/etcd"
	"github.com/openshift/origin/pkg/image/registry/imagestreamimage"
	"github.com/openshift/origin/pkg/image/registry/imagestreamimport"
	"github.com/openshift/origin/pkg/image/registry/imagestreammapping"
	"github.com/openshift/origin/pkg/image/registry/imagestreamtag"
	oauthapi "github.com/openshift/origin/pkg/oauth/apis/oauth"
	oauthapiv1 "github.com/openshift/origin/pkg/oauth/apis/oauth/v1"
	accesstokenetcd "github.com/openshift/origin/pkg/oauth/registry/oauthaccesstoken/etcd"
	authorizetokenetcd "github.com/openshift/origin/pkg/oauth/registry/oauthauthorizetoken/etcd"
	clientregistry "github.com/openshift/origin/pkg/oauth/registry/oauthclient"
	clientetcd "github.com/openshift/origin/pkg/oauth/registry/oauthclient/etcd"
	clientauthetcd "github.com/openshift/origin/pkg/oauth/registry/oauthclientauthorization/etcd"
	projectapiv1 "github.com/openshift/origin/pkg/project/apis/project/v1"
	projectproxy "github.com/openshift/origin/pkg/project/registry/project/proxy"
	projectrequeststorage "github.com/openshift/origin/pkg/project/registry/projectrequest/delegated"
	routeapiv1 "github.com/openshift/origin/pkg/route/apis/route/v1"
	routeetcd "github.com/openshift/origin/pkg/route/registry/route/etcd"
	networkapiv1 "github.com/openshift/origin/pkg/sdn/apis/network/v1"
	clusternetworketcd "github.com/openshift/origin/pkg/sdn/registry/clusternetwork/etcd"
	egressnetworkpolicyetcd "github.com/openshift/origin/pkg/sdn/registry/egressnetworkpolicy/etcd"
	hostsubnetetcd "github.com/openshift/origin/pkg/sdn/registry/hostsubnet/etcd"
	netnamespaceetcd "github.com/openshift/origin/pkg/sdn/registry/netnamespace/etcd"
	saoauth "github.com/openshift/origin/pkg/serviceaccounts/oauthclient"
	templateapiv1 "github.com/openshift/origin/pkg/template/apis/template/v1"
	brokertemplateinstanceetcd "github.com/openshift/origin/pkg/template/registry/brokertemplateinstance/etcd"
	templateregistry "github.com/openshift/origin/pkg/template/registry/template"
	templateetcd "github.com/openshift/origin/pkg/template/registry/template/etcd"
	templateinstanceetcd "github.com/openshift/origin/pkg/template/registry/templateinstance/etcd"
	userapiv1 "github.com/openshift/origin/pkg/user/apis/user/v1"
	groupetcd "github.com/openshift/origin/pkg/user/registry/group/etcd"
	identityregistry "github.com/openshift/origin/pkg/user/registry/identity"
	identityetcd "github.com/openshift/origin/pkg/user/registry/identity/etcd"
	userregistry "github.com/openshift/origin/pkg/user/registry/user"
	useretcd "github.com/openshift/origin/pkg/user/registry/user/etcd"
	"github.com/openshift/origin/pkg/user/registry/useridentitymapping"

	"github.com/openshift/origin/pkg/build/registry/buildclone"
	"github.com/openshift/origin/pkg/build/registry/buildconfiginstantiate"

	quotaapiv1 "github.com/openshift/origin/pkg/quota/apis/quota/v1"
	appliedclusterresourcequotaregistry "github.com/openshift/origin/pkg/quota/registry/appliedclusterresourcequota"
	clusterresourcequotaetcd "github.com/openshift/origin/pkg/quota/registry/clusterresourcequota/etcd"

	"github.com/openshift/origin/pkg/api/v1"
	"github.com/openshift/origin/pkg/authorization/registry/localresourceaccessreview"
	"github.com/openshift/origin/pkg/authorization/registry/localsubjectaccessreview"
	"github.com/openshift/origin/pkg/authorization/registry/resourceaccessreview"
	rolebindingrestrictionetcd "github.com/openshift/origin/pkg/authorization/registry/rolebindingrestriction/etcd"
	"github.com/openshift/origin/pkg/authorization/registry/selfsubjectrulesreview"
	"github.com/openshift/origin/pkg/authorization/registry/subjectaccessreview"
	"github.com/openshift/origin/pkg/authorization/registry/subjectrulesreview"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	securityapiv1 "github.com/openshift/origin/pkg/security/apis/security/v1"
	"github.com/openshift/origin/pkg/security/registry/podsecuritypolicyreview"
	"github.com/openshift/origin/pkg/security/registry/podsecuritypolicyselfsubjectreview"
	"github.com/openshift/origin/pkg/security/registry/podsecuritypolicysubjectreview"
	sccstorage "github.com/openshift/origin/pkg/security/registry/securitycontextconstraints/etcd"
	oscc "github.com/openshift/origin/pkg/security/scc"

	// register api groups
	_ "github.com/openshift/origin/pkg/api/install"
)

// TODO this function needs to be broken apart with each API group owning their own storage, probably with two method
// per API group to give us legacy and current storage
func (c OpenshiftAPIConfig) GetRestStorage() (map[schema.GroupVersion]map[string]rest.Storage, error) {
	// TODO sort out who is using this and why.  it was hardcoded before the migration and I suspect that it is being used
	// to serialize out objects into annotations.
	externalVersionCodec := kapi.Codecs.LegacyCodec(schema.GroupVersion{Group: "", Version: "v1"})

	//TODO/REBASE use something other than c.KubeClientsetInternal
	nodeConnectionInfoGetter, err := kubeletclient.NewNodeConnectionInfoGetter(c.KubeClientExternal.CoreV1().Nodes(), *c.KubeletClientConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to configure the node connection info getter: %v", err)
	}

	// TODO: allow the system CAs and the local CAs to be joined together.
	importTransport, err := restclient.TransportFor(&restclient.Config{})
	if err != nil {
		return nil, fmt.Errorf("unable to configure a default transport for importing: %v", err)
	}
	insecureImportTransport, err := restclient.TransportFor(&restclient.Config{
		TLSClientConfig: restclient.TLSClientConfig{
			Insecure: true,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("unable to configure a default transport for importing: %v", err)
	}

	buildStorage, buildDetailsStorage, err := buildetcd.NewREST(c.GenericConfig.RESTOptionsGetter)
	if err != nil {
		return nil, fmt.Errorf("error building REST storage: %v", err)
	}
	buildRegistry := buildregistry.NewRegistry(buildStorage)

	buildConfigStorage, err := buildconfigetcd.NewREST(c.GenericConfig.RESTOptionsGetter)
	if err != nil {
		return nil, fmt.Errorf("error building REST storage: %v", err)
	}
	buildConfigRegistry := buildconfigregistry.NewRegistry(buildConfigStorage)

	deployConfigStorage, deployConfigStatusStorage, deployConfigScaleStorage, err := deployconfigetcd.NewREST(c.GenericConfig.RESTOptionsGetter)

	dcInstantiateStorage := deployconfiginstantiate.NewREST(
		*deployConfigStorage.Store,
		c.DeprecatedOpenshiftClient,
		c.KubeClientInternal,
		externalVersionCodec,
		c.GenericConfig.AdmissionControl,
	)

	if err != nil {
		return nil, fmt.Errorf("error building REST storage: %v", err)
	}
	deployConfigRegistry := deployconfigregistry.NewRegistry(deployConfigStorage)

	hostSubnetStorage, err := hostsubnetetcd.NewREST(c.GenericConfig.RESTOptionsGetter)
	if err != nil {
		return nil, fmt.Errorf("error building REST storage: %v", err)
	}
	netNamespaceStorage, err := netnamespaceetcd.NewREST(c.GenericConfig.RESTOptionsGetter)
	if err != nil {
		return nil, fmt.Errorf("error building REST storage: %v", err)
	}
	clusterNetworkStorage, err := clusternetworketcd.NewREST(c.GenericConfig.RESTOptionsGetter)
	if err != nil {
		return nil, fmt.Errorf("error building REST storage: %v", err)
	}
	egressNetworkPolicyStorage, err := egressnetworkpolicyetcd.NewREST(c.GenericConfig.RESTOptionsGetter)
	if err != nil {
		return nil, fmt.Errorf("error building REST storage: %v", err)
	}

	userStorage, err := useretcd.NewREST(c.GenericConfig.RESTOptionsGetter)
	if err != nil {
		return nil, fmt.Errorf("error building REST storage: %v", err)
	}
	userRegistry := userregistry.NewRegistry(userStorage)
	identityStorage, err := identityetcd.NewREST(c.GenericConfig.RESTOptionsGetter)
	if err != nil {
		return nil, fmt.Errorf("error building REST storage: %v", err)
	}
	identityRegistry := identityregistry.NewRegistry(identityStorage)
	userIdentityMappingStorage := useridentitymapping.NewREST(userRegistry, identityRegistry)
	groupStorage, err := groupetcd.NewREST(c.GenericConfig.RESTOptionsGetter)
	if err != nil {
		return nil, fmt.Errorf("error building REST storage: %v", err)
	}

	clusterPolicies := clusterPolicyLister{
		ClusterPolicyLister: c.AuthorizationInformers.Authorization().InternalVersion().ClusterPolicies().Lister(),
		versioner:           c.AuthorizationInformers.Authorization().InternalVersion().ClusterPolicies().Informer(),
	}
	selfSubjectRulesReviewStorage := selfsubjectrulesreview.NewREST(c.RuleResolver, clusterPolicies)
	subjectRulesReviewStorage := subjectrulesreview.NewREST(c.RuleResolver, clusterPolicies)

	authStorage, err := util.GetAuthorizationStorage(c.GenericConfig.RESTOptionsGetter, c.RuleResolver)
	if err != nil {
		return nil, fmt.Errorf("error building authorization REST storage: %v", err)
	}

	subjectAccessReviewStorage := subjectaccessreview.NewREST(c.GenericConfig.Authorizer)
	subjectAccessReviewRegistry := subjectaccessreview.NewRegistry(subjectAccessReviewStorage)
	localSubjectAccessReviewStorage := localsubjectaccessreview.NewREST(subjectAccessReviewRegistry)
	resourceAccessReviewStorage := resourceaccessreview.NewREST(c.GenericConfig.Authorizer, c.SubjectLocator)
	resourceAccessReviewRegistry := resourceaccessreview.NewRegistry(resourceAccessReviewStorage)
	localResourceAccessReviewStorage := localresourceaccessreview.NewREST(resourceAccessReviewRegistry)

	sccStorage := c.SCCStorage
	// TODO allow this when we're sure that its storing correctly and we want to allow starting up without embedding kube
	if false && sccStorage == nil {
		sccStorage = sccstorage.NewREST(c.GenericConfig.RESTOptionsGetter)
	}
	podSecurityPolicyReviewStorage := podsecuritypolicyreview.NewREST(
		oscc.NewDefaultSCCMatcher(c.SecurityInformers.Security().InternalVersion().SecurityContextConstraints().Lister()),
		c.KubeInternalInformers.Core().InternalVersion().ServiceAccounts().Lister(),
		c.KubeClientInternal,
	)
	podSecurityPolicySubjectStorage := podsecuritypolicysubjectreview.NewREST(
		oscc.NewDefaultSCCMatcher(c.SecurityInformers.Security().InternalVersion().SecurityContextConstraints().Lister()),
		c.KubeClientInternal,
	)
	podSecurityPolicySelfSubjectReviewStorage := podsecuritypolicyselfsubjectreview.NewREST(
		oscc.NewDefaultSCCMatcher(c.SecurityInformers.Security().InternalVersion().SecurityContextConstraints().Lister()),
		c.KubeClientInternal,
	)

	imageStorage, err := imageetcd.NewREST(c.GenericConfig.RESTOptionsGetter)
	if err != nil {
		return nil, fmt.Errorf("error building REST storage: %v", err)
	}
	imageRegistry := image.NewRegistry(imageStorage)
	imageSignatureStorage := imagesignature.NewREST(c.DeprecatedOpenshiftClient.Images())
	imageStreamSecretsStorage := imagesecret.NewREST(c.KubeClientInternal.Core())
	imageStreamStorage, imageStreamStatusStorage, internalImageStreamStorage, err := imagestreametcd.NewREST(c.GenericConfig.RESTOptionsGetter, c.RegistryNameFn, subjectAccessReviewRegistry, c.LimitVerifier)
	if err != nil {
		return nil, fmt.Errorf("error building REST storage: %v", err)
	}
	imageStreamRegistry := imagestream.NewRegistry(imageStreamStorage, imageStreamStatusStorage, internalImageStreamStorage)
	imageStreamMappingStorage := imagestreammapping.NewREST(imageRegistry, imageStreamRegistry, c.RegistryNameFn)
	imageStreamTagStorage := imagestreamtag.NewREST(imageRegistry, imageStreamRegistry)
	imageStreamTagRegistry := imagestreamtag.NewRegistry(imageStreamTagStorage)
	importerCache, err := imageimporter.NewImageStreamLayerCache(imageimporter.DefaultImageStreamLayerCacheSize)
	if err != nil {
		return nil, fmt.Errorf("error building REST storage: %v", err)
	}
	importerFn := func(r importer.RepositoryRetriever) imageimporter.Interface {
		return imageimporter.NewImageStreamImporter(r, c.MaxImagesBulkImportedPerRepository, flowcontrol.NewTokenBucketRateLimiter(2.0, 3), &importerCache)
	}
	importerDockerClientFn := func() dockerregistry.Client {
		return dockerregistry.NewClient(20*time.Second, false)
	}
	imageStreamImportStorage := imagestreamimport.NewREST(
		importerFn,
		imageStreamRegistry,
		internalImageStreamStorage,
		imageStorage,
		c.DeprecatedOpenshiftClient,
		importTransport,
		insecureImportTransport,
		importerDockerClientFn,
		c.AllowedRegistriesForImport,
		c.RegistryNameFn,
		c.DeprecatedOpenshiftClient.SubjectAccessReviews())
	imageStreamImageStorage := imagestreamimage.NewREST(imageRegistry, imageStreamRegistry)
	imageStreamImageRegistry := imagestreamimage.NewRegistry(imageStreamImageStorage)

	routeStorage, routeStatusStorage, err := routeetcd.NewREST(c.GenericConfig.RESTOptionsGetter, c.RouteAllocator, subjectAccessReviewRegistry)
	if err != nil {
		return nil, fmt.Errorf("error building REST storage: %v", err)
	}

	buildGenerator := &buildgenerator.BuildGenerator{
		Client: buildgenerator.Client{
			GetBuildConfigFunc:      buildConfigRegistry.GetBuildConfig,
			UpdateBuildConfigFunc:   buildConfigRegistry.UpdateBuildConfig,
			GetBuildFunc:            buildRegistry.GetBuild,
			CreateBuildFunc:         buildRegistry.CreateBuild,
			UpdateBuildFunc:         buildRegistry.UpdateBuild,
			GetImageStreamFunc:      imageStreamRegistry.GetImageStream,
			GetImageStreamImageFunc: imageStreamImageRegistry.GetImageStreamImage,
			GetImageStreamTagFunc:   imageStreamTagRegistry.GetImageStreamTag,
		},
		ServiceAccounts: c.KubeClientInternal.Core(),
		Secrets:         c.KubeClientInternal.Core(),
	}

	// TODO: with sharding, this needs to be changed
	deployConfigGenerator := &deployconfiggenerator.DeploymentConfigGenerator{
		Client: deployconfiggenerator.Client{
			DCFn:   deployConfigRegistry.GetDeploymentConfig,
			ISFn:   imageStreamRegistry.GetImageStream,
			LISFn2: imageStreamRegistry.ListImageStreams,
		},
	}
	deployRollbackClient := deployrollback.Client{
		DCFn: deployConfigRegistry.GetDeploymentConfig,
		RCFn: clientDeploymentInterface{c.KubeClientInternal}.GetDeployment,
		GRFn: deployrollback.NewRollbackGenerator().GenerateRollback,
	}
	deployConfigRollbackStorage := deployrollback.NewREST(c.DeprecatedOpenshiftClient, c.KubeClientInternal, externalVersionCodec)

	projectStorage := projectproxy.NewREST(c.KubeClientInternal.Core().Namespaces(), c.ProjectAuthorizationCache, c.ProjectAuthorizationCache, c.ProjectCache)

	namespace, templateName, err := configapi.ParseNamespaceAndName(c.ProjectRequestTemplate)
	if err != nil {
		glog.Errorf("Error parsing project request template value: %v", err)
		// we can continue on, the storage that gets created will be valid, it simply won't work properly.  There's no reason to kill the master
	}

	policyBindings := policyBindingLister{
		PolicyBindingLister: c.AuthorizationInformers.Authorization().InternalVersion().PolicyBindings().Lister(),
		versioner:           c.AuthorizationInformers.Authorization().InternalVersion().PolicyBindings().Informer(),
	}
	projectRequestStorage := projectrequeststorage.NewREST(c.ProjectRequestMessage, namespace, templateName, c.DeprecatedOpenshiftClient, c.GenericConfig.LoopbackClientConfig, policyBindings)

	buildConfigWebHooks := buildconfigregistry.NewWebHookREST(
		buildConfigRegistry,
		buildclient.NewOSClientBuildConfigInstantiatorClient(c.DeprecatedOpenshiftClient),
		// We use the buildapiv1 schemegroup to encode the Build that gets
		// returned. As such, we need to make sure that the GroupVersion we use
		// is the same API version that the storage is going to be used for.
		buildapiv1.SchemeGroupVersion,
		map[string]webhook.Plugin{
			"generic":   generic.New(),
			"github":    github.New(),
			"gitlab":    gitlab.New(),
			"bitbucket": bitbucket.New(),
		},
	)

	clientStorage, err := clientetcd.NewREST(c.GenericConfig.RESTOptionsGetter)
	if err != nil {
		return nil, fmt.Errorf("error building REST storage: %v", err)
	}
	clientRegistry := clientregistry.NewRegistry(clientStorage)

	// If OAuth is disabled, set the strategy to Deny
	saAccountGrantMethod := oauthapi.GrantHandlerDeny
	if len(c.ServiceAccountMethod) > 0 {
		// Otherwise, take the value provided in master-config.yaml
		saAccountGrantMethod = oauthapi.GrantHandlerType(c.ServiceAccountMethod)
	}

	combinedOAuthClientGetter := saoauth.NewServiceAccountOAuthClientGetter(c.KubeClientInternal.Core(), c.KubeClientInternal.Core(), c.DeprecatedOpenshiftClient, clientRegistry, saAccountGrantMethod)
	authorizeTokenStorage, err := authorizetokenetcd.NewREST(c.GenericConfig.RESTOptionsGetter, combinedOAuthClientGetter)
	if err != nil {
		return nil, fmt.Errorf("error building REST storage: %v", err)
	}
	accessTokenStorage, err := accesstokenetcd.NewREST(c.GenericConfig.RESTOptionsGetter, combinedOAuthClientGetter)
	if err != nil {
		return nil, fmt.Errorf("error building REST storage: %v", err)
	}
	clientAuthorizationStorage, err := clientauthetcd.NewREST(c.GenericConfig.RESTOptionsGetter, combinedOAuthClientGetter)
	if err != nil {
		return nil, fmt.Errorf("error building REST storage: %v", err)
	}

	templateStorage, err := templateetcd.NewREST(c.GenericConfig.RESTOptionsGetter)
	if err != nil {
		return nil, fmt.Errorf("error building REST storage: %v", err)
	}

	clusterResourceQuotaStorage, clusterResourceQuotaStatusStorage, err := clusterresourcequotaetcd.NewREST(c.GenericConfig.RESTOptionsGetter)
	if err != nil {
		return nil, fmt.Errorf("error building REST storage: %v", err)
	}
	roleBindingRestrictionStorage, err := rolebindingrestrictionetcd.NewREST(c.GenericConfig.RESTOptionsGetter)
	if err != nil {
		return nil, fmt.Errorf("error building REST storage: %v", err)
	}

	storage := map[schema.GroupVersion]map[string]rest.Storage{
		v1.SchemeGroupVersion: {
			// TODO: Deprecate these
			"generateDeploymentConfigs": deployconfiggenerator.NewREST(deployConfigGenerator, externalVersionCodec),
			"deploymentConfigRollbacks": deployrollback.NewDeprecatedREST(deployRollbackClient, externalVersionCodec),
		},
	}

	storage[quotaapiv1.SchemeGroupVersion] = map[string]rest.Storage{
		"clusterResourceQuotas":        clusterResourceQuotaStorage,
		"clusterResourceQuotas/status": clusterResourceQuotaStatusStorage,
		"appliedClusterResourceQuotas": appliedclusterresourcequotaregistry.NewREST(
			c.ClusterQuotaMappingController.GetClusterQuotaMapper(),
			c.QuotaInformers.Quota().InternalVersion().ClusterResourceQuotas().Lister(),
			c.KubeInternalInformers.Core().InternalVersion().Namespaces().Lister(),
		),
	}

	storage[networkapiv1.SchemeGroupVersion] = map[string]rest.Storage{
		"hostSubnets":           hostSubnetStorage,
		"netNamespaces":         netNamespaceStorage,
		"clusterNetworks":       clusterNetworkStorage,
		"egressNetworkPolicies": egressNetworkPolicyStorage,
	}

	storage[userapiv1.SchemeGroupVersion] = map[string]rest.Storage{
		"users":                userStorage,
		"groups":               groupStorage,
		"identities":           identityStorage,
		"userIdentityMappings": userIdentityMappingStorage,
	}

	storage[oauthapiv1.SchemeGroupVersion] = map[string]rest.Storage{
		"oAuthAuthorizeTokens":      authorizeTokenStorage,
		"oAuthAccessTokens":         accessTokenStorage,
		"oAuthClients":              clientStorage,
		"oAuthClientAuthorizations": clientAuthorizationStorage,
	}

	storage[authzapiv1.SchemeGroupVersion] = map[string]rest.Storage{
		"resourceAccessReviews":      resourceAccessReviewStorage,
		"subjectAccessReviews":       subjectAccessReviewStorage,
		"localSubjectAccessReviews":  localSubjectAccessReviewStorage,
		"localResourceAccessReviews": localResourceAccessReviewStorage,
		"selfSubjectRulesReviews":    selfSubjectRulesReviewStorage,
		"subjectRulesReviews":        subjectRulesReviewStorage,

		"policies":       authStorage.Policy,
		"policyBindings": authStorage.PolicyBinding,
		"roles":          authStorage.Role,
		"roleBindings":   authStorage.RoleBinding,

		"clusterPolicies":       authStorage.ClusterPolicy,
		"clusterPolicyBindings": authStorage.ClusterPolicyBinding,
		"clusterRoleBindings":   authStorage.ClusterRoleBinding,
		"clusterRoles":          authStorage.ClusterRole,

		"roleBindingRestrictions": roleBindingRestrictionStorage,
	}

	storage[securityapiv1.SchemeGroupVersion] = map[string]rest.Storage{
		"securityContextConstraints":          sccStorage,
		"podSecurityPolicyReviews":            podSecurityPolicyReviewStorage,
		"podSecurityPolicySubjectReviews":     podSecurityPolicySubjectStorage,
		"podSecurityPolicySelfSubjectReviews": podSecurityPolicySelfSubjectReviewStorage,
	}

	storage[projectapiv1.SchemeGroupVersion] = map[string]rest.Storage{
		"projects":        projectStorage,
		"projectRequests": projectRequestStorage,
	}

	storage[deployapiv1.SchemeGroupVersion] = map[string]rest.Storage{
		"deploymentConfigs":             deployConfigStorage,
		"deploymentConfigs/scale":       deployConfigScaleStorage,
		"deploymentConfigs/status":      deployConfigStatusStorage,
		"deploymentConfigs/rollback":    deployConfigRollbackStorage,
		"deploymentConfigs/log":         deploylogregistry.NewREST(c.DeprecatedOpenshiftClient, c.KubeClientInternal.Core(), c.KubeClientInternal.Core(), nodeConnectionInfoGetter),
		"deploymentConfigs/instantiate": dcInstantiateStorage,
	}

	storage[templateapiv1.SchemeGroupVersion] = map[string]rest.Storage{
		"processedTemplates": templateregistry.NewREST(),
		"templates":          templateStorage,
	}

	storage[imageapiv1.SchemeGroupVersion] = map[string]rest.Storage{
		"images":               imageStorage,
		"imagesignatures":      imageSignatureStorage,
		"imageStreams/secrets": imageStreamSecretsStorage,
		"imageStreams":         imageStreamStorage,
		"imageStreams/status":  imageStreamStatusStorage,
		"imageStreamImports":   imageStreamImportStorage,
		"imageStreamImages":    imageStreamImageStorage,
		"imageStreamMappings":  imageStreamMappingStorage,
		"imageStreamTags":      imageStreamTagStorage,
	}

	storage[routeapiv1.SchemeGroupVersion] = map[string]rest.Storage{
		"routes":        routeStorage,
		"routes/status": routeStatusStorage,
	}

	if c.EnableTemplateServiceBroker {
		templateInstanceStorage, templateInstanceStatusStorage, err := templateinstanceetcd.NewREST(c.GenericConfig.RESTOptionsGetter, c.KubeClientInternal)
		if err != nil {
			return nil, fmt.Errorf("error building REST storage: %v", err)
		}
		brokerTemplateInstanceStorage, err := brokertemplateinstanceetcd.NewREST(c.GenericConfig.RESTOptionsGetter)
		if err != nil {
			return nil, fmt.Errorf("error building REST storage: %v", err)
		}

		storage[templateapiv1.SchemeGroupVersion]["templateinstances"] = templateInstanceStorage
		storage[templateapiv1.SchemeGroupVersion]["templateinstances/status"] = templateInstanceStatusStorage
		storage[templateapiv1.SchemeGroupVersion]["brokertemplateinstances"] = brokerTemplateInstanceStorage
	}

	if c.EnableBuilds {
		storage[buildapiv1.SchemeGroupVersion] = map[string]rest.Storage{
			"builds":         buildStorage,
			"builds/clone":   buildclone.NewStorage(buildGenerator),
			"builds/log":     buildlogregistry.NewREST(buildStorage, buildStorage, c.KubeClientInternal.Core(), nodeConnectionInfoGetter),
			"builds/details": buildDetailsStorage,

			"buildConfigs":                   buildConfigStorage,
			"buildConfigs/webhooks":          buildConfigWebHooks,
			"buildConfigs/instantiate":       buildconfiginstantiate.NewStorage(buildGenerator),
			"buildConfigs/instantiatebinary": buildconfiginstantiate.NewBinaryStorage(buildGenerator, buildStorage, c.KubeClientInternal.Core(), nodeConnectionInfoGetter),
		}
	}

	return storage, nil
}

// TODO, this shoudl be removed
type clientDeploymentInterface struct {
	KubeClient kclientset.Interface
}

// GetDeployment returns the deployment with the provided context and name
func (c clientDeploymentInterface) GetDeployment(ctx apirequest.Context, name string, options *metav1.GetOptions) (*kapi.ReplicationController, error) {
	opts := metav1.GetOptions{}
	if options != nil {
		opts = *options
	}
	return c.KubeClient.Core().ReplicationControllers(apirequest.NamespaceValue(ctx)).Get(name, opts)
}
