package origin

import (
	"net/http"

	"github.com/golang/glog"
	"github.com/pborman/uuid"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/util/net"

	authenticationapi "github.com/openshift/origin/pkg/auth/api"
)

type auditResponseWriter struct {
	http.ResponseWriter
	id string
}

func (a *auditResponseWriter) WriteHeader(code int) {
	glog.Infof("AUDIT: id=%q response=\"%d\"", a.id, code)
	a.ResponseWriter.WriteHeader(code)
}

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
		namespace := kapi.NamespaceValue(ctx)
		if len(namespace) == 0 {
			namespace = "<none>"
		}
		id := uuid.NewRandom().String()

		glog.Infof("AUDIT: id=%q ip=%q method=%q user=%q as=%q namespace=%q uri=%q",
			id, net.GetClientIP(req), req.Method, user.GetName(), asuser, namespace, req.URL)
		handler.ServeHTTP(&auditResponseWriter{w, id}, req)
	})
}
