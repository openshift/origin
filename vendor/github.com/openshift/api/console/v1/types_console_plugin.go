package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +openshift:compatibility-gen:level=1

// ConsolePlugin is an extension for customizing OpenShift web console by
// dynamically loading code from another service running on the cluster.
//
// Compatibility level 1: Stable within a major release for a minimum of 12 months or 3 minor releases (whichever is longer).
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=consoleplugins,scope=Cluster
// +openshift:api-approved.openshift.io=https://github.com/openshift/api/pull/1186
// +openshift:file-pattern=operatorOrdering=90
// +openshift:capability=Console
// +kubebuilder:metadata:annotations="description=Extension for configuring openshift web console plugins."
// +kubebuilder:metadata:annotations="displayName=ConsolePlugin"
// +kubebuilder:metadata:annotations="service.beta.openshift.io/inject-cabundle=true"
type ConsolePlugin struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata"`

	// spec contains the desired configuration for the console plugin.
	// +required
	Spec ConsolePluginSpec `json:"spec"`
}

// ConsolePluginSpec is the desired plugin configuration.
type ConsolePluginSpec struct {
	// displayName is the display name of the plugin.
	// The dispalyName should be between 1 and 128 characters.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=128
	DisplayName string `json:"displayName"`
	// backend holds the configuration of backend which is serving console's plugin .
	// +required
	Backend ConsolePluginBackend `json:"backend"`
	// proxy is a list of proxies that describe various service type
	// to which the plugin needs to connect to.
	// +listType=atomic
	// +optional
	Proxy []ConsolePluginProxy `json:"proxy,omitempty"`
	// i18n is the configuration of plugin's localization resources.
	// +optional
	I18n ConsolePluginI18n `json:"i18n"`
	// contentSecurityPolicy is a list of Content-Security-Policy (CSP) directives for the plugin.
	// Each directive specifies a list of values, appropriate for the given directive type,
	// for example a list of remote endpoints for fetch directives such as ScriptSrc.
	// Console web application uses CSP to detect and mitigate certain types of attacks,
	// such as cross-site scripting (XSS) and data injection attacks.
	// Dynamic plugins should specify this field if need to load assets from outside
	// the cluster or if violation reports are observed. Dynamic plugins should always prefer
	// loading their assets from within the cluster, either by vendoring them, or fetching
	// from a cluster service.
	// CSP violation reports can be viewed in the browser's console logs during development and
	// testing of the plugin in the OpenShift web console.
	// Available directive types are DefaultSrc, ScriptSrc, StyleSrc, ImgSrc, FontSrc and ConnectSrc.
	// Each of the available directives may be defined only once in the list.
	// The value 'self' is automatically included in all fetch directives by the OpenShift web
	// console's backend.
	// For more information about the CSP directives, see:
	// https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Content-Security-Policy
	//
	// The OpenShift web console server aggregates the CSP directives and values across
	// its own default values and all enabled ConsolePlugin CRs, merging them into a single
	// policy string that is sent to the browser via `Content-Security-Policy` HTTP response header.
	//
	// Example:
	//   ConsolePlugin A directives:
	//     script-src: https://script1.com/, https://script2.com/
	//     font-src: https://font1.com/
	//
	//   ConsolePlugin B directives:
	//     script-src: https://script2.com/, https://script3.com/
	//     font-src: https://font2.com/
	//     img-src: https://img1.com/
	//
	//   Unified set of CSP directives, passed to the OpenShift web console server:
	//     script-src: https://script1.com/, https://script2.com/, https://script3.com/
	//     font-src: https://font1.com/, https://font2.com/
	//     img-src: https://img1.com/
	//
	//   OpenShift web console server CSP response header:
	//     Content-Security-Policy: default-src 'self'; base-uri 'self'; script-src 'self' https://script1.com/ https://script2.com/ https://script3.com/; font-src 'self' https://font1.com/ https://font2.com/; img-src 'self' https://img1.com/; style-src 'self'; frame-src 'none'; object-src 'none'
	//
	// +openshift:enable:FeatureGate=ConsolePluginContentSecurityPolicy
	// +kubebuilder:validation:MaxItems=5
	// +kubebuilder:validation:XValidation:rule="self.map(x, x.values.map(y, y.size()).sum()).sum() < 8192",message="the total combined size of values of all directives must not exceed 8192 (8kb)"
	// +listType=map
	// +listMapKey=directive
	// +optional
	ContentSecurityPolicy []ConsolePluginCSP `json:"contentSecurityPolicy"`
}

