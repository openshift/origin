package route

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/storage/names"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	authorizationapi "k8s.io/kubernetes/pkg/apis/authorization"
	kvalidation "k8s.io/kubernetes/pkg/apis/core/validation"
	authorizationclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/authorization/internalversion"

	authorizationutil "github.com/openshift/origin/pkg/authorization/util"
	"github.com/openshift/origin/pkg/route"
	routeapi "github.com/openshift/origin/pkg/route/apis/route"
	"github.com/openshift/origin/pkg/route/apis/route/validation"
)

// HostGeneratedAnnotationKey is the key for an annotation set to "true" if the route's host was generated
const HostGeneratedAnnotationKey = "openshift.io/host.generated"

// Registry is an interface for performing subject access reviews
type SubjectAccessReviewInterface interface {
	Create(sar *authorizationapi.SubjectAccessReview) (result *authorizationapi.SubjectAccessReview, err error)
}

var _ SubjectAccessReviewInterface = authorizationclient.SubjectAccessReviewInterface(nil)

type routeStrategy struct {
	runtime.ObjectTyper
	names.NameGenerator
	route.RouteAllocator
	sarClient SubjectAccessReviewInterface
}

// NewStrategy initializes the default logic that applies when creating and updating
// Route objects via the REST API.
func NewStrategy(allocator route.RouteAllocator, sarClient SubjectAccessReviewInterface) routeStrategy {
	return routeStrategy{
		ObjectTyper:    legacyscheme.Scheme,
		NameGenerator:  names.SimpleNameGenerator,
		RouteAllocator: allocator,
		sarClient:      sarClient,
	}
}

func (routeStrategy) NamespaceScoped() bool {
	return true
}

func (s routeStrategy) PrepareForCreate(ctx apirequest.Context, obj runtime.Object) {
	route := obj.(*routeapi.Route)
	route.Status = routeapi.RouteStatus{}
	stripEmptyDestinationCACertificate(route)
}

func (s routeStrategy) PrepareForUpdate(ctx apirequest.Context, obj, old runtime.Object) {
	route := obj.(*routeapi.Route)
	oldRoute := old.(*routeapi.Route)

	route.Status = oldRoute.Status
	stripEmptyDestinationCACertificate(route)
	// Ignore attempts to clear the spec Host
	// Prevents "immutable field" errors when applying the same route definition used to create
	if len(route.Spec.Host) == 0 {
		route.Spec.Host = oldRoute.Spec.Host
	}
}

// allocateHost allocates a host name ONLY if the route doesn't specify a subdomain wildcard policy and
// the host name on the route is empty and an allocator is configured.
// It must first allocate the shard and may return an error if shard allocation fails.
func (s routeStrategy) allocateHost(ctx apirequest.Context, route *routeapi.Route) field.ErrorList {
	hostSet := len(route.Spec.Host) > 0
	certSet := route.Spec.TLS != nil && (len(route.Spec.TLS.CACertificate) > 0 || len(route.Spec.TLS.Certificate) > 0 || len(route.Spec.TLS.DestinationCACertificate) > 0 || len(route.Spec.TLS.Key) > 0)
	if hostSet || certSet {
		user, ok := apirequest.UserFrom(ctx)
		if !ok {
			return field.ErrorList{field.InternalError(field.NewPath("spec", "host"), fmt.Errorf("unable to verify host field can be set"))}
		}
		res, err := s.sarClient.Create(
			authorizationutil.AddUserToSAR(
				user,
				&authorizationapi.SubjectAccessReview{
					Spec: authorizationapi.SubjectAccessReviewSpec{
						ResourceAttributes: &authorizationapi.ResourceAttributes{
							Namespace:   apirequest.NamespaceValue(ctx),
							Verb:        "create",
							Group:       routeapi.GroupName,
							Resource:    "routes",
							Subresource: "custom-host",
						},
					},
				},
			),
		)
		if err != nil {
			return field.ErrorList{field.InternalError(field.NewPath("spec", "host"), err)}
		}
		if !res.Status.Allowed {
			if hostSet {
				return field.ErrorList{field.Forbidden(field.NewPath("spec", "host"), "you do not have permission to set the host field of the route")}
			}
			return field.ErrorList{field.Forbidden(field.NewPath("spec", "tls"), "you do not have permission to set certificate fields on the route")}
		}
	}

	if route.Spec.WildcardPolicy == routeapi.WildcardPolicySubdomain {
		// Don't allocate a host if subdomain wildcard policy.
		return nil
	}

	if len(route.Spec.Host) == 0 && s.RouteAllocator != nil {
		// TODO: this does not belong here, and should be removed
		shard, err := s.RouteAllocator.AllocateRouterShard(route)
		if err != nil {
			return field.ErrorList{field.InternalError(field.NewPath("spec", "host"), fmt.Errorf("allocation error: %v for route: %#v", err, route))}
		}
		route.Spec.Host = s.RouteAllocator.GenerateHostname(route, shard)
		if route.Annotations == nil {
			route.Annotations = map[string]string{}
		}
		route.Annotations[HostGeneratedAnnotationKey] = "true"
	}
	return nil
}

