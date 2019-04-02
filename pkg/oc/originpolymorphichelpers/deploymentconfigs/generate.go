package deploymentconfigs

import (
	"fmt"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kubernetes/pkg/kubectl/generate"
	"k8s.io/kubernetes/pkg/kubectl/generate/versioned"

	appsv1 "github.com/openshift/api/apps/v1"
)

var basic = versioned.BasicReplicationController{}

type BasicDeploymentConfigController struct{}

func (BasicDeploymentConfigController) ParamNames() []generate.GeneratorParam {
	return basic.ParamNames()
}

func (BasicDeploymentConfigController) Generate(genericParams map[string]interface{}) (runtime.Object, error) {
	obj, err := basic.Generate(genericParams)
	if err != nil {
		return nil, err
	}
	switch t := obj.(type) {
	case *corev1.ReplicationController:
		obj = &appsv1.DeploymentConfig{
			ObjectMeta: t.ObjectMeta,
			Spec: appsv1.DeploymentConfigSpec{
				Selector: t.Spec.Selector,
				Replicas: *t.Spec.Replicas, // the generator never leaves this nil
				Template: t.Spec.Template,
			},
		}
	default:
		return nil, fmt.Errorf("unrecognized object type: %v", reflect.TypeOf(t))
	}
	return obj, nil
}
