package ingress

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/openshift/cluster-ingress-operator/pkg/manifests"
	"github.com/openshift/cluster-ingress-operator/pkg/operator/controller"
	corev1 "k8s.io/api/core/v1"

	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"

	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// awsLBAdditionalResourceTags is a comma separated list of
	// Key=Value pairs that are additionally recorded on
	// load balancer resources and security groups.
	//
	// https://kubernetes.io/docs/concepts/services-networking/service/#aws-load-balancer-additional-resource-tags
	awsLBAdditionalResourceTags = "service.beta.kubernetes.io/aws-load-balancer-additional-resource-tags"

	// awsLBProxyProtocolAnnotation is used to enable the PROXY protocol on any
	// AWS load balancer services created.
	//
	// https://kubernetes.io/docs/concepts/services-networking/service/#proxy-protocol-support-on-aws
	awsLBProxyProtocolAnnotation = "service.beta.kubernetes.io/aws-load-balancer-proxy-protocol"

	// AWSLBTypeAnnotation is a Service annotation used to specify an AWS load
	// balancer type. See the following for additional details:
	//
	// https://kubernetes.io/docs/concepts/services-networking/service/#aws-nlb-support
	AWSLBTypeAnnotation = "service.beta.kubernetes.io/aws-load-balancer-type"

	// AWSNLBAnnotation is the annotation value of an AWS Network Load Balancer (NLB).
	AWSNLBAnnotation = "nlb"

	// awsInternalLBAnnotation is the annotation used on a service to specify an AWS
	// load balancer as being internal.
	awsInternalLBAnnotation = "service.beta.kubernetes.io/aws-load-balancer-internal"

	// awsLBHealthCheckIntervalAnnotation is the approximate interval, in seconds, between AWS
	// load balancer health checks of an individual AWS instance. Defaults to 5, must be between
	// 5 and 300.
	awsLBHealthCheckIntervalAnnotation = "service.beta.kubernetes.io/aws-load-balancer-healthcheck-interval"
	awsLBHealthCheckIntervalDefault    = "5"
	// Network Load Balancers require a health check interval of 10 or 30.
	// See https://docs.aws.amazon.com/elasticloadbalancing/latest/network/target-group-health-checks.html
	awsLBHealthCheckIntervalNLB = "10"

	// awsLBHealthCheckTimeoutAnnotation is the amount of time, in seconds, during which no response
	// means a failed AWS load balancer health check. The value must be less than the value of
	// awsLBHealthCheckIntervalAnnotation. Defaults to 4, must be between 2 and 60.
	awsLBHealthCheckTimeoutAnnotation = "service.beta.kubernetes.io/aws-load-balancer-healthcheck-timeout"
	awsLBHealthCheckTimeoutDefault    = "4"

	// awsLBHealthCheckUnhealthyThresholdAnnotation is the number of unsuccessful health checks required
	// for an AWS load balancer backend to be considered unhealthy for traffic. Defaults to 2, must be
	// between 2 and 10.
	awsLBHealthCheckUnhealthyThresholdAnnotation = "service.beta.kubernetes.io/aws-load-balancer-healthcheck-unhealthy-threshold"
	awsLBHealthCheckUnhealthyThresholdDefault    = "2"

	// awsLBHealthCheckHealthyThresholdAnnotation is the number of successive successful health checks
	// required for an AWS load balancer backend to be considered healthy for traffic. Defaults to 2,
	// must be between 2 and 10.
	awsLBHealthCheckHealthyThresholdAnnotation = "service.beta.kubernetes.io/aws-load-balancer-healthcheck-healthy-threshold"
	awsLBHealthCheckHealthyThresholdDefault    = "2"

	// awsELBConnectionIdleTimeoutAnnotation specifies the timeout for idle
	// connections for a Classic ELB.
	awsELBConnectionIdleTimeoutAnnotation = "service.beta.kubernetes.io/aws-load-balancer-connection-idle-timeout"

	// iksLBScopeAnnotation is the annotation used on a service to specify an IBM
	// load balancer IP type.
	iksLBScopeAnnotation = "service.kubernetes.io/ibm-load-balancer-cloud-provider-ip-type"
	// iksLBScopePublic is the service annotation value used to specify an IBM load balancer
	// IP as public.
	iksLBScopePublic = "public"
	// iksLBScopePublic is the service annotation value used to specify an IBM load balancer
	// IP as private.
	iksLBScopePrivate = "private"

	// azureInternalLBAnnotation is the annotation used on a service to specify an Azure
	// load balancer as being internal.
	azureInternalLBAnnotation = "service.beta.kubernetes.io/azure-load-balancer-internal"

	// gcpLBTypeAnnotation is the annotation used on a service to specify a type of GCP
	// load balancer.
	gcpLBTypeAnnotation = "cloud.google.com/load-balancer-type"

	// GCPGlobalAccessAnnotation is the annotation used on an internal load balancer service
	// to enable the GCP Global Access feature.
	GCPGlobalAccessAnnotation = "networking.gke.io/internal-load-balancer-allow-global-access"

	// openstackInternalLBAnnotation is the annotation used on a service to specify an
	// OpenStack load balancer as being internal.
	openstackInternalLBAnnotation = "service.beta.kubernetes.io/openstack-internal-load-balancer"

	// localWithFallbackAnnotation is the annotation used on a service that
	// has "Local" external traffic policy to indicate that the service
	// proxy should prefer using a local endpoint but forward traffic to any
	// available endpoint if no local endpoint is available.
	localWithFallbackAnnotation = "traffic-policy.network.alpha.openshift.io/local-with-fallback"

	// alibabaCloudLBAddressTypeAnnotation is the annotation used on a service
	// to specify the network type of an Aliyun SLB
	alibabaCloudLBAddressTypeAnnotation = "service.beta.kubernetes.io/alibaba-cloud-loadbalancer-address-type"

	// alibabaCloudLBAddressTypeInternet is the service annotation value used to specify an Aliyun SLB
	// IP is exposed to the internet (public)
	alibabaCloudLBAddressTypeInternet = "internet"

	// alibabaCloudLBAddressTypeIntranet is the service annotation value used to specify an Aliyun SLB
	// IP is exposed to the intranet (private)
	alibabaCloudLBAddressTypeIntranet = "intranet"

	// autoDeleteLoadBalancerAnnotation is an annotation that can be set on
	// an IngressController to indicate that the operator should
	// automatically delete any associated service load-balancer when its
	// scope changes if changing scope requires deleting service
	// load-balancers on the current platform.
	autoDeleteLoadBalancerAnnotation = "ingress.operator.openshift.io/auto-delete-load-balancer"
)