// DirectiveType is an enumeration of OpenShift web console supported CSP directives.
// LoadType is an enumeration of i18n loading types.
// +kubebuilder:validation:Enum:="DefaultSrc";"ScriptSrc";"StyleSrc";"ImgSrc";"FontSrc";"ConnectSrc"
// +enum
type DirectiveType string

const (
	// DefaultSrc directive serves as a fallback for the other CSP fetch directives.
	// For more information about the DefaultSrc directive, see:
	// https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Content-Security-Policy/default-src
	DefaultSrc DirectiveType = "DefaultSrc"
	// ScriptSrc directive specifies valid sources for JavaScript.
	// For more information about the ScriptSrc directive, see:
	// https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Content-Security-Policy/script-src
	ScriptSrc DirectiveType = "ScriptSrc"
	// StyleSrc directive specifies valid sources for stylesheets.
	// For more information about the StyleSrc directive, see:
	// https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Content-Security-Policy/style-src
	StyleSrc DirectiveType = "StyleSrc"
	// ImgSrc directive specifies a valid sources of images and favicons.
	// For more information about the ImgSrc directive, see:
	// https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Content-Security-Policy/img-src
	ImgSrc DirectiveType = "ImgSrc"
	// FontSrc directive specifies valid sources for fonts loaded using @font-face.
	// For more information about the FontSrc directive, see:
	// https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Content-Security-Policy/font-src
	FontSrc DirectiveType = "FontSrc"
	// ConnectSrc directive restricts the URLs which can be loaded using script interfaces.
	// For more information about the ConnectSrc directive, see:
	// https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Content-Security-Policy/connect-src
	ConnectSrc DirectiveType = "ConnectSrc"
)

// CSPDirectiveValue is single value for a Content-Security-Policy directive.
// Each directive value must have a maximum length of 1024 characters and must not contain
// whitespace, commas (,), semicolons (;) or single quotes ('). The value '*' is not permitted.
// +kubebuilder:validation:MinLength=1
// +kubebuilder:validation:MaxLength=1024
// +kubebuilder:validation:XValidation:rule="!self.contains(\"'\")",message="CSP directive value cannot contain a quote"
// +kubebuilder:validation:XValidation:rule="!self.matches('\\\\s')",message="CSP directive value cannot contain a whitespace"
// +kubebuilder:validation:XValidation:rule="!self.contains(',')",message="CSP directive value cannot contain a comma"
// +kubebuilder:validation:XValidation:rule="!self.contains(';')",message="CSP directive value cannot contain a semi-colon"
// +kubebuilder:validation:XValidation:rule="self != '*'",message="CSP directive value cannot be a wildcard"
type CSPDirectiveValue string

