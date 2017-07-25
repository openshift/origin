package origin

import (
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path"
	"reflect"
	"strings"
	"time"

	"github.com/golang/glog"

	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/admission/initializer"
	"k8s.io/apiserver/pkg/admission/plugin/namespace/lifecycle"
	"k8s.io/apiserver/pkg/authentication/authenticator"
	"k8s.io/apiserver/pkg/authentication/group"
	"k8s.io/apiserver/pkg/authentication/request/anonymous"
	"k8s.io/apiserver/pkg/authentication/request/bearertoken"
	"k8s.io/apiserver/pkg/authentication/request/headerrequest"
	"k8s.io/apiserver/pkg/authentication/request/union"
	"k8s.io/apiserver/pkg/authentication/request/websocket"
	x509request "k8s.io/apiserver/pkg/authentication/request/x509"
	"k8s.io/apiserver/pkg/authentication/user"
	kauthorizer "k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/authorization/authorizerfactory"
	authorizerunion "k8s.io/apiserver/pkg/authorization/union"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	kubeclientgoinformers "k8s.io/client-go/informers"
	kubeclientgoclient "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/util/cert"
	kapi "k8s.io/kubernetes/pkg/api"
	kclientsetexternal "k8s.io/kubernetes/pkg/client/clientset_generated/clientset"
	kclientsetinternal "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	kinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/externalversions"
	kinternalinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion"
	kcontroller "k8s.io/kubernetes/pkg/controller"
	sacontroller "k8s.io/kubernetes/pkg/controller/serviceaccount"
	kadmission "k8s.io/kubernetes/pkg/kubeapiserver/admission"
	kubeletclient "k8s.io/kubernetes/pkg/kubelet/client"
	"k8s.io/kubernetes/pkg/serviceaccount"
	scheduleroptions "k8s.io/kubernetes/plugin/cmd/kube-scheduler/app/options"
	saadmit "k8s.io/kubernetes/plugin/pkg/admission/serviceaccount"
	serviceaccountadmission "k8s.io/kubernetes/plugin/pkg/admission/serviceaccount"
	storageclassdefaultadmission "k8s.io/kubernetes/plugin/pkg/admission/storageclass/setdefault"
	schedulerapi "k8s.io/kubernetes/plugin/pkg/scheduler/api"
	latestschedulerapi "k8s.io/kubernetes/plugin/pkg/scheduler/api/latest"

	"github.com/openshift/origin/pkg/auth/authenticator/request/paramtoken"
	authnregistry "github.com/openshift/origin/pkg/auth/oauth/registry"
	"github.com/openshift/origin/pkg/auth/userregistry/identitymapper"
	"github.com/openshift/origin/pkg/authorization/authorizer"
	"github.com/openshift/origin/pkg/authorization/authorizer/scope"
	authorizationinformer "github.com/openshift/origin/pkg/authorization/generated/informers/internalversion"
	authorizationinternalinformer "github.com/openshift/origin/pkg/authorization/generated/informers/internalversion/authorization/internalversion"
	"github.com/openshift/origin/pkg/authorization/rulevalidation"
	buildinformer "github.com/openshift/origin/pkg/build/generated/informers/internalversion"
	osclient "github.com/openshift/origin/pkg/client"
	oadmission "github.com/openshift/origin/pkg/cmd/server/admission"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	"github.com/openshift/origin/pkg/cmd/server/crypto"
	kubernetes "github.com/openshift/origin/pkg/cmd/server/kubernetes/master"
	kubecontrollers "github.com/openshift/origin/pkg/cmd/server/kubernetes/master/controller"
	admissionregistry "github.com/openshift/origin/pkg/cmd/server/origin/admission"
	origincontrollers "github.com/openshift/origin/pkg/cmd/server/origin/controller"
	originrest "github.com/openshift/origin/pkg/cmd/server/origin/rest"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	cmdflags "github.com/openshift/origin/pkg/cmd/util/flags"
	"github.com/openshift/origin/pkg/cmd/util/pluginconfig"
	"github.com/openshift/origin/pkg/cmd/util/variable"
	appinformer "github.com/openshift/origin/pkg/deploy/generated/informers/internalversion"
	imageadmission "github.com/openshift/origin/pkg/image/admission"
	imagepolicy "github.com/openshift/origin/pkg/image/admission/imagepolicy/api"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	imageinformer "github.com/openshift/origin/pkg/image/generated/informers/internalversion"
	ingressadmission "github.com/openshift/origin/pkg/ingress/admission"
	accesstokenregistry "github.com/openshift/origin/pkg/oauth/registry/oauthaccesstoken"
	accesstokenetcd "github.com/openshift/origin/pkg/oauth/registry/oauthaccesstoken/etcd"
	projectauth "github.com/openshift/origin/pkg/project/auth"
	projectcache "github.com/openshift/origin/pkg/project/cache"
	"github.com/openshift/origin/pkg/quota"
	overrideapi "github.com/openshift/origin/pkg/quota/admission/clusterresourceoverride/api"
	"github.com/openshift/origin/pkg/quota/controller/clusterquotamapping"
	quotainformer "github.com/openshift/origin/pkg/quota/generated/informers/internalversion"

	securityinformer "github.com/openshift/origin/pkg/security/generated/informers/internalversion"
	"github.com/openshift/origin/pkg/service"
	serviceadmit "github.com/openshift/origin/pkg/service/admission"
	templateinformer "github.com/openshift/origin/pkg/template/generated/informers/internalversion"
	usercache "github.com/openshift/origin/pkg/user/cache"
	groupregistry "github.com/openshift/origin/pkg/user/registry/group"
	groupstorage "github.com/openshift/origin/pkg/user/registry/group/etcd"
	userregistry "github.com/openshift/origin/pkg/user/registry/user"
	useretcd "github.com/openshift/origin/pkg/user/registry/user/etcd"
	"github.com/openshift/origin/pkg/util/restoptions"
)

