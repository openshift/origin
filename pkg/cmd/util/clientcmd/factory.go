package clientcmd

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/pkg/apis/apps"
	kclientcmd "k8s.io/client-go/tools/clientcmd"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/apis/apps"
	"k8s.io/kubernetes/pkg/apis/batch"
	"k8s.io/kubernetes/pkg/apis/extensions"
	extensions_v1beta1 "k8s.io/kubernetes/pkg/apis/extensions/v1beta1"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	coreclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/core/internalversion"
	"k8s.io/kubernetes/pkg/controller"
	k8s_deployutil "k8s.io/kubernetes/pkg/controller/deployment/util"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"
	"k8s.io/kubernetes/pkg/printers"

	deployapi "github.com/openshift/origin/pkg/apps/apis/apps"
	deployutil "github.com/openshift/origin/pkg/apps/util"
	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	"github.com/openshift/origin/pkg/cmd/util"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
)

// New creates a default Factory for commands that should share identical server
// connection behavior. Most commands should use this method to get a factory.
func New(flags *pflag.FlagSet) *Factory {
	f := NewFactory(nil)
	f.BindFlags(flags)

	return f
}

// Factory provides common options for OpenShift commands
type Factory struct {
	ClientAccessFactory
	kcmdutil.ObjectMappingFactory
	kcmdutil.BuilderFactory
}

var _ kcmdutil.Factory = &Factory{}

// NewFactory creates an object that holds common methods across all OpenShift commands
func NewFactory(optionalClientConfig kclientcmd.ClientConfig) *Factory {
	clientAccessFactory := NewClientAccessFactory(optionalClientConfig)
	objectMappingFactory := NewObjectMappingFactory(clientAccessFactory)
	builderFactory := kcmdutil.NewBuilderFactory(clientAccessFactory, objectMappingFactory)

	return &Factory{
		ClientAccessFactory:  clientAccessFactory,
		ObjectMappingFactory: objectMappingFactory,
		BuilderFactory:       builderFactory,
	}
}

// PrintResourceInfos receives a list of resource infos and prints versioned objects if a generic output format was specified
// otherwise, it iterates through info objects, printing each resource with a unique printer for its mapping
func (f *Factory) PrintResourceInfos(cmd *cobra.Command, isLocal bool, infos []*resource.Info, out io.Writer) error {
	// mirrors PrintResourceInfoForCommand upstream
	printer, err := f.PrinterForCommand(cmd, isLocal, nil, printers.PrintOptions{})
	if err != nil {
		return nil
	}
	if !printer.IsGeneric() {
		for _, info := range infos {
			mapping := info.ResourceMapping()
			printer, err := f.PrinterForMapping(cmd, isLocal, nil, mapping, false)
			if err != nil {
				return err
			}
			if err := printer.PrintObj(info.Object, out); err != nil {
				return nil
			}
		}
		return nil
	}

	printAsList := len(infos) != 1
	object, err := resource.AsVersionedObject(infos, printAsList, schema.GroupVersion{}, api.Codecs.LegacyCodec())
	if err != nil {
		return err
	}

	return printer.PrintObj(object, out)
}

// FlagBinder represents an interface that allows to bind extra flags into commands.
type FlagBinder interface {
	// Bound returns true if the flag is already bound to a command.
	Bound() bool
	// Bind allows to bind an extra flag to a command
	Bind(*pflag.FlagSet)
}

func ResourceMapper(f kcmdutil.Factory) *resource.Mapper {
	mapper, typer := f.Object()
	return &resource.Mapper{
		RESTMapper:   mapper,
		ObjectTyper:  typer,
		ClientMapper: resource.ClientMapperFunc(f.ClientForMapping),
	}
}

