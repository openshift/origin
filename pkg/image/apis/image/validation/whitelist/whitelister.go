package whitelist

import (
	"fmt"
	"net"
	"reflect"
	"strings"

	"github.com/golang/glog"

	kerrutil "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"

	serverapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	stringsutil "github.com/openshift/origin/pkg/util/strings"
)

// WhitelistTransport says whether the associated registry host shall be treated as secure or insecure.
type WhitelistTransport string

const (
	WhitelistTransportAny      WhitelistTransport = "any"
	WhitelistTransportSecure   WhitelistTransport = "secure"
	WhitelistTransportInsecure WhitelistTransport = "insecure"
)

// RegistryWhitelister decides whether given image pull specs are allowed by system's image policy.
type RegistryWhitelister interface {
	// AdmitHostname returns error if the given host is not allowed by the whitelist.
	AdmitHostname(host string, transport WhitelistTransport) error
	// AdmitPullSpec returns error if the given pull spec is allowed neither by the whitelist nor by the
	// collected whitelisted pull specs.
	AdmitPullSpec(pullSpec string, transport WhitelistTransport) error
	// AdmitDockerImageReference returns error if the given reference is allowed neither by the whitelist nor
	// by the collected whitelisted pull specs.
	AdmitDockerImageReference(ref *imageapi.DockerImageReference, transport WhitelistTransport) error
	// WhitelistRegistry extends internal whitelist for additional registry domain name. Accepted values are:
	//  <host>, <host>:<port>
	// where each component can contain wildcards like '*' or '??' to match wide range of registries. If the
	// port is omitted, the default will be appended based on the given transport. If the transport is "any",
	// the given glob will match hosts with both :80 and :443 ports.
	WhitelistRegistry(hostPortGlob string, transport WhitelistTransport) error
	// WhitelistPullSpecs allows to whitelist particular pull specs. References must match exactly one of the
	// given pull specs for it to be whitelisted.
	WhitelistPullSpecs(pullSpecs ...string)
	// Copy returns a deep copy of the whitelister. This is useful for temporarily whitelisting additional
	// registries/pullSpecs before a specific validation.
	Copy() RegistryWhitelister
}

type allowedHostPortGlobs struct {
	host string
	port string
}

type registryWhitelister struct {
	whitelist             []allowedHostPortGlobs
	pullSpecs             sets.String
	registryHostRetriever imageapi.RegistryHostnameRetriever
}

var _ RegistryWhitelister = &registryWhitelister{}

// NewRegistryWhitelister creates a whitelister that admits registry domains and pull specs based on the given
// list of allowed registries and the current domain name of the integrated Docker registry.
func NewRegistryWhitelister(
	whitelist serverapi.AllowedRegistries,
	registryHostRetriever imageapi.RegistryHostnameRetriever,
) (RegistryWhitelister, error) {
	errs := []error{}
	rw := registryWhitelister{
		whitelist:             make([]allowedHostPortGlobs, 0, len(whitelist)),
		pullSpecs:             sets.NewString(),
		registryHostRetriever: registryHostRetriever,
	}
	// iterate in reversed order to make the patterns appear in the same order as given (patterns are prepended)
	for i := len(whitelist) - 1; i >= 0; i-- {
		registry := whitelist[i]
		transport := WhitelistTransportSecure
		if registry.Insecure {
			transport = WhitelistTransportInsecure
		}
		err := rw.WhitelistRegistry(registry.DomainName, transport)
		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return nil, kerrutil.NewAggregate(errs)
	}
	return &rw, nil
}

// WhitelistAllRegistries returns a whitelister that will allow any given registry host name.
// TODO: make a new implementation of RegistryWhitelister instead that will not bother with pull specs
func WhitelistAllRegistries() RegistryWhitelister {
	return &registryWhitelister{
		whitelist: []allowedHostPortGlobs{{host: "*", port: "*"}},
		pullSpecs: sets.NewString(),
	}
}

func (rw *registryWhitelister) AdmitHostname(hostname string, transport WhitelistTransport) error {
	return rw.AdmitDockerImageReference(&imageapi.DockerImageReference{Registry: hostname}, transport)
}

func (rw *registryWhitelister) AdmitPullSpec(pullSpec string, transport WhitelistTransport) error {
	ref, err := imageapi.ParseDockerImageReference(pullSpec)
	if err != nil {
		return err
	}
	return rw.AdmitDockerImageReference(&ref, transport)
}

