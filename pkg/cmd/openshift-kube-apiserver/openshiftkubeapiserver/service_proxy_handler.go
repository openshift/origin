package openshiftkubeapiserver

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"net/url"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/apimachinery/pkg/util/proxy"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apiserver/pkg/endpoints/handlers/responsewriters"
)

var proxyErrorPageTemplate = template.Must(template.New("proxyErrorPage").Parse(proxyErrorPageTemplateString))

const proxyErrorPageTemplateString = `<!doctype html>
<html>
  <head>
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>{{.ApplicationDisplayName}} is not available</title>
    <style type="text/css">
      body {
	font-family: "Helvetica Neue", Helvetica, Arial, sans-serif;
	line-height: 1.66666667;
	font-size: 13px;
	color: #333333;
	background-color: #ffffff;
      }
      h1 {
	font-size: 24px;
	font-weight: 300;
      }
      p {
	margin: 0 0 10px;
	font-size: 13px;
      }
      small {
	color: #9c9c9c;
	white-space: pre-line;
      }
      @media (min-width: 768px) {
	body {
	  margin: 2em 3em;
	}
	h1 {
	  font-size: 2.15em;
	}
      }
    </style>
  </head>
  <body>
    <h1>{{.ApplicationDisplayName}} is not available</h1>
    <p>The application is currently not serving requests. It might not be installed or is still installing.</p>
    <small>{{.ErrorMessage}}</small>
    <div>
  </body>
</html>
`

// serviceProxyErrorPageDetails contains the error details to show in the HTML error page for proxy errors.
type serviceProxyErrorPageDetails struct {
	ApplicationDisplayName string
	ErrorMessage           string
}

// A ServiceResolver knows how to get a URL given a service.
type ServiceResolver interface {
	ResolveEndpoint(namespace, name string) (*url.URL, error)
}

// proxyHandler provides a http.Handler which will proxy traffic to locations
// specified by items implementing Redirector.
type serviceProxyHandler struct {
	serviceName      string
	serviceNamespace string

	// Endpoints based routing to map from cluster IP to routable IP
	serviceResolver ServiceResolver

	applicationDisplayName string

	// proxyRoundTripper is the re-useable portion of the transport.  It does not vary with any request.
	proxyRoundTripper http.RoundTripper
}

func (r *serviceProxyHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// write a new location based on the existing request pointed at the target service
	location := &url.URL{}
	location.Scheme = "https"
	rloc, err := r.serviceResolver.ResolveEndpoint(r.serviceNamespace, r.serviceName)
	if err != nil {
		errorPageDetails := serviceProxyErrorPageDetails{
			ApplicationDisplayName: r.applicationDisplayName,
		}
		if errors.IsNotFound(err) {
			w.WriteHeader(http.StatusNotFound)
			errorPageDetails.ErrorMessage = fmt.Sprintf("Missing service: %s", err.Error())
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			errorPageDetails.ErrorMessage = fmt.Sprintf("Missing route: %s", err.Error())
		}
		if err := proxyErrorPageTemplate.Execute(w, errorPageDetails); err != nil {
			utilruntime.HandleError(fmt.Errorf("unable to render proxy error page template: %v", err))
		}
		return
	}
	location.Host = rloc.Host
	location.Path = req.URL.Path
	location.RawQuery = req.URL.Query().Encode()

	// WithContext creates a shallow clone of the request with the new context.
	newReq := req.WithContext(context.Background())
	newReq.Header = utilnet.CloneHeader(req.Header)
	newReq.URL = location

	handler := proxy.NewUpgradeAwareHandler(location, r.proxyRoundTripper, false, false, &responder{w: w})
	handler.ServeHTTP(w, newReq)
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

func (r *responder) Error(_ http.ResponseWriter, _ *http.Request, err error) {
	http.Error(r.w, err.Error(), http.StatusInternalServerError)
}
