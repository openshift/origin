package controller

import (
	"fmt"

	operatorv1 "github.com/openshift/api/operator/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	// GlobalMachineSpecifiedConfigNamespace is the location for global
	// config.  In particular, the operator will put the configmap with the
	// CA certificate in this namespace.
	GlobalMachineSpecifiedConfigNamespace = "openshift-config-managed"

	// GlobalUserSpecifiedConfigNamespace is the namespace for configuring OpenShift.
	GlobalUserSpecifiedConfigNamespace = "openshift-config"

	// ControllerDeploymentLabel identifies a deployment as an ingress controller
	// deployment, and the value is the name of the owning ingress controller.
	ControllerDeploymentLabel = "ingresscontroller.operator.openshift.io/deployment-ingresscontroller"

	// ControllerDeploymentHashLabel identifies an ingress controller
	// deployment's generation.  This label is used for affinity, to
	// colocate replicas of different generations of the same ingress
	// controller, and for anti-affinity, to prevent colocation of replicas
	// of the same generation of the same ingress controller.
	ControllerDeploymentHashLabel = "ingresscontroller.operator.openshift.io/hash"

	// CanaryDaemonsetLabel identifies a daemonset as an ingress canary daemonset, and
	// the value is the name of the owning canary controller.
	CanaryDaemonSetLabel = "ingresscanary.operator.openshift.io/daemonset-ingresscanary"

	DefaultOperatorNamespace = "openshift-ingress-operator"
	DefaultOperandNamespace  = "openshift-ingress"
	SourceConfigMapNamespace = "openshift-config"

	// DefaultCanaryNamespace is the default namespace for
	// the ingress canary check resources.
	DefaultCanaryNamespace = "openshift-ingress-canary"
)

// IngressClusterOperatorName returns the namespaced name of the ClusterOperator
// resource for the operator.
func IngressClusterOperatorName() types.NamespacedName {
	return types.NamespacedName{
		Name: "ingress",
	}
}

// IngressClusterConfigName returns the namespaced name of the ingress.config.openshift.io
// resource for the operator.
func IngressClusterConfigName() types.NamespacedName {
	return types.NamespacedName{
		Name: "cluster",
	}
}

// InfrastructureClusterConfigName returns the namespaced name of the infrastructure.config.openshift.io
// resource of the cluster.
func InfrastructureClusterConfigName() types.NamespacedName {
	return types.NamespacedName{
		Name: "cluster",
	}
}

// RouterDeploymentName returns the namespaced name for the router deployment.
func RouterDeploymentName(ci *operatorv1.IngressController) types.NamespacedName {
	return types.NamespacedName{
		Namespace: DefaultOperandNamespace,
		Name:      "router-" + ci.Name,
	}
}

// RouterCASecretName returns the namespaced name for the router CA secret.
// This secret holds the CA certificate that the operator will use to create
// default certificates for ingresscontrollers.
func RouterCASecretName(operatorNamespace string) types.NamespacedName {
	return types.NamespacedName{
		Namespace: operatorNamespace,
		Name:      "router-ca",
	}
}

// DefaultIngressCertConfigMapName returns the namespaced name for the default ingress cert configmap.
// The operator uses this configmap to publish the public key that golang clients can use to trust
// the default ingress wildcard serving cert.
func DefaultIngressCertConfigMapName() types.NamespacedName {
	return types.NamespacedName{
		Namespace: GlobalMachineSpecifiedConfigNamespace,
		Name:      "default-ingress-cert",
	}
}

// RouterCertsGlobalSecretName returns the namespaced name for the router certs
// secret.  The operator uses this secret to publish the default certificates and
// their keys, so that the authentication operator can configure the OAuth server
// to use the same certificates.
func RouterCertsGlobalSecretName() types.NamespacedName {
	return types.NamespacedName{
		Namespace: GlobalMachineSpecifiedConfigNamespace,
		Name:      "router-certs",
	}
}

// RouterOperatorGeneratedDefaultCertificateSecretName returns the namespaced name for
// the operator-generated router default certificate secret.
func RouterOperatorGeneratedDefaultCertificateSecretName(ci *operatorv1.IngressController, namespace string) types.NamespacedName {
	return types.NamespacedName{
		Namespace: namespace,
		Name:      fmt.Sprintf("router-certs-%s", ci.Name),
	}
}

// ClientCAConfigMapName returns the namespaced name for the operator-managed
// client CA configmap, which is a copy of the user-managed configmap from the
// openshift-config namespace.
func ClientCAConfigMapName(ic *operatorv1.IngressController) types.NamespacedName {
	return types.NamespacedName{
		Namespace: "openshift-ingress",
		Name:      "router-client-ca-" + ic.Name,
	}
}

