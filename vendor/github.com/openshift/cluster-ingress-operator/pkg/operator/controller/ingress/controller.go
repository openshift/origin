package ingress

import (
	"context"
	"fmt"
	"regexp"
	"regexp/syntax"
	"strings"
	"time"

	"github.com/pkg/errors"

	logf "github.com/openshift/cluster-ingress-operator/pkg/log"
	"github.com/openshift/cluster-ingress-operator/pkg/manifests"
	operatorcontroller "github.com/openshift/cluster-ingress-operator/pkg/operator/controller"
	routemetrics "github.com/openshift/cluster-ingress-operator/pkg/operator/controller/route-metrics"
	"github.com/openshift/cluster-ingress-operator/pkg/util/ingresscontroller"
	retryable "github.com/openshift/cluster-ingress-operator/pkg/util/retryableerror"
	"github.com/openshift/cluster-ingress-operator/pkg/util/slice"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"k8s.io/client-go/tools/record"

	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	iov1 "github.com/openshift/api/operatoringress/v1"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"

	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	controllerName = "ingress_controller"
)

// TODO: consider moving these to openshift/api
const (
	IngressControllerAdmittedConditionType                       = "Admitted"
	IngressControllerPodsScheduledConditionType                  = "PodsScheduled"
	IngressControllerDeploymentAvailableConditionType            = "DeploymentAvailable"
	IngressControllerDeploymentReplicasMinAvailableConditionType = "DeploymentReplicasMinAvailable"
	IngressControllerDeploymentReplicasAllAvailableConditionType = "DeploymentReplicasAllAvailable"
	IngressControllerDeploymentRollingOutConditionType           = "DeploymentRollingOut"
	IngressControllerLoadBalancerProgressingConditionType        = "LoadBalancerProgressing"
	IngressControllerCanaryCheckSuccessConditionType             = "CanaryChecksSucceeding"
	IngressControllerEvaluationConditionsDetectedConditionType   = "EvaluationConditionsDetected"

	routerDefaultHeaderBufferSize           = 32768
	routerDefaultHeaderBufferMaxRewriteSize = 8192
	routerDefaultHostNetworkHTTPPort        = 80
	routerDefaultHostNetworkHTTPSPort       = 443
	routerDefaultHostNetworkStatsPort       = 1936
)

var (
	log = logf.Logger.WithName(controllerName)
	// tlsVersion13Ciphers is a list of TLS v1.3 cipher suites as specified by
	// https://www.openssl.org/docs/man1.1.1/man1/ciphers.html
	tlsVersion13Ciphers = sets.NewString(
		"TLS_AES_128_GCM_SHA256",
		"TLS_AES_256_GCM_SHA384",
		"TLS_CHACHA20_POLY1305_SHA256",
		"TLS_AES_128_CCM_SHA256",
		"TLS_AES_128_CCM_8_SHA256",
	)
)

// New creates the ingress controller from configuration. This is the controller
// that handles all the logic for implementing ingress based on
// IngressController resources.
//
// The controller will be pre-configured to watch for IngressController resources
// in the manager namespace.
func New(mgr manager.Manager, config Config) (controller.Controller, error) {
	reconciler := &reconciler{
		config:   config,
		client:   mgr.GetClient(),
		cache:    mgr.GetCache(),
		recorder: mgr.GetEventRecorderFor(controllerName),
	}
	c, err := controller.New(controllerName, mgr, controller.Options{Reconciler: reconciler})
	if err != nil {
		return nil, err
	}
	if err := c.Watch(&source.Kind{Type: &operatorv1.IngressController{}}, &handler.EnqueueRequestForObject{}); err != nil {
		return nil, err
	}
	if err := c.Watch(&source.Kind{Type: &appsv1.Deployment{}}, enqueueRequestForOwningIngressController(config.Namespace)); err != nil {
		return nil, err
	}
	if err := c.Watch(&source.Kind{Type: &corev1.Service{}}, enqueueRequestForOwningIngressController(config.Namespace)); err != nil {
		return nil, err
	}
	// Add watch for deleted pods specifically for ensuring ingress deletion.
	if err := c.Watch(&source.Kind{Type: &corev1.Pod{}}, enqueueRequestForOwningIngressController(config.Namespace), predicate.Funcs{
		CreateFunc:  func(e event.CreateEvent) bool { return false },
		DeleteFunc:  func(e event.DeleteEvent) bool { return true },
		UpdateFunc:  func(e event.UpdateEvent) bool { return false },
		GenericFunc: func(e event.GenericEvent) bool { return false },
	}); err != nil {
		return nil, err
	}
	// add watch for changes in DNS config
	if err := c.Watch(&source.Kind{Type: &configv1.DNS{}}, handler.EnqueueRequestsFromMapFunc(reconciler.ingressConfigToIngressController)); err != nil {
		return nil, err
	}
	if err := c.Watch(&source.Kind{Type: &iov1.DNSRecord{}}, &handler.EnqueueRequestForOwner{OwnerType: &operatorv1.IngressController{}}); err != nil {
		return nil, err
	}
	if err := c.Watch(&source.Kind{Type: &configv1.Ingress{}}, handler.EnqueueRequestsFromMapFunc(reconciler.ingressConfigToIngressController)); err != nil {
		return nil, err
	}
	return c, nil
}

func (r *reconciler) ingressConfigToIngressController(o client.Object) []reconcile.Request {
	var requests []reconcile.Request
	controllers := &operatorv1.IngressControllerList{}
	if err := r.cache.List(context.Background(), controllers, client.InNamespace(r.config.Namespace)); err != nil {
		log.Error(err, "failed to list ingresscontrollers for ingress", "related", o.GetSelfLink())
		return requests
	}
	for _, ic := range controllers.Items {
		log.Info("queueing ingresscontroller", "name", ic.Name, "related", o.GetSelfLink())
		request := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: ic.Namespace,
				Name:      ic.Name,
			},
		}
		requests = append(requests, request)
	}
	return requests
}

func enqueueRequestForOwningIngressController(namespace string) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(
		func(a client.Object) []reconcile.Request {
			labels := a.GetLabels()
			if ingressName, ok := labels[manifests.OwningIngressControllerLabel]; ok {
				log.Info("queueing ingress", "name", ingressName, "related", a.GetSelfLink())
				return []reconcile.Request{
					{
						NamespacedName: types.NamespacedName{
							Namespace: namespace,
							Name:      ingressName,
						},
					},
				}
			} else if ingressName, ok := labels[operatorcontroller.ControllerDeploymentLabel]; ok {
				log.Info("queueing ingress", "name", ingressName, "related", a.GetSelfLink())
				return []reconcile.Request{
					{
						NamespacedName: types.NamespacedName{
							Namespace: namespace,
							Name:      ingressName,
						},
					},
				}
			} else {
				return []reconcile.Request{}
			}
		})
}

