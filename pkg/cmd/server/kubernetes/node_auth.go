package kubernetes

import (
	"net/http"
	"strings"
	"time"

	"github.com/golang/glog"

	kauthorizer "k8s.io/kubernetes/pkg/auth/authorizer"
	"k8s.io/kubernetes/pkg/auth/user"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	oauthorizer "github.com/openshift/origin/pkg/authorization/authorizer"
	authzadapter "github.com/openshift/origin/pkg/authorization/authorizer/adapter"
	authzcache "github.com/openshift/origin/pkg/authorization/authorizer/cache"
	authzremote "github.com/openshift/origin/pkg/authorization/authorizer/remote"
	oclient "github.com/openshift/origin/pkg/client"
)

func newAuthorizerAttributesGetter(nodeName string) (kauthorizer.RequestAttributesGetter, error) {
	return NodeAuthorizerAttributesGetter{nodeName}, nil
}

type NodeAuthorizerAttributesGetter struct {
	nodeName string
}

func isSubpath(r *http.Request, path string) bool {
	path = strings.TrimSuffix(path, "/")
	return r.URL.Path == path || strings.HasPrefix(r.URL.Path, path+"/")
}

// GetRequestAttributes populates authorizer attributes for the requests to the kubelet API.
// Default attributes are {apiVersion=v1,verb=proxy,resource=nodes,resourceName=<node name>}
// More specific verb/resource is set for the following request patterns:
//    /stats/*   => verb=<api verb from request>, resource=nodes/stats
//    /metrics/* => verb=<api verb from request>, resource=nodes/metrics
//    /logs/*    => verb=<api verb from request>, resource=nodes/log
func (n NodeAuthorizerAttributesGetter) GetRequestAttributes(u user.Info, r *http.Request) kauthorizer.Attributes {

	namespace := ""

	apiVerb := ""
	switch r.Method {
	case "POST":
		apiVerb = "create"
	case "GET":
		apiVerb = "get"
	case "PUT":
		apiVerb = "update"
	case "PATCH":
		apiVerb = "patch"
	case "DELETE":
		apiVerb = "delete"
	}

	// Default verb/resource is <apiVerb> nodes/proxy, which allows full access to the kubelet API
	attrs := oauthorizer.DefaultAuthorizationAttributes{
		APIVersion:   "v1",
		APIGroup:     "",
		Verb:         apiVerb,
		Resource:     "nodes/proxy",
		ResourceName: n.nodeName,
		URL:          r.URL.Path,
	}

	// Override verb/resource for specific paths
	// Updates to these rules require updating NodeAdminRole and NodeReaderRole in bootstrap policy
	switch {
	case isSubpath(r, "/spec"):
		attrs.Verb = apiVerb
		attrs.Resource = authorizationapi.NodeSpecResource
	case isSubpath(r, "/stats"):
		attrs.Verb = apiVerb
		attrs.Resource = authorizationapi.NodeStatsResource
	case isSubpath(r, "/metrics"):
		attrs.Verb = apiVerb
		attrs.Resource = authorizationapi.NodeMetricsResource
	case isSubpath(r, "/logs"):
		attrs.Verb = apiVerb
		attrs.Resource = authorizationapi.NodeLogResource
	}
	// TODO: handle other things like /healthz/*? not sure if "non-resource" urls on the kubelet make sense to authorize against master non-resource URL policy

	glog.V(2).Infof("Node request attributes: namespace=%s, user=%#v, attrs=%#v", namespace, u, attrs)

	return authzadapter.KubernetesAuthorizerAttributes(namespace, u, attrs)
}

func newAuthorizer(c *oclient.Client, cacheTTL time.Duration, cacheSize int) (kauthorizer.Authorizer, error) {
	var (
		authz oauthorizer.Authorizer
		err   error
	)

	// Authorize against the remote master
	authz, err = authzremote.NewAuthorizer(c)
	if err != nil {
		return nil, err
	}

	// Cache results
	if cacheTTL > 0 && cacheSize > 0 {
		authz, err = authzcache.NewAuthorizer(authz, cacheTTL, cacheSize)
		if err != nil {
			return nil, err
		}
	}

	// Adapt to the Kubernetes authorizer interface
	kauthz, err := authzadapter.NewAuthorizer(authz)
	if err != nil {
		return nil, err
	}

	return kauthz, nil
}
