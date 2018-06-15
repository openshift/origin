package clientcmd

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	restclient "k8s.io/client-go/rest"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/controller"
	"k8s.io/kubernetes/pkg/kubectl"
	"k8s.io/kubernetes/pkg/kubectl/categories"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/cmd/util/openapi"
	"k8s.io/kubernetes/pkg/kubectl/resource"
	"k8s.io/kubernetes/pkg/kubectl/validation"
	kprinters "k8s.io/kubernetes/pkg/printers"

	"github.com/openshift/origin/pkg/api/latest"
	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	appsmanualclient "github.com/openshift/origin/pkg/apps/client/internalversion"
	appsclient "github.com/openshift/origin/pkg/apps/generated/internalclientset"
	appsclientinternal "github.com/openshift/origin/pkg/apps/generated/internalclientset"
	appsutil "github.com/openshift/origin/pkg/apps/util"
	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	buildmanualclient "github.com/openshift/origin/pkg/build/client/internalversion"
	buildclientinternal "github.com/openshift/origin/pkg/build/generated/internalclientset"
	buildutil "github.com/openshift/origin/pkg/build/util"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	deploymentcmd "github.com/openshift/origin/pkg/oc/cli/deploymentconfigs"
	"github.com/openshift/origin/pkg/oc/cli/describe"
)

type ring1Factory struct {
	clientAccessFactory      kcmdutil.ClientAccessFactory
	kubeObjectMappingFactory kcmdutil.ObjectMappingFactory
}

func NewObjectMappingFactory(clientAccessFactory kcmdutil.ClientAccessFactory) kcmdutil.ObjectMappingFactory {
	return &ring1Factory{
		clientAccessFactory:      clientAccessFactory,
		kubeObjectMappingFactory: kcmdutil.NewObjectMappingFactory(clientAccessFactory),
	}
}

func (f *ring1Factory) Object() (meta.RESTMapper, runtime.ObjectTyper) {
	return f.kubeObjectMappingFactory.Object()
}

func (f *ring1Factory) CategoryExpander() categories.CategoryExpander {
	return f.kubeObjectMappingFactory.CategoryExpander()
}

func (f *ring1Factory) ClientForMapping(mapping *meta.RESTMapping) (resource.RESTClient, error) {
	return f.kubeObjectMappingFactory.ClientForMapping(mapping)
}

func (f *ring1Factory) UnstructuredClientForMapping(mapping *meta.RESTMapping) (resource.RESTClient, error) {
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
		describer, ok := describe.DescriberFor(mapping.GroupVersionKind.GroupKind(), clientConfig, kClient, clientConfig.Host)
		if !ok {
			return nil, fmt.Errorf("no description has been implemented for %q", mapping.GroupVersionKind.Kind)
		}
		return describer, nil
	}

	return f.kubeObjectMappingFactory.Describer(mapping)
}

func (f *ring1Factory) LogsForObject(object, options runtime.Object, timeout time.Duration) (*restclient.Request, error) {
	clientConfig, err := f.clientAccessFactory.ClientConfig()
	if err != nil {
		return nil, err
	}

	switch t := object.(type) {
	case *appsapi.DeploymentConfig:
		dopts, ok := options.(*appsapi.DeploymentLogOptions)
		if !ok {
			return nil, errors.New("provided options object is not a DeploymentLogOptions")
		}
		appsClient, err := appsclientinternal.NewForConfig(clientConfig)
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
		buildClient, err := buildclientinternal.NewForConfig(clientConfig)
		if err != nil {
			return nil, err
		}
		return buildmanualclient.NewBuildLogClient(buildClient.Build().RESTClient(), t.Namespace).Logs(t.Name, *bopts), nil
	case *buildapi.BuildConfig:
		bopts, ok := options.(*buildapi.BuildLogOptions)
		if !ok {
			return nil, errors.New("provided options object is not a BuildLogOptions")
		}
		buildClient, err := buildclientinternal.NewForConfig(clientConfig)
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

func (f *ring1Factory) HistoryViewer(mapping *meta.RESTMapping) (kubectl.HistoryViewer, error) {
	if appsapi.Kind("DeploymentConfig") == mapping.GroupVersionKind.GroupKind() {
		kc, err := f.clientAccessFactory.ClientSet()
		if err != nil {
			return nil, err
		}
		return deploymentcmd.NewDeploymentConfigHistoryViewer(kc), nil
	}
	return f.kubeObjectMappingFactory.HistoryViewer(mapping)
}

func (f *ring1Factory) Rollbacker(mapping *meta.RESTMapping) (kubectl.Rollbacker, error) {
	if appsapi.Kind("DeploymentConfig") == mapping.GroupVersionKind.GroupKind() {
		config, err := f.clientAccessFactory.ClientConfig()
		if err != nil {
			return nil, err
		}
		return deploymentcmd.NewDeploymentConfigRollbacker(appsclient.NewForConfigOrDie(config)), nil
	}
	return f.kubeObjectMappingFactory.Rollbacker(mapping)
}

func (f *ring1Factory) StatusViewer(mapping *meta.RESTMapping) (kubectl.StatusViewer, error) {
	config, err := f.clientAccessFactory.ClientConfig()
	if err != nil {
		return nil, err
	}
	if appsapi.Kind("DeploymentConfig") == mapping.GroupVersionKind.GroupKind() {
		return deploymentcmd.NewDeploymentConfigStatusViewer(appsclient.NewForConfigOrDie(config)), nil
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
	case *appsapi.DeploymentConfig:
		fallback := t.Spec.Template

		kc, err := f.clientAccessFactory.ClientSet()
		if err != nil {
			return fallback, err
		}

		latestDeploymentName := appsutil.LatestDeploymentNameForConfig(t)
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
			if fallback == nil || pod.CreationTimestamp.Before(&fallback.CreationTimestamp) {
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
	case *appsapi.DeploymentConfig:
		kc, err := f.clientAccessFactory.ClientSet()
		if err != nil {
			return nil, err
		}
		selector := labels.SelectorFromSet(t.Spec.Selector)
		f := func(pods []*v1.Pod) sort.Interface { return sort.Reverse(controller.ActivePods(pods)) }
		pod, _, err := kcmdutil.GetFirstPod(kc.Core(), t.Namespace, selector.String(), 1*time.Minute, f)
		return pod, err
	default:
		return f.kubeObjectMappingFactory.AttachablePodForObject(object, timeout)
	}
}

func (f *ring1Factory) Validator(validate bool) (validation.Schema, error) {
	return f.kubeObjectMappingFactory.Validator(validate)
}

func (f *ring1Factory) OpenAPISchema() (openapi.Resources, error) {
	return f.kubeObjectMappingFactory.OpenAPISchema()
}
