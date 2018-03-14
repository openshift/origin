package controller

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"time"

	"github.com/golang/glog"

	utilwait "k8s.io/apimachinery/pkg/util/wait"
	apifilters "k8s.io/apiserver/pkg/endpoints/filters"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	apiserver "k8s.io/apiserver/pkg/server"
	apiserverfilters "k8s.io/apiserver/pkg/server/filters"
	"k8s.io/apiserver/pkg/server/healthz"
	genericmux "k8s.io/apiserver/pkg/server/mux"
	genericroutes "k8s.io/apiserver/pkg/server/routes"
	authzwebhook "k8s.io/apiserver/plugin/pkg/authorizer/webhook"
	clientgoclientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	"github.com/openshift/origin/pkg/cmd/server/crypto"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
)

// TODO make this an actual API server built on the genericapiserver
func RunControllerServer(servingInfo configapi.HTTPServingInfo, kubeExternal clientgoclientset.Interface) error {
	clientCAs, err := getClientCertCAPool(servingInfo)
	if err != nil {
		return err
	}

	mux := genericmux.NewPathRecorderMux("master-healthz")

	healthz.InstallHandler(mux, healthz.PingHealthz)
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
	// the request context mapper for controllers is always separate
	requestContextMapper := apirequest.NewRequestContextMapper()

	// we use direct bypass to allow readiness and health to work regardless of the master health
	authz := newBypassAuthorizer(remoteAuthz, "/healthz", "/healthz/ready")
	handler := apifilters.WithAuthorization(mux, requestContextMapper, authz, legacyscheme.Codecs)
	handler = apifilters.WithAuthentication(handler, requestContextMapper, authn, apifilters.Unauthorized(requestContextMapper, legacyscheme.Codecs, false))
	handler = apiserverfilters.WithPanicRecovery(handler)
	handler = apifilters.WithRequestInfo(handler, requestInfoResolver, requestContextMapper)
	handler = apirequest.WithRequestContext(handler, requestContextMapper)

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
func serveControllers(servingInfo configapi.HTTPServingInfo, handler http.Handler) error {
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

		extraCerts, err := configapi.GetNamedCertificateMap(servingInfo.NamedCertificates)
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
		glog.Fatal(cmdutil.ListenAndServeTLS(server, servingInfo.BindNetwork, servingInfo.ServerCert.CertFile, servingInfo.ServerCert.KeyFile))
	}, 0)

	return nil
}

func getClientCertCAPool(servingInfo configapi.HTTPServingInfo) (*x509.CertPool, error) {
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
