// Package gitserver provides a smart Git HTTP server that can also set and
// remove hooks. The server is lightweight (<7M compiled with a ~2M footprint)
// and can mirror remote repositories in a containerized environment.
package gitserver

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/AaronO/go-git-http"
	"github.com/AaronO/go-git-http/auth"
	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apiserver/pkg/server/healthz"
	"k8s.io/client-go/tools/clientcmd"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	authorizationtypedclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/authorization/internalversion"

	"github.com/openshift/origin/pkg/git"

	s2igit "github.com/openshift/source-to-image/pkg/scm/git"
	"k8s.io/client-go/rest"
	"k8s.io/kubernetes/pkg/apis/authorization"
)

const (
	initialClonePrefix = "GIT_INITIAL_CLONE_"
	EnvironmentHelp    = `Supported environment variables:
GIT_HOME
  directory containing Git repositories; defaults to current directory
PUBLIC_URL
  the url of this server for constructing URLs that point to this repository
INTERNAL_URL
  the url of this server that can be used internally by build configs in the cluster
GIT_PATH
  path to Git binary; defaults to location of 'git' in PATH
HOOK_PATH
  path to a directory containing hooks for all repositories; if not set no global hooks will be used
ALLOW_GIT_PUSH
  if 'false', pushes will be not be accepted; defaults to true
ALLOW_ANON_GIT_PULL
  if 'true', pulls may be made without authorization; defaults to false
ALLOW_GIT_HOOKS
  if 'false', hooks cannot be read or set; defaults to true
ALLOW_LAZY_CREATE
  if 'false', repositories will not automatically be initialized on push; defaults to true
REQUIRE_GIT_AUTH
  a user/password combination required to access the repo of the form "<user>:<password>"; defaults to none
REQUIRE_SERVER_AUTH
  a URL to an OpenShift server for verifying authorization credentials provided by a user. Requires
  AUTOLINK_NAMESPACE to be set (the namespace that authorization will be checked in). Users must have
  'get' on 'pods' to pull (be a viewer) and 'create' on 'pods' to push (be an editor)
GIT_FORCE_CLEAN
  if 'true', any initial repository directories will be deleted prior to start; defaults to 'false'
  WARNING: this is destructive and you will lose any data you have already pushed
GIT_INITIAL_CLONE_*=<url>[;<name>]
  each environment variable in this pattern will be cloned when the process starts; failures will be logged
  <name> must be [A-Z0-9_\-\.], the cloned directory name will be lowercased. If the name is invalid the
  process will halt. If the repository already exists on disk, it will be updated from the remote.
AUTOLINK_KUBECONFIG
  The location to read auth configuration from for autolinking.
  If '-', use the service account token to link. The account
  represented by this config must have the edit role on the
  namespace.
AUTOLINK_NAMESPACE
  The namespace to autolink
AUTOLINK_HOOK
  The path to a script in the image to use as the default
  post-receive hook - only set during link, so has no effect
  on cloned repositories. See the "hooks" directory in the
  image for examples.
`
)

var (
	invalidCloneNameChars = regexp.MustCompile("[^a-zA-Z0-9_\\-\\.]")
	reservedNames         = map[string]struct{}{"_": {}}

	eventCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "git_event_count",
			Help: "Counter of events broken out for each repository and type",
		},
		[]string{"repository", "type"},
	)
)

func init() {
	prometheus.MustRegister(eventCounter)
}

// Config represents the configuration to use for running the server
type Config struct {
	Home        string
	GitBinary   string
	URL         *url.URL
	InternalURL *url.URL

	AllowHooks      bool
	AllowPush       bool
	AllowLazyCreate bool

	HookDirectory string
	MaxHookBytes  int64

	Listen string

	AuthenticatorFn func(http http.Handler) http.Handler

	CleanBeforeClone bool
	InitialClones    map[string]Clone

	AuthMessage string
}

// Clone is a repository to clone
type Clone struct {
	URL   s2igit.URL
	Hooks map[string]string
}