// ConsolePluginCSP holds configuration for a specific CSP directive
type ConsolePluginCSP struct {
	// directive specifies which Content-Security-Policy directive to configure.
	// Available directive types are DefaultSrc, ScriptSrc, StyleSrc, ImgSrc, FontSrc and ConnectSrc.
	// DefaultSrc directive serves as a fallback for the other CSP fetch directives.
	// For more information about the DefaultSrc directive, see:
	// https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Content-Security-Policy/default-src
	// ScriptSrc directive specifies valid sources for JavaScript.
	// For more information about the ScriptSrc directive, see:
	// https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Content-Security-Policy/script-src
	// StyleSrc directive specifies valid sources for stylesheets.
	// For more information about the StyleSrc directive, see:
	// https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Content-Security-Policy/style-src
	// ImgSrc directive specifies a valid sources of images and favicons.
	// For more information about the ImgSrc directive, see:
	// https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Content-Security-Policy/img-src
	// FontSrc directive specifies valid sources for fonts loaded using @font-face.
	// For more information about the FontSrc directive, see:
	// https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Content-Security-Policy/font-src
	// ConnectSrc directive restricts the URLs which can be loaded using script interfaces.
	// For more information about the ConnectSrc directive, see:
	// https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Content-Security-Policy/connect-src
	// +required
	Directive DirectiveType `json:"directive"`
	// values defines an array of values to append to the console defaults for this directive.
	// Each ConsolePlugin may define their own directives with their values. These will be set
	// by the OpenShift web console's backend, as part of its Content-Security-Policy header.
	// The array can contain at most 16 values. Each directive value must have a maximum length
	// of 1024 characters and must not contain whitespace, commas (,), semicolons (;) or single
	// quotes ('). The value '*' is not permitted.
	// Each value in the array must be unique.
	//
	// +required
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=16
	// +kubebuilder:validation:XValidation:rule="self.all(x, self.exists_one(y, x == y))",message="each CSP directive value must be unique"
	// +listType=atomic
	Values []CSPDirectiveValue `json:"values"`
}

// LoadType is an enumeration of i18n loading types
// +kubebuilder:validation:Enum:=Preload;Lazy;""
type LoadType string

const (
	// Preload will load all plugin's localization resources during
	// loading of the plugin.
	Preload LoadType = "Preload"
	// Lazy wont preload any plugin's localization resources, instead
	// will leave thier loading to runtime's lazy-loading.
	Lazy LoadType = "Lazy"
	// Empty is the default value of the LoadType field and it's
	// purpose is to improve discoverability of the field. The
	// the behaviour is equivalent to Lazy type.
	Empty LoadType = ""
)

// ConsolePluginI18n holds information on localization resources that are served by
// the dynamic plugin.
type ConsolePluginI18n struct {
	// loadType indicates how the plugin's localization resource should be loaded.
	// Valid values are Preload, Lazy and the empty string.
	// When set to Preload, all localization resources are fetched when the plugin is loaded.
	// When set to Lazy, localization resources are lazily loaded as and when they are required by the console.
	// When omitted or set to the empty string, the behaviour is equivalent to Lazy type.
	// +required
	LoadType LoadType `json:"loadType"`
}

// ConsolePluginProxy holds information on various service types
// to which console's backend will proxy the plugin's requests.
type ConsolePluginProxy struct {
	// endpoint provides information about endpoint to which the request is proxied to.
	// +required
	Endpoint ConsolePluginProxyEndpoint `json:"endpoint"`
	// alias is a proxy name that identifies the plugin's proxy. An alias name
	// should be unique per plugin. The console backend exposes following
	// proxy endpoint:
	//
	// /api/proxy/plugin/<plugin-name>/<proxy-alias>/<request-path>?<optional-query-parameters>
	//
	// Request example path:
	//
	// /api/proxy/plugin/acm/search/pods?namespace=openshift-apiserver
	//
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=128
	// +kubebuilder:validation:Pattern=`^[A-Za-z0-9-_]+$`
	Alias string `json:"alias"`
	// caCertificate provides the cert authority certificate contents,
	// in case the proxied Service is using custom service CA.
	// By default, the service CA bundle provided by the service-ca operator is used.
	// +kubebuilder:validation:Pattern=`^-----BEGIN CERTIFICATE-----([\s\S]*)-----END CERTIFICATE-----\s?$`
	// +optional
	CACertificate string `json:"caCertificate,omitempty"`
	// authorization provides information about authorization type,
	// which the proxied request should contain
	// +kubebuilder:default:="None"
	// +optional
	Authorization AuthorizationType `json:"authorization,omitempty"`
}

