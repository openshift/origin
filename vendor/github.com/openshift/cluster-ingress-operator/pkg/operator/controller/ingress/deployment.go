package ingress

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"hash"
	"hash/fnv"
	"math"
	"net"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	operatorv1 "github.com/openshift/api/operator/v1"

	"github.com/openshift/cluster-ingress-operator/pkg/manifests"
	"github.com/openshift/cluster-ingress-operator/pkg/operator/controller"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/rand"

	configv1 "github.com/openshift/api/config/v1"
)

const (
	WildcardRouteAdmissionPolicy = "ROUTER_ALLOW_WILDCARD_ROUTES"

	RouterForwardedHeadersPolicy = "ROUTER_SET_FORWARDED_HEADERS"

	RouterUniqueHeaderName   = "ROUTER_UNIQUE_ID_HEADER_NAME"
	RouterUniqueHeaderFormat = "ROUTER_UNIQUE_ID_FORMAT"

	RouterHTTPHeaderNameCaseAdjustments = "ROUTER_H1_CASE_ADJUST"

	RouterLogLevelEnvName        = "ROUTER_LOG_LEVEL"
	RouterSyslogAddressEnvName   = "ROUTER_SYSLOG_ADDRESS"
	RouterSyslogFormatEnvName    = "ROUTER_SYSLOG_FORMAT"
	RouterSyslogFacilityEnvName  = "ROUTER_LOG_FACILITY"
	RouterSyslogMaxLengthEnvName = "ROUTER_LOG_MAX_LENGTH"

	RouterCaptureHTTPRequestHeaders  = "ROUTER_CAPTURE_HTTP_REQUEST_HEADERS"
	RouterCaptureHTTPResponseHeaders = "ROUTER_CAPTURE_HTTP_RESPONSE_HEADERS"
	RouterCaptureHTTPCookies         = "ROUTER_CAPTURE_HTTP_COOKIE"

	RouterHeaderBufferSize           = "ROUTER_BUF_SIZE"
	RouterHeaderBufferMaxRewriteSize = "ROUTER_MAX_REWRITE_SIZE"

	RouterLoadBalancingAlgorithmEnvName    = "ROUTER_LOAD_BALANCE_ALGORITHM"
	RouterTCPLoadBalancingAlgorithmEnvName = "ROUTER_TCP_BALANCE_SCHEME"

	RouterMaxConnectionsEnvName = "ROUTER_MAX_CONNECTIONS"

	RouterReloadIntervalEnvName = "RELOAD_INTERVAL"

	RouterDontLogNull      = "ROUTER_DONT_LOG_NULL"
	RouterHTTPIgnoreProbes = "ROUTER_HTTP_IGNORE_PROBES"

	RouterDisableHTTP2EnvName          = "ROUTER_DISABLE_HTTP2"
	RouterDefaultEnableHTTP2Annotation = "ingress.operator.openshift.io/default-enable-http2"

	RouterHardStopAfterEnvName    = "ROUTER_HARD_STOP_AFTER"
	RouterHardStopAfterAnnotation = "ingress.operator.openshift.io/hard-stop-after"

	LivenessGracePeriodSecondsAnnotation = "unsupported.do-not-use.openshift.io/override-liveness-grace-period-seconds"

	RouterHAProxyConfigManager = "ROUTER_HAPROXY_CONFIG_MANAGER"

	RouterHAProxyThreadsEnvName      = "ROUTER_THREADS"
	RouterHAProxyThreadsDefaultValue = 4

	WorkloadPartitioningManagement = "target.workload.openshift.io/management"

	RouterClientAuthPolicy = "ROUTER_MUTUAL_TLS_AUTH"
	RouterClientAuthCA     = "ROUTER_MUTUAL_TLS_AUTH_CA"
	RouterClientAuthCRL    = "ROUTER_MUTUAL_TLS_AUTH_CRL"
	RouterClientAuthFilter = "ROUTER_MUTUAL_TLS_AUTH_FILTER"

	RouterEnableCompression    = "ROUTER_ENABLE_COMPRESSION"
	RouterCompressionMIMETypes = "ROUTER_COMPRESSION_MIME"
	RouterBackendCheckInterval = "ROUTER_BACKEND_CHECK_INTERVAL"

	RouterServiceHTTPPort  = "ROUTER_SERVICE_HTTP_PORT"
	RouterServiceHTTPSPort = "ROUTER_SERVICE_HTTPS_PORT"
	StatsPort              = "STATS_PORT"

	HTTPPortName  = "http"
	HTTPSPortName = "https"
	StatsPortName = "metrics"

	haproxyMaxTimeoutMilliseconds = 2147483647 * time.Millisecond
)

// ensureRouterDeployment ensures the router deployment exists for a given
// ingresscontroller.
func (r *reconciler) ensureRouterDeployment(ci *operatorv1.IngressController, infraConfig *configv1.Infrastructure, ingressConfig *configv1.Ingress, apiConfig *configv1.APIServer, networkConfig *configv1.Network, haveClientCAConfigmap bool, clientCAConfigmap *corev1.ConfigMap, platformStatus *configv1.PlatformStatus) (bool, *appsv1.Deployment, error) {
	haveDepl, current, err := r.currentRouterDeployment(ci)
	if err != nil {
		return false, nil, err
	}
	proxyNeeded, err := IsProxyProtocolNeeded(ci, platformStatus)
	if err != nil {
		return false, nil, fmt.Errorf("failed to determine if proxy protocol is needed for ingresscontroller %s/%s: %v", ci.Namespace, ci.Name, err)
	}
	desired, err := desiredRouterDeployment(ci, r.config.IngressControllerImage, ingressConfig, infraConfig, apiConfig, networkConfig, proxyNeeded, haveClientCAConfigmap, clientCAConfigmap)
	if err != nil {
		return haveDepl, current, fmt.Errorf("failed to build router deployment: %v", err)
	}

	switch {
	case !haveDepl:
		if err := r.createRouterDeployment(desired); err != nil {
			return false, nil, err
		}
		return r.currentRouterDeployment(ci)
	case haveDepl:
		if updated, err := r.updateRouterDeployment(current, desired); err != nil {
			return true, current, err
		} else if updated {
			return r.currentRouterDeployment(ci)
		}
	}
	return true, current, nil
}

// ensureRouterDeleted ensures that any router resources associated with the
// ingresscontroller are deleted.
func (r *reconciler) ensureRouterDeleted(ci *operatorv1.IngressController) error {
	deployment := &appsv1.Deployment{}
	name := controller.RouterDeploymentName(ci)
	deployment.Name = name.Name
	deployment.Namespace = name.Namespace
	if err := r.client.Delete(context.TODO(), deployment); err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
	}
	log.Info("deleted deployment", "namespace", deployment.Namespace, "name", deployment.Name)
	r.recorder.Eventf(ci, "Normal", "DeletedDeployment", "Deleted deployment %s/%s", deployment.Namespace, deployment.Name)
	return nil
}

// HTTP2IsEnabledByAnnotation returns true if the map m has the key
// RouterDisableHTTP2Annotation present and true|false depending on
// the annotation's value that is parsed by strconv.ParseBool.
func HTTP2IsEnabledByAnnotation(m map[string]string) (bool, bool) {
	if val, ok := m[RouterDefaultEnableHTTP2Annotation]; ok {
		v, _ := strconv.ParseBool(val)
		return true, v
	}
	return false, false
}

// HTTP2IsEnabled returns true if the ingress controller enables
// http/2, or if the ingress config enables http/2. It will return
// false for the case where the ingress config has been enabled but
// the ingress controller explicitly overrides that by having the
// annotation present (even if its value is "false").
func HTTP2IsEnabled(ic *operatorv1.IngressController, ingressConfig *configv1.Ingress) bool {
	controllerHasHTTP2Annotation, controllerHasHTTP2Enabled := HTTP2IsEnabledByAnnotation(ic.Annotations)
	_, configHasHTTP2Enabled := HTTP2IsEnabledByAnnotation(ingressConfig.Annotations)

	if controllerHasHTTP2Annotation {
		return controllerHasHTTP2Enabled
	}

	return configHasHTTP2Enabled
}

// HardStopAfterIsEnabledByAnnotation returns true if the map m has
// the key RouterHardStopAfterEnvName and its value is a valid HAProxy
// time duration.
func HardStopAfterIsEnabledByAnnotation(m map[string]string) (bool, string) {
	if val, ok := m[RouterHardStopAfterAnnotation]; ok && len(val) > 0 {
		if clippedVal, err := clipHAProxyTimeoutValue(val); err != nil {
			log.Error(err, "invalid HAProxy time value", "annotation", RouterHardStopAfterAnnotation, "value", val)
			return false, ""
		} else {
			return true, clippedVal
		}
	}
	return false, ""
}

// HardStopAfterIsEnabled returns true if either the ingress
// controller or the ingress config has the "hard-stop-after"
// annotation. The presence of the annotation on the ingress
// controller, irrespective of its value, always overrides any setting
// on the ingress config.
func HardStopAfterIsEnabled(ic *operatorv1.IngressController, ingressConfig *configv1.Ingress) (bool, string) {
	if controllerAnnotation, controllerValue := HardStopAfterIsEnabledByAnnotation(ic.Annotations); controllerAnnotation {
		return controllerAnnotation, controllerValue
	}
	return HardStopAfterIsEnabledByAnnotation(ingressConfig.Annotations)
}

