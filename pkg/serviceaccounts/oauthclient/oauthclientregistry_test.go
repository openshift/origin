package oauthclient

import (
	"reflect"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	clientgotesting "k8s.io/client-go/testing"
	kapi "k8s.io/kubernetes/pkg/api"
	kapihelper "k8s.io/kubernetes/pkg/api/helper"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"

	ostestclient "github.com/openshift/origin/pkg/client/testclient"
	oauthapi "github.com/openshift/origin/pkg/oauth/apis/oauth"
	oauthapiv1 "github.com/openshift/origin/pkg/oauth/apis/oauth/v1"
	routeapi "github.com/openshift/origin/pkg/route/apis/route"
)

var (
	encoder                 = kapi.Codecs.LegacyCodec(oauthapiv1.SchemeGroupVersion)
	decoder                 = kapi.Codecs.UniversalDecoder()
	serviceAccountsResource = schema.GroupVersionResource{Group: "", Version: "", Resource: "serviceaccounts"}
	secretsResource         = schema.GroupVersionResource{Group: "", Version: "", Resource: "secrets"}
	secretKind              = schema.GroupVersionKind{Group: "", Version: "", Kind: "Secret"}
	routesResource          = schema.GroupVersionResource{Group: "", Version: "", Resource: "routes"}
	routeClientKind         = schema.GroupVersionKind{Group: "", Version: "", Kind: "Route"}
)