// Config holds all the things necessary for the controller to run.
type Config struct {
	Namespace              string
	IngressControllerImage string
}

// reconciler handles the actual ingress reconciliation logic in response to
// events.
type reconciler struct {
	config   Config
	client   client.Client
	cache    cache.Cache
	recorder record.EventRecorder
}

// admissionRejection is an error type for ingresscontroller admission
// rejections.
type admissionRejection struct {
	// Reason describes why the ingresscontroller was rejected.
	Reason string
}

// Error returns the reason or reasons why an ingresscontroller was rejected.
func (e *admissionRejection) Error() string {
	return e.Reason
}

// Reconcile expects request to refer to a ingresscontroller in the operator
// namespace, and will do all the work to ensure the ingresscontroller is in the
// desired state.
func (r *reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log.Info("reconciling", "request", request)

	// Only proceed if we can get the ingresscontroller's state.
	ingress := &operatorv1.IngressController{}
	if err := r.client.Get(ctx, request.NamespacedName, ingress); err != nil {
		if kerrors.IsNotFound(err) {
			// This means the ingress was already deleted/finalized and there are
			// stale queue entries (or something edge triggering from a related
			// resource that got deleted async).
			log.Info("ingresscontroller not found; reconciliation will be skipped", "request", request)
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("failed to get ingresscontroller %q: %v", request, err)
	}

	// If the ingresscontroller is deleted, handle that and return early.
	if ingress.DeletionTimestamp != nil {
		if err := r.ensureIngressDeleted(ingress); err != nil {
			switch e := err.(type) {
			case retryable.Error:
				log.Error(e, "got retryable error; requeueing", "after", e.After())
				return reconcile.Result{RequeueAfter: e.After()}, nil
			default:
				return reconcile.Result{}, fmt.Errorf("failed to ensure ingress deletion: %v", err)
			}
		}
		log.Info("ingresscontroller was successfully deleted", "ingresscontroller", ingress)
		return reconcile.Result{}, nil
	}

	// Only proceed if we can collect cluster config.
	apiConfig := &configv1.APIServer{}
	if err := r.client.Get(ctx, types.NamespacedName{Name: "cluster"}, apiConfig); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get apiserver 'cluster': %v", err)
	}
	dnsConfig := &configv1.DNS{}
	if err := r.client.Get(ctx, types.NamespacedName{Name: "cluster"}, dnsConfig); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get dns 'cluster': %v", err)
	}
	infraConfig := &configv1.Infrastructure{}
	if err := r.client.Get(ctx, types.NamespacedName{Name: "cluster"}, infraConfig); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get infrastructure 'cluster': %v", err)
	}
	ingressConfig := &configv1.Ingress{}
	if err := r.client.Get(ctx, types.NamespacedName{Name: "cluster"}, ingressConfig); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get ingress 'cluster': %v", err)
	}
	networkConfig := &configv1.Network{}
	if err := r.client.Get(ctx, types.NamespacedName{Name: "cluster"}, networkConfig); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get network 'cluster': %v", err)
	}
	platformStatus := infraConfig.Status.PlatformStatus
	if platformStatus == nil {
		return reconcile.Result{}, fmt.Errorf("failed to determine infrastructure platform status for ingresscontroller %s/%s: PlatformStatus is nil", ingress.Namespace, ingress.Name)
	}

	// Admit if necessary. Don't process until admission succeeds. If admission is
	// successful, immediately re-queue to refresh state.
	alreadyAdmitted := ingresscontroller.IsAdmitted(ingress)
	if !alreadyAdmitted || needsReadmission(ingress) {
		if err := r.admit(ingress, ingressConfig, platformStatus, dnsConfig, alreadyAdmitted); err != nil {
			switch err := err.(type) {
			case *admissionRejection:
				r.recorder.Event(ingress, "Warning", "Rejected", err.Reason)
				return reconcile.Result{}, nil
			default:
				return reconcile.Result{}, fmt.Errorf("failed to admit ingresscontroller: %v", err)
			}
		}
		r.recorder.Event(ingress, "Normal", "Admitted", "ingresscontroller passed validation")
		// Just re-queue for simplicity
		return reconcile.Result{Requeue: true}, nil
	}

	// During upgrades, an already admitted controller might require overriding
	// default dnsManagementPolicy to "Unmanaged" due to mismatch in its domain and
	// and the pre-configured base domain.
	// TODO: Remove this in 4.13
	if eps := ingress.Status.EndpointPublishingStrategy; eps != nil && eps.Type == operatorv1.LoadBalancerServiceStrategyType && eps.LoadBalancer != nil {

		domainMatchesBaseDomain := manageDNSForDomain(ingress.Status.Domain, platformStatus, dnsConfig)

		// Set dnsManagementPolicy based on current domain on the ingresscontroller
		// and base domain on dns config. This is needed to ensure the correct dnsManagementPolicy
		// is set on the default ingress controller since the status.domain is updated
		// in r.admit() and spec.domain is unset on the default ingress controller.
		if !domainMatchesBaseDomain && eps.LoadBalancer.DNSManagementPolicy != operatorv1.UnmanagedLoadBalancerDNS {
			ingress.Status.EndpointPublishingStrategy.LoadBalancer.DNSManagementPolicy = operatorv1.UnmanagedLoadBalancerDNS

			if err := r.client.Status().Update(context.TODO(), ingress); err != nil {
				return reconcile.Result{}, fmt.Errorf("failed to update status: %w", err)
			}
			log.Info("Updated ingresscontroller status: dnsManagementPolicy as Unmanaged", "ingresscontroller", ingress.Status)
			return reconcile.Result{Requeue: true}, nil
		}
	}

	// The ingresscontroller is safe to process, so ensure it.
	if err := r.ensureIngressController(ingress, dnsConfig, infraConfig, platformStatus, ingressConfig, apiConfig, networkConfig); err != nil {
		switch e := err.(type) {
		case retryable.Error:
			log.Error(e, "got retryable error; requeueing", "after", e.After())
			return reconcile.Result{RequeueAfter: e.After()}, nil
		default:
			return reconcile.Result{}, err
		}
	}
	return reconcile.Result{}, nil
}

