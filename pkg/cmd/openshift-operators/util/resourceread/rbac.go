package resourceread

import (
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

var (
	rbacScheme = runtime.NewScheme()
	rbacCodecs = serializer.NewCodecFactory(rbacScheme)
)

func init() {
	if err := rbacv1.AddToScheme(rbacScheme); err != nil {
		panic(err)
	}
}

func ReadClusterRoleBindingOrDie(objBytes []byte) *rbacv1.ClusterRoleBinding {
	requiredObj, err := runtime.Decode(rbacCodecs.UniversalDecoder(rbacv1.SchemeGroupVersion), objBytes)
	if err != nil {
		panic(err)
	}
	return requiredObj.(*rbacv1.ClusterRoleBinding)
}