// MasterConfig defines the required parameters for starting the OpenShift master
type MasterConfig struct {
	Options configapi.MasterConfig

	// RESTOptionsGetter provides access to storage and RESTOptions for a particular resource
	RESTOptionsGetter restoptions.Getter

	RuleResolver   rulevalidation.AuthorizationRuleResolver
	Authenticator  authenticator.Request
	Authorizer     kauthorizer.Authorizer
	SubjectLocator authorizer.SubjectLocator

	// TODO(sttts): replace AuthorizationAttributeBuilder with apiserverfilters.NewRequestAttributeGetter
	AuthorizationAttributeBuilder authorizer.AuthorizationAttributeBuilder

	GroupCache                    *usercache.GroupCache
	ProjectAuthorizationCache     *projectauth.AuthorizationCache
	ProjectCache                  *projectcache.ProjectCache
	ClusterQuotaMappingController *clusterquotamapping.ClusterQuotaMappingController
	LimitVerifier                 imageadmission.LimitVerifier

	// RequestContextMapper maps requests to contexts
	RequestContextMapper apirequest.RequestContextMapper

	AdmissionControl admission.Interface

	// KubeAdmissionControl holds the kube admission chain.  Because of the way the plugin initializer is built
	// you'll be passing information in this direction either way.  Knowing how to build this chain requires knowledge
	// of both the origin config AND the kube config, so this spot makes more sense.
	KubeAdmissionControl admission.Interface

	TLS bool

	// ImageFor is a function that returns the appropriate image to use for a named component
	ImageFor func(component string) string
	// RegistryNameFn retrieves the name of the integrated registry, or false if no such registry
	// is available.
	RegistryNameFn imageapi.DefaultRegistryFunc

	// ExternalVersionCodec is the codec used when serializing annotations, which cannot be changed
	// without all clients being aware of the new version.
	ExternalVersionCodec runtime.Codec

	KubeletClientConfig *kubeletclient.KubeletClientConfig

	// ClientCAs will be used to request client certificates in connections to the API.
	// This CertPool should contain all the CAs that will be used for client certificate verification.
	ClientCAs *x509.CertPool
	// APIClientCAs is used to verify client certificates presented for API auth
	APIClientCAs *x509.CertPool

	// PrivilegedLoopbackClientConfig is the client configuration used to call OpenShift APIs from system components
	// To apply different access control to a system component, create a client config specifically for that component.
	PrivilegedLoopbackClientConfig restclient.Config

	// PrivilegedLoopbackKubernetesClientsetInternal is the client used to call Kubernetes APIs from system components,
	// built from KubeClientConfig. It should only be accessed via the *Client() helper methods. To apply
	// different access control to a system component, create a separate client/config specifically for
	// that component.
	PrivilegedLoopbackKubernetesClientsetInternal kclientsetinternal.Interface
	// PrivilegedLoopbackKubernetesClientsetExternal is the client used to call Kubernetes APIs from system components,
	// built from KubeClientConfig. It should only be accessed via the *Client() helper methods. To apply
	// different access control to a system component, create a separate client/config specifically for
	// that component.
	PrivilegedLoopbackKubernetesClientsetExternal kclientsetexternal.Interface
	// PrivilegedLoopbackOpenShiftClient is the client used to call OpenShift APIs from system components,
	// built from PrivilegedLoopbackClientConfig. It should only be accessed via the *Client() helper methods.
	// To apply different access control to a system component, create a separate client/config specifically
	// for that component.
	PrivilegedLoopbackOpenShiftClient *osclient.Client

	// TODO inspect uses to eliminate them
	InternalKubeInformers  kinternalinformers.SharedInformerFactory
	ExternalKubeInformers  kinformers.SharedInformerFactory
	ClientGoKubeInformers  kubeclientgoinformers.SharedInformerFactory
	AuthorizationInformers authorizationinformer.SharedInformerFactory
	AppInformers           appinformer.SharedInformerFactory
	BuildInformers         buildinformer.SharedInformerFactory
	ImageInformers         imageinformer.SharedInformerFactory
	QuotaInformers         quotainformer.SharedInformerFactory
	SecurityInformers      securityinformer.SharedInformerFactory
	TemplateInformers      templateinformer.SharedInformerFactory
}

type InformerAccess interface {
	GetInternalKubeInformers() kinternalinformers.SharedInformerFactory
	GetExternalKubeInformers() kinformers.SharedInformerFactory
	GetClientGoKubeInformers() kubeclientgoinformers.SharedInformerFactory
	GetAuthorizationInformers() authorizationinformer.SharedInformerFactory
	GetAppInformers() appinformer.SharedInformerFactory
	GetBuildInformers() buildinformer.SharedInformerFactory
	GetImageInformers() imageinformer.SharedInformerFactory
	GetQuotaInformers() quotainformer.SharedInformerFactory
	GetSecurityInformers() securityinformer.SharedInformerFactory
	GetTemplateInformers() templateinformer.SharedInformerFactory
}

