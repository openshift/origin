package server

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	log "github.com/Sirupsen/logrus"
	context "github.com/docker/distribution/context"
	registryauth "github.com/docker/distribution/registry/auth"

	kerrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/client/restclient"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	imageapi "github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/util/httprequest"
)

type deferredErrors map[string]error

func (d deferredErrors) Add(namespace string, name string, err error) {
	d[namespace+"/"+name] = err
}
func (d deferredErrors) Get(namespace string, name string) (error, bool) {
	err, exists := d[namespace+"/"+name]
	return err, exists
}
func (d deferredErrors) Empty() bool {
	return len(d) == 0
}

const (
	OpenShiftAuth = "openshift"

	defaultTokenPath = "/openshift/token"

	RealmKey      = "realm"
	TokenRealmKey = "tokenrealm"
)

// RegistryClient encapsulates getting access to the OpenShift API.
type RegistryClient interface {
	// Clients return the authenticated clients to use with the server.
	Clients() (client.Interface, kclient.Interface, error)
	// SafeClientConfig returns a client config without authentication info.
	SafeClientConfig() restclient.Config
}

// DefaultRegistryClient is exposed for testing the registry with fake client.
var DefaultRegistryClient = NewRegistryClient(clientcmd.NewConfig().BindToFile())

// registryClient implements RegistryClient
type registryClient struct {
	config *clientcmd.Config
}

var _ RegistryClient = &registryClient{}

// NewRegistryClient creates a registry client.
func NewRegistryClient(config *clientcmd.Config) RegistryClient {
	return &registryClient{config: config}
}

// Client returns the authenticated client to use with the server.
func (r *registryClient) Clients() (client.Interface, kclient.Interface, error) {
	return r.config.Clients()
}

// SafeClientConfig returns a client config without authentication info.
func (r *registryClient) SafeClientConfig() restclient.Config {
	return clientcmd.AnonymousClientConfig(r.config.OpenShiftConfig())
}

func init() {
	registryauth.Register(OpenShiftAuth, registryauth.InitFunc(newAccessController))
}

type contextKey int

var userClientKey contextKey = 0

func WithUserClient(parent context.Context, userClient client.Interface) context.Context {
	return context.WithValue(parent, userClientKey, userClient)
}

func UserClientFrom(ctx context.Context) (client.Interface, bool) {
	userClient, ok := ctx.Value(userClientKey).(client.Interface)
	return userClient, ok
}

const authPerformedKey = "openshift.auth.performed"

func WithAuthPerformed(parent context.Context) context.Context {
	return context.WithValue(parent, authPerformedKey, true)
}

func AuthPerformed(ctx context.Context) bool {
	authPerformed, ok := ctx.Value(authPerformedKey).(bool)
	return ok && authPerformed
}

const deferredErrorsKey = "openshift.auth.deferredErrors"

func WithDeferredErrors(parent context.Context, errs deferredErrors) context.Context {
	return context.WithValue(parent, deferredErrorsKey, errs)
}
func DeferredErrorsFrom(ctx context.Context) (deferredErrors, bool) {
	errs, ok := ctx.Value(deferredErrorsKey).(deferredErrors)
	return errs, ok
}

type AccessController struct {
	realm      string
	tokenRealm *url.URL
	config     restclient.Config
}

var _ registryauth.AccessController = &AccessController{}

type authChallenge struct {
	realm string
	err   error
}

var _ registryauth.Challenge = &authChallenge{}

type tokenAuthChallenge struct {
	realm   string
	service string
	err     error
}

var _ registryauth.Challenge = &tokenAuthChallenge{}

// Errors used and exported by this package.
var (
	// Challenging errors
	ErrTokenRequired         = errors.New("authorization header required")
	ErrTokenInvalid          = errors.New("failed to decode credentials")
	ErrOpenShiftAccessDenied = errors.New("access denied")

	// Non-challenging errors
	ErrNamespaceRequired   = errors.New("repository namespace required")
	ErrUnsupportedAction   = errors.New("unsupported action")
	ErrUnsupportedResource = errors.New("unsupported resource")
)

