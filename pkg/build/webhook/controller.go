package webhook

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/golang/glog"

	buildapi "github.com/openshift/origin/pkg/build/api"
	buildclient "github.com/openshift/origin/pkg/build/client"
	kapi "k8s.io/kubernetes/pkg/api"
)

// Plugin for Webhook verification is dependent on the sending side, it can be
// eg. github, bitbucket or else, so there must be a separate Plugin
// instance for each webhook provider.
type Plugin interface {
	// Method extracts build information and returns:
	// - newly created build object or nil if default is to be created
	// - information whether to trigger the build itself
	// - eventual error.
	Extract(buildCfg *buildapi.BuildConfig, secret, path string, req *http.Request) (*buildapi.SourceRevision, bool, error)
}

// controller used for processing webhook requests.
type controller struct {
	buildConfigInstantiator buildclient.BuildConfigInstantiator
	buildConfigGetter       buildclient.BuildConfigGetter
	plugins                 map[string]Plugin
}

// urlVars holds parsed URL parts.
type urlVars struct {
	namespace       string
	buildConfigName string
	secret          string
	plugin          string
	path            string
}

// NewController creates new webhook controller and feed it with provided plugins.
func NewController(bcg buildclient.BuildConfigGetter,
	bci buildclient.BuildConfigInstantiator, plugins map[string]Plugin) http.Handler {
	return &controller{
		buildConfigGetter:       bcg,
		buildConfigInstantiator: bci,
		plugins:                 plugins,
	}
}

// ServeHTTP main REST service method.
func (c *controller) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	uv, err := parseURL(req)
	if err != nil {
		glog.V(2).Infof("Failed to parse request URL: %v", err)
		notFound(w, err.Error())
		return
	}

	buildCfg, err := c.buildConfigGetter.Get(uv.namespace, uv.buildConfigName)
	if err != nil {
		glog.V(2).Infof("Failed to get BuildConfig %s/%s: %v", uv.namespace, uv.buildConfigName, err)
		badRequest(w, err.Error())
		return
	}

	plugin, ok := c.plugins[uv.plugin]
	if !ok {
		glog.V(2).Infof("Plugin %s not found", uv.plugin)
		notFound(w, "Plugin ", uv.plugin, " not found")
		return
	}
	revision, proceed, err := plugin.Extract(buildCfg, uv.secret, uv.path, req)
	if err != nil {
		glog.V(2).Infof("Failed to extract information from webhook: %v", err)
		badRequest(w, err.Error())
		return
	}
	if !proceed {
		return
	}
	request := &buildapi.BuildRequest{
		ObjectMeta: kapi.ObjectMeta{Name: buildCfg.Name},
		Revision:   revision,
	}
	if _, err := c.buildConfigInstantiator.Instantiate(uv.namespace, request); err != nil {
		glog.V(2).Infof("Failed to generate new Build from BuildConfig %s/%s: %v", buildCfg.Namespace, buildCfg.Name, err)
		badRequest(w, err.Error())
	}
}

// parseURL retrieves the namespace from the query parameters and returns a context wrapping the namespace,
// the parameters for the webhook call, and an error.
// according to the docs (http://godoc.org/code.google.com/p/go.net/context) ctx is not supposed to be wrapped in another object
func parseURL(req *http.Request) (uv urlVars, err error) {
	url := req.URL.Path

	parts := splitPath(url)
	if len(parts) < 3 {
		err = fmt.Errorf("unexpected URL %s", url)
		return
	}
	uv = urlVars{
		namespace:       kapi.NamespaceDefault,
		buildConfigName: parts[0],
		secret:          parts[1],
		plugin:          parts[2],
		path:            "",
	}
	if len(parts) > 3 {
		uv.path = strings.Join(parts[3:], "/")
	}

	// TODO for now, we pull namespace from query parameter, but according to spec, it must go in resource path in future PR
	// if a namespace if specified, it's always used.
	// for list/watch operations, a namespace is not required if omitted.
	// for all other operations, if namespace is omitted, we will default to default namespace.
	namespace := req.URL.Query().Get("namespace")
	if len(namespace) > 0 {
		uv.namespace = namespace
	}

	return
}

func splitPath(path string) []string {
	path = strings.Trim(path, "/")
	if path == "" {
		return []string{}
	}
	return strings.Split(path, "/")
}

func notFound(w http.ResponseWriter, args ...string) {
	http.Error(w, strings.Join(args, ""), http.StatusNotFound)
}

func badRequest(w http.ResponseWriter, args ...string) {
	http.Error(w, strings.Join(args, ""), http.StatusBadRequest)
}
