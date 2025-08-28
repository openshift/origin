package authentication

import (
	"context"
	"fmt"
	"path"
	"time"

	"github.com/go-logr/logr"
	routev1 "github.com/openshift/api/route/v1"
	typedroutev1 "github.com/openshift/client-go/route/clientset/versioned/typed/route/v1"
	exutil "github.com/openshift/origin/test/extended/util"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	typedappsv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/utils/ptr"
)

const (
	keycloakResourceName          = "keycloak"
	keycloakServingCertSecretName = "keycloak-serving-cert"
	keycloakLabelKey              = "app"
	keycloakLabelValue            = "keycloak"
	keycloakHTTPSPort             = 8443

	// TODO: should this be an openshift image?
	keycloakImage          = "quay.io/keycloak/keycloak:25.0"
	keycloakAdminUsername  = "admin"
	keycloakAdminPassword  = "password"
	keycloakCertVolumeName = "certkeypair"
	keycloakCertMountPath  = "/etc/x509/https"
	keycloakCertFile       = "tls.crt"
	keycloakKeyFile        = "tls.key"
)

func deployKeycloak(ctx context.Context, client *exutil.CLI, namespace string, logger logr.Logger) ([]removalFunc, error) {
	cleanups := []removalFunc{}

	corev1Client := client.AdminKubeClient().CoreV1()

	cleanup, err := createKeycloakNamespace(ctx, corev1Client.Namespaces(), namespace)
	if err != nil {
		return cleanups, fmt.Errorf("creating namespace for keycloak: %w", err)
	}
	cleanups = append(cleanups, cleanup)

	cleanup, err = createKeycloakServiceAccount(ctx, corev1Client.ServiceAccounts(namespace))
	if err != nil {
		return cleanups, fmt.Errorf("creating serviceaccount for keycloak: %w", err)
	}
	cleanups = append(cleanups, cleanup)

	service, cleanup, err := createKeycloakService(ctx, corev1Client.Services(namespace))
	if err != nil {
		return cleanups, fmt.Errorf("creating service for keycloak: %w", err)
	}
	cleanups = append(cleanups, cleanup)

	cleanup, err = createKeycloakDeployment(ctx, client.AdminKubeClient().AppsV1().Deployments(namespace))
	if err != nil {
		return cleanups, fmt.Errorf("creating deployment for keycloak: %w", err)
	}
	cleanups = append(cleanups, cleanup)

	cleanup, err = createKeycloakRoute(ctx, service, client.AdminRouteClient().RouteV1().Routes(namespace))
	if err != nil {
		return cleanups, fmt.Errorf("creating route for keycloak: %w", err)
	}
	cleanups = append(cleanups, cleanup)

	cleanup, err = createKeycloakCAConfigMap(ctx, corev1Client)
	if err != nil {
		return cleanups, fmt.Errorf("creating CA configmap for keycloak: %w", err)
	}
	cleanups = append(cleanups, cleanup)

	return cleanups, waitForKeycloakAvailable(ctx, client, namespace, logger)
}

func createKeycloakNamespace(ctx context.Context, client typedcorev1.NamespaceInterface, namespace string) (removalFunc, error) {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}

	_, err := client.Create(ctx, ns, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return nil, fmt.Errorf("creating serviceaccount: %w", err)
	}

	return func(ctx context.Context) error {
		return client.Delete(ctx, ns.Name, metav1.DeleteOptions{})
	}, nil
}

func createKeycloakServiceAccount(ctx context.Context, client typedcorev1.ServiceAccountInterface) (removalFunc, error) {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name: keycloakResourceName,
		},
	}
	sa.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("ServiceAccount"))

	_, err := client.Create(ctx, sa, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return nil, fmt.Errorf("creating serviceaccount: %w", err)
	}

	return func(ctx context.Context) error {
		return client.Delete(ctx, sa.Name, metav1.DeleteOptions{})
	}, nil
}

func createKeycloakService(ctx context.Context, client typedcorev1.ServiceInterface) (*corev1.Service, removalFunc, error) {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: keycloakResourceName,
			Annotations: map[string]string{
				"service.beta.openshift.io/serving-cert-secret-name": keycloakServingCertSecretName,
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: keycloakLabels(),
			Ports: []corev1.ServicePort{
				{
					Name: "https",
					Port: keycloakHTTPSPort,
				},
			},
		},
	}
	service.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Service"))

	_, err := client.Create(ctx, service, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return nil, nil, fmt.Errorf("creating service: %w", err)
	}

	return service, func(ctx context.Context) error {
		return client.Delete(ctx, service.Name, metav1.DeleteOptions{})
	}, nil
}

func createKeycloakCAConfigMap(ctx context.Context, client typedcorev1.ConfigMapsGetter) (removalFunc, error) {
	defaultIngressCACM, err := client.ConfigMaps("openshift-config-managed").Get(ctx, "default-ingress-cert", metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting configmap openshift-config-managed/default-ingress-cert: %w", err)
	}

	data := defaultIngressCACM.Data["ca-bundle.crt"]

	keycloakCACM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-ca", keycloakResourceName),
		},
		Data: map[string]string{
			"ca-bundle.crt": data,
		},
	}
	keycloakCACM.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("ConfigMap"))

	_, err = client.ConfigMaps("openshift-config").Create(ctx, keycloakCACM, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return nil, fmt.Errorf("creating configmap: %w", err)
	}

	return func(ctx context.Context) error {
		return client.ConfigMaps("openshift-config").Delete(ctx, keycloakCACM.Name, metav1.DeleteOptions{})
	}, nil
}

