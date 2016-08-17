package admission

import (
	"fmt"
	"io"
	"net"
	"reflect"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/authorizer"
	"github.com/openshift/origin/pkg/client"
	oadmission "github.com/openshift/origin/pkg/cmd/server/admission"

	kadmission "k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
)

const RestrictedEndpointsPluginName = "openshift.io/RestrictedEndpointsAdmission"

func init() {
	kadmission.RegisterPlugin(RestrictedEndpointsPluginName, func(client clientset.Interface, config io.Reader) (kadmission.Interface, error) {
		return NewRestrictedEndpointsAdmission(nil), nil
	})
}

type restrictedEndpointsAdmission struct {
	*kadmission.Handler

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
		Handler:            kadmission.NewHandler(kadmission.Create, kadmission.Update),
		restrictedNetworks: restrictedNetworks,
	}
}

func (r *restrictedEndpointsAdmission) SetAuthorizer(a authorizer.Authorizer) {
	r.authorizer = a
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

func (r *restrictedEndpointsAdmission) checkAccess(attr kadmission.Attributes) (bool, error) {
	ctx := kapi.WithUser(kapi.WithNamespace(kapi.NewContext(), attr.GetNamespace()), attr.GetUserInfo())
	authzAttr := authorizer.DefaultAuthorizationAttributes{
		Verb:         "create",
		Resource:     authorizationapi.RestrictedEndpointsResource,
		APIGroup:     kapi.GroupName,
		ResourceName: attr.GetName(),
	}
	allow, _, err := r.authorizer.Authorize(ctx, authzAttr)
	return allow, err
}

// Admit determines if the endpoints object should be admitted
func (r *restrictedEndpointsAdmission) Admit(a kadmission.Attributes) error {
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
		return kadmission.NewForbidden(a, fmt.Errorf("endpoint address %s is not allowed", restrictedIP))
	}
	return nil
}
