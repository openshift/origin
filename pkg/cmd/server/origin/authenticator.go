package origin

import (
	"crypto/x509"
	"fmt"
	"time"

	"k8s.io/apiserver/pkg/authentication/authenticator"
	"k8s.io/apiserver/pkg/authentication/group"
	"k8s.io/apiserver/pkg/authentication/request/anonymous"
	"k8s.io/apiserver/pkg/authentication/request/bearertoken"
	"k8s.io/apiserver/pkg/authentication/request/headerrequest"
	"k8s.io/apiserver/pkg/authentication/request/union"
	"k8s.io/apiserver/pkg/authentication/request/websocket"
	x509request "k8s.io/apiserver/pkg/authentication/request/x509"
	tokencache "k8s.io/apiserver/pkg/authentication/token/cache"
	tokenunion "k8s.io/apiserver/pkg/authentication/token/union"
	kclientsetexternal "k8s.io/kubernetes/pkg/client/clientset_generated/clientset"
	sacontroller "k8s.io/kubernetes/pkg/controller/serviceaccount"
	"k8s.io/kubernetes/pkg/serviceaccount"

	"github.com/openshift/origin/pkg/auth/authenticator/request/paramtoken"
	authnregistry "github.com/openshift/origin/pkg/auth/oauth/registry"
	"github.com/openshift/origin/pkg/auth/userregistry/identitymapper"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	oauthclient "github.com/openshift/origin/pkg/oauth/generated/internalclientset/typed/oauth/internalversion"
	usercache "github.com/openshift/origin/pkg/user/cache"
	userclient "github.com/openshift/origin/pkg/user/generated/internalclientset"
	usertypedclient "github.com/openshift/origin/pkg/user/generated/internalclientset/typed/user/internalversion"
	"k8s.io/client-go/rest"
)

func NewAuthenticator(
	options configapi.MasterConfig,
	privilegedLoopbackConfig *rest.Config,
	informers InformerAccess,
) (authenticator.Request, error) {
	kubeExternalClient, err := kclientsetexternal.NewForConfig(privilegedLoopbackConfig)
	if err != nil {
		return nil, err
	}
	oauthClient, err := oauthclient.NewForConfig(privilegedLoopbackConfig)
	if err != nil {
		return nil, err
	}
	userClient, err := userclient.NewForConfig(privilegedLoopbackConfig)
	if err != nil {
		return nil, err
	}

	// this is safe because the server does a quorum read and we're hitting a "magic" authorizer to get permissions based on system:masters
	// once the cache is added, we won't be paying a double hop cost to etcd on each request, so the simplification will help.
	serviceAccountTokenGetter := sacontroller.NewGetterFromClient(kubeExternalClient)
	apiClientCAs, err := configapi.GetAPIClientCertCAPool(options)
	if err != nil {
		return nil, err
	}

	return newAuthenticator(
		options,
		oauthClient.OAuthAccessTokens(),
		serviceAccountTokenGetter,
		userClient.Users(),
		apiClientCAs,
		usercache.NewGroupCache(informers.GetUserInformers().User().InternalVersion().Groups()),
	)
}

func newAuthenticator(config configapi.MasterConfig, accessTokenGetter oauthclient.OAuthAccessTokenInterface, tokenGetter serviceaccount.ServiceAccountTokenGetter, userGetter usertypedclient.UserResourceInterface, apiClientCAs *x509.CertPool, groupMapper identitymapper.UserToGroupMapper) (authenticator.Request, error) {
	authenticators := []authenticator.Request{}
	tokenAuthenticators := []authenticator.Token{}

	// ServiceAccount token
	if len(config.ServiceAccountConfig.PublicKeyFiles) > 0 {
		publicKeys := []interface{}{}
		for _, keyFile := range config.ServiceAccountConfig.PublicKeyFiles {
			readPublicKeys, err := serviceaccount.ReadPublicKeys(keyFile)
			if err != nil {
				return nil, fmt.Errorf("Error reading service account key file %s: %v", keyFile, err)
			}
			publicKeys = append(publicKeys, readPublicKeys...)
		}
		serviceAccountTokenAuthenticator := serviceaccount.JWTTokenAuthenticator(publicKeys, true, tokenGetter)
		tokenAuthenticators = append(tokenAuthenticators, serviceAccountTokenAuthenticator)
	}

	// OAuth token
	if config.OAuthConfig != nil {
		oauthTokenAuthenticator := authnregistry.NewTokenAuthenticator(accessTokenGetter, userGetter, groupMapper)
		tokenAuthenticators = append(tokenAuthenticators,
			// if you have a bearer token, you're a human (usually)
			// if you change this, have a look at the impersonationFilter where we attach groups to the impersonated user
			group.NewTokenGroupAdder(oauthTokenAuthenticator, []string{bootstrappolicy.AuthenticatedOAuthGroup}))
	}

	if len(tokenAuthenticators) > 0 {
		// Combine all token authenticators
		tokenAuth := tokenunion.New(tokenAuthenticators...)

		// wrap with short cache on success.
		// this means a revoked service account token or access token will be valid for up to 10 seconds.
		// it also means group membership changes on users may take up to 10 seconds to become effective.
		tokenAuth = tokencache.New(tokenAuth, 10*time.Second, 0)

		authenticators = append(authenticators,
			bearertoken.New(tokenAuth),
			websocket.NewProtocolAuthenticator(tokenAuth),
			paramtoken.New("access_token", tokenAuth, true),
		)
	}

	// build cert authenticator
	// TODO: add "system:" prefix in authenticator, limit cert to username
	// TODO: add "system:" prefix to groups in authenticator, limit cert to group name
	opts := x509request.DefaultVerifyOptions()
	opts.Roots = apiClientCAs
	certauth := x509request.New(opts, x509request.CommonNameUserConversion)
	authenticators = append(authenticators, certauth)

	resultingAuthenticator := union.NewFailOnError(authenticators...)

	topLevelAuthenticators := []authenticator.Request{}
	// if we have a front proxy providing authentication configuration, wire it up and it should come first
	if config.AuthConfig.RequestHeader != nil {
		requestHeaderAuthenticator, err := headerrequest.NewSecure(
			config.AuthConfig.RequestHeader.ClientCA,
			config.AuthConfig.RequestHeader.ClientCommonNames,
			config.AuthConfig.RequestHeader.UsernameHeaders,
			config.AuthConfig.RequestHeader.GroupHeaders,
			config.AuthConfig.RequestHeader.ExtraHeaderPrefixes,
		)
		if err != nil {
			return nil, fmt.Errorf("Error building front proxy auth config: %v", err)
		}
		topLevelAuthenticators = append(topLevelAuthenticators, union.New(requestHeaderAuthenticator, resultingAuthenticator))

	} else {
		topLevelAuthenticators = append(topLevelAuthenticators, resultingAuthenticator)

	}
	topLevelAuthenticators = append(topLevelAuthenticators, anonymous.NewAuthenticator())

	return group.NewAuthenticatedGroupAdder(union.NewFailOnError(topLevelAuthenticators...)), nil
}
