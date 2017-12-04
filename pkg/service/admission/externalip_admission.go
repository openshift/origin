package admission

import (
	"io"
	"net"
	"strings"

	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
	admission "k8s.io/apiserver/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/apis/core"
)

const ExternalIPPluginName = "ExternalIPRanger"

func RegisterExternalIP(plugins *admission.Plugins) {
	plugins.Register("ExternalIPRanger",
		func(config io.Reader) (admission.Interface, error) {
			return NewExternalIPRanger(nil, nil, false), nil
		})
}

type externalIPRanger struct {
	*admission.Handler
	reject         []*net.IPNet
	admit          []*net.IPNet
	allowIngressIP bool
}

var _ admission.Interface = &externalIPRanger{}

// ParseRejectAdmitCIDRRules calculates a blacklist and whitelist from a list of string CIDR rules (treating
// a leading ! as a negation). Returns an error if any rule is invalid.
func ParseRejectAdmitCIDRRules(rules []string) (reject, admit []*net.IPNet, err error) {
	for _, s := range rules {
		negate := false
		if strings.HasPrefix(s, "!") {
			negate = true
			s = s[1:]
		}
		_, cidr, err := net.ParseCIDR(s)
		if err != nil {
			return nil, nil, err
		}
		if negate {
			reject = append(reject, cidr)
		} else {
			admit = append(admit, cidr)
		}
	}
	return reject, admit, nil
}

// NewConstraint creates a new SCC constraint admission plugin.
func NewExternalIPRanger(reject, admit []*net.IPNet, allowIngressIP bool) *externalIPRanger {
	return &externalIPRanger{
		Handler:        admission.NewHandler(admission.Create, admission.Update),
		reject:         reject,
		admit:          admit,
		allowIngressIP: allowIngressIP,
	}
}

// NetworkSlice is a helper for checking whether an IP is contained in a range
// of networks.
type NetworkSlice []*net.IPNet

func (s NetworkSlice) Contains(ip net.IP) bool {
	for _, cidr := range s {
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}

// Admit determines if the service should be admitted based on the configured network CIDR.
func (r *externalIPRanger) Admit(a admission.Attributes) error {
	if a.GetResource().GroupResource() != kapi.Resource("services") {
		return nil
	}

	svc, ok := a.GetObject().(*kapi.Service)
	// if we can't convert then we don't handle this object so just return
	if !ok {
		return nil
	}

	// Determine if an ingress ip address should be allowed as an
	// external ip by checking the loadbalancer status of the previous
	// object state. Only updates need to be validated against the
	// ingress ip since the loadbalancer status cannot be set on
	// create.
	ingressIP := ""
	retrieveIngressIP := a.GetOperation() == admission.Update &&
		r.allowIngressIP && svc.Spec.Type == kapi.ServiceTypeLoadBalancer
	if retrieveIngressIP {
		old, ok := a.GetOldObject().(*kapi.Service)
		ipPresent := ok && old != nil && len(old.Status.LoadBalancer.Ingress) > 0
		if ipPresent {
			ingressIP = old.Status.LoadBalancer.Ingress[0].IP
		}
	}

	var errs field.ErrorList
	switch {
	// administrator disabled externalIPs
	case len(svc.Spec.ExternalIPs) > 0 && len(r.admit) == 0:
		onlyIngressIP := len(svc.Spec.ExternalIPs) == 1 && svc.Spec.ExternalIPs[0] == ingressIP
		if !onlyIngressIP {
			errs = append(errs, field.Forbidden(field.NewPath("spec", "externalIPs"), "externalIPs have been disabled"))
		}
	// administrator has limited the range
	case len(svc.Spec.ExternalIPs) > 0 && len(r.admit) > 0:
		for i, s := range svc.Spec.ExternalIPs {
			ip := net.ParseIP(s)
			if ip == nil {
				errs = append(errs, field.Forbidden(field.NewPath("spec", "externalIPs").Index(i), "externalIPs must be a valid address"))
				continue
			}
			notIngressIP := s != ingressIP
			if (NetworkSlice(r.reject).Contains(ip) || !NetworkSlice(r.admit).Contains(ip)) && notIngressIP {
				errs = append(errs, field.Forbidden(field.NewPath("spec", "externalIPs").Index(i), "externalIP is not allowed"))
				continue
			}
		}
	}
	if len(errs) > 0 {
		return apierrs.NewInvalid(a.GetKind().GroupKind(), a.GetName(), errs)
	}
	return nil
}