func TestGetClient(t *testing.T) {
	testCases := []struct {
		name       string
		clientName string
		kubeClient *fake.Clientset
		osClient   *ostestclient.Fake

		expectedDelegation  bool
		expectedErr         string
		expectedClient      *oauthapi.OAuthClient
		expectedKubeActions []clientgotesting.Action
		expectedOSActions   []clientgotesting.Action
	}{
		{
			name:                "delegate",
			clientName:          "not:serviceaccount",
			kubeClient:          fake.NewSimpleClientset(),
			osClient:            ostestclient.NewSimpleFake(),
			expectedDelegation:  true,
			expectedKubeActions: []clientgotesting.Action{},
			expectedOSActions:   []clientgotesting.Action{},
		},
		{
			name:                "missing sa",
			clientName:          "system:serviceaccount:ns-01:missing-sa",
			kubeClient:          fake.NewSimpleClientset(),
			osClient:            ostestclient.NewSimpleFake(),
			expectedErr:         `serviceaccounts "missing-sa" not found`,
			expectedKubeActions: []clientgotesting.Action{clientgotesting.NewGetAction(serviceAccountsResource, "ns-01", "missing-sa")},
			expectedOSActions:   []clientgotesting.Action{},
		},
		{
			name:       "sa no redirects",
			clientName: "system:serviceaccount:ns-01:default",
			kubeClient: fake.NewSimpleClientset(
				&kapi.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Namespace:   "ns-01",
						Name:        "default",
						Annotations: map[string]string{},
					},
				}),
			osClient:            ostestclient.NewSimpleFake(),
			expectedErr:         `system:serviceaccount:ns-01:default has no redirectURIs; set serviceaccounts.openshift.io/oauth-redirecturi.<some-value>`,
			expectedKubeActions: []clientgotesting.Action{clientgotesting.NewGetAction(serviceAccountsResource, "ns-01", "default")},
			expectedOSActions:   []clientgotesting.Action{},
		},
		{
			name:       "sa no tokens",
			clientName: "system:serviceaccount:ns-01:default",
			kubeClient: fake.NewSimpleClientset(
				&kapi.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Namespace:   "ns-01",
						Name:        "default",
						Annotations: map[string]string{OAuthRedirectModelAnnotationURIPrefix + "one": "http://anywhere"},
					},
				}),
			osClient:    ostestclient.NewSimpleFake(),
			expectedErr: `system:serviceaccount:ns-01:default has no tokens`,
			expectedKubeActions: []clientgotesting.Action{
				clientgotesting.NewGetAction(serviceAccountsResource, "ns-01", "default"),
				clientgotesting.NewListAction(secretsResource, secretKind, "ns-01", metav1.ListOptions{}),
			},
			expectedOSActions: []clientgotesting.Action{},
		},
		{
			name:       "good SA",
			clientName: "system:serviceaccount:ns-01:default",
			kubeClient: fake.NewSimpleClientset(
				&kapi.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Namespace:   "ns-01",
						Name:        "default",
						UID:         types.UID("any"),
						Annotations: map[string]string{OAuthRedirectModelAnnotationURIPrefix + "one": "http://anywhere"},
					},
				},
				&kapi.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "ns-01",
						Name:      "default",
						Annotations: map[string]string{
							kapi.ServiceAccountNameKey: "default",
							kapi.ServiceAccountUIDKey:  "any",
						},
					},
					Type: kapi.SecretTypeServiceAccountToken,
					Data: map[string][]byte{kapi.ServiceAccountTokenKey: []byte("foo")},
				}),
			osClient: ostestclient.NewSimpleFake(),
			expectedClient: &oauthapi.OAuthClient{
				ObjectMeta:        metav1.ObjectMeta{Name: "system:serviceaccount:ns-01:default"},
				ScopeRestrictions: getScopeRestrictionsFor("ns-01", "default"),
				AdditionalSecrets: []string{"foo"},
				RedirectURIs:      []string{"http://anywhere"},
				GrantMethod:       oauthapi.GrantHandlerPrompt,
			},
			expectedKubeActions: []clientgotesting.Action{
				clientgotesting.NewGetAction(serviceAccountsResource, "ns-01", "default"),
				clientgotesting.NewListAction(secretsResource, secretKind, "ns-01", metav1.ListOptions{}),
			},
			expectedOSActions: []clientgotesting.Action{},
		},
		{
			name:       "good SA with valid, simple route redirects",
			clientName: "system:serviceaccount:ns-01:default",
			kubeClient: fake.NewSimpleClientset(
				&kapi.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "ns-01",
						Name:      "default",
						UID:       types.UID("any"),
						Annotations: map[string]string{
							OAuthRedirectModelAnnotationURIPrefix + "one":     "http://anywhere",
							OAuthRedirectModelAnnotationReferencePrefix + "1": buildRedirectObjectReferenceString(routeKind, "route1", ""),
						},
					},
				},
				&kapi.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "ns-01",
						Name:      "default",
						Annotations: map[string]string{
							kapi.ServiceAccountNameKey: "default",
							kapi.ServiceAccountUIDKey:  "any",
						},
					},
					Type: kapi.SecretTypeServiceAccountToken,
					Data: map[string][]byte{kapi.ServiceAccountTokenKey: []byte("foo")},
				}),
			osClient: ostestclient.NewSimpleFake(
				&routeapi.Route{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "ns-01",
						Name:      "route1",
						UID:       types.UID("route1"),
					},
					Spec: routeapi.RouteSpec{
						Path: "/defaultpath",
						TLS:  &routeapi.TLSConfig{},
					},
					Status: routeapi.RouteStatus{
						Ingress: []routeapi.RouteIngress{
							{Host: "example1.com", Conditions: buildValidRouteIngressCondition()},
						},
					},
				},
			),
			expectedClient: &oauthapi.OAuthClient{
				ObjectMeta:        metav1.ObjectMeta{Name: "system:serviceaccount:ns-01:default"},
				ScopeRestrictions: getScopeRestrictionsFor("ns-01", "default"),
				AdditionalSecrets: []string{"foo"},
				RedirectURIs:      []string{"http://anywhere", "https://example1.com/defaultpath"},
				GrantMethod:       oauthapi.GrantHandlerPrompt,
			},
			expectedKubeActions: []clientgotesting.Action{
				clientgotesting.NewGetAction(serviceAccountsResource, "ns-01", "default"),
				clientgotesting.NewListAction(secretsResource, secretKind, "ns-01", metav1.ListOptions{}),
			},
			expectedOSActions: []clientgotesting.Action{
				clientgotesting.NewGetAction(routesResource, "ns-01", "route1"),
			},
		},
		{
			name:       "good SA with invalid route redirects",
			clientName: "system:serviceaccount:ns-01:default",
			kubeClient: fake.NewSimpleClientset(
				&kapi.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "ns-01",
						Name:      "default",
						UID:       types.UID("any"),
						Annotations: map[string]string{
							OAuthRedirectModelAnnotationURIPrefix + "one":     "http://anywhere",
							OAuthRedirectModelAnnotationReferencePrefix + "1": buildRedirectObjectReferenceString(routeKind, "route1", "wronggroup"),
							OAuthRedirectModelAnnotationReferencePrefix + "2": buildRedirectObjectReferenceString("wrongkind", "route1", "route.openshift.io"),
						},
					},
				},
				&kapi.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "ns-01",
						Name:      "default",
						Annotations: map[string]string{
							kapi.ServiceAccountNameKey: "default",
							kapi.ServiceAccountUIDKey:  "any",
						},
					},
					Type: kapi.SecretTypeServiceAccountToken,
					Data: map[string][]byte{kapi.ServiceAccountTokenKey: []byte("foo")},
				}),
			osClient: ostestclient.NewSimpleFake(
				&routeapi.Route{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "ns-01",
						Name:      "route1",
						UID:       types.UID("route1"),
					},
					Spec: routeapi.RouteSpec{
						Path: "/defaultpath",
						TLS:  &routeapi.TLSConfig{},
					},
					Status: routeapi.RouteStatus{
						Ingress: []routeapi.RouteIngress{
							{Host: "example1.com", Conditions: buildValidRouteIngressCondition()},
							{Host: "example2.com", Conditions: buildValidRouteIngressCondition()},
							{Host: "example3.com", Conditions: buildValidRouteIngressCondition()},
						},
					},
				},
			),
			expectedClient: &oauthapi.OAuthClient{
				ObjectMeta:        metav1.ObjectMeta{Name: "system:serviceaccount:ns-01:default"},
				ScopeRestrictions: getScopeRestrictionsFor("ns-01", "default"),
				AdditionalSecrets: []string{"foo"},
				RedirectURIs:      []string{"http://anywhere"},
				GrantMethod:       oauthapi.GrantHandlerPrompt,
			},
			expectedKubeActions: []clientgotesting.Action{
				clientgotesting.NewGetAction(serviceAccountsResource, "ns-01", "default"),
				clientgotesting.NewListAction(secretsResource, secretKind, "ns-01", metav1.ListOptions{}),
			},
			expectedOSActions: []clientgotesting.Action{},
		},
		{
			name:       "good SA with a route that don't have a host",
			clientName: "system:serviceaccount:ns-01:default",
			kubeClient: fake.NewSimpleClientset(
				&kapi.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "ns-01",
						Name:      "default",
						UID:       types.UID("any"),
						Annotations: map[string]string{
							OAuthRedirectModelAnnotationURIPrefix + "one":     "http://anywhere",
							OAuthRedirectModelAnnotationReferencePrefix + "1": buildRedirectObjectReferenceString(routeKind, "route1", ""),
						},
					},
				},
				&kapi.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "ns-01",
						Name:      "default",
						Annotations: map[string]string{
							kapi.ServiceAccountNameKey: "default",
							kapi.ServiceAccountUIDKey:  "any",
						},
					},
					Type: kapi.SecretTypeServiceAccountToken,
					Data: map[string][]byte{kapi.ServiceAccountTokenKey: []byte("foo")},
				}),
			osClient: ostestclient.NewSimpleFake(
				&routeapi.Route{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "ns-01",
						Name:      "route1",
						UID:       types.UID("route1"),
					},
					Spec: routeapi.RouteSpec{
						Path: "/defaultpath",
						TLS:  &routeapi.TLSConfig{},
					},
					Status: routeapi.RouteStatus{
						Ingress: []routeapi.RouteIngress{
							{Host: "", Conditions: buildValidRouteIngressCondition()},
						},
					},
				},
			),
			expectedClient: &oauthapi.OAuthClient{
				ObjectMeta:        metav1.ObjectMeta{Name: "system:serviceaccount:ns-01:default"},
				ScopeRestrictions: getScopeRestrictionsFor("ns-01", "default"),
				AdditionalSecrets: []string{"foo"},
				RedirectURIs:      []string{"http://anywhere"},
				GrantMethod:       oauthapi.GrantHandlerPrompt,
			},
			expectedKubeActions: []clientgotesting.Action{
				clientgotesting.NewGetAction(serviceAccountsResource, "ns-01", "default"),
				clientgotesting.NewListAction(secretsResource, secretKind, "ns-01", metav1.ListOptions{}),
			},
			expectedOSActions: []clientgotesting.Action{
				clientgotesting.NewGetAction(routesResource, "ns-01", "route1"),
			},
		},
		{
			name:       "good SA with routes that don't have hosts, some of which are empty or duplicates",
			clientName: "system:serviceaccount:ns-01:default",
			kubeClient: fake.NewSimpleClientset(
				&kapi.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "ns-01",
						Name:      "default",
						UID:       types.UID("any"),
						Annotations: map[string]string{
							OAuthRedirectModelAnnotationURIPrefix + "one":     "http://anywhere",
							OAuthRedirectModelAnnotationReferencePrefix + "1": buildRedirectObjectReferenceString(routeKind, "route1", "route.openshift.io"),
							OAuthRedirectModelAnnotationReferencePrefix + "2": buildRedirectObjectReferenceString(routeKind, "route2", ""),
							OAuthRedirectModelAnnotationReferencePrefix + "3": buildRedirectObjectReferenceString(routeKind, "missingroute", ""),
						},
					},
				},
				&kapi.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "ns-01",
						Name:      "default",
						Annotations: map[string]string{
							kapi.ServiceAccountNameKey: "default",
							kapi.ServiceAccountUIDKey:  "any",
						},
					},
					Type: kapi.SecretTypeServiceAccountToken,
					Data: map[string][]byte{kapi.ServiceAccountTokenKey: []byte("foo")},
				}),
			osClient: ostestclient.NewSimpleFake(
				&routeapi.Route{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "ns-01",
						Name:      "route1",
						UID:       types.UID("route1"),
					},
					Spec: routeapi.RouteSpec{
						Path: "/defaultpath",
						TLS:  &routeapi.TLSConfig{},
					},
					Status: routeapi.RouteStatus{
						Ingress: []routeapi.RouteIngress{
							{Host: "", Conditions: buildValidRouteIngressCondition()},
							{Host: "a.com", Conditions: buildValidRouteIngressCondition()},
							{Host: ""},
							{Host: "a.com", Conditions: buildValidRouteIngressCondition()},
							{Host: "b.com", Conditions: buildValidRouteIngressCondition()},
						},
					},
				},
				&routeapi.Route{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "ns-01",
						Name:      "route2",
						UID:       types.UID("route2"),
					},
					Spec: routeapi.RouteSpec{
						Path: "/path2",
						TLS:  &routeapi.TLSConfig{},
					},
					Status: routeapi.RouteStatus{
						Ingress: []routeapi.RouteIngress{
							{Host: "a.com", Conditions: buildValidRouteIngressCondition()},
							{Host: "", Conditions: buildValidRouteIngressCondition()},
							{Host: "b.com", Conditions: buildValidRouteIngressCondition()},
							{Host: "b.com"},
							{Host: ""},
						},
					},
				},
			),
			expectedClient: &oauthapi.OAuthClient{
				ObjectMeta:        metav1.ObjectMeta{Name: "system:serviceaccount:ns-01:default"},
				ScopeRestrictions: getScopeRestrictionsFor("ns-01", "default"),
				AdditionalSecrets: []string{"foo"},
				RedirectURIs:      []string{"http://anywhere", "https://a.com/defaultpath", "https://a.com/path2", "https://b.com/defaultpath", "https://b.com/path2"},
				GrantMethod:       oauthapi.GrantHandlerPrompt,
			},
			expectedKubeActions: []clientgotesting.Action{
				clientgotesting.NewGetAction(serviceAccountsResource, "ns-01", "default"),
				clientgotesting.NewListAction(secretsResource, secretKind, "ns-01", metav1.ListOptions{}),
			},
			expectedOSActions: []clientgotesting.Action{
				clientgotesting.NewListAction(routesResource, routeClientKind, "ns-01", metav1.ListOptions{}),
			},
		},
		{
			name:       "host overrides route data",
			clientName: "system:serviceaccount:ns-01:default",
			kubeClient: fake.NewSimpleClientset(
				&kapi.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "ns-01",
						Name:      "default",
						UID:       types.UID("any"),
						Annotations: map[string]string{
							OAuthRedirectModelAnnotationReferencePrefix + "1": buildRedirectObjectReferenceString(routeKind, "route1", ""),
							OAuthRedirectModelAnnotationURIPrefix + "1":       "//redhat.com",
							OAuthRedirectModelAnnotationReferencePrefix + "2": buildRedirectObjectReferenceString(routeKind, "route2", "route.openshift.io"),
							OAuthRedirectModelAnnotationURIPrefix + "2":       "//google.com",
						},
					},
				},
				&kapi.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "ns-01",
						Name:      "default",
						Annotations: map[string]string{
							kapi.ServiceAccountNameKey: "default",
							kapi.ServiceAccountUIDKey:  "any",
						},
					},
					Type: kapi.SecretTypeServiceAccountToken,
					Data: map[string][]byte{kapi.ServiceAccountTokenKey: []byte("foo")},
				}),
			osClient: ostestclient.NewSimpleFake(
				&routeapi.Route{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "ns-01",
						Name:      "route1",
						UID:       types.UID("route1"),
					},
					Spec: routeapi.RouteSpec{
						Path: "/defaultpath",
						TLS:  &routeapi.TLSConfig{},
					},
					Status: routeapi.RouteStatus{
						Ingress: []routeapi.RouteIngress{
							{Host: ""},
						},
					},
				},
				&routeapi.Route{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "ns-01",
						Name:      "route2",
						UID:       types.UID("route2"),
					},
					Spec: routeapi.RouteSpec{
						Path: "/otherpath",
						TLS:  &routeapi.TLSConfig{},
					},
					Status: routeapi.RouteStatus{
						Ingress: []routeapi.RouteIngress{
							{Host: "ignored.com", Conditions: buildValidRouteIngressCondition()},
							{Host: "alsoignored.com", Conditions: buildValidRouteIngressCondition()},
						},
					},
				},
			),
			expectedClient: &oauthapi.OAuthClient{
				ObjectMeta:        metav1.ObjectMeta{Name: "system:serviceaccount:ns-01:default"},
				ScopeRestrictions: getScopeRestrictionsFor("ns-01", "default"),
				AdditionalSecrets: []string{"foo"},
				RedirectURIs:      []string{"https://google.com/otherpath", "https://redhat.com/defaultpath"},
				GrantMethod:       oauthapi.GrantHandlerPrompt,
			},
			expectedKubeActions: []clientgotesting.Action{
				clientgotesting.NewGetAction(serviceAccountsResource, "ns-01", "default"),
				clientgotesting.NewListAction(secretsResource, secretKind, "ns-01", metav1.ListOptions{}),
			},
			expectedOSActions: []clientgotesting.Action{
				clientgotesting.NewListAction(routesResource, routeClientKind, "ns-01", metav1.ListOptions{}),
			},
		},
		{
			name:       "good SA with valid, route redirects using the same route twice",
			clientName: "system:serviceaccount:ns-01:default",
			kubeClient: fake.NewSimpleClientset(
				&kapi.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "ns-01",
						Name:      "default",
						UID:       types.UID("any"),
						Annotations: map[string]string{
							OAuthRedirectModelAnnotationURIPrefix + "1":       "/awesomepath",
							OAuthRedirectModelAnnotationReferencePrefix + "1": buildRedirectObjectReferenceString(routeKind, "route1", ""),
							OAuthRedirectModelAnnotationURIPrefix + "2":       "//:8000",
							OAuthRedirectModelAnnotationReferencePrefix + "2": buildRedirectObjectReferenceString(routeKind, "route1", "route.openshift.io"),
						},
					},
				},
				&kapi.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "ns-01",
						Name:      "default",
						Annotations: map[string]string{
							kapi.ServiceAccountNameKey: "default",
							kapi.ServiceAccountUIDKey:  "any",
						},
					},
					Type: kapi.SecretTypeServiceAccountToken,
					Data: map[string][]byte{kapi.ServiceAccountTokenKey: []byte("foo")},
				}),
			osClient: ostestclient.NewSimpleFake(
				&routeapi.Route{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "ns-01",
						Name:      "route1",
						UID:       types.UID("route1"),
					},
					Spec: routeapi.RouteSpec{
						TLS: &routeapi.TLSConfig{},
					},
					Status: routeapi.RouteStatus{
						Ingress: []routeapi.RouteIngress{
							{Host: "woot.com", Conditions: buildValidRouteIngressCondition()},
						},
					},
				},
			),
			expectedClient: &oauthapi.OAuthClient{
				ObjectMeta:        metav1.ObjectMeta{Name: "system:serviceaccount:ns-01:default"},
				ScopeRestrictions: getScopeRestrictionsFor("ns-01", "default"),
				AdditionalSecrets: []string{"foo"},
				RedirectURIs:      []string{"https://woot.com/awesomepath", "https://woot.com:8000"},
				GrantMethod:       oauthapi.GrantHandlerPrompt,
			},
			expectedKubeActions: []clientgotesting.Action{
				clientgotesting.NewGetAction(serviceAccountsResource, "ns-01", "default"),
				clientgotesting.NewListAction(secretsResource, secretKind, "ns-01", metav1.ListOptions{}),
			},
			expectedOSActions: []clientgotesting.Action{
				clientgotesting.NewGetAction(routesResource, "ns-01", "route1"),
			},
		},
	}

	for _, tc := range testCases {
		delegate := &fakeDelegate{}
		getter := NewServiceAccountOAuthClientGetter(tc.kubeClient.Core(), tc.kubeClient.Core(), tc.osClient, delegate, oauthapi.GrantHandlerPrompt)
		client, err := getter.GetClient(apirequest.NewContext(), tc.clientName, &metav1.GetOptions{})
		switch {
		case len(tc.expectedErr) == 0 && err == nil:
		case len(tc.expectedErr) == 0 && err != nil,
			len(tc.expectedErr) > 0 && err == nil,
			len(tc.expectedErr) > 0 && err != nil && !strings.Contains(err.Error(), tc.expectedErr):
			t.Errorf("%s: expected %#v, got %#v", tc.name, tc.expectedErr, err)
			continue
		}

		if tc.expectedDelegation != delegate.called {
			t.Errorf("%s: expected %#v, got %#v", tc.name, tc.expectedDelegation, delegate.called)
			continue
		}

		if !kapihelper.Semantic.DeepEqual(tc.expectedClient, client) {
			t.Errorf("%s: expected %#v, got %#v", tc.name, tc.expectedClient, client)
			continue
		}

		if !reflect.DeepEqual(tc.expectedKubeActions, tc.kubeClient.Actions()) {
			t.Errorf("%s: expected %#v, got %#v", tc.name, tc.expectedKubeActions, tc.kubeClient.Actions())
			continue
		}

		if !reflect.DeepEqual(tc.expectedOSActions, tc.osClient.Actions()) {
			t.Errorf("%s: expected %#v, got %#v", tc.name, tc.expectedOSActions, tc.osClient.Actions())
			continue
		}
	}

}