// BuildMasterConfig builds and returns the OpenShift master configuration based on the
// provided options
func BuildMasterConfig(options configapi.MasterConfig, informers InformerAccess) (*MasterConfig, error) {
	restOptsGetter, err := originrest.StorageOptions(options)
	if err != nil {
		return nil, err
	}

	clientCAs, err := configapi.GetClientCertCAPool(options)
	if err != nil {
		return nil, err
	}
	apiClientCAs, err := configapi.GetAPIClientCertCAPool(options)
	if err != nil {
		return nil, err
	}

	privilegedLoopbackKubeClientsetInternal, _, err := configapi.GetInternalKubeClient(options.MasterClients.OpenShiftLoopbackKubeConfig, options.MasterClients.OpenShiftLoopbackClientConnectionOverrides)
	if err != nil {
		return nil, err
	}
	privilegedLoopbackKubeClientsetExternal, _, err := configapi.GetExternalKubeClient(options.MasterClients.OpenShiftLoopbackKubeConfig, options.MasterClients.OpenShiftLoopbackClientConnectionOverrides)
	if err != nil {
		return nil, err
	}
	privilegedLoopbackOpenShiftClient, privilegedLoopbackClientConfig, err := configapi.GetOpenShiftClient(options.MasterClients.OpenShiftLoopbackKubeConfig, options.MasterClients.OpenShiftLoopbackClientConnectionOverrides)
	if err != nil {
		return nil, err
	}
	kubeClientGoClientSet, err := kubeclientgoclient.NewForConfig(privilegedLoopbackClientConfig)
	if err != nil {
		return nil, err
	}

	imageTemplate := variable.NewDefaultImageTemplate()
	imageTemplate.Format = options.ImageConfig.Format
	imageTemplate.Latest = options.ImageConfig.Latest

	defaultRegistry := env("OPENSHIFT_DEFAULT_REGISTRY", "${DOCKER_REGISTRY_SERVICE_HOST}:${DOCKER_REGISTRY_SERVICE_PORT}")
	svcCache := service.NewServiceResolverCache(privilegedLoopbackKubeClientsetInternal.Core().Services(metav1.NamespaceDefault).Get)
	defaultRegistryFunc, err := svcCache.Defer(defaultRegistry)
	if err != nil {
		return nil, fmt.Errorf("OPENSHIFT_DEFAULT_REGISTRY variable is invalid %q: %v", defaultRegistry, err)
	}

	requestContextMapper := apirequest.NewRequestContextMapper()

	groupStorage, err := groupstorage.NewREST(restOptsGetter)
	if err != nil {
		return nil, err
	}
	groupCache := usercache.NewGroupCache(groupregistry.NewRegistry(groupStorage))
	projectCache := projectcache.NewProjectCache(
		informers.GetInternalKubeInformers().Core().InternalVersion().Namespaces().Informer(),
		privilegedLoopbackKubeClientsetInternal.Core().Namespaces(),
		options.ProjectConfig.DefaultNodeSelector)
	clusterQuotaMappingController := clusterquotamapping.NewClusterQuotaMappingControllerInternal(
		informers.GetInternalKubeInformers().Core().InternalVersion().Namespaces(),
		informers.GetQuotaInformers().Quota().InternalVersion().ClusterResourceQuotas())

	kubeletClientConfig := configapi.GetKubeletClientConfig(options)

	quotaRegistry := quota.NewAllResourceQuotaRegistryForAdmission(
		informers.GetExternalKubeInformers(),
		informers.GetImageInformers().Image().InternalVersion().ImageStreams(),
		privilegedLoopbackOpenShiftClient,
		privilegedLoopbackKubeClientsetExternal)
	ruleResolver := rulevalidation.NewDefaultRuleResolver(
		informers.GetAuthorizationInformers().Authorization().InternalVersion().Policies().Lister(),
		informers.GetAuthorizationInformers().Authorization().InternalVersion().PolicyBindings().Lister(),
		informers.GetAuthorizationInformers().Authorization().InternalVersion().ClusterPolicies().Lister(),
		informers.GetAuthorizationInformers().Authorization().InternalVersion().ClusterPolicyBindings().Lister(),
	)
	authorizer, subjectLocator := newAuthorizer(
		ruleResolver,
		informers.GetAuthorizationInformers().Authorization().InternalVersion(),
		options.ProjectConfig.ProjectRequestMessage)

	// punch through layers to build this in order to get a string for a cloud provider file
	// TODO refactor us into a forward building flow with a side channel like this
	kubeOptions, err := kubernetes.BuildKubeAPIserverOptions(options)
	if err != nil {
		return nil, err
	}

	var cloudConfig []byte
	if kubeOptions.CloudProvider.CloudConfigFile != "" {
		var err error
		cloudConfig, err = ioutil.ReadFile(kubeOptions.CloudProvider.CloudConfigFile)
		if err != nil {
			return nil, fmt.Errorf("Error reading from cloud configuration file %s: %v", kubeOptions.CloudProvider.CloudConfigFile, err)
		}
	}
	// note: we are passing a combined quota registry here...
	genericInitializer, err := initializer.New(kubeClientGoClientSet, informers.GetClientGoKubeInformers(), authorizer)
	if err != nil {
		return nil, err
	}
	kubePluginInitializer := kadmission.NewPluginInitializer(
		privilegedLoopbackKubeClientsetInternal,
		privilegedLoopbackKubeClientsetExternal,
		informers.GetInternalKubeInformers(),
		authorizer,
		cloudConfig,
		// TODO: use a dynamic restmapper. See https://github.com/kubernetes/kubernetes/pull/42615.
		kapi.Registry.RESTMapper(),
		quotaRegistry)
	openshiftPluginInitializer := &oadmission.PluginInitializer{
		OpenshiftClient:              privilegedLoopbackOpenShiftClient,
		ProjectCache:                 projectCache,
		OriginQuotaRegistry:          quotaRegistry,
		Authorizer:                   authorizer,
		JenkinsPipelineConfig:        options.JenkinsPipelineConfig,
		RESTClientConfig:             *privilegedLoopbackClientConfig,
		Informers:                    informers.GetInternalKubeInformers(),
		ClusterResourceQuotaInformer: informers.GetQuotaInformers().Quota().InternalVersion().ClusterResourceQuotas(),
		ClusterQuotaMapper:           clusterQuotaMappingController.GetClusterQuotaMapper(),
		DefaultRegistryFn:            imageapi.DefaultRegistryFunc(defaultRegistryFunc),
		GroupCache:                   groupCache,
		SecurityInformers:            informers.GetSecurityInformers(),
	}
	initializersChain := admission.PluginInitializers{genericInitializer, kubePluginInitializer, openshiftPluginInitializer}

	originAdmission, kubeAdmission, err := buildAdmissionChains(options, privilegedLoopbackKubeClientsetInternal, initializersChain)
	if err != nil {
		return nil, err
	}

	// this is safe because the server does a quorum read and we're hitting a "magic" authorizer to get permissions based on system:masters
	// once the cache is added, we won't be paying a double hop cost to etcd on each request, so the simplification will help.
	serviceAccountTokenGetter := sacontroller.NewGetterFromClient(privilegedLoopbackKubeClientsetExternal)
	authenticator, err := newAuthenticator(options, restOptsGetter, serviceAccountTokenGetter, apiClientCAs, groupCache)
	if err != nil {
		return nil, err
	}

	config := &MasterConfig{
		Options: options,

		RESTOptionsGetter: restOptsGetter,

		RuleResolver:                  ruleResolver,
		Authenticator:                 authenticator,
		Authorizer:                    authorizer,
		SubjectLocator:                subjectLocator,
		AuthorizationAttributeBuilder: newAuthorizationAttributeBuilder(requestContextMapper),

		GroupCache: groupCache,
		ProjectAuthorizationCache: newProjectAuthorizationCache(
			subjectLocator,
			privilegedLoopbackKubeClientsetInternal,
			informers.GetInternalKubeInformers(),
			informers.GetAuthorizationInformers().Authorization().InternalVersion()),
		ProjectCache:                  projectCache,
		ClusterQuotaMappingController: clusterQuotaMappingController,

		RequestContextMapper: requestContextMapper,

		AdmissionControl:     originAdmission,
		KubeAdmissionControl: kubeAdmission,

		TLS: configapi.UseTLS(options.ServingInfo.ServingInfo),

		ImageFor:       imageTemplate.ExpandOrDie,
		RegistryNameFn: imageapi.DefaultRegistryFunc(defaultRegistryFunc),

		// TODO: migration of versions of resources stored in annotations must be sorted out
		ExternalVersionCodec: kapi.Codecs.LegacyCodec(schema.GroupVersion{Group: "", Version: "v1"}),

		KubeletClientConfig: kubeletClientConfig,

		ClientCAs:    clientCAs,
		APIClientCAs: apiClientCAs,

		PrivilegedLoopbackClientConfig:                *privilegedLoopbackClientConfig,
		PrivilegedLoopbackOpenShiftClient:             privilegedLoopbackOpenShiftClient,
		PrivilegedLoopbackKubernetesClientsetInternal: privilegedLoopbackKubeClientsetInternal,
		PrivilegedLoopbackKubernetesClientsetExternal: privilegedLoopbackKubeClientsetExternal,

		InternalKubeInformers:  informers.GetInternalKubeInformers(),
		ExternalKubeInformers:  informers.GetExternalKubeInformers(),
		ClientGoKubeInformers:  informers.GetClientGoKubeInformers(),
		AppInformers:           informers.GetAppInformers(),
		AuthorizationInformers: informers.GetAuthorizationInformers(),
		BuildInformers:         informers.GetBuildInformers(),
		ImageInformers:         informers.GetImageInformers(),
		QuotaInformers:         informers.GetQuotaInformers(),
		SecurityInformers:      informers.GetSecurityInformers(),
		TemplateInformers:      informers.GetTemplateInformers(),
	}

	// ensure that the limit range informer will be started
	informer := config.InternalKubeInformers.Core().InternalVersion().LimitRanges().Informer()
	config.LimitVerifier = imageadmission.NewLimitVerifier(imageadmission.LimitRangesForNamespaceFunc(func(ns string) ([]*kapi.LimitRange, error) {
		list, err := config.InternalKubeInformers.Core().InternalVersion().LimitRanges().Lister().LimitRanges(ns).List(labels.Everything())
		if err != nil {
			return nil, err
		}
		// the verifier must return an error
		if len(list) == 0 && len(informer.LastSyncResourceVersion()) == 0 {
			glog.V(4).Infof("LimitVerifier still waiting for ranges to load: %#v", informer)
			forbiddenErr := kapierrors.NewForbidden(schema.GroupResource{Resource: "limitranges"}, "", fmt.Errorf("the server is still loading limit information"))
			forbiddenErr.ErrStatus.Details.RetryAfterSeconds = 1
			return nil, forbiddenErr
		}
		return list, nil
	}))

	return config, nil
}