func createKeycloakDeployment(ctx context.Context, client typedappsv1.DeploymentInterface) (removalFunc, error) {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:   keycloakResourceName,
			Labels: keycloakLabels(),
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: keycloakLabels(),
			},
			Replicas: ptr.To(int32(1)),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:   keycloakResourceName,
					Labels: keycloakLabels(),
				},
				Spec: corev1.PodSpec{
					Containers: keycloakContainers(),
					Volumes:    keycloakVolumes(),
				},
			},
		},
	}
	deployment.SetGroupVersionKind(appsv1.SchemeGroupVersion.WithKind("Deployment"))

	_, err := client.Create(ctx, deployment, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return nil, fmt.Errorf("creating deployment: %w", err)
	}

	return func(ctx context.Context) error {
		return client.Delete(ctx, deployment.Name, metav1.DeleteOptions{})
	}, nil
}

func keycloakLabels() map[string]string {
	return map[string]string{
		keycloakLabelKey: keycloakLabelValue,
	}
}

func keycloakReadinessProbe() *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path:   "/health/ready",
				Port:   intstr.FromInt(9000),
				Scheme: corev1.URISchemeHTTPS,
			},
		},
		InitialDelaySeconds: 10,
	}
}

func keycloakLivenessProbe() *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path:   "/health/live",
				Port:   intstr.FromInt(9000),
				Scheme: corev1.URISchemeHTTPS,
			},
		},
		InitialDelaySeconds: 10,
	}
}

func keycloakStartupProbe() *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path:   "/health/started",
				Port:   intstr.FromInt(9000),
				Scheme: corev1.URISchemeHTTPS,
			},
		},
		FailureThreshold: 20,
		PeriodSeconds:    10,
	}
}

func keycloakEnvVars() []corev1.EnvVar {
	return []corev1.EnvVar{
		{
			Name:  "KEYCLOAK_ADMIN",
			Value: keycloakAdminUsername,
		},
		{
			Name:  "KEYCLOAK_ADMIN_PASSWORD",
			Value: keycloakAdminPassword,
		},
		{
			Name:  "KC_HEALTH_ENABLED",
			Value: "true",
		},
		{
			Name:  "KC_HOSTNAME_STRICT",
			Value: "false",
		},
		{
			Name:  "KC_PROXY",
			Value: "reencrypt",
		},
		{
			Name:  "KC_HTTPS_CERTIFICATE_FILE",
			Value: path.Join(keycloakCertMountPath, keycloakCertFile),
		},
		{
			Name:  "KC_HTTPS_CERTIFICATE_KEY_FILE",
			Value: path.Join(keycloakCertMountPath, keycloakKeyFile),
		},
	}
}

func keycloakVolumes() []corev1.Volume {
	return []corev1.Volume{
		{
			Name: keycloakCertVolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: keycloakServingCertSecretName,
				},
			},
		},
	}
}

func keycloakVolumeMounts() []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{
			Name:      keycloakCertVolumeName,
			MountPath: keycloakCertMountPath,
			ReadOnly:  true,
		},
	}
}

func keycloakContainers() []corev1.Container {
	return []corev1.Container{
		{
			Name:         "keycloak",
			Image:        keycloakImage,
			Env:          keycloakEnvVars(),
			VolumeMounts: keycloakVolumeMounts(),
			Ports: []corev1.ContainerPort{
				{
					ContainerPort: keycloakHTTPSPort,
				},
			},
			LivenessProbe:  keycloakLivenessProbe(),
			ReadinessProbe: keycloakReadinessProbe(),
			StartupProbe:   keycloakStartupProbe(),
			Command: []string{
				"/opt/keycloak/bin/kc.sh",
				"start-dev",
			},
		},
	}
}

func createKeycloakRoute(ctx context.Context, service *corev1.Service, client typedroutev1.RouteInterface) (removalFunc, error) {
	route := &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name: keycloakResourceName,
		},
		Spec: routev1.RouteSpec{
			TLS: &routev1.TLSConfig{
				Termination:                   routev1.TLSTerminationReencrypt,
				InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
			},
			To: routev1.RouteTargetReference{
				Kind: "Service",
				Name: service.Name,
			},
			Port: &routev1.RoutePort{
				TargetPort: intstr.FromString("https"),
			},
		},
	}
	route.SetGroupVersionKind(routev1.SchemeGroupVersion.WithKind("Route"))

	_, err := client.Create(ctx, route, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return nil, fmt.Errorf("creating route: %w", err)
	}

	return func(ctx context.Context) error {
		return client.Delete(ctx, route.Name, metav1.DeleteOptions{})
	}, nil
}

func waitForKeycloakAvailable(ctx context.Context, client *exutil.CLI, namespace string, logger logr.Logger) error {
	timeoutCtx, cancel := context.WithDeadline(ctx, time.Now().Add(10*time.Minute))
	defer cancel()
	err := wait.PollUntilContextCancel(timeoutCtx, 10*time.Second, true, func(ctx context.Context) (done bool, err error) {
		deploy, err := client.AdminKubeClient().AppsV1().Deployments(namespace).Get(ctx, keycloakResourceName, metav1.GetOptions{})
		if err != nil {
			logger.Error(err, "getting keycloak deployment")
			return false, nil
		}

		for _, condition := range deploy.Status.Conditions {
			if condition.Type == appsv1.DeploymentAvailable && condition.Status == corev1.ConditionTrue {
				return true, nil
			}
		}

		logger.Info("keycloak deployment is not yet available", "status", deploy.Status)

		return false, nil
	})

	return err
}
