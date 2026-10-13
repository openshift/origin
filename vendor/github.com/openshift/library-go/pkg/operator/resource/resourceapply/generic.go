package resourceapply

import (
	"context"
	"fmt"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	appsv1 "k8s.io/api/apps/v1"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	migrationv1alpha1 "sigs.k8s.io/kube-storage-version-migrator/pkg/apis/migration/v1alpha1"
	migrationclient "sigs.k8s.io/kube-storage-version-migrator/pkg/clients/clientset"

	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourceread"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
)

type AssetFunc func(name string) ([]byte, error)

type ApplyResult struct {
	File    string
	Type    string
	Result  runtime.Object
	Changed bool
	Error   error
}

// ConditionalFunction provides needed dependency for a resource on another condition instead of blindly creating
// a resource. This conditional function can also be used to delete the resource when not needed
type ConditionalFunction func() bool

type ClientHolder struct {
	kubeClient          kubernetes.Interface
	apiExtensionsClient apiextensionsclient.Interface
	kubeInformers       v1helpers.KubeInformersForNamespaces
	dynamicClient       dynamic.Interface
	migrationClient     migrationclient.Interface
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

func (c *ClientHolder) WithDynamicClient(client dynamic.Interface) *ClientHolder {
	c.dynamicClient = client
	return c
}

func (c *ClientHolder) WithMigrationClient(client migrationclient.Interface) *ClientHolder {
	c.migrationClient = client
	return c
}

// ApplyDirectly applies the given manifest files to API server.
func ApplyDirectly(ctx context.Context, clients *ClientHolder, recorder events.Recorder, cache ResourceCache, manifests AssetFunc, files ...string) []ApplyResult {
	ret := []ApplyResult{}

	for _, file := range files {
		result := ApplyResult{File: file}
		objBytes, err := manifests(file)
		if err != nil {
			result.Error = fmt.Errorf("missing %q: %v", file, err)
			ret = append(ret, result)
			continue
		}
		requiredObj, err := resourceread.ReadGenericWithUnstructured(objBytes)
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
			} else {
				result.Result, result.Changed, result.Error = ApplyNamespaceImproved(ctx, clients.kubeClient.CoreV1(), recorder, t, cache)
			}
		case *corev1.Service:
			if clients.kubeClient == nil {
				result.Error = fmt.Errorf("missing kubeClient")
			} else {
				result.Result, result.Changed, result.Error = ApplyServiceImproved(ctx, clients.kubeClient.CoreV1(), recorder, t, cache)
			}
		case *corev1.Pod:
			if clients.kubeClient == nil {
				result.Error = fmt.Errorf("missing kubeClient")
			} else {
				result.Result, result.Changed, result.Error = ApplyPodImproved(ctx, clients.kubeClient.CoreV1(), recorder, t, cache)
			}
		case *corev1.ServiceAccount:
			if clients.kubeClient == nil {
				result.Error = fmt.Errorf("missing kubeClient")
			} else {
				result.Result, result.Changed, result.Error = ApplyServiceAccountImproved(ctx, clients.kubeClient.CoreV1(), recorder, t, cache)
			}
		case *corev1.ConfigMap:
			client := clients.configMapsGetter()
			if client == nil {
				result.Error = fmt.Errorf("missing kubeClient")
			} else {
				result.Result, result.Changed, result.Error = ApplyConfigMapImproved(ctx, client, recorder, t, cache)
			}
		case *corev1.Secret:
			client := clients.secretsGetter()
			if client == nil {
				result.Error = fmt.Errorf("missing kubeClient")
			} else {
				result.Result, result.Changed, result.Error = ApplySecretImproved(ctx, client, recorder, t, cache)
			}
		case *networkingv1.NetworkPolicy:
			if clients.kubeClient == nil {
				result.Error = fmt.Errorf("missing kubeClient")
			} else {
				result.Result, result.Changed, result.Error = ApplyNetworkPolicy(ctx, clients.kubeClient.NetworkingV1(), recorder, t, cache)
			}
		case *rbacv1.ClusterRole:
			if clients.kubeClient == nil {
				result.Error = fmt.Errorf("missing kubeClient")
			} else {
				result.Result, result.Changed, result.Error = ApplyClusterRole(ctx, clients.kubeClient.RbacV1(), recorder, t)
			}
		case *rbacv1.ClusterRoleBinding:
			if clients.kubeClient == nil {
				result.Error = fmt.Errorf("missing kubeClient")
			} else {
				result.Result, result.Changed, result.Error = ApplyClusterRoleBinding(ctx, clients.kubeClient.RbacV1(), recorder, t)
			}
		case *rbacv1.Role:
			if clients.kubeClient == nil {
				result.Error = fmt.Errorf("missing kubeClient")
			} else {
				result.Result, result.Changed, result.Error = ApplyRole(ctx, clients.kubeClient.RbacV1(), recorder, t)
			}
		case *rbacv1.RoleBinding:
			if clients.kubeClient == nil {
				result.Error = fmt.Errorf("missing kubeClient")
			} else {
				result.Result, result.Changed, result.Error = ApplyRoleBinding(ctx, clients.kubeClient.RbacV1(), recorder, t)
			}
		case *policyv1.PodDisruptionBudget:
			if clients.kubeClient == nil {
				result.Error = fmt.Errorf("missing kubeClient")
			} else {
				result.Result, result.Changed, result.Error = ApplyPodDisruptionBudget(ctx, clients.kubeClient.PolicyV1(), recorder, t)
			}
		case *apiextensionsv1.CustomResourceDefinition:
			if clients.apiExtensionsClient == nil {
				result.Error = fmt.Errorf("missing apiExtensionsClient")
			} else {
				result.Result, result.Changed, result.Error = ApplyCustomResourceDefinitionV1(ctx, clients.apiExtensionsClient.ApiextensionsV1(), recorder, t)
			}
		case *storagev1.StorageClass:
			if clients.kubeClient == nil {
				result.Error = fmt.Errorf("missing kubeClient")
			} else {
				result.Result, result.Changed, result.Error = ApplyStorageClass(ctx, clients.kubeClient.StorageV1(), recorder, t)
			}
		case *admissionregistrationv1.ValidatingWebhookConfiguration:
			if clients.kubeClient == nil {
				result.Error = fmt.Errorf("missing kubeClient")
			} else {
				result.Result, result.Changed, result.Error = ApplyValidatingWebhookConfigurationImproved(ctx, clients.kubeClient.AdmissionregistrationV1(), recorder, t, cache)
			}
		case *admissionregistrationv1.MutatingWebhookConfiguration:
			if clients.kubeClient == nil {
				result.Error = fmt.Errorf("missing kubeClient")
			} else {
				result.Result, result.Changed, result.Error = ApplyMutatingWebhookConfigurationImproved(ctx, clients.kubeClient.AdmissionregistrationV1(), recorder, t, cache)
			}
		case *admissionregistrationv1beta1.ValidatingAdmissionPolicy:
			if clients.kubeClient == nil {
				result.Error = fmt.Errorf("missing kubeClient")
			} else {
				result.Result, result.Changed, result.Error = ApplyValidatingAdmissionPolicyV1beta1(ctx, clients.kubeClient.AdmissionregistrationV1beta1(), recorder, t, cache)
			}
		case *admissionregistrationv1beta1.ValidatingAdmissionPolicyBinding:
			if clients.kubeClient == nil {
				result.Error = fmt.Errorf("missing kubeClient")
			} else {
				result.Result, result.Changed, result.Error = ApplyValidatingAdmissionPolicyBindingV1beta1(ctx, clients.kubeClient.AdmissionregistrationV1beta1(), recorder, t, cache)
			}
		case *admissionregistrationv1.ValidatingAdmissionPolicy:
			if clients.kubeClient == nil {
				result.Error = fmt.Errorf("missing kubeClient")
			} else {
				result.Result, result.Changed, result.Error = ApplyValidatingAdmissionPolicyV1(ctx, clients.kubeClient.AdmissionregistrationV1(), recorder, t, cache)
			}
		case *admissionregistrationv1.ValidatingAdmissionPolicyBinding:
			if clients.kubeClient == nil {
				result.Error = fmt.Errorf("missing kubeClient")
			} else {
				result.Result, result.Changed, result.Error = ApplyValidatingAdmissionPolicyBindingV1(ctx, clients.kubeClient.AdmissionregistrationV1(), recorder, t, cache)
			}
		case *storagev1.CSIDriver:
			if clients.kubeClient == nil {
				result.Error = fmt.Errorf("missing kubeClient")
			} else {
				result.Result, result.Changed, result.Error = ApplyCSIDriver(ctx, clients.kubeClient.StorageV1(), recorder, t)
			}
		case *migrationv1alpha1.StorageVersionMigration:
			if clients.migrationClient == nil {
				result.Error = fmt.Errorf("missing migrationClient")
			} else {
				result.Result, result.Changed, result.Error = ApplyStorageVersionMigration(ctx, clients.migrationClient, recorder, t)
			}
		case *unstructured.Unstructured:
			if clients.dynamicClient == nil {
				result.Error = fmt.Errorf("missing dynamicClient")
			} else {
				result.Result, result.Changed, result.Error = ApplyKnownUnstructured(ctx, clients.dynamicClient, recorder, t)
			}
		default:
			result.Error = fmt.Errorf("unhandled type %T", requiredObj)
		}

		ret = append(ret, result)
	}

	return ret
}