func BuildOpenshiftControllerConfig(options configapi.MasterConfig, informers InformerAccess) (*origincontrollers.OpenshiftControllerConfig, error) {
	var err error
	ret := &origincontrollers.OpenshiftControllerConfig{}

	kubeInternal, loopbackClientConfig, err := configapi.GetInternalKubeClient(options.MasterClients.OpenShiftLoopbackKubeConfig, options.MasterClients.OpenShiftLoopbackClientConnectionOverrides)
	if err != nil {
		return nil, err
	}

	ret.ServiceAccountTokenControllerOptions = origincontrollers.ServiceAccountTokenControllerOptions{
		RootClientBuilder: kcontroller.SimpleControllerClientBuilder{
			ClientConfig: loopbackClientConfig,
		},
	}
	if len(options.ServiceAccountConfig.PrivateKeyFile) > 0 {
		ret.ServiceAccountTokenControllerOptions.PrivateKey, err = serviceaccount.ReadPrivateKey(options.ServiceAccountConfig.PrivateKeyFile)
		if err != nil {
			return nil, fmt.Errorf("error reading signing key for Service Account Token Manager: %v", err)
		}
	}
	if len(options.ServiceAccountConfig.MasterCA) > 0 {
		ret.ServiceAccountTokenControllerOptions.RootCA, err = ioutil.ReadFile(options.ServiceAccountConfig.MasterCA)
		if err != nil {
			return nil, fmt.Errorf("error reading master ca file for Service Account Token Manager: %s: %v", options.ServiceAccountConfig.MasterCA, err)
		}
		if _, err := cert.ParseCertsPEM(ret.ServiceAccountTokenControllerOptions.RootCA); err != nil {
			return nil, fmt.Errorf("error parsing master ca file for Service Account Token Manager: %s: %v", options.ServiceAccountConfig.MasterCA, err)
		}
	}
	if options.ControllerConfig.ServiceServingCert.Signer != nil && len(options.ControllerConfig.ServiceServingCert.Signer.CertFile) > 0 {
		certFile := options.ControllerConfig.ServiceServingCert.Signer.CertFile
		serviceServingCA, err := ioutil.ReadFile(certFile)
		if err != nil {
			return nil, fmt.Errorf("error reading ca file for Service Serving Certificate Signer: %s: %v", certFile, err)
		}
		if _, err := crypto.CertsFromPEM(serviceServingCA); err != nil {
			return nil, fmt.Errorf("error parsing ca file for Service Serving Certificate Signer: %s: %v", certFile, err)
		}

		// if we have a rootCA bundle add that too.  The rootCA will be used when hitting the default master service, since those are signed
		// using a different CA by default.  The rootCA's key is more closely guarded than ours and if it is compromised, that power could
		// be used to change the trusted signers for every pod anyway, so we're already effectively trusting it.
		if len(ret.ServiceAccountTokenControllerOptions.RootCA) > 0 {
			ret.ServiceAccountTokenControllerOptions.ServiceServingCA = append(ret.ServiceAccountTokenControllerOptions.ServiceServingCA, ret.ServiceAccountTokenControllerOptions.RootCA...)
			ret.ServiceAccountTokenControllerOptions.ServiceServingCA = append(ret.ServiceAccountTokenControllerOptions.ServiceServingCA, []byte("\n")...)
		}
		ret.ServiceAccountTokenControllerOptions.ServiceServingCA = append(ret.ServiceAccountTokenControllerOptions.ServiceServingCA, serviceServingCA...)
	}

	ret.ServiceAccountControllerOptions = origincontrollers.ServiceAccountControllerOptions{
		ManagedNames: options.ServiceAccountConfig.ManagedNames,
	}

	storageVersion := options.EtcdStorageConfig.OpenShiftStorageVersion
	groupVersion := schema.GroupVersion{Group: "", Version: storageVersion}
	annotationCodec := kapi.Codecs.LegacyCodec(groupVersion)

	imageTemplate := variable.NewDefaultImageTemplate()
	imageTemplate.Format = options.ImageConfig.Format
	imageTemplate.Latest = options.ImageConfig.Latest

	ret.BuildControllerConfig = origincontrollers.BuildControllerConfig{
		DockerImage:           imageTemplate.ExpandOrDie("docker-builder"),
		STIImage:              imageTemplate.ExpandOrDie("sti-builder"),
		AdmissionPluginConfig: options.AdmissionConfig.PluginConfig,
		Codec: annotationCodec,
	}

	vars, err := getOpenShiftClientEnvVars(options)
	if err != nil {
		return nil, err
	}
	ret.DeployerControllerConfig = origincontrollers.DeployerControllerConfig{
		ImageName:     imageTemplate.ExpandOrDie("deployer"),
		Codec:         annotationCodec,
		ClientEnvVars: vars,
	}
	ret.DeploymentConfigControllerConfig = origincontrollers.DeploymentConfigControllerConfig{
		Codec: annotationCodec,
	}
	ret.DeploymentTriggerControllerConfig = origincontrollers.DeploymentTriggerControllerConfig{
		Codec: annotationCodec,
	}

	ret.ImageTriggerControllerConfig = origincontrollers.ImageTriggerControllerConfig{
		HasBuilderEnabled: options.DisabledFeatures.Has(configapi.FeatureBuilder),
		// TODO: make these consts in configapi
		HasDeploymentsEnabled:  options.DisabledFeatures.Has("triggers.image.openshift.io/deployments"),
		HasDaemonSetsEnabled:   options.DisabledFeatures.Has("triggers.image.openshift.io/daemonsets"),
		HasStatefulSetsEnabled: options.DisabledFeatures.Has("triggers.image.openshift.io/statefulsets"),
		HasCronJobsEnabled:     options.DisabledFeatures.Has("triggers.image.openshift.io/cronjobs"),
	}
	ret.ImageImportControllerConfig = origincontrollers.ImageImportControllerConfig{
		MaxScheduledImageImportsPerMinute:          options.ImagePolicyConfig.MaxScheduledImageImportsPerMinute,
		ResyncPeriod:                               10 * time.Minute,
		DisableScheduledImport:                     options.ImagePolicyConfig.DisableScheduledImport,
		ScheduledImageImportMinimumIntervalSeconds: options.ImagePolicyConfig.ScheduledImageImportMinimumIntervalSeconds,
	}

	ret.ServiceServingCertsControllerOptions = origincontrollers.ServiceServingCertsControllerOptions{
		Signer: options.ControllerConfig.ServiceServingCert.Signer,
	}

	ret.SDNControllerConfig = origincontrollers.SDNControllerConfig{
		NetworkConfig: options.NetworkConfig,
	}
	ret.UnidlingControllerConfig = origincontrollers.UnidlingControllerConfig{
		ResyncPeriod: 2 * time.Hour,
	}
	ret.IngressIPControllerConfig = origincontrollers.IngressIPControllerConfig{
		IngressIPSyncPeriod:  10 * time.Minute,
		IngressIPNetworkCIDR: options.NetworkConfig.IngressIPNetworkCIDR,
	}

	// this needs a privileged client to skip the RBAC escalation checks.
	ret.OriginToRBACSyncControllerConfig = origincontrollers.OriginToRBACSyncControllerConfig{
		PrivilegedRBACClient: kubeInternal.Rbac(),
	}

	// TODO rewire this to accep them during the init function
	clusterQuotaMappingController := clusterquotamapping.NewClusterQuotaMappingController(
		informers.GetExternalKubeInformers().Core().V1().Namespaces(),
		informers.GetQuotaInformers().Quota().InternalVersion().ClusterResourceQuotas())
	ret.ClusterQuotaMappingControllerConfig = origincontrollers.ClusterQuotaMappingControllerConfig{
		ClusterQuotaMappingController: clusterQuotaMappingController,
	}
	ret.ClusterQuotaReconciliationControllerConfig = origincontrollers.ClusterQuotaReconciliationControllerConfig{
		Mapper:                         clusterQuotaMappingController.GetClusterQuotaMapper(),
		DefaultResyncPeriod:            5 * time.Minute,
		DefaultReplenishmentSyncPeriod: 12 * time.Hour,
	}

	return ret, nil
}