// TokenRealm returns the template URL to use as the token realm redirect.
// An empty scheme/host in the returned URL means to match the scheme/host on incoming requests.
func TokenRealm(options map[string]interface{}) (*url.URL, error) {
	if options[TokenRealmKey] == nil {
		// If not specified, default to "/openshift/token", auto-detecting the scheme and host
		return &url.URL{Path: defaultTokenPath}, nil
	}

	tokenRealmString, ok := options[TokenRealmKey].(string)
	if !ok {
		return nil, fmt.Errorf("%s config option must be a string, got %T", TokenRealmKey, options[TokenRealmKey])
	}

	tokenRealm, err := url.Parse(tokenRealmString)
	if err != nil {
		return nil, fmt.Errorf("error parsing URL in %s config option: %v", TokenRealmKey, err)
	}
	if len(tokenRealm.RawQuery) > 0 || len(tokenRealm.Fragment) > 0 {
		return nil, fmt.Errorf("%s config option may not contain query parameters or a fragment", TokenRealmKey)
	}
	if len(tokenRealm.Path) > 0 {
		return nil, fmt.Errorf("%s config option may not contain a path (%q was specified)", TokenRealmKey, tokenRealm.Path)
	}

	// pin to "/openshift/token"
	tokenRealm.Path = defaultTokenPath

	return tokenRealm, nil
}

func newAccessController(options map[string]interface{}) (registryauth.AccessController, error) {
	log.Info("Using Origin Auth handler")
	realm, ok := options[RealmKey].(string)
	if !ok {
		// Default to openshift if not present
		realm = "origin"
	}

	tokenRealm, err := TokenRealm(options)
	if err != nil {
		return nil, err
	}

	return &AccessController{realm: realm, tokenRealm: tokenRealm, config: DefaultRegistryClient.SafeClientConfig()}, nil
}

// Error returns the internal error string for this authChallenge.
func (ac *authChallenge) Error() string {
	return ac.err.Error()
}

// SetHeaders sets the basic challenge header on the response.
func (ac *authChallenge) SetHeaders(w http.ResponseWriter) {
	// WWW-Authenticate response challenge header.
	// See https://tools.ietf.org/html/rfc6750#section-3
	str := fmt.Sprintf("Basic realm=%s", ac.realm)
	if ac.err != nil {
		str = fmt.Sprintf("%s,error=%q", str, ac.Error())
	}
	w.Header().Set("WWW-Authenticate", str)
}

// Error returns the internal error string for this authChallenge.
func (ac *tokenAuthChallenge) Error() string {
	return ac.err.Error()
}

// SetHeaders sets the bearer challenge header on the response.
func (ac *tokenAuthChallenge) SetHeaders(w http.ResponseWriter) {
	// WWW-Authenticate response challenge header.
	// See https://docs.docker.com/registry/spec/auth/token/#/how-to-authenticate and https://tools.ietf.org/html/rfc6750#section-3
	str := fmt.Sprintf("Bearer realm=%q", ac.realm)
	if ac.service != "" {
		str += fmt.Sprintf(",service=%q", ac.service)
	}
	w.Header().Set("WWW-Authenticate", str)
}

