package oauthclient

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	clientv1 "k8s.io/api/core/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	apiserverserviceaccount "k8s.io/apiserver/pkg/authentication/serviceaccount"
	kcoreclient "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/serviceaccount"

	oauthapi "github.com/openshift/api/oauth/v1"
	routeapi "github.com/openshift/api/route/v1"
	routeclient "github.com/openshift/client-go/route/clientset/versioned/typed/route/v1"
	scopeauthorizer "github.com/openshift/origin/pkg/authorization/authorizer/scope"
	"github.com/openshift/origin/pkg/oauth/registry/oauthclient"
)

const (
	OAuthWantChallengesAnnotationPrefix = "serviceaccounts.openshift.io/oauth-want-challenges"

	// Prefix used for statically specifying redirect URIs for a service account via annotations
	// The value can be partially supplied with the dynamic prefix to override the resource's defaults
	OAuthRedirectModelAnnotationURIPrefix = "serviceaccounts.openshift.io/oauth-redirecturi."

	// Prefix used for dynamically specifying redirect URIs using resources for a service account via annotations
	OAuthRedirectModelAnnotationReferencePrefix = "serviceaccounts.openshift.io/oauth-redirectreference."

	routeKind = "Route"
	// TODO add ingress support
	// IngressKind = "Ingress"
)

var modelPrefixes = []string{
	OAuthRedirectModelAnnotationURIPrefix,
	OAuthRedirectModelAnnotationReferencePrefix,
}

// namesToObjMapperFunc is linked to a given GroupKind.
// Based on the namespace and names provided, it builds a map of resource name to redirect URIs.
// The redirect URIs represent the default values as specified by the resource.
// These values can be overridden by user specified data. Errors returned are informative and non-fatal.
type namesToObjMapperFunc func(namespace string, names sets.String) (map[string]redirectURIList, []error)

var emptyGroupKind = schema.GroupKind{} // Used with static redirect URIs
var routeGroupKind = routeapi.SchemeGroupVersion.WithKind(routeKind).GroupKind()
var legacyRouteGroupKind = routeapi.LegacySchemeGroupVersion.WithKind(routeKind).GroupKind() // to support redirect reference with old group

// TODO add ingress support
// var ingressGroupKind = routeapi.SchemeGroupVersion.WithKind(IngressKind).GroupKind()

type saOAuthClientAdapter struct {
	saClient      kcoreclient.ServiceAccountsGetter
	secretClient  kcoreclient.SecretsGetter
	eventRecorder record.EventRecorder
	routeClient   routeclient.RoutesGetter
	// TODO add ingress support
	//ingressClient ??

	delegate    oauthclient.Getter
	grantMethod oauthapi.GrantHandlerType

	decoder runtime.Decoder
}

// model holds fields that could be used to build redirect URI(s).
// The resource components define where to get the default redirect data from.
// If specified, the uri components are used to override the default data.
// As long as the resulting URI(s) have a scheme and a host, they are considered valid.
type model struct {
	scheme string
	port   string
	path   string
	host   string

	group string
	kind  string
	name  string
}

// getGroupKind is used to determine if a group and kind combination is supported.
func (m *model) getGroupKind() schema.GroupKind {
	return schema.GroupKind{Group: m.group, Kind: m.kind}
}

// updateFromURI updates the data in the model with the user provided URL data.
func (m *model) updateFromURI(u *url.URL) {
	m.scheme, m.host, m.path = u.Scheme, u.Host, u.Path
	if h, p, err := net.SplitHostPort(m.host); err == nil {
		m.host = h
		m.port = p
	}
}

// updateFromReference updates the data in the model with the user provided object reference data.
func (m *model) updateFromReference(r *oauthapi.RedirectReference) {
	m.group, m.kind, m.name = r.Group, r.Kind, r.Name
}

type modelList []model

// getNames determines the unique, non-empty resource names specified by the models.
func (ml modelList) getNames() sets.String {
	data := sets.NewString()
	for _, model := range ml {
		if len(model.name) > 0 {
			data.Insert(model.name)
		}
	}
	return data
}