var (
	// InternalLBAnnotations maps platform to the annotation name and value
	// that tell the cloud provider that is associated with the platform
	// that the load balancer is internal.
	//
	// https://kubernetes.io/docs/concepts/services-networking/service/#internal-load-balancer
	InternalLBAnnotations = map[configv1.PlatformType]map[string]string{
		// Prior to 4.8, the aws internal LB annotation was set to "0.0.0.0/0".
		// While "0.0.0.0/0" is valid, the preferred value, according to the
		// documentation[1], is "true".
		// [1] https://kubernetes.io/docs/concepts/services-networking/service/#internal-load-balancer
		configv1.AWSPlatformType: {
			awsInternalLBAnnotation: "true",
		},
		configv1.AzurePlatformType: {
			// Azure load balancers are not customizable and are set to (2 fail @ 5s interval, 2 healthy)
			azureInternalLBAnnotation: "true",
		},
		// There are no required annotations for this platform as of
		// 2019-06-17 (but maybe MetalLB will have something for
		// internal load-balancers in the future).
		configv1.BareMetalPlatformType: nil,
		configv1.GCPPlatformType: {
			gcpLBTypeAnnotation: "Internal",
		},
		// There are no required annotations for this platform.
		configv1.LibvirtPlatformType: nil,
		configv1.OpenStackPlatformType: {
			openstackInternalLBAnnotation: "true",
		},
		configv1.NonePlatformType: nil,
		// vSphere does not support load balancers as of 2019-06-17.
		configv1.VSpherePlatformType: nil,
		configv1.IBMCloudPlatformType: {
			iksLBScopeAnnotation: iksLBScopePrivate,
		},
		configv1.PowerVSPlatformType: {
			iksLBScopeAnnotation: iksLBScopePrivate,
		},
		configv1.AlibabaCloudPlatformType: {
			alibabaCloudLBAddressTypeAnnotation: alibabaCloudLBAddressTypeIntranet,
		},
		configv1.NutanixPlatformType: nil,
	}

	// externalLBAnnotations maps platform to the annotation name and value
	// that tell the cloud provider that is associated with the platform
	// that the load balancer is external.  This is the default for most
	// platforms; only platforms for which it is not the default need
	// entries in this map.
	externalLBAnnotations = map[configv1.PlatformType]map[string]string{
		configv1.IBMCloudPlatformType: {
			iksLBScopeAnnotation: iksLBScopePublic,
		},
		configv1.PowerVSPlatformType: {
			iksLBScopeAnnotation: iksLBScopePublic,
		},
	}

	// platformsWithMutableScope is the set of platforms that support
	// mutating load-balancer scope without deleting and recreating a
	// service load-balancer.
	platformsWithMutableScope = map[configv1.PlatformType]struct{}{
		configv1.AzurePlatformType: {},
		configv1.GCPPlatformType:   {},
	}

	// managedLoadBalancerServiceAnnotations is a set of annotation keys for
	// annotations that the operator manages for LoadBalancer-type services.
	// The operator preserves all other annotations.
	//
	// Be careful when adding annotation keys to this set.  If a new release
	// of the operator starts managing an annotation that it previously
	// ignored, it could stomp annotations that the user has set when the
	// user upgrades the operator to the new release (see
	// <https://bugzilla.redhat.com/show_bug.cgi?id=1905490>).  In order to
	// avoid problems, make sure the previous release blocks upgrades when
	// the user has modified an annotation that the new release manages.
	managedLoadBalancerServiceAnnotations = func() sets.String {
		result := sets.NewString(
			// AWS LB health check interval annotation (see
			// <https://bugzilla.redhat.com/show_bug.cgi?id=1908758>).
			awsLBHealthCheckIntervalAnnotation,
			// AWS connection idle timeout annotation.
			awsELBConnectionIdleTimeoutAnnotation,
			// GCP Global Access internal Load Balancer annotation
			// (see <https://issues.redhat.com/browse/NE-447>).
			GCPGlobalAccessAnnotation,
			// local-with-fallback annotation for kube-proxy (see
			// <https://bugzilla.redhat.com/show_bug.cgi?id=1960284>).
			localWithFallbackAnnotation,
			// AWS load balancer type annotation to set either CLB/ELB or NLB
			AWSLBTypeAnnotation,
			// awsLBProxyProtocolAnnotation is used to enable the PROXY protocol on any
			// AWS load balancer services created.
			//
			// https://kubernetes.io/docs/concepts/services-networking/service/#proxy-protocol-support-on-aws
			awsLBProxyProtocolAnnotation,
		)

		// Azure and GCP support switching between internal and external
		// scope by changing the annotation, so the operator manages the
		// corresponding load-balancer scope annotations for these
		// platforms.  Other platforms require deleting and recreating
		// the service, so the operator doesn't update the annotations
		// that specify load-balancer scope for those platforms.  See
		// <https://issues.redhat.com/browse/NE-621>.
		for platform := range platformsWithMutableScope {
			for name := range InternalLBAnnotations[platform] {
				result.Insert(name)
			}
			for name := range externalLBAnnotations[platform] {
				result.Insert(name)
			}
		}

		return result
	}()
)

