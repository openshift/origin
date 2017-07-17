package v1

import (
	"k8s.io/apimachinery/pkg/runtime"
	kapi "k8s.io/kubernetes/pkg/api"
)

func SetDefaults_ImagePolicyConfig(obj *ImagePolicyConfig) {
	if obj == nil {
		return
	}

	if len(obj.ResolveImages) == 0 {
		obj.ResolveImages = Attempt
	}

	if obj.ResolutionRules == nil {
		obj.ResolutionRules = []ImageResolutionPolicyRule{
			{TargetResource: GroupResource{Resource: "pods"}, LocalNames: true},
			{TargetResource: GroupResource{Group: "build.openshift.io", Resource: "builds"}, LocalNames: true},
			{TargetResource: GroupResource{Resource: "replicationcontrollers"}, LocalNames: true},
			{TargetResource: GroupResource{Group: "extensions", Resource: "replicasets"}, LocalNames: true},
			{TargetResource: GroupResource{Group: "batch", Resource: "jobs"}, LocalNames: true},

			// TODO: consider adding these
			// {TargetResource: GroupResource{Group: "extensions", Resource: "deployments"}, LocalNames: true},
			// {TargetResource: GroupResource{Group: "apps", Resource: "statefulsets"}, LocalNames: true},
		}
	}

	for i := range obj.ExecutionRules {
		if len(obj.ExecutionRules[i].OnResources) == 0 {
			obj.ExecutionRules[i].OnResources = []GroupResource{{Resource: "pods", Group: kapi.GroupName}}
		}
	}

}

func addDefaultingFuncs(scheme *runtime.Scheme) error {
	scheme.AddTypeDefaultingFunc(&ImagePolicyConfig{}, func(obj interface{}) { SetDefaults_ImagePolicyConfig(obj.(*ImagePolicyConfig)) })
	return nil
}
