package openshiftkubeapiserver

import (
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
	"k8s.io/apiserver/pkg/authentication/token/cache"
	tokencache "k8s.io/apiserver/pkg/authentication/token/cache"
	tokenunion "k8s.io/apiserver/pkg/authentication/token/union"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/apiserver/pkg/server/certs"
	webhooktoken "k8s.io/apiserver/plugin/pkg/authenticator/token/webhook"
	kclientsetexternal "k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/keyutil"
	sacontroller "k8s.io/kubernetes/pkg/controller/serviceaccount"
	"k8s.io/kubernetes/pkg/serviceaccount"

	configv1 "github.com/openshift/api/config/v1"
	kubecontrolplanev1 "github.com/openshift/api/kubecontrolplane/v1"
	osinv1 "github.com/openshift/api/osin/v1"
	oauthclient "github.com/openshift/client-go/oauth/clientset/versioned/typed/oauth/v1"
	oauthclientlister "github.com/openshift/client-go/oauth/listers/oauth/v1"
	userclient "github.com/openshift/client-go/user/clientset/versioned"
	usertypedclient "github.com/openshift/client-go/user/clientset/versioned/typed/user/v1"
	userinformer "github.com/openshift/client-go/user/informers/externalversions/user/v1"
	bootstrap "github.com/openshift/library-go/pkg/authentication/bootstrapauthenticator"
	"k8s.io/kubernetes/openshift-kube-apiserver/admission/authorization/restrictusers/usercache"
	oauthvalidation "k8s.io/kubernetes/openshift-kube-apiserver/admission/customresourcevalidation/oauth"
	"k8s.io/kubernetes/openshift-kube-apiserver/authentication/oauth"
	"k8s.io/kubernetes/openshift-kube-apiserver/openshiftkubeapiserver/paramtoken"
)

const authenticatedOAuthGroup = "system:authenticated:oauth"

// TODO we can re-trim these args to the the kubeapiserver config again if we feel like it, but for now we need it to be
// TODO obviously safe for 3.11
func NewAuthenticator(
	servingInfo configv1.ServingInfo,
	serviceAccountPublicKeyFiles []string, oauthConfig *osinv1.OAuthConfig, authConfig kubecontrolplanev1.MasterAuthConfig,
	privilegedLoopbackConfig *rest.Config,
	podLister corev1listers.PodLister,
	secretLister corev1listers.SecretLister,
	saLister corev1listers.ServiceAccountLister,
	oauthClientLister oauthclientlister.OAuthClientLister,
	groupInformer userinformer.GroupInformer,
) (authenticator.Request, map[string]genericapiserver.PostStartHookFunc, error) {
	kubeExternalClient, err := kclientsetexternal.NewForConfig(privilegedLoopbackConfig)
	if err != nil {
		return nil, nil, err
	}
	oauthClient, err := oauthclient.NewForConfig(privilegedLoopbackConfig)
	if err != nil {
		return nil, nil, err
	}
	userClient, err := userclient.NewForConfig(privilegedLoopbackConfig)
	if err != nil {
		return nil, nil, err
	}

	// this is safe because the server does a quorum read and we're hitting a "magic" authorizer to get permissions based on system:masters
	// once the cache is added, we won't be paying a double hop cost to etcd on each request, so the simplification will help.
	serviceAccountTokenGetter := sacontroller.NewGetterFromClient(kubeExternalClient, secretLister, saLister, podLister)

	return newAuthenticator(
		serviceAccountPublicKeyFiles,
		oauthConfig,
		authConfig,
		oauthClient.OAuthAccessTokens(),
		oauthClientLister,
		serviceAccountTokenGetter,
		userClient.UserV1().Users(),
		servingInfo.ClientCA,
		usercache.NewGroupCache(groupInformer),
		bootstrap.NewBootstrapUserDataGetter(kubeExternalClient.CoreV1(), kubeExternalClient.CoreV1()),
	)
}

