package dns

import (
	"fmt"
	"strings"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	"github.com/skynetservices/skydns/msg"
	"github.com/skynetservices/skydns/server"
)

// ServiceResolver is a SkyDNS backend that will serve lookups for DNS entries
// based on Kubernetes service entries. The default DNS name for each service
// will be `<name>.<namespace>.<base>` where base can be an arbitrary depth
// DNS suffix. Queries not recognized within this base will return an error.
type ServiceResolver struct {
	accessor ServiceAccessor
	config   *server.Config
	base     string
}

// ServiceResolver implements server.Backend
var _ server.Backend = &ServiceResolver{}

// NewServiceResolver creates an object that will return DNS record entries for
// SkyDNS based on service names.
func NewServiceResolver(config *server.Config, accessor ServiceAccessor) *ServiceResolver {
	domain := config.Domain
	if !strings.HasSuffix(domain, ".") {
		domain = domain + "."
	}
	return &ServiceResolver{
		accessor: accessor,
		config:   config,
		base:     domain,
	}
}

// Records implements the SkyDNS Backend interface and returns standard records for
// a name.
func (b *ServiceResolver) Records(name string, exact bool) ([]msg.Service, error) {
	if !strings.HasSuffix(name, b.base) {
		return nil, nil
	}
	prefix := strings.Trim(strings.TrimSuffix(name, b.base), ".")
	segments := strings.Split(prefix, ".")
	switch c := len(segments); {
	case c == 1:
		items, err := b.accessor.Services(segments[0]).List(labels.Everything())
		if err != nil {
			return nil, err
		}
		services := make([]msg.Service, 0, len(items.Items))
		for _, svc := range items.Items {
			services = append(services, msg.Service{
				Host: fmt.Sprintf("%s.%s", svc.Name, name),
				Port: svc.Spec.Port,

				Priority: 10,
				Weight:   10,
				Ttl:      30,

				Text: "",
				Key:  msg.Path(name),
			})
		}
		return services, nil
	case c >= 2:
		svc, err := b.accessor.Services(segments[c-1]).Get(segments[c-2])
		if err != nil {
			return nil, err
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
