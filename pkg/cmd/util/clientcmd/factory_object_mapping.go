package clientcmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/emicklei/go-restful-swagger12"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	restclient "k8s.io/client-go/rest"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/api/validation"
	"k8s.io/kubernetes/pkg/controller"
	"k8s.io/kubernetes/pkg/kubectl"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/cmd/util/openapi"
	"k8s.io/kubernetes/pkg/kubectl/resource"
	kprinters "k8s.io/kubernetes/pkg/printers"

	"github.com/openshift/origin/pkg/api/latest"
	deployapi "github.com/openshift/origin/pkg/apps/apis/apps"
	appsmanualclient "github.com/openshift/origin/pkg/apps/client/internalversion"
	deploycmd "github.com/openshift/origin/pkg/apps/cmd"
	appsclient "github.com/openshift/origin/pkg/apps/generated/internalclientset"
	deployutil "github.com/openshift/origin/pkg/apps/util"
	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	authorizationreaper "github.com/openshift/origin/pkg/authorization/reaper"
	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	buildmanualclient "github.com/openshift/origin/pkg/build/client/internalversion"
	buildcmd "github.com/openshift/origin/pkg/build/cmd"
	buildclient "github.com/openshift/origin/pkg/build/generated/internalclientset"
	buildutil "github.com/openshift/origin/pkg/build/util"
	configcmd "github.com/openshift/origin/pkg/config/cmd"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	"github.com/openshift/origin/pkg/oc/cli/describe"
	userapi "github.com/openshift/origin/pkg/user/apis/user"
	authenticationreaper "github.com/openshift/origin/pkg/user/reaper"
)

type ring1Factory struct {
	clientAccessFactory      ClientAccessFactory
	kubeObjectMappingFactory kcmdutil.ObjectMappingFactory
}

func NewObjectMappingFactory(clientAccessFactory ClientAccessFactory) kcmdutil.ObjectMappingFactory {
	return &ring1Factory{
		clientAccessFactory:      clientAccessFactory,
		kubeObjectMappingFactory: kcmdutil.NewObjectMappingFactory(clientAccessFactory),
	}
}

func (f *ring1Factory) Object() (meta.RESTMapper, runtime.ObjectTyper) {
	return f.kubeObjectMappingFactory.Object()
}

func (f *ring1Factory) UnstructuredObject() (meta.RESTMapper, runtime.ObjectTyper, error) {
	return f.kubeObjectMappingFactory.UnstructuredObject()
}

func (f *ring1Factory) CategoryExpander() resource.CategoryExpander {
	upstreamExpander := f.kubeObjectMappingFactory.CategoryExpander()

	var openshiftCategoryExpander resource.CategoryExpander
	openshiftCategoryExpander = legacyOpeshiftCategoryExpander
	discoveryClient, err := f.clientAccessFactory.DiscoveryClient()
	if err == nil {
		// wrap with discovery based filtering
		openshiftCategoryExpander, err = resource.NewDiscoveryFilteredExpander(openshiftCategoryExpander, discoveryClient)
		// you only have an error on missing discoveryClient, so this shouldn't fail.  Check anyway.
		kcmdutil.CheckErr(err)
	}

	return resource.UnionCategoryExpander{legacyOpeshiftCategoryExpander, upstreamExpander}
}

var legacyOpenshiftUserResources = []schema.GroupResource{
	{Group: "", Resource: "buildconfigs"},
	{Group: "", Resource: "builds"},
	{Group: "", Resource: "imagestreams"},
	{Group: "", Resource: "deploymentconfigs"},
	{Group: "", Resource: "routes"},
}

// legacyOpeshiftCategoryExpander is the old hardcoded expansion for servers without listed categories
var legacyOpeshiftCategoryExpander resource.CategoryExpander = resource.SimpleCategoryExpander{
	Expansions: map[string][]schema.GroupResource{
		"all": legacyOpenshiftUserResources,
	},
}

func (f *ring1Factory) ClientForMapping(mapping *meta.RESTMapping) (resource.RESTClient, error) {
	// TODO only do this for legacy kinds
	if latest.OriginKind(mapping.GroupVersionKind) {
		cfg, err := f.clientAccessFactory.ClientConfig()
		if err != nil {
			return nil, err
		}
		if err := configcmd.SetLegacyOpenShiftDefaults(cfg); err != nil {
			return nil, err
		}
		cfg.APIPath = "/apis"
		if mapping.GroupVersionKind.Group == kapi.GroupName {
			cfg.APIPath = "/oapi"
		}
		gv := mapping.GroupVersionKind.GroupVersion()
		cfg.GroupVersion = &gv
		return restclient.RESTClientFor(cfg)
	}
	return f.kubeObjectMappingFactory.ClientForMapping(mapping)
}

