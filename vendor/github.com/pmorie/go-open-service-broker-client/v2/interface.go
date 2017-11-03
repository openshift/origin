package v2

import (
	"crypto/tls"
)

// AuthConfig is a union-type representing the possible auth configurations a
// client may use to authenticate to a broker.  Currently, only basic auth is
// supported.
type AuthConfig struct {
	BasicAuthConfig *BasicAuthConfig
	BearerConfig    *BearerConfig
}

// BasicAuthConfig represents a set of basic auth credentials.
type BasicAuthConfig struct {
	// Username is the basic auth username.
	Username string
	// Password is the basic auth password.
	Password string
}

// BearerConfig represents bearer token credentials.
type BearerConfig struct {
	// Token is the bearer token.
	Token string
}

// ClientConfiguration represents the configuration of a Client.
type ClientConfiguration struct {
	// Name is the name to use for this client in log messages.  Using the
	// logical name of the Broker this client is for is recommended.
	Name string
	// URL is the URL to use to contact the broker.
	URL string
	// APIVersion is the APIVersion to use for this client.  API features
	// adopted after the 2.11 version of the API will only be sent if
	// APIVersion is an API version that supports them.
	APIVersion APIVersion
	// AuthInfo is the auth configuration the client should use to authenticate
	// to the broker.
	AuthConfig *AuthConfig
	// TLSConfig is the TLS configuration to use when communicating with the
	// broker.
	TLSConfig *tls.Config
	// Insecure represents whether the 'InsecureSkipVerify' TLS configuration
	// field should be set.  If the TLSConfig field is set and this field is
	// set to true, it overrides the value in the TLSConfig field.
	Insecure bool
	// TimeoutSeconds is the length of the timeout of any request to the
	// broker, in seconds.
	TimeoutSeconds int
	// EnableAlphaFeatures controls whether alpha features in the Open Service
	// Broker API are enabled in a client.  Features are considered to be
	// alpha if they have been accepted into the Open Service Broker API but
	// not released in a version of the API specification.  Features are
	// indicated as being alpha when the client API fields they represent
	// begin with the 'Alpha' prefix.
	//
	// If alpha features are not enabled, the client will not send or return
	// any request parameters or request or response fields that correspond to
	// alpha features.
	EnableAlphaFeatures bool
	// CAData holds PEM-encoded bytes (typically read from a root certificates bundle).
	// This CA certificate will be added to any specified in TLSConfig.RootCAs.
	CAData []byte
}

// DefaultClientConfiguration returns a default ClientConfiguration:
//
// - latest API version
// - 60 second timeout (referenced as a typical timeout in the Open Service
//   Broker API spec)
// - alpha features disabled
func DefaultClientConfiguration() *ClientConfiguration {
	return &ClientConfiguration{
		APIVersion:          LatestAPIVersion(),
		TimeoutSeconds:      60,
		EnableAlphaFeatures: false,
	}
}

