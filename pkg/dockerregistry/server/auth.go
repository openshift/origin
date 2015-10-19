package server

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"

	log "github.com/Sirupsen/logrus"
	context "github.com/docker/distribution/context"
	registryauth "github.com/docker/distribution/registry/auth"
	kerrors "k8s.io/kubernetes/pkg/api/errors"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/client"
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
	// Challenging errors
	ErrTokenRequired          = errors.New("authorization header with basic token required")
	ErrTokenInvalid           = errors.New("failed to decode basic token")
	ErrOpenShiftTokenRequired = errors.New("expected bearer token as password for basic token to registry")
	ErrOpenShiftAccessDenied  = errors.New("access denied")

	// Non-challenging errors
	ErrNamespaceRequired   = errors.New("repository namespace required")
	ErrUnsupportedAction   = errors.New("unsupported action")
	ErrUnsupportedResource = errors.New("unsupported resource")
)

func newAccessController(options map[string]interface{}) (registryauth.AccessController, error) {
	log.Info("Using Origin Auth handler")
	realm, ok := options["realm"].(string)
	if !ok {
		// Default to openshift if not present
		realm = "origin"
	}
	return &AccessController{realm: realm}, nil
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

// wrapErr wraps errors related to authorization in an authChallenge error that will present a WWW-Authenticate challenge response
func (ac *AccessController) wrapErr(err error) error {
	switch err {
	case ErrTokenRequired, ErrTokenInvalid, ErrOpenShiftTokenRequired, ErrOpenShiftAccessDenied:
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
		return nil, ac.wrapErr(err)
	}

	// TODO: change this to an anonymous Access record, don't require a token for it, and fold into the access record check look below
	if req.URL.Path == "/healthz" {
		return ctx, nil
	}

	bearerToken, err := getToken(req)
	if err != nil {
		return nil, ac.wrapErr(err)
	}

	client, err := NewUserOpenShiftClient(bearerToken)
	if err != nil {
		return nil, ac.wrapErr(err)
	}

	// In case of docker login, hits endpoint /v2
	if len(accessRecords) == 0 {
		if err := verifyOpenShiftUser(client); err != nil {
			return nil, ac.wrapErr(err)
		}
	}

	verifiedPrune := false

	// Validate all requested accessRecords
	// Only return failure errors from this loop. Success should continue to validate all records
	for _, access := range accessRecords {
		log.Debugf("Origin auth: checking for access to %s:%s:%s", access.Resource.Type, access.Resource.Name, access.Action)

		switch access.Resource.Type {
		case "repository":
			imageStreamNS, imageStreamName, err := getNamespaceName(access.Resource.Name)
			if err != nil {
				return nil, ac.wrapErr(err)
			}

			verb := ""
			switch access.Action {
			case "push":
				verb = "update"
			case "pull":
				verb = "get"
			case "*":
				verb = "prune"
			default:
				return nil, ac.wrapErr(ErrUnsupportedAction)
			}

			switch verb {
			case "prune":
				if verifiedPrune {
					continue
				}
				if err := verifyPruneAccess(client); err != nil {
					return nil, ac.wrapErr(err)
				}
				verifiedPrune = true
			default:
				if err := verifyImageStreamAccess(imageStreamNS, imageStreamName, verb, client); err != nil {
					return nil, ac.wrapErr(err)
				}
			}

		case "admin":
			switch access.Action {
			case "prune":
				if verifiedPrune {
					continue
				}
				if err := verifyPruneAccess(client); err != nil {
					return nil, ac.wrapErr(err)
				}
				verifiedPrune = true
			default:
				return nil, ac.wrapErr(ErrUnsupportedAction)
			}
		default:
			return nil, ac.wrapErr(ErrUnsupportedResource)
		}
	}

	return WithUserClient(ctx, client), nil
}

func getNamespaceName(resourceName string) (string, string, error) {
	repoParts := strings.SplitN(resourceName, "/", 2)
	if len(repoParts) != 2 {
		return "", "", ErrNamespaceRequired
	}
	ns := repoParts[0]
	if len(ns) == 0 {
		return "", "", ErrNamespaceRequired
	}
	name := repoParts[1]
	if len(name) == 0 {
		return "", "", ErrNamespaceRequired
	}
	return ns, name, nil
}

func getToken(req *http.Request) (string, error) {
	authParts := strings.SplitN(req.Header.Get("Authorization"), " ", 2)
	if len(authParts) != 2 || strings.ToLower(authParts[0]) != "basic" {
		return "", ErrTokenRequired
	}
	basicToken := authParts[1]

	payload, err := base64.StdEncoding.DecodeString(basicToken)
	if err != nil {
		log.Errorf("Basic token decode failed: %s", err)
		return "", ErrTokenInvalid
	}

	osAuthParts := strings.SplitN(string(payload), ":", 2)
	if len(osAuthParts) != 2 {
		return "", ErrOpenShiftTokenRequired
	}

	bearerToken := osAuthParts[1]
	return bearerToken, nil
}

func verifyOpenShiftUser(client *client.Client) error {
	if _, err := client.Users().Get("~"); err != nil {
		log.Errorf("Get user failed with error: %s", err)
		if kerrors.IsUnauthorized(err) || kerrors.IsForbidden(err) {
			return ErrOpenShiftAccessDenied
		}
		return err
	}

	return nil
}

func verifyImageStreamAccess(namespace, imageRepo, verb string, client *client.Client) error {
	sar := authorizationapi.LocalSubjectAccessReview{
		Action: authorizationapi.AuthorizationAttributes{
			Verb:         verb,
			Resource:     "imagestreams/layers",
			ResourceName: imageRepo,
		},
	}
	response, err := client.LocalSubjectAccessReviews(namespace).Create(&sar)

	if err != nil {
		log.Errorf("OpenShift client error: %s", err)
		if kerrors.IsUnauthorized(err) || kerrors.IsForbidden(err) {
			return ErrOpenShiftAccessDenied
		}
		return err
	}

	if !response.Allowed {
		log.Errorf("OpenShift access denied: %s", response.Reason)
		return ErrOpenShiftAccessDenied
	}

	return nil
}

func verifyPruneAccess(client *client.Client) error {
	sar := authorizationapi.SubjectAccessReview{
		Action: authorizationapi.AuthorizationAttributes{
			Verb:     "delete",
			Resource: "images",
		},
	}
	response, err := client.SubjectAccessReviews().Create(&sar)
	if err != nil {
		log.Errorf("OpenShift client error: %s", err)
		if kerrors.IsUnauthorized(err) || kerrors.IsForbidden(err) {
			return ErrOpenShiftAccessDenied
		}
		return err
	}
	if !response.Allowed {
		log.Errorf("OpenShift access denied: %s", response.Reason)
		return ErrOpenShiftAccessDenied
	}
	return nil
}