// determineDeploymentReplicas determines the number of replicas that should be
// set in the Deployment for an IngressController. If the user explicitly set a
// replica count in the IngressController resource, that value will be used.
// Otherwise, if unset, we follow the choice algorithm as described in the
// documentation for the IngressController replicas parameter.
func determineDeploymentReplicas(ic *operatorv1.IngressController, ingressConfig *configv1.Ingress, infraConfig *configv1.Infrastructure) int32 {
	if ic.Spec.Replicas != nil {
		return *ic.Spec.Replicas
	}

	return DetermineReplicas(ingressConfig, infraConfig)
}

// desiredRouterDeployment returns the desired router deployment.
func desiredRouterDeployment(ci *operatorv1.IngressController, ingressControllerImage string, ingressConfig *configv1.Ingress, infraConfig *configv1.Infrastructure, apiConfig *configv1.APIServer, networkConfig *configv1.Network, proxyNeeded bool, haveClientCAConfigmap bool, clientCAConfigmap *corev1.ConfigMap) (*appsv1.Deployment, error) {
	deployment := manifests.RouterDeployment()
	name := controller.RouterDeploymentName(ci)
	deployment.Name = name.Name
	deployment.Namespace = name.Namespace

	deployment.Labels = map[string]string{
		// associate the deployment with the ingresscontroller
		manifests.OwningIngressControllerLabel: ci.Name,
	}

	// Ensure the deployment adopts only its own pods.
	deployment.Spec.Selector = controller.IngressControllerDeploymentPodSelector(ci)
	deployment.Spec.Template.Labels = controller.IngressControllerDeploymentPodSelector(ci).MatchLabels

	// the router should have a very long grace period by default (1h)
	gracePeriod := int64(60 * 60)
	deployment.Spec.Template.Spec.TerminationGracePeriodSeconds = &gracePeriod

	// Services behind load balancers should roll out new instances only after we are certain
	// the new instance is part of rotation. This is set based on the highest value across all
	// platforms, excluding custom load balancers like an F5, but our recommendation for these
	// values for those should be indentical to the slowest cloud, AWS (which does not allow
	// health checks to be more frequent than 10 seconds).
	deployment.Spec.MinReadySeconds =
		(2 + /* max healthy checks required to be brought into rotation across all platforms */
			1) * /* we could miss one */
			10 /* the longest health check interval on any platform */

	volumes := deployment.Spec.Template.Spec.Volumes
	routerVolumeMounts := deployment.Spec.Template.Spec.Containers[0].VolumeMounts

	desiredReplicas := determineDeploymentReplicas(ci, ingressConfig, infraConfig)
	deployment.Spec.Replicas = &desiredReplicas

	configureAffinity := false
	switch ci.Status.EndpointPublishingStrategy.Type {
	case operatorv1.HostNetworkStrategyType:
		// Typically, an ingress controller will be scaled with replicas
		// set equal to the node pool size, in which case, using surge
		// for rolling updates would fail to create new replicas (in the
		// absence of node auto-scaling).  Thus, when using HostNetwork,
		// we set max unavailable to 25% and surge to 0.
		pointerTo := func(ios intstr.IntOrString) *intstr.IntOrString { return &ios }
		deployment.Spec.Strategy = appsv1.DeploymentStrategy{
			Type: appsv1.RollingUpdateDeploymentStrategyType,
			RollingUpdate: &appsv1.RollingUpdateDeployment{
				MaxUnavailable: pointerTo(intstr.FromString("25%")),
				MaxSurge:       pointerTo(intstr.FromInt(0)),
			},
		}

		// Pod replicas for ingress controllers that use the host
		// network cannot be colocated because replicas on the same node
		// would conflict with each other by trying to bind the same
		// ports.  The scheduler avoids scheduling multiple pods that
		// use host networking and specify the same port to the same
		// node.  Thus no affinity policy is required when using
		// HostNetwork.
	case operatorv1.PrivateStrategyType, operatorv1.LoadBalancerServiceStrategyType, operatorv1.NodePortServiceStrategyType:
		// To avoid downtime during a rolling update, we need two
		// things: a deployment strategy and an affinity policy.  First,
		// the deployment strategy: During a rolling update, we want the
		// deployment controller to scale up the new replica set first
		// and scale down the old replica set once the new replica is
		// ready.  Thus set max unavailable to 50% (if replicas < 4) or
		// 25% (if replicas >= 4) and surge to 25%.  Note that the
		// deployment controller rounds surge up and max unavailable
		// down.

		maxUnavailable := "50%"
		if desiredReplicas >= 4 {
			maxUnavailable = "25%"
		}
		pointerTo := func(ios intstr.IntOrString) *intstr.IntOrString { return &ios }
		deployment.Spec.Strategy = appsv1.DeploymentStrategy{
			Type: appsv1.RollingUpdateDeploymentStrategyType,
			RollingUpdate: &appsv1.RollingUpdateDeployment{
				MaxUnavailable: pointerTo(intstr.FromString(maxUnavailable)),
				MaxSurge:       pointerTo(intstr.FromString("25%")),
			},
		}

		// Next, the affinity policy: We want the deployment controller
		// to scale the new replica set up in such a way that each new
		// pod is colocated with a pod from the old replica set.  To
		// this end, we add a label with a hash of the deployment, using
		// which we can select replicas of the same generation (or
		// select replicas that are *not* of the same generation).
		// Then, we can configure affinity to colocate replicas of
		// different generations of the same ingress controller, and configure
		// anti-affinity to prevent colocation of replicas of the same
		// generation of the same ingress controller.
		//
		// Together, the deployment strategy and affinity policy ensure
		// that a node that had local endpoints at the start of a
		// rolling update continues to have local endpoints for the
		// duration of and at the completion of the update.
		configureAffinity = true
		deployment.Spec.Template.Spec.Affinity = &corev1.Affinity{
			PodAffinity: &corev1.PodAffinity{
				PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
					{
						Weight: int32(100),
						PodAffinityTerm: corev1.PodAffinityTerm{
							TopologyKey: "kubernetes.io/hostname",
							LabelSelector: &metav1.LabelSelector{
								MatchExpressions: []metav1.LabelSelectorRequirement{
									{
										Key:      controller.ControllerDeploymentLabel,
										Operator: metav1.LabelSelectorOpIn,
										Values:   []string{controller.IngressControllerDeploymentLabel(ci)},
									},
									{
										Key:      controller.ControllerDeploymentHashLabel,
										Operator: metav1.LabelSelectorOpNotIn,
										// Values is set at the end of this function.
									},
								},
							},
						},
					},
				},
			},
			// TODO: Once https://issues.redhat.com/browse/RFE-1759
			// is implemented, replace
			// "RequiredDuringSchedulingIgnoredDuringExecution" with
			// "PreferredDuringSchedulingIgnoredDuringExecution".
			PodAntiAffinity: &corev1.PodAntiAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
					{
						TopologyKey: "kubernetes.io/hostname",
						LabelSelector: &metav1.LabelSelector{
							MatchExpressions: []metav1.LabelSelectorRequirement{
								{
									Key:      controller.ControllerDeploymentLabel,
									Operator: metav1.LabelSelectorOpIn,
									Values:   []string{controller.IngressControllerDeploymentLabel(ci)},
								},
								{
									Key:      controller.ControllerDeploymentHashLabel,
									Operator: metav1.LabelSelectorOpIn,
									// Values is set at the end of this function.
								},
							},
						},
					},
				},
			},
		}
	}

	// Configure topology constraints to spread replicas across availability
	// zones.  We want to allow scheduling more replicas than there are AZs,
	// so we specify "ScheduleAnyway".  We want to allow scheduling a
	// newer-generation replica on the same node as an older-generation
	// replica where the deployment strategy allows and depends on doing so,
	// so we specify a label selector with the deployment's hash.
	deployment.Spec.Template.Spec.TopologySpreadConstraints = []corev1.TopologySpreadConstraint{{
		MaxSkew:           int32(1),
		TopologyKey:       corev1.LabelTopologyZone,
		WhenUnsatisfiable: corev1.ScheduleAnyway,
		LabelSelector: &metav1.LabelSelector{
			MatchExpressions: []metav1.LabelSelectorRequirement{
				{
					Key:      controller.ControllerDeploymentHashLabel,
					Operator: metav1.LabelSelectorOpIn,
					// Values is set at the end of this function.
				},
			},
		},
	}}

	statsSecretName := fmt.Sprintf("router-stats-%s", ci.Name)
	statsVolumeName := "stats-auth"
	statsVolumeMountPath := "/var/lib/haproxy/conf/metrics-auth"
	statsVolume := corev1.Volume{
		Name: statsVolumeName,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: statsSecretName,
			},
		},
	}
	statsVolumeMount := corev1.VolumeMount{
		Name:      statsVolumeName,
		MountPath: statsVolumeMountPath,
		ReadOnly:  true,
	}

	volumes = append(volumes, statsVolume)
	routerVolumeMounts = append(routerVolumeMounts, statsVolumeMount)
	env := []corev1.EnvVar{
		{Name: "ROUTER_SERVICE_NAME", Value: ci.Name},
		{Name: "STATS_USERNAME_FILE", Value: filepath.Join(statsVolumeMountPath, "statsUsername")},
		{Name: "STATS_PASSWORD_FILE", Value: filepath.Join(statsVolumeMountPath, "statsPassword")},
	}

	// Enable prometheus metrics
	certsSecretName := fmt.Sprintf("router-metrics-certs-%s", ci.Name)
	certsVolumeName := "metrics-certs"
	certsVolumeMountPath := "/etc/pki/tls/metrics-certs"

	certsVolume := corev1.Volume{
		Name: certsVolumeName,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: certsSecretName,
			},
		},
	}
	certsVolumeMount := corev1.VolumeMount{
		Name:      certsVolumeName,
		MountPath: certsVolumeMountPath,
		ReadOnly:  true,
	}

	volumes = append(volumes, certsVolume)
	routerVolumeMounts = append(routerVolumeMounts, certsVolumeMount)

	if len(ci.Spec.HttpErrorCodePages.Name) != 0 {
		configmapName := controller.HttpErrorCodePageConfigMapName(ci)
		httpErrorCodeConfigVolume := corev1.Volume{
			Name: "error-pages",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: configmapName.Name,
					},
				},
			},
		}
		volumes = append(volumes, httpErrorCodeConfigVolume)
		httpErrorCodeVolumeMount := corev1.VolumeMount{
			Name:      httpErrorCodeConfigVolume.Name,
			MountPath: "/var/lib/haproxy/conf/error_code_pages",
		}
		routerVolumeMounts = append(routerVolumeMounts, httpErrorCodeVolumeMount)
		if len(configmapName.Name) != 0 {
			env = append(env, corev1.EnvVar{
				Name:  "ROUTER_ERRORFILE_503",
				Value: "/var/lib/haproxy/conf/error_code_pages/error-page-503.http",
			})
			env = append(env, corev1.EnvVar{
				Name:  "ROUTER_ERRORFILE_404",
				Value: "/var/lib/haproxy/conf/error_code_pages/error-page-404.http",
			})
		}
	}

	env = append(env, corev1.EnvVar{Name: "ROUTER_METRICS_TYPE", Value: "haproxy"})
	env = append(env, corev1.EnvVar{Name: "ROUTER_METRICS_TLS_CERT_FILE", Value: filepath.Join(certsVolumeMountPath, "tls.crt")})
	env = append(env, corev1.EnvVar{Name: "ROUTER_METRICS_TLS_KEY_FILE", Value: filepath.Join(certsVolumeMountPath, "tls.key")})

	var unsupportedConfigOverrides struct {
		LoadBalancingAlgorithm string `json:"loadBalancingAlgorithm"`
		DynamicConfigManager   string `json:"dynamicConfigManager"`
	}
	if len(ci.Spec.UnsupportedConfigOverrides.Raw) > 0 {
		if err := json.Unmarshal(ci.Spec.UnsupportedConfigOverrides.Raw, &unsupportedConfigOverrides); err != nil {
			return nil, fmt.Errorf("ingresscontroller %q has invalid spec.unsupportedConfigOverrides: %w", ci.Name, err)
		}
	}

	// For non-TLS, edge-terminated, and reencrypt routes, use the
	// "random" balancing algorithm by default, but allow an unsupported
	// config override to override it.  For passthrough routes, use the
	// "source" balancing algorithm in order to provide some
	// session-affinity.
	// We've had issues with "random" in the past due to it incurring significant
	// memory overhead with large weights on the server lines in haproxy config;
	// however we mitigated that in openshift-router by effectively setting all
	// servers lines in "random" backends to weight 1 to avoid incurring extraneous
	// memory allocations.
	// Reference: https://issues.redhat.com/browse/NE-709
	loadBalancingAlgorithm := "random"
	switch unsupportedConfigOverrides.LoadBalancingAlgorithm {
	case "leastconn":
		loadBalancingAlgorithm = "leastconn"
	}
	env = append(env, corev1.EnvVar{
		Name:  RouterLoadBalancingAlgorithmEnvName,
		Value: loadBalancingAlgorithm,
	}, corev1.EnvVar{
		Name:  RouterTCPLoadBalancingAlgorithmEnvName,
		Value: "source",
	})

	switch v := ci.Spec.TuningOptions.MaxConnections; {
	case v == -1:
		env = append(env, corev1.EnvVar{
			Name:  RouterMaxConnectionsEnvName,
			Value: "auto",
		})
	case v > 0:
		env = append(env, corev1.EnvVar{
			Name:  RouterMaxConnectionsEnvName,
			Value: strconv.Itoa(int(v)),
		})
	}

	dynamicConfigOverride := unsupportedConfigOverrides.DynamicConfigManager
	if v, err := strconv.ParseBool(dynamicConfigOverride); err == nil && v {
		env = append(env, corev1.EnvVar{
			Name:  RouterHAProxyConfigManager,
			Value: "true",
		})
	}

	if len(ci.Status.Domain) > 0 {
		cName := "router-" + ci.Name + "." + ci.Status.Domain
		env = append(env,
			corev1.EnvVar{Name: "ROUTER_DOMAIN", Value: ci.Status.Domain},
			corev1.EnvVar{Name: "ROUTER_CANONICAL_HOSTNAME", Value: cName},
		)
	}

	if proxyNeeded {
		env = append(env, corev1.EnvVar{Name: "ROUTER_USE_PROXY_PROTOCOL", Value: "true"})
	}

	threads := RouterHAProxyThreadsDefaultValue
	if ci.Spec.TuningOptions.ThreadCount > 0 {
		threads = int(ci.Spec.TuningOptions.ThreadCount)
	}
	env = append(env, corev1.EnvVar{Name: RouterHAProxyThreadsEnvName, Value: strconv.Itoa(threads)})

	if ci.Spec.TuningOptions.ClientTimeout != nil && ci.Spec.TuningOptions.ClientTimeout.Duration > 0*time.Second {
		env = append(env, corev1.EnvVar{Name: "ROUTER_DEFAULT_CLIENT_TIMEOUT", Value: durationToHAProxyTimespec(ci.Spec.TuningOptions.ClientTimeout.Duration)})
	}
	if ci.Spec.TuningOptions.ClientFinTimeout != nil && ci.Spec.TuningOptions.ClientFinTimeout.Duration > 0*time.Second {
		env = append(env, corev1.EnvVar{Name: "ROUTER_CLIENT_FIN_TIMEOUT", Value: durationToHAProxyTimespec(ci.Spec.TuningOptions.ClientFinTimeout.Duration)})
	}
	if ci.Spec.TuningOptions.ServerTimeout != nil && ci.Spec.TuningOptions.ServerTimeout.Duration > 0*time.Second {
		env = append(env, corev1.EnvVar{Name: "ROUTER_DEFAULT_SERVER_TIMEOUT", Value: durationToHAProxyTimespec(ci.Spec.TuningOptions.ServerTimeout.Duration)})
	}
	if ci.Spec.TuningOptions.ServerFinTimeout != nil && ci.Spec.TuningOptions.ServerFinTimeout.Duration > 0*time.Second {
		env = append(env, corev1.EnvVar{Name: "ROUTER_DEFAULT_SERVER_FIN_TIMEOUT", Value: durationToHAProxyTimespec(ci.Spec.TuningOptions.ServerFinTimeout.Duration)})
	}
	if ci.Spec.TuningOptions.TunnelTimeout != nil && ci.Spec.TuningOptions.TunnelTimeout.Duration > 0*time.Second {
		env = append(env, corev1.EnvVar{Name: "ROUTER_DEFAULT_TUNNEL_TIMEOUT", Value: durationToHAProxyTimespec(ci.Spec.TuningOptions.TunnelTimeout.Duration)})
	}
	if ci.Spec.TuningOptions.TLSInspectDelay != nil && ci.Spec.TuningOptions.TLSInspectDelay.Duration > 0*time.Second {
		env = append(env, corev1.EnvVar{Name: "ROUTER_INSPECT_DELAY", Value: durationToHAProxyTimespec(ci.Spec.TuningOptions.TLSInspectDelay.Duration)})
	}
	if ci.Spec.TuningOptions.HealthCheckInterval != nil && ci.Spec.TuningOptions.HealthCheckInterval.Duration >= 1*time.Second {
		env = append(env, corev1.EnvVar{Name: RouterBackendCheckInterval, Value: durationToHAProxyTimespec(ci.Spec.TuningOptions.HealthCheckInterval.Duration)})
	}
	env = append(env, corev1.EnvVar{Name: RouterReloadIntervalEnvName, Value: durationToHAProxyTimespec(capReloadIntervalValue(ci.Spec.TuningOptions.ReloadInterval.Duration))})

	nodeSelector := map[string]string{
		"kubernetes.io/os": "linux",
	}

	switch ingressConfig.Status.DefaultPlacement {
	case configv1.DefaultPlacementControlPlane:
		nodeSelector["node-role.kubernetes.io/master"] = ""
	default:
		nodeSelector["node-role.kubernetes.io/worker"] = ""
	}

	if ci.Spec.NodePlacement != nil {
		if ci.Spec.NodePlacement.NodeSelector != nil {
			var err error
			nodeSelector, err = metav1.LabelSelectorAsMap(ci.Spec.NodePlacement.NodeSelector)
			if err != nil {
				return nil, fmt.Errorf("ingresscontroller %q has invalid spec.nodePlacement.nodeSelector: %v",
					ci.Name, err)
			}
		}
		if ci.Spec.NodePlacement.Tolerations != nil {
			deployment.Spec.Template.Spec.Tolerations = ci.Spec.NodePlacement.Tolerations
		}
	}
	deployment.Spec.Template.Spec.NodeSelector = nodeSelector

	if ci.Spec.NamespaceSelector != nil {
		namespaceSelector, err := metav1.LabelSelectorAsSelector(ci.Spec.NamespaceSelector)
		if err != nil {
			return nil, fmt.Errorf("ingresscontroller %q has invalid spec.namespaceSelector: %v",
				ci.Name, err)
		}

		env = append(env, corev1.EnvVar{
			Name:  "NAMESPACE_LABELS",
			Value: namespaceSelector.String(),
		})
	}

	if ci.Spec.RouteSelector != nil {
		routeSelector, err := metav1.LabelSelectorAsSelector(ci.Spec.RouteSelector)
		if err != nil {
			return nil, fmt.Errorf("ingresscontroller %q has invalid spec.routeSelector: %v", ci.Name, err)
		}
		env = append(env, corev1.EnvVar{Name: "ROUTE_LABELS", Value: routeSelector.String()})
	}

	deployment.Spec.Template.Spec.Containers[0].Image = ingressControllerImage
	deployment.Spec.Template.Spec.DNSPolicy = corev1.DNSClusterFirst

	var (
		statsPort int32 = routerDefaultHostNetworkStatsPort
		httpPort  int32 = routerDefaultHostNetworkHTTPPort
		httpsPort int32 = routerDefaultHostNetworkHTTPSPort
	)

	if ci.Status.EndpointPublishingStrategy.Type == operatorv1.HostNetworkStrategyType {
		// Expose ports 80, 443, and 1936 on the host to provide
		// endpoints for the user's HA solution.
		deployment.Spec.Template.Spec.HostNetwork = true

		// With container networking, probes default to using the pod IP
		// address.  With host networking, probes default to using the
		// node IP address.  Using localhost avoids potential routing
		// problems or firewall restrictions.
		deployment.Spec.Template.Spec.Containers[0].LivenessProbe.ProbeHandler.HTTPGet.Host = "localhost"
		deployment.Spec.Template.Spec.Containers[0].ReadinessProbe.ProbeHandler.HTTPGet.Host = "localhost"
		deployment.Spec.Template.Spec.Containers[0].StartupProbe.ProbeHandler.HTTPGet.Host = "localhost"
		deployment.Spec.Template.Spec.DNSPolicy = corev1.DNSClusterFirstWithHostNet

		if config := ci.Status.EndpointPublishingStrategy.HostNetwork; config != nil {
			if config.HTTPSPort == config.HTTPPort || config.HTTPPort == config.StatsPort || config.StatsPort == config.HTTPSPort {
				return nil, fmt.Errorf("the specified HTTPS, HTTP and Stats ports %d, %d, %d are not unique", config.HTTPSPort, config.HTTPPort, config.StatsPort)
			}

			// Set the ports to the values from the host network configuration
			httpPort = config.HTTPPort
			httpsPort = config.HTTPSPort
			statsPort = config.StatsPort
		}

		// Append the environment variables for the HTTP and HTTPS ports
		env = append(env,
			corev1.EnvVar{
				Name:  RouterServiceHTTPSPort,
				Value: strconv.Itoa(int(httpsPort)),
			},
			corev1.EnvVar{
				Name:  RouterServiceHTTPPort,
				Value: strconv.Itoa(int(httpPort)),
			},
		)
	}

	// Set the port for the probes from the host network configuration
	deployment.Spec.Template.Spec.Containers[0].LivenessProbe.ProbeHandler.HTTPGet.Port.IntVal = statsPort
	deployment.Spec.Template.Spec.Containers[0].ReadinessProbe.ProbeHandler.HTTPGet.Port.IntVal = statsPort
	deployment.Spec.Template.Spec.Containers[0].StartupProbe.ProbeHandler.HTTPGet.Port.IntVal = statsPort

	// append the value for the metrics port to the list of environment variables
	env = append(env, corev1.EnvVar{
		Name:  StatsPort,
		Value: strconv.Itoa(int(statsPort)),
	})

	// Fill in the default certificate secret name.
	secretName := controller.RouterEffectiveDefaultCertificateSecretName(ci, deployment.Namespace)
	deployment.Spec.Template.Spec.Volumes[0].Secret.SecretName = secretName.Name

	if accessLogging := accessLoggingForIngressController(ci); accessLogging != nil {
		switch {
		case accessLogging.Destination.Type == operatorv1.ContainerLoggingDestinationType:
			rsyslogConfigVolume := corev1.Volume{
				Name: "rsyslog-config",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: controller.RsyslogConfigMapName(ci).Name,
						},
					},
				},
			}
			rsyslogConfigVolumeMount := corev1.VolumeMount{
				Name:      rsyslogConfigVolume.Name,
				MountPath: "/etc/rsyslog",
			}

			// Ideally we would use a Unix domain socket in the abstract
			// namespace, but rsyslog does not support that, so we need a
			// filesystem that is common to the router and syslog
			// containers.
			rsyslogSocketVolume := corev1.Volume{
				Name: "rsyslog-socket",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			}
			rsyslogSocketVolumeMount := corev1.VolumeMount{
				Name:      rsyslogSocketVolume.Name,
				MountPath: "/var/lib/rsyslog",
			}

			configPath := filepath.Join(rsyslogConfigVolumeMount.MountPath, "rsyslog.conf")
			socketPath := filepath.Join(rsyslogSocketVolumeMount.MountPath, "rsyslog.sock")

			syslogContainer := corev1.Container{
				Name: operatorv1.ContainerLoggingSidecarContainerName,
				// The ingresscontroller image has rsyslog built in.
				Image: ingressControllerImage,
				Command: []string{
					"/sbin/rsyslogd", "-n",
					// TODO: Once we have rsyslog 8.32 or later,
					// we can switch to -i NONE.
					"-i", "/tmp/rsyslog.pid",
					"-f", configPath,
				},
				ImagePullPolicy: corev1.PullIfNotPresent,
				VolumeMounts: []corev1.VolumeMount{
					rsyslogConfigVolumeMount,
					rsyslogSocketVolumeMount,
				},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("256Mi"),
					},
				},
			}

			env = append(env,
				corev1.EnvVar{Name: RouterSyslogAddressEnvName, Value: socketPath},
				corev1.EnvVar{Name: RouterLogLevelEnvName, Value: "info"},
			)
			volumes = append(volumes, rsyslogConfigVolume, rsyslogSocketVolume)
			routerVolumeMounts = append(routerVolumeMounts, rsyslogSocketVolumeMount)
			deployment.Spec.Template.Spec.Containers = append(deployment.Spec.Template.Spec.Containers, syslogContainer)
		case accessLogging.Destination.Type == operatorv1.SyslogLoggingDestinationType:
			if len(accessLogging.Destination.Syslog.Facility) > 0 {
				env = append(env, corev1.EnvVar{Name: RouterSyslogFacilityEnvName, Value: accessLogging.Destination.Syslog.Facility})
			}
			if accessLogging.Destination.Syslog.MaxLength > 0 {
				env = append(env, corev1.EnvVar{
					Name:  RouterSyslogMaxLengthEnvName,
					Value: fmt.Sprintf("%d", accessLogging.Destination.Syslog.MaxLength),
				})
			}
			address := accessLogging.Destination.Syslog.Address
			port := accessLogging.Destination.Syslog.Port
			endpoint := net.JoinHostPort(address, fmt.Sprintf("%d", port))
			env = append(env,
				corev1.EnvVar{Name: RouterLogLevelEnvName, Value: "info"},
				corev1.EnvVar{Name: RouterSyslogAddressEnvName, Value: endpoint},
			)
		}

		if len(accessLogging.HttpLogFormat) > 0 {
			env = append(env, corev1.EnvVar{Name: RouterSyslogFormatEnvName, Value: fmt.Sprintf("%q", accessLogging.HttpLogFormat)})
		}
		if val := serializeCaptureHeaders(accessLogging.HTTPCaptureHeaders.Request); len(val) != 0 {
			env = append(env, corev1.EnvVar{
				Name:  RouterCaptureHTTPRequestHeaders,
				Value: val,
			})
		}
		if val := serializeCaptureHeaders(accessLogging.HTTPCaptureHeaders.Response); len(val) != 0 {
			env = append(env, corev1.EnvVar{
				Name:  RouterCaptureHTTPResponseHeaders,
				Value: val,
			})
		}
		if len(accessLogging.HTTPCaptureCookies) > 0 {
			var (
				cookieName string
				maxLength  = accessLogging.HTTPCaptureCookies[0].MaxLength
			)
			switch accessLogging.HTTPCaptureCookies[0].MatchType {
			case operatorv1.CookieMatchTypeExact:
				cookieName = accessLogging.HTTPCaptureCookies[0].Name + "="
			case operatorv1.CookieMatchTypePrefix:
				cookieName = accessLogging.HTTPCaptureCookies[0].NamePrefix
			}
			if maxLength == 0 {
				maxLength = 256
			}
			env = append(env, corev1.EnvVar{
				Name:  RouterCaptureHTTPCookies,
				Value: fmt.Sprintf("%s:%d", cookieName, maxLength),
			})
		}

		if accessLogging.LogEmptyRequests == operatorv1.LoggingPolicyIgnore {
			env = append(env, corev1.EnvVar{Name: RouterDontLogNull, Value: "true"})
		}
	}

	tlsProfileSpec := tlsProfileSpecForIngressController(ci, apiConfig)

	var tls13Ciphers, otherCiphers []string
	for _, cipher := range tlsProfileSpec.Ciphers {
		if tlsVersion13Ciphers.Has(cipher) {
			tls13Ciphers = append(tls13Ciphers, cipher)
		} else {
			otherCiphers = append(otherCiphers, cipher)
		}
	}
	env = append(env, corev1.EnvVar{
		Name:  "ROUTER_CIPHERS",
		Value: strings.Join(otherCiphers, ":"),
	})
	if len(tls13Ciphers) != 0 {
		env = append(env, corev1.EnvVar{
			Name:  "ROUTER_CIPHERSUITES",
			Value: strings.Join(tls13Ciphers, ":"),
		})
	}

	var minTLSVersion string
	switch tlsProfileSpec.MinTLSVersion {
	// TLS 1.0 is not supported, convert to TLS 1.1.
	case configv1.VersionTLS10:
		minTLSVersion = "TLSv1.1"
	case configv1.VersionTLS11:
		minTLSVersion = "TLSv1.1"
	case configv1.VersionTLS12:
		minTLSVersion = "TLSv1.2"
	case configv1.VersionTLS13:
		minTLSVersion = "TLSv1.3"
	default:
		minTLSVersion = "TLSv1.2"
	}
	env = append(env, corev1.EnvVar{Name: "SSL_MIN_VERSION", Value: minTLSVersion})

	usingIPv4 := false
	usingIPv6 := false
	for _, clusterNetworkEntry := range networkConfig.Status.ClusterNetwork {
		addr, _, err := net.ParseCIDR(clusterNetworkEntry.CIDR)
		if err != nil {
			continue
		}
		if addr.To4() != nil {
			usingIPv4 = true
		} else {
			usingIPv6 = true
		}
	}
	if usingIPv6 {
		mode := "v4v6"
		if !usingIPv4 {
			mode = "v6"
		}
		env = append(env, corev1.EnvVar{Name: "ROUTER_IP_V4_V6_MODE", Value: mode})
	}

	routeAdmission := operatorv1.RouteAdmissionPolicy{
		NamespaceOwnership: operatorv1.StrictNamespaceOwnershipCheck,
		WildcardPolicy:     operatorv1.WildcardPolicyDisallowed,
	}
	if admission := ci.Spec.RouteAdmission; admission != nil {
		if len(admission.NamespaceOwnership) > 0 {
			routeAdmission.NamespaceOwnership = admission.NamespaceOwnership
		}
		if len(admission.WildcardPolicy) > 0 {
			routeAdmission.WildcardPolicy = admission.WildcardPolicy
		}
	}
	switch routeAdmission.NamespaceOwnership {
	case operatorv1.StrictNamespaceOwnershipCheck:
		env = append(env, corev1.EnvVar{Name: "ROUTER_DISABLE_NAMESPACE_OWNERSHIP_CHECK", Value: "false"})
	case operatorv1.InterNamespaceAllowedOwnershipCheck:
		env = append(env, corev1.EnvVar{Name: "ROUTER_DISABLE_NAMESPACE_OWNERSHIP_CHECK", Value: "true"})
	}
	switch routeAdmission.WildcardPolicy {
	case operatorv1.WildcardPolicyAllowed:
		env = append(env, corev1.EnvVar{Name: WildcardRouteAdmissionPolicy, Value: "true"})
	default:
		env = append(env, corev1.EnvVar{Name: WildcardRouteAdmissionPolicy, Value: "false"})
	}

	forwardedHeaderPolicy := operatorv1.AppendHTTPHeaderPolicy
	if ci.Spec.HTTPHeaders != nil && len(ci.Spec.HTTPHeaders.ForwardedHeaderPolicy) != 0 {
		forwardedHeaderPolicy = ci.Spec.HTTPHeaders.ForwardedHeaderPolicy
	}
	routerForwardedHeadersPolicyValue := "append"
	switch forwardedHeaderPolicy {
	case operatorv1.AppendHTTPHeaderPolicy:
		// Nothing to do.
	case operatorv1.ReplaceHTTPHeaderPolicy:
		routerForwardedHeadersPolicyValue = "replace"
	case operatorv1.IfNoneHTTPHeaderPolicy:
		routerForwardedHeadersPolicyValue = "if-none"
	case operatorv1.NeverHTTPHeaderPolicy:
		routerForwardedHeadersPolicyValue = "never"
	}
	env = append(env, corev1.EnvVar{Name: RouterForwardedHeadersPolicy, Value: routerForwardedHeadersPolicyValue})

	if ci.Spec.HTTPHeaders != nil && len(ci.Spec.HTTPHeaders.UniqueId.Name) > 0 {
		headerName := ci.Spec.HTTPHeaders.UniqueId.Name
		headerFormat := ci.Spec.HTTPHeaders.UniqueId.Format
		if len(headerFormat) == 0 {
			headerFormat = "%{+X}o %ci:%cp_%fi:%fp_%Ts_%rt:%pid"
		}
		env = append(env,
			corev1.EnvVar{Name: RouterUniqueHeaderName, Value: headerName},
			corev1.EnvVar{Name: RouterUniqueHeaderFormat, Value: fmt.Sprintf("%q", headerFormat)},
		)
	}

	if ci.Spec.HTTPHeaders != nil && len(ci.Spec.HTTPHeaders.HeaderNameCaseAdjustments) > 0 {
		var adjustments []string
		for _, v := range ci.Spec.HTTPHeaders.HeaderNameCaseAdjustments {
			adjustments = append(adjustments, string(v))
		}
		v := strings.Join(adjustments, ",")
		env = append(env, corev1.EnvVar{Name: RouterHTTPHeaderNameCaseAdjustments, Value: v})
	}

	if ci.Spec.HTTPEmptyRequestsPolicy == operatorv1.HTTPEmptyRequestsPolicyIgnore {
		env = append(env, corev1.EnvVar{Name: RouterHTTPIgnoreProbes, Value: "true"})
	}

	if HTTP2IsEnabled(ci, ingressConfig) {
		env = append(env, corev1.EnvVar{Name: RouterDisableHTTP2EnvName, Value: "false"})
	} else {
		env = append(env, corev1.EnvVar{Name: RouterDisableHTTP2EnvName, Value: "true"})
	}

	if enabled, value := HardStopAfterIsEnabled(ci, ingressConfig); enabled {
		env = append(env, corev1.EnvVar{Name: RouterHardStopAfterEnvName, Value: value})
	}

	// Apply HTTP Header Buffer size values to env
	// when they are specified.
	if ci.Spec.TuningOptions.HeaderBufferBytes != 0 {
		env = append(env, corev1.EnvVar{Name: RouterHeaderBufferSize, Value: strconv.Itoa(
			int(ci.Spec.TuningOptions.HeaderBufferBytes))})
	}

	if ci.Spec.TuningOptions.HeaderBufferMaxRewriteBytes != 0 {
		env = append(env, corev1.EnvVar{Name: RouterHeaderBufferMaxRewriteSize, Value: strconv.Itoa(
			int(ci.Spec.TuningOptions.HeaderBufferMaxRewriteBytes))})
	}

	if len(ci.Spec.ClientTLS.ClientCertificatePolicy) != 0 {
		var clientAuthPolicy string
		switch ci.Spec.ClientTLS.ClientCertificatePolicy {
		case operatorv1.ClientCertificatePolicyRequired:
			clientAuthPolicy = "required"
		case operatorv1.ClientCertificatePolicyOptional:
			clientAuthPolicy = "optional"
		}
		env = append(env,
			corev1.EnvVar{Name: RouterClientAuthPolicy, Value: clientAuthPolicy},
		)

		if len(ci.Spec.ClientTLS.ClientCA.Name) != 0 {
			clientCAConfigmapName := controller.ClientCAConfigMapName(ci)
			clientCAVolumeName := "client-ca"
			clientCAVolumeMountPath := "/etc/pki/tls/client-ca"
			clientCABundleFilename := "ca-bundle.pem"
			clientCAVolume := corev1.Volume{
				Name: clientCAVolumeName,
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: clientCAConfigmapName.Name,
						},
						Items: []corev1.KeyToPath{
							{
								Key:  clientCABundleFilename,
								Path: clientCABundleFilename,
							},
						},
					},
				},
			}
			clientCAVolumeMount := corev1.VolumeMount{
				Name:      clientCAVolumeName,
				MountPath: clientCAVolumeMountPath,
				ReadOnly:  true,
			}
			volumes = append(volumes, clientCAVolume)
			routerVolumeMounts = append(routerVolumeMounts, clientCAVolumeMount)

			clientAuthCAPath := filepath.Join(clientCAVolumeMount.MountPath, clientCABundleFilename)
			env = append(env, corev1.EnvVar{Name: RouterClientAuthCA, Value: clientAuthCAPath})

			if haveClientCAConfigmap {
				// If any certificates in the client CA bundle
				// specify any CRL distribution points, then we
				// need to configure a configmap volume.  The
				// crl controller is responsible for managing
				// the configmap.
				var clientCAData []byte
				if v, ok := clientCAConfigmap.Data[clientCABundleFilename]; !ok {
					return nil, fmt.Errorf("client CA configmap %s/%s is missing %q", clientCAConfigmap.Namespace, clientCAConfigmap.Name, clientCABundleFilename)
				} else {
					clientCAData = []byte(v)
				}
				var someClientCAHasCRL bool
				for len(clientCAData) > 0 {
					block, data := pem.Decode(clientCAData)
					if block == nil {
						break
					}
					clientCAData = data
					cert, err := x509.ParseCertificate(block.Bytes)
					if err != nil {
						return nil, fmt.Errorf("client CA configmap %s/%s has an invalid certificate: %w", clientCAConfigmap.Namespace, clientCAConfigmap.Name, err)
					}
					if len(cert.CRLDistributionPoints) != 0 {
						someClientCAHasCRL = true
						break
					}
				}
				if someClientCAHasCRL {
					clientCACRLSecretName := controller.CRLConfigMapName(ci)
					clientCACRLVolumeName := "client-ca-crl"
					clientCACRLVolumeMountPath := "/etc/pki/tls/client-ca-crl"
					clientCACRLFilename := "crl.pem"
					clientCACRLVolume := corev1.Volume{
						Name: clientCACRLVolumeName,
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: clientCACRLSecretName.Name,
								},
								Items: []corev1.KeyToPath{
									{
										Key:  clientCACRLFilename,
										Path: clientCACRLFilename,
									},
								},
							},
						},
					}
					clientCACRLVolumeMount := corev1.VolumeMount{
						Name:      clientCACRLVolumeName,
						MountPath: clientCACRLVolumeMountPath,
						ReadOnly:  true,
					}
					volumes = append(volumes, clientCACRLVolume)
					routerVolumeMounts = append(routerVolumeMounts, clientCACRLVolumeMount)

					clientAuthCRLPath := filepath.Join(clientCACRLVolumeMount.MountPath, clientCACRLFilename)
					env = append(env, corev1.EnvVar{Name: RouterClientAuthCRL, Value: clientAuthCRLPath})
				}
			}

			if len(ci.Spec.ClientTLS.AllowedSubjectPatterns) != 0 {
				pattern := "(?:" + strings.Join(ci.Spec.ClientTLS.AllowedSubjectPatterns, "|") + ")"
				env = append(env, corev1.EnvVar{Name: RouterClientAuthFilter, Value: pattern})
			}
		}

	}

	deployment.Spec.Template.Spec.Volumes = volumes
	deployment.Spec.Template.Spec.Containers[0].VolumeMounts = routerVolumeMounts

	// If MIMETypes were supplied, expose the RouterEnableCompression and RouterCompressionMIMETypes
	// environment variables.
	if len(ci.Spec.HTTPCompression.MimeTypes) != 0 {
		env = append(env, corev1.EnvVar{Name: RouterEnableCompression, Value: "true"})
		mimes := GetMIMETypes(ci.Spec.HTTPCompression.MimeTypes)
		env = append(env, corev1.EnvVar{Name: RouterCompressionMIMETypes, Value: strings.Join(mimes, " ")})
	}

	// Add the environment variables to the container
	deployment.Spec.Template.Spec.Containers[0].Env = append(deployment.Spec.Template.Spec.Containers[0].Env, env...)

	// Add the ports to the container
	deployment.Spec.Template.Spec.Containers[0].Ports = append(
		deployment.Spec.Template.Spec.Containers[0].Ports,
		corev1.ContainerPort{
			Name:          HTTPPortName,
			ContainerPort: httpPort,
			Protocol:      corev1.ProtocolTCP,
		},
		corev1.ContainerPort{
			Name:          HTTPSPortName,
			ContainerPort: httpsPort,
			Protocol:      corev1.ProtocolTCP,
		},
		corev1.ContainerPort{
			Name:          StatsPortName,
			ContainerPort: statsPort,
			Protocol:      corev1.ProtocolTCP,
		},
	)

	// Compute the hash for topology spread constraints and possibly
	// affinity policy now, after all the other fields have been computed,
	// and inject it into the appropriate fields.
	hash := deploymentTemplateHash(deployment)
	deployment.Spec.Template.Labels[controller.ControllerDeploymentHashLabel] = hash
	values := []string{hash}
	deployment.Spec.Template.Spec.TopologySpreadConstraints[0].LabelSelector.MatchExpressions[0].Values = values
	if configureAffinity {
		deployment.Spec.Template.Spec.Affinity.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution[0].PodAffinityTerm.LabelSelector.MatchExpressions[1].Values = values
		deployment.Spec.Template.Spec.Affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution[0].LabelSelector.MatchExpressions[1].Values = values
	}

	return deployment, nil
}