func BuildKubeControllerConfig(options configapi.MasterConfig) (*kubecontrollers.KubeControllerConfig, error) {
	var err error
	ret := &kubecontrollers.KubeControllerConfig{}

	kubeExternal, _, err := configapi.GetExternalKubeClient(options.MasterClients.OpenShiftLoopbackKubeConfig, options.MasterClients.OpenShiftLoopbackClientConnectionOverrides)
	if err != nil {
		return nil, err
	}

	var schedulerPolicy *schedulerapi.Policy
	if _, err := os.Stat(options.KubernetesMasterConfig.SchedulerConfigFile); err == nil {
		schedulerPolicy = &schedulerapi.Policy{}
		configData, err := ioutil.ReadFile(options.KubernetesMasterConfig.SchedulerConfigFile)
		if err != nil {
			return nil, fmt.Errorf("unable to read scheduler config: %v", err)
		}
		if err := runtime.DecodeInto(latestschedulerapi.Codec, configData, schedulerPolicy); err != nil {
			return nil, fmt.Errorf("invalid scheduler configuration: %v", err)
		}
	}
	// resolve extended arguments
	// TODO: this should be done in config validation (along with the above) so we can provide
	// proper errors
	schedulerserver := scheduleroptions.NewSchedulerServer()
	schedulerserver.PolicyConfigFile = options.KubernetesMasterConfig.SchedulerConfigFile
	if err := cmdflags.Resolve(options.KubernetesMasterConfig.SchedulerArguments, schedulerserver.AddFlags); len(err) > 0 {
		return nil, kerrors.NewAggregate(err)
	}
	ret.SchedulerControllerConfig = kubecontrollers.SchedulerControllerConfig{
		PrivilegedClient: kubeExternal,
		SchedulerServer:  schedulerserver,
	}

	imageTemplate := variable.NewDefaultImageTemplate()
	imageTemplate.Format = options.ImageConfig.Format
	imageTemplate.Latest = options.ImageConfig.Latest
	ret.RecyclerImage = imageTemplate.ExpandOrDie("recycler")

	ret.HeapsterNamespace = options.PolicyConfig.OpenShiftInfrastructureNamespace

	return ret, nil
}

var (
	// openshiftAdmissionControlPlugins gives the in-order default admission chain for openshift resources.
	openshiftAdmissionControlPlugins = []string{
		"ProjectRequestLimit",
		"OriginNamespaceLifecycle",
		"openshift.io/RestrictSubjectBindings",
		"PodNodeConstraints",
		"openshift.io/JenkinsBootstrapper",
		"openshift.io/BuildConfigSecretInjector",
		"BuildByStrategy",
		imageadmission.PluginName,
		"OwnerReferencesPermissionEnforcement",
		"Initializers",
		"GenericAdmissionWebhook",
		"ResourceQuota",
	}

	// KubeAdmissionPlugins gives the in-order default admission chain for kube resources.
	KubeAdmissionPlugins = []string{
		lifecycle.PluginName,
		"RunOnceDuration",
		"PodNodeConstraints",
		"OriginPodNodeEnvironment",
		"PodNodeSelector",
		overrideapi.PluginName,
		serviceadmit.ExternalIPPluginName,
		serviceadmit.RestrictedEndpointsPluginName,
		imagepolicy.PluginName,
		"ImagePolicyWebhook",
		"PodPreset",
		"LimitRanger",
		"ServiceAccount",
		"SecurityContextConstraint",
		storageclassdefaultadmission.PluginName,
		"AlwaysPullImages",
		"LimitPodHardAntiAffinityTopology",
		"SCCExecRestrictions",
		"PersistentVolumeLabel",
		"OwnerReferencesPermissionEnforcement",
		ingressadmission.IngressAdmission,
		"DefaultTolerationSeconds",
		"Initializers",
		"GenericAdmissionWebhook",
		"NodeRestriction",
		"PodTolerationRestriction",
		// NOTE: ResourceQuota and ClusterResourceQuota must be the last 2 plugins.
		// DO NOT ADD ANY PLUGINS AFTER THIS LINE!
		"ResourceQuota",
		"openshift.io/ClusterResourceQuota",
	}

	// CombinedAdmissionControlPlugins gives the in-order default admission chain for all resources resources.
	// When possible, this list is used.  The set of openshift+kube chains must exactly match this set.  In addition,
	// the order specified in the openshift and kube chains must match the order here.
	CombinedAdmissionControlPlugins = []string{
		lifecycle.PluginName,
		"ProjectRequestLimit",
		"OriginNamespaceLifecycle",
		"openshift.io/RestrictSubjectBindings",
		"PodNodeConstraints",
		"openshift.io/JenkinsBootstrapper",
		"openshift.io/BuildConfigSecretInjector",
		"BuildByStrategy",
		imageadmission.PluginName,
		"RunOnceDuration",
		"PodNodeConstraints",
		"OriginPodNodeEnvironment",
		"PodNodeSelector",
		overrideapi.PluginName,
		serviceadmit.ExternalIPPluginName,
		serviceadmit.RestrictedEndpointsPluginName,
		imagepolicy.PluginName,
		"ImagePolicyWebhook",
		"PodPreset",
		"LimitRanger",
		"ServiceAccount",
		"SecurityContextConstraint",
		storageclassdefaultadmission.PluginName,
		"AlwaysPullImages",
		"LimitPodHardAntiAffinityTopology",
		"SCCExecRestrictions",
		"PersistentVolumeLabel",
		"OwnerReferencesPermissionEnforcement",
		ingressadmission.IngressAdmission,
		"DefaultTolerationSeconds",
		"Initializers",
		"GenericAdmissionWebhook",
		"NodeRestriction",
		"PodTolerationRestriction",
		// NOTE: ResourceQuota and ClusterResourceQuota must be the last 2 plugins.
		// DO NOT ADD ANY PLUGINS AFTER THIS LINE!
		"ResourceQuota",
		"openshift.io/ClusterResourceQuota",
	}
)

