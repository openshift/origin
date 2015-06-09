package dns

import (
	"fmt"
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
//
// The standard pattern is <prefix>.<service_name>.<namespace>.(svc|endpoints).<base>
//
// * prefix may be any series of prefix values
// * service_name and namespace must locate a real service
// * svc indicates standard service rules apply (portalIP or endpoints as A records)
//   * reverse lookup of IP is only possible for portalIP
//   * SRV records are returned for each host+port combination as:
//     _<port_name>._<port_protocol>.<dns>
//     _<port_name>.<endpoint_id>.<dns>
//   * endpoint_id is "portal" when portalIP is set
// * endpoints always returns each individual endpoint as A records
//
func (b *ServiceResolver) Records(name string, exact bool) ([]msg.Service, error) {
	if !strings.HasSuffix(name, b.base) {
		return nil, nil
	}
	prefix := strings.Trim(strings.TrimSuffix(name, b.base), ".")
	segments := strings.Split(prefix, ".")
	for i, j := 0, len(segments)-1; i < j; i, j = i+1, j-1 {
		segments[i], segments[j] = segments[j], segments[i]
	}
	if len(segments) == 0 {
		return nil, nil
	}

	switch segments[0] {
	case "svc", "endpoints":
		if len(segments) < 3 {
			return nil, nil
		}
		namespace, name := segments[1], segments[2]
		svc, err := b.accessor.Services(namespace).Get(name)
		if err != nil {
			if errors.IsNotFound(err) && b.fallback != nil {
				if fallback, ok := b.fallback(prefix, exact); ok {
					return b.Records(fallback+b.base, exact)
				}
			}
			return nil, err
		}

		// no portalIP and not headless, no DNS
		if len(svc.Spec.PortalIP) == 0 {
			return nil, nil
		}

		retrieveEndpoints := segments[0] == "endpoints" || (len(segments) > 3 && segments[3] == "_endpoints")

		// if has a portal IP and looking at svc
		if svc.Spec.PortalIP != kapi.PortalIPNone && !retrieveEndpoints {
			if len(svc.Spec.Ports) == 0 {
				return nil, nil
			}
			services := []msg.Service{}
			for _, p := range svc.Spec.Ports {
				port := p.Port
				if port == 0 {
					port = p.TargetPort.IntVal
				}
				if port == 0 {
					continue
				}
				if len(p.Protocol) == 0 {
					p.Protocol = kapi.ProtocolTCP
				}
				portName := p.Name
				if len(portName) == 0 {
					portName = fmt.Sprintf("unknown-port-%d", port)
				}
				srvName := fmt.Sprintf("%s.portal.%s", portName, name)
				keyName := fmt.Sprintf("_%s._%s.%s", portName, p.Protocol, name)
				services = append(services,
					msg.Service{
						Host: svc.Spec.PortalIP,
						Port: port,

						Priority: 10,
						Weight:   10,
						Ttl:      30,

						Text: "",
						Key:  msg.Path(srvName),
					},
					msg.Service{
						Host: srvName,
						Port: port,

						Priority: 10,
						Weight:   10,
						Ttl:      30,

						Text: "",
						Key:  msg.Path(keyName),
					},
				)
			}
			return services, nil
		}

		// return endpoints
		endpoints, err := b.endpoints.Endpoints(namespace).Get(name)
		if err != nil {
			return nil, err
		}
		targets := make(map[string]int)
		services := make([]msg.Service, 0, len(endpoints.Subsets)*4)
		count := 1
		for _, s := range endpoints.Subsets {
			for _, a := range s.Addresses {
				shortName := ""
				if a.TargetRef != nil {
					name := fmt.Sprintf("%s-%s", a.TargetRef.Name, a.TargetRef.Namespace)
					if c, ok := targets[name]; ok {
						shortName = fmt.Sprintf("e%d", c)
					} else {
						shortName = fmt.Sprintf("e%d", count)
						targets[name] = count
						count++
					}
				} else {
					shortName = fmt.Sprintf("e%d", count)
					count++
				}
				hadPort := false
				for _, p := range s.Ports {
					port := p.Port
					if port == 0 {
						continue
					}
					hadPort = true
					if len(p.Protocol) == 0 {
						p.Protocol = kapi.ProtocolTCP
					}
					portName := p.Name
					if len(portName) == 0 {
						portName = fmt.Sprintf("unknown-port-%d", port)
					}
					srvName := fmt.Sprintf("%s.%s.%s", portName, shortName, name)
					services = append(services, msg.Service{
						Host: a.IP,
						Port: port,

						Priority: 10,
						Weight:   10,
						Ttl:      30,

						Text: "",
						Key:  msg.Path(srvName),
					})
					keyName := fmt.Sprintf("_%s._%s.%s", portName, p.Protocol, name)
					services = append(services, msg.Service{
						Host: srvName,
						Port: port,

						Priority: 10,
						Weight:   10,
						Ttl:      30,

						Text: "",
						Key:  msg.Path(keyName),
					})
				}

				if !hadPort {
					services = append(services, msg.Service{
						Host: a.IP,

						Priority: 10,
						Weight:   10,
						Ttl:      30,

						Text: "",
						Key:  msg.Path(name),
					})
				}
			}
		}
		return services, nil
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
	port := 0
	if len(svc.Spec.Ports) > 0 {
		port = svc.Spec.Ports[0].Port
	}
	return &msg.Service{
		Host: fmt.Sprintf("%s.%s.svc.%s", svc.Name, svc.Namespace, b.base),
		Port: port,

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
