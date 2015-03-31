package dns

import (
	"fmt"
	"log"
	"strings"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"

	"github.com/skynetservices/skydns/msg"
	"github.com/skynetservices/skydns/server"
)

// ServiceResolver is a SkyDNS backend that will serve lookups for DNS entries
// based on Kubernetes service entries. The default DNS name for each service
// will be `<name>.<namespace>.<base>` where base can be an arbitrary depth
// DNS suffix. Queries not recognized within this base will return an error.
type ServiceResolver struct {
	config    *server.Config
	accessor  ServiceAccessor
	endpoints kclient.EndpointsNamespacer
	base      string
	fallback  FallbackFunc
}

// ServiceResolver implements server.Backend
var _ server.Backend = &ServiceResolver{}

type FallbackFunc func(name string, exact bool) (string, bool)

// NewServiceResolver creates an object that will return DNS record entries for
// SkyDNS based on service names.
func NewServiceResolver(config *server.Config, accessor ServiceAccessor, endpoints kclient.EndpointsNamespacer, fn FallbackFunc) *ServiceResolver {
	domain := config.Domain
	if !strings.HasSuffix(domain, ".") {
		domain = domain + "."
	}
	return &ServiceResolver{
		config:    config,
		accessor:  accessor,
		endpoints: endpoints,
		base:      domain,
		fallback:  fn,
	}
}

// Records implements the SkyDNS Backend interface and returns standard records for
// a name.
func (b *ServiceResolver) Records(name string, exact bool) ([]msg.Service, error) {
	if !strings.HasSuffix(name, b.base) {
		return nil, nil
	}
	log.Printf("serving records for %s %t", name, exact)
	prefix := strings.Trim(strings.TrimSuffix(name, b.base), ".")
	segments := strings.Split(prefix, ".")
	switch c := len(segments); {
	case c >= 2:
		svc, err := b.accessor.Services(segments[c-1]).Get(segments[c-2])
		if err != nil {
			if errors.IsNotFound(err) && b.fallback != nil {
				if fallback, ok := b.fallback(prefix, exact); ok {
					return b.Records(fallback+b.base, exact)
				}
			}
			return nil, err
		}
		if svc.Spec.PortalIP == kapi.PortalIPNone {
			endpoints, err := b.endpoints.Endpoints(segments[c-1]).Get(segments[c-2])
			if err != nil {
				return nil, err
			}
			services := make([]msg.Service, 0, len(endpoints.Endpoints))
			for _, e := range endpoints.Endpoints {
				services = append(services, msg.Service{
					Host: e.IP,
					Port: e.Port,

					Priority: 10,
					Weight:   10,
					Ttl:      30,

					Text: "",
					Key:  msg.Path(name),
				})
			}
			return services, nil
		}
		if len(svc.Spec.PortalIP) == 0 {
			return nil, nil
		}
		return []msg.Service{
			{
				Host: svc.Spec.PortalIP,
				Port: svc.Spec.Port,

				Priority: 10,
				Weight:   10,
				Ttl:      30,

				Text: "",
				Key:  msg.Path(name),
			},
		}, nil
	}
	return nil, nil
}

// ReverseRecord implements the SkyDNS Backend interface and returns standard records for
// a name.
func (b *ServiceResolver) ReverseRecord(name string) (*msg.Service, error) {
	portalIP, ok := extractIP(name)
	if !ok {
		return nil, fmt.Errorf("does not support reverse lookup with %s", name)
	}

	svc, err := b.accessor.ServiceByPortalIP(portalIP)
	if err != nil {
		return nil, err
	}
	return &msg.Service{
		Host: fmt.Sprintf("%s.%s.%s", svc.Name, svc.Namespace, b.base),
		Port: svc.Spec.Port,

		Priority: 10,
		Weight:   10,
		Ttl:      30,

		Text: "",
		Key:  msg.Path(name),
	}, nil
}

// arpaSuffix is the standard suffix for PTR IP reverse lookups.
const arpaSuffix = ".in-addr.arpa."

// extractIP turns a standard PTR reverse record lookup name
// into an IP address
func extractIP(reverseName string) (string, bool) {
	if !strings.HasSuffix(reverseName, arpaSuffix) {
		return "", false
	}
	search := strings.TrimSuffix(reverseName, arpaSuffix)

	// reverse the segments and then combine them
	segments := strings.Split(search, ".")
	for i := 0; i < len(segments)/2; i++ {
		j := len(segments) - i - 1
		segments[i], segments[j] = segments[j], segments[i]
	}
	return strings.Join(segments, "."), true
}