// admit processes the given ingresscontroller by defaulting and validating its
// fields.  Returns an error value, which will have a non-nil value of type
// admissionRejection if the ingresscontroller was rejected, or a non-nil
// value of a different type if the ingresscontroller could not be processed.
func (r *reconciler) admit(current *operatorv1.IngressController, ingressConfig *configv1.Ingress, platformStatus *configv1.PlatformStatus, dnsConfig *configv1.DNS, alreadyAdmitted bool) error {
	updated := current.DeepCopy()

	setDefaultDomain(updated, ingressConfig)

	// To set default publishing strategy we need to verify if the domains match
	// so that we can set the appropriate dnsManagementPolicy. This can only be
	// done after status.domain has been updated in setDefaultDomain().
	domainMatchesBaseDomain := manageDNSForDomain(updated.Status.Domain, platformStatus, dnsConfig)
	setDefaultPublishingStrategy(updated, platformStatus, domainMatchesBaseDomain, ingressConfig, alreadyAdmitted)

	// The TLS security profile need not be defaulted.  If none is set, we
	// get the default from the APIServer config (which is assumed to be
	// valid).

	if err := r.validate(updated); err != nil {
		switch err := err.(type) {
		case *admissionRejection:
			updated.Status.Conditions = MergeConditions(updated.Status.Conditions, operatorv1.OperatorCondition{
				Type:    IngressControllerAdmittedConditionType,
				Status:  operatorv1.ConditionFalse,
				Reason:  "Invalid",
				Message: err.Reason,
			})
			updated.Status.ObservedGeneration = updated.Generation
			if !IngressStatusesEqual(current.Status, updated.Status) {
				if err := r.client.Status().Update(context.TODO(), updated); err != nil {
					return fmt.Errorf("failed to update status: %v", err)
				}
			}
		}
		return err
	}

	updated.Status.Conditions = MergeConditions(updated.Status.Conditions, operatorv1.OperatorCondition{
		Type:   IngressControllerAdmittedConditionType,
		Status: operatorv1.ConditionTrue,
		Reason: "Valid",
	})
	updated.Status.ObservedGeneration = updated.Generation

	if !domainMatchesBaseDomain {
		r.recorder.Eventf(updated, "Warning", "DomainNotMatching", fmt.Sprintf("Domain [%s] of ingresscontroller does not match the baseDomain [%s] of the cluster DNS config, so DNS management is not supported.", updated.Status.Domain, dnsConfig.Spec.BaseDomain))
	}

	if !IngressStatusesEqual(current.Status, updated.Status) {
		if err := r.client.Status().Update(context.TODO(), updated); err != nil {
			return fmt.Errorf("failed to update status: %v", err)
		}
	}
	return nil
}

// needsReadmission returns a Boolean value indicating whether the given
// ingresscontroller needs to be re-admitted.  Re-admission is necessary in
// order to revalidate mutable fields that are subject to admission checks.  The
// determination whether re-admission is needed is based on the
// ingresscontroller's current generation and the observed generation recorded
// in its status.
func needsReadmission(ic *operatorv1.IngressController) bool {
	if ic.Generation != ic.Status.ObservedGeneration {
		return true
	}
	return false
}

func setDefaultDomain(ic *operatorv1.IngressController, ingressConfig *configv1.Ingress) bool {
	var effectiveDomain string
	switch {
	case len(ic.Spec.Domain) > 0:
		effectiveDomain = ic.Spec.Domain
	default:
		effectiveDomain = ingressConfig.Spec.Domain
	}
	if len(ic.Status.Domain) == 0 {
		ic.Status.Domain = effectiveDomain
		return true
	}
	return false
}

