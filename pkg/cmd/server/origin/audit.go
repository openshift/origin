package origin

import (
	"bufio"
	"fmt"
	"net"
	"net/http"

	"github.com/golang/glog"
	"github.com/pborman/uuid"

	kapi "k8s.io/kubernetes/pkg/api"
	utilnet "k8s.io/kubernetes/pkg/util/net"

	authenticationapi "github.com/openshift/origin/pkg/auth/api"
)

// auditResponseWriter implements http.ResponseWriter interface.
type auditResponseWriter struct {
	http.ResponseWriter
	id string
}

func (a *auditResponseWriter) WriteHeader(code int) {
	glog.Infof("AUDIT: id=%q response=\"%d\"", a.id, code)
	a.ResponseWriter.WriteHeader(code)
}

var _ http.ResponseWriter = &auditResponseWriter{}

// fancyResponseWriterDelegator implements http.CloseNotifier, http.Flusher and
// http.Hijacker which are needed to make certain http operation (eg. watch, rsh, etc)
// working.
type fancyResponseWriterDelegator struct {
	*auditResponseWriter
}

func (f *fancyResponseWriterDelegator) CloseNotify() <-chan bool {
	return f.ResponseWriter.(http.CloseNotifier).CloseNotify()
}

func (f *fancyResponseWriterDelegator) Flush() {
	f.ResponseWriter.(http.Flusher).Flush()
}

func (f *fancyResponseWriterDelegator) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return f.ResponseWriter.(http.Hijacker).Hijack()
}

var _ http.CloseNotifier = &fancyResponseWriterDelegator{}
var _ http.Flusher = &fancyResponseWriterDelegator{}
var _ http.Hijacker = &fancyResponseWriterDelegator{}

// auditHandler is responsible for logging audit information for all the
// request coming to server. Each audit log contains two entries:
// 1. the request line containing:
//    - unique id allowing to match the response line (see 2)
//    - source ip of the request
//    - HTTP method being invoked
//    - original user invoking the operation
//    - impersonated user for the operation
//    - namespace of the request or <none>
//    - uri is the full URI as requested
// 2. the response line containing the unique id from 1 and response code
func (c *MasterConfig) auditHandler(handler http.Handler) http.Handler {
	if !c.Options.AuditConfig.Enabled {
		return handler
	}

	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ctx, _ := c.RequestContextMapper.Get(req)
		user, _ := kapi.UserFrom(ctx)
		asuser := req.Header.Get(authenticationapi.ImpersonateUserHeader)
		if len(asuser) == 0 {
			asuser = "<self>"
		}
		requestedGroups := req.Header[authenticationapi.ImpersonateGroupHeader]
		asgroups := "<lookup>"
		if len(requestedGroups) == 0 {
			asgroups = ""
			first := true
			for _, group := range requestedGroups {
				if !first {
					asgroups = asgroups + ","
				}
				asgroups = asgroups + fmt.Sprintf("%q", group)
				first = false
			}
		}
		namespace := kapi.NamespaceValue(ctx)
		if len(namespace) == 0 {
			namespace = "<none>"
		}
		id := uuid.NewRandom().String()

		glog.Infof("AUDIT: id=%q ip=%q method=%q user=%q as=%q asgroups=%q namespace=%q uri=%q",
			id, utilnet.GetClientIP(req), req.Method, user.GetName(), asuser, asgroups, namespace, req.URL)
		handler.ServeHTTP(constructResponseWriter(w, id), req)
	})
}

func constructResponseWriter(responseWriter http.ResponseWriter, id string) http.ResponseWriter {
	delegate := &auditResponseWriter{ResponseWriter: responseWriter, id: id}
	// check if the ResponseWriter we're wrapping is the fancy one we need
	// or if the basic is sufficient
	_, cn := responseWriter.(http.CloseNotifier)
	_, fl := responseWriter.(http.Flusher)
	_, hj := responseWriter.(http.Hijacker)
	if cn && fl && hj {
		return &fancyResponseWriterDelegator{delegate}
	}
	return delegate
}
