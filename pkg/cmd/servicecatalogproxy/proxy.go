package servicecatalogproxy

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/httpstream/spdy"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/authentication/authenticator"
	genericapifilters "k8s.io/apiserver/pkg/endpoints/filters"
	"k8s.io/apiserver/pkg/endpoints/handlers/responsewriters"
	genericrest "k8s.io/apiserver/pkg/registry/generic/rest"
	"k8s.io/apiserver/pkg/server"
	genericapiserver "k8s.io/apiserver/pkg/server"
	genericoptions "k8s.io/apiserver/pkg/server/options"
	kflag "k8s.io/apiserver/pkg/util/flag"
	"k8s.io/apiserver/plugin/pkg/authenticator/password/passwordfile"
	"k8s.io/apiserver/plugin/pkg/authenticator/request/basicauth"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/transport"
	"k8s.io/kubernetes/pkg/util/logs"
)

// ProxyRunOptions is a proxy that can terminate basic auth and attach the token
// from the incluster config to the exact same endpoint on the openshift apiserver.
// It only works inside of a pod.
type ProxyRunOptions struct {
	Out    io.Writer
	ErrOut io.Writer

	SecureServing *genericoptions.SecureServingOptions

	// PasswordFile is the file containing the content as a kube-compatible csv
	PasswordFile string
}

const proxyLong = `
Start proxy for the service catalog.

This command launches a proxy that terminates basic auth an attached a service account token.`

func NewCommand(name string, out, errout io.Writer) *cobra.Command {
	proxyOptions := ProxyRunOptions{
		Out:           out,
		ErrOut:        errout,
		SecureServing: genericoptions.NewSecureServingOptions(),
	}

	cmd := &cobra.Command{
		Use:   name,
		Short: "Launch proxy for the service catalog",
		Long:  proxyLong,
		Run: func(c *cobra.Command, args []string) {
			logs.InitLogs()
			defer logs.FlushLogs()

			if err := proxyOptions.Run(); err != nil {
				fmt.Fprintf(errout, "%v\n", err)
				os.Exit(1)
			}
		},
	}
	cmd.SetOutput(out)

	flags := cmd.Flags()
	proxyOptions.SecureServing.AddFlags(flags)
	flags.StringVar(&proxyOptions.PasswordFile, "basic-auth-file", "", "The file that will be used to admit requests to the proxy via http basic authentication.")

	flags.SetNormalizeFunc(kflag.WordSepNormalizeFunc)

	return cmd
}

func (p *ProxyRunOptions) Run() error {
	clientConfig, err := rest.InClusterConfig()
	if err != nil {
		return err
	}
	passwordAuthenticator, err := passwordfile.NewCSV(p.PasswordFile)
	if err != nil {
		return err
	}
	authenticator := basicauth.New(passwordAuthenticator)

	var handler http.Handler
	handler = &proxyHandler{
		clientConfig: clientConfig,
	}
	handler = WithAuthentication(handler, authenticator, genericapifilters.Unauthorized(true))

	secureServingInfo := &server.SecureServingInfo{
		BindAddress: net.JoinHostPort(p.SecureServing.BindAddress.String(), strconv.Itoa(p.SecureServing.BindPort)),
	}

	serverCertFile, serverKeyFile := p.SecureServing.ServerCert.CertKey.CertFile, p.SecureServing.ServerCert.CertKey.KeyFile

	// load main cert
	if len(serverCertFile) != 0 || len(serverKeyFile) != 0 {
		tlsCert, err := tls.LoadX509KeyPair(serverCertFile, serverKeyFile)
		if err != nil {
			return fmt.Errorf("unable to load server certificate: %v", err)
		}
		secureServingInfo.Cert = &tlsCert
	}

	// load SNI certs
	namedTLSCerts := make([]server.NamedTLSCert, 0, len(p.SecureServing.SNICertKeys))
	for _, nck := range p.SecureServing.SNICertKeys {
		tlsCert, err := tls.LoadX509KeyPair(nck.CertFile, nck.KeyFile)
		namedTLSCerts = append(namedTLSCerts, server.NamedTLSCert{
			TLSCert: tlsCert,
			Names:   nck.Names,
		})
		if err != nil {
			return fmt.Errorf("failed to load SNI cert and key: %v", err)
		}
	}
	secureServingInfo.SNICerts, err = server.GetNamedCertificateMap(namedTLSCerts)
	if err != nil {
		return err
	}

	secureServer := &http.Server{
		Addr:           secureServingInfo.BindAddress,
		Handler:        handler,
		MaxHeaderBytes: 1 << 20,
		TLSConfig: &tls.Config{
			NameToCertificate: secureServingInfo.SNICerts,
			// Can't use SSLv3 because of POODLE and BEAST
			// Can't use TLSv1.0 because of POODLE and BEAST using CBC cipher
			// Can't use TLSv1.1 because of RC4 cipher usage
			MinVersion: tls.VersionTLS12,
			// enable HTTP2 for go's 1.7 HTTP Server
			NextProtos: []string{"h2", "http/1.1"},
		},
	}

	if secureServingInfo.MinTLSVersion > 0 {
		secureServer.TLSConfig.MinVersion = secureServingInfo.MinTLSVersion
	}
	if len(secureServingInfo.CipherSuites) > 0 {
		secureServer.TLSConfig.CipherSuites = secureServingInfo.CipherSuites
	}

	if secureServingInfo.Cert != nil {
		secureServer.TLSConfig.Certificates = []tls.Certificate{*secureServingInfo.Cert}
	}

	// append all named certs. Otherwise, the go tls stack will think no SNI processing
	// is necessary because there is only one cert anyway.
	// Moreover, if ServerCert.CertFile/ServerCert.KeyFile are not set, the first SNI
	// cert will become the default cert. That's what we expect anyway.
	for _, c := range secureServingInfo.SNICerts {
		secureServer.TLSConfig.Certificates = append(secureServer.TLSConfig.Certificates, *c)
	}

	if secureServingInfo.ClientCA != nil {
		// Populate PeerCertificates in requests, but don't reject connections without certificates
		// This allows certificates to be validated by authenticators, while still allowing other auth types
		secureServer.TLSConfig.ClientAuth = tls.RequestClientCert
		// Specify allowed CAs for client certificates
		secureServer.TLSConfig.ClientCAs = secureServingInfo.ClientCA
	}

	glog.Infof("Serving securely on %s", secureServingInfo.BindAddress)
	_, err = genericapiserver.RunServer(secureServer, secureServingInfo.BindNetwork, wait.NeverStop)
	if err != nil {
		return err
	}
	<-wait.NeverStop
	return nil
}