// accessLoggingForIngressController returns an AccessLogging value for the
// given ingresscontroller, or nil if the ingresscontroller does not specify
// a valid access logging configuration.
func accessLoggingForIngressController(ic *operatorv1.IngressController) *operatorv1.AccessLogging {
	if ic.Spec.Logging == nil || ic.Spec.Logging.Access == nil {
		return nil
	}

	switch ic.Spec.Logging.Access.Destination.Type {
	case operatorv1.ContainerLoggingDestinationType:
		return &operatorv1.AccessLogging{
			Destination: operatorv1.LoggingDestination{
				Type:      operatorv1.ContainerLoggingDestinationType,
				Container: &operatorv1.ContainerLoggingDestinationParameters{},
			},
			HttpLogFormat:      ic.Spec.Logging.Access.HttpLogFormat,
			HTTPCaptureHeaders: ic.Spec.Logging.Access.HTTPCaptureHeaders,
			HTTPCaptureCookies: ic.Spec.Logging.Access.HTTPCaptureCookies,
			LogEmptyRequests:   ic.Spec.Logging.Access.LogEmptyRequests,
		}
	case operatorv1.SyslogLoggingDestinationType:
		if ic.Spec.Logging.Access.Destination.Syslog != nil {
			return &operatorv1.AccessLogging{
				Destination: operatorv1.LoggingDestination{
					Type: operatorv1.SyslogLoggingDestinationType,
					Syslog: &operatorv1.SyslogLoggingDestinationParameters{
						Address:   ic.Spec.Logging.Access.Destination.Syslog.Address,
						Port:      ic.Spec.Logging.Access.Destination.Syslog.Port,
						Facility:  ic.Spec.Logging.Access.Destination.Syslog.Facility,
						MaxLength: ic.Spec.Logging.Access.Destination.Syslog.MaxLength,
					},
				},
				HttpLogFormat:      ic.Spec.Logging.Access.HttpLogFormat,
				HTTPCaptureHeaders: ic.Spec.Logging.Access.HTTPCaptureHeaders,
				HTTPCaptureCookies: ic.Spec.Logging.Access.HTTPCaptureCookies,
				LogEmptyRequests:   ic.Spec.Logging.Access.LogEmptyRequests,
			}
		}
	}
	return nil
}