// replace returns a slice where each instance of the input that is x is replaced with y
func replace(input []string, x, y string) []string {
	result := []string{}
	for i := range input {
		if input[i] == x {
			result = append(result, y)
		} else {
			result = append(result, input[i])
		}
	}
	return result
}

// dedupe removes duplicate items from the input list.
// the last instance of a duplicate is kept in the input list.
func dedupe(input []string) []string {
	items := sets.NewString()
	result := []string{}
	for i := len(input) - 1; i >= 0; i-- {
		if items.Has(input[i]) {
			continue
		}
		items.Insert(input[i])
		result = append([]string{input[i]}, result...)
	}
	return result
}

// fixupAdmissionPlugins fixes the input plugins to handle deprecation and duplicates.
func fixupAdmissionPlugins(plugins []string) []string {
	result := replace(plugins, "openshift.io/OriginResourceQuota", "ResourceQuota")
	result = dedupe(result)
	return result
}

func buildAdmissionChains(options configapi.MasterConfig, kubeClientSet kclientsetinternal.Interface, admissionInitializer admission.PluginInitializer) (admission.Interface /*origin*/, admission.Interface /*kube*/, error) {
	// check to see if they've taken explicit control of the kube admission chain
	// this happens when any of the following are true:
	// 1. extended kube server args are used to change the admission plugin list
	// 2. kube explicit config changes the admission plugin list
	// 3. extended kube server args are used to change the admission config file
	// 4. openshift explicit config changes the admission plugins list
	// 5. kube and openshift plugin config try to configure the same plugin differently
	// TODO: one release from now, I think we should start failing on setting the kube admission config
	//       two releases from now, I think we should start removing it
	//       two releases from now, I think we should remove the PluginOverrideOrder entirely
	hasSeparateKubeAdmissionChain := false
	KubeAdmissionPlugins := KubeAdmissionPlugins
	if options.KubernetesMasterConfig != nil && len(options.KubernetesMasterConfig.APIServerArguments["admission-control"]) > 0 {
		KubeAdmissionPlugins = strings.Split(options.KubernetesMasterConfig.APIServerArguments["admission-control"][0], ",")
		hasSeparateKubeAdmissionChain = true
	}
	if options.KubernetesMasterConfig != nil && len(options.KubernetesMasterConfig.AdmissionConfig.PluginOrderOverride) > 0 {
		KubeAdmissionPlugins = options.KubernetesMasterConfig.AdmissionConfig.PluginOrderOverride
		hasSeparateKubeAdmissionChain = true
	}
	KubeAdmissionPlugins = fixupAdmissionPlugins(KubeAdmissionPlugins)

	kubeAdmissionPluginConfigFilename := ""
	if options.KubernetesMasterConfig != nil && len(options.KubernetesMasterConfig.APIServerArguments["admission-control-config-file"]) > 0 {
		kubeAdmissionPluginConfigFilename = options.KubernetesMasterConfig.APIServerArguments["admission-control-config-file"][0]
		hasSeparateKubeAdmissionChain = true
	}

	openshiftAdmissionPlugins := openshiftAdmissionControlPlugins
	if len(options.AdmissionConfig.PluginOrderOverride) > 0 {
		openshiftAdmissionPlugins = options.AdmissionConfig.PluginOrderOverride
		hasSeparateKubeAdmissionChain = true
	}
	openshiftAdmissionPlugins = fixupAdmissionPlugins(openshiftAdmissionPlugins)

	if options.KubernetesMasterConfig != nil && !hasSeparateKubeAdmissionChain {
		// check for collisions between openshift and kube plugin config
		for pluginName, kubeConfig := range options.KubernetesMasterConfig.AdmissionConfig.PluginConfig {
			if openshiftConfig, exists := options.AdmissionConfig.PluginConfig[pluginName]; exists && !reflect.DeepEqual(kubeConfig, openshiftConfig) {
				hasSeparateKubeAdmissionChain = true
				break
			}
		}
	}

	if hasSeparateKubeAdmissionChain {
		// build kube admission
		var kubeAdmission admission.Interface
		if options.KubernetesMasterConfig != nil {
			var err error
			kubeAdmission, err = newAdmissionChainFunc(KubeAdmissionPlugins, kubeAdmissionPluginConfigFilename, options.KubernetesMasterConfig.AdmissionConfig.PluginConfig, options, kubeClientSet, admissionInitializer)
			if err != nil {
				return nil, nil, err
			}
		}

		// build openshift admission
		openshiftAdmission, err := newAdmissionChainFunc(openshiftAdmissionPlugins, "", options.AdmissionConfig.PluginConfig, options, kubeClientSet, admissionInitializer)
		if err != nil {
			return nil, nil, err
		}

		return openshiftAdmission, kubeAdmission, nil
	}

	// if we have a unified chain, build the combined config
	pluginConfig := map[string]configapi.AdmissionPluginConfig{}
	if options.KubernetesMasterConfig != nil {
		for pluginName, config := range options.KubernetesMasterConfig.AdmissionConfig.PluginConfig {
			pluginConfig[pluginName] = config
		}
	}
	for pluginName, config := range options.AdmissionConfig.PluginConfig {
		pluginConfig[pluginName] = config
	}

	admissionChain, err := newAdmissionChainFunc(CombinedAdmissionControlPlugins, "", pluginConfig, options, kubeClientSet, admissionInitializer)
	if err != nil {
		return nil, nil, err
	}

	return admissionChain, admissionChain, err
}

// newAdmissionChainFunc is for unit testing only.  You should NEVER OVERRIDE THIS outside of a unit test.
var newAdmissionChainFunc = newAdmissionChain

