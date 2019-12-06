package authenticator

import (
	"fmt"
	"time"

	oauthclient "github.com/openshift/client-go/oauth/clientset/versioned"
	oauthinformer "github.com/openshift/client-go/oauth/informers/externalversions"
	userclient "github.com/openshift/client-go/user/clientset/versioned"
	userinformer "github.com/openshift/client-go/user/informers/externalversions"
	bootstrap "github.com/openshift/library-go/pkg/authentication/bootstrapauthenticator"
	"k8s.io/apiserver/pkg/authentication/authenticator"
	"k8s.io/apiserver/pkg/authentication/group"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/keyutil"
	"k8s.io/kubernetes/openshift-kube-apiserver/admission/authorization/restrictusers/usercache"
	oauthvalidation "k8s.io/kubernetes/openshift-kube-apiserver/admission/customresourcevalidation/oauth"
	"k8s.io/kubernetes/openshift-kube-apiserver/authentication/oauth"
	"k8s.io/kubernetes/openshift-kube-apiserver/enablement"
	"k8s.io/kubernetes/pkg/serviceaccount"
)

const authenticatedOAuthGroup = "system:authenticated:oauth"

// TODO find a single place to create and start informers.  During the 1.7 rebase this will come more naturally in a config object,
// before then we should try to eliminate our direct to storage access.  It's making us do weird things.
const defaultInformerResyncPeriod = 10 * time.Minute

func AddOAuthServerAuthenticatorIfNeeded(tokenAuthenticators []authenticator.Token, serviceAccountTokenGetter serviceaccount.ServiceAccountTokenGetter) []authenticator.Token {
	if !enablement.IsOpenShift() {
		return tokenAuthenticators
	}

	kubeClient, err := kubernetes.NewForConfig(enablement.LoopbackClientConfig())
	if err != nil {
		panic(err)
	}
	bootstrapUserDataGetter := bootstrap.NewBootstrapUserDataGetter(kubeClient.CoreV1(), kubeClient.CoreV1())

	oauthClient, err := oauthclient.NewForConfig(enablement.LoopbackClientConfig())
	if err != nil {
		panic(err)
	}
	userClient, err := userclient.NewForConfig(enablement.LoopbackClientConfig())
	if err != nil {
		panic(err)
	}

	oauthInformer := oauthinformer.NewSharedInformerFactory(oauthClient, defaultInformerResyncPeriod)
	userInformer := userinformer.NewSharedInformerFactory(userClient, defaultInformerResyncPeriod)
	if err := userInformer.User().V1().Groups().Informer().AddIndexers(cache.Indexers{
		usercache.ByUserIndexName: usercache.ByUserIndexKeys,
	}); err != nil {
		panic(err)
	}

	// add service account token authentication.
	// TODO remove this and rely on the kube wiring once we fix our input
	publicKeys := []interface{}{}
	for _, keyFile := range enablement.OpenshiftConfig().ServiceAccountPublicKeyFiles {
		readPublicKeys, err := keyutil.PublicKeysFromFile(keyFile)
		if err != nil {
			panic(fmt.Errorf("Error reading service account key file %s: %v", keyFile, err))
		}
		publicKeys = append(publicKeys, readPublicKeys...)
	}
	serviceAccountTokenAuthenticator := serviceaccount.JWTTokenAuthenticator(
		serviceaccount.LegacyIssuer,
		publicKeys,
		nil, // TODO audiences
		serviceaccount.NewLegacyValidator(true, serviceAccountTokenGetter),
	)
	tokenAuthenticators = append(tokenAuthenticators, serviceAccountTokenAuthenticator)

	// add our oauth token validator
	validators := []oauth.OAuthTokenValidator{oauth.NewExpirationValidator(), oauth.NewUIDValidator()}
	if enablement.OpenshiftConfig().OAuthConfig != nil {
		if inactivityTimeout := enablement.OpenshiftConfig().OAuthConfig.TokenConfig.AccessTokenInactivityTimeoutSeconds; inactivityTimeout != nil {
			timeoutValidator := oauth.NewTimeoutValidator(oauthClient.OauthV1().OAuthAccessTokens(), oauthInformer.Oauth().V1().OAuthClients().Lister(), *inactivityTimeout, oauthvalidation.MinimumInactivityTimeoutSeconds)
			validators = append(validators, timeoutValidator)
			enablement.AddPostStartHookOrDie("openshift.io-TokenTimeoutUpdater", func(context genericapiserver.PostStartHookContext) error {
				go timeoutValidator.Run(context.StopCh)
				return nil
			})
		}
	}
	enablement.AddPostStartHookOrDie("openshift.io-StartOAuthInformers", func(context genericapiserver.PostStartHookContext) error {
		go oauthInformer.Start(context.StopCh)
		go userInformer.Start(context.StopCh)
		return nil
	})
	groupMapper := usercache.NewGroupCache(userInformer.User().V1().Groups())
	oauthTokenAuthenticator := oauth.NewTokenAuthenticator(oauthClient.OauthV1().OAuthAccessTokens(), userClient.UserV1().Users(), groupMapper, validators...)
	tokenAuthenticators = append(tokenAuthenticators,
		// if you have an OAuth bearer token, you're a human (usually)
		group.NewTokenGroupAdder(oauthTokenAuthenticator, []string{authenticatedOAuthGroup}))

	// add the bootstrap user token authenticator
	tokenAuthenticators = append(tokenAuthenticators,
		// bootstrap oauth user that can do anything, backed by a secret
		oauth.NewBootstrapAuthenticator(oauthClient.OauthV1().OAuthAccessTokens(), bootstrapUserDataGetter, validators...))

	return tokenAuthenticators
}