// ensureLoadBalancerService creates an LB service if one is desired but absent.
// Always returns the current LB service if one exists (whether it already
// existed or was created during the course of the function).
func (r *reconciler) ensureLoadBalancerService(ci *operatorv1.IngressController, deploymentRef metav1.OwnerReference, platformStatus *configv1.PlatformStatus) (bool, *corev1.Service, error) {
	wantLBS, desiredLBService, err := desiredLoadBalancerService(ci, deploymentRef, platformStatus)
	if err != nil {
		return false, nil, err
	}

	haveLBS, currentLBService, err := r.currentLoadBalancerService(ci)
	if err != nil {
		return false, nil, err
	}

	// BZ2054200: Don't modify/delete services that are not directly owned by this controller.
	ownLBS := isServiceOwnedByIngressController(currentLBService, ci)

	switch {
	case !wantLBS && !haveLBS:
		return false, nil, nil
	case !wantLBS && haveLBS:
		if !ownLBS {
			return false, nil, fmt.Errorf("a conflicting load balancer service exists that is not owned by the ingress controller: %s", controller.LoadBalancerServiceName(ci))
		}
		if err := r.deleteLoadBalancerService(currentLBService, &crclient.DeleteOptions{}); err != nil {
			return true, currentLBService, err
		}
		return false, nil, nil
	case wantLBS && !haveLBS:
		if err := r.createLoadBalancerService(desiredLBService); err != nil {
			return false, nil, err
		}
		return r.currentLoadBalancerService(ci)
	case wantLBS && haveLBS:
		if !ownLBS {
			return false, nil, fmt.Errorf("a conflicting load balancer service exists that is not owned by the ingress controller: %s", controller.LoadBalancerServiceName(ci))
		}
		if updated, err := r.normalizeLoadBalancerServiceAnnotations(currentLBService); err != nil {
			return true, currentLBService, fmt.Errorf("failed to normalize annotations for load balancer service: %w", err)
		} else if updated {
			haveLBS, currentLBService, err = r.currentLoadBalancerService(ci)
			if err != nil {
				return haveLBS, currentLBService, err
			}
		}
		deleteIfScopeChanged := false
		if _, ok := ci.Annotations[autoDeleteLoadBalancerAnnotation]; ok {
			deleteIfScopeChanged = true
		}
		if updated, err := r.updateLoadBalancerService(currentLBService, desiredLBService, platformStatus, deleteIfScopeChanged); err != nil {
			return true, currentLBService, fmt.Errorf("failed to update load balancer service: %v", err)
		} else if updated {
			return r.currentLoadBalancerService(ci)
		}
	}
	return true, currentLBService, nil
}