// serializeCaptureHeaders serializes a slice of
// IngressControllerCaptureHTTPHeader values into a value suitable for
// ROUTER_CAPTURE_HTTP_RESPONSE_HEADERS or ROUTER_CAPTURE_HTTP_REQUEST_HEADERS.
func serializeCaptureHeaders(captureHeaders []operatorv1.IngressControllerCaptureHTTPHeader) string {
	var headerSpecs []string
	for _, header := range captureHeaders {
		headerSpecs = append(headerSpecs, fmt.Sprintf("%s:%d", header.Name, header.MaxLength))
	}
	return strings.Join(headerSpecs, ",")
}

// inferTLSProfileSpecFromDeployment examines the given deployment's pod
// template spec and reconstructs a TLS profile spec based on that pod spec.
func inferTLSProfileSpecFromDeployment(deployment *appsv1.Deployment) *configv1.TLSProfileSpec {
	var env []corev1.EnvVar
	foundContainer := false
	for _, container := range deployment.Spec.Template.Spec.Containers {
		if container.Name == "router" {
			env = container.Env
			foundContainer = true
			break
		}
	}

	if !foundContainer {
		return &configv1.TLSProfileSpec{}
	}

	var (
		ciphersString       string
		cipherSuitesString  string
		minTLSVersionString string
	)
	for _, v := range env {
		switch v.Name {
		case "ROUTER_CIPHERS":
			ciphersString = v.Value
		case "ROUTER_CIPHERSUITES":
			cipherSuitesString = v.Value
		case "SSL_MIN_VERSION":
			minTLSVersionString = v.Value
		}
	}

	var ciphers []string
	if len(ciphersString) > 0 {
		ciphers = strings.Split(ciphersString, ":")
	}
	if len(cipherSuitesString) > 0 {
		ciphers = append(ciphers, strings.Split(cipherSuitesString, ":")...)
	}

	var minTLSVersion configv1.TLSProtocolVersion
	switch minTLSVersionString {
	case "TLSv1.1":
		minTLSVersion = configv1.VersionTLS11
	case "TLSv1.2":
		minTLSVersion = configv1.VersionTLS12
	case "TLSv1.3":
		minTLSVersion = configv1.VersionTLS13
	default:
		minTLSVersion = configv1.VersionTLS12
	}

	profile := &configv1.TLSProfileSpec{
		Ciphers:       ciphers,
		MinTLSVersion: minTLSVersion,
	}

	return profile
}

