package start

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/elazarl/goproxy"
	"github.com/golang/glog"
	"github.com/spf13/cobra"

	kapi "k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/client/restclient"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/runtime"
	kcrypto "k8s.io/kubernetes/pkg/util/crypto"
	"k8s.io/kubernetes/pkg/util/sets"

	"github.com/openshift/origin/pkg/auth/authenticator"
	"github.com/openshift/origin/pkg/auth/authenticator/anonymous"
	"github.com/openshift/origin/pkg/auth/authenticator/request/bearertoken"
	"github.com/openshift/origin/pkg/auth/authenticator/request/unionrequest"
	"github.com/openshift/origin/pkg/auth/authenticator/request/x509request"
	authncache "github.com/openshift/origin/pkg/auth/authenticator/token/cache"
	authnremote "github.com/openshift/origin/pkg/auth/authenticator/token/remotetokenreview"
	"github.com/openshift/origin/pkg/auth/group"
	authzapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/authorizer"
	authzcache "github.com/openshift/origin/pkg/authorization/authorizer/cache"
	authzremote "github.com/openshift/origin/pkg/authorization/authorizer/remote"
	"github.com/openshift/origin/pkg/client"
	oclient "github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/flagtypes"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	oclientcmd "github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

type proxyFlags struct {
	ListenArg    ListenArg
	KubeConfig   string
	ClientCA     string
	CertFile     string
	KeyFile      string
	MITMCertFile string
	MITMKeyFile  string

	AsPod         bool
	Unprivileged  bool
	PublicAddress string
}

type ProxyOptions struct {
	ServingInfo          configapi.ServingInfo
	KubeClient           internalclientset.Interface
	OpenShiftClient      oclient.Interface
	RequestContextMapper kapi.RequestContextMapper

	AuthenticationCacheTTL  time.Duration
	AuthenticationCacheSize int
	AuthorizationCacheTTL   time.Duration
	AuthorizationCacheSize  int

	MITMSigner tls.Certificate

	AnonymousConfig restclient.Config
	Unprivileged    bool
	PublicAddress   string
}

const proxyLong = `Start the service proxy to allow safe, authenticated access to services without a route.`