func (s routeStrategy) Validate(ctx apirequest.Context, obj runtime.Object) field.ErrorList {
	route := obj.(*routeapi.Route)
	errs := s.allocateHost(ctx, route)
	errs = append(errs, validation.ValidateRoute(route)...)
	return errs
}

func (routeStrategy) AllowCreateOnUpdate() bool {
	return false
}

// Canonicalize normalizes the object after validation.
func (routeStrategy) Canonicalize(obj runtime.Object) {
}

func (s routeStrategy) ValidateUpdate(ctx apirequest.Context, obj, old runtime.Object) field.ErrorList {
	oldRoute := old.(*routeapi.Route)
	objRoute := obj.(*routeapi.Route)
	errs := s.validateHostUpdate(ctx, objRoute, oldRoute)
	errs = append(errs, validation.ValidateRouteUpdate(objRoute, oldRoute)...)
	return errs
}

func hasCertificateInfo(tls *routeapi.TLSConfig) bool {
	if tls == nil {
		return false
	}
	return len(tls.Certificate) > 0 ||
		len(tls.Key) > 0 ||
		len(tls.CACertificate) > 0 ||
		len(tls.DestinationCACertificate) > 0
}

func certificateChangeRequiresAuth(route, older *routeapi.Route) bool {
	switch {
	case route.Spec.TLS != nil && older.Spec.TLS != nil:
		a, b := route.Spec.TLS, older.Spec.TLS
		if !hasCertificateInfo(a) {
			// removing certificate info is allowed
			return false
		}
		return a.CACertificate != b.CACertificate ||
			a.Certificate != b.Certificate ||
			a.DestinationCACertificate != b.DestinationCACertificate ||
			a.Key != b.Key
	case route.Spec.TLS != nil:
		// using any default certificate is allowed
		return hasCertificateInfo(route.Spec.TLS)
	default:
		// all other cases we are not adding additional certificate info
		return false
	}
}