// getRedirectURIs uses the mapping provided by a namesToObjMapperFunc to enumerate all of the redirect URIs
// based on the name of each resource.  The user provided data in the model overrides the data in the mapping.
// The returned redirect URIs may contain duplicate and invalid entries.  All items in the modelList must have a
// uniform group/kind, and the objMapper must be specifically for that group/kind.
func (ml modelList) getRedirectURIs(objMapper map[string]redirectURIList) redirectURIList {
	var data redirectURIList
	for _, m := range ml {
		if uris, ok := objMapper[m.name]; ok {
			for _, uri := range uris {
				u := uri // Make sure we do not mutate objMapper
				u.merge(&m)
				data = append(data, u)
			}
		}
	}
	return data
}

type redirectURI struct {
	scheme string
	host   string
	port   string
	path   string
}

func (uri *redirectURI) String() string {
	host := uri.host
	if len(uri.port) > 0 {
		host = net.JoinHostPort(host, uri.port)
	}
	return (&url.URL{Scheme: uri.scheme, Host: host, Path: uri.path}).String()
}

// isValid returns true when both scheme and host are non-empty.
func (uri *redirectURI) isValid() bool {
	return len(uri.scheme) > 0 && len(uri.host) > 0
}

type redirectURIList []redirectURI

// extractValidRedirectURIStrings returns the redirect URIs that are valid per `isValid` as strings.
func (rl redirectURIList) extractValidRedirectURIStrings() []string {
	var data []string
	for _, u := range rl {
		if u.isValid() {
			data = append(data, u.String())
		}
	}
	return data
}

// merge overrides the default data in the uri with the user provided data in the model.
func (uri *redirectURI) merge(m *model) {
	if len(m.scheme) > 0 {
		uri.scheme = m.scheme
	}
	if len(m.path) > 0 {
		uri.path = m.path
	}
	if len(m.port) > 0 {
		uri.port = m.port
	}
	if len(m.host) > 0 {
		uri.host = m.host
	}
}

var _ oauthclient.Getter = &saOAuthClientAdapter{}

func NewServiceAccountOAuthClientGetter(
	saClient kcoreclient.ServiceAccountsGetter,
	secretClient kcoreclient.SecretsGetter,
	eventClient kcoreclient.EventInterface,
	routeClient routeclient.RoutesGetter,
	delegate oauthclient.Getter,
	grantMethod oauthapi.GrantHandlerType,
) oauthclient.Getter {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(&kcoreclient.EventSinkImpl{Interface: eventClient})
	recorder := eventBroadcaster.NewRecorder(legacyscheme.Scheme, clientv1.EventSource{Component: "service-account-oauth-client-getter"})
	return &saOAuthClientAdapter{
		saClient:      saClient,
		secretClient:  secretClient,
		eventRecorder: recorder,
		routeClient:   routeClient,
		delegate:      delegate,
		grantMethod:   grantMethod,
		decoder:       legacyscheme.Codecs.UniversalDecoder(),
	}
}