// NewCommandStartProxy starts only the APIserver
func NewCommandStartProxy(name, basename string, out io.Writer) *cobra.Command {
	f := &proxyFlags{
		ListenArg: ListenArg{
			ListenAddr: flagtypes.Addr{
				Value:         "0.0.0.0:8445",
				DefaultScheme: "https",
				DefaultPort:   8445,
				AllowPrefix:   true,
			}.Default(),
		},
	}

	cmd := &cobra.Command{
		Use:   name,
		Short: "Launch service proxy.",
		Long:  fmt.Sprintf(proxyLong, basename, name),
		Run: func(c *cobra.Command, args []string) {
			kcmdutil.CheckErr(f.Validate())
			o, err := f.ToOptions()
			kcmdutil.CheckErr(err)
			kcmdutil.CheckErr(o.RunProxy())
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&f.PublicAddress, "public-address", f.PublicAddress, "Public address for PAC file.")
	flags.BoolVar(&f.AsPod, "as-pod", f.AsPod, "Indicates that this proxy should run as an unprivileged pod.")
	flags.BoolVar(&f.Unprivileged, "unprivileged", f.Unprivileged, "Indicates that this proxy should run as an unprivileged pod.")
	flags.StringVar(&f.KubeConfig, "kubeconfig", f.KubeConfig, "Location of the kubeconfig file to for contacting the API server. Required")
	flags.StringVar(&f.ClientCA, "client-ca-file", f.ClientCA, ""+
		"If set, any request presenting a client certificate signed by one of "+
		"the authorities in the client-ca-file is authenticated with an identity "+
		"corresponding to the CommonName of the client certificate.")
	flags.StringVar(&f.CertFile, "tls-cert-file", f.CertFile, ""+
		"File containing x509 Certificate for HTTPS. (CA cert, if any, concatenated "+
		"after server cert). If HTTPS serving is enabled, and --tls-cert-file and "+
		"--tls-private-key-file are not provided, a self-signed certificate and key "+
		"are generated for the public address and saved to /var/run/kubernetes.")
	flags.StringVar(&f.KeyFile, "tls-private-key-file", f.KeyFile, "File containing x509 private key matching --tls-cert-file.")
	flags.StringVar(&f.MITMCertFile, "mitm-cert-file", f.MITMCertFile, "Key pair for signing the MITM attack")
	flags.StringVar(&f.MITMKeyFile, "mitm-key-file", f.MITMKeyFile, "Key pair for signing the MITM attack")

	BindListenArg(&f.ListenArg, flags, "")

	return cmd
}

func (f proxyFlags) ToOptions() (*ProxyOptions, error) {
	o := &ProxyOptions{
		AuthenticationCacheTTL:  5 * time.Minute,
		AuthenticationCacheSize: 0,
		AuthorizationCacheTTL:   5 * time.Minute,
		AuthorizationCacheSize:  0,
		RequestContextMapper:    kapi.NewRequestContextMapper(),
		PublicAddress:           f.PublicAddress,
		Unprivileged:            f.Unprivileged,
	}

	var kubeconfig *restclient.Config
	var err error

	if !f.AsPod {
		_, kubeconfig, err = configapi.GetKubeClient(f.KubeConfig, &configapi.ClientConnectionOverrides{QPS: 100, Burst: 200})
		if err != nil {
			return nil, err
		}
	} else {
		kubeconfig, err = restclient.InClusterConfig()
		if err != nil {
			return nil, err
		}
	}
	kubeconfig.QPS = 100
	kubeconfig.Burst = 200
	o.AnonymousConfig = oclientcmd.AnonymousClientConfig(kubeconfig)

	o.KubeClient, err = internalclientset.NewForConfig(kubeconfig)
	if err != nil {
		return nil, err
	}
	o.OpenShiftClient, err = client.New(kubeconfig)
	if err != nil {
		return nil, err
	}

	o.ServingInfo = servingInfoForAddr(&f.ListenArg.ListenAddr)
	o.ServingInfo.ServerCert.CertFile = f.CertFile
	o.ServingInfo.ServerCert.KeyFile = f.KeyFile
	o.ServingInfo.ClientCA = f.ClientCA

	cert, err := ioutil.ReadFile(f.MITMCertFile)
	if err != nil {
		return nil, err
	}
	key, err := ioutil.ReadFile(f.MITMKeyFile)
	if err != nil {
		return nil, err
	}
	o.MITMSigner, err = tls.X509KeyPair(cert, key)
	if err != nil {
		return nil, err
	}

	return o, nil
}

func (f proxyFlags) Validate() error {
	if f.AsPod {
		if len(f.KubeConfig) != 0 {
			return fmt.Errorf("has kubeconfig")
		}
	} else {
		if len(f.KubeConfig) == 0 {
			return fmt.Errorf("missing kubeconfig")
		}
	}

	if len(f.CertFile) == 0 {
		return fmt.Errorf("missing tls-cert-file")
	}
	if len(f.KeyFile) == 0 {
		return fmt.Errorf("missing tls-key-file")
	}
	if len(f.MITMCertFile) == 0 {
		return fmt.Errorf("missing mitm-cert-file")
	}
	if len(f.MITMKeyFile) == 0 {
		return fmt.Errorf("missing mitm-key-file")
	}
	return nil
}

func (o *ProxyOptions) RunProxy() error {
	var handler http.Handler
	var err error

	proxy := goproxy.NewProxyHttpServer()
	proxy.Verbose = true
	proxy.OnRequest().HandleConnect(goproxy.FuncHttpsHandler(o.mitm))

	handler = proxy
	handler = o.withAuthorization(handler)
	handler = o.withAuthentication(handler)
	handler = o.withPAC(handler)
	handler, err = kapi.NewRequestContextFilter(o.RequestContextMapper, handler)
	if err != nil {
		return err
	}

	secureServer := &http.Server{
		Addr:           o.ServingInfo.BindAddress,
		Handler:        handler,
		MaxHeaderBytes: 1 << 20,
		// this forces us to use http1 or 1.1 which allows hijacking for the ssl bump
		TLSNextProto: map[string]func(*http.Server, *tls.Conn, http.Handler){},
		// if false
		TLSConfig: &tls.Config{
			// Can't use SSLv3 because of POODLE and BEAST
			// Can't use TLSv1.0 because of POODLE and BEAST using CBC cipher
			// Can't use TLSv1.1 because of RC4 cipher usage
			MinVersion: tls.VersionTLS12,
		},
	}

	if len(o.ServingInfo.ClientCA) > 0 {
		clientCAs, err := NewPool(o.ServingInfo.ClientCA)
		if err != nil {
			glog.Fatalf("Unable to load client CA file: %v", err)
		}
		// Populate PeerCertificates in requests, but don't reject connections without certificates
		// This allows certificates to be validated by authenticators, while still allowing other auth types
		secureServer.TLSConfig.ClientAuth = tls.RequestClientCert
		// Specify allowed CAs for client certificates
		secureServer.TLSConfig.ClientCAs = clientCAs
	}

	glog.Infof("Serving securely on %s", o.ServingInfo.BindAddress)
	if err := secureServer.ListenAndServeTLS(o.ServingInfo.ServerCert.CertFile, o.ServingInfo.ServerCert.KeyFile); err != nil {
		return err
	}

	return nil
}

func (o *ProxyOptions) withPAC(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/pac" {
			w.Write([]byte(`function FindProxyForURL(url, host) { 
	if (shExpMatch(host, "*.svc"))
		return "HTTPS ` + o.PublicAddress + `"; 
	if (shExpMatch(host, "*.svc.cluster.local"))
		return "HTTPS ` + o.PublicAddress + `"; 

	return "DIRECT";
}`))
			w.WriteHeader(http.StatusOK)
			return
		}

		handler.ServeHTTP(w, req)
	})
}

