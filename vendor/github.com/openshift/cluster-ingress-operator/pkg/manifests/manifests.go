package manifests

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"

	operatorcontroller "github.com/openshift/cluster-ingress-operator/pkg/operator/controller"

	operatorv1 "github.com/openshift/api/operator/v1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/apiserver/pkg/storage/names"

	routev1 "github.com/openshift/api/route/v1"
)

const (
	RouterNamespaceAsset          = "assets/router/namespace.yaml"
	RouterServiceAccountAsset     = "assets/router/service-account.yaml"
	RouterClusterRoleAsset        = "assets/router/cluster-role.yaml"
	RouterClusterRoleBindingAsset = "assets/router/cluster-role-binding.yaml"
	RouterDeploymentAsset         = "assets/router/deployment.yaml"
	RouterServiceInternalAsset    = "assets/router/service-internal.yaml"
	RouterServiceCloudAsset       = "assets/router/service-cloud.yaml"

	MetricsClusterRoleAsset        = "assets/router/metrics/cluster-role.yaml"
	MetricsClusterRoleBindingAsset = "assets/router/metrics/cluster-role-binding.yaml"
	MetricsRoleAsset               = "assets/router/metrics/role.yaml"
	MetricsRoleBindingAsset        = "assets/router/metrics/role-binding.yaml"

	CanaryNamespaceAsset = "assets/canary/namespace.yaml"
	CanaryDaemonSetAsset = "assets/canary/daemonset.yaml"
	CanaryServiceAsset   = "assets/canary/service.yaml"
	CanaryRouteAsset     = "assets/canary/route.yaml"

	// Annotation used to inform the certificate generation service to
	// generate a cluster-signed certificate and populate the secret.
	ServingCertSecretAnnotation = "service.alpha.openshift.io/serving-cert-secret-name"

	// OwningIngressControllerLabel should be applied to any objects "owned by" a
	// ingress controller to aid in selection (especially in cases where an ownerref
	// can't be established due to namespace boundaries).
	OwningIngressControllerLabel = "ingresscontroller.operator.openshift.io/owning-ingresscontroller"

	// OwningIngressCanaryCheckLabel should be applied to any objects "owned by" the
	// ingress operator's canary end-to-end check controller.
	OwningIngressCanaryCheckLabel = "ingress.openshift.io/canary"

	// IngressControllerFinalizer is used to block deletion of ingresscontrollers
	// until the operator has ensured it's safe for deletion to proceed.
	IngressControllerFinalizer = "ingresscontroller.operator.openshift.io/finalizer-ingresscontroller"

	// DNSRecordFinalizer is used to block deletion of dnsrecords until the
	// operator has ensured it's safe for deletion to proceeed.
	DNSRecordFinalizer = "operator.openshift.io/ingress-dns"

	// DefaultIngressControllerName is the name of the default IngressController
	// instance.
	DefaultIngressControllerName = "default"

	// ClusterIngressConfigName is the name of the cluster Ingress Config
	ClusterIngressConfigName = "cluster"

	NamespaceManifest                = "manifests/00-namespace.yaml"
	CustomResourceDefinitionManifest = "manifests/00-custom-resource-definition.yaml"
)

func MustAssetReader(asset string) io.Reader {
	return bytes.NewReader(MustAsset(asset))
}

func RouterNamespace() *corev1.Namespace {
	ns, err := NewNamespace(MustAssetReader(RouterNamespaceAsset))
	if err != nil {
		panic(err)
	}
	return ns
}

func RouterServiceAccount() *corev1.ServiceAccount {
	sa, err := NewServiceAccount(MustAssetReader(RouterServiceAccountAsset))
	if err != nil {
		panic(err)
	}
	return sa
}

func RouterClusterRole() *rbacv1.ClusterRole {
	cr, err := NewClusterRole(MustAssetReader(RouterClusterRoleAsset))
	if err != nil {
		panic(err)
	}
	return cr
}

func RouterClusterRoleBinding() *rbacv1.ClusterRoleBinding {
	crb, err := NewClusterRoleBinding(MustAssetReader(RouterClusterRoleBindingAsset))
	if err != nil {
		panic(err)
	}
	return crb
}

func RouterStatsSecret(cr *operatorv1.IngressController) *corev1.Secret {
	s := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("router-stats-%s", cr.Name),
			Namespace: operatorcontroller.DefaultOperandNamespace,
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{},
	}

	generatedUser := names.SimpleNameGenerator.GenerateName("user")
	generatedPassword := names.SimpleNameGenerator.GenerateName("pass")
	s.Data["statsUsername"] = []byte(base64.StdEncoding.EncodeToString([]byte(generatedUser)))
	s.Data["statsPassword"] = []byte(base64.StdEncoding.EncodeToString([]byte(generatedPassword)))
	return s
}

func RouterDeployment() *appsv1.Deployment {
	deployment, err := NewDeployment(MustAssetReader(RouterDeploymentAsset))
	if err != nil {
		panic(err)
	}
	return deployment
}