// wrapErr wraps errors related to authorization in an authChallenge error that will present a WWW-Authenticate challenge response
func (ac *AccessController) wrapErr(ctx context.Context, err error) error {
	switch err {
	case ErrTokenRequired:
		// Challenge for errors that involve missing tokens
		if ac.tokenRealm == nil {
			// Send the basic challenge if we don't have a place to redirect
			return &authChallenge{realm: ac.realm, err: err}
		}

		if len(ac.tokenRealm.Scheme) > 0 && len(ac.tokenRealm.Host) > 0 {
			// Redirect to token auth if we've been given an absolute URL
			return &tokenAuthChallenge{realm: ac.tokenRealm.String(), err: err}
		}

		// Auto-detect scheme/host from request
		req, reqErr := context.GetRequest(ctx)
		if reqErr != nil {
			return reqErr
		}
		scheme, host := httprequest.SchemeHost(req)
		tokenRealmCopy := *ac.tokenRealm
		if len(tokenRealmCopy.Scheme) == 0 {
			tokenRealmCopy.Scheme = scheme
		}
		if len(tokenRealmCopy.Host) == 0 {
			tokenRealmCopy.Host = host
		}
		return &tokenAuthChallenge{realm: tokenRealmCopy.String(), err: err}
	case ErrTokenInvalid, ErrOpenShiftAccessDenied:
		// Challenge for errors that involve tokens or access denied
		return &authChallenge{realm: ac.realm, err: err}
	case ErrNamespaceRequired, ErrUnsupportedAction, ErrUnsupportedResource:
		// Malformed or unsupported request, no challenge
		return err
	default:
		// By default, just return the error, this gets surfaced as a bad request / internal error, but no challenge
		return err
	}
}

// Authorized handles checking whether the given request is authorized
// for actions on resources allowed by openshift.
// Sources of access records:
//   origin/pkg/cmd/dockerregistry/dockerregistry.go#Execute
//   docker/distribution/registry/handlers/app.go#appendAccessRecords
func (ac *AccessController) Authorized(ctx context.Context, accessRecords ...registryauth.Access) (context.Context, error) {
	req, err := context.GetRequest(ctx)
	if err != nil {
		return nil, ac.wrapErr(ctx, err)
	}

	bearerToken, err := getOpenShiftAPIToken(ctx, req)
	if err != nil {
		return nil, ac.wrapErr(ctx, err)
	}

	copied := ac.config
	copied.BearerToken = bearerToken
	osClient, err := client.New(&copied)
	if err != nil {
		return nil, ac.wrapErr(ctx, err)
	}

	// In case of docker login, hits endpoint /v2
	if len(accessRecords) == 0 {
		if err := verifyOpenShiftUser(ctx, osClient); err != nil {
			return nil, ac.wrapErr(ctx, err)
		}
	}

	// pushChecks remembers which ns/name pairs had push access checks done
	pushChecks := map[string]bool{}
	// possibleCrossMountErrors holds errors which may be related to cross mount errors
	possibleCrossMountErrors := deferredErrors{}

	verifiedPrune := false

	// Validate all requested accessRecords
	// Only return failure errors from this loop. Success should continue to validate all records
	for _, access := range accessRecords {
		context.GetLogger(ctx).Debugf("Origin auth: checking for access to %s:%s:%s", access.Resource.Type, access.Resource.Name, access.Action)

		switch access.Resource.Type {
		case "repository":
			imageStreamNS, imageStreamName, err := getNamespaceName(access.Resource.Name)
			if err != nil {
				return nil, ac.wrapErr(ctx, err)
			}

			verb := ""
			switch access.Action {
			case "push":
				verb = "update"
				pushChecks[imageStreamNS+"/"+imageStreamName] = true
			case "pull":
				verb = "get"
			case "*":
				verb = "prune"
			default:
				return nil, ac.wrapErr(ctx, ErrUnsupportedAction)
			}

			switch verb {
			case "prune":
				if verifiedPrune {
					continue
				}
				if err := verifyPruneAccess(ctx, osClient); err != nil {
					return nil, ac.wrapErr(ctx, err)
				}
				verifiedPrune = true
			default:
				if err := verifyImageStreamAccess(ctx, imageStreamNS, imageStreamName, verb, osClient); err != nil {
					if access.Action != "pull" {
						return nil, ac.wrapErr(ctx, err)
					}
					possibleCrossMountErrors.Add(imageStreamNS, imageStreamName, ac.wrapErr(ctx, err))
				}
			}

		case "admin":
			switch access.Action {
			case "prune":
				if verifiedPrune {
					continue
				}
				if err := verifyPruneAccess(ctx, osClient); err != nil {
					return nil, ac.wrapErr(ctx, err)
				}
				verifiedPrune = true
			default:
				return nil, ac.wrapErr(ctx, ErrUnsupportedAction)
			}
		default:
			return nil, ac.wrapErr(ctx, ErrUnsupportedResource)
		}
	}

	// deal with any possible cross-mount errors
	for namespaceAndName, err := range possibleCrossMountErrors {
		// If we have no push requests, this can't be a cross-mount request, so error
		if len(pushChecks) == 0 {
			return nil, err
		}
		// If we also requested a push to this ns/name, this isn't a cross-mount request, so error
		if pushChecks[namespaceAndName] {
			return nil, err
		}
	}

	// Conditionally add auth errors we want to handle later to the context
	if !possibleCrossMountErrors.Empty() {
		context.GetLogger(ctx).Debugf("Origin auth: deferring errors: %#v", possibleCrossMountErrors)
		ctx = WithDeferredErrors(ctx, possibleCrossMountErrors)
	}
	// Always add a marker to the context so we know auth was run
	ctx = WithAuthPerformed(ctx)

	return WithUserClient(ctx, osClient), nil
}

