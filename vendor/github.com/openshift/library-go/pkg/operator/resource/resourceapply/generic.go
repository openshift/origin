package resourceapply

import (
	"fmt"

	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"

	"github.com/openshift/library-go/pkg/operator/v1helpers"

	"github.com/openshift/api"
	"github.com/openshift/library-go/pkg/operator/events"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
)

var (
	genericScheme = runtime.NewScheme()
	genericCodecs = serializer.NewCodecFactory(genericScheme)
	genericCodec  = genericCodecs.UniversalDeserializer()
)

func init() {
	utilruntime.Must(api.InstallKube(genericScheme))
	utilruntime.Must(apiextensionsv1beta1.AddToScheme(genericScheme))
}

type AssetFunc func(name string) ([]byte, error)

type ApplyResult struct {
	File    string
	Type    string
	Result  runtime.Object
	Changed bool
	Error   error
}

type ClientHolder struct {
	kubeClient          kubernetes.Interface
	apiExtensionsClient apiextensionsclient.Interface
	kubeInformers       v1helpers.KubeInformersForNamespaces
}

func NewClientHolder() *ClientHolder {
	return &ClientHolder{}
}

func NewKubeClientHolder(client kubernetes.Interface) *ClientHolder {
	return NewClientHolder().WithKubernetes(client)
}

func (c *ClientHolder) WithKubernetes(client kubernetes.Interface) *ClientHolder {
	c.kubeClient = client
	return c
}

func (c *ClientHolder) WithKubernetesInformers(kubeInformers v1helpers.KubeInformersForNamespaces) *ClientHolder {
	c.kubeInformers = kubeInformers
	return c
}

func (c *ClientHolder) WithAPIExtensionsClient(client apiextensionsclient.Interface) *ClientHolder {
	c.apiExtensionsClient = client
	return c
}

// ApplyDirectly applies the given manifest files to API server.
func ApplyDirectly(clients *ClientHolder, recorder events.Recorder, manifests AssetFunc, files ...string) []ApplyResult {
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

		// NOTE: Do not add CR resources into this switch otherwise the protobuf client can cause problems.
		switch t := requiredObj.(type) {
		case *corev1.Namespace:
			if clients.kubeClient == nil {
				result.Error = fmt.Errorf("missing kubeClient")
			}
			result.Result, result.Changed, result.Error = ApplyNamespace(clients.kubeClient.CoreV1(), recorder, t)
		case *corev1.Service:
			if clients.kubeClient == nil {
				result.Error = fmt.Errorf("missing kubeClient")
			}
			result.Result, result.Changed, result.Error = ApplyService(clients.kubeClient.CoreV1(), recorder, t)
		case *corev1.Pod:
			if clients.kubeClient == nil {
				result.Error = fmt.Errorf("missing kubeClient")
			}
			result.Result, result.Changed, result.Error = ApplyPod(clients.kubeClient.CoreV1(), recorder, t)
		case *corev1.ServiceAccount:
			if clients.kubeClient == nil {
				result.Error = fmt.Errorf("missing kubeClient")
			}
			result.Result, result.Changed, result.Error = ApplyServiceAccount(clients.kubeClient.CoreV1(), recorder, t)
		case *corev1.ConfigMap:
			client := clients.configMapsGetter()
			if client == nil {
				result.Error = fmt.Errorf("missing kubeClient")
			}
			result.Result, result.Changed, result.Error = ApplyConfigMap(client, recorder, t)
		case *corev1.Secret:
			client := clients.secretsGetter()
			if client == nil {
				result.Error = fmt.Errorf("missing kubeClient")
			}
			result.Result, result.Changed, result.Error = ApplySecret(client, recorder, t)
		case *rbacv1.ClusterRole:
			if clients.kubeClient == nil {
				result.Error = fmt.Errorf("missing kubeClient")
			}
			result.Result, result.Changed, result.Error = ApplyClusterRole(clients.kubeClient.RbacV1(), recorder, t)
		case *rbacv1.ClusterRoleBinding:
			if clients.kubeClient == nil {
				result.Error = fmt.Errorf("missing kubeClient")
			}
			result.Result, result.Changed, result.Error = ApplyClusterRoleBinding(clients.kubeClient.RbacV1(), recorder, t)
		case *rbacv1.Role:
			if clients.kubeClient == nil {
				result.Error = fmt.Errorf("missing kubeClient")
			}
			result.Result, result.Changed, result.Error = ApplyRole(clients.kubeClient.RbacV1(), recorder, t)
		case *rbacv1.RoleBinding:
			if clients.kubeClient == nil {
				result.Error = fmt.Errorf("missing kubeClient")
			}
			result.Result, result.Changed, result.Error = ApplyRoleBinding(clients.kubeClient.RbacV1(), recorder, t)
		case *apiextensionsv1beta1.CustomResourceDefinition:
			if clients.apiExtensionsClient == nil {
				result.Error = fmt.Errorf("missing apiExtensionsClient")
			}
			result.Result, result.Changed, result.Error = ApplyCustomResourceDefinitionV1Beta1(clients.apiExtensionsClient.ApiextensionsV1beta1(), recorder, t)
		case *apiextensionsv1.CustomResourceDefinition:
			if clients.apiExtensionsClient == nil {
				result.Error = fmt.Errorf("missing apiExtensionsClient")
			}
			result.Result, result.Changed, result.Error = ApplyCustomResourceDefinitionV1(clients.apiExtensionsClient.ApiextensionsV1(), recorder, t)
		case *storagev1.StorageClass:
			if clients.kubeClient == nil {
				result.Error = fmt.Errorf("missing kubeClient")
			}
			result.Result, result.Changed, result.Error = ApplyStorageClass(clients.kubeClient.StorageV1(), recorder, t)
		case *storagev1.CSIDriver:
			if clients.kubeClient == nil {
				result.Error = fmt.Errorf("missing kubeClient")
			}
			result.Result, result.Changed, result.Error = ApplyCSIDriver(clients.kubeClient.StorageV1(), recorder, t)
		default:
			result.Error = fmt.Errorf("unhandled type %T", requiredObj)
		}

		ret = append(ret, result)
	}

	return ret
}

func (c *ClientHolder) configMapsGetter() corev1client.ConfigMapsGetter {
	if c.kubeClient == nil {
		return nil
	}
	if c.kubeInformers == nil {
		return c.kubeClient.CoreV1()
	}
	return v1helpers.CachedConfigMapGetter(c.kubeClient.CoreV1(), c.kubeInformers)
}

func (c *ClientHolder) secretsGetter() corev1client.SecretsGetter {
	if c.kubeClient == nil {
		return nil
	}
	if c.kubeInformers == nil {
		return c.kubeClient.CoreV1()
	}
	return v1helpers.CachedSecretGetter(c.kubeClient.CoreV1(), c.kubeInformers)
}