func (f *ring1Factory) UnstructuredClientForMapping(mapping *meta.RESTMapping) (resource.RESTClient, error) {
	// TODO only do this for legacy kinds
	if latest.OriginKind(mapping.GroupVersionKind) {
		cfg, err := f.clientAccessFactory.ClientConfig()
		if err != nil {
			return nil, err
		}
		if err := configcmd.SetLegacyOpenShiftDefaults(cfg); err != nil {
			return nil, err
		}
		cfg.APIPath = "/apis"
		if mapping.GroupVersionKind.Group == kapi.GroupName {
			cfg.APIPath = "/oapi"
		}
		gv := mapping.GroupVersionKind.GroupVersion()
		cfg.ContentConfig = dynamic.ContentConfig()
		cfg.GroupVersion = &gv
		return restclient.RESTClientFor(cfg)
	}
	return f.kubeObjectMappingFactory.UnstructuredClientForMapping(mapping)
}

func (f *ring1Factory) Describer(mapping *meta.RESTMapping) (kprinters.Describer, error) {
	// TODO we need to refactor the describer logic to handle misses or run serverside.
	// for now we can special case our "sometimes origin, sometimes kube" resource
	// I think it is correct for more code if this is NOT considered an origin type since
	// it wasn't an origin type pre 3.6.
	isSCC := mapping.GroupVersionKind.Kind == "SecurityContextConstraints"
	if latest.OriginKind(mapping.GroupVersionKind) || isSCC {
		kClient, err := f.clientAccessFactory.ClientSet()
		if err != nil {
			return nil, fmt.Errorf("unable to create client %s: %v", mapping.GroupVersionKind.Kind, err)
		}
		clientConfig, err := f.clientAccessFactory.ClientConfig()
		if err != nil {
			return nil, fmt.Errorf("unable to create client config %s: %v", mapping.GroupVersionKind.Kind, err)
		}

		mappingVersion := mapping.GroupVersionKind.GroupVersion()
		cfg, err := f.clientAccessFactory.ClientConfigForVersion(&mappingVersion)
		if err != nil {
			return nil, fmt.Errorf("unable to load a client %s: %v", mapping.GroupVersionKind.Kind, err)
		}

		describer, ok := describe.DescriberFor(mapping.GroupVersionKind.GroupKind(), clientConfig, kClient, cfg.Host)
		if !ok {
			return nil, fmt.Errorf("no description has been implemented for %q", mapping.GroupVersionKind.Kind)
		}
		return describer, nil
	}
	return f.kubeObjectMappingFactory.Describer(mapping)
}