// isServiceOwnedByIngressController determines whether a service is owned by an ingress controller.
func isServiceOwnedByIngressController(service *corev1.Service, ic *operatorv1.IngressController) bool {
	if service != nil && service.Labels[manifests.OwningIngressControllerLabel] == ic.Name {
		return true
	}
	return false
}

// desiredLoadBalancerService returns the desired LB service for a
// ingresscontroller, or nil if an LB service isn't desired. An LB service is
// desired if the high availability type is Cloud. An LB service will declare an
// owner reference to the given deployment.
func desiredLoadBalancerService(ci *operatorv1.IngressController, deploymentRef metav1.OwnerReference, platform *configv1.PlatformStatus) (bool, *corev1.Service, error) {
	if ci.Status.EndpointPublishingStrategy.Type != operatorv1.LoadBalancerServiceStrategyType {
		return false, nil, nil
	}
	service := manifests.LoadBalancerService()

	name := controller.LoadBalancerServiceName(ci)

	service.Namespace = name.Namespace
	service.Name = name.Name

	if service.Labels == nil {
		service.Labels = map[string]string{}
	}
	service.Labels["router"] = name.Name
	service.Labels[manifests.OwningIngressControllerLabel] = ci.Name

	service.Spec.Selector = controller.IngressControllerDeploymentPodSelector(ci).MatchLabels

	lb := ci.Status.EndpointPublishingStrategy.LoadBalancer
	isInternal := lb != nil && lb.Scope == operatorv1.InternalLoadBalancer

	if service.Annotations == nil {
		service.Annotations = map[string]string{}
	}
	if proxyNeeded, err := IsProxyProtocolNeeded(ci, platform); err != nil {
		return false, nil, fmt.Errorf("failed to determine if proxy protocol is proxyNeeded for ingresscontroller %q: %v", ci.Name, err)
	} else if proxyNeeded {
		service.Annotations[awsLBProxyProtocolAnnotation] = "*"
	}

	if platform != nil {
		if isInternal {
			annotation := InternalLBAnnotations[platform.Type]
			for name, value := range annotation {
				service.Annotations[name] = value
			}

			// Set the GCP Global Access annotation for internal load balancers on GCP only
			if platform.Type == configv1.GCPPlatformType {
				if lb != nil && lb.ProviderParameters != nil &&
					lb.ProviderParameters.Type == operatorv1.GCPLoadBalancerProvider &&
					lb.ProviderParameters.GCP != nil {
					globalAccessEnabled := lb.ProviderParameters.GCP.ClientAccess == operatorv1.GCPGlobalAccess
					service.Annotations[GCPGlobalAccessAnnotation] = strconv.FormatBool(globalAccessEnabled)
				}
			}
		} else {
			annotation := externalLBAnnotations[platform.Type]
			for name, value := range annotation {
				service.Annotations[name] = value
			}
		}
		switch platform.Type {
		case configv1.AWSPlatformType:
			service.Annotations[awsLBHealthCheckIntervalAnnotation] = awsLBHealthCheckIntervalDefault
			if lb != nil && lb.ProviderParameters != nil {
				if aws := lb.ProviderParameters.AWS; aws != nil && lb.ProviderParameters.Type == operatorv1.AWSLoadBalancerProvider {
					switch aws.Type {
					case operatorv1.AWSNetworkLoadBalancer:
						service.Annotations[AWSLBTypeAnnotation] = AWSNLBAnnotation
						// NLBs require a different health check interval than CLBs.
						// See <https://bugzilla.redhat.com/show_bug.cgi?id=1908758>.
						service.Annotations[awsLBHealthCheckIntervalAnnotation] = awsLBHealthCheckIntervalNLB
					case operatorv1.AWSClassicLoadBalancer:
						if aws.ClassicLoadBalancerParameters != nil {
							if v := aws.ClassicLoadBalancerParameters.ConnectionIdleTimeout; v.Duration > 0 {
								service.Annotations[awsELBConnectionIdleTimeoutAnnotation] = strconv.FormatUint(uint64(v.Round(time.Second).Seconds()), 10)
							}
						}
					}
				}
			}

			if platform.AWS != nil && len(platform.AWS.ResourceTags) > 0 {
				var additionalTags []string
				for _, userTag := range platform.AWS.ResourceTags {
					if len(userTag.Key) > 0 {
						additionalTags = append(additionalTags, userTag.Key+"="+userTag.Value)
					}
				}
				if len(additionalTags) > 0 {
					service.Annotations[awsLBAdditionalResourceTags] = strings.Join(additionalTags, ",")
				}
			}

			// Set the load balancer for AWS to be as aggressive as Azure (2 fail @ 5s interval, 2 healthy)
			service.Annotations[awsLBHealthCheckTimeoutAnnotation] = awsLBHealthCheckTimeoutDefault
			service.Annotations[awsLBHealthCheckUnhealthyThresholdAnnotation] = awsLBHealthCheckUnhealthyThresholdDefault
			service.Annotations[awsLBHealthCheckHealthyThresholdAnnotation] = awsLBHealthCheckHealthyThresholdDefault
		case configv1.IBMCloudPlatformType, configv1.PowerVSPlatformType:
			// Set ExternalTrafficPolicy to type Cluster - IBM's LoadBalancer impl is created within the cluster.
			// LB places VIP on one of the worker nodes, using keepalived to maintain the VIP and ensuring redundancy
			// LB relies on iptable rules kube-proxy puts in to send traffic from the VIP node to the cluster
			// If policy is local, traffic is only sent to pods on the local node, as such Cluster enables traffic to flow to  all the pods in the cluster
			service.Spec.ExternalTrafficPolicy = corev1.ServiceExternalTrafficPolicyTypeCluster
		case configv1.AlibabaCloudPlatformType:
			if !isInternal {
				service.Annotations[alibabaCloudLBAddressTypeAnnotation] = alibabaCloudLBAddressTypeInternet
			}
		}
		// Azure load balancers are not customizable and are set to (2 fail @ 5s interval, 2 healthy)
		// GCP load balancers are not customizable and are set to (3 fail @ 8s interval, 1 healthy)

		if v, err := shouldUseLocalWithFallback(ci, service); err != nil {
			return true, service, err
		} else if v {
			service.Annotations[localWithFallbackAnnotation] = ""
		}
	}

	if ci.Spec.EndpointPublishingStrategy != nil {
		lb := ci.Spec.EndpointPublishingStrategy.LoadBalancer
		if lb != nil && len(lb.AllowedSourceRanges) > 0 {
			cidrs := make([]string, len(lb.AllowedSourceRanges))
			for i, cidr := range lb.AllowedSourceRanges {
				cidrs[i] = string(cidr)
			}
			service.Spec.LoadBalancerSourceRanges = cidrs
		}
	}

	service.SetOwnerReferences([]metav1.OwnerReference{deploymentRef})
	return true, service, nil
}