func setDefaultPublishingStrategy(ic *operatorv1.IngressController, platformStatus *configv1.PlatformStatus, domainMatchesBaseDomain bool, ingressConfig *configv1.Ingress, alreadyAdmitted bool) bool {
	effectiveStrategy := ic.Spec.EndpointPublishingStrategy.DeepCopy()
	if effectiveStrategy == nil {
		var strategyType operatorv1.EndpointPublishingStrategyType
		switch platformStatus.Type {
		case configv1.AWSPlatformType, configv1.AzurePlatformType, configv1.GCPPlatformType, configv1.IBMCloudPlatformType, configv1.PowerVSPlatformType, configv1.AlibabaCloudPlatformType:
			strategyType = operatorv1.LoadBalancerServiceStrategyType
		case configv1.LibvirtPlatformType:
			strategyType = operatorv1.HostNetworkStrategyType
		default:
			strategyType = operatorv1.HostNetworkStrategyType
		}
		effectiveStrategy = &operatorv1.EndpointPublishingStrategy{
			Type: strategyType,
		}
	}
	switch effectiveStrategy.Type {
	case operatorv1.LoadBalancerServiceStrategyType:
		if effectiveStrategy.LoadBalancer == nil {
			effectiveStrategy.LoadBalancer = &operatorv1.LoadBalancerStrategy{
				DNSManagementPolicy: operatorv1.ManagedLoadBalancerDNS,
				Scope:               operatorv1.ExternalLoadBalancer,
			}
		}

		// Set dnsManagementPolicy based on current domain on the ingresscontroller
		// and base domain on dns config.
		if !domainMatchesBaseDomain {
			effectiveStrategy.LoadBalancer.DNSManagementPolicy = operatorv1.UnmanagedLoadBalancerDNS
		}

		// Set provider parameters based on the cluster ingress config.
		setDefaultProviderParameters(effectiveStrategy.LoadBalancer, ingressConfig, alreadyAdmitted)

	case operatorv1.NodePortServiceStrategyType:
		if effectiveStrategy.NodePort == nil {
			effectiveStrategy.NodePort = &operatorv1.NodePortStrategy{}
		}
		if effectiveStrategy.NodePort.Protocol == operatorv1.DefaultProtocol {
			effectiveStrategy.NodePort.Protocol = operatorv1.TCPProtocol
		}
	case operatorv1.HostNetworkStrategyType:
		if effectiveStrategy.HostNetwork == nil {
			effectiveStrategy.HostNetwork = &operatorv1.HostNetworkStrategy{}
		}
		// explicitly set the default ports if some of them are omitted
		if effectiveStrategy.HostNetwork.HTTPPort == 0 {
			effectiveStrategy.HostNetwork.HTTPPort = routerDefaultHostNetworkHTTPPort
		}
		if effectiveStrategy.HostNetwork.HTTPSPort == 0 {
			effectiveStrategy.HostNetwork.HTTPSPort = routerDefaultHostNetworkHTTPSPort
		}
		if effectiveStrategy.HostNetwork.StatsPort == 0 {
			effectiveStrategy.HostNetwork.StatsPort = routerDefaultHostNetworkStatsPort
		}

		if effectiveStrategy.HostNetwork.Protocol == operatorv1.DefaultProtocol {
			effectiveStrategy.HostNetwork.Protocol = operatorv1.TCPProtocol
		}
	case operatorv1.PrivateStrategyType:
		if effectiveStrategy.Private == nil {
			effectiveStrategy.Private = &operatorv1.PrivateStrategy{}
		}
		if effectiveStrategy.Private.Protocol == operatorv1.DefaultProtocol {
			effectiveStrategy.Private.Protocol = operatorv1.TCPProtocol
		}
	}
	if ic.Status.EndpointPublishingStrategy == nil {
		ic.Status.EndpointPublishingStrategy = effectiveStrategy
		return true
	}

	// Detect changes to endpoint publishing strategy parameters that the
	// operator can safely update.
	switch effectiveStrategy.Type {
	case operatorv1.LoadBalancerServiceStrategyType:
		// Update if LB provider parameters changed.
		statusLB := ic.Status.EndpointPublishingStrategy.LoadBalancer
		specLB := effectiveStrategy.LoadBalancer
		if specLB != nil && statusLB != nil {
			changed := false

			// Detect changes to LB scope.
			if specLB.Scope != statusLB.Scope {
				ic.Status.EndpointPublishingStrategy.LoadBalancer.Scope = effectiveStrategy.LoadBalancer.Scope
				changed = true
			}

			// Detect changes to LB dnsManagementPolicy
			if specLB.DNSManagementPolicy != statusLB.DNSManagementPolicy {
				ic.Status.EndpointPublishingStrategy.LoadBalancer.DNSManagementPolicy = effectiveStrategy.LoadBalancer.DNSManagementPolicy
				changed = true
			}

			// Detect changes to provider-specific parameters.
			// Currently the only platforms with configurable
			// provider-specific parameters are AWS and GCP.
			var lbType operatorv1.LoadBalancerProviderType
			if specLB.ProviderParameters != nil {
				lbType = specLB.ProviderParameters.Type
			}
			switch lbType {
			case operatorv1.AWSLoadBalancerProvider:
				if statusLB.ProviderParameters == nil {
					statusLB.ProviderParameters = &operatorv1.ProviderLoadBalancerParameters{}
				}
				if len(statusLB.ProviderParameters.Type) == 0 {
					statusLB.ProviderParameters.Type = operatorv1.AWSLoadBalancerProvider
				}
				if statusLB.ProviderParameters.AWS == nil {
					statusLB.ProviderParameters.AWS = &operatorv1.AWSLoadBalancerParameters{}
				}
				if specLB.ProviderParameters.AWS.Type != statusLB.ProviderParameters.AWS.Type {
					statusLB.ProviderParameters.AWS.Type = specLB.ProviderParameters.AWS.Type
					changed = true
				}
				if statusLB.ProviderParameters.AWS.Type == operatorv1.AWSClassicLoadBalancer {
					if statusLB.ProviderParameters.AWS.ClassicLoadBalancerParameters == nil {
						statusLB.ProviderParameters.AWS.ClassicLoadBalancerParameters = &operatorv1.AWSClassicLoadBalancerParameters{}
					}
					// The only provider parameter that is
					// supported for AWS Classic ELBs is the
					// connection idle timeout.
					var specIdleTimeout metav1.Duration
					if specLB.ProviderParameters.AWS != nil && specLB.ProviderParameters.AWS.ClassicLoadBalancerParameters != nil {
						specIdleTimeout = specLB.ProviderParameters.AWS.ClassicLoadBalancerParameters.ConnectionIdleTimeout
					}
					statusIdleTimeout := statusLB.ProviderParameters.AWS.ClassicLoadBalancerParameters.ConnectionIdleTimeout
					if specIdleTimeout != statusIdleTimeout {
						var v metav1.Duration
						if specIdleTimeout.Duration > 0 {
							v = specIdleTimeout
						}
						statusLB.ProviderParameters.AWS.ClassicLoadBalancerParameters.ConnectionIdleTimeout = v
						changed = true
					}
				}
			case operatorv1.GCPLoadBalancerProvider:
				// The only provider parameter that is supported
				// for GCP is the ClientAccess parameter.
				var statusClientAccess operatorv1.GCPClientAccess
				specClientAccess := specLB.ProviderParameters.GCP.ClientAccess
				if statusLB.ProviderParameters != nil && statusLB.ProviderParameters.GCP != nil {
					statusClientAccess = statusLB.ProviderParameters.GCP.ClientAccess
				}
				if specClientAccess != statusClientAccess {
					if statusLB.ProviderParameters == nil {
						statusLB.ProviderParameters = &operatorv1.ProviderLoadBalancerParameters{}
					}
					if len(statusLB.ProviderParameters.Type) == 0 {
						statusLB.ProviderParameters.Type = operatorv1.GCPLoadBalancerProvider
					}
					if statusLB.ProviderParameters.GCP == nil {
						statusLB.ProviderParameters.GCP = &operatorv1.GCPLoadBalancerParameters{}
					}
					statusLB.ProviderParameters.GCP.ClientAccess = specClientAccess
					changed = true
				}
			}

			return changed
		}
	case operatorv1.NodePortServiceStrategyType:
		// Update if PROXY protocol is turned on or off.
		if ic.Status.EndpointPublishingStrategy.NodePort == nil {
			ic.Status.EndpointPublishingStrategy.NodePort = &operatorv1.NodePortStrategy{}
		}
		statusNP := ic.Status.EndpointPublishingStrategy.NodePort
		specNP := effectiveStrategy.NodePort
		if specNP != nil && specNP.Protocol != statusNP.Protocol {
			statusNP.Protocol = specNP.Protocol
			return true
		}
	case operatorv1.HostNetworkStrategyType:
		if ic.Status.EndpointPublishingStrategy.HostNetwork == nil {
			ic.Status.EndpointPublishingStrategy.HostNetwork = &operatorv1.HostNetworkStrategy{}
		}

		statusHN := ic.Status.EndpointPublishingStrategy.HostNetwork
		specHN := effectiveStrategy.HostNetwork

		var changed bool
		if specHN != nil {
			// Update if PROXY protocol is turned on or off.
			if specHN.Protocol != statusHN.Protocol {
				statusHN.Protocol = specHN.Protocol
				changed = true
			}

			// Update if ports have been changed.
			if specHN.HTTPPort != statusHN.HTTPPort {
				statusHN.HTTPPort = specHN.HTTPPort
				changed = true
			}
			if specHN.HTTPSPort != statusHN.HTTPSPort {
				statusHN.HTTPSPort = specHN.HTTPSPort
				changed = true
			}
			if specHN.StatsPort != statusHN.StatsPort {
				statusHN.StatsPort = specHN.StatsPort
				changed = true
			}
		}
		return changed
	case operatorv1.PrivateStrategyType:
		// Update if PROXY protocol is turned on or off.
		if ic.Status.EndpointPublishingStrategy.Private == nil {
			ic.Status.EndpointPublishingStrategy.Private = &operatorv1.PrivateStrategy{}
		}
		statusPrivate := ic.Status.EndpointPublishingStrategy.Private
		specPrivate := effectiveStrategy.Private
		if specPrivate != nil && specPrivate.Protocol != statusPrivate.Protocol {
			statusPrivate.Protocol = specPrivate.Protocol
			return true
		}
	}

	return false
}