// Client defines the interface to the v2 Open Service Broker client.  The
// logical lifecycle of client operations is:
//
// 1.  Get the broker's catalog of services with the GetCatalog method
// 2.  Provision a new instance of a service with the ProvisionInstance method
// 3.  Update the parameters or plan of an instance with the UpdateInstance method
// 4.  Deprovision an instance with the DeprovisionInstance method
//
// Some services and plans support binding from an instance of the service to
// an application.  The logical lifecycle of a binding is:
//
// 1.  Create a new binding to an instance of a service with the Bind method
// 2.  Delete a binding to an instance with the Unbind method
type Client interface {
	// GetCatalog returns information about the services the broker offers and
	// their plans or an error.  GetCatalog calls GET on the Broker's catalog
	// endpoint (/v2/catalog).
	GetCatalog() (*CatalogResponse, error)
	// ProvisionInstance requests that a new instance of a service be
	// provisioned and returns information about the instance or an error.
	// ProvisionInstance does a PUT on the Broker's endpoint for the requested
	// instance ID (/v2/service_instances/instance-id).
	//
	// If the AcceptsIncomplete field of the request is set to true, the
	// broker may complete the request asynchronously.  Callers should check
	// the value of the Async field on the response and check the operation
	// status using PollLastOperation if the Async field is true.
	ProvisionInstance(r *ProvisionRequest) (*ProvisionResponse, error)
	// UpdateInstance requests that an instances plan or parameters be updated
	// and returns information about asynchronous responses or an error.
	// UpdateInstance does a PATCH on the Broker's endpoint for the requested
	// instance ID (/v2/service_instances/instance-id).
	//
	// If the AcceptsIncomplete field of the request is set to true, the
	// broker may complete the request asynchronously.  Callers should check
	// the value of the Async field on the response and check the operation
	// status using PollLastOperation if the Async field is true.
	UpdateInstance(r *UpdateInstanceRequest) (*UpdateInstanceResponse, error)
	// DeprovisionInstance requests that an instances plan or parameters be
	// updated and returns information about asynchronous responses or an
	// error. DeprovisionInstance does a DELETE on the Broker's endpoint for
	// the requested instance ID (/v2/service_instances/instance-id).
	//
	// If the AcceptsIncomplete field of the request is set to true, the
	// broker may complete the request asynchronously.  Callers should check
	// the value of the Async field on the response and check the operation
	// status using PollLastOperation if the Async field is true.  Note that
	// there are special semantics for PollLastOperation when checking the
	// status of deprovision operations; see the doc for that method.
	DeprovisionInstance(r *DeprovisionRequest) (*DeprovisionResponse, error)
	// PollLastOperation sends a request to query the last operation for a
	// service instance to the broker and returns information about the
	// operation or an error.  PollLastOperation does a GET on the broker's
	// last operation endpoint for the requested instance ID
	// (/v2/service_instances/instance-id/last_operation).
	//
	// Callers should periodically call PollLastOperation until they receive a
	// success response.  PollLastOperation may return an HTTP GONE error for
	// asynchronous deprovisions.  This is a valid response for async
	// operations and means that the instance has been successfully
	// deprovisioned.  When calling PollLastOperation to check the status of
	// an asynchronous deprovision, callers check the status of an
	// asynchronous deprovision, callers should test the value of the returned
	// error with IsGoneError.
	PollLastOperation(r *LastOperationRequest) (*LastOperationResponse, error)
	// PollBindingLastOperation is an ALPHA API method and may change.
	// Alpha features must be enabled and the client must be using the
	// latest API Version in order to use this method.
	//
	// PollBindingLastOperation sends a request to query the last operation
	// for a service binding to the broker and returns information about the
	// operation or an error.  PollBindingLastOperation does a GET on the broker's
	// last operation endpoint for the requested binding ID
	// (/v2/service_instances/instance-id/service_bindings/binding-id/last_operation).
	//
	// Callers should periodically call PollBindingLastOperation until they
	// receive a success response.  PollBindingLastOperation may return an
	// HTTP GONE error for asynchronous unbinding.  This is a valid response
	// for async operations and means that the binding has been successfully
	// deleted.  When calling PollBindingLastOperation to check the status of
	// an asynchronous unbind, callers should test the value of the returned
	// error with IsGoneError.
	PollBindingLastOperation(r *BindingLastOperationRequest) (*LastOperationResponse, error)
	// Bind requests a new binding between a service instance and an
	// application and returns information about the binding or an error. Bind
	// does a PUT on the Broker's endpoint for the requested instance and
	// binding IDs (/v2/service_instances/instance-id/service_bindings/binding-id).
	Bind(r *BindRequest) (*BindResponse, error)
	// Bind requests that a binding between a service instance and an
	// application be deleted and returns information about the binding or an
	// error. Unbind does a DELETE on the Broker's endpoint for the requested
	// instance and binding IDs (/v2/service_instances/instance-id/service_bindings/binding-id).
	Unbind(r *UnbindRequest) (*UnbindResponse, error)
	// GetBinding is an ALPHA API method and may change. Alpha features must
	// be enabled and the client must be using the latest API Version in
	// order to use this method.
	//
	// GetBinding returns configuration and credential information
	// about an existing binding. GetBindings calls GET on the Broker's
	// binding endpoint
	// (/v2/service_instances/instance-id/service_bindings/binding-id)
	GetBinding(r *GetBindingRequest) (*GetBindingResponse, error)
}

// CreateFunc allows control over which implementation of a Client is
// returned.  Users of the Client interface may need to create clients for
// multiple brokers in a way that makes normal dependency injection
// prohibitive.  In order to make such code testable, users of the API can
// inject a CreateFunc, and use the CreateFunc from the fake package in tests.
type CreateFunc func(*ClientConfiguration) (Client, error)