// deploymentHash returns a stringified hash value for the router deployment
// fields that, if changed, should trigger an update.
func deploymentHash(deployment *appsv1.Deployment) string {
	hasher := fnv.New32a()
	deepHashObject(hasher, hashableDeployment(deployment, false))
	return rand.SafeEncodeString(fmt.Sprint(hasher.Sum32()))
}

// deploymentTemplateHash returns a stringified hash value for the router
// deployment fields that should be used to distinguish the given deployment
// from the deployment for another ingresscontroller or another generation of
// the same ingresscontroller (which will trigger a rolling update of the
// deployment).
func deploymentTemplateHash(deployment *appsv1.Deployment) string {
	hasher := fnv.New32a()
	deepHashObject(hasher, hashableDeployment(deployment, true))
	return rand.SafeEncodeString(fmt.Sprint(hasher.Sum32()))
}

// hashableDeployment returns a copy of the given deployment with exactly the
// fields from deployment that should be used for computing its hash copied
// over.  In particular, these are the fields that desiredRouterDeployment sets.
// Fields with slice values will be sorted.  Fields that should be ignored, or
// that have explicit values that are equal to their respective default values,
// will be zeroed.  If onlyTemplate is true, fields that should not trigger a
// rolling update are zeroed as well.
func hashableDeployment(deployment *appsv1.Deployment, onlyTemplate bool) *appsv1.Deployment {
	var hashableDeployment appsv1.Deployment

	// Copy metadata fields that distinguish the deployment for one
	// ingresscontroller from the deployment for another.
	hashableDeployment.Name = deployment.Name
	hashableDeployment.Namespace = deployment.Namespace

	// Copy pod template spec fields to which any changes should
	// trigger a rolling update of the deployment.
	affinity := deployment.Spec.Template.Spec.Affinity.DeepCopy()
	if affinity != nil {
		if affinity.PodAffinity != nil {
			terms := affinity.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution
			for _, term := range terms {
				labelSelector := term.PodAffinityTerm.LabelSelector
				zeroOutDeploymentHash(labelSelector)
				exprs := labelSelector.MatchExpressions
				sort.Slice(exprs, func(i, j int) bool {
					return cmpMatchExpressions(exprs[i], exprs[j])
				})
			}
		}
		if affinity.PodAntiAffinity != nil {
			terms := affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution
			for _, term := range terms {
				zeroOutDeploymentHash(term.LabelSelector)
				exprs := term.LabelSelector.MatchExpressions
				sort.Slice(exprs, func(i, j int) bool {
					return cmpMatchExpressions(exprs[i], exprs[j])
				})
			}
		}
	}
	hashableDeployment.Spec.Template.Spec.Affinity = affinity
	tolerations := make([]corev1.Toleration, len(deployment.Spec.Template.Spec.Tolerations))
	for i, toleration := range deployment.Spec.Template.Spec.Tolerations {
		tolerations[i] = *toleration.DeepCopy()
		if toleration.Effect == corev1.TaintEffectNoExecute {
			// TolerationSeconds is ignored unless Effect is
			// NoExecute.
			tolerations[i].TolerationSeconds = nil
		}
	}
	sort.Slice(tolerations, func(i, j int) bool {
		return tolerations[i].Key < tolerations[j].Key || tolerations[i].Operator < tolerations[j].Operator || tolerations[i].Value < tolerations[j].Value || tolerations[i].Effect < tolerations[j].Effect
	})
	hashableDeployment.Spec.Template.Spec.Tolerations = tolerations
	topologySpreadConstraints := make([]corev1.TopologySpreadConstraint, len(deployment.Spec.Template.Spec.TopologySpreadConstraints))
	for i, constraint := range deployment.Spec.Template.Spec.TopologySpreadConstraints {
		topologySpreadConstraints[i] = *constraint.DeepCopy()
		labelSelector := topologySpreadConstraints[i].LabelSelector
		zeroOutDeploymentHash(labelSelector)
		exprs := labelSelector.MatchExpressions
		sort.Slice(exprs, func(i, j int) bool {
			return cmpMatchExpressions(exprs[i], exprs[j])
		})
	}
	hashableDeployment.Spec.Template.Spec.TopologySpreadConstraints = topologySpreadConstraints
	hashableDeployment.Spec.Template.Spec.NodeSelector = deployment.Spec.Template.Spec.NodeSelector
	containers := make([]corev1.Container, len(deployment.Spec.Template.Spec.Containers))
	for i, container := range deployment.Spec.Template.Spec.Containers {
		env := container.Env
		sort.Slice(env, func(i, j int) bool {
			return env[i].Name < env[j].Name
		})
		containers[i] = corev1.Container{
			Command:         container.Command,
			Env:             env,
			Image:           container.Image,
			ImagePullPolicy: container.ImagePullPolicy,
			Name:            container.Name,
			LivenessProbe:   hashableProbe(container.LivenessProbe),
			ReadinessProbe:  hashableProbe(container.ReadinessProbe),
			StartupProbe:    hashableProbe(container.StartupProbe),
			SecurityContext: container.SecurityContext,
			Ports:           container.Ports,
		}
	}
	sort.Slice(containers, func(i, j int) bool {
		return containers[i].Name < containers[j].Name
	})
	hashableDeployment.Spec.Template.Spec.Containers = containers
	hashableDeployment.Spec.Template.Spec.DNSPolicy = deployment.Spec.Template.Spec.DNSPolicy
	hashableDeployment.Spec.Template.Spec.HostNetwork = deployment.Spec.Template.Spec.HostNetwork
	volumes := make([]corev1.Volume, len(deployment.Spec.Template.Spec.Volumes))
	for i, vol := range deployment.Spec.Template.Spec.Volumes {
		volumes[i] = *vol.DeepCopy()
		// 420 is the default value for DefaultMode for ConfigMap and
		// Secret volumes.
		if vol.ConfigMap != nil && vol.ConfigMap.DefaultMode != nil && *vol.ConfigMap.DefaultMode == int32(420) {
			volumes[i].ConfigMap.DefaultMode = nil
		}
		if vol.Secret != nil && vol.Secret.DefaultMode != nil && *vol.Secret.DefaultMode == int32(420) {
			volumes[i].Secret.DefaultMode = nil
		}
	}
	sort.Slice(volumes, func(i, j int) bool {
		return volumes[i].Name < volumes[j].Name
	})
	hashableDeployment.Spec.Template.Spec.Volumes = volumes
	hashableDeployment.Spec.Template.Annotations = make(map[string]string)
	annotations := []string{LivenessGracePeriodSecondsAnnotation, WorkloadPartitioningManagement}
	for _, key := range annotations {
		if val, ok := deployment.Spec.Template.Annotations[key]; ok && len(val) > 0 {
			hashableDeployment.Spec.Template.Annotations[key] = val
		}
	}

	if onlyTemplate {
		return &hashableDeployment
	}

	// Copy metadata and spec fields to which any changes should trigger an
	// update of the deployment but should not trigger a rolling update.
	hashableDeployment.Labels = deployment.Labels
	hashableDeployment.Spec.MinReadySeconds = deployment.Spec.MinReadySeconds
	hashableDeployment.Spec.Strategy = deployment.Spec.Strategy
	var replicas *int32
	if deployment.Spec.Replicas != nil && *deployment.Spec.Replicas != int32(1) {
		// 1 is the default value for Replicas.
		replicas = deployment.Spec.Replicas
	}
	hashableDeployment.Spec.Replicas = replicas
	delete(hashableDeployment.Labels, controller.ControllerDeploymentHashLabel)
	hashableDeployment.Spec.Selector = deployment.Spec.Selector

	return &hashableDeployment
}