// shouldUseLocalWithFallback returns a Boolean value indicating whether the
// local-with-fallback annotation should be set for the given service, and
// returns an error if the given ingresscontroller has an invalid unsupported
// config override.
func shouldUseLocalWithFallback(ic *operatorv1.IngressController, service *corev1.Service) (bool, error) {
	// By default, use local-with-fallback when using the "Local" external
	// traffic policy.
	if service.Spec.ExternalTrafficPolicy != corev1.ServiceExternalTrafficPolicyTypeLocal {
		return false, nil
	}

	// Allow the user to override local-with-fallback.
	if len(ic.Spec.UnsupportedConfigOverrides.Raw) > 0 {
		var unsupportedConfigOverrides struct {
			LocalWithFallback string `json:"localWithFallback"`
		}
		if err := json.Unmarshal(ic.Spec.UnsupportedConfigOverrides.Raw, &unsupportedConfigOverrides); err != nil {
			return false, fmt.Errorf("ingresscontroller %q has invalid spec.unsupportedConfigOverrides: %w", ic.Name, err)
		}
		override := unsupportedConfigOverrides.LocalWithFallback
		if len(override) != 0 {
			if val, err := strconv.ParseBool(override); err != nil {
				return false, fmt.Errorf("ingresscontroller %q has invalid spec.unsupportedConfigOverrides.localWithFallback: %w", ic.Name, err)
			} else {
				return val, nil
			}
		}
	}

	return true, nil
}

// currentLoadBalancerService returns any existing LB service for the
// ingresscontroller.
func (r *reconciler) currentLoadBalancerService(ci *operatorv1.IngressController) (bool, *corev1.Service, error) {
	service := &corev1.Service{}
	if err := r.client.Get(context.TODO(), controller.LoadBalancerServiceName(ci), service); err != nil {
		if errors.IsNotFound(err) {
			return false, nil, nil
		}
		return false, nil, err
	}
	return true, service, nil
}