type fakeDelegate struct {
	called bool
}

func (d *fakeDelegate) GetClient(ctx apirequest.Context, name string, options *metav1.GetOptions) (*oauthapi.OAuthClient, error) {
	d.called = true
	return nil, nil
}

func TestRedirectURIString(t *testing.T) {
	for _, test := range []struct {
		name     string
		uri      redirectURI
		expected string
	}{
		{
			name: "host with no port",
			uri: redirectURI{
				scheme: "http",
				host:   "example1.com",
				port:   "",
				path:   "/test1",
			},
			expected: "http://example1.com/test1",
		},
		{
			name: "host with port",
			uri: redirectURI{
				scheme: "https",
				host:   "example2.com",
				port:   "8000",
				path:   "/test2",
			},
			expected: "https://example2.com:8000/test2",
		},
	} {
		if test.expected != test.uri.String() {
			t.Errorf("%s: expected %s, got %s", test.name, test.expected, test.uri.String())
		}
	}
}

func TestMerge(t *testing.T) {
	for _, test := range []struct {
		name     string
		uri      redirectURI
		m        model
		expected redirectURI
	}{
		{
			name: "empty model",
			uri: redirectURI{
				scheme: "http",
				host:   "example1.com",
				port:   "9000",
				path:   "/test1",
			},
			m: model{
				scheme: "",
				port:   "",
				path:   "",
			},
			expected: redirectURI{
				scheme: "http",
				host:   "example1.com",
				port:   "9000",
				path:   "/test1",
			},
		},
		{
			name: "full model",
			uri: redirectURI{
				scheme: "http",
				host:   "example1.com",
				port:   "9000",
				path:   "/test1",
			},
			m: model{
				scheme: "https",
				port:   "8000",
				path:   "/ello",
			},
			expected: redirectURI{
				scheme: "https",
				host:   "example1.com",
				port:   "8000",
				path:   "/ello",
			},
		},
		{
			name: "only path",
			uri: redirectURI{
				scheme: "http",
				host:   "example1.com",
				port:   "9000",
				path:   "/test1",
			},
			m: model{
				scheme: "",
				port:   "",
				path:   "/newpath",
			},
			expected: redirectURI{
				scheme: "http",
				host:   "example1.com",
				port:   "9000",
				path:   "/newpath",
			},
		},
	} {
		test.uri.merge(&test.m)
		if test.expected != test.uri {
			t.Errorf("%s: expected %#v, got %#v", test.name, test.expected, test.uri)
		}
	}
}