func (a *saOAuthClientAdapter) Get(name string, options metav1.GetOptions) (*oauthapi.OAuthClient, error) {
	var err error
	saNamespace, saName, err := apiserverserviceaccount.SplitUsername(name)
	if err != nil {
		return a.delegate.Get(name, options)
	}

	sa, err := a.saClient.ServiceAccounts(saNamespace).Get(saName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	var saErrors []error
	var failReason string
	// Create a warning event combining the collected annotation errors upon failure.
	defer func() {
		if err != nil && len(saErrors) > 0 && len(failReason) > 0 {
			a.eventRecorder.Event(sa, kapi.EventTypeWarning, failReason, utilerrors.NewAggregate(saErrors).Error())
		}
	}()

	redirectURIs := []string{}
	modelsMap, errs := parseModelsMap(sa.Annotations, a.decoder)
	if len(errs) > 0 {
		saErrors = append(saErrors, errs...)
	}

	if len(modelsMap) > 0 {
		uris, extractErrors := a.extractRedirectURIs(modelsMap, saNamespace)
		if len(uris) > 0 {
			redirectURIs = append(redirectURIs, uris.extractValidRedirectURIStrings()...)
		}
		if len(extractErrors) > 0 {
			saErrors = append(saErrors, extractErrors...)
		}
	}
	if len(redirectURIs) == 0 {
		err = fmt.Errorf("%v has no redirectURIs; set %v<some-value>=<redirect> or create a dynamic URI using %v<some-value>=<reference>",
			name, OAuthRedirectModelAnnotationURIPrefix, OAuthRedirectModelAnnotationReferencePrefix,
		)
		failReason = "NoSAOAuthRedirectURIs"
		saErrors = append(saErrors, err)
		return nil, err
	}

	tokens, err := a.getServiceAccountTokens(sa)
	if err != nil {
		return nil, err
	}
	if len(tokens) == 0 {
		err = fmt.Errorf("%v has no tokens", name)
		failReason = "NoSAOAuthTokens"
		saErrors = append(saErrors, err)
		return nil, err
	}

	saWantsChallenges, _ := strconv.ParseBool(sa.Annotations[OAuthWantChallengesAnnotationPrefix])

	saClient := &oauthapi.OAuthClient{
		ObjectMeta:            metav1.ObjectMeta{Name: name},
		ScopeRestrictions:     getScopeRestrictionsFor(saNamespace, saName),
		AdditionalSecrets:     tokens,
		RespondWithChallenges: saWantsChallenges,

		// TODO update this to allow https redirection to any
		// 1. service IP (useless in general)
		// 2. service DNS (useless in general)
		// 3. loopback? (useful, but maybe a bit weird)
		RedirectURIs: sets.NewString(redirectURIs...).List(),
		GrantMethod:  a.grantMethod,
	}
	return saClient, nil
}

// parseModelsMap builds a map of model name to model using a service account's annotations.
// The model name is only used for building the map (it ties together the uri and reference annotations)
// and serves no functional purpose other than making testing easier. Errors returned are informative and non-fatal.
func parseModelsMap(annotations map[string]string, decoder runtime.Decoder) (map[string]model, []error) {
	models := map[string]model{}
	parseErrors := []error{}
	for key, value := range annotations {
		prefix, name, ok := parseModelPrefixName(key)
		if !ok {
			continue
		}
		m := models[name]
		switch prefix {
		case OAuthRedirectModelAnnotationURIPrefix:
			if u, err := url.Parse(value); err == nil {
				m.updateFromURI(u)
			} else {
				parseErrors = append(parseErrors, err)
			}
		case OAuthRedirectModelAnnotationReferencePrefix:
			r := &oauthapi.OAuthRedirectReference{}
			if err := runtime.DecodeInto(decoder, []byte(value), r); err == nil {
				m.updateFromReference(&r.Reference)
			} else {
				parseErrors = append(parseErrors, err)
			}
		}
		models[name] = m
	}
	return models, parseErrors
}

// parseModelPrefixName determines if the given key is a model prefix.
// Returns what prefix was used, the name of the model, and true if a model prefix was actually used.
func parseModelPrefixName(key string) (string, string, bool) {
	for _, prefix := range modelPrefixes {
		if strings.HasPrefix(key, prefix) {
			return prefix, key[len(prefix):], true
		}
	}
	return "", "", false
}

// extractRedirectURIs builds redirect URIs using the given models and namespace.
// The returned redirect URIs may contain duplicates and invalid entries. Errors returned are informative and non-fatal.
func (a *saOAuthClientAdapter) extractRedirectURIs(modelsMap map[string]model, namespace string) (redirectURIList, []error) {
	var data redirectURIList
	routeErrors := []error{}
	groupKindModelListMapper := map[schema.GroupKind]modelList{} // map of GroupKind to all models belonging to it
	groupKindModelToURI := map[schema.GroupKind]namesToObjMapperFunc{
		routeGroupKind: a.redirectURIsFromRoutes,
		// TODO add support for ingresses by creating the appropriate GroupKind and namesToObjMapperFunc
		// ingressGroupKind: a.redirectURIsFromIngresses,
	}

	for _, m := range modelsMap {
		gk := m.getGroupKind()
		if gk == legacyRouteGroupKind {
			gk = routeGroupKind // support legacy route group without doing extra API calls
		}
		if len(m.name) == 0 && gk == emptyGroupKind { // Is this a static redirect URI?
			uri := redirectURI{} // No defaults wanted
			uri.merge(&m)
			data = append(data, uri)
		} else if _, ok := groupKindModelToURI[gk]; ok { // a GroupKind is valid if we have a namesToObjMapperFunc to handle it
			groupKindModelListMapper[gk] = append(groupKindModelListMapper[gk], m)
		}
	}

	for gk, models := range groupKindModelListMapper {
		if names := models.getNames(); names.Len() > 0 {
			objMapper, errs := groupKindModelToURI[gk](namespace, names)
			if len(objMapper) > 0 {
				data = append(data, models.getRedirectURIs(objMapper)...)
			}
			if len(errs) > 0 {
				routeErrors = append(routeErrors, errs...)
			}
		}
	}

	return data, routeErrors
}

// redirectURIsFromRoutes is the namesToObjMapperFunc specific to Routes.
// Returns a map of route name to redirect URIs that contain the default data as specified by the route's ingresses.
// Errors returned are informative and non-fatal.
func (a *saOAuthClientAdapter) redirectURIsFromRoutes(namespace string, osRouteNames sets.String) (map[string]redirectURIList, []error) {
	var routes []routeapi.Route
	routeErrors := []error{}
	routeInterface := a.routeClient.Routes(namespace)
	if osRouteNames.Len() > 1 {
		if r, err := routeInterface.List(metav1.ListOptions{}); err == nil {
			routes = r.Items
		} else {
			routeErrors = append(routeErrors, err)
		}
	} else {
		if r, err := routeInterface.Get(osRouteNames.List()[0], metav1.GetOptions{}); err == nil {
			routes = append(routes, *r)
		} else {
			routeErrors = append(routeErrors, err)
		}
	}
	routeMap := map[string]redirectURIList{}
	for _, route := range routes {
		if osRouteNames.Has(route.Name) {
			routeMap[route.Name] = redirectURIsFromRoute(&route)
		}
	}
	return routeMap, routeErrors
}

// redirectURIsFromRoute returns a list of redirect URIs that contain the default data as specified by the given route's ingresses.
func redirectURIsFromRoute(route *routeapi.Route) redirectURIList {
	var uris redirectURIList
	uri := redirectURI{scheme: "https"} // Default to TLS
	uri.path = route.Spec.Path
	if route.Spec.TLS == nil {
		uri.scheme = "http"
	}
	for _, ingress := range route.Status.Ingress {
		if !isRouteIngressValid(&ingress) {
			continue
		}
		u := uri // Copy to avoid mutating the base uri
		u.host = ingress.Host
		uris = append(uris, u)
	}
	// If we get this far we know the Route does actually exist, so we need to have at least one uri
	// to allow the user to override it in their annotations in case there is no valid ingress
	// `extractValidRedirectURIStrings` guarantees that we eventually have the minimum set of required fields
	if len(uris) == 0 {
		uris = append(uris, uri)
	}
	return uris
}

// isRouteIngressValid determines if the RouteIngress has a host and that its conditions has an element with Type=RouteAdmitted and Status=ConditionTrue
func isRouteIngressValid(routeIngress *routeapi.RouteIngress) bool {
	if len(routeIngress.Host) == 0 {
		return false
	}
	for _, condition := range routeIngress.Conditions {
		if condition.Type == routeapi.RouteAdmitted && condition.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func getScopeRestrictionsFor(namespace, name string) []oauthapi.ScopeRestriction {
	return []oauthapi.ScopeRestriction{
		{ExactValues: []string{
			scopeauthorizer.UserInfo,
			scopeauthorizer.UserAccessCheck,
			scopeauthorizer.UserListScopedProjects,
			scopeauthorizer.UserListAllProjects,
		}},
		{ClusterRole: &oauthapi.ClusterRoleScopeRestriction{RoleNames: []string{"*"}, Namespaces: []string{namespace}, AllowEscalation: true}},
	}
}

// getServiceAccountTokens returns all ServiceAccountToken secrets for the given ServiceAccount
func (a *saOAuthClientAdapter) getServiceAccountTokens(sa *corev1.ServiceAccount) ([]string, error) {
	allSecrets, err := a.secretClient.Secrets(sa.Namespace).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	tokens := []string{}
	for i := range allSecrets.Items {
		secret := &allSecrets.Items[i]
		if serviceaccount.IsServiceAccountToken(secret, sa) {
			tokens = append(tokens, string(secret.Data[kapi.ServiceAccountTokenKey]))
		}
	}
	return tokens, nil
}