// cmpMatchExpressions is a helper for hashableDeployment.
func cmpMatchExpressions(a, b metav1.LabelSelectorRequirement) bool {
	if a.Key != b.Key {
		return a.Key < b.Key
	}
	if a.Operator != b.Operator {
		return a.Operator < b.Operator
	}
	for i := range b.Values {
		if i == len(a.Values) {
			return true
		}
		if a.Values[i] != b.Values[i] {
			return a.Values[i] < b.Values[i]
		}
	}
	return false
}

// zeroOutDeploymentHash is a helper for hashableDeployment.
func zeroOutDeploymentHash(labelSelector *metav1.LabelSelector) {
	if labelSelector != nil {
		for i, expr := range labelSelector.MatchExpressions {
			if expr.Key == controller.ControllerDeploymentHashLabel {
				// Hash value should be ignored.
				labelSelector.MatchExpressions[i].Values = nil
			}
		}
	}
}

// hashableProbe returns a copy of the given probe with exactly the fields that
// should be used for computing a deployment's hash copied over.  Fields that
// should be ignored, or that have explicit values that are equal to their
// respective default values, will be zeroed.
func hashableProbe(probe *corev1.Probe) *corev1.Probe {
	if probe == nil {
		return nil
	}

	var hashableProbe corev1.Probe

	copyProbe(probe, &hashableProbe)

	return &hashableProbe
}