func TestParseModelsMap(t *testing.T) {
	for _, test := range []struct {
		name        string
		annotations map[string]string
		expected    map[string]model
	}{
		{
			name:        "empty annotations",
			annotations: map[string]string{},
			expected:    map[string]model{},
		},
		{
			name:        "no model annotations",
			annotations: map[string]string{"one": "anywhere"},
			expected:    map[string]model{},
		},
		{
			name: "static URI annotations",
			annotations: map[string]string{
				OAuthRedirectModelAnnotationURIPrefix + "one":        "//google.com",
				OAuthRedirectModelAnnotationURIPrefix + "two":        "justapath",
				OAuthRedirectModelAnnotationURIPrefix + "three":      "http://redhat.com",
				OAuthRedirectModelAnnotationURIPrefix + "four":       "http://hello:90/world",
				OAuthRedirectModelAnnotationURIPrefix + "five":       "scheme0://host0:port0/path0",
				OAuthRedirectModelAnnotationReferencePrefix + "five": buildRedirectObjectReferenceString("kind0", "name0", "group0"),
			},
			expected: map[string]model{
				"one": {
					scheme: "",
					port:   "",
					path:   "",
					group:  "",
					kind:   "",
					name:   "",
					host:   "google.com",
				},
				"two": {
					scheme: "",
					port:   "",
					path:   "justapath",
					group:  "",
					kind:   "",
					name:   "",
				},
				"three": {
					scheme: "http",
					port:   "",
					path:   "",
					group:  "",
					kind:   "",
					name:   "",
					host:   "redhat.com",
				},
				"four": {
					scheme: "http",
					port:   "90",
					path:   "/world",
					group:  "",
					kind:   "",
					name:   "",
					host:   "hello",
				},
				"five": {
					scheme: "scheme0",
					port:   "port0",
					path:   "/path0",
					group:  "group0",
					kind:   "kind0",
					name:   "name0",
					host:   "host0",
				},
			},
		},
		{
			name: "simple model",
			annotations: map[string]string{
				OAuthRedirectModelAnnotationReferencePrefix + "one": buildRedirectObjectReferenceString(routeKind, "route1", ""),
			},
			expected: map[string]model{
				"one": {
					scheme: "",
					port:   "",
					path:   "",
					group:  "",
					kind:   routeKind,
					name:   "route1",
				},
			},
		},
		{
			name: "multiple full models",
			annotations: map[string]string{
				OAuthRedirectModelAnnotationReferencePrefix + "one": buildRedirectObjectReferenceString(routeKind, "route1", ""),
				OAuthRedirectModelAnnotationURIPrefix + "one":       "https://:8000/path1",

				OAuthRedirectModelAnnotationReferencePrefix + "two": buildRedirectObjectReferenceString(routeKind, "route2", "route.openshift.io"),
				OAuthRedirectModelAnnotationURIPrefix + "two":       "http://:9000/path2",
			},
			expected: map[string]model{
				"one": {
					scheme: "https",
					port:   "8000",
					path:   "/path1",
					group:  "",
					kind:   routeKind,
					name:   "route1",
				},
				"two": {
					scheme: "http",
					port:   "9000",
					path:   "/path2",
					group:  "route.openshift.io",
					kind:   routeKind,
					name:   "route2",
				},
			},
		},
	} {
		if !reflect.DeepEqual(test.expected, parseModelsMap(test.annotations, decoder)) {
			t.Errorf("%s: expected %#v, got %#v", test.name, test.expected, parseModelsMap(test.annotations, decoder))
		}
	}
}