// UpdateObjectEnvironment update the environment variables in object specification.
func (f *Factory) UpdateObjectEnvironment(obj runtime.Object, fn func(*[]api.EnvVar) error) (bool, error) {
	switch t := obj.(type) {
	case *buildapi.BuildConfig:
		if t.Spec.Strategy.CustomStrategy != nil {
			return true, fn(&t.Spec.Strategy.CustomStrategy.Env)
		}
		if t.Spec.Strategy.SourceStrategy != nil {
			return true, fn(&t.Spec.Strategy.SourceStrategy.Env)
		}
		if t.Spec.Strategy.DockerStrategy != nil {
			return true, fn(&t.Spec.Strategy.DockerStrategy.Env)
		}
	}
	return false, fmt.Errorf("object does not contain any environment variables")
}

// ExtractFileContents returns a map of keys to contents, false if the object cannot support such an
// operation, or an error.
func (f *Factory) ExtractFileContents(obj runtime.Object) (map[string][]byte, bool, error) {
	switch t := obj.(type) {
	case *api.Secret:
		return t.Data, true, nil
	case *api.ConfigMap:
		out := make(map[string][]byte)
		for k, v := range t.Data {
			out[k] = []byte(v)
		}
		return out, true, nil
	default:
		return nil, false, nil
	}
}

// ApproximatePodTemplateForObject returns a pod template object for the provided source.
// It may return both an error and a object. It attempt to return the best possible template
// available at the current time.
func (f *Factory) ApproximatePodTemplateForObject(object runtime.Object) (*api.PodTemplateSpec, error) {
	switch t := object.(type) {
	case *imageapi.ImageStreamTag:
		// create a minimal pod spec that uses the image referenced by the istag without any introspection
		// it possible that we could someday do a better job introspecting it
		return &api.PodTemplateSpec{
			Spec: api.PodSpec{
				RestartPolicy: api.RestartPolicyNever,
				Containers: []api.Container{
					{Name: "container-00", Image: t.Image.DockerImageReference},
				},
			},
		}, nil
	case *imageapi.ImageStreamImage:
		// create a minimal pod spec that uses the image referenced by the istag without any introspection
		// it possible that we could someday do a better job introspecting it
		return &api.PodTemplateSpec{
			Spec: api.PodSpec{
				RestartPolicy: api.RestartPolicyNever,
				Containers: []api.Container{
					{Name: "container-00", Image: t.Image.DockerImageReference},
				},
			},
		}, nil
	case *deployapi.DeploymentConfig:
		fallback := t.Spec.Template

		kc, err := f.ClientSet()
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

		for i := range pods.Items {
			pod := &pods.Items[i]
			if fallback == nil || pod.CreationTimestamp.Before(fallback.CreationTimestamp) {
				fallback = &api.PodTemplateSpec{
					ObjectMeta: pod.ObjectMeta,
					Spec:       pod.Spec,
				}
			}
		}
		return fallback, nil

	default:

		pod, err := f.MostAccuratePodTemplateForObject(object)
		if pod != nil {
			return &api.PodTemplateSpec{
				ObjectMeta: pod.ObjectMeta,
				Spec:       pod.Spec,
			}, err
		}
		return nil, err
	}
}

