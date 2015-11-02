package dns

import (
	"fmt"
	"hash/fnv"
	"net"
	"strings"

	"github.com/golang/glog"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"

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
// The standard pattern is <prefix>.<service_name>.<namespace>.(svc|endpoints|pod).<base>
//
// * prefix may be any series of prefix values
//   * _endpoints is a special prefix that returns the same as <service_name>.<namespace>.svc.<base>
// * service_name and namespace must locate a real service
//   * unless a fallback is defined, in which case the fallback name will be looked up
// * svc indicates standard service rules apply (portalIP or endpoints as A records)
//   * reverse lookup of IP is only possible for portalIP
//   * SRV records are returned for each host+port combination as:
//     _<port_name>._<port_protocol>.<dns>
//     _<port_name>.<endpoint_id>.<dns>
// * endpoints always returns each individual endpoint as A records
//   * SRV records for endpoints are similar to SVC, but are prefixed with a single label
//     that is a hash of the endpoint IP
// * pods is of the form <IP_with_dashes>.<namespace>.pod.<base> and resolves to <IP>
//
func (b *ServiceResolver) Records(dnsName string, exact bool) ([]msg.Service, error) {
	if !strings.HasSuffix(dnsName, b.base) {
		return nil, nil
	}
	prefix := strings.Trim(strings.TrimSuffix(dnsName, b.base), ".")
	segments := strings.Split(prefix, ".")
	for i, j := 0, len(segments)-1; i < j; i, j = i+1, j-1 {
		segments[i], segments[j] = segments[j], segments[i]
	}
	if len(segments) == 0 {
		return nil, nil
	}
	glog.V(4).Infof("Answering query %s:%t", dnsName, exact)
	switch base := segments[0]; base {
	case "pod":
		if len(segments) != 3 {
			return nil, nil
		}
		namespace, encodedIP := segments[1], segments[2]
		ip := convertDashIPToIP(encodedIP)
		if net.ParseIP(ip) == nil {
			return nil, nil
		}
		return []msg.Service{
			{
				Host: ip,
				Port: 0,

				Priority: 10,
				Weight:   10,
				Ttl:      30,

				Key: msg.Path(buildDNSName(b.base, "pod", namespace, getHash(ip))),
			},
		}, nil

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
		if len(svc.Spec.ClusterIP) == 0 {
			return nil, nil
		}

		subdomain := buildDNSName(b.base, base, namespace, name)
		endpointPrefix := base == "endpoints"
		retrieveEndpoints := endpointPrefix || (len(segments) > 3 && segments[3] == "_endpoints")

		// if has a portal IP and looking at svc
		if svc.Spec.ClusterIP != kapi.ClusterIPNone && !retrieveEndpoints {
			defaultService := msg.Service{
				Host: svc.Spec.ClusterIP,
				Port: 0,

				Priority: 10,
				Weight:   10,
				Ttl:      30,
			}
			defaultHash := getHash(defaultService.Host)
			defaultName := buildDNSName(subdomain, defaultHash)
			defaultService.Key = msg.Path(defaultName)

			if len(svc.Spec.Ports) == 0 {
				return []msg.Service{defaultService}, nil
			}

			services := []msg.Service{}
			if len(segments) == 3 {
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
					keyName := buildDNSName(subdomain, "_"+strings.ToLower(string(p.Protocol)), "_"+portName)
					services = append(services,
						msg.Service{
							Host: svc.Spec.ClusterIP,
							Port: port,

							Priority: 10,
							Weight:   10,
							Ttl:      30,

							Key: msg.Path(keyName),
						},
					)
				}
			}
			if len(services) == 0 {
				services = append(services, defaultService)
			}
			glog.V(4).Infof("Answered %s:%t with %#v", dnsName, exact, services)
			return services, nil
		}

		// return endpoints
		endpoints, err := b.endpoints.Endpoints(namespace).Get(name)
		if err != nil {
			return nil, err
		}

		services := make([]msg.Service, 0, len(endpoints.Subsets)*4)
		for _, s := range endpoints.Subsets {
			for _, a := range s.Addresses {
				defaultService := msg.Service{
					Host: a.IP,
					Port: 0,

					Priority: 10,
					Weight:   10,
					Ttl:      30,
				}
				defaultHash := getHash(defaultService.Host)
				defaultName := buildDNSName(subdomain, defaultHash)
				defaultService.Key = msg.Path(defaultName)

				for _, p := range s.Ports {
					port := p.Port
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

					keyName := buildDNSName(subdomain, "_"+strings.ToLower(string(p.Protocol)), "_"+portName, defaultHash)
					services = append(services, msg.Service{
						Host: a.IP,
						Port: port,

						Priority: 10,
						Weight:   10,
						Ttl:      30,

						Key: msg.Path(keyName),
					})
				}
				if len(services) == 0 {
					services = append(services, defaultService)
				}
			}
		}
		glog.V(4).Infof("Answered %s:%t with %#v", dnsName, exact, services)
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
	hostName := buildDNSName(b.base, "svc", svc.Namespace, svc.Name)
	return &msg.Service{
		Host: hostName,
		Port: port,

		Priority: 10,
		Weight:   10,
		Ttl:      30,

		Key: msg.Path(name),
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

// buildDNSName reverses the labels order and joins them with dots.
func buildDNSName(labels ...string) string {
	var res string
	for _, label := range labels {
		if len(res) == 0 {
			res = label
		} else {
			res = fmt.Sprintf("%s.%s", label, res)
		}
	}
	return res
}

// return a hash for the key name
func getHash(text string) string {
	h := fnv.New32a()
	h.Write([]byte(text))
	return fmt.Sprintf("%x", h.Sum32())
}

// convertDashIPToIP takes an encoded IP (with dashes) and replaces them with
// dots.
func convertDashIPToIP(ip string) string {
	return strings.Join(strings.Split(ip, "-"), ".")
}