func TestGetRedirectURIs(t *testing.T) {
	for _, test := range []struct {
		name      string
		namespace string
		models    modelList
		routes    []*routeapi.Route
		expected  redirectURIList
	}{
		{
			name:      "single ingress routes",
			namespace: "ns01",
			models: modelList{
				{
					scheme: "https",
					port:   "8000",
					path:   "/path1",
					group:  "",
					kind:   routeKind,
					name:   "route1",
				},
				{
					scheme: "http",
					port:   "9000",
					path:   "",
					group:  "",
					kind:   routeKind,
					name:   "route2",
				},
			},
			routes: []*routeapi.Route{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "route1",
						Namespace: "ns01",
					},
					Spec: routeapi.RouteSpec{
						Path: "/pathA",
					},
					Status: routeapi.RouteStatus{
						Ingress: []routeapi.RouteIngress{
							{Host: "exampleA.com", Conditions: buildValidRouteIngressCondition()},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "route2",
						Namespace: "ns01",
					},
					Spec: routeapi.RouteSpec{
						Path: "/pathB",
					},
					Status: routeapi.RouteStatus{
						Ingress: []routeapi.RouteIngress{
							{Host: "exampleB.com", Conditions: buildValidRouteIngressCondition()},
						},
					},
				},
			},
			expected: redirectURIList{
				{
					scheme: "https",
					host:   "exampleA.com",
					port:   "8000",
					path:   "/path1",
				},
				{
					scheme: "http",
					host:   "exampleB.com",
					port:   "9000",
					path:   "/pathB",
				},
			},
		},
		{
			name:      "multiple ingress routes",
			namespace: "ns01",
			models: modelList{
				{
					scheme: "https",
					port:   "8000",
					path:   "/path1",
					group:  "",
					kind:   routeKind,
					name:   "route1",
				},
				{
					scheme: "http",
					port:   "9000",
					path:   "",
					group:  "",
					kind:   routeKind,
					name:   "route2",
				},
				{
					scheme: "http",
					port:   "",
					path:   "/secondroute2path",
					group:  "",
					kind:   routeKind,
					name:   "route2",
				},
			},
			routes: []*routeapi.Route{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "route1",
						Namespace: "ns01",
					},
					Spec: routeapi.RouteSpec{
						Path: "/pathA",
					},
					Status: routeapi.RouteStatus{
						Ingress: []routeapi.RouteIngress{
							{Host: "A.com", Conditions: buildValidRouteIngressCondition()},
							{Host: "B.com", Conditions: buildValidRouteIngressCondition()},
							{Host: "C.com", Conditions: buildValidRouteIngressCondition()},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "route2",
						Namespace: "ns01",
					},
					Spec: routeapi.RouteSpec{
						Path: "/pathB",
					},
					Status: routeapi.RouteStatus{
						Ingress: []routeapi.RouteIngress{
							{Host: "0.com", Conditions: buildValidRouteIngressCondition()},
							{Host: "1.com", Conditions: buildValidRouteIngressCondition()},
						},
					},
				},
			},
			expected: redirectURIList{
				{
					scheme: "https",
					host:   "A.com",
					port:   "8000",
					path:   "/path1",
				},
				{
					scheme: "https",
					host:   "B.com",
					port:   "8000",
					path:   "/path1",
				},
				{
					scheme: "https",
					host:   "C.com",
					port:   "8000",
					path:   "/path1",
				},
				{
					scheme: "http",
					host:   "0.com",
					port:   "9000",
					path:   "/pathB",
				},
				{
					scheme: "http",
					host:   "1.com",
					port:   "9000",
					path:   "/pathB",
				},
				{
					scheme: "http",
					host:   "0.com",
					port:   "",
					path:   "/secondroute2path",
				},
				{
					scheme: "http",
					host:   "1.com",
					port:   "",
					path:   "/secondroute2path",
				},
			},
		},
	} {
		a := buildRouteClient(test.routes)
		actual := test.models.getRedirectURIs(a.redirectURIsFromRoutes(test.namespace, test.models.getNames()))
		if !reflect.DeepEqual(test.expected, actual) {
			t.Errorf("%s: expected %#v, got %#v", test.name, test.expected, actual)
		}
	}
}