func newAdmissionChain(pluginNames []string, admissionConfigFilename string, pluginConfig map[string]configapi.AdmissionPluginConfig, options configapi.MasterConfig, kubeClientSet kclientsetinternal.Interface, admissionInitializer admission.PluginInitializer) (admission.Interface, error) {
	plugins := []admission.Interface{}
	for _, pluginName := range pluginNames {
		var (
			plugin             admission.Interface
			skipInitialization bool
		)

		switch pluginName {
		case lifecycle.PluginName:
			// We need to include our infrastructure and shared resource namespaces in the immortal namespaces list
			immortalNamespaces := sets.NewString(metav1.NamespaceDefault)
			if len(options.PolicyConfig.OpenShiftSharedResourcesNamespace) > 0 {
				immortalNamespaces.Insert(options.PolicyConfig.OpenShiftSharedResourcesNamespace)
			}
			if len(options.PolicyConfig.OpenShiftInfrastructureNamespace) > 0 {
				immortalNamespaces.Insert(options.PolicyConfig.OpenShiftInfrastructureNamespace)
			}
			lc, err := lifecycle.NewLifecycle(immortalNamespaces)
			if err != nil {
				return nil, err
			}
			admissionInitializer.Initialize(lc)
			if err := lc.(admission.Validator).Validate(); err != nil {
				return nil, err
			}
			plugin = lc

		case serviceadmit.ExternalIPPluginName:
			// this needs to be moved upstream to be part of core config
			reject, admit, err := serviceadmit.ParseRejectAdmitCIDRRules(options.NetworkConfig.ExternalIPNetworkCIDRs)
			if err != nil {
				// should have been caught with validation
				return nil, err
			}
			allowIngressIP := false
			if _, ipNet, err := net.ParseCIDR(options.NetworkConfig.IngressIPNetworkCIDR); err == nil && !ipNet.IP.IsUnspecified() {
				allowIngressIP = true
			}
			plugin = serviceadmit.NewExternalIPRanger(reject, admit, allowIngressIP)

		case serviceadmit.RestrictedEndpointsPluginName:
			// we need to set some customer parameters, so create by hand
			restrictedNetworks, err := serviceadmit.ParseSimpleCIDRRules([]string{options.NetworkConfig.ClusterNetworkCIDR, options.NetworkConfig.ServiceNetworkCIDR})
			if err != nil {
				// should have been caught with validation
				return nil, err
			}
			plugin = serviceadmit.NewRestrictedEndpointsAdmission(restrictedNetworks)

		case saadmit.PluginName:
			// we need to set some custom parameters on the service account admission controller, so create that one by hand
			saAdmitter := saadmit.NewServiceAccount()
			saAdmitter.SetInternalKubeClientSet(kubeClientSet)
			saAdmitter.LimitSecretReferences = options.ServiceAccountConfig.LimitSecretReferences
			plugin = saAdmitter

		default:
			configFile, err := pluginconfig.GetAdmissionConfigurationFile(pluginConfig, pluginName, admissionConfigFilename)
			if err != nil {
				return nil, err
			}
			configReader, err := admission.ReadAdmissionConfiguration([]string{pluginName}, configFile)
			if err != nil {
				return nil, err
			}
			pluginConfigReader, err := configReader.ConfigFor(pluginName)
			if err != nil {
				return nil, err
			}

			plugin, err = admissionregistry.OriginAdmissionPlugins.InitPlugin(pluginName, pluginConfigReader, admissionInitializer)
			if err != nil {
				// should have been caught with validation
				return nil, err
			}
			if plugin == nil {
				continue
			}

			// skip initialization below because admission.InitPlugin does all the work
			skipInitialization = true
		}

		plugins = append(plugins, plugin)

		if !skipInitialization {
			admissionInitializer.Initialize(plugin)
		}
	}

	// ensure that plugins have been properly initialized
	if err := oadmission.Validate(plugins); err != nil {
		return nil, err
	}

	return admission.NewChainHandler(plugins...), nil
}

func newAuthenticator(config configapi.MasterConfig, restOptionsGetter restoptions.Getter, tokenGetter serviceaccount.ServiceAccountTokenGetter, apiClientCAs *x509.CertPool, groupMapper identitymapper.UserToGroupMapper) (authenticator.Request, error) {
	authenticators := []authenticator.Request{}
	tokenAuthenticators := []authenticator.Request{}

	// ServiceAccount token
	if len(config.ServiceAccountConfig.PublicKeyFiles) > 0 {
		publicKeys := []interface{}{}
		for _, keyFile := range config.ServiceAccountConfig.PublicKeyFiles {
			readPublicKeys, err := serviceaccount.ReadPublicKeys(keyFile)
			if err != nil {
				return nil, fmt.Errorf("Error reading service account key file %s: %v", keyFile, err)
			}
			publicKeys = append(publicKeys, readPublicKeys...)
		}
		serviceAccountTokenAuthenticator := serviceaccount.JWTTokenAuthenticator(publicKeys, true, tokenGetter)
		tokenAuthenticators = append(
			tokenAuthenticators,
			bearertoken.New(serviceAccountTokenAuthenticator),
			websocket.NewProtocolAuthenticator(serviceAccountTokenAuthenticator),
			paramtoken.New("access_token", serviceAccountTokenAuthenticator, true),
		)
	}

	// OAuth token
	if config.OAuthConfig != nil {
		oauthTokenAuthenticator, err := getEtcdTokenAuthenticator(restOptionsGetter, groupMapper)
		if err != nil {
			return nil, fmt.Errorf("Error building OAuth token authenticator: %v", err)
		}
		oauthTokenRequestAuthenticators := []authenticator.Request{
			bearertoken.New(oauthTokenAuthenticator),
			websocket.NewProtocolAuthenticator(oauthTokenAuthenticator),
			paramtoken.New("access_token", oauthTokenAuthenticator, true),
		}

		tokenAuthenticators = append(tokenAuthenticators,
			// if you have a bearer token, you're a human (usually)
			// if you change this, have a look at the impersonationFilter where we attach groups to the impersonated user
			group.NewGroupAdder(union.New(oauthTokenRequestAuthenticators...), []string{bootstrappolicy.AuthenticatedOAuthGroup}))
	}

	if len(tokenAuthenticators) > 0 {
		authenticators = append(authenticators, union.New(tokenAuthenticators...))
	}

	if configapi.UseTLS(config.ServingInfo.ServingInfo) {
		// build cert authenticator
		// TODO: add "system:" prefix in authenticator, limit cert to username
		// TODO: add "system:" prefix to groups in authenticator, limit cert to group name
		opts := x509request.DefaultVerifyOptions()
		opts.Roots = apiClientCAs
		certauth := x509request.New(opts, x509request.CommonNameUserConversion)
		authenticators = append(authenticators, certauth)
	}

	resultingAuthenticator := union.NewFailOnError(authenticators...)

	topLevelAuthenticators := []authenticator.Request{}
	// if we have a front proxy providing authentication configuration, wire it up and it should come first
	if config.AuthConfig.RequestHeader != nil {
		requestHeaderAuthenticator, err := headerrequest.NewSecure(
			config.AuthConfig.RequestHeader.ClientCA,
			config.AuthConfig.RequestHeader.ClientCommonNames,
			config.AuthConfig.RequestHeader.UsernameHeaders,
			config.AuthConfig.RequestHeader.GroupHeaders,
			config.AuthConfig.RequestHeader.ExtraHeaderPrefixes,
		)
		if err != nil {
			return nil, fmt.Errorf("Error building front proxy auth config: %v", err)
		}
		topLevelAuthenticators = append(topLevelAuthenticators, union.New(requestHeaderAuthenticator, resultingAuthenticator))

	} else {
		topLevelAuthenticators = append(topLevelAuthenticators, resultingAuthenticator)

	}
	topLevelAuthenticators = append(topLevelAuthenticators, anonymous.NewAuthenticator())

	return group.NewAuthenticatedGroupAdder(union.NewFailOnError(topLevelAuthenticators...)), nil
}