// normalizeLoadBalancerServiceAnnotations normalizes annotations for the
// provided LoadBalancer-type service.
func (r *reconciler) normalizeLoadBalancerServiceAnnotations(service *corev1.Service) (bool, error) {
	// On AWS, the service.beta.kubernetes.io/aws-load-balancer-internal
	// annotation can have either the value "0.0.0.0/0" or the value "true"
	// to indicate that the service load-balancer should be internal.
	// OpenShift 4.7 and earlier use the value "0.0.0.0/0", and OpenShift
	// 4.8 and later use the value "true".  A service that was created on an
	// older cluster might have the value "0.0.0.0/0".  Normalize the value
	// to ensure that comparisons that use "true" behave as expected.  See
	// <https://bugzilla.redhat.com/show_bug.cgi?id=2055470>.
	if v, ok := service.Annotations[awsInternalLBAnnotation]; ok && v == "0.0.0.0/0" {
		// Mutate a copy to avoid assuming we know where the current one came from
		// (i.e. it could have been from a cache).
		updated := service.DeepCopy()
		updated.Annotations[awsInternalLBAnnotation] = "true"
		if err := r.client.Update(context.TODO(), updated); err != nil {
			return false, fmt.Errorf("failed to normalize %s annotation on service %s/%s: %w", awsInternalLBAnnotation, service.Namespace, service.Name, err)
		}

		log.Info("normalized annotation", "namespace", service.Namespace, "name", service.Name, "annotation", awsInternalLBAnnotation, "old", v, "new", updated.Annotations[awsInternalLBAnnotation])

		return true, nil
	}

	return false, nil
}

// createLoadBalancerService creates a load balancer service.
func (r *reconciler) createLoadBalancerService(service *corev1.Service) error {
	if err := r.client.Create(context.TODO(), service); err != nil {
		return fmt.Errorf("failed to create load balancer service %s/%s: %v", service.Namespace, service.Name, err)
	}
	log.Info("created load balancer service", "namespace", service.Namespace, "name", service.Name)
	return nil
}

// deleteLoadBalancerService deletes a load balancer service.
func (r *reconciler) deleteLoadBalancerService(service *corev1.Service, options *crclient.DeleteOptions) error {
	if err := r.client.Delete(context.TODO(), service, options); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to delete load balancer service %s/%s: %v", service.Namespace, service.Name, err)
	}
	log.Info("deleted load balancer service", "namespace", service.Namespace, "name", service.Name)
	return nil
}

// updateLoadBalancerService updates a load balancer service.  Returns a Boolean
// indicating whether the service was updated, and an error value.
func (r *reconciler) updateLoadBalancerService(current, desired *corev1.Service, platform *configv1.PlatformStatus, deleteIfScopeChanged bool) (bool, error) {
	_, platformHasMutableScope := platformsWithMutableScope[platform.Type]
	if !platformHasMutableScope && deleteIfScopeChanged && !scopeEqual(current, desired, platform) {
		log.Info("deleting and recreating the load balancer because its scope changed", "namespace", desired.Namespace, "name", desired.Name)
		foreground := metav1.DeletePropagationForeground
		deleteOptions := crclient.DeleteOptions{PropagationPolicy: &foreground}
		if err := r.deleteLoadBalancerService(current, &deleteOptions); err != nil {
			return false, err
		}
		if err := r.createLoadBalancerService(desired); err != nil {
			return false, err
		}
		return true, nil
	}

	changed, updated := loadBalancerServiceChanged(current, desired)
	if !changed {
		return false, nil
	}
	// Diff before updating because the client may mutate the object.
	diff := cmp.Diff(current, updated, cmpopts.EquateEmpty())
	if err := r.client.Update(context.TODO(), updated); err != nil {
		return false, err
	}
	log.Info("updated load balancer service", "namespace", updated.Namespace, "name", updated.Name, "diff", diff)
	return true, nil
}

// scopeEqual returns true if the scope is the same between the two given
// services and false if the scope is different.
func scopeEqual(a, b *corev1.Service, platform *configv1.PlatformStatus) bool {
	aAnnotations := a.Annotations
	if aAnnotations == nil {
		aAnnotations = map[string]string{}
	}
	bAnnotations := b.Annotations
	if bAnnotations == nil {
		bAnnotations = map[string]string{}
	}
	for name := range InternalLBAnnotations[platform.Type] {
		if aAnnotations[name] != bAnnotations[name] {
			return false
		}
	}
	for name := range externalLBAnnotations[platform.Type] {
		if aAnnotations[name] != bAnnotations[name] {
			return false
		}
	}
	return true
}