func TestRedirectURIsFromRoutes(t *testing.T) {
	for _, test := range []struct {
		name      string
		namespace string
		names     sets.String
		routes    []*routeapi.Route
		expected  map[string]redirectURIList
	}{
		{
			name:      "single route with single ingress",
			namespace: "ns01",
			names:     sets.NewString("routeA"),
			routes: []*routeapi.Route{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "routeA",
						Namespace: "ns01",
					},
					Spec: routeapi.RouteSpec{
						Path: "/pathA",
					},
					Status: routeapi.RouteStatus{
						Ingress: []routeapi.RouteIngress{
							{Host: "exampleA.com", Conditions: buildValidRouteIngressCondition()},
						},
					},
				},
			},
			expected: map[string]redirectURIList{
				"routeA": {
					{
						scheme: "http",
						host:   "exampleA.com",
						port:   "",
						path:   "/pathA",
					},
				},
			},
		},
		{
			name:      "multiple routes with multiple ingresses",
			namespace: "ns01",
			names:     sets.NewString("route0", "route1", "route2"),
			routes: []*routeapi.Route{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "route0",
						Namespace: "ns01",
					},
					Spec: routeapi.RouteSpec{
						Path: "/path0",
					},
					Status: routeapi.RouteStatus{
						Ingress: []routeapi.RouteIngress{
							{Host: "example0A.com", Conditions: buildValidRouteIngressCondition()},
							{Host: "example0B.com", Conditions: buildValidRouteIngressCondition()},
							{Host: "example0C.com", Conditions: buildValidRouteIngressCondition()},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "route1",
						Namespace: "ns01",
					},
					Spec: routeapi.RouteSpec{
						Path: "/path1",
						TLS:  &routeapi.TLSConfig{},
					},
					Status: routeapi.RouteStatus{
						Ingress: []routeapi.RouteIngress{
							{Host: "redhat.com", Conditions: buildValidRouteIngressCondition()},
							{Host: "coreos.com", Conditions: buildValidRouteIngressCondition()},
							{Host: "github.com", Conditions: buildValidRouteIngressCondition()},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "route2",
						Namespace: "ns01",
					},
					Spec: routeapi.RouteSpec{
						Path: "/path2",
						TLS:  &routeapi.TLSConfig{},
					},
					Status: routeapi.RouteStatus{
						Ingress: []routeapi.RouteIngress{
							{Host: "google.com", Conditions: buildValidRouteIngressCondition()},
							{Host: "yahoo.com", Conditions: buildValidRouteIngressCondition()},
							{Host: "bing.com", Conditions: buildValidRouteIngressCondition()},
						},
					},
				},
			},
			expected: map[string]redirectURIList{
				"route0": {
					{
						scheme: "http",
						host:   "example0A.com",
						port:   "",
						path:   "/path0",
					},
					{
						scheme: "http",
						host:   "example0B.com",
						port:   "",
						path:   "/path0",
					},
					{
						scheme: "http",
						host:   "example0C.com",
						port:   "",
						path:   "/path0",
					},
				},
				"route1": {
					{
						scheme: "https",
						host:   "redhat.com",
						port:   "",
						path:   "/path1",
					},
					{
						scheme: "https",
						host:   "coreos.com",
						port:   "",
						path:   "/path1",
					},
					{
						scheme: "https",
						host:   "github.com",
						port:   "",
						path:   "/path1",
					},
				},
				"route2": {
					{
						scheme: "https",
						host:   "google.com",
						port:   "",
						path:   "/path2",
					},
					{
						scheme: "https",
						host:   "yahoo.com",
						port:   "",
						path:   "/path2",
					},
					{
						scheme: "https",
						host:   "bing.com",
						port:   "",
						path:   "/path2",
					},
				},
			},
		},
	} {
		a := buildRouteClient(test.routes)
		if !reflect.DeepEqual(test.expected, a.redirectURIsFromRoutes(test.namespace, test.names)) {
			t.Errorf("%s: expected %#v, got %#v", test.name, test.expected, a.redirectURIsFromRoutes(test.namespace, test.names))
		}
	}
}

func buildRouteClient(routes []*routeapi.Route) saOAuthClientAdapter {
	objects := []runtime.Object{}
	for _, route := range routes {
		objects = append(objects, route)
	}
	return saOAuthClientAdapter{routeClient: ostestclient.NewSimpleFake(objects...)}
}

func buildRedirectObjectReferenceString(kind, name, group string) string {
	ref := &oauthapiv1.OAuthRedirectReference{
		Reference: oauthapiv1.RedirectReference{
			Kind:  kind,
			Name:  name,
			Group: group,
		},
	}
	data, err := runtime.Encode(encoder, ref)
	if err != nil {
		panic(err)
	}
	return string(data)
}

func buildValidRouteIngressCondition() []routeapi.RouteIngressCondition {
	return []routeapi.RouteIngressCondition{{Type: routeapi.RouteAdmitted, Status: kapi.ConditionTrue}}
}