func WithAuthentication(handler http.Handler, auth authenticator.Request, failed http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		_, ok, err := auth.AuthenticateRequest(req)
		if err != nil || !ok {
			if err != nil {
				glog.Errorf("Unable to authenticate the request due to an error: %v", err)
			}
			failed.ServeHTTP(w, req)
			return
		}

		// authorization header is not required anymore in case of a successful authentication.
		req.Header.Del("Authorization")

		handler.ServeHTTP(w, req)
	})
}

type proxyHandler struct {
	clientConfig *rest.Config
}

func (p *proxyHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	fmt.Printf("#### proxying %v\n", req.URL.Path)
	proxyRoundTripper, err := rest.TransportFor(p.clientConfig)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// write a new location based on the existing request pointed at the target service
	location := &url.URL{}
	location.Scheme = "https"
	location.Host = p.clientConfig.Host
	location.Path = req.URL.Path
	location.RawQuery = req.URL.Query().Encode()

	fmt.Printf("#### proxying to %v\n", location)

	// make a new request object with the updated location and the body we already have
	newReq, err := http.NewRequest(req.Method, location.String(), req.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	mergeHeader(newReq.Header, req.Header)
	newReq.ContentLength = req.ContentLength
	// Copy the TransferEncoding is for future-proofing. Currently Go only supports "chunked" and
	// it can determine the TransferEncoding based on ContentLength and the Body.
	newReq.TransferEncoding = req.TransferEncoding

	upgrade := false
	// we need to wrap the roundtripper in another roundtripper which will apply the front proxy headers
	proxyRoundTripper, upgrade, err = maybeWrapForConnectionUpgrades(p.clientConfig, proxyRoundTripper, req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	proxyRoundTripper = transport.NewBearerAuthRoundTripper(p.clientConfig.BearerToken, proxyRoundTripper)

	// if we are upgrading, then the upgrade path tries to use this request with the TLS config we provide, but it does
	// NOT use the roundtripper.  Its a direct call that bypasses the round tripper.  This means that we have to
	// attach the "correct" user headers to the request ahead of time.  After the initial upgrade, we'll be back
	// at the roundtripper flow, so we only have to muck with this request, but we do have to do it.
	if upgrade {
		newReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.clientConfig.BearerToken))
	}

	handler := genericrest.NewUpgradeAwareProxyHandler(location, proxyRoundTripper, true, upgrade, &responder{w: w})
	handler.ServeHTTP(w, newReq)
}

// maybeWrapForConnectionUpgrades wraps the roundtripper for upgrades.  The bool indicates if it was wrapped
func maybeWrapForConnectionUpgrades(restConfig *rest.Config, rt http.RoundTripper, req *http.Request) (http.RoundTripper, bool, error) {
	connectionHeader := req.Header.Get("Connection")
	if len(connectionHeader) == 0 {
		return rt, false, nil
	}

	tlsConfig, err := rest.TLSConfigFor(restConfig)
	if err != nil {
		return nil, true, err
	}
	upgradeRoundTripper := spdy.NewRoundTripper(tlsConfig)
	wrappedRT, err := rest.HTTPWrappersForConfig(restConfig, upgradeRoundTripper)
	if err != nil {
		return nil, true, err
	}

	return wrappedRT, true, nil
}

func mergeHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

// responder implements rest.Responder for assisting a connector in writing objects or errors.
type responder struct {
	w http.ResponseWriter
}

// TODO this should properly handle content type negotiation
// if the caller asked for protobuf and you write JSON bad things happen.
func (r *responder) Object(statusCode int, obj runtime.Object) {
	responsewriters.WriteRawJSON(statusCode, obj, r.w)
}

func (r *responder) Error(err error) {
	http.Error(r.w, err.Error(), http.StatusInternalServerError)
}