// loadBalancerServiceChanged checks if the current load balancer service
// matches the expected and if not returns an updated one.
func loadBalancerServiceChanged(current, expected *corev1.Service) (bool, *corev1.Service) {
	// Preserve most fields and annotations.  If a new release of the
	// operator starts managing an annotation or spec field that it
	// previously ignored, it could stomp user changes when the user
	// upgrades the operator to the new release (see
	// <https://bugzilla.redhat.com/show_bug.cgi?id=1905490>).  In order to
	// avoid problems, make sure the previous release blocks upgrades when
	// the user has modified an annotation or spec field that the new
	// release manages.
	changed, updated := loadBalancerServiceAnnotationsChanged(current, expected, managedLoadBalancerServiceAnnotations)

	// If spec.loadBalancerSourceRanges is nonempty on the service, that
	// means that allowedSourceRanges is nonempty on the ingresscontroller,
	// which means we can clear the annotation if it's set and overwrite the
	// value in the current service.
	if len(expected.Spec.LoadBalancerSourceRanges) != 0 {
		if _, ok := current.Annotations[corev1.AnnotationLoadBalancerSourceRangesKey]; ok {
			if !changed {
				changed = true
				updated = current.DeepCopy()
			}
			delete(updated.Annotations, corev1.AnnotationLoadBalancerSourceRangesKey)
		}
		if !reflect.DeepEqual(current.Spec.LoadBalancerSourceRanges, expected.Spec.LoadBalancerSourceRanges) {
			if !changed {
				changed = true
				updated = current.DeepCopy()
			}
			updated.Spec.LoadBalancerSourceRanges = expected.Spec.LoadBalancerSourceRanges
		}
	}

	return changed, updated
}

// loadBalancerServiceAnnotationsChanged checks if the annotations on the expected Service
// match the ones on the current Service.
func loadBalancerServiceAnnotationsChanged(current, expected *corev1.Service, annotations sets.String) (bool, *corev1.Service) {
	changed := false
	for annotation := range annotations {
		currentVal, have := current.Annotations[annotation]
		expectedVal, want := expected.Annotations[annotation]
		if (want && (!have || currentVal != expectedVal)) || (have && !want) {
			changed = true
			break
		}
	}
	if !changed {
		return false, nil
	}

	updated := current.DeepCopy()

	if updated.Annotations == nil {
		updated.Annotations = map[string]string{}
	}

	for annotation := range annotations {
		currentVal, have := current.Annotations[annotation]
		expectedVal, want := expected.Annotations[annotation]
		if want && (!have || currentVal != expectedVal) {
			updated.Annotations[annotation] = expected.Annotations[annotation]
		} else if have && !want {
			delete(updated.Annotations, annotation)
		}
	}

	return true, updated
}

// IsServiceInternal returns a Boolean indicating whether the provided service
// is annotated to request an internal load balancer.
func IsServiceInternal(service *corev1.Service) bool {
	for dk, dv := range service.Annotations {
		for _, annotations := range InternalLBAnnotations {
			for ik, iv := range annotations {
				if dk == ik && dv == iv {
					return true
				}
			}
		}
	}
	return false
}

// loadBalancerServiceTagsModified verifies that none of the managedAnnotations have been changed and also the AWS tags annotation
func loadBalancerServiceTagsModified(current, expected *corev1.Service) (bool, *corev1.Service) {
	ignoredAnnotations := managedLoadBalancerServiceAnnotations.Union(sets.NewString(awsLBAdditionalResourceTags))
	return loadBalancerServiceAnnotationsChanged(current, expected, ignoredAnnotations)
}

// loadBalancerServiceIsUpgradeable returns an error value indicating if the
// load balancer service is safe to upgrade.  In particular, if the current
// service matches the desired service, then the service is upgradeable, and the
// return value is nil.  Otherwise, if something or someone else has modified
// the service, then the return value is a non-nil error indicating that the
// modification must be reverted before upgrading is allowed.
func loadBalancerServiceIsUpgradeable(ic *operatorv1.IngressController, deploymentRef metav1.OwnerReference, current *corev1.Service, platform *configv1.PlatformStatus) error {
	want, desired, err := desiredLoadBalancerService(ic, deploymentRef, platform)
	if err != nil {
		return err
	}

	if !want {
		return nil
	}

	changed, updated := loadBalancerServiceTagsModified(current, desired)
	if changed {
		diff := cmp.Diff(current, updated, cmpopts.EquateEmpty())
		return fmt.Errorf("load balancer service has been modified; changes must be reverted before upgrading: %s", diff)
	}

	return nil
}

