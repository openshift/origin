package oauthclient

import (
	"reflect"
	"strings"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"
	"k8s.io/kubernetes/pkg/types"

	oauthapi "github.com/openshift/origin/pkg/oauth/api"
)

func TestGetClient(t *testing.T) {
	testCases := []struct {
		name       string
		clientName string
		kubeClient *ktestclient.Fake

		expectedDelegation bool
		expectedErr        string
		expectedClient     *oauthapi.OAuthClient
		expectedActions    []ktestclient.Action
	}{
		{
			name:               "delegate",
			clientName:         "not:serviceaccount",
			kubeClient:         ktestclient.NewSimpleFake(),
			expectedDelegation: true,
			expectedActions:    []ktestclient.Action{},
		},
		{
			name:            "missing sa",
			clientName:      "system:serviceaccount:ns-01:missing-sa",
			kubeClient:      ktestclient.NewSimpleFake(),
			expectedErr:     `ServiceAccount "missing-sa" not found`,
			expectedActions: []ktestclient.Action{ktestclient.NewGetAction("serviceaccounts", "ns-01", "missing-sa")},
		},
		{
			name:       "sa no redirects",
			clientName: "system:serviceaccount:ns-01:default",
			kubeClient: ktestclient.NewSimpleFake(
				&kapi.ServiceAccount{
					ObjectMeta: kapi.ObjectMeta{
						Namespace:   "ns-01",
						Name:        "default",
						Annotations: map[string]string{},
					},
				}),
			expectedErr:     `system:serviceaccount:ns-01:default has no redirectURIs; set serviceaccounts.openshift.io/oauth-redirecturi.<some-value>`,
			expectedActions: []ktestclient.Action{ktestclient.NewGetAction("serviceaccounts", "ns-01", "default")},
		},
		{
			name:       "sa no tokens",
			clientName: "system:serviceaccount:ns-01:default",
			kubeClient: ktestclient.NewSimpleFake(
				&kapi.ServiceAccount{
					ObjectMeta: kapi.ObjectMeta{
						Namespace:   "ns-01",
						Name:        "default",
						Annotations: map[string]string{OAuthRedirectURISecretAnnotationPrefix + "one": "anywhere"},
					},
				}),
			expectedErr: `system:serviceaccount:ns-01:default has no tokens`,
			expectedActions: []ktestclient.Action{
				ktestclient.NewGetAction("serviceaccounts", "ns-01", "default"),
				ktestclient.NewListAction("secrets", "ns-01", kapi.ListOptions{}),
			},
		},
		{
			name:       "good SA",
			clientName: "system:serviceaccount:ns-01:default",
			kubeClient: ktestclient.NewSimpleFake(
				&kapi.ServiceAccount{
					ObjectMeta: kapi.ObjectMeta{
						Namespace:   "ns-01",
						Name:        "default",
						UID:         types.UID("any"),
						Annotations: map[string]string{OAuthRedirectURISecretAnnotationPrefix + "one": "anywhere"},
					},
				},
				&kapi.Secret{
					ObjectMeta: kapi.ObjectMeta{
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
			expectedClient: &oauthapi.OAuthClient{
				ObjectMeta:        kapi.ObjectMeta{Name: "system:serviceaccount:ns-01:default"},
				ScopeRestrictions: getScopeRestrictionsFor("ns-01", "default"),
				AdditionalSecrets: []string{"foo"},
				RedirectURIs:      []string{"anywhere"},
				GrantMethod:       oauthapi.GrantHandlerPrompt,
			},
			expectedActions: []ktestclient.Action{
				ktestclient.NewGetAction("serviceaccounts", "ns-01", "default"),
				ktestclient.NewListAction("secrets", "ns-01", kapi.ListOptions{}),
			},
		},
	}

	for _, tc := range testCases {
		delegate := &fakeDelegate{}
		getter := NewServiceAccountOAuthClientGetter(tc.kubeClient, tc.kubeClient, delegate, oauthapi.GrantHandlerPrompt)
		client, err := getter.GetClient(kapi.NewContext(), tc.clientName)
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

		if !kapi.Semantic.DeepEqual(tc.expectedClient, client) {
			t.Errorf("%s: expected %#v, got %#v", tc.name, tc.expectedClient, client)
			continue
		}

		if !reflect.DeepEqual(tc.expectedActions, tc.kubeClient.Actions()) {
			t.Errorf("%s: expected %#v, got %#v", tc.name, tc.expectedActions, tc.kubeClient.Actions())
			continue
		}
	}

}

type fakeDelegate struct {
	called bool
}

func (d *fakeDelegate) GetClient(ctx kapi.Context, name string) (*oauthapi.OAuthClient, error) {
	d.called = true
	return nil, nil
}
