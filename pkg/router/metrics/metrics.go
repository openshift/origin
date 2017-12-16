package metrics

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	"strings"

	"github.com/cockroachdb/cmux"
	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"

	"k8s.io/apiserver/pkg/server/healthz"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apiserver/pkg/authentication/authenticator"
	"k8s.io/apiserver/pkg/authorization/authorizer"
)

type Listener struct {
	Addr string

	TLSConfig *tls.Config

	Username string
	Password string

	Authenticator authenticator.Request
	Authorizer    authorizer.Authorizer
	Record        authorizer.AttributesRecord

	Checks []healthz.HealthzChecker
}

func (l Listener) handler() http.Handler {
	mux := http.NewServeMux()
	healthz.InstallHandler(mux, l.Checks...)

	if l.Authenticator != nil {
		protected := http.NewServeMux()
		protected.HandleFunc("/debug/pprof/", pprof.Index)
		protected.HandleFunc("/debug/pprof/profile", pprof.Profile)
		protected.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		protected.Handle("/metrics", prometheus.Handler())
		mux.Handle("/", l.authorizeHandler(protected))
	}
	return mux
}

func (l Listener) authorizeHandler(protected http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if len(l.Username) > 0 || len(l.Password) > 0 {
			if u, p, ok := req.BasicAuth(); ok {
				if u == l.Username && p == l.Password {
					protected.ServeHTTP(w, req)
				} else {
					http.Error(w, fmt.Sprintf("Unauthorized"), http.StatusUnauthorized)
				}
				return
			}
		}

		user, ok, err := l.Authenticator.AuthenticateRequest(req)
		if !ok || err != nil {
			// older routers will not have permission to check token access review, so treat this
			// as an authorization denied if so
			if !ok || errors.IsUnauthorized(err) {
				glog.V(5).Infof("Unable to authenticate: %v", err)
				http.Error(w, "Unable to authenticate due to an error", http.StatusUnauthorized)
			} else {
				glog.V(3).Infof("Unable to authenticate: %v", err)
				http.Error(w, "Unable to authenticate due to an error", http.StatusInternalServerError)
			}
			return
		}
		scopedRecord := l.Record
		switch req.Method {
		case "POST":
			scopedRecord.Verb = "create"
		case "GET", "HEAD":
			scopedRecord.Verb = "get"
		case "PUT":
			scopedRecord.Verb = "update"
		case "PATCH":
			scopedRecord.Verb = "patch"
		case "DELETE":
			scopedRecord.Verb = "delete"
		default:
			scopedRecord.Verb = ""
		}
		switch {
		case req.URL.Path == "/metrics":
			scopedRecord.Subresource = "metrics"
		case strings.HasPrefix(req.URL.Path, "/debug/"):
			scopedRecord.Subresource = "debug"
		}
		scopedRecord.User = user
		authorized, reason, err := l.Authorizer.Authorize(scopedRecord)
		if err != nil {
			glog.V(3).Infof("Unable to authorize: %v", err)
			http.Error(w, "Unable to authorize the user due to an error", http.StatusInternalServerError)
			return
		}
		if authorized != authorizer.DecisionAllow {
			glog.V(5).Infof("Unable to authorize: %v", err)
			http.Error(w, fmt.Sprintf("Forbidden: %s", reason), http.StatusForbidden)
			return
		}
		protected.ServeHTTP(w, req)
	})
}

// Listen starts a server for health, metrics, and profiling on the provided listen port.
// It will terminate the process if the server fails. Metrics and profiling are only exposed
// if username and password are provided and the user's input matches.
func (l Listener) Listen() {
	handler := l.handler()

	tcpl, err := net.Listen("tcp", l.Addr)
	if err != nil {
		glog.Fatal(err)
	}

	// if a TLS connection was requested, set up a connection mux that will send TLS requests to
	// the TLS server but send HTTP requests to the HTTP server. Preserves the ability for HTTP
	// health checks to call HTTP on the router while still allowing TLS certs and end to end
	// metrics protection.
	m := cmux.New(tcpl)

	// match HTTP first
	httpl := m.Match(cmux.HTTP1Fast())
	go func() {
		s := &http.Server{
			Handler: handler,
		}
		if err := s.Serve(httpl); err != cmux.ErrListenerClosed {
			glog.Fatal(err)
		}
	}()

	// match TLS if configured
	if l.TLSConfig != nil {
		glog.Infof("Router health and metrics port listening at %s on HTTP and HTTPS", l.Addr)
		tlsl := m.Match(cmux.Any())
		tlsl = tls.NewListener(tlsl, l.TLSConfig)
		go func() {
			s := &http.Server{
				Handler: handler,
			}
			if err := s.Serve(tlsl); err != cmux.ErrListenerClosed {
				glog.Fatal(err)
			}
		}()
	} else {
		glog.Infof("Router health and metrics port listening at %s", l.Addr)
	}

	go func() {
		if err := m.Serve(); !strings.Contains(err.Error(), "use of closed network connection") {
			glog.Fatal(err)
		}
	}()
}