// loadBalancerServiceIsProgressing returns an error value indicating if the
// load balancer service is in progressing status.
func loadBalancerServiceIsProgressing(ic *operatorv1.IngressController, service *corev1.Service, platform *configv1.PlatformStatus) error {
	var errs []error
	wantScope := ic.Status.EndpointPublishingStrategy.LoadBalancer.Scope
	haveScope := operatorv1.ExternalLoadBalancer
	if IsServiceInternal(service) {
		haveScope = operatorv1.InternalLoadBalancer
	}
	if wantScope != haveScope {
		err := fmt.Errorf("The IngressController scope was changed from %q to %q.", haveScope, wantScope)
		switch platform.Type {
		case configv1.AWSPlatformType, configv1.IBMCloudPlatformType:
			err = fmt.Errorf("%[1]s  To effectuate this change, you must delete the service: `oc -n %[2]s delete svc/%[3]s`; the service load-balancer will then be deprovisioned and a new one created.  This will most likely cause the new load-balancer to have a different host name and IP address from the old one's.  Alternatively, you can revert the change to the IngressController: `oc -n openshift-ingress-operator patch ingresscontrollers/%[4]s --type=merge --patch='{\"spec\":{\"endpointPublishingStrategy\":{\"loadBalancer\":{\"scope\":\"%[5]s\"}}}}'", err.Error(), service.Namespace, service.Name, ic.Name, haveScope)
		}
		errs = append(errs, err)
	}

	errs = append(errs, loadBalancerSourceRangesAnnotationSet(service))
	errs = append(errs, loadBalancerSourceRangesMatch(ic, service))

	return kerrors.NewAggregate(errs)
}

// loadBalancerServiceEvaluationConditionsDetected returns an error value indicating if the
// load balancer service is in EvaluationConditionsDetected status.
func loadBalancerServiceEvaluationConditionsDetected(ic *operatorv1.IngressController, service *corev1.Service) error {
	var errs []error
	errs = append(errs, loadBalancerSourceRangesAnnotationSet(service))
	errs = append(errs, loadBalancerSourceRangesMatch(ic, service))

	return kerrors.NewAggregate(errs)
}

// loadBalancerSourceRangesAnnotationSet returns an error value indicating if the
// ingresscontroller associated with load balancer service should report the Progressing
// and EvaluationConditionsDetected status conditions with status True by checking
// if it has "service.beta.kubernetes.io/load-balancer-source-ranges"
// annotation set. If it is not set, then the service is not progressing and should not affect
// evaluation conditions, and the return value is nil.
// Otherwise, the return value is a non-nil error indicating that the annotation
// must be unset. The intention is to guide the cluster
// admin towards using the IngressController API and deprecate use of the service
// annotation for ingress.
func loadBalancerSourceRangesAnnotationSet(current *corev1.Service) error {
	if a, ok := current.Annotations[corev1.AnnotationLoadBalancerSourceRangesKey]; !ok || (ok && len(a) == 0) {
		return nil
	}

	return fmt.Errorf("You have manually edited an operator-managed object. You must revert your modifications by removing the %v annotation on service %q. You can use the new AllowedSourceRanges API field on the ingresscontroller object to configure this setting instead.", corev1.AnnotationLoadBalancerSourceRangesKey, current.Name)
}

// loadBalancerSourceRangesMatch returns an error value indicating if the
// ingresscontroller associated with the load balancer service should report the Progressing
// and EvaluationConditionsDetected status conditions with status True.  This function
// checks if the service's LoadBalancerSourceRanges field is empty when AllowedSourceRanges
// of the ingresscontroller is empty. If this is the case, then the service is not progressing
// and has no evaluation conditions, and the return value is nil. Otherwise, if AllowedSourceRanges
// is empty and LoadBalancerSourceRanges is nonempty, the return value is a non-nil error indicating
// that the LoadBalancerSourceRanges field must be cleared. The intention is to guide the cluster
// admin towards using the IngressController API and deprecate use of the LoadBalancerSourceRanges
// field for ingress.
func loadBalancerSourceRangesMatch(ic *operatorv1.IngressController, current *corev1.Service) error {
	if len(current.Spec.LoadBalancerSourceRanges) < 1 {
		return nil
	}
	if ic.Spec.EndpointPublishingStrategy != nil && ic.Spec.EndpointPublishingStrategy.LoadBalancer != nil {
		if len(ic.Spec.EndpointPublishingStrategy.LoadBalancer.AllowedSourceRanges) > 0 {
			return nil
		}
	}

	return fmt.Errorf("You have manually edited an operator-managed object. You must revert your modifications by removing the Spec.LoadBalancerSourceRanges field of LoadBalancer-typed service %q. You can use the new AllowedSourceRanges API field on the ingresscontroller to configure this setting instead.", current.Name)
}