func (s routeStrategy) validateHostUpdate(ctx apirequest.Context, route, older *routeapi.Route) field.ErrorList {
	hostChanged := route.Spec.Host != older.Spec.Host
	certChanged := certificateChangeRequiresAuth(route, older)
	if !hostChanged && !certChanged {
		return nil
	}
	user, ok := apirequest.UserFrom(ctx)
	if !ok {
		return field.ErrorList{field.InternalError(field.NewPath("spec", "host"), fmt.Errorf("unable to verify host field can be changed"))}
	}
	res, err := s.sarClient.Create(
		authorizationutil.AddUserToSAR(
			user,
			&authorizationapi.SubjectAccessReview{
				Spec: authorizationapi.SubjectAccessReviewSpec{
					ResourceAttributes: &authorizationapi.ResourceAttributes{
						Namespace:   apirequest.NamespaceValue(ctx),
						Verb:        "update",
						Group:       routeapi.GroupName,
						Resource:    "routes",
						Subresource: "custom-host",
					},
				},
			},
		),
	)
	if err != nil {
		return field.ErrorList{field.InternalError(field.NewPath("spec", "host"), err)}
	}
	if !res.Status.Allowed {
		if hostChanged {
			return kvalidation.ValidateImmutableField(route.Spec.Host, older.Spec.Host, field.NewPath("spec", "host"))
		}

		// if tls is being updated without host being updated, we check if 'create' permission exists on custom-host subresource
		res, err := s.sarClient.Create(
			authorizationutil.AddUserToSAR(
				user,
				&authorizationapi.SubjectAccessReview{
					Spec: authorizationapi.SubjectAccessReviewSpec{
						ResourceAttributes: &authorizationapi.ResourceAttributes{
							Namespace:   apirequest.NamespaceValue(ctx),
							Verb:        "create",
							Group:       routeapi.GroupName,
							Resource:    "routes",
							Subresource: "custom-host",
						},
					},
				},
			),
		)
		if err != nil {
			return field.ErrorList{field.InternalError(field.NewPath("spec", "host"), err)}
		}
		if !res.Status.Allowed {
			if route.Spec.TLS == nil || older.Spec.TLS == nil {
				return kvalidation.ValidateImmutableField(route.Spec.TLS, older.Spec.TLS, field.NewPath("spec", "tls"))
			}
			errs := kvalidation.ValidateImmutableField(route.Spec.TLS.CACertificate, older.Spec.TLS.CACertificate, field.NewPath("spec", "tls", "caCertificate"))
			errs = append(errs, kvalidation.ValidateImmutableField(route.Spec.TLS.Certificate, older.Spec.TLS.Certificate, field.NewPath("spec", "tls", "certificate"))...)
			errs = append(errs, kvalidation.ValidateImmutableField(route.Spec.TLS.DestinationCACertificate, older.Spec.TLS.DestinationCACertificate, field.NewPath("spec", "tls", "destinationCACertificate"))...)
			errs = append(errs, kvalidation.ValidateImmutableField(route.Spec.TLS.Key, older.Spec.TLS.Key, field.NewPath("spec", "tls", "key"))...)
			return errs
		}
	}
	return nil
}

func (routeStrategy) AllowUnconditionalUpdate() bool {
	return false
}

type routeStatusStrategy struct {
	routeStrategy
}

var StatusStrategy = routeStatusStrategy{NewStrategy(nil, nil)}

func (routeStatusStrategy) PrepareForUpdate(ctx apirequest.Context, obj, old runtime.Object) {
	newRoute := obj.(*routeapi.Route)
	oldRoute := old.(*routeapi.Route)
	newRoute.Spec = oldRoute.Spec
}

func (routeStatusStrategy) ValidateUpdate(ctx apirequest.Context, obj, old runtime.Object) field.ErrorList {
	return validation.ValidateRouteStatusUpdate(obj.(*routeapi.Route), old.(*routeapi.Route))
}

const emptyDestinationCertificate = `-----BEGIN COMMENT-----
This is an empty PEM file created to provide backwards compatibility
for reencrypt routes that have no destinationCACertificate. This 
content will only appear for routes accessed via /oapi/v1/routes.
-----END COMMENT-----
`

// stripEmptyDestinationCACertificate removes the empty destinationCACertificate if it matches
// the current route destination CA certificate.
func stripEmptyDestinationCACertificate(route *routeapi.Route) {
	tls := route.Spec.TLS
	if tls == nil || tls.Termination != routeapi.TLSTerminationReencrypt {
		return
	}
	if tls.DestinationCACertificate == emptyDestinationCertificate {
		tls.DestinationCACertificate = ""
	}
}

// DecorateLegacyRouteWithEmptyDestinationCACertificates is used for /oapi/v1 route endpoints
// to prevent legacy clients from seeing an empty destination CA certificate for reencrypt routes,
// which the 'route.openshift.io/v1' endpoint allows. These values are injected in REST responses
// and stripped in PrepareForCreate and PrepareForUpdate.
func DecorateLegacyRouteWithEmptyDestinationCACertificates(obj runtime.Object) error {
	switch t := obj.(type) {
	case *routeapi.Route:
		tls := t.Spec.TLS
		if tls == nil || tls.Termination != routeapi.TLSTerminationReencrypt {
			return nil
		}
		if len(tls.DestinationCACertificate) == 0 {
			tls.DestinationCACertificate = emptyDestinationCertificate
		}
		return nil
	case *routeapi.RouteList:
		for i := range t.Items {
			tls := t.Items[i].Spec.TLS
			if tls == nil || tls.Termination != routeapi.TLSTerminationReencrypt {
				continue
			}
			if len(tls.DestinationCACertificate) == 0 {
				tls.DestinationCACertificate = emptyDestinationCertificate
			}
		}
		return nil
	default:
		return fmt.Errorf("unknown type passed to %T", obj)
	}
}