type statusError struct {
	*errors.StatusError
}

func (e *statusError) StatusCode() int {
	return int(e.StatusError.Status().Code)
}

// NewDefaultConfig returns a default server config.
func NewDefaultConfig() *Config {
	return &Config{
		Home:         "",
		GitBinary:    "git",
		Listen:       ":8080",
		MaxHookBytes: 50 * 1024,
	}
}

func checkURLIsAllowed(url *s2igit.URL) bool {
	switch url.Type {
	case s2igit.URLTypeLocal:
		return false
	case s2igit.URLTypeURL:
		if url.URL.Opaque != "" {
			return false
		}
		switch url.URL.Scheme {
		case "git", "http", "https", "ssh":
		default:
			return false
		}
	}
	return true
}

// NewEnvironmentConfig sets up the initial config from environment variables
// TODO break out the code that generates the handler functions so that they
// can be individually unit tested. Also separate out the code that generates
// the initial set of clones.
func NewEnvironmentConfig() (*Config, error) {
	config := NewDefaultConfig()

	home := os.Getenv("GIT_HOME")
	if len(home) == 0 {
		return nil, fmt.Errorf("GIT_HOME is required")
	}
	abs, err := filepath.Abs(home)
	if err != nil {
		return nil, fmt.Errorf("can't make %q absolute: %v", home, err)
	}
	if stat, err := os.Stat(abs); err != nil || !stat.IsDir() {
		return nil, fmt.Errorf("GIT_HOME must be an existing directory: %v", err)
	}
	config.Home = home

	if publicURL := os.Getenv("PUBLIC_URL"); len(publicURL) > 0 {
		valid, err := url.Parse(publicURL)
		if err != nil {
			return nil, fmt.Errorf("PUBLIC_URL must be a valid URL: %v", err)
		}
		config.URL = valid
	}

	if internalURL := os.Getenv("INTERNAL_URL"); len(internalURL) > 0 {
		valid, err := url.Parse(internalURL)
		if err != nil {
			return nil, fmt.Errorf("INTERNAL_URL must be a valid URL: %v", err)
		}
		config.InternalURL = valid
	}

	gitpath := os.Getenv("GIT_PATH")
	if len(gitpath) == 0 {
		path, err := exec.LookPath("git")
		if err != nil {
			return nil, fmt.Errorf("could not find 'git' in PATH; specify GIT_PATH or set your PATH")
		}
		gitpath = path
	}
	config.GitBinary = gitpath

	config.AllowPush = os.Getenv("ALLOW_GIT_PUSH") != "false"
	config.AllowHooks = os.Getenv("ALLOW_GIT_HOOKS") != "false"
	config.AllowLazyCreate = os.Getenv("ALLOW_LAZY_CREATE") != "false"

	if hookpath := os.Getenv("HOOK_PATH"); len(hookpath) != 0 {
		path, err := filepath.Abs(hookpath)
		if err != nil {
			return nil, fmt.Errorf("HOOK_PATH was set but cannot be made absolute: %v", err)
		}
		if stat, err := os.Stat(path); err != nil || !stat.IsDir() {
			return nil, fmt.Errorf("HOOK_PATH must be an existing directory if set: %v", err)
		}
		config.HookDirectory = path
	}

	allowAnonymousGet := os.Getenv("ALLOW_ANON_GIT_PULL") == "true"
	serverAuth := os.Getenv("REQUIRE_SERVER_AUTH")
	gitAuth := os.Getenv("REQUIRE_GIT_AUTH")
	if len(serverAuth) > 0 && len(gitAuth) > 0 {
		return nil, fmt.Errorf("only one of REQUIRE_SERVER_AUTH or REQUIRE_GIT_AUTH may be specified")
	}

	if len(serverAuth) > 0 {
		namespace := os.Getenv("AUTH_NAMESPACE")
		if len(namespace) == 0 {
			return nil, fmt.Errorf("when REQUIRE_SERVER_AUTH is set, AUTH_NAMESPACE must also be specified")
		}

		if serverAuth == "-" {
			serverAuth = ""
		}
		rules := clientcmd.NewDefaultClientConfigLoadingRules()
		rules.ExplicitPath = serverAuth
		kubeconfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, &clientcmd.ConfigOverrides{})
		cfg, err := kubeconfig.ClientConfig()
		if err != nil {
			return nil, fmt.Errorf("could not create a client for REQUIRE_SERVER_AUTH: %v", err)
		}

		config.AuthMessage = fmt.Sprintf("Authenticating against %s allow-push=%t anon-pull=%t", cfg.Host, config.AllowPush, allowAnonymousGet)
		authHandlerFn := auth.Authenticator(func(info auth.AuthInfo) (bool, error) {
			if !info.Push && allowAnonymousGet {
				glog.V(5).Infof("Allowing pull because anonymous get is enabled")
				return true, nil
			}
			req := &authorization.SelfSubjectAccessReview{
				Spec: authorization.SelfSubjectAccessReviewSpec{
					ResourceAttributes: &authorization.ResourceAttributes{
						Verb:      "get",
						Namespace: namespace,
						Group:     kapi.GroupName,
						Resource:  "pods",
					},
				},
			}
			if info.Push {
				if !config.AllowPush {
					return false, nil
				}
				req.Spec.ResourceAttributes.Verb = "create"
			}

			configForUser := rest.AnonymousClientConfig(cfg)
			configForUser.BearerToken = info.Password
			authorizationClient, err := authorizationtypedclient.NewForConfig(configForUser)
			if err != nil {
				return false, err
			}

			glog.V(5).Infof("Checking for %s permission on pods", req.Spec.ResourceAttributes.Verb)
			res, err := authorizationClient.SelfSubjectAccessReviews().Create(req)
			if err != nil {
				if se, ok := err.(*errors.StatusError); ok {
					return false, &statusError{se}
				}
				return false, err
			}
			glog.V(5).Infof("server response allowed=%t message=%s", res.Status.Allowed, res.Status.Reason)
			return res.Status.Allowed, nil
		})
		if allowAnonymousGet {
			authHandlerFn = anonymousHandler(authHandlerFn)
		}
		config.AuthenticatorFn = authHandlerFn
	}

	if len(gitAuth) > 0 {
		parts := strings.Split(gitAuth, ":")
		if len(parts) != 2 {
			return nil, fmt.Errorf("REQUIRE_GIT_AUTH must be a username and password separated by a ':'")
		}
		config.AuthMessage = fmt.Sprintf("Authenticating against username/password allow-push=%t", config.AllowPush)
		username, password := parts[0], parts[1]
		authHandlerFn := auth.Authenticator(func(info auth.AuthInfo) (bool, error) {
			if info.Push {
				if !config.AllowPush {
					glog.V(5).Infof("Denying push request because it is disabled in config.")
					return false, nil
				}
			}
			if info.Username != username || info.Password != password {
				glog.V(5).Infof("Username or password doesn't match")
				return false, nil
			}
			return true, nil
		})
		if allowAnonymousGet {
			authHandlerFn = anonymousHandler(authHandlerFn)
		}
		config.AuthenticatorFn = authHandlerFn
	}

	if value := os.Getenv("GIT_LISTEN"); len(value) > 0 {
		config.Listen = value
	}

	config.CleanBeforeClone = os.Getenv("GIT_FORCE_CLEAN") == "true"

	clones := make(map[string]Clone)
	for _, env := range os.Environ() {
		if !strings.HasPrefix(env, initialClonePrefix) {
			continue
		}
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key, value := parts[0], parts[1]
		part := key[len(initialClonePrefix):]
		if len(part) == 0 {
			continue
		}
		if len(value) == 0 {
			return nil, fmt.Errorf("%s must not have an empty value", key)
		}

		defaultName := strings.Replace(strings.ToLower(part), "_", "-", -1)
		values := strings.Split(value, ";")

		var uri, name string
		switch len(values) {
		case 1:
			uri, name = values[0], ""
		case 2:
			uri, name = values[0], values[1]
			if len(name) == 0 {
				return nil, fmt.Errorf("%s name may not be empty", key)
			}
		default:
			return nil, fmt.Errorf("%s may only have two segments (<url> or <url>;<name>)", key)
		}

		url, err := s2igit.Parse(uri)
		if err != nil {
			return nil, fmt.Errorf("%s is not a valid repository URI: %v", key, err)
		}
		if !checkURLIsAllowed(url) {
			return nil, fmt.Errorf("%s %q must be a http, https, git, or ssh URL", key, uri)
		}

		if len(name) == 0 {
			if n, ok := git.NameFromRepositoryURL(&url.URL); ok {
				name = n + ".git"
			}
		}
		if len(name) == 0 {
			name = defaultName + ".git"
		}

		if invalidCloneNameChars.MatchString(name) {
			return nil, fmt.Errorf("%s name %q must be only letters, numbers, dashes, or underscores", key, name)
		}
		if _, ok := reservedNames[name]; ok {
			return nil, fmt.Errorf("%s name %q is reserved (%v)", key, name, reservedNames)
		}

		glog.V(5).Infof("Adding initial clone to repo at %s", url.String())

		clones[name] = Clone{
			URL: *url,
		}
	}
	config.InitialClones = clones

	return config, nil
}