// currentRouterDeployment returns the current router deployment.
func (r *reconciler) currentRouterDeployment(ci *operatorv1.IngressController) (bool, *appsv1.Deployment, error) {
	deployment := &appsv1.Deployment{}
	if err := r.client.Get(context.TODO(), controller.RouterDeploymentName(ci), deployment); err != nil {
		if errors.IsNotFound(err) {
			return false, nil, nil
		}
		return false, nil, err
	}
	return true, deployment, nil
}

// createRouterDeployment creates a router deployment.
func (r *reconciler) createRouterDeployment(deployment *appsv1.Deployment) error {
	if err := r.client.Create(context.TODO(), deployment); err != nil {
		return fmt.Errorf("failed to create router deployment %s/%s: %v", deployment.Namespace, deployment.Name, err)
	}
	log.Info("created router deployment", "namespace", deployment.Namespace, "name", deployment.Name)
	return nil
}

// updateRouterDeployment updates a router deployment.
func (r *reconciler) updateRouterDeployment(current, desired *appsv1.Deployment) (bool, error) {
	changed, updated := deploymentConfigChanged(current, desired)
	if !changed {
		return false, nil
	}

	// Diff before updating because the client may mutate the object.
	diff := cmp.Diff(current, updated, cmpopts.EquateEmpty())
	if err := r.client.Update(context.TODO(), updated); err != nil {
		return false, fmt.Errorf("failed to update router deployment %s/%s: %v", updated.Namespace, updated.Name, err)
	}
	log.Info("updated router deployment", "namespace", updated.Namespace, "name", updated.Name, "diff", diff)
	return true, nil
}

// deepHashObject writes a specified object to a hash using the spew library
// which follows pointers and prints actual values of the nested objects
// ensuring the hash does not change when a pointer changes.
//
// Copied from github.com/kubernetes/kubernetes/pkg/util/hash/hash.go.
func deepHashObject(hasher hash.Hash, objectToWrite interface{}) {
	hasher.Reset()
	printer := spew.ConfigState{
		Indent:         " ",
		SortKeys:       true,
		DisableMethods: true,
		SpewKeys:       true,
	}
	printer.Fprintf(hasher, "%#v", objectToWrite)
}

