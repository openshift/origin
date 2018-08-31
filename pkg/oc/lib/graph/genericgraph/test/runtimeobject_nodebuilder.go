package test

import (
	"fmt"
	"io/ioutil"
	"reflect"

	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"

	"github.com/openshift/api"
	appsv1 "github.com/openshift/api/apps/v1"
	buildv1 "github.com/openshift/api/build/v1"
	imagev1 "github.com/openshift/api/image/v1"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/openshift/origin/pkg/api/legacy"
	appsgraph "github.com/openshift/origin/pkg/oc/lib/graph/appsgraph/nodes"
	buildgraph "github.com/openshift/origin/pkg/oc/lib/graph/buildgraph/nodes"
	osgraph "github.com/openshift/origin/pkg/oc/lib/graph/genericgraph"
	imagegraph "github.com/openshift/origin/pkg/oc/lib/graph/imagegraph/nodes"
	kubegraph "github.com/openshift/origin/pkg/oc/lib/graph/kubegraph/nodes"
	routegraph "github.com/openshift/origin/pkg/oc/lib/graph/routegraph/nodes"
)

// typeToEnsureMethod stores types to Ensure*Node methods
var typeToEnsureMethod = map[reflect.Type]reflect.Value{}

func init() {
	if err := RegisterEnsureNode(&imagev1.Image{}, imagegraph.EnsureImageNode); err != nil {
		panic(err)
	}
	if err := RegisterEnsureNode(&imagev1.ImageStream{}, imagegraph.EnsureImageStreamNode); err != nil {
		panic(err)
	}
	if err := RegisterEnsureNode(&appsv1.DeploymentConfig{}, appsgraph.EnsureDeploymentConfigNode); err != nil {
		panic(err)
	}
	if err := RegisterEnsureNode(&buildv1.BuildConfig{}, buildgraph.EnsureBuildConfigNode); err != nil {
		panic(err)
	}
	if err := RegisterEnsureNode(&buildv1.Build{}, buildgraph.EnsureBuildNode); err != nil {
		panic(err)
	}
	if err := RegisterEnsureNode(&routev1.Route{}, routegraph.EnsureRouteNode); err != nil {
		panic(err)
	}

	if err := RegisterEnsureNode(&corev1.Pod{}, kubegraph.EnsurePodNode); err != nil {
		panic(err)
	}
	if err := RegisterEnsureNode(&corev1.Service{}, kubegraph.EnsureServiceNode); err != nil {
		panic(err)
	}
	if err := RegisterEnsureNode(&corev1.ServiceAccount{}, kubegraph.EnsureServiceAccountNode); err != nil {
		panic(err)
	}
	if err := RegisterEnsureNode(&corev1.Secret{}, kubegraph.EnsureSecretNode); err != nil {
		panic(err)
	}
	if err := RegisterEnsureNode(&corev1.ReplicationController{}, kubegraph.EnsureReplicationControllerNode); err != nil {
		panic(err)
	}
	if err := RegisterEnsureNode(&corev1.PersistentVolumeClaim{}, kubegraph.EnsurePersistentVolumeClaimNode); err != nil {
		panic(err)
	}
	if err := RegisterEnsureNode(&autoscalingv1.HorizontalPodAutoscaler{}, kubegraph.EnsureHorizontalPodAutoscalerNode); err != nil {
	}
}

func RegisterEnsureNode(containedType, ensureFunction interface{}) error {
	ensureFunctionValue := reflect.ValueOf(ensureFunction)
	ensureType := ensureFunctionValue.Type()
	if err := verifyEnsureFunctionSignature(ensureType); err != nil {
		return err
	}

	reflectedContainedType := reflect.TypeOf(containedType)
	if _, exists := typeToEnsureMethod[reflectedContainedType]; exists {
		return fmt.Errorf("%v is already registered", reflectedContainedType)
	}

	typeToEnsureMethod[reflectedContainedType] = reflect.ValueOf(ensureFunction)

	return nil
}

func EnsureNode(g osgraph.Graph, obj interface{}) error {
	reflectedContainedType := reflect.TypeOf(obj)

	ensureMethod, exists := typeToEnsureMethod[reflectedContainedType]
	if !exists {
		return fmt.Errorf("%v is not registered: %#v", reflectedContainedType, obj)
	}

	callEnsureNode(g, reflect.ValueOf(obj), ensureMethod)
	return nil
}

func verifyEnsureFunctionSignature(ft reflect.Type) error {
	if ft.Kind() != reflect.Func {
		return fmt.Errorf("expected func, got: %v", ft)
	}
	if ft.NumIn() != 2 {
		return fmt.Errorf("expected two 'in' param, got: %v", ft)
	}
	if ft.NumOut() != 1 {
		return fmt.Errorf("expected one 'out' param, got: %v", ft)
	}
	if ft.In(1).Kind() != reflect.Ptr {
		return fmt.Errorf("expected pointer arg for 'in' param 1, got: %v", ft)
	}
	return nil
}

// callEnsureNode calls 'custom' with sv & dv. custom must be a conversion function.
func callEnsureNode(g osgraph.Graph, obj, ensureMethod reflect.Value) {
	args := []reflect.Value{reflect.ValueOf(g), obj}
	ensureMethod.Call(args)[0].Interface()
}

func BuildGraph(path string) (osgraph.Graph, []runtime.Object, error) {
	g := osgraph.New()
	objs := []runtime.Object{}

	data, err := ioutil.ReadFile(path)
	if err != nil {
		return g, objs, err
	}
	scheme := runtime.NewScheme()
	kubernetesscheme.AddToScheme(scheme)
	api.Install(scheme)
	legacy.InstallExternalLegacyAll(scheme)
	codecs := serializer.NewCodecFactory(scheme)
	decoder := codecs.UniversalDeserializer()
	obj, err := runtime.Decode(decoder, data)
	if err != nil {
		return g, objs, err
	}
	if !meta.IsListType(obj) {
		objs = []runtime.Object{obj}
	} else {
		list, err := meta.ExtractList(obj)
		if err != nil {
			return g, objs, err
		}
		errs := runtime.DecodeList(list, decoder)
		if len(errs) > 0 {
			return g, objs, errs[0]
		}
		objs = list
	}

	for _, obj := range objs {
		if err := EnsureNode(g, obj); err != nil {
			return g, objs, err
		}
	}

	return g, objs, nil
}
