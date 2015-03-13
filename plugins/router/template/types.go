package templaterouter

import (
	routeapi "github.com/openshift/origin/pkg/route/api"
	"strings"
)

// ServiceUnit is an encapsulation of a service, the endpoints that back that service, and the routes
// that point to the service.  This is the data that drives the creation of the router configuration files
type ServiceUnit struct {
	// Name corresponds to a service name & namespace.  Uniquely identifies the ServiceUnit
	Name string
	// EndpointTable are endpoints that back the service, this translates into a final backend implementation for routers
	// keyed by IP:port for easy access
	EndpointTable map[string]Endpoint
	// ServiceAliasConfigs is a collection of unique routes that support this service, keyed by host + path
	ServiceAliasConfigs map[string]ServiceAliasConfig
}

// ServiceAliasConfig is a route for a service.  Uniquely identified by host + path.
type ServiceAliasConfig struct {
	// Host is a required host name ie. www.example.com
	Host string
	// Path is an optional path ie. www.example.com/myservice where "myservice" is the path
	Path string
	// TLSTermination is the termination policy for this backend and drives the mapping files and router configuration
	TLSTermination routeapi.TLSTerminationType
	// Certificates used for securing this backend.  Keyed by the cert id
	Certificates map[string]Certificate
}

// Certificate represents a pub/private key pair.  It is identified by ID which is set to indicate if this is
// a client or ca certificate (see router.go).  A CA certificate will not have a PrivateKey set.
type Certificate struct {
	ID         string
	Contents   string
	PrivateKey string
}

// Endpoint is an internal representation of a k8s endpoint.
type Endpoint struct {
	ID   string
	IP   string
	Port string
}

//TemplateSafeName provides a name that can be used in the template that does not contain restricted
//characters like / which is used to concat namespace and name in the service unit key
func (s ServiceUnit) TemplateSafeName() string {
	return templateSafeString(s.Name)
}

//TemplateSafePath provides a name that can be used in the template that does not contain restricted
//characters like / which is used to concat namespace and name in the service unit key
func (s ServiceAliasConfig) TemplateSafePath() string {
	return templateSafeString(s.Path)
}

func templateSafeString(s string) string {
	return strings.Replace(s, "/", "-", -1)
}