func InternalIngressControllerService() *corev1.Service {
	s, err := NewService(MustAssetReader(RouterServiceInternalAsset))
	if err != nil {
		panic(err)
	}
	return s
}

func LoadBalancerService() *corev1.Service {
	s, err := NewService(MustAssetReader(RouterServiceCloudAsset))
	if err != nil {
		panic(err)
	}
	return s
}

func MetricsClusterRole() *rbacv1.ClusterRole {
	cr, err := NewClusterRole(MustAssetReader(MetricsClusterRoleAsset))
	if err != nil {
		panic(err)
	}
	return cr
}

func MetricsClusterRoleBinding() *rbacv1.ClusterRoleBinding {
	crb, err := NewClusterRoleBinding(MustAssetReader(MetricsClusterRoleBindingAsset))
	if err != nil {
		panic(err)
	}
	return crb
}

func MetricsRole() *rbacv1.Role {
	r, err := NewRole(MustAssetReader(MetricsRoleAsset))
	if err != nil {
		panic(err)
	}
	return r
}

func MetricsRoleBinding() *rbacv1.RoleBinding {
	rb, err := NewRoleBinding(MustAssetReader(MetricsRoleBindingAsset))
	if err != nil {
		panic(err)
	}
	return rb
}

func CanaryNamespace() *corev1.Namespace {
	ns, err := NewNamespace(MustAssetReader(CanaryNamespaceAsset))
	if err != nil {
		panic(err)
	}
	return ns
}

func CanaryDaemonSet() *appsv1.DaemonSet {
	daemonset, err := NewDaemonSet(MustAssetReader(CanaryDaemonSetAsset))
	if err != nil {
		panic(err)
	}
	return daemonset
}

func CanaryService() *corev1.Service {
	service, err := NewService(MustAssetReader(CanaryServiceAsset))
	if err != nil {
		panic(err)
	}
	return service
}

func CanaryRoute() *routev1.Route {
	route, err := NewRoute(MustAssetReader(CanaryRouteAsset))
	if err != nil {
		panic(err)
	}
	return route
}

func NewServiceAccount(manifest io.Reader) (*corev1.ServiceAccount, error) {
	sa := corev1.ServiceAccount{}
	if err := yaml.NewYAMLOrJSONDecoder(manifest, 100).Decode(&sa); err != nil {
		return nil, err
	}

	return &sa, nil
}

func NewRole(manifest io.Reader) (*rbacv1.Role, error) {
	r := rbacv1.Role{}
	if err := yaml.NewYAMLOrJSONDecoder(manifest, 100).Decode(&r); err != nil {
		return nil, err
	}

	return &r, nil
}

func NewRoleBinding(manifest io.Reader) (*rbacv1.RoleBinding, error) {
	rb := rbacv1.RoleBinding{}
	if err := yaml.NewYAMLOrJSONDecoder(manifest, 100).Decode(&rb); err != nil {
		return nil, err
	}

	return &rb, nil
}

func NewClusterRole(manifest io.Reader) (*rbacv1.ClusterRole, error) {
	cr := rbacv1.ClusterRole{}
	if err := yaml.NewYAMLOrJSONDecoder(manifest, 100).Decode(&cr); err != nil {
		return nil, err
	}

	return &cr, nil
}

func NewClusterRoleBinding(manifest io.Reader) (*rbacv1.ClusterRoleBinding, error) {
	crb := rbacv1.ClusterRoleBinding{}
	if err := yaml.NewYAMLOrJSONDecoder(manifest, 100).Decode(&crb); err != nil {
		return nil, err
	}

	return &crb, nil
}

func NewService(manifest io.Reader) (*corev1.Service, error) {
	s := corev1.Service{}
	if err := yaml.NewYAMLOrJSONDecoder(manifest, 100).Decode(&s); err != nil {
		return nil, err
	}

	return &s, nil
}

func NewNamespace(manifest io.Reader) (*corev1.Namespace, error) {
	ns := corev1.Namespace{}
	if err := yaml.NewYAMLOrJSONDecoder(manifest, 100).Decode(&ns); err != nil {
		return nil, err
	}

	return &ns, nil
}

func NewDeployment(manifest io.Reader) (*appsv1.Deployment, error) {
	o := appsv1.Deployment{}
	if err := yaml.NewYAMLOrJSONDecoder(manifest, 100).Decode(&o); err != nil {
		return nil, err
	}

	return &o, nil
}

func NewDaemonSet(manifest io.Reader) (*appsv1.DaemonSet, error) {
	o := appsv1.DaemonSet{}
	if err := yaml.NewYAMLOrJSONDecoder(manifest, 100).Decode(&o); err != nil {
		return nil, err
	}

	return &o, nil
}

func NewRoute(manifest io.Reader) (*routev1.Route, error) {
	o := routev1.Route{}
	if err := yaml.NewYAMLOrJSONDecoder(manifest, 100).Decode(&o); err != nil {
		return nil, err
	}

	return &o, nil
}
