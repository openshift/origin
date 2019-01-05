package resourceapply

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"

	"github.com/openshift/api"
	"github.com/openshift/library-go/pkg/operator/events"
)

var (
	genericScheme = runtime.NewScheme()
	genericCodecs = serializer.NewCodecFactory(genericScheme)
	genericCodec  = genericCodecs.UniversalDeserializer()
)

func init() {
	utilruntime.Must(api.InstallKube(genericScheme))
}

type AssetFunc func(name string) ([]byte, error)

type ApplyResult struct {
	File    string
	Type    string
	Result  runtime.Object
	Changed bool
	Error   error
}

func ApplyDirectly(client kubernetes.Interface, recorder events.Recorder, manifests AssetFunc, files ...string) []ApplyResult {
	ret := []ApplyResult{}

	for _, file := range files {
		result := ApplyResult{File: file}
		objBytes, err := manifests(file)
		if err != nil {
			result.Error = fmt.Errorf("missing %q: %v", file, err)
			ret = append(ret, result)
			continue
		}
		requiredObj, _, err := genericCodec.Decode(objBytes, nil, nil)
		if err != nil {
			result.Error = fmt.Errorf("cannot decode %q: %v", file, err)
			ret = append(ret, result)
			continue
		}
		result.Type = fmt.Sprintf("%T", requiredObj)

		switch t := requiredObj.(type) {
		case *corev1.Namespace:
			result.Result, result.Changed, result.Error = ApplyNamespace(client.CoreV1(), recorder, t)
		case *corev1.Service:
			result.Result, result.Changed, result.Error = ApplyService(client.CoreV1(), recorder, t)
		case *corev1.ServiceAccount:
			result.Result, result.Changed, result.Error = ApplyServiceAccount(client.CoreV1(), recorder, t)
		case *corev1.ConfigMap:
			result.Result, result.Changed, result.Error = ApplyConfigMap(client.CoreV1(), recorder, t)
		case *corev1.Secret:
			result.Result, result.Changed, result.Error = ApplySecret(client.CoreV1(), recorder, t)
		case *rbacv1.ClusterRole:
			result.Result, result.Changed, result.Error = ApplyClusterRole(client.RbacV1(), recorder, t)
		case *rbacv1.ClusterRoleBinding:
			result.Result, result.Changed, result.Error = ApplyClusterRoleBinding(client.RbacV1(), recorder, t)
		case *rbacv1.Role:
			result.Result, result.Changed, result.Error = ApplyRole(client.RbacV1(), recorder, t)
		case *rbacv1.RoleBinding:
			result.Result, result.Changed, result.Error = ApplyRoleBinding(client.RbacV1(), recorder, t)
		default:
			result.Error = fmt.Errorf("unhandled type %T", requiredObj)
		}

		ret = append(ret, result)
	}

	return ret
}
