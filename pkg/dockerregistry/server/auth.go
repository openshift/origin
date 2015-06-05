package server

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"

	log "github.com/Sirupsen/logrus"
	ctxu "github.com/docker/distribution/context"
	registryauth "github.com/docker/distribution/registry/auth"
	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/client"
	"golang.org/x/net/context"
)

func init() {
	registryauth.Register("openshift", registryauth.InitFunc(newAccessController))
}

type contextKey int

var userClientKey contextKey = 0

func WithUserClient(parent context.Context, userClient *client.Client) context.Context {
	return context.WithValue(parent, userClientKey, userClient)
}

func UserClientFrom(ctx context.Context) (*client.Client, bool) {
	userClient, ok := ctx.Value(userClientKey).(*client.Client)
	return userClient, ok
}

type AccessController struct {
	realm string
}

var _ registryauth.AccessController = &AccessController{}

type authChallenge struct {
	realm string
	err   error
}

var _ registryauth.Challenge = &authChallenge{}

// Errors used and exported by this package.
var (
	ErrTokenRequired          = errors.New("authorization header with basic token required")
	ErrTokenInvalid           = errors.New("failed to decode basic token")
	ErrOpenShiftTokenRequired = errors.New("expected openshift bearer token as password for basic token to registry")
	ErrNamespaceRequired      = errors.New("repository namespace required")
	ErrOpenShiftAccessDenied  = errors.New("openshift access denied")
)

func newAccessController(options map[string]interface{}) (registryauth.AccessController, error) {
	log.Info("Using OpenShift Auth handler")
	realm, ok := options["realm"].(string)
	if !ok {
		// Default to openshift if not present
		realm = "openshift"
	}
	return &AccessController{realm: realm}, nil
}

// Error returns the internal error string for this authChallenge.
func (ac *authChallenge) Error() string {
	return ac.err.Error()
}

// ServeHttp handles writing the challenge response
// by setting the challenge header and status code.
func (ac *authChallenge) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// WWW-Authenticate response challenge header.
	// See https://tools.ietf.org/html/rfc6750#section-3
	str := fmt.Sprintf("Basic realm=%s", ac.realm)
	if ac.err != nil {
		str = fmt.Sprintf("%s,error=%q", str, ac.Error())
	}
	w.Header().Add("WWW-Authenticate", str)
	w.WriteHeader(http.StatusUnauthorized)
}

// Authorized handles checking whether the given request is authorized
// for actions on resources allowed by openshift.
func (ac *AccessController) Authorized(ctx context.Context, accessRecords ...registryauth.Access) (context.Context, error) {
	req, err := ctxu.GetRequest(ctx)
	if err != nil {
		return nil, err
	}

	// TODO try to find a better way to handle this
	if req.URL.Path == "/healthz" {
		return ctx, nil
	}

	challenge := &authChallenge{realm: ac.realm}

	authParts := strings.SplitN(req.Header.Get("Authorization"), " ", 2)
	if len(authParts) != 2 || strings.ToLower(authParts[0]) != "basic" {
		challenge.err = ErrTokenRequired
		return nil, challenge
	}
	basicToken := authParts[1]

	payload, err := base64.StdEncoding.DecodeString(basicToken)
	if err != nil {
		log.Errorf("Basic token decode failed: %s", err)
		challenge.err = ErrTokenInvalid
		return nil, challenge
	}

	osAuthParts := strings.SplitN(string(payload), ":", 2)
	if len(osAuthParts) != 2 {
		challenge.err = ErrOpenShiftTokenRequired
		return nil, challenge
	}
	bearerToken := osAuthParts[1]

	client, err := NewUserOpenShiftClient(bearerToken)
	if err != nil {
		return nil, err
	}

	// In case of docker login, hits endpoint /v2
	if len(accessRecords) == 0 {
		err = verifyOpenShiftUser(client)
		if err != nil {
			challenge.err = err
			return nil, challenge
		}
	}

	for _, access := range accessRecords {
		log.Debugf("OpenShift auth: checking for access to %s:%s:%s", access.Resource.Type, access.Resource.Name, access.Action)

		switch access.Resource.Type {
		case "repository":
			repoParts := strings.SplitN(access.Resource.Name, "/", 2)
			if len(repoParts) != 2 {
				challenge.err = ErrNamespaceRequired
				return nil, challenge
			}

			verb := ""
			switch access.Action {
			case "push":
				verb = "update"
			case "pull":
				verb = "get"
			default:
				challenge.err = fmt.Errorf("Unknown action: %s", access.Action)
				return nil, challenge
			}

			if err := verifyImageStreamAccess(repoParts[0], repoParts[1], verb, client); err != nil {
				challenge.err = err
				return nil, challenge
			}

			return WithUserClient(ctx, client), nil
		case "admin":
			switch access.Action {
			case "prune":
				if err := verifyPruneAccess(client); err != nil {
					challenge.err = err
					return nil, challenge
				}

				return ctx, nil
			default:
				challenge.err = fmt.Errorf("Unknown action: %s", access.Action)
				return nil, challenge
			}
		}
	}

	return ctx, nil
}

func verifyOpenShiftUser(client *client.Client) error {
	if _, err := client.Users().Get("~"); err != nil {
		log.Errorf("Get user failed with error: %s", err)
		return ErrOpenShiftAccessDenied
	}
	return nil
}

func verifyImageStreamAccess(namespace, imageRepo, verb string, client *client.Client) error {
	sar := authorizationapi.SubjectAccessReview{
		Verb:         verb,
		Resource:     "imagestreams/layers",
		ResourceName: imageRepo,
	}
	response, err := client.SubjectAccessReviews(namespace).Create(&sar)
	if err != nil {
		log.Errorf("OpenShift client error: %s", err)
		return ErrOpenShiftAccessDenied
	}
	if !response.Allowed {
		log.Errorf("OpenShift access denied: %s", response.Reason)
		return ErrOpenShiftAccessDenied
	}
	return nil
}

func verifyPruneAccess(client *client.Client) error {
	sar := authorizationapi.SubjectAccessReview{
		Verb:     "delete",
		Resource: "images",
	}
	response, err := client.ClusterSubjectAccessReviews().Create(&sar)
	if err != nil {
		log.Errorf("OpenShift client error: %s", err)
		return ErrOpenShiftAccessDenied
	}
	if !response.Allowed {
		log.Errorf("OpenShift access denied: %s", response.Reason)
		return ErrOpenShiftAccessDenied
	}
	return nil
}