func (f *Factory) PodForResource(resource string, timeout time.Duration) (string, error) {
	sortBy := func(pods []*v1.Pod) sort.Interface { return sort.Reverse(controller.ActivePods(pods)) }
	namespace, _, err := f.DefaultNamespace()
	if err != nil {
		return "", err
	}
	mapper, _ := f.Object()
	resourceType, name, err := util.ResolveResource(api.Resource("pods"), resource, mapper)
	if err != nil {
		return "", err
	}

	switch resourceType {
	case api.Resource("pods"):
		return name, nil
	case api.Resource("replicationcontrollers"):
		kc, err := f.ClientSet()
		if err != nil {
			return "", err
		}
		rc, err := kc.Core().ReplicationControllers(namespace).Get(name, metav1.GetOptions{})
		if err != nil {
			return "", err
		}
		selector := labels.SelectorFromSet(rc.Spec.Selector)
		pod, _, err := kcmdutil.GetFirstPod(kc.Core(), namespace, selector, timeout, sortBy)
		if err != nil {
			return "", err
		}
		return pod.Name, nil
	case deployapi.Resource("deploymentconfigs"), deployapi.LegacyResource("deploymentconfigs"):
		appsClient, err := f.OpenshiftInternalAppsClient()
		if err != nil {
			return "", err
		}
		kc, err := f.ClientSet()
		if err != nil {
			return "", err
		}
		dc, err := appsClient.Apps().DeploymentConfigs(namespace).Get(name, metav1.GetOptions{})
		if err != nil {
			return "", err
		}
		selector := labels.SelectorFromSet(dc.Spec.Selector)
		pod, _, err := kcmdutil.GetFirstPod(kc.Core(), namespace, selector, timeout, sortBy)
		if err != nil {
			return "", err
		}
		return pod.Name, nil
	case extensions.Resource("daemonsets"):
		kc, err := f.ClientSet()
		if err != nil {
			return "", err
		}
		ds, err := kc.Extensions().DaemonSets(namespace).Get(name, metav1.GetOptions{})
		if err != nil {
			return "", err
		}
		selector, err := metav1.LabelSelectorAsSelector(ds.Spec.Selector)
		if err != nil {
			return "", err
		}
		pod, _, err := kcmdutil.GetFirstPod(kc.Core(), namespace, selector, timeout, sortBy)
		if err != nil {
			return "", err
		}
		return pod.Name, nil
	case extensions.Resource("deployments"):
		kc, err := f.ClientSet()
		if err != nil {
			return "", err
		}
		d, err := kc.Extensions().Deployments(namespace).Get(name, metav1.GetOptions{})
		if err != nil {
			return "", err
		}
		selector, err := metav1.LabelSelectorAsSelector(d.Spec.Selector)
		if err != nil {
			return "", err
		}
		pod, _, err := kcmdutil.GetFirstPod(kc.Core(), namespace, selector, timeout, sortBy)
		if err != nil {
			return "", err
		}
		return pod.Name, nil
	case apps.Resource("statefulsets"):
		kc, err := f.ClientSet()
		if err != nil {
			return "", err
		}
		s, err := kc.Apps().StatefulSets(namespace).Get(name, metav1.GetOptions{})
		if err != nil {
			return "", err
		}
		selector, err := metav1.LabelSelectorAsSelector(s.Spec.Selector)
		if err != nil {
			return "", err
		}
		pod, _, err := kcmdutil.GetFirstPod(kc.Core(), namespace, selector, timeout, sortBy)
		if err != nil {
			return "", err
		}
		return pod.Name, nil
	case extensions.Resource("replicasets"):
		kc, err := f.ClientSet()
		if err != nil {
			return "", err
		}
		rs, err := kc.Extensions().ReplicaSets(namespace).Get(name, metav1.GetOptions{})
		if err != nil {
			return "", err
		}
		selector, err := metav1.LabelSelectorAsSelector(rs.Spec.Selector)
		if err != nil {
			return "", err
		}
		pod, _, err := kcmdutil.GetFirstPod(kc.Core(), namespace, selector, timeout, sortBy)
		if err != nil {
			return "", err
		}
		return pod.Name, nil
	case batch.Resource("jobs"):
		kc, err := f.ClientSet()
		if err != nil {
			return "", err
		}
		job, err := kc.Batch().Jobs(namespace).Get(name, metav1.GetOptions{})
		if err != nil {
			return "", err
		}
		return podNameForJob(job, kc, timeout, sortBy)
	default:
		return "", fmt.Errorf("remote shell for %s is not supported", resourceType)
	}
}