// CRLConfigMapName returns the namespaced name for the CRL configmap.
func CRLConfigMapName(ic *operatorv1.IngressController) types.NamespacedName {
	return types.NamespacedName{
		Namespace: "openshift-ingress",
		Name:      "router-client-ca-crl-" + ic.Name,
	}
}

// RsyslogConfigMapName returns the namespaced name for the rsyslog configmap.
func RsyslogConfigMapName(ic *operatorv1.IngressController) types.NamespacedName {
	return types.NamespacedName{
		Namespace: DefaultOperandNamespace,
		Name:      "rsyslog-conf-" + ic.Name,
	}
}

// HttpErrorCodePageConfigMapName returns the namespaced name for the errorpage configmap.
func HttpErrorCodePageConfigMapName(ic *operatorv1.IngressController) types.NamespacedName {
	return types.NamespacedName{
		Namespace: DefaultOperandNamespace,
		Name:      ic.Name + "-errorpages",
	}
}

// RouterPodDisruptionBudgetName returns the namespaced name for the router
// deployment's pod disruption budget.
func RouterPodDisruptionBudgetName(ic *operatorv1.IngressController) types.NamespacedName {
	return types.NamespacedName{
		Namespace: DefaultOperandNamespace,
		Name:      "router-" + ic.Name,
	}
}

// RouterEffectiveDefaultCertificateSecretName returns the namespaced name for
// the in-use router default certificate secret.
func RouterEffectiveDefaultCertificateSecretName(ci *operatorv1.IngressController, namespace string) types.NamespacedName {
	if cert := ci.Spec.DefaultCertificate; cert != nil {
		return types.NamespacedName{Namespace: namespace, Name: cert.Name}
	}
	return RouterOperatorGeneratedDefaultCertificateSecretName(ci, namespace)
}

// ServiceCAConfigMapName returns the namespaced name for the
// configmap with the service CA bundle.
func ServiceCAConfigMapName() types.NamespacedName {
	return types.NamespacedName{
		Namespace: DefaultOperandNamespace,
		Name:      "service-ca-bundle",
	}
}

func IngressControllerDeploymentLabel(ic *operatorv1.IngressController) string {
	return ic.Name
}

func IngressControllerDeploymentPodSelector(ic *operatorv1.IngressController) *metav1.LabelSelector {
	return &metav1.LabelSelector{
		MatchLabels: map[string]string{
			ControllerDeploymentLabel: IngressControllerDeploymentLabel(ic),
		},
	}
}

func InternalIngressControllerServiceName(ic *operatorv1.IngressController) types.NamespacedName {
	// TODO: remove hard-coded namespace
	return types.NamespacedName{Namespace: DefaultOperandNamespace, Name: "router-internal-" + ic.Name}
}

func IngressControllerServiceMonitorName(ic *operatorv1.IngressController) types.NamespacedName {
	return types.NamespacedName{
		Namespace: DefaultOperandNamespace,
		Name:      "router-" + ic.Name,
	}
}

func LoadBalancerServiceName(ic *operatorv1.IngressController) types.NamespacedName {
	return types.NamespacedName{Namespace: DefaultOperandNamespace, Name: "router-" + ic.Name}
}

func NodePortServiceName(ic *operatorv1.IngressController) types.NamespacedName {
	return types.NamespacedName{Namespace: DefaultOperandNamespace, Name: "router-nodeport-" + ic.Name}
}

func WildcardDNSRecordName(ic *operatorv1.IngressController) types.NamespacedName {
	return types.NamespacedName{
		Namespace: ic.Namespace,
		Name:      fmt.Sprintf("%s-wildcard", ic.Name),
	}
}

func CanaryDaemonSetName() types.NamespacedName {
	return types.NamespacedName{
		Namespace: DefaultCanaryNamespace,
		Name:      "ingress-canary",
	}
}

func CanaryDaemonSetPodSelector(canaryControllerName string) *metav1.LabelSelector {
	return &metav1.LabelSelector{
		MatchLabels: map[string]string{
			CanaryDaemonSetLabel: canaryControllerName,
		},
	}
}

func CanaryServiceName() types.NamespacedName {
	return types.NamespacedName{
		Namespace: DefaultCanaryNamespace,
		Name:      "ingress-canary",
	}
}

func CanaryRouteName() types.NamespacedName {
	return types.NamespacedName{
		Namespace: DefaultCanaryNamespace,
		Name:      "canary",
	}
}

func IngressClassName(ingressControllerName string) types.NamespacedName {
	return types.NamespacedName{Name: "openshift-" + ingressControllerName}
}