// setDefaultProviderParameters mutates the given LoadBalancerStrategy by
// defaulting its ProviderParameters field based on the defaults in the provided
// ingress config object.
func setDefaultProviderParameters(lbs *operatorv1.LoadBalancerStrategy, ingressConfig *configv1.Ingress, alreadyAdmitted bool) {
	var provider operatorv1.LoadBalancerProviderType
	if lbs.ProviderParameters != nil {
		provider = lbs.ProviderParameters.Type
	}
	if len(provider) == 0 && !alreadyAdmitted {
		// Infer the LB type from the cluster ingress config, but only
		// if the ingresscontroller isn't already admitted.
		switch ingressConfig.Spec.LoadBalancer.Platform.Type {
		case configv1.AWSPlatformType:
			provider = operatorv1.AWSLoadBalancerProvider
		}
	}
	switch provider {
	case operatorv1.AWSLoadBalancerProvider:
		if lbs.ProviderParameters == nil {
			lbs.ProviderParameters = &operatorv1.ProviderLoadBalancerParameters{}
		}
		lbs.ProviderParameters.Type = provider
		defaultLBType := operatorv1.AWSClassicLoadBalancer
		if p := ingressConfig.Spec.LoadBalancer.Platform; !alreadyAdmitted && p.Type == configv1.AWSPlatformType && p.AWS != nil {
			if p.AWS.Type == configv1.NLB {
				defaultLBType = operatorv1.AWSNetworkLoadBalancer
			}
		}
		if lbs.ProviderParameters.AWS == nil {
			lbs.ProviderParameters.AWS = &operatorv1.AWSLoadBalancerParameters{}
		}
		if len(lbs.ProviderParameters.AWS.Type) == 0 {
			lbs.ProviderParameters.AWS.Type = defaultLBType
		}
		switch lbs.ProviderParameters.AWS.Type {
		case operatorv1.AWSClassicLoadBalancer:
			if lbs.ProviderParameters.AWS.ClassicLoadBalancerParameters == nil {
				lbs.ProviderParameters.AWS.ClassicLoadBalancerParameters = &operatorv1.AWSClassicLoadBalancerParameters{}
			}
		}
	case operatorv1.GCPLoadBalancerProvider:
		if lbs.ProviderParameters == nil {
			lbs.ProviderParameters = &operatorv1.ProviderLoadBalancerParameters{}
		}
		lbs.ProviderParameters.Type = provider
		if lbs.ProviderParameters.GCP == nil {
			lbs.ProviderParameters.GCP = &operatorv1.GCPLoadBalancerParameters{}
		}
	}
}

// tlsProfileSpecForIngressController returns a TLS profile spec based on either
// the profile specified by the given ingresscontroller, the profile specified
// by the APIServer config if the ingresscontroller does not specify one, or the
// "Intermediate" profile if neither the ingresscontroller nor the APIServer
// config specifies one.  Note that the return value must not be mutated by the
// caller; the caller must make a copy if it needs to mutate the value.
func tlsProfileSpecForIngressController(ic *operatorv1.IngressController, apiConfig *configv1.APIServer) *configv1.TLSProfileSpec {
	if hasTLSSecurityProfile(ic) {
		return tlsProfileSpecForSecurityProfile(ic.Spec.TLSSecurityProfile)
	}
	return tlsProfileSpecForSecurityProfile(apiConfig.Spec.TLSSecurityProfile)
}

// hasTLSSecurityProfile checks whether the given ingresscontroller specifies a
// TLS security profile.
func hasTLSSecurityProfile(ic *operatorv1.IngressController) bool {
	if ic.Spec.TLSSecurityProfile == nil {
		return false
	}
	if len(ic.Spec.TLSSecurityProfile.Type) == 0 {
		return false
	}
	return true
}

// tlsProfileSpecForSecurityProfile returns a TLS profile spec based on the
// provided security profile, or the "Intermediate" profile if an unknown
// security profile type is provided.  Note that the return value must not be
// mutated by the caller; the caller must make a copy if it needs to mutate the
// value.
func tlsProfileSpecForSecurityProfile(profile *configv1.TLSSecurityProfile) *configv1.TLSProfileSpec {
	if profile != nil {
		if profile.Type == configv1.TLSProfileCustomType {
			if profile.Custom != nil {
				return &profile.Custom.TLSProfileSpec
			}
			return &configv1.TLSProfileSpec{}
		} else if spec, ok := configv1.TLSProfiles[profile.Type]; ok {
			return spec
		}
	}
	return configv1.TLSProfiles[configv1.TLSProfileIntermediateType]
}