// mitm is very very naughty.  This is terminating SSL here, decrypting traffic, making a connection on your behalf
// with full knowledge of you payload data.  This is trusting the proxy with all incoming data and it effectively delegates
// trust of remote certificates to the proxy.  It can be used to steal all manner of "private" communications.
// Abusing this power is immoral.  Be good.
func (o ProxyOptions) mitm(host string, ctx *goproxy.ProxyCtx) (*goproxy.ConnectAction, string) {
	return &goproxy.ConnectAction{
		Action:    goproxy.ConnectMitm,
		TLSConfig: goproxy.TLSConfigFromCA(&o.MITMSigner),
	}, host
}

func (o ProxyOptions) withAuthentication(handler http.Handler) http.Handler {
	if o.Unprivileged {
		return handler
	}
	authenticator, err := o.newAuthenticator()
	if err != nil {
		panic(err)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		user, ok, err := authenticator.AuthenticateRequest(req)
		if err != nil || !ok {
			http.Error(w, "Unauthorized", http.StatusProxyAuthRequired)
			return
		}

		ctx, ok := o.RequestContextMapper.Get(req)
		if !ok {
			http.Error(w, "Unable to find request context", http.StatusInternalServerError)
			return
		}
		if err := o.RequestContextMapper.Update(req, kapi.WithUser(ctx, user)); err != nil {
			glog.V(4).Infof("Error setting authenticated context: %v", err)
			http.Error(w, "Unable to set authenticated request context", http.StatusInternalServerError)
			return
		}

		handler.ServeHTTP(w, req)
	})
}

func (o ProxyOptions) newAuthenticator() (authenticator.Request, error) {
	var err error
	authenticators := []authenticator.Request{}

	// Authenticate against the remote master
	var tokenAuthenticator authenticator.Token
	tokenAuthenticator, err = authnremote.NewAuthenticator(o.KubeClient.Authentication())
	if err != nil {
		return nil, err
	}
	// Cache results
	if o.AuthenticationCacheTTL > 0 && o.AuthenticationCacheSize > 0 {
		tokenAuthenticator, err = authncache.NewAuthenticator(tokenAuthenticator, o.AuthenticationCacheTTL, o.AuthenticationCacheSize)
		if err != nil {
			return nil, err
		}
	}
	authenticators = append(authenticators, bearertoken.NewProxy(tokenAuthenticator, true))

	// Client-cert auth
	if len(o.ServingInfo.ClientCA) > 0 {
		clientCAs, err := kcrypto.CertPoolFromFile(o.ServingInfo.ClientCA)
		if err != nil {
			return nil, err
		}

		opts := x509request.DefaultVerifyOptions()
		opts.Roots = clientCAs
		certauth := x509request.New(opts, x509request.SubjectToUserConversion)
		authenticators = append(authenticators, certauth)
	}

	ret := &unionrequest.Authenticator{
		// Anonymous requests will pass the token and cert checks without errors
		// Bad tokens or bad certs will produce errors, in which case we should not continue to authenticate them as "system:anonymous"
		FailOnError: true,
		Handlers: []authenticator.Request{
			// Add the "system:authenticated" group to users that pass token/cert authentication
			group.NewGroupAdder(unionrequest.NewUnionAuthentication(authenticators...), []string{bootstrappolicy.AuthenticatedGroup}),
			// Fall back to the "system:anonymous" user
			anonymous.NewAuthenticator(),
		},
	}

	return ret, nil
}