func newAuthenticator(
	serviceAccountPublicKeyFiles []string,
	oauthConfig *osinv1.OAuthConfig,
	authConfig kubecontrolplanev1.MasterAuthConfig,
	accessTokenGetter oauthclient.OAuthAccessTokenInterface,
	oauthClientLister oauthclientlister.OAuthClientLister,
	tokenGetter serviceaccount.ServiceAccountTokenGetter,
	userGetter usertypedclient.UserInterface,
	apiClientCABundle string,
	groupMapper oauth.UserToGroupMapper,
	bootstrapUserDataGetter bootstrap.BootstrapUserDataGetter,
) (authenticator.Request, map[string]genericapiserver.PostStartHookFunc, error) {
	postStartHooks := map[string]genericapiserver.PostStartHookFunc{}
	authenticators := []authenticator.Request{}
	tokenAuthenticators := []authenticator.Token{}

	// ServiceAccount token
	if len(serviceAccountPublicKeyFiles) > 0 {
		publicKeys := []interface{}{}
		for _, keyFile := range serviceAccountPublicKeyFiles {
			readPublicKeys, err := keyutil.PublicKeysFromFile(keyFile)
			if err != nil {
				return nil, nil, fmt.Errorf("Error reading service account key file %s: %v", keyFile, err)
			}
			publicKeys = append(publicKeys, readPublicKeys...)
		}

		serviceAccountTokenAuthenticator := serviceaccount.JWTTokenAuthenticator(
			serviceaccount.LegacyIssuer,
			publicKeys,
			nil, // TODO audiences
			serviceaccount.NewLegacyValidator(true, tokenGetter),
		)
		tokenAuthenticators = append(tokenAuthenticators, serviceAccountTokenAuthenticator)
	}

	// OAuth token
	// this looks weird because it no longer belongs here (needs to be a remote token auth backed by osin)
	validators := []oauth.OAuthTokenValidator{oauth.NewExpirationValidator(), oauth.NewUIDValidator()}
	// if we have an oauth config, try to configure the inactivity timeout
	// TODO we need to fix config observation / move this to remote token authn so this can be honored in 4.x
	if oauthConfig != nil {
		if inactivityTimeout := oauthConfig.TokenConfig.AccessTokenInactivityTimeoutSeconds; inactivityTimeout != nil {
			timeoutValidator := oauth.NewTimeoutValidator(accessTokenGetter, oauthClientLister, *inactivityTimeout, oauthvalidation.MinimumInactivityTimeoutSeconds)
			validators = append(validators, timeoutValidator)
			postStartHooks["openshift.io-TokenTimeoutUpdater"] = func(context genericapiserver.PostStartHookContext) error {
				go timeoutValidator.Run(context.StopCh)
				return nil
			}
		}
	}
	oauthTokenAuthenticator := oauth.NewTokenAuthenticator(accessTokenGetter, userGetter, groupMapper, validators...)
	tokenAuthenticators = append(tokenAuthenticators,
		// if you have an OAuth bearer token, you're a human (usually)
		group.NewTokenGroupAdder(oauthTokenAuthenticator, []string{authenticatedOAuthGroup}))

	tokenAuthenticators = append(tokenAuthenticators,
		// bootstrap oauth user that can do anything, backed by a secret
		oauth.NewBootstrapAuthenticator(accessTokenGetter, bootstrapUserDataGetter, validators...))

	for _, wta := range authConfig.WebhookTokenAuthenticators {
		ttl, err := time.ParseDuration(wta.CacheTTL)
		if err != nil {
			return nil, nil, fmt.Errorf("Error parsing CacheTTL=%q: %v", wta.CacheTTL, err)
		}
		// TODO audiences
		webhookTokenAuthenticator, err := webhooktoken.New(wta.ConfigFile, nil)
		if err != nil {
			return nil, nil, fmt.Errorf("Failed to create webhook token authenticator for ConfigFile=%q: %v", wta.ConfigFile, err)
		}
		cachingTokenAuth := cache.New(webhookTokenAuthenticator, false, ttl, ttl)
		tokenAuthenticators = append(tokenAuthenticators, cachingTokenAuth)
	}

	if len(tokenAuthenticators) > 0 {
		// Combine all token authenticators
		tokenAuth := tokenunion.New(tokenAuthenticators...)

		// wrap with short cache on success.
		// this means a revoked service account token or access token will be valid for up to 10 seconds.
		// it also means group membership changes on users may take up to 10 seconds to become effective.
		tokenAuth = tokencache.New(tokenAuth, true, 10*time.Second, 0)

		authenticators = append(authenticators,
			bearertoken.New(tokenAuth),
			websocket.NewProtocolAuthenticator(tokenAuth),
			paramtoken.New("access_token", tokenAuth, true),
		)
	}

	// build cert authenticator
	// TODO: add "system:" prefix in authenticator, limit cert to username
	// TODO: add "system:" prefix to groups in authenticator, limit cert to group name
	dynamicCA := certs.NewDynamicCA(apiClientCABundle)
	if err := dynamicCA.CheckCerts(); err != nil {
		return nil, nil, err
	}
	certauth := x509request.NewDynamic(dynamicCA.GetVerifier, x509request.CommonNameUserConversion)
	postStartHooks["openshift.io-clientCA-reload"] = func(context genericapiserver.PostStartHookContext) error {
		go dynamicCA.Run(context.StopCh)
		return nil
	}
	authenticators = append(authenticators, certauth)

	resultingAuthenticator := union.NewFailOnError(authenticators...)

	topLevelAuthenticators := []authenticator.Request{}
	// if we have a front proxy providing authentication configuration, wire it up and it should come first
	if authConfig.RequestHeader != nil {
		requestHeaderAuthenticator, dynamicReloadFn, err := headerrequest.NewSecure(
			authConfig.RequestHeader.ClientCA,
			authConfig.RequestHeader.ClientCommonNames,
			authConfig.RequestHeader.UsernameHeaders,
			authConfig.RequestHeader.GroupHeaders,
			authConfig.RequestHeader.ExtraHeaderPrefixes,
		)
		if err != nil {
			return nil, nil, fmt.Errorf("Error building front proxy auth config: %v", err)
		}
		postStartHooks["openshift.io-requestheader-reload"] = func(context genericapiserver.PostStartHookContext) error {
			go dynamicReloadFn(context.StopCh)
			return nil
		}
		topLevelAuthenticators = append(topLevelAuthenticators, union.New(requestHeaderAuthenticator, resultingAuthenticator))

	} else {
		topLevelAuthenticators = append(topLevelAuthenticators, resultingAuthenticator)

	}
	topLevelAuthenticators = append(topLevelAuthenticators, anonymous.NewAuthenticator())

	return group.NewAuthenticatedGroupAdder(union.NewFailOnError(topLevelAuthenticators...)), postStartHooks, nil
}