func DeleteAll(ctx context.Context, clients *ClientHolder, recorder events.Recorder, manifests AssetFunc,
	files ...string) []ApplyResult {
	ret := []ApplyResult{}

	for _, file := range files {
		result := ApplyResult{File: file}
		objBytes, err := manifests(file)
		if err != nil {
			result.Error = fmt.Errorf("missing %q: %v", file, err)
			ret = append(ret, result)
			continue
		}
		requiredObj, err := resourceread.ReadGenericWithUnstructured(objBytes)
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
			} else {
				_, result.Changed, result.Error = DeleteNamespace(ctx, clients.kubeClient.CoreV1(), recorder, t)
			}
		case *corev1.Service:
			if clients.kubeClient == nil {
				result.Error = fmt.Errorf("missing kubeClient")
			} else {
				_, result.Changed, result.Error = DeleteService(ctx, clients.kubeClient.CoreV1(), recorder, t)
			}
		case *corev1.Pod:
			if clients.kubeClient == nil {
				result.Error = fmt.Errorf("missing kubeClient")
			} else {
				_, result.Changed, result.Error = DeletePod(ctx, clients.kubeClient.CoreV1(), recorder, t)
			}
		case *corev1.ServiceAccount:
			if clients.kubeClient == nil {
				result.Error = fmt.Errorf("missing kubeClient")
			} else {
				_, result.Changed, result.Error = DeleteServiceAccount(ctx, clients.kubeClient.CoreV1(), recorder, t)
			}
		case *corev1.ConfigMap:
			client := clients.configMapsGetter()
			if client == nil {
				result.Error = fmt.Errorf("missing kubeClient")
			} else {
				_, result.Changed, result.Error = DeleteConfigMap(ctx, client, recorder, t)
			}
		case *corev1.Secret:
			client := clients.secretsGetter()
			if client == nil {
				result.Error = fmt.Errorf("missing kubeClient")
			} else {
				_, result.Changed, result.Error = DeleteSecret(ctx, client, recorder, t)
			}
		case *networkingv1.NetworkPolicy:
			if clients.kubeClient == nil {
				result.Error = fmt.Errorf("missing kubeClient")
			} else {
				_, result.Changed, result.Error = DeleteNetworkPolicy(ctx, clients.kubeClient.NetworkingV1(), recorder, t)
			}
		case *rbacv1.ClusterRole:
			if clients.kubeClient == nil {
				result.Error = fmt.Errorf("missing kubeClient")
			} else {
				_, result.Changed, result.Error = DeleteClusterRole(ctx, clients.kubeClient.RbacV1(), recorder, t)
			}
		case *rbacv1.ClusterRoleBinding:
			if clients.kubeClient == nil {
				result.Error = fmt.Errorf("missing kubeClient")
			} else {
				_, result.Changed, result.Error = DeleteClusterRoleBinding(ctx, clients.kubeClient.RbacV1(), recorder, t)
			}
		case *rbacv1.Role:
			if clients.kubeClient == nil {
				result.Error = fmt.Errorf("missing kubeClient")
			} else {
				_, result.Changed, result.Error = DeleteRole(ctx, clients.kubeClient.RbacV1(), recorder, t)
			}
		case *rbacv1.RoleBinding:
			if clients.kubeClient == nil {
				result.Error = fmt.Errorf("missing kubeClient")
			} else {
				_, result.Changed, result.Error = DeleteRoleBinding(ctx, clients.kubeClient.RbacV1(), recorder, t)
			}
		case *appsv1.Deployment:
			if clients.kubeClient == nil {
				result.Error = fmt.Errorf("missing kubeClient")
			} else {
				_, result.Changed, result.Error = DeleteDeployment(ctx, clients.kubeClient.AppsV1(), recorder, t)
			}
		case *appsv1.DaemonSet:
			if clients.kubeClient == nil {
				result.Error = fmt.Errorf("missing kubeClient")
			} else {
				_, result.Changed, result.Error = DeleteDaemonSet(ctx, clients.kubeClient.AppsV1(), recorder, t)
			}
		case *policyv1.PodDisruptionBudget:
			if clients.kubeClient == nil {
				result.Error = fmt.Errorf("missing kubeClient")
			} else {
				_, result.Changed, result.Error = DeletePodDisruptionBudget(ctx, clients.kubeClient.PolicyV1(), recorder, t)
			}
		case *apiextensionsv1.CustomResourceDefinition:
			if clients.apiExtensionsClient == nil {
				result.Error = fmt.Errorf("missing apiExtensionsClient")
			} else {
				_, result.Changed, result.Error = DeleteCustomResourceDefinitionV1(ctx, clients.apiExtensionsClient.ApiextensionsV1(), recorder, t)
			}
		case *storagev1.StorageClass:
			if clients.kubeClient == nil {
				result.Error = fmt.Errorf("missing kubeClient")
			} else {
				_, result.Changed, result.Error = DeleteStorageClass(ctx, clients.kubeClient.StorageV1(), recorder, t)
			}
		case *admissionregistrationv1.ValidatingWebhookConfiguration:
			if clients.kubeClient == nil {
				result.Error = fmt.Errorf("missing kubeClient")
			} else {
				_, result.Changed, result.Error = DeleteValidatingWebhookConfiguration(ctx, clients.kubeClient.AdmissionregistrationV1(), recorder, t)
			}
		case *admissionregistrationv1beta1.ValidatingAdmissionPolicy:
			if clients.kubeClient == nil {
				result.Error = fmt.Errorf("missing kubeClient")
			} else {
				_, result.Changed, result.Error = DeleteValidatingAdmissionPolicyV1beta1(ctx, clients.kubeClient.AdmissionregistrationV1beta1(), recorder, t)
			}
		case *admissionregistrationv1beta1.ValidatingAdmissionPolicyBinding:
			if clients.kubeClient == nil {
				result.Error = fmt.Errorf("missing kubeClient")
			} else {
				_, result.Changed, result.Error = DeleteValidatingAdmissionPolicyBindingV1beta1(ctx, clients.kubeClient.AdmissionregistrationV1beta1(), recorder, t)
			}
		case *admissionregistrationv1.ValidatingAdmissionPolicy:
			if clients.kubeClient == nil {
				result.Error = fmt.Errorf("missing kubeClient")
			} else {
				_, result.Changed, result.Error = DeleteValidatingAdmissionPolicyV1(ctx, clients.kubeClient.AdmissionregistrationV1(), recorder, t)
			}
		case *admissionregistrationv1.ValidatingAdmissionPolicyBinding:
			if clients.kubeClient == nil {
				result.Error = fmt.Errorf("missing kubeClient")
			} else {
				_, result.Changed, result.Error = DeleteValidatingAdmissionPolicyBindingV1(ctx, clients.kubeClient.AdmissionregistrationV1(), recorder, t)
			}
		case *storagev1.CSIDriver:
			if clients.kubeClient == nil {
				result.Error = fmt.Errorf("missing kubeClient")
			} else {
				_, result.Changed, result.Error = DeleteCSIDriver(ctx, clients.kubeClient.StorageV1(), recorder, t)
			}
		case *migrationv1alpha1.StorageVersionMigration:
			if clients.migrationClient == nil {
				result.Error = fmt.Errorf("missing migrationClient")
			} else {
				_, result.Changed, result.Error = DeleteStorageVersionMigration(ctx, clients.migrationClient, recorder, t)
			}
		case *unstructured.Unstructured:
			if clients.dynamicClient == nil {
				result.Error = fmt.Errorf("missing dynamicClient")
			} else {
				_, result.Changed, result.Error = DeleteKnownUnstructured(ctx, clients.dynamicClient, recorder, t)
			}
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