func (f *ring1Factory) LogsForObject(object, options runtime.Object, timeout time.Duration) (*restclient.Request, error) {
	switch t := object.(type) {
	case *deployapi.DeploymentConfig:
		dopts, ok := options.(*deployapi.DeploymentLogOptions)
		if !ok {
			return nil, errors.New("provided options object is not a DeploymentLogOptions")
		}
		appsClient, err := f.clientAccessFactory.OpenshiftInternalAppsClient()
		if err != nil {
			return nil, err
		}
		return appsmanualclient.NewRolloutLogClient(appsClient.Apps().RESTClient(), t.Namespace).Logs(t.Name, *dopts), nil
	case *buildapi.Build:
		bopts, ok := options.(*buildapi.BuildLogOptions)
		if !ok {
			return nil, errors.New("provided options object is not a BuildLogOptions")
		}
		if bopts.Version != nil {
			return nil, errors.New("cannot specify a version and a build")
		}
		buildClient, err := f.clientAccessFactory.OpenshiftInternalBuildClient()
		if err != nil {
			return nil, err
		}
		return buildmanualclient.NewBuildLogClient(buildClient.Build().RESTClient(), t.Namespace).Logs(t.Name, *bopts), nil
	case *buildapi.BuildConfig:
		bopts, ok := options.(*buildapi.BuildLogOptions)
		if !ok {
			return nil, errors.New("provided options object is not a BuildLogOptions")
		}
		buildClient, err := f.clientAccessFactory.OpenshiftInternalBuildClient()
		if err != nil {
			return nil, err
		}
		logClient := buildmanualclient.NewBuildLogClient(buildClient.Build().RESTClient(), t.Namespace)
		builds, err := buildClient.Build().Builds(t.Namespace).List(metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
		builds.Items = buildapi.FilterBuilds(builds.Items, buildapi.ByBuildConfigPredicate(t.Name))
		if len(builds.Items) == 0 {
			return nil, fmt.Errorf("no builds found for %q", t.Name)
		}
		if bopts.Version != nil {
			// If a version has been specified, try to get the logs from that build.
			desired := buildutil.BuildNameForConfigVersion(t.Name, int(*bopts.Version))
			return logClient.Logs(desired, *bopts), nil
		}
		sort.Sort(sort.Reverse(buildapi.BuildSliceByCreationTimestamp(builds.Items)))
		return logClient.Logs(builds.Items[0].Name, *bopts), nil
	default:
		return f.kubeObjectMappingFactory.LogsForObject(object, options, timeout)
	}
}

func (f *ring1Factory) Scaler(mapping *meta.RESTMapping) (kubectl.Scaler, error) {
	if deployapi.IsKindOrLegacy("DeploymentConfig", mapping.GroupVersionKind.GroupKind()) {
		kc, err := f.clientAccessFactory.ClientSet()
		if err != nil {
			return nil, err
		}
		config, err := f.clientAccessFactory.OpenShiftClientConfig().ClientConfig()
		if err != nil {
			return nil, err
		}
		return deploycmd.NewDeploymentConfigScaler(appsclient.NewForConfigOrDie(config), kc), nil
	}
	return f.kubeObjectMappingFactory.Scaler(mapping)
}

func (f *ring1Factory) Reaper(mapping *meta.RESTMapping) (kubectl.Reaper, error) {
	gk := mapping.GroupVersionKind.GroupKind()
	switch {
	case deployapi.IsKindOrLegacy("DeploymentConfig", gk):
		kc, err := f.clientAccessFactory.ClientSet()
		if err != nil {
			return nil, err
		}
		config, err := f.clientAccessFactory.OpenShiftClientConfig().ClientConfig()
		if err != nil {
			return nil, err
		}
		return deploycmd.NewDeploymentConfigReaper(appsclient.NewForConfigOrDie(config), kc), nil
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
	return f.kubeObjectMappingFactory.Reaper(mapping)
}

func (f *ring1Factory) HistoryViewer(mapping *meta.RESTMapping) (kubectl.HistoryViewer, error) {
	if deployapi.IsKindOrLegacy("DeploymentConfig", mapping.GroupVersionKind.GroupKind()) {
		kc, err := f.clientAccessFactory.ClientSet()
		if err != nil {
			return nil, err
		}
		return deploycmd.NewDeploymentConfigHistoryViewer(kc), nil
	}
	return f.kubeObjectMappingFactory.HistoryViewer(mapping)
}

func (f *ring1Factory) Rollbacker(mapping *meta.RESTMapping) (kubectl.Rollbacker, error) {
	if deployapi.IsKindOrLegacy("DeploymentConfig", mapping.GroupVersionKind.GroupKind()) {
		config, err := f.clientAccessFactory.OpenShiftClientConfig().ClientConfig()
		if err != nil {
			return nil, err
		}
		return deploycmd.NewDeploymentConfigRollbacker(appsclient.NewForConfigOrDie(config)), nil
	}
	return f.kubeObjectMappingFactory.Rollbacker(mapping)
}

func (f *ring1Factory) StatusViewer(mapping *meta.RESTMapping) (kubectl.StatusViewer, error) {
	config, err := f.clientAccessFactory.OpenShiftClientConfig().ClientConfig()
	if err != nil {
		return nil, err
	}
	if deployapi.IsKindOrLegacy("DeploymentConfig", mapping.GroupVersionKind.GroupKind()) {
		return deploycmd.NewDeploymentConfigStatusViewer(appsclient.NewForConfigOrDie(config)), nil
	}
	return f.kubeObjectMappingFactory.StatusViewer(mapping)
}

// ApproximatePodTemplateForObject returns a pod template object for the provided source.
// It may return both an error and a object. It attempt to return the best possible template
// available at the current time.
func (f *ring1Factory) ApproximatePodTemplateForObject(object runtime.Object) (*kapi.PodTemplateSpec, error) {
	switch t := object.(type) {
	case *imageapi.ImageStreamTag:
		// create a minimal pod spec that uses the image referenced by the istag without any introspection
		// it possible that we could someday do a better job introspecting it
		return &kapi.PodTemplateSpec{
			Spec: kapi.PodSpec{
				RestartPolicy: kapi.RestartPolicyNever,
				Containers: []kapi.Container{
					{Name: "container-00", Image: t.Image.DockerImageReference},
				},
			},
		}, nil
	case *imageapi.ImageStreamImage:
		// create a minimal pod spec that uses the image referenced by the istag without any introspection
		// it possible that we could someday do a better job introspecting it
		return &kapi.PodTemplateSpec{
			Spec: kapi.PodSpec{
				RestartPolicy: kapi.RestartPolicyNever,
				Containers: []kapi.Container{
					{Name: "container-00", Image: t.Image.DockerImageReference},
				},
			},
		}, nil
	case *deployapi.DeploymentConfig:
		fallback := t.Spec.Template

		kc, err := f.clientAccessFactory.ClientSet()
		if err != nil {
			return fallback, err
		}

		latestDeploymentName := deployutil.LatestDeploymentNameForConfig(t)
		deployment, err := kc.Core().ReplicationControllers(t.Namespace).Get(latestDeploymentName, metav1.GetOptions{})
		if err != nil {
			return fallback, err
		}

		fallback = deployment.Spec.Template

		pods, err := kc.Core().Pods(deployment.Namespace).List(metav1.ListOptions{LabelSelector: labels.SelectorFromSet(deployment.Spec.Selector).String()})
		if err != nil {
			return fallback, err
		}

		// If we have any pods available, find the newest
		// pod with regards to our most recent deployment.
		// If the fallback PodTemplateSpec is nil, prefer
		// the newest pod available.
		for i := range pods.Items {
			pod := &pods.Items[i]
			if fallback == nil || pod.CreationTimestamp.Before(fallback.CreationTimestamp) {
				fallback = &kapi.PodTemplateSpec{
					ObjectMeta: pod.ObjectMeta,
					Spec:       pod.Spec,
				}
			}
		}
		return fallback, nil

	default:
		return f.kubeObjectMappingFactory.ApproximatePodTemplateForObject(object)
	}
}

func (f *ring1Factory) AttachablePodForObject(object runtime.Object, timeout time.Duration) (*kapi.Pod, error) {
	switch t := object.(type) {
	case *deployapi.DeploymentConfig:
		kc, err := f.clientAccessFactory.ClientSet()
		if err != nil {
			return nil, err
		}
		selector := labels.SelectorFromSet(t.Spec.Selector)
		f := func(pods []*v1.Pod) sort.Interface { return sort.Reverse(controller.ActivePods(pods)) }
		pod, _, err := kcmdutil.GetFirstPod(kc.Core(), t.Namespace, selector, 1*time.Minute, f)
		return pod, err
	default:
		return f.kubeObjectMappingFactory.AttachablePodForObject(object, timeout)
	}
}

func (f *ring1Factory) Validator(validate bool, cacheDir string) (validation.Schema, error) {
	return f.kubeObjectMappingFactory.Validator(validate, cacheDir)
}

func (f *ring1Factory) SwaggerSchema(gvk schema.GroupVersionKind) (*swagger.ApiDeclaration, error) {
	if !latest.OriginLegacyKind(gvk) {
		return f.kubeObjectMappingFactory.SwaggerSchema(gvk)
	}
	kubeClient, err := f.clientAccessFactory.ClientSet()
	if err != nil {
		return nil, err
	}
	return f.OriginSwaggerSchema(kubeClient.Discovery().RESTClient(), gvk.GroupVersion())
}

func (f *ring1Factory) OpenAPISchema(cacheDir string) (*openapi.Resources, error) {
	return f.kubeObjectMappingFactory.OpenAPISchema(cacheDir)
}

// OriginSwaggerSchema returns a swagger API doc for an Origin schema under the /oapi prefix.
func (f *ring1Factory) OriginSwaggerSchema(client restclient.Interface, version schema.GroupVersion) (*swagger.ApiDeclaration, error) {
	if version.Empty() {
		return nil, fmt.Errorf("groupVersion cannot be empty")
	}
	body, err := client.Get().AbsPath("/").Suffix("swaggerapi", "oapi", version.Version).Do().Raw()
	if err != nil {
		return nil, err
	}
	var schema swagger.ApiDeclaration
	err = json.Unmarshal(body, &schema)
	if err != nil {
		return nil, fmt.Errorf("got '%s': %v", string(body), err)
	}
	return &schema, nil
}