func newProjectAuthorizationCache(subjectLocator authorizer.SubjectLocator, kubeClient kclientsetinternal.Interface, internalKubeInformers kinternalinformers.SharedInformerFactory, authorizationInformers authorizationinternalinformer.Interface) *projectauth.AuthorizationCache {
	return projectauth.NewAuthorizationCache(
		internalKubeInformers.Core().InternalVersion().Namespaces().Informer(),
		projectauth.NewAuthorizerReviewer(subjectLocator),
		clusterPolicyLister{authorizationInformers.ClusterPolicies().Lister(), authorizationInformers.ClusterPolicies().Informer()},
		clusterPolicyBindingLister{authorizationInformers.ClusterPolicyBindings().Lister(), authorizationInformers.ClusterPolicyBindings().Informer()},
		policyLister{authorizationInformers.Policies().Lister(), authorizationInformers.Policies().Informer()},
		policyBindingLister{authorizationInformers.PolicyBindings().Lister(), authorizationInformers.PolicyBindings().Informer()},
	)
}

func newAuthorizer(ruleResolver rulevalidation.AuthorizationRuleResolver, authorizationInformer authorizationinternalinformer.Interface, projectRequestDenyMessage string) (kauthorizer.Authorizer, authorizer.SubjectLocator) {
	messageMaker := authorizer.NewForbiddenMessageResolver(projectRequestDenyMessage)
	roleBasedAuthorizer, subjectLocator := authorizer.NewAuthorizer(ruleResolver, messageMaker)
	scopeLimitedAuthorizer := scope.NewAuthorizer(roleBasedAuthorizer, authorizationInformer.ClusterPolicies().Lister(), messageMaker)

	authorizer := authorizerunion.New(
		authorizerfactory.NewPrivilegedGroups(user.SystemPrivilegedGroup), // authorizes system:masters to do anything, just like upstream
		scopeLimitedAuthorizer)

	return authorizer, subjectLocator
}

func newAuthorizationAttributeBuilder(requestContextMapper apirequest.RequestContextMapper) authorizer.AuthorizationAttributeBuilder {
	// Default API request info factory
	requestInfoFactory := &apirequest.RequestInfoFactory{APIPrefixes: sets.NewString("api", "osapi", "oapi", "apis"), GrouplessAPIPrefixes: sets.NewString("api", "osapi", "oapi")}
	// Wrap with a request info factory that detects unsafe requests and modifies verbs/resources appropriately so policy can address them separately
	browserSafeRequestInfoResolver := authorizer.NewBrowserSafeRequestInfoResolver(
		requestContextMapper,
		sets.NewString(bootstrappolicy.AuthenticatedGroup),
		requestInfoFactory,
	)
	personalSARRequestInfoResolver := authorizer.NewPersonalSARRequestInfoResolver(browserSafeRequestInfoResolver)
	projectRequestInfoResolver := authorizer.NewProjectRequestInfoResolver(personalSARRequestInfoResolver)

	authorizationAttributeBuilder := authorizer.NewAuthorizationAttributeBuilder(requestContextMapper, projectRequestInfoResolver)
	return authorizationAttributeBuilder
}

func getEtcdTokenAuthenticator(optsGetter restoptions.Getter, groupMapper identitymapper.UserToGroupMapper) (authenticator.Token, error) {
	// this never does a create for access tokens, so we don't need to be able to validate scopes against the client
	accessTokenStorage, err := accesstokenetcd.NewREST(optsGetter, nil)
	if err != nil {
		return nil, err
	}
	accessTokenRegistry := accesstokenregistry.NewRegistry(accessTokenStorage)

	userStorage, err := useretcd.NewREST(optsGetter)
	if err != nil {
		return nil, err
	}
	userRegistry := userregistry.NewRegistry(userStorage)

	return authnregistry.NewTokenAuthenticator(accessTokenRegistry, userRegistry, groupMapper), nil
}

// KubeClientsetInternal returns the kubernetes client object
func (c *MasterConfig) KubeClientsetInternal() kclientsetinternal.Interface {
	return c.PrivilegedLoopbackKubernetesClientsetInternal
}

// KubeClientsetInternal returns the kubernetes client object
func (c *MasterConfig) KubeClientsetExternal() kclientsetexternal.Interface {
	return c.PrivilegedLoopbackKubernetesClientsetExternal
}

// OAuthServerClients returns the openshift and kubernetes OAuth server client objects
// The returned clients are privileged
func (c *MasterConfig) OAuthServerClients() (*osclient.Client, kclientsetinternal.Interface) {
	return c.PrivilegedLoopbackOpenShiftClient, c.PrivilegedLoopbackKubernetesClientsetInternal
}

// ServiceAccountRoleBindingClient returns the client object used to bind roles to service accounts
// It must have the following capabilities:
//  get, list, update, create policyBindings and clusterPolicyBindings in all namespaces
func (c *MasterConfig) ServiceAccountRoleBindingClient() *osclient.Client {
	return c.PrivilegedLoopbackOpenShiftClient
}

// PolicyClient returns the policy client object
// It must have the following capabilities:
//  list, watch all policyBindings in all namespaces
//  list, watch all policies in all namespaces
//  create resourceAccessReviews in all namespaces
func (c *MasterConfig) PolicyClient() *osclient.Client {
	return c.PrivilegedLoopbackOpenShiftClient
}

// RouteAllocatorClients returns the route allocator client objects
func (c *MasterConfig) RouteAllocatorClients() (*osclient.Client, kclientsetinternal.Interface) {
	return c.PrivilegedLoopbackOpenShiftClient, c.PrivilegedLoopbackKubernetesClientsetInternal
}

// SecurityAllocationControllerClient returns the security allocation controller client object
func (c *MasterConfig) SecurityAllocationControllerClient() kclientsetexternal.Interface {
	return c.PrivilegedLoopbackKubernetesClientsetExternal
}

func getOpenShiftClientEnvVars(options configapi.MasterConfig) ([]kapi.EnvVar, error) {
	_, kclientConfig, err := configapi.GetInternalKubeClient(
		options.MasterClients.OpenShiftLoopbackKubeConfig,
		options.MasterClients.OpenShiftLoopbackClientConnectionOverrides,
	)
	if err != nil {
		return nil, err
	}
	return clientcmd.EnvVars(
		kclientConfig.Host,
		kclientConfig.CAData,
		kclientConfig.Insecure,
		path.Join(serviceaccountadmission.DefaultAPITokenMountPath, kapi.ServiceAccountTokenKey),
	), nil
}

// WebConsoleEnabled says whether web ui is not a disabled feature and asset service is configured.
func (c *MasterConfig) WebConsoleEnabled() bool {
	return c.Options.AssetConfig != nil && !c.Options.DisabledFeatures.Has(configapi.FeatureWebConsole)
}