func anonymousHandler(f func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		authHandler := f(h)
		anonymousHandler := func(w http.ResponseWriter, req *http.Request) {
			// If anonymous read request, simply serve content
			if req.FormValue("service") != "git-receive-pack" && len(req.Header.Get("Authorization")) == 0 {
				h.ServeHTTP(w, req)
				return
			}
			authHandler.ServeHTTP(w, req)
		}
		return http.HandlerFunc(anonymousHandler)
	}
}

func handler(config *Config) http.Handler {
	git := githttp.New(config.Home)
	git.GitBinPath = config.GitBinary
	git.UploadPack = config.AllowPush
	git.ReceivePack = config.AllowPush
	git.EventHandler = func(ev githttp.Event) {
		path := ev.Dir
		if strings.HasPrefix(path, config.Home+"/") {
			path = path[len(config.Home)+1:]
		}
		eventCounter.WithLabelValues(path, ev.Type.String()).Inc()
	}
	handler := http.Handler(git)

	if config.AllowLazyCreate {
		handler = lazyInitRepositoryHandler(config, handler)
	}

	if config.AuthenticatorFn != nil {
		handler = config.AuthenticatorFn(handler)
	}
	return handler
}

func Start(config *Config) error {
	if err := clone(config); err != nil {
		return err
	}
	handler := handler(config)

	ops := http.NewServeMux()
	if config.AllowHooks {
		glog.V(5).Infof("Installing handler for the /_/hooks endpoint")
		ops.Handle("/hooks/", prometheus.InstrumentHandler("hooks", http.StripPrefix("/hooks", hooksHandler(config))))
	}
	/*ops.Handle("/reflect/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		fmt.Fprintf(os.Stdout, "%s %s\n", r.Method, r.URL)
		io.Copy(os.Stdout, r.Body)
	}))*/
	glog.V(5).Infof("Installing handler for the /_/metrics endpoint")
	ops.Handle("/metrics", prometheus.UninstrumentedHandler())
	healthz.InstallHandler(ops)

	mux := http.NewServeMux()
	mux.Handle("/", prometheus.InstrumentHandler("git", handler))
	mux.Handle("/_/", http.StripPrefix("/_", ops))

	if len(config.AuthMessage) > 0 {
		glog.Infof("%s", config.AuthMessage)
	}
	glog.Infof("Serving %s on %s", config.Home, config.Listen)
	return http.ListenAndServe(config.Listen, mux)
}