// deploymentConfigChanged checks if current config matches the expected config
// for the ingress controller deployment and if it does not, returns the updated config.
func deploymentConfigChanged(current, expected *appsv1.Deployment) (bool, *appsv1.Deployment) {
	if deploymentHash(current) == deploymentHash(expected) {
		return false, nil
	}

	updated := current.DeepCopy()
	// Copy the primary container from current and update its fields
	// selectively.  Copy any sidecars from expected verbatim.
	containers := make([]corev1.Container, len(expected.Spec.Template.Spec.Containers))
	containers[0] = updated.Spec.Template.Spec.Containers[0]
	for i, container := range expected.Spec.Template.Spec.Containers[1:] {
		containers[i+1] = *container.DeepCopy()
	}
	updated.Spec.Template.Spec.Containers = containers
	updated.Spec.Template.Spec.DNSPolicy = expected.Spec.Template.Spec.DNSPolicy
	updated.Spec.Template.Labels = expected.Spec.Template.Labels

	annotations := []string{LivenessGracePeriodSecondsAnnotation, WorkloadPartitioningManagement}
	for _, key := range annotations {
		if val, ok := expected.Spec.Template.Annotations[key]; ok && len(val) > 0 {
			if updated.Spec.Template.Annotations == nil {
				updated.Spec.Template.Annotations = make(map[string]string)
			}
			updated.Spec.Template.Annotations[key] = val
		}
	}

	updated.Spec.Strategy = expected.Spec.Strategy
	volumes := make([]corev1.Volume, len(expected.Spec.Template.Spec.Volumes))
	for i, vol := range expected.Spec.Template.Spec.Volumes {
		volumes[i] = *vol.DeepCopy()
	}
	updated.Spec.Template.Spec.Volumes = volumes
	updated.Spec.Template.Spec.NodeSelector = expected.Spec.Template.Spec.NodeSelector
	updated.Spec.Template.Spec.Containers[0].SecurityContext = expected.Spec.Template.Spec.Containers[0].SecurityContext
	updated.Spec.Template.Spec.Containers[0].Env = expected.Spec.Template.Spec.Containers[0].Env
	updated.Spec.Template.Spec.Containers[0].Image = expected.Spec.Template.Spec.Containers[0].Image
	copyProbe(expected.Spec.Template.Spec.Containers[0].LivenessProbe, updated.Spec.Template.Spec.Containers[0].LivenessProbe)
	copyProbe(expected.Spec.Template.Spec.Containers[0].ReadinessProbe, updated.Spec.Template.Spec.Containers[0].ReadinessProbe)
	copyProbe(expected.Spec.Template.Spec.Containers[0].StartupProbe, updated.Spec.Template.Spec.Containers[0].StartupProbe)
	updated.Spec.Template.Spec.Containers[0].VolumeMounts = expected.Spec.Template.Spec.Containers[0].VolumeMounts
	updated.Spec.Template.Spec.Containers[0].Ports = expected.Spec.Template.Spec.Containers[0].Ports
	updated.Spec.Template.Spec.Tolerations = expected.Spec.Template.Spec.Tolerations
	updated.Spec.Template.Spec.TopologySpreadConstraints = expected.Spec.Template.Spec.TopologySpreadConstraints
	updated.Spec.Template.Spec.Affinity = expected.Spec.Template.Spec.Affinity
	replicas := int32(1)
	if expected.Spec.Replicas != nil {
		replicas = *expected.Spec.Replicas
	}
	updated.Spec.Replicas = &replicas
	updated.Spec.MinReadySeconds = expected.Spec.MinReadySeconds
	return true, updated
}

// copyProbe copies probe parameters that the operator manages from probe a to
// probe b.
func copyProbe(a, b *corev1.Probe) {
	if a == nil || b == nil {
		return
	}

	if a.ProbeHandler.HTTPGet != nil {
		b.ProbeHandler.HTTPGet = &corev1.HTTPGetAction{
			Path: a.ProbeHandler.HTTPGet.Path,
			Port: a.ProbeHandler.HTTPGet.Port,
			Host: a.ProbeHandler.HTTPGet.Host,
		}
		if a.ProbeHandler.HTTPGet.Scheme != "HTTP" {
			b.ProbeHandler.HTTPGet.Scheme = a.ProbeHandler.HTTPGet.Scheme
		}
	}

	// Users are permitted to modify the timeout, so *don't* copy it.

	// Don't copy default values that the API set.
	if a.PeriodSeconds != int32(10) {
		b.PeriodSeconds = a.PeriodSeconds
	}
	if a.SuccessThreshold != int32(1) {
		b.SuccessThreshold = a.SuccessThreshold
	}
	if a.FailureThreshold != int32(3) {
		b.FailureThreshold = a.FailureThreshold
	}
}

// clipHAProxyTimeoutValue prevents the HAProxy config file from using
// timeout values that exceed the maximum value allowed by HAProxy.
// Returns an error in the event that a timeout string value is not
// parsable as a valid time duration, or the clipped time duration
// otherwise.
//
// TODO: this is a modified copy from openshift/router but returns ""
// if there's any error.
//
// Ideally we need to share this utility function via:
// https://github.com/openshift/library-go/blob/master/pkg/route/routeapihelpers
func clipHAProxyTimeoutValue(val string) (string, error) {
	const haproxyMaxTimeout = "2147483647ms" // max timeout allowable by HAProxy

	endIndex := len(val) - 1
	maxTimeout, err := time.ParseDuration(haproxyMaxTimeout)
	if err != nil {
		return "", err
	}
	// time.ParseDuration doesn't work with days
	// despite HAProxy accepting timeouts that specify day units
	if val[endIndex] == 'd' {
		days, err := strconv.Atoi(val[:endIndex])
		if err != nil {
			return "", err
		}
		if maxTimeout.Hours() < float64(days*24) {
			log.V(7).Info("Route annotation timeout exceeds maximum allowable by HAProxy, clipping to max")
			return haproxyMaxTimeout, nil
		}
	} else {
		duration, err := time.ParseDuration(val)
		if err != nil {
			return "", err
		}
		if maxTimeout.Milliseconds() < duration.Milliseconds() {
			log.V(7).Info("Route annotation timeout exceeds maximum allowable by HAProxy, clipping to max")
			return haproxyMaxTimeout, nil
		}
	}
	return val, nil
}

// durationToHAProxyTimespec converts a time.Duration into a number that
// HAProxy can consume, in the simplest unit possible. If the value would be
// truncated by being converted to milliseconds, it outputs in microseconds, or
// if the value would be truncated by being converted to seconds, it outputs in
// milliseconds, otherwise if the value wouldn't be truncated by converting to
// seconds, but would be if converted to minutes, it outputs in seconds, etc.
// up to a maximum unit in hours (the largest time unit natively supported by
// time.Duration).
//
// Also truncates values to the maximum length HAProxy allows if the value is
// too large, and truncates values to 0s if they are less than 0.
func durationToHAProxyTimespec(duration time.Duration) string {
	if duration <= 0 {
		return "0s"
	} else if duration > haproxyMaxTimeoutMilliseconds {
		log.V(2).Info("time value %v exceeds the maximum timeout length of %v; truncating to maximum value", duration, haproxyMaxTimeoutMilliseconds)
		return "2147483647ms"
	} else if µs := duration.Microseconds(); µs%1000 != 0 {
		return fmt.Sprintf("%dus", µs)
	} else if ms := duration.Milliseconds(); ms%1000 != 0 {
		return fmt.Sprintf("%dms", ms)
	} else if ms%time.Minute.Milliseconds() != 0 {
		return fmt.Sprintf("%ds", int(math.Round(duration.Seconds())))
	} else if ms%time.Hour.Milliseconds() != 0 {
		return fmt.Sprintf("%dm", int(math.Round(duration.Minutes())))
	} else {
		return fmt.Sprintf("%dh", int(math.Round(duration.Hours())))
	}
}

// GetMIMETypes returns a slice of strings from an array of operatorv1.CompressionMIMETypes.
// MIME strings that contain spaces must be quoted, as HAProxy requires a space-delimited MIME
// type list. Also quote/escape any characters that are special to HAProxy (\,', and ").
// See http://cbonte.github.io/haproxy-dconv/2.2/configuration.html#2.2
func GetMIMETypes(mimeTypes []operatorv1.CompressionMIMEType) []string {
	var mimes []string

	for _, m := range mimeTypes {
		mimeType := string(m)
		if strings.ContainsAny(mimeType, ` \"`) {
			mimeType = strconv.Quote(mimeType)
		}
		// A single quote doesn't get escaped by strconv.Quote, so do it explicitly
		if strings.Contains(mimeType, "'") {
			mimeType = strings.ReplaceAll(mimeType, "'", "\\'")
		}
		mimes = append(mimes, mimeType)
	}

	return mimes
}

// caps the value of ReloadInterval between the bounds of 1s and 120s
// returns the default of 5s if the user gives a 0 value
func capReloadIntervalValue(interval time.Duration) time.Duration {
	const (
		maxInterval     = 120 * time.Second
		minInterval     = 1 * time.Second
		defaultInterval = 5 * time.Second
	)

	switch {
	case interval == 0:
		return defaultInterval
	case interval > maxInterval:
		return maxInterval
	case interval < minInterval:
		return minInterval
	default:
		return interval
	}
}