func (f *Factory) MostAccuratePodTemplateForObject(object runtime.Object) (*api.PodTemplateSpec, error) {
	sortBy := func(pods []*v1.Pod) sort.Interface { return controller.ActivePods(pods) }
	clientset, err := f.ClientAccessFactory.ClientSet()
	if err != nil {
		return nil, err
	}
	var podSpec *api.PodTemplateSpec
	var pod *api.Pod
	var selector labels.Selector
	var namespace string
	switch t := object.(type) {
	case *extensions.ReplicaSet:
		namespace = t.Namespace
		selector = labels.SelectorFromSet(t.Spec.Selector.MatchLabels)
		if err != nil {
			return &t.Spec.Template, fmt.Errorf("invalid label selector: %v", err)
		}
		pods, err := GetAllPods(clientset.Core(), namespace, selector, sortBy)
		if err != nil || len(pods) == 0 {
			return &t.Spec.Template, err
		}
		pod = pods[0]
	case *api.ReplicationController:
		namespace = t.Namespace
		selector = labels.SelectorFromSet(t.Spec.Selector)
		if err != nil {
			return t.Spec.Template, fmt.Errorf("invalid label selector: %v", err)
		}
		pods, err := GetAllPods(clientset.Core(), namespace, selector, sortBy)
		if err != nil || len(pods) == 0 {
			return t.Spec.Template, err
		}
		pod = pods[0]
	case *apps.StatefulSet:
		namespace = t.Namespace
		selector, err = metav1.LabelSelectorAsSelector(t.Spec.Selector)
		if err != nil {
			return &t.Spec.Template, fmt.Errorf("invalid label selector: %v", err)
		}
		pods, err := GetAllPods(clientset.Core(), namespace, selector, sortBy)
		if err != nil || len(pods) == 0 {
			return &t.Spec.Template, err
		}
		pod = pods[0]
	case *extensions.Deployment:
		namespace = t.Namespace
		selector, err = metav1.LabelSelectorAsSelector(t.Spec.Selector)
		if err != nil {
			return &t.Spec.Template, fmt.Errorf("invalid label selector: %v", err)
		}
		options := metav1.ListOptions{LabelSelector: selector.String()}
		var allRs []*extensions_v1beta1.ReplicaSet
		rsList, err := clientset.Extensions().ReplicaSets(namespace).List(options)
		if err != nil {
			return &t.Spec.Template, err
		}
		for _, rs := range rsList.Items {
			rs_external := &extensions_v1beta1.ReplicaSet{}
			extensions_v1beta1.Convert_extensions_ReplicaSet_To_v1beta1_ReplicaSet(&rs, rs_external, nil)
			allRs = append(allRs, rs_external)
		}
		d := &extensions_v1beta1.Deployment{}
		api.Scheme.Convert(t, d, nil)
		rs, err := k8s_deployutil.FindNewReplicaSet(d, allRs)
		if err != nil {
			return &t.Spec.Template, err
		}
		rs_selector, err := metav1.LabelSelectorAsSelector(rs.Spec.Selector)
		if err != nil {
			return &t.Spec.Template, fmt.Errorf("invalid label selector: %v", err)
		}
		pods, err := GetAllPods(clientset.Core(), namespace, rs_selector, sortBy)
		if err != nil || len(pods) == 0 {
			return &t.Spec.Template, err
		}
		pod = pods[0]
	case *batch.Job:
		namespace = t.Namespace
		selector, err = metav1.LabelSelectorAsSelector(t.Spec.Selector)
		if err != nil {
			return nil, fmt.Errorf("invalid label selector: %v", err)
		}
		pods, err := GetAllPods(clientset.Core(), namespace, selector, sortBy)
		if err != nil || len(pods) == 0 {
			return &t.Spec.Template, err
		}
		pod = pods[0]
	case *api.Pod:
		podSpec = &api.PodTemplateSpec{
			ObjectMeta: t.ObjectMeta,
			Spec:       t.Spec,
		}
	default:
		gvks, _, err := api.Scheme.ObjectKinds(object)
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("cannot get pod template of %v: not implemented", gvks[0])
	}
	if pod != nil {
		podSpec = &api.PodTemplateSpec{
			ObjectMeta: pod.ObjectMeta,
			Spec:       pod.Spec,
		}
	}
	return podSpec, err
}