// validate attempts to perform validation of the given ingresscontroller and
// returns an error value, which will have a non-nil value of type
// admissionRejection if the ingresscontroller is invalid, or a non-nil value of
// a different type if validation could not be completed.
func (r *reconciler) validate(ic *operatorv1.IngressController) error {
	var errors []error

	ingresses := &operatorv1.IngressControllerList{}
	if err := r.cache.List(context.TODO(), ingresses, client.InNamespace(r.config.Namespace)); err != nil {
		return fmt.Errorf("failed to list ingresscontrollers: %v", err)
	}

	if err := validateDomain(ic); err != nil {
		errors = append(errors, err)
	}
	if err := validateDomainUniqueness(ic, ingresses.Items); err != nil {
		errors = append(errors, err)
	}
	if err := validateTLSSecurityProfile(ic); err != nil {
		errors = append(errors, err)
	}
	if err := validateHTTPHeaderBufferValues(ic); err != nil {
		errors = append(errors, err)
	}
	if err := validateClientTLS(ic); err != nil {
		errors = append(errors, err)
	}
	if err := utilerrors.NewAggregate(errors); err != nil {
		return &admissionRejection{err.Error()}
	}

	return nil
}

func validateDomain(ic *operatorv1.IngressController) error {
	if len(ic.Status.Domain) == 0 {
		return fmt.Errorf("domain is required")
	}
	return nil
}

// validateDomainUniqueness returns an error if the desired controller's domain
// conflicts with any other admitted controllers.
func validateDomainUniqueness(desired *operatorv1.IngressController, existing []operatorv1.IngressController) error {
	for i := range existing {
		current := existing[i]
		if !ingresscontroller.IsAdmitted(&current) {
			continue
		}
		if desired.UID != current.UID && desired.Status.Domain == current.Status.Domain {
			return fmt.Errorf("conflicts with: %s", current.Name)
		}
	}

	return nil
}