func (rw *registryWhitelister) AdmitDockerImageReference(ref *imageapi.DockerImageReference, transport WhitelistTransport) error {
	const showMax = 5
	if rw.pullSpecs.Len() > 0 {
		if rw.pullSpecs.Has(ref.Exact()) || rw.pullSpecs.Has(ref.DockerClientDefaults().Exact()) || rw.pullSpecs.Has(ref.DaemonMinimal().Exact()) {
			return nil
		}
	}

	if rw.registryHostRetriever != nil {
		if localRegistry, ok := rw.registryHostRetriever.InternalRegistryHostname(); ok {
			rw.WhitelistRegistry(localRegistry, WhitelistTransportSecure)
		}
	}

	var (
		host, port string
		err        error
	)
	switch transport {
	case WhitelistTransportAny:
		host, port, err = net.SplitHostPort(ref.Registry)
		if err != nil || len(port) == 0 {
			host = ref.Registry
			port = ""
		}
		if len(host) == 0 {
			host, _ = ref.RegistryHostPort(false)
		}
	case WhitelistTransportInsecure:
		host, port = ref.RegistryHostPort(true)
	default:
		host, port = ref.RegistryHostPort(false)
	}

	matchHost := func(h string) bool {
		for _, hp := range rw.whitelist {
			if stringsutil.IsWildcardMatch(h, hp.host) {
				if len(port) == 0 {
					switch hp.port {
					case "80", "443", "*":
						return true
					default:
						continue
					}
				}
				if stringsutil.IsWildcardMatch(port, hp.port) {
					return true
				}
			}
		}
		return false
	}

	switch host {
	case imageapi.DockerDefaultV1Registry, imageapi.DockerDefaultV2Registry:
		// try to match plain docker.io first to satisfy `docker.io:*` wildcard
		if matchHost(imageapi.DockerDefaultRegistry) {
			return nil
		}
		fallthrough
	default:
		if matchHost(host) {
			return nil
		}
	}

	hostname := ref.Registry
	if len(ref.Registry) == 0 {
		if len(port) > 0 {
			hostname = net.JoinHostPort(host, port)
		} else {
			hostname = host
		}
	}

	var whitelist []string
	for i := 0; i < len(rw.whitelist); i++ {
		whitelist = append(whitelist, fmt.Sprintf("%q", net.JoinHostPort(rw.whitelist[i].host, rw.whitelist[i].port)))
	}

	if len(rw.whitelist) == 0 {
		glog.V(5).Infof("registry %q not allowed by empty whitelist", hostname)
		return fmt.Errorf("registry %q not allowed by empty whitelist", hostname)
	}

	glog.V(5).Infof("registry %q not allowed by whitelist: %s", hostname, strings.Join(whitelist, ", "))
	if len(rw.whitelist) <= showMax {
		return fmt.Errorf("registry %q not allowed by whitelist: %s", hostname, strings.Join(whitelist, ", "))
	}
	return fmt.Errorf("registry %q not allowed by whitelist: %s, and %d more ...", hostname, strings.Join(whitelist[:showMax-1], ", "), len(whitelist)-showMax+1)
}

func (rw *registryWhitelister) WhitelistRegistry(hostPortGlob string, transport WhitelistTransport) error {
	hps := make([]allowedHostPortGlobs, 1, 2)

	parts := strings.SplitN(hostPortGlob, ":", 3)
	switch len(parts) {
	case 1:
		hps[0].host = parts[0]
		switch transport {
		case WhitelistTransportAny:
			hps[0].port = "80"
			// add two entries matching both secure and insecure ports
			hps = append(hps, allowedHostPortGlobs{host: parts[0], port: "443"})
		case WhitelistTransportInsecure:
			hps[0].port = "80"
		default:
			hps[0].port = "443"
		}
	case 2:
		hps[0].host, hps[0].port = parts[0], parts[1]
	default:
		return fmt.Errorf("failed to parse allowed registry %q: too many colons", hostPortGlob)
	}

addHPsLoop:
	for i := range hps {
		for _, item := range rw.whitelist {
			if reflect.DeepEqual(item, hps[i]) {
				continue addHPsLoop
			}
		}
		rw.whitelist = append([]allowedHostPortGlobs{hps[i]}, rw.whitelist...)
	}

	return nil
}

func (rw *registryWhitelister) WhitelistPullSpecs(pullSpecs ...string) {
	rw.pullSpecs.Insert(pullSpecs...)
}

func (rw *registryWhitelister) Copy() RegistryWhitelister {
	newRW := registryWhitelister{
		whitelist:             make([]allowedHostPortGlobs, len(rw.whitelist)),
		pullSpecs:             sets.NewString(rw.pullSpecs.List()...),
		registryHostRetriever: rw.registryHostRetriever,
	}

	for i, item := range rw.whitelist {
		newRW.whitelist[i] = item
	}
	return &newRW
}
