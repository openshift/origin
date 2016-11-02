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
	// EndpointTable are endpoints that back the service, this translates into a final backend
	// implementation for routers.
	EndpointTable []Endpoint
}

// ServiceAliasConfig is a route for a service.  Uniquely identified by host + path.
type ServiceAliasConfig struct {
	// Name is the user-specified name of the route.
	Name string
	// Namespace is the namespace of the route.
	Namespace string
	// Host is a required host name ie. www.example.com
	Host string
	// Path is an optional path ie. www.example.com/myservice where "myservice" is the path
	Path string
	// TLSTermination is the termination policy for this backend and drives the mapping files and router configuration
	TLSTermination routeapi.TLSTerminationType
	// Certificates used for securing this backend.  Keyed by the cert id
	Certificates map[string]Certificate
	// Indicates the status of configuration that needs to be persisted.  Right now this only
	// includes the certificates and is not an indicator of being written to the underlying
	// router implementation
	Status ServiceAliasConfigStatus
	// Indicates the port the user wishes to expose. If empty, a port will be selected for the service.
	PreferPort string
	// InsecureEdgeTerminationPolicy indicates desired behavior for
	// insecure connections to an edge-terminated route:
	//   none (or disable), allow or redirect
	InsecureEdgeTerminationPolicy routeapi.InsecureEdgeTerminationPolicyType

	// Hash of the route name - used to obscure cookieId
	RoutingKeyName string

	// IsWildcard indicates this service unit needs wildcarding support.
	IsWildcard bool

	// Annotations attached to this route
	Annotations map[string]string

	// ServiceUnitNames is a collection of services that support this route, keyed by service name
	// and valued on the weight attached to it with respect to other entries in the map
	ServiceUnitNames map[string]int32
}

type ServiceAliasConfigStatus string

const (
	// ServiceAliasConfigStatusSaved indicates that the necessary files for this config have
	// been persisted to disk.
	ServiceAliasConfigStatusSaved ServiceAliasConfigStatus = "saved"
)

// Certificate represents a pub/private key pair.  It is identified by ID which will become the file name.
// A CA certificate will not have a PrivateKey set.
type Certificate struct {
	ID         string
	Contents   string
	PrivateKey string
}

// Endpoint is an internal representation of a k8s endpoint.
type Endpoint struct {
	ID            string
	IP            string
	Port          string
	TargetName    string
	PortName      string
	IdHash        string
	NoHealthCheck bool
}

// certificateManager provides the ability to write certificates for a ServiceAliasConfig
type certificateManager interface {
	// WriteCertificatesForConfig writes all certificates for all ServiceAliasConfigs in config
	WriteCertificatesForConfig(config *ServiceAliasConfig) error
	// DeleteCertificatesForConfig deletes all certificates for all ServiceAliasConfigs in config
	DeleteCertificatesForConfig(config *ServiceAliasConfig) error
	// CertificateWriter provides direct access to the underlying writer if required
	CertificateWriter() certificateWriter
}

// certManagerConfig provides the configuration necessary for certmanager to manipulate certificates.
type certificateManagerConfig struct {
	// certKeyFunc is used to find the edge certificate (which also has the key) from the cert map
	// of the ServiceAliasConfig
	certKeyFunc certificateKeyFunc
	// caCertKeyFunc is used to find the edge ca certificate from the cert map of the ServiceAliasConfig
	caCertKeyFunc certificateKeyFunc
	// destCertKeyFunc is used to find the ca certificate of a destination (pod) from the cert map
	// of the ServiceAliasConfig
	destCertKeyFunc certificateKeyFunc
	// certDir is where the edge certificates will be written.
	certDir string
	// caCertDir is where the edge certificates will be written.  It must be different than certDir
	caCertDir string
}

// certificateKeyFunc provides the certificateManager a way to create keys the same way the template
// router creates them so it can retrieve the certificates from a ServiceAliasConfig correctly
type certificateKeyFunc func(config *ServiceAliasConfig) string

// certificateWriter is used by a certificateManager to perform the actual writing.  It is abstracteed
// out in order to provide the ability to inject a test writer for unit testing
type certificateWriter interface {
	WriteCertificate(directory string, id string, cert []byte) error
	DeleteCertificate(directory, id string) error
}

//TemplateSafeName provides a name that can be used in the template that does not contain restricted
//characters like / which is used to concat namespace and name in the service unit key
func (s ServiceUnit) TemplateSafeName() string {
	return strings.Replace(s.Name, "/", "-", -1)
}
