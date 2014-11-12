package webhook

import (
	"fmt"
	"net/http"
	"strings"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/openshift/origin/pkg/build/api"
)

// Webhook verification is dependent on the sending side, it can be
// eg. github, bitbucket or else, so there must be a separate Plugin
// instance for each webhook provider.
type Plugin interface {
	// Method extracts build information and returns:
	// - newly created build object or nil if default is to be created
	// - information whether to trigger the build itself
	// - eventual error.
	Extract(buildCfg *api.BuildConfig, secret, path string, req *http.Request) (*api.Build, bool, error)
}

// controller used for processing webhook requests.
type controller struct {
	osClient webhookBuildInterface
	plugins  map[string]Plugin
}

type webhookBuildInterface interface {
	CreateBuild(ctx kapi.Context, build *api.Build) (*api.Build, error)
	GetBuildConfig(ctx kapi.Context, id string) (*api.BuildConfig, error)
}

// urlVars holds parsed URL parts.
type urlVars struct {
	buildId string
	secret  string
	plugin  string
	path    string
}

// NewController creates new webhook controller and feed it with provided plugins.
func NewController(osClient webhookBuildInterface, plugins map[string]Plugin) http.Handler {
	return &controller{osClient: osClient, plugins: plugins}
}

// ServeHTTP main REST service method.
func (c *controller) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	ctx, uv, err := parseUrl(req)
	if err != nil {
		notFound(w, err.Error())
		return
	}

	buildCfg, err := c.osClient.GetBuildConfig(kapi.WithNamespaceDefaultIfNone(ctx), uv.buildId)
	if err != nil {
		badRequest(w, err.Error())
		return
	}

	plugin, ok := c.plugins[uv.plugin]
	if !ok {
		notFound(w, "Plugin ", uv.plugin, " not found")
		return
	}
	build, proceed, err := plugin.Extract(buildCfg, uv.secret, uv.path, req)
	if err != nil {
		badRequest(w, err.Error())
		return
	}
	if !proceed {
		return
	}
	if build == nil {
		build = &api.Build{
			Parameters: buildCfg.Parameters,
		}
	}
	if build.Labels == nil {
		build.Labels = make(map[string]string)
	}
	build.Labels[api.BuildConfigLabel] = buildCfg.Name

	if _, err := c.osClient.CreateBuild(kapi.WithNamespaceDefaultIfNone(ctx), build); err != nil {
		badRequest(w, err.Error())
	}
}

// parseUrl retrieves the namespace from the query parameters and returns a context wrapping the namespace,
// the parameters for the webhook call, and an error.
// according to the docs (http://godoc.org/code.google.com/p/go.net/context) ctx is not supposed to be wrapped in another object
func parseUrl(req *http.Request) (ctx kapi.Context, uv urlVars, err error) {
	url := req.URL.Path
	ctx = kapi.NewContext()

	parts := splitPath(url)
	if len(parts) < 3 {
		err = fmt.Errorf("Unexpected URL %s", url)
		return
	}
	uv = urlVars{parts[0], parts[1], parts[2], ""}
	if len(parts) > 3 {
		uv.path = strings.Join(parts[3:], "/")
	}

	// TODO for now, we pull namespace from query parameter, but according to spec, it must go in resource path in future PR
	// if a namespace if specified, it's always used.
	// for list/watch operations, a namespace is not required if omitted.
	// for all other operations, if namespace is omitted, we will default to default namespace.
	namespace := req.URL.Query().Get("namespace")
	if len(namespace) > 0 {
		ctx = kapi.WithNamespace(ctx, namespace)
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
