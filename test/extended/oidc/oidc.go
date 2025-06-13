package oidc

import (
	"context"
	"errors"
	"fmt"
	"path"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"
	routev1 "github.com/openshift/api/route/v1"
	exutil "github.com/openshift/origin/test/extended/util"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/pod-security-admission/api"
	"k8s.io/utils/ptr"
)

type kubeObject interface {
	runtime.Object
	metav1.Object
}

var _ = g.Describe("[sig-auth][Serial][Slow][OCPFeatureGate:ExternalOIDC] Configuring an external OIDC provider", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLI("external-oidc")
	oc.KubeFramework().NamespacePodSecurityLevel = api.LevelPrivileged
	var resources []kubeObject
	var originalAuth *configv1.Authentication

	g.BeforeAll(func() {
		var err error
		ctx := context.TODO()
		resources, err = deployKeycloak(ctx, oc)
		o.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error deploying keycloak")

		// TODO: Configure Keycloak

		original, _, err := configureOIDCAuthentication(ctx, oc)
		o.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error configuring OIDC authentication")
		originalAuth = original
	})

	g.AfterAll(func() {
		ctx := context.TODO()
		err := removeResources(ctx, oc, resources...)
		o.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error cleaning up keycloak resources")

		_, err = oc.AdminConfigClient().ConfigV1().Authentications().Update(ctx, originalAuth, metav1.UpdateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error reverting authentication to original state")
	})

	g.It("should rollout configuration on the kube-apiserver successfully", func() {
		g.Fail("not implemented")
	})

	g.It("should remove the OpenShift OAuth stack", func() {
		g.Fail("not implemented")
	})

	g.It("should not accept tokens provided by the OAuth server", func() {
		g.Fail("not implemented")
	})

	g.It("should accept tokens issued by the external IdP", func() {
		g.Fail("not implemented")
	})

	g.It("should accept authentication via a kubeconfig (break-glass)", func() {
		g.Fail("not implemented")
	})

	g.It("should map cluster identities correctly", func() {
		g.Fail("not implemented")
	})
})

var _ = g.Describe("[sig-auth][Serial][Slow][OCPFeatureGate:ExternalOIDC] Changing from OIDC authentication type to IntegratedOAuth", func() {
	defer g.GinkgoRecover()
	// oc := exutil.NewCLI("oidc")

	g.BeforeAll(func() {
		// TODO: Deploy Keycloak
		// TODO: Configure OIDC authentication type
		// TODO: Wait for sucessful rollout
		// TODO: Revert authentication configuration to previous configuration
	})

	g.AfterAll(func() {
		// TODO: Tear down Keycloak if exists
		// TODO: Revert authentication configuration to previous configuration
	})

	g.It("should rollout configuration on the kube-apiserver successfully", func() {
		g.Fail("not implemented")
	})

	g.It("should rollout the OpenShift OAuth stack", func() {
		g.Fail("not implemented")
	})

	g.It("should not accept tokens provided by an external IdP", func() {
		g.Fail("not implemented")
	})

	g.It("should accept tokens provided by the OpenShift OAuth server", func() {
		g.Fail("not implemented")
	})
})

// TODO: Add test skeleton for the ExternalOIDCWithUIDAndExtraClaimMappings feature gate

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

func deployKeycloak(ctx context.Context, client *exutil.CLI) ([]kubeObject, error) {
	resources := []kubeObject{}

	sa, err := createKeycloakServiceAccount(ctx, client)
	if err != nil {
		return resources, fmt.Errorf("creating serviceaccount for keycloak: %w", err)
	}
	resources = append(resources, sa)

	rb, err := createKeycloakPrivilegedSSARoleBinding(ctx, sa.Name, client)
	if err != nil {
		return resources, fmt.Errorf("creating privileged ssa rolebinding for keycloak: %w", err)
	}
	resources = append(resources, rb)

	service, err := createKeycloakService(ctx, client)
	if err != nil {
		return resources, fmt.Errorf("creating service for keycloak: %w", err)
	}
	resources = append(resources, service)

	dep, err := createKeycloakDeployment(ctx, client)
	if err != nil {
		return resources, fmt.Errorf("creating deployment for keycloak: %w", err)
	}
	resources = append(resources, dep)

	route, err := createKeycloakRoute(ctx, service, client)
	if err != nil {
		return resources, fmt.Errorf("creating route for keycloak: %w", err)
	}
	resources = append(resources, route)

	// TODO: CA bundle ConfigMap in the openshift-config namespace
	// Done by copying the default ingress CA in openshift-config-managed/default-ingress-cert

	return resources, nil
}

func createKeycloakServiceAccount(ctx context.Context, client *exutil.CLI) (*corev1.ServiceAccount, error) {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name: keycloakResourceName,
		},
	}

	return client.KubeClient().CoreV1().ServiceAccounts(client.Namespace()).Create(ctx, sa, metav1.CreateOptions{})
}

func createKeycloakPrivilegedSSARoleBinding(ctx context.Context, saName string, client *exutil.CLI) (*rbacv1.RoleBinding, error) {
	rb := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: keycloakResourceName,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.SchemeGroupVersion.Group,
			Kind:     "ClusterRole",
			Name:     "system:openshift:scc:privileged",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind: "ServiceAccount",
				Name: saName,
			},
		},
	}

	return client.KubeClient().RbacV1().RoleBindings(client.Namespace()).Create(ctx, rb, metav1.CreateOptions{})
}

