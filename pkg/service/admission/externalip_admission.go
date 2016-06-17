package admission

import (
	"io"
	"net"
	"strings"

	kadmission "k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	apierrs "k8s.io/kubernetes/pkg/api/errors"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/util/validation/field"
)

const ExternalIPPluginName = "ExternalIPRanger"

func init() {
	kadmission.RegisterPlugin("ExternalIPRanger", func(client clientset.Interface, config io.Reader) (kadmission.Interface, error) {
		return NewExternalIPRanger(nil, nil), nil
	})
}

type externalIPRanger struct {
	*kadmission.Handler
	reject []*net.IPNet
	admit  []*net.IPNet
}

var _ kadmission.Interface = &externalIPRanger{}

// ParseCIDRRules calculates a blacklist and whitelist from a list of string CIDR rules (treating
// a leading ! as a negation). Returns an error if any rule is invalid.
func ParseCIDRRules(rules []string) (reject, admit []*net.IPNet, err error) {
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
func NewExternalIPRanger(reject, admit []*net.IPNet) *externalIPRanger {
	return &externalIPRanger{
		Handler: kadmission.NewHandler(kadmission.Create, kadmission.Update),
		reject:  reject,
		admit:   admit,
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
func (r *externalIPRanger) Admit(a kadmission.Attributes) error {
	if a.GetResource().GroupResource() != kapi.Resource("services") {
		return nil
	}

	svc, ok := a.GetObject().(*kapi.Service)
	// if we can't convert then we don't handle this object so just return
	if !ok {
		return nil
	}

	var errs field.ErrorList
	switch {
	// administrator disabled externalIPs
	case len(svc.Spec.ExternalIPs) > 0 && len(r.admit) == 0:
		errs = append(errs, field.Forbidden(field.NewPath("spec", "externalIPs"), "externalIPs have been disabled"))
	// administrator has limited the range
	case len(svc.Spec.ExternalIPs) > 0 && len(r.admit) > 0:
		for i, s := range svc.Spec.ExternalIPs {
			ip := net.ParseIP(s)
			if ip == nil {
				errs = append(errs, field.Forbidden(field.NewPath("spec", "externalIPs").Index(i), "externalIPs must be a valid address"))
				continue
			}
			if NetworkSlice(r.reject).Contains(ip) || !NetworkSlice(r.admit).Contains(ip) {
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