func getOpenShiftAPIToken(ctx context.Context, req *http.Request) (string, error) {
	token := ""

	authParts := strings.SplitN(req.Header.Get("Authorization"), " ", 2)
	if len(authParts) != 2 {
		return "", ErrTokenRequired
	}

	switch strings.ToLower(authParts[0]) {
	case "bearer":
		// This is either a direct API token, or a token issued by our docker token handler
		token = authParts[1]
		// Recognize the token issued to anonymous users by our docker token handler
		if token == anonymousToken {
			token = ""
		}

	case "basic":
		_, password, ok := req.BasicAuth()
		if !ok || len(password) == 0 {
			return "", ErrTokenInvalid
		}
		token = password

	default:
		return "", ErrTokenRequired
	}

	return token, nil
}

func verifyOpenShiftUser(ctx context.Context, client client.UsersInterface) error {
	if _, err := client.Users().Get("~"); err != nil {
		context.GetLogger(ctx).Errorf("Get user failed with error: %s", err)
		if kerrors.IsUnauthorized(err) || kerrors.IsForbidden(err) {
			return ErrOpenShiftAccessDenied
		}
		return err
	}

	return nil
}

func verifyImageStreamAccess(ctx context.Context, namespace, imageRepo, verb string, client client.LocalSubjectAccessReviewsNamespacer) error {
	sar := authorizationapi.LocalSubjectAccessReview{
		Action: authorizationapi.Action{
			Verb:         verb,
			Group:        imageapi.GroupName,
			Resource:     "imagestreams/layers",
			ResourceName: imageRepo,
		},
	}
	response, err := client.LocalSubjectAccessReviews(namespace).Create(&sar)

	if err != nil {
		context.GetLogger(ctx).Errorf("OpenShift client error: %s", err)
		if kerrors.IsUnauthorized(err) || kerrors.IsForbidden(err) {
			return ErrOpenShiftAccessDenied
		}
		return err
	}

	if !response.Allowed {
		context.GetLogger(ctx).Errorf("OpenShift access denied: %s", response.Reason)
		return ErrOpenShiftAccessDenied
	}

	return nil
}

func verifyPruneAccess(ctx context.Context, client client.SubjectAccessReviews) error {
	sar := authorizationapi.SubjectAccessReview{
		Action: authorizationapi.Action{
			Verb:     "delete",
			Group:    imageapi.GroupName,
			Resource: "images",
		},
	}
	response, err := client.SubjectAccessReviews().Create(&sar)
	if err != nil {
		context.GetLogger(ctx).Errorf("OpenShift client error: %s", err)
		if kerrors.IsUnauthorized(err) || kerrors.IsForbidden(err) {
			return ErrOpenShiftAccessDenied
		}
		return err
	}
	if !response.Allowed {
		context.GetLogger(ctx).Errorf("OpenShift access denied: %s", response.Reason)
		return ErrOpenShiftAccessDenied
	}
	return nil
}
