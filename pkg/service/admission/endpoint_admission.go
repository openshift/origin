package admission

import (
	"fmt"
	"io"
	"net"
	"reflect"

	"github.com/openshift/origin/pkg/client"
	oadmission "github.com/openshift/origin/pkg/cmd/server/admission"

	admission "k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	kapi "k8s.io/kubernetes/pkg/api"
)

const RestrictedEndpointsPluginName = "openshift.io/RestrictedEndpointsAdmission"

func RegisterRestrictedEndpoints(plugins *admission.Plugins) {
	plugins.Register(RestrictedEndpointsPluginName,
		func(config io.Reader) (admission.Interface, error) {
			return NewRestrictedEndpointsAdmission(nil), nil
		})
}

type restrictedEndpointsAdmission struct {
	*admission.Handler

	client             client.Interface
	authorizer         authorizer.Authorizer
	restrictedNetworks []*net.IPNet
}

var _ = oadmission.WantsAuthorizer(&restrictedEndpointsAdmission{})

// ParseSimpleCIDRRules parses a list of CIDR strings
func ParseSimpleCIDRRules(rules []string) (networks []*net.IPNet, err error) {
	for _, s := range rules {
		_, cidr, err := net.ParseCIDR(s)
		if err != nil {
			return nil, err
		}
		networks = append(networks, cidr)
	}
	return networks, nil
}

// NewRestrictedEndpointsAdmission creates a new endpoints admission plugin.
func NewRestrictedEndpointsAdmission(restrictedNetworks []*net.IPNet) *restrictedEndpointsAdmission {
	return &restrictedEndpointsAdmission{
		Handler:            admission.NewHandler(admission.Create, admission.Update),
		restrictedNetworks: restrictedNetworks,
	}
}

func (r *restrictedEndpointsAdmission) SetAuthorizer(a authorizer.Authorizer) {
	r.authorizer = a
}

func (r *restrictedEndpointsAdmission) Validate() error {
	if r.authorizer == nil {
		return fmt.Errorf("missing authorizer")
	}
	return nil
}

func (r *restrictedEndpointsAdmission) findRestrictedIP(ep *kapi.Endpoints) string {
	for _, subset := range ep.Subsets {
		for _, addr := range subset.Addresses {
			ip := net.ParseIP(addr.IP)
			if ip == nil {
				continue
			}
			for _, net := range r.restrictedNetworks {
				if net.Contains(ip) {
					return addr.IP
				}
			}
		}
	}
	return ""
}

func (r *restrictedEndpointsAdmission) checkAccess(attr admission.Attributes) (bool, error) {
	authzAttr := authorizer.AttributesRecord{
		User:            attr.GetUserInfo(),
		Verb:            "create",
		Namespace:       attr.GetNamespace(),
		Resource:        "endpoints",
		Subresource:     "restricted",
		APIGroup:        kapi.GroupName,
		Name:            attr.GetName(),
		ResourceRequest: true,
	}
	allow, _, err := r.authorizer.Authorize(authzAttr)
	return allow, err
}

// Admit determines if the endpoints object should be admitted
func (r *restrictedEndpointsAdmission) Admit(a admission.Attributes) error {
	if a.GetResource().GroupResource() != kapi.Resource("endpoints") {
		return nil
	}
	ep, ok := a.GetObject().(*kapi.Endpoints)
	if !ok {
		return nil
	}
	old, ok := a.GetOldObject().(*kapi.Endpoints)
	if ok && reflect.DeepEqual(ep.Subsets, old.Subsets) {
		return nil
	}

	restrictedIP := r.findRestrictedIP(ep)
	if restrictedIP == "" {
		return nil
	}

	allow, err := r.checkAccess(a)
	if err != nil {
		return err
	}
	if !allow {
		return admission.NewForbidden(a, fmt.Errorf("endpoint address %s is not allowed", restrictedIP))
	}
	return nil
}
