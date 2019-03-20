package controller

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"time"

	"github.com/golang/glog"

	utilwait "k8s.io/apimachinery/pkg/util/wait"
	apifilters "k8s.io/apiserver/pkg/endpoints/filters"
	apiserver "k8s.io/apiserver/pkg/server"
	apiserverfilters "k8s.io/apiserver/pkg/server/filters"
	"k8s.io/apiserver/pkg/server/healthz"
	genericmux "k8s.io/apiserver/pkg/server/mux"
	genericroutes "k8s.io/apiserver/pkg/server/routes"
	authzwebhook "k8s.io/apiserver/plugin/pkg/authorizer/webhook"
	clientgoclientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/library-go/pkg/crypto"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
)

// TODO make this an actual API server built on the genericapiserver
func RunControllerServer(servingInfo configv1.HTTPServingInfo, kubeExternal clientgoclientset.Interface) error {
	clientCAs, err := getClientCertCAPool(servingInfo)
	if err != nil {
		return err
	}

	mux := genericmux.NewPathRecorderMux("master-healthz")

	healthz.InstallHandler(mux, healthz.PingHealthz, healthz.LogHealthz)
	initReadinessCheckRoute(mux, "/healthz/ready", func() bool { return true })
	genericroutes.Profiling{}.Install(mux)
	genericroutes.MetricsWithReset{}.Install(mux)

	// TODO: replace me with a service account for controller manager
	tokenReview := kubeExternal.AuthenticationV1beta1().TokenReviews()
	authn, err := newRemoteAuthenticator(tokenReview, clientCAs, 5*time.Minute)
	if err != nil {
		return err
	}
	sarClient := kubeExternal.AuthorizationV1beta1().SubjectAccessReviews()
	remoteAuthz, err := authzwebhook.NewFromInterface(sarClient, 5*time.Minute, 5*time.Minute)
	if err != nil {
		return err
	}

	// requestInfoFactory for controllers only needs to be able to handle non-API endpoints
	requestInfoResolver := apiserver.NewRequestInfoResolver(&apiserver.Config{})

	// we use direct bypass to allow readiness and health to work regardless of the master health
	authz := newBypassAuthorizer(remoteAuthz, "/healthz", "/healthz/ready")
	handler := apifilters.WithAuthorization(mux, authz, legacyscheme.Codecs)
	// TODO need audiences
	handler = apifilters.WithAuthentication(handler, authn, apifilters.Unauthorized(legacyscheme.Codecs, false), nil)
	handler = apiserverfilters.WithPanicRecovery(handler)
	handler = apifilters.WithRequestInfo(handler, requestInfoResolver)

	return serveControllers(servingInfo, handler)
}

// initReadinessCheckRoute initializes an HTTP endpoint for readiness checking
// TODO this looks pointless
func initReadinessCheckRoute(mux *genericmux.PathRecorderMux, path string, readyFunc func() bool) {
	mux.HandleFunc(path, func(w http.ResponseWriter, req *http.Request) {
		if readyFunc() {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ok"))

		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	})
}

// serve starts serving the provided http.Handler using security settings derived from the MasterConfig
func serveControllers(servingInfo configv1.HTTPServingInfo, handler http.Handler) error {
	timeout := servingInfo.RequestTimeoutSeconds
	if timeout == -1 {
		timeout = 0
	}

	server := &http.Server{
		Addr:           servingInfo.BindAddress,
		Handler:        handler,
		ReadTimeout:    time.Duration(timeout) * time.Second,
		WriteTimeout:   time.Duration(timeout) * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	clientCAs, err := getClientCertCAPool(servingInfo)
	if err != nil {
		return err
	}

	go utilwait.Forever(func() {
		glog.Infof("Started health checks at %s", servingInfo.BindAddress)

		extraCerts, err := getNamedCertificateMap(servingInfo.NamedCertificates)
		if err != nil {
			glog.Fatal(err)
		}
		server.TLSConfig = crypto.SecureTLSConfig(&tls.Config{
			// Populate PeerCertificates in requests, but don't reject connections without certificates
			// This allows certificates to be validated by authenticators, while still allowing other auth types
			ClientAuth: tls.RequestClientCert,
			ClientCAs:  clientCAs,
			// Set SNI certificate func
			GetCertificate: cmdutil.GetCertificateFunc(extraCerts),
			MinVersion:     crypto.TLSVersionOrDie(servingInfo.MinTLSVersion),
			CipherSuites:   crypto.CipherSuitesOrDie(servingInfo.CipherSuites),
		})
		glog.Fatal(cmdutil.ListenAndServeTLS(server, servingInfo.BindNetwork, servingInfo.CertFile, servingInfo.KeyFile))
	}, 0)

	return nil
}

func getClientCertCAPool(servingInfo configv1.HTTPServingInfo) (*x509.CertPool, error) {
	roots := x509.NewCertPool()
	// Add CAs for API
	certs, err := cmdutil.CertificatesFromFile(servingInfo.ClientCA)
	if err != nil {
		return nil, err
	}
	for _, root := range certs {
		roots.AddCert(root)
	}

	return roots, nil
}

// getNamedCertificateMap returns a map of strings to *tls.Certificate, suitable for use in tls.Config#NamedCertificates
// Returns an error if any of the certs cannot be loaded, or do not match the configured name
// Returns nil if len(namedCertificates) == 0
func getNamedCertificateMap(namedCertificates []configv1.NamedCertificate) (map[string]*tls.Certificate, error) {
	if len(namedCertificates) == 0 {
		return nil, nil
	}
	namedCerts := map[string]*tls.Certificate{}
	for _, namedCertificate := range namedCertificates {
		cert, err := tls.LoadX509KeyPair(namedCertificate.CertFile, namedCertificate.KeyFile)
		if err != nil {
			return nil, err
		}
		for _, name := range namedCertificate.Names {
			namedCerts[name] = &cert
		}
	}
	return namedCerts, nil
}