// ConsolePluginProxyEndpoint holds information about the endpoint to which
// request will be proxied to.
// +union
type ConsolePluginProxyEndpoint struct {
	// type is the type of the console plugin's proxy. Currently only "Service" is supported.
	//
	// ---
	// + When handling unknown values, consumers should report an error and stop processing the plugin.
	//
	// +required
	// +unionDiscriminator
	Type ConsolePluginProxyType `json:"type"`
	// service is an in-cluster Service that the plugin will connect to.
	// The Service must use HTTPS. The console backend exposes an endpoint
	// in order to proxy communication between the plugin and the Service.
	// Note: service field is required for now, since currently only "Service"
	// type is supported.
	// +optional
	Service *ConsolePluginProxyServiceConfig `json:"service,omitempty"`
}

// ProxyType is an enumeration of available proxy types
// +kubebuilder:validation:Enum:=Service
type ConsolePluginProxyType string

const (
	// ProxyTypeService is used when proxying communication to a Service
	ProxyTypeService ConsolePluginProxyType = "Service"
)

// AuthorizationType is an enumerate of available authorization types
// +kubebuilder:validation:Enum:=UserToken;None
type AuthorizationType string

const (
	// UserToken indicates that the proxied request should contain the logged-in user's
	// OpenShift access token in the "Authorization" request header. For example:
	//
	// Authorization: Bearer sha256~kV46hPnEYhCWFnB85r5NrprAxggzgb6GOeLbgcKNsH0
	//
	UserToken AuthorizationType = "UserToken"
	// None indicates that proxied request wont contain authorization of any type.
	None AuthorizationType = "None"
)

// ProxyTypeServiceConfig holds information on Service to which
// console's backend will proxy the plugin's requests.
type ConsolePluginProxyServiceConfig struct {
	// name of Service that the plugin needs to connect to.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=128
	Name string `json:"name"`
	// namespace of Service that the plugin needs to connect to
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=128
	Namespace string `json:"namespace"`
	// port on which the Service that the plugin needs to connect to
	// is listening on.
	// +required
	// +kubebuilder:validation:Maximum:=65535
	// +kubebuilder:validation:Minimum:=1
	Port int32 `json:"port"`
}

// ConsolePluginBackendType is an enumeration of available backend types
// +kubebuilder:validation:Enum:=Service
type ConsolePluginBackendType string

const (
	// Service is used when plugin's backend is served by a Kubernetes Service
	Service ConsolePluginBackendType = "Service"
)

// ConsolePluginBackend holds information about the endpoint which serves
// the console's plugin
// +union
type ConsolePluginBackend struct {
	// type is the backend type which servers the console's plugin. Currently only "Service" is supported.
	//
	// ---
	// + When handling unknown values, consumers should report an error and stop processing the plugin.
	//
	// +required
	// +unionDiscriminator
	Type ConsolePluginBackendType `json:"type"`
	// service is a Kubernetes Service that exposes the plugin using a
	// deployment with an HTTP server. The Service must use HTTPS and
	// Service serving certificate. The console backend will proxy the
	// plugins assets from the Service using the service CA bundle.
	// +optional
	Service *ConsolePluginService `json:"service,omitempty"`
}

// ConsolePluginService holds information on Service that is serving
// console dynamic plugin assets.
type ConsolePluginService struct {
	// name of Service that is serving the plugin assets.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=128
	Name string `json:"name"`
	// namespace of Service that is serving the plugin assets.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=128
	Namespace string `json:"namespace"`
	// port on which the Service that is serving the plugin is listening to.
	// +required
	// +kubebuilder:validation:Maximum:=65535
	// +kubebuilder:validation:Minimum:=1
	Port int32 `json:"port"`
	// basePath is the path to the plugin's assets. The primary asset it the
	// manifest file called `plugin-manifest.json`, which is a JSON document
	// that contains metadata about the plugin and the extensions.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=256
	// +kubebuilder:validation:Pattern=`^[a-zA-Z0-9.\-_~!$&'()*+,;=:@\/]*$`
	// +kubebuilder:default:="/"
	// +optional
	BasePath string `json:"basePath,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +openshift:compatibility-gen:level=1

// Compatibility level 1: Stable within a major release for a minimum of 12 months or 3 minor releases (whichever is longer).
type ConsolePluginList struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard list's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ListMeta `json:"metadata"`

	Items []ConsolePlugin `json:"items"`
}