func (o *ProxyOptions) withAuthorization(handler http.Handler) http.Handler {
	authorizer, err := o.newAuthorizer()
	if err != nil {
		panic(err)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		attributes, namespace, err := getAuthorizationAttributes(req)
		if err != nil {
			forbidden(err.Error(), attributes, w, req)
			return
		}
		if len(namespace) == 0 {
			forbidden("No namespace", attributes, w, req)
			return
		}
		if attributes == nil {
			forbidden("No attributes", attributes, w, req)
			return
		}

		ctx, exists := o.RequestContextMapper.Get(req)
		if !exists {
			forbidden("context not found", attributes, w, req)
			return
		}
		ctx = kapi.WithNamespace(ctx, namespace)
		bearer := req.Header.Get("Proxy-Authorization")
		if len(bearer) > 0 {
			bearer = strings.Split(bearer, " ")[1]
		}
		fmt.Printf("#### bearer: %q\n", bearer)
		ctx = kapi.WithValue(ctx, "proxybearer", bearer)

		allowed, reason, err := authorizer.Authorize(ctx, attributes)
		if err != nil {
			forbidden(err.Error(), attributes, w, req)
			return
		}
		if !allowed {
			forbidden(reason, attributes, w, req)
			return
		}

		handler.ServeHTTP(w, req)
	})
}

// forbidden renders a simple forbidden error
func forbidden(reason string, attributes authorizer.Action, w http.ResponseWriter, req *http.Request) {
	kind := ""
	resource := ""
	group := ""
	name := ""
	// the attributes can be empty for two basic reasons:
	// 1. malformed API request
	// 2. not an API request at all
	// In these cases, just assume default that will work better than nothing
	if attributes != nil {
		group = attributes.GetAPIGroup()
		resource = attributes.GetResource()
		kind = attributes.GetResource()
		if len(attributes.GetAPIGroup()) > 0 {
			kind = attributes.GetAPIGroup() + "." + kind
		}
		name = attributes.GetResourceName()
	}

	// Reason is an opaque string that describes why access is allowed or forbidden (forbidden by the time we reach here).
	// We don't have direct access to kind or name (not that those apply either in the general case)
	// We create a NewForbidden to stay close the API, but then we override the message to get a serialization
	// that makes sense when a human reads it.
	forbiddenError := kapierrors.NewForbidden(unversioned.GroupResource{Group: group, Resource: resource}, name, errors.New("") /*discarded*/)
	forbiddenError.ErrStatus.Message = reason

	formatted := &bytes.Buffer{}
	output, err := runtime.Encode(kapi.Codecs.LegacyCodec(kapi.SchemeGroupVersion), &forbiddenError.ErrStatus)
	if err != nil {
		fmt.Fprintf(formatted, "%s", forbiddenError.Error())
	} else {
		json.Indent(formatted, output, "", "  ")
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	w.Write(formatted.Bytes())
}

func getAuthorizationAttributes(req *http.Request) (authorizer.Action, string, error) {
	var err error
	hostPort := req.Host
	absRequestURI := strings.HasPrefix(req.RequestURI, "http://") || strings.HasPrefix(req.RequestURI, "https://")
	if !absRequestURI {
		hostPort = req.Host
		if len(hostPort) == 0 && req.URL != nil {
			hostPort = req.URL.Host
		}
	}
	host := hostPort
	if strings.Contains(hostPort, ":") {
		host, _, err = net.SplitHostPort(hostPort)
		if err != nil {
			return nil, "", err
		}
	}

	tokens := strings.Split(host, ".")
	if len(tokens) < 3 {
		return nil, "", fmt.Errorf("not enough tokens in %v")
	}

	// matches normal resource evaluation verbs
	verb := ""
	switch req.Method {
	case "POST":
		verb = "create"
	case "GET", "HEAD":
		verb = "get"
	case "PUT":
		verb = "update"
	case "PATCH":
		verb = "patch"
	case "DELETE":
		verb = "delete"
	default:
		verb = ""
	}

	return authorizer.DefaultAuthorizationAttributes{
		Verb:         verb,
		APIGroup:     "",
		Resource:     "services/proxy",
		ResourceName: tokens[0],
	}, tokens[1], nil
}

func (o *ProxyOptions) newAuthorizer() (authorizer.Authorizer, error) {
	var (
		authz authorizer.Authorizer
		err   error
	)

	if o.Unprivileged {
		authz = newSelfSARAuthorizer(o.AnonymousConfig)

	} else {
		// Authorize against the remote master
		// unprivileged pods won't be able to run token access review or non-self-sar
		authz, err = authzremote.NewAuthorizer(o.OpenShiftClient)
	}
	if err != nil {
		return nil, err
	}

	// Cache results
	if o.AuthorizationCacheTTL > 0 && o.AuthorizationCacheSize > 0 {
		authz, err = authzcache.NewAuthorizer(authz, o.AuthorizationCacheTTL, o.AuthorizationCacheSize)
		if err != nil {
			return nil, err
		}
	}

	return authz, nil
}

// NewPool returns an x509.CertPool containing the certificates in the given PEM-encoded file.
// Returns an error if the file could not be read, a certificate could not be parsed, or if the file does not contain any certificates
func NewPool(filename string) (*x509.CertPool, error) {
	certs, err := certsFromFile(filename)
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	for _, cert := range certs {
		pool.AddCert(cert)
	}
	return pool, nil
}

// certsFromFile returns the x509.Certificates contained in the given PEM-encoded file.
// Returns an error if the file could not be read, a certificate could not be parsed, or if the file does not contain any certificates
func certsFromFile(file string) ([]*x509.Certificate, error) {
	if len(file) == 0 {
		return nil, errors.New("error reading certificates from an empty filename")
	}
	pemBlock, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}
	certs, err := ParseCertsPEM(pemBlock)
	if err != nil {
		return nil, fmt.Errorf("error reading %s: %s", file, err)
	}
	return certs, nil
}