func podNameForJob(job *batch.Job, kc kclientset.Interface, timeout time.Duration, sortBy func(pods []*v1.Pod) sort.Interface) (string, error) {
	selector, err := metav1.LabelSelectorAsSelector(job.Spec.Selector)
	if err != nil {
		return "", err
	}
	pod, _, err := kcmdutil.GetFirstPod(kc.Core(), job.Namespace, selector, timeout, sortBy)
	if err != nil {
		return "", err
	}
	return pod.Name, nil
}

// FindAllCanonicalResources returns all resource names that map directly to their kind (Kind -> Resource -> Kind)
// and are not subresources. This is the closest mapping possible from the client side to resources that can be
// listed and updated. Note that this may return some virtual resources (like imagestreamtags) that can be otherwise
// represented.
// TODO: add a field to APIResources for "virtual" (or that points to the canonical resource).
// TODO: fallback to the scheme when discovery is not possible.
func FindAllCanonicalResources(d discovery.DiscoveryInterface, m meta.RESTMapper) ([]schema.GroupResource, error) {
	set := make(map[schema.GroupResource]struct{})
	all, err := d.ServerResources()
	if err != nil {
		return nil, err
	}
	for _, serverResource := range all {
		gv, err := schema.ParseGroupVersion(serverResource.GroupVersion)
		if err != nil {
			continue
		}
		for _, r := range serverResource.APIResources {
			// ignore subresources
			if strings.Contains(r.Name, "/") {
				continue
			}
			// because discovery info doesn't tell us whether the object is virtual or not, perform a lookup
			// by the kind for resource (which should be the canonical resource) and then verify that the reverse
			// lookup (KindsFor) does not error.
			if mapping, err := m.RESTMapping(schema.GroupKind{Group: gv.Group, Kind: r.Kind}, gv.Version); err == nil {
				if _, err := m.KindsFor(mapping.GroupVersionKind.GroupVersion().WithResource(mapping.Resource)); err == nil {
					set[schema.GroupResource{Group: mapping.GroupVersionKind.Group, Resource: mapping.Resource}] = struct{}{}
				}
			}
		}
	}
	var groupResources []schema.GroupResource
	for k := range set {
		groupResources = append(groupResources, k)
	}
	sort.Sort(groupResourcesByName(groupResources))
	return groupResources, nil
}

type groupResourcesByName []schema.GroupResource

func (g groupResourcesByName) Len() int { return len(g) }
func (g groupResourcesByName) Less(i, j int) bool {
	if g[i].Resource < g[j].Resource {
		return true
	}
	if g[i].Resource > g[j].Resource {
		return false
	}
	return g[i].Group < g[j].Group
}
func (g groupResourcesByName) Swap(i, j int) { g[i], g[j] = g[j], g[i] }

// GetAllPods returns all pods matching the namespace and label selector
func GetAllPods(client coreclient.PodsGetter, namespace string, selector labels.Selector, sortBy func([]*v1.Pod) sort.Interface) ([]*api.Pod, error) {
	options := metav1.ListOptions{LabelSelector: selector.String()}
	res := []*api.Pod{}
	podList, err := client.Pods(namespace).List(options)
	if err != nil {
		return nil, err
	}
	pods := []*v1.Pod{}
	for i := range podList.Items {
		pod := podList.Items[i]
		externalPod := &v1.Pod{}
		v1.Convert_api_Pod_To_v1_Pod(&pod, externalPod, nil)
		pods = append(pods, externalPod)
	}

	if len(pods) > 0 {
		sort.Sort(sortBy(pods))
		for j := range pods {
			internalPod := &api.Pod{}
			v1.Convert_v1_Pod_To_api_Pod(pods[j], internalPod, nil)
			res = append(res, internalPod)
		}
	}
	return res, nil
}