var (
	// validTLSVersions is all allowed values for TLSProtocolVersion.
	validTLSVersions = map[configv1.TLSProtocolVersion]struct{}{
		configv1.VersionTLS10: {},
		configv1.VersionTLS11: {},
		configv1.VersionTLS12: {},
		configv1.VersionTLS13: {},
	}

	// isValidCipher is a regexp for strings that look like cipher names.
	isValidCipher = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_+-]+$`).MatchString
)

// validateTLSSecurityProfile validates the given ingresscontroller's TLS
// security profile, if it specifies one.
func validateTLSSecurityProfile(ic *operatorv1.IngressController) error {
	if !hasTLSSecurityProfile(ic) {
		return nil
	}

	if ic.Spec.TLSSecurityProfile.Type != configv1.TLSProfileCustomType {
		return nil
	}

	spec := ic.Spec.TLSSecurityProfile.Custom
	if spec == nil {
		return fmt.Errorf("security profile is not defined")
	}

	var errs []error

	if len(spec.Ciphers) == 0 {
		errs = append(errs, fmt.Errorf("security profile has an empty ciphers list"))
	} else {
		invalidCiphers := []string{}
		for _, cipher := range spec.Ciphers {
			if !isValidCipher(strings.TrimPrefix(cipher, "!")) {
				invalidCiphers = append(invalidCiphers, cipher)
			}
		}
		if len(invalidCiphers) != 0 {
			errs = append(errs, fmt.Errorf("security profile has invalid ciphers: %s", strings.Join(invalidCiphers, ", ")))
		}
		switch spec.MinTLSVersion {
		case configv1.VersionTLS10, configv1.VersionTLS11, configv1.VersionTLS12:
			if tlsVersion13Ciphers.HasAll(spec.Ciphers...) {
				errs = append(errs, fmt.Errorf("security profile specifies minTLSVersion: %s and contains only TLSv1.3 cipher suites", spec.MinTLSVersion))
			}
		case configv1.VersionTLS13:
			if !tlsVersion13Ciphers.HasAny(spec.Ciphers...) {
				errs = append(errs, fmt.Errorf("security profile specifies minTLSVersion: %s and contains no TLSv1.3 cipher suites", spec.MinTLSVersion))
			}
		}
	}

	if _, ok := validTLSVersions[spec.MinTLSVersion]; !ok {
		errs = append(errs, fmt.Errorf("security profile has invalid minimum security protocol version: %q", spec.MinTLSVersion))
	}

	return utilerrors.NewAggregate(errs)
}

// validateHTTPHeaderBufferValues validates the given ingresscontroller's header buffer
// size configuration, if it specifies one.
func validateHTTPHeaderBufferValues(ic *operatorv1.IngressController) error {
	bufSize := int(ic.Spec.TuningOptions.HeaderBufferBytes)
	maxRewrite := int(ic.Spec.TuningOptions.HeaderBufferMaxRewriteBytes)

	if bufSize == 0 && maxRewrite == 0 {
		return nil
	}

	// HeaderBufferBytes and HeaderBufferMaxRewriteBytes are both
	// optional fields. Substitute the default values used by the
	// router when either field is empty so that we can ensure that
	// tune.maxrewrite will never wind up being greater than tune.bufsize
	// (which would break router reloads).
	if bufSize == 0 {
		bufSize = routerDefaultHeaderBufferSize
	}
	if maxRewrite == 0 {
		maxRewrite = routerDefaultHeaderBufferMaxRewriteSize
	}

	if bufSize <= maxRewrite {
		return fmt.Errorf("invalid spec.httpHeaderBuffer: headerBufferBytes (%d) "+
			"must be larger than headerBufferMaxRewriteBytes (%d)", bufSize, maxRewrite)
	}

	return nil
}

// validateClientTLS validates the given ingresscontroller's client TLS
// configuration.
func validateClientTLS(ic *operatorv1.IngressController) error {
	errs := []error{}
	for i, pattern := range ic.Spec.ClientTLS.AllowedSubjectPatterns {
		if _, err := syntax.Parse(pattern, syntax.Perl); err != nil {
			errs = append(errs, errors.Wrapf(err, "failed to parse spec.clientTLS.allowedSubjectPatterns[%d]", i))
		}
	}
	return utilerrors.NewAggregate(errs)
}

// ensureIngressDeleted tries to delete ingress, and if successful, will remove
// the finalizer.
func (r *reconciler) ensureIngressDeleted(ingress *operatorv1.IngressController) error {
	errs := []error{}

	// Delete the wildcard DNS record, and block ingresscontroller finalization
	// until the dnsrecord has been finalized.
	if err := r.deleteWildcardDNSRecord(ingress); err != nil {
		errs = append(errs, fmt.Errorf("failed to delete wildcard dnsrecord for ingress %s/%s: %v", ingress.Namespace, ingress.Name, err))
	}
	haveRec, _, err := r.currentWildcardDNSRecord(ingress)
	switch {
	case err != nil:
		errs = append(errs, fmt.Errorf("failed to get current wildcard dnsrecord for ingress %s/%s: %v", ingress.Namespace, ingress.Name, err))
	case haveRec:
		errs = append(errs, fmt.Errorf("wildcard dnsrecord exists for ingress %s/%s", ingress.Namespace, ingress.Name))
	default:
		// The router deployment manages the load-balancer service
		// which is used to find the hosted zone id. Delete the deployment
		// only when the dnsrecord does not exist.
		if err := r.ensureRouterDeleted(ingress); err != nil {
			errs = append(errs, fmt.Errorf("failed to delete deployment for ingress %s/%s: %v", ingress.Namespace, ingress.Name, err))
		}
		if haveDepl, _, err := r.currentRouterDeployment(ingress); err != nil {
			errs = append(errs, fmt.Errorf("failed to get deployment for ingress %s/%s: %v", ingress.Namespace, ingress.Name, err))
		} else if haveDepl {
			errs = append(errs, fmt.Errorf("deployment still exists for ingress %s/%s", ingress.Namespace, ingress.Name))
		} else {
			// Wait for all the router pods to be deleted. This is important because the router deployment
			// gets deleted a handful of milliseconds before the router pods process the graceful shutdown. This causes
			// a race condition in which we clear route status, then the router pod will race to re-admit the status in
			// these few milliseconds before it initiates the graceful shutdown. The only way to avoid is to wait
			// until all router pods are deleted.
			if allDeleted, err := r.allRouterPodsDeleted(ingress); err != nil {
				errs = append(errs, err)
			} else if allDeleted {
				// Deployment has been deleted and there are no more pods left.
				// Clear all routes status for this ingress controller.
				statusErrs := r.clearAllRoutesStatusForIngressController(ingress.ObjectMeta.Name)
				errs = append(errs, statusErrs...)
			} else {
				errs = append(errs, retryable.New(fmt.Errorf("not all router pods have been deleted for %s/%s", ingress.Namespace, ingress.Name), 15*time.Second))
			}
		}
	}

	// Delete the metrics related to the ingresscontroller
	DeleteIngressControllerConditionsMetric(ingress)
	DeleteActiveNLBMetrics(ingress)

	// Delete the RoutesPerShard metric label corresponding to the Ingress Controller.
	routemetrics.DeleteRouteMetricsControllerRoutesPerShardMetric(ingress.Name)

	if len(errs) == 0 {
		// Remove the ingresscontroller finalizer.
		if slice.ContainsString(ingress.Finalizers, manifests.IngressControllerFinalizer) {
			updated := ingress.DeepCopy()
			updated.Finalizers = slice.RemoveString(updated.Finalizers, manifests.IngressControllerFinalizer)
			if err := r.client.Update(context.TODO(), updated); err != nil {
				errs = append(errs, fmt.Errorf("failed to remove finalizer from ingresscontroller %s: %v", ingress.Name, err))
			}
		}
	}
	return retryable.NewMaybeRetryableAggregate(errs)
}

// ensureIngressController ensures all necessary router resources exist for a
// given ingresscontroller.  Any error values are collected into either a
// retryable.Error value, if any of the error values are retryable, or else an
// Aggregate error value.
func (r *reconciler) ensureIngressController(ci *operatorv1.IngressController, dnsConfig *configv1.DNS, infraConfig *configv1.Infrastructure, platformStatus *configv1.PlatformStatus, ingressConfig *configv1.Ingress, apiConfig *configv1.APIServer, networkConfig *configv1.Network) error {
	// Before doing anything at all with the controller, ensure it has a finalizer
	// so we can clean up later.
	if !slice.ContainsString(ci.Finalizers, manifests.IngressControllerFinalizer) {
		updated := ci.DeepCopy()
		updated.Finalizers = append(updated.Finalizers, manifests.IngressControllerFinalizer)
		if err := r.client.Update(context.TODO(), updated); err != nil {
			return fmt.Errorf("failed to update finalizers: %v", err)
		}
		if err := r.client.Get(context.TODO(), types.NamespacedName{Namespace: updated.Namespace, Name: updated.Name}, updated); err != nil {
			return fmt.Errorf("failed to get ingresscontroller: %v", err)
		}
		ci = updated
	}

	if _, _, err := r.ensureClusterRole(); err != nil {
		return fmt.Errorf("failed to ensure cluster role: %v", err)
	}

	if _, _, err := r.ensureRouterNamespace(); err != nil {
		return fmt.Errorf("failed to ensure namespace: %v", err)
	}

	if err := r.ensureRouterServiceAccount(); err != nil {
		return fmt.Errorf("failed to ensure service account: %v", err)
	}

	if err := r.ensureRouterClusterRoleBinding(); err != nil {
		return fmt.Errorf("failed to ensure cluster role binding: %v", err)
	}

	var errs []error
	if _, _, err := r.ensureServiceCAConfigMap(); err != nil {
		// Even if we were unable to create the configmap at this time,
		// it is still safe try to create the deployment, as it
		// specifies that the volume mount is non-optional, meaning the
		// deployment will not start until the configmap exists.
		errs = append(errs, err)
	}

	var haveClientCAConfigmap bool
	clientCAConfigmap := &corev1.ConfigMap{}
	if len(ci.Spec.ClientTLS.ClientCA.Name) != 0 {
		name := operatorcontroller.ClientCAConfigMapName(ci)
		if err := r.cache.Get(context.TODO(), name, clientCAConfigmap); err != nil {
			errs = append(errs, fmt.Errorf("failed to get client CA configmap: %w", err))
			return utilerrors.NewAggregate(errs)
		}
		haveClientCAConfigmap = true
	}

	haveDepl, deployment, err := r.ensureRouterDeployment(ci, infraConfig, ingressConfig, apiConfig, networkConfig, haveClientCAConfigmap, clientCAConfigmap, platformStatus)
	if err != nil {
		errs = append(errs, fmt.Errorf("failed to ensure deployment: %v", err))
		return utilerrors.NewAggregate(errs)
	} else if !haveDepl {
		errs = append(errs, fmt.Errorf("failed to get router deployment %s/%s", ci.Namespace, ci.Name))
		return utilerrors.NewAggregate(errs)
	}

	trueVar := true
	deploymentRef := metav1.OwnerReference{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
		Name:       deployment.Name,
		UID:        deployment.UID,
		Controller: &trueVar,
	}

	var lbService *corev1.Service
	var wildcardRecord *iov1.DNSRecord
	if haveLB, lb, err := r.ensureLoadBalancerService(ci, deploymentRef, platformStatus); err != nil {
		errs = append(errs, fmt.Errorf("failed to ensure load balancer service for %s: %v", ci.Name, err))
	} else {
		lbService = lb
		if _, record, err := r.ensureWildcardDNSRecord(ci, lbService, haveLB); err != nil {
			errs = append(errs, fmt.Errorf("failed to ensure wildcard dnsrecord for %s: %v", ci.Name, err))
		} else {
			wildcardRecord = record
		}
	}

	if _, _, err := r.ensureNodePortService(ci, deploymentRef); err != nil {
		errs = append(errs, err)
	}

	if internalSvc, err := r.ensureInternalIngressControllerService(ci, deploymentRef); err != nil {
		errs = append(errs, fmt.Errorf("failed to create internal router service for ingresscontroller %s: %v", ci.Name, err))
	} else if err := r.ensureMetricsIntegration(ci, internalSvc, deploymentRef); err != nil {
		errs = append(errs, fmt.Errorf("failed to integrate metrics with openshift-monitoring for ingresscontroller %s: %v", ci.Name, err))
	}

	if _, _, err := r.ensureRsyslogConfigMap(ci, deploymentRef); err != nil {
		errs = append(errs, err)
	}

	if _, _, err := r.ensureRouterPodDisruptionBudget(ci, deploymentRef); err != nil {
		errs = append(errs, err)
	}

	operandEvents := &corev1.EventList{}
	if err := r.cache.List(context.TODO(), operandEvents, client.InNamespace(operatorcontroller.DefaultOperandNamespace)); err != nil {
		errs = append(errs, fmt.Errorf("failed to list events in namespace %q: %v", operatorcontroller.DefaultOperandNamespace, err))
	}

	pods := &corev1.PodList{}
	if err := r.cache.List(context.TODO(), pods, client.InNamespace(operatorcontroller.DefaultOperandNamespace)); err != nil {
		errs = append(errs, fmt.Errorf("failed to list pods in namespace %q: %v", operatorcontroller.DefaultOperatorNamespace, err))
	}

	syncStatusErr, updated := r.syncIngressControllerStatus(ci, deployment, deploymentRef, pods.Items, lbService, operandEvents.Items, wildcardRecord, dnsConfig, platformStatus)
	errs = append(errs, syncStatusErr)

	// If syncIngressControllerStatus updated our ingress status, it's important we query for that new object.
	// If we don't, then the next function syncRouteStatus would always fail because it has a stale ingress object.
	if updated {
		updatedIc := &operatorv1.IngressController{}
		if err := r.client.Get(context.TODO(), types.NamespacedName{Namespace: ci.Namespace, Name: ci.Name}, updatedIc); err != nil {
			errs = append(errs, fmt.Errorf("failed to get ingresscontroller: %w", err))
		}
		ci = updatedIc
	}

	SetIngressControllerNLBMetric(ci)

	errs = append(errs, r.syncRouteStatus(ci)...)

	return retryable.NewMaybeRetryableAggregate(errs)
}

// IsStatusDomainSet checks whether status.domain of ingress is set.
func IsStatusDomainSet(ingress *operatorv1.IngressController) bool {
	if len(ingress.Status.Domain) == 0 {
		return false
	}
	return true
}

// IsProxyProtocolNeeded checks whether proxy protocol is needed based
// upon the given ic and platform.
func IsProxyProtocolNeeded(ic *operatorv1.IngressController, platform *configv1.PlatformStatus) (bool, error) {
	if platform == nil {
		return false, fmt.Errorf("platform status is missing; failed to determine if proxy protocol is needed for %s/%s",
			ic.Namespace, ic.Name)
	}
	switch ic.Status.EndpointPublishingStrategy.Type {
	case operatorv1.LoadBalancerServiceStrategyType:
		// For now, check if we are on AWS. This can really be done for for any external
		// [cloud] LBs that support the proxy protocol.
		if platform.Type == configv1.AWSPlatformType {
			if ic.Status.EndpointPublishingStrategy.LoadBalancer == nil ||
				ic.Status.EndpointPublishingStrategy.LoadBalancer.ProviderParameters == nil ||
				ic.Status.EndpointPublishingStrategy.LoadBalancer.ProviderParameters.AWS == nil ||
				ic.Status.EndpointPublishingStrategy.LoadBalancer.ProviderParameters.Type == operatorv1.AWSLoadBalancerProvider &&
					ic.Status.EndpointPublishingStrategy.LoadBalancer.ProviderParameters.AWS.Type == operatorv1.AWSClassicLoadBalancer {
				return true, nil
			}
		}
	case operatorv1.HostNetworkStrategyType:
		if ic.Status.EndpointPublishingStrategy.HostNetwork != nil {
			return ic.Status.EndpointPublishingStrategy.HostNetwork.Protocol == operatorv1.ProxyProtocol, nil
		}
	case operatorv1.NodePortServiceStrategyType:
		if ic.Status.EndpointPublishingStrategy.NodePort != nil {
			return ic.Status.EndpointPublishingStrategy.NodePort.Protocol == operatorv1.ProxyProtocol, nil
		}
	case operatorv1.PrivateStrategyType:
		if ic.Status.EndpointPublishingStrategy.Private != nil {
			return ic.Status.EndpointPublishingStrategy.Private.Protocol == operatorv1.ProxyProtocol, nil
		}
	}
	return false, nil
}

// allRouterPodsDeleted determines if all the router pods for a given ingress controller are deleted.
func (r *reconciler) allRouterPodsDeleted(ingress *operatorv1.IngressController) (bool, error) {
	// List all pods that are owned by the ingress controller.
	podList := &corev1.PodList{}
	labels := map[string]string{
		operatorcontroller.ControllerDeploymentLabel: ingress.Name,
	}
	if err := r.client.List(context.TODO(), podList, client.InNamespace(operatorcontroller.DefaultOperandNamespace), client.MatchingLabels(labels)); err != nil {
		return false, fmt.Errorf("failed to list all pods owned by %s: %w", ingress.Name, err)
	}
	// If any pods exist, return false since they haven't all been deleted.
	if len(podList.Items) > 0 {
		return false, nil
	}

	return true, nil
}