// ParseCertsPEM returns the x509.Certificates contained in the given PEM-encoded byte array
// Returns an error if a certificate could not be parsed, or if the data does not contain any certificates
func ParseCertsPEM(pemCerts []byte) ([]*x509.Certificate, error) {
	ok := false
	certs := []*x509.Certificate{}
	for len(pemCerts) > 0 {
		var block *pem.Block
		block, pemCerts = pem.Decode(pemCerts)
		if block == nil {
			break
		}
		// Only use PEM "CERTIFICATE" blocks without extra headers
		if block.Type != "CERTIFICATE" || len(block.Headers) != 0 {
			continue
		}

		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return certs, err
		}

		certs = append(certs, cert)
		ok = true
	}

	if !ok {
		return certs, errors.New("could not read any certificates")
	}
	return certs, nil
}

// SelfSARAuthorizer provides authorization using subject access review and resource access review requests
type SelfSARAuthorizer struct {
	anonymousConfig restclient.Config
}

func newSelfSARAuthorizer(anonymousConfig restclient.Config) authorizer.Authorizer {
	return &SelfSARAuthorizer{anonymousConfig}
}

func (r *SelfSARAuthorizer) Authorize(ctx kapi.Context, a authorizer.Action) (bool, string, error) {
	var (
		result *authzapi.SubjectAccessReviewResponse
		err    error
	)

	bearertoken := ctx.Value("proxybearer").(string)
	fmt.Printf("#### bearertoken: %q\n", bearertoken)
	config := r.anonymousConfig
	config.BearerToken = bearertoken
	// TODO plumb something on request to include bearer token if we want to allow unprivileged
	client, err := oclient.New(&config)
	if err != nil {
		glog.Errorf("error make client: %v", err)
		return false, "", kapierrors.NewInternalError(err)
	}

	// Extract namespace from context
	namespace, _ := kapi.NamespaceFrom(ctx)

	if len(namespace) > 0 {
		result, err = client.LocalSubjectAccessReviews(namespace).Create(
			&authzapi.LocalSubjectAccessReview{Action: getAction(namespace, a)})
	} else {
		result, err = client.SubjectAccessReviews().Create(
			&authzapi.SubjectAccessReview{Action: getAction(namespace, a)})
	}

	if err != nil {
		glog.Errorf("error running subject access review: %v", err)
		return false, "", kapierrors.NewInternalError(err)
	}
	glog.V(2).Infof("allowed=%v, reason=%s", result.Allowed, result.Reason)
	return result.Allowed, result.Reason, nil
}

func (r *SelfSARAuthorizer) GetAllowedSubjects(ctx kapi.Context, attributes authorizer.Action) (sets.String, sets.String, error) {
	return nil, nil, nil
}

func getAction(namespace string, attributes authorizer.Action) authzapi.Action {
	return authzapi.Action{
		Namespace:    namespace,
		Verb:         attributes.GetVerb(),
		Group:        attributes.GetAPIGroup(),
		Version:      attributes.GetAPIVersion(),
		Resource:     attributes.GetResource(),
		ResourceName: attributes.GetResourceName(),

		// TODO: missing from authorizer.Action:
		// Content

		// TODO: missing from authzapi.Action
		// RequestAttributes (unserializable?)
		// IsNonResourceURL
		// URL (doesn't make sense for remote authz?)
	}
}