func createKeycloakService(ctx context.Context, client *exutil.CLI) (*corev1.Service, error) {
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

	return client.KubeClient().CoreV1().Services(client.Namespace()).Create(ctx, service, metav1.CreateOptions{})
}

func createKeycloakCAConfigMap(ctx context.Context, client *exutil.CLI) (*corev1.ConfigMap, error) {
	defaultIngressCACM, err := client.KubeClient().CoreV1().ConfigMaps("openshift-config-managed").Get(ctx, "default-ingress-cert", metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting configmap openshift-config-managed/default-ingress-cert: %w", err)
	}

	data := defaultIngressCACM.Data["ca-bundle.crt"]

	keycloakCACM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-ca", keycloakResourceName),
		},
		Data: map[string]string{
			"ca.crt": data,
		},
	}

	return client.KubeClient().CoreV1().ConfigMaps("openshift-config").Create(ctx, keycloakCACM, metav1.CreateOptions{})
}

func createKeycloakDeployment(ctx context.Context, client *exutil.CLI) (*appsv1.Deployment, error) {
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

	return client.KubeClient().AppsV1().Deployments(client.Namespace()).Create(ctx, deployment, metav1.CreateOptions{})
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
			Command: []string{
				"/opt/keycloak/bin/kc.sh",
				"start-dev",
			},
			SecurityContext: &corev1.SecurityContext{
				Privileged: ptr.To(true),
			},
		},
	}
}

func createKeycloakRoute(ctx context.Context, service *corev1.Service, client *exutil.CLI) (*routev1.Route, error) {
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

	return client.RouteClient().RouteV1().Routes(client.Namespace()).Create(ctx, route, metav1.CreateOptions{})
}

func removeResources(ctx context.Context, client *exutil.CLI, resources ...kubeObject) error {
	errs := []error{}
	for _, resource := range resources {
		gvk := resource.GetObjectKind().GroupVersionKind()
		mapping, err := client.RESTMapper().RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			errs = append(errs, fmt.Errorf("getting GVR for GVK %v: %w", gvk, err))
			continue
		}

		err = client.DynamicClient().Resource(mapping.Resource).Namespace(resource.GetNamespace()).Delete(ctx, resource.GetName(), metav1.DeleteOptions{})
		if err != nil {
			errs = append(errs, fmt.Errorf("deleting resource %v/%s: %w", mapping.Resource, resource.GetName(), err))
			continue
		}
	}

	return errors.Join(errs...)
}

func configureOIDCAuthentication(ctx context.Context, client *exutil.CLI) (*configv1.Authentication, *configv1.Authentication, error) {
	authConfig, err := client.AdminConfigClient().ConfigV1().Authentications().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("getting authentications.config.openshift.io/cluster: %w", err)
	}

	original := authConfig.DeepCopy()
	modified := authConfig.DeepCopy()

	oidcProvider, err := generateOIDCProvider(ctx, client)
	if err != nil {
		return nil, nil, fmt.Errorf("generating OIDC provider: %w", err)
	}

	modified.Spec.Type = configv1.AuthenticationTypeOIDC
	modified.Spec.WebhookTokenAuthenticator = nil
	modified.Spec.OIDCProviders = append(modified.Spec.OIDCProviders, *oidcProvider)

	modified, err = client.AdminConfigClient().ConfigV1().Authentications().Update(ctx, modified, metav1.UpdateOptions{})
	if err != nil {
		return nil, nil, err
	}

	return original, modified, nil
}

func generateOIDCProvider(ctx context.Context, client *exutil.CLI) (*configv1.OIDCProvider, error) {
	idpName := "keycloak"
	caBundle := "keycloak-ca"
	audiences := []configv1.TokenAudience{
		"admin-cli",
	}
	usernameClaim := "email"
	groupsClaim := "groups"

	idpUrl, err := admittedURLForRoute(ctx, client, keycloakResourceName)
	if err != nil {
		return nil, fmt.Errorf("getting issuer URL: %w", err)
	}

	return &configv1.OIDCProvider{
		Name: idpName,
		Issuer: configv1.TokenIssuer{
			URL: idpUrl,
			CertificateAuthority: configv1.ConfigMapNameReference{
				Name: caBundle,
			},
			Audiences: audiences,
		},
		ClaimMappings: configv1.TokenClaimMappings{
			Username: configv1.UsernameClaimMapping{
				TokenClaimMapping: configv1.TokenClaimMapping{
					Claim: usernameClaim,
				},
			},
			Groups: configv1.PrefixedClaimMapping{
				TokenClaimMapping: configv1.TokenClaimMapping{
					Claim: groupsClaim,
				},
			},
		},
	}, nil
}

func admittedURLForRoute(ctx context.Context, client *exutil.CLI, routeName string) (string, error) {
	var admittedURL string

	// TODO: should probably create a new context that has a timeout to pass into this
	err := wait.PollUntilContextCancel(ctx, 1*time.Second, true, func(ctx context.Context) (bool, error) {
		route, err := client.RouteClient().RouteV1().Routes(client.Namespace()).Get(ctx, routeName, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}

		for _, ingress := range route.Status.Ingress {
			for _, condition := range ingress.Conditions {
				if condition.Type == routev1.RouteAdmitted && condition.Status == corev1.ConditionTrue {
					admittedURL = ingress.Host
					return true, nil
				}
			}
		}

		return false, fmt.Errorf("no admitted ingress for route %q", route.Name)
	})
	return admittedURL, err
}
