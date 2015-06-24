package test

import (
	"fmt"
	"reflect"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"

	osgraph "github.com/openshift/origin/pkg/api/graph"
	kubegraph "github.com/openshift/origin/pkg/api/kubegraph/nodes"
	buildapi "github.com/openshift/origin/pkg/build/api"
	buildgraph "github.com/openshift/origin/pkg/build/graph/nodes"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deploygraph "github.com/openshift/origin/pkg/deploy/graph/nodes"
	imageapi "github.com/openshift/origin/pkg/image/api"
	imagegraph "github.com/openshift/origin/pkg/image/graph/nodes"
)

// typeToEnsureMethod stores types to Ensure*Node methods
var typeToEnsureMethod = map[reflect.Type]reflect.Value{}

func init() {
	if err := RegisterEnsureNode(&imageapi.Image{}, imagegraph.EnsureImageNode); err != nil {
		panic(err)
	}
	if err := RegisterEnsureNode(&imageapi.ImageStream{}, imagegraph.EnsureImageStreamNode); err != nil {
		panic(err)
	}
	if err := RegisterEnsureNode(&deployapi.DeploymentConfig{}, deploygraph.EnsureDeploymentConfigNode); err != nil {
		panic(err)
	}
	if err := RegisterEnsureNode(&buildapi.BuildConfig{}, buildgraph.EnsureBuildConfigNode); err != nil {
		panic(err)
	}
	if err := RegisterEnsureNode(&buildapi.Build{}, buildgraph.EnsureBuildNode); err != nil {
		panic(err)
	}
	if err := RegisterEnsureNode(&kapi.Pod{}, kubegraph.EnsurePodNode); err != nil {
		panic(err)
	}
	if err := RegisterEnsureNode(&kapi.Secret{}, kubegraph.EnsureSecretNode); err != nil {
		panic(err)
	}
	if err := RegisterEnsureNode(&kapi.Service{}, kubegraph.EnsureServiceNode); err != nil {
		panic(err)
	}
	if err := RegisterEnsureNode(&kapi.ReplicationController{}, kubegraph.EnsureReplicationControllerNode); err != nil {
		panic(err)
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
		return fmt.Errorf("%v is not registered", reflectedContainedType)
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
