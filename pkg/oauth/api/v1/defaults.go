package v1

import "k8s.io/apimachinery/pkg/runtime"

func SetDefaults_OAuthAuthorizeToken(obj *OAuthAuthorizeToken) {
	if len(obj.CodeChallenge) > 0 && len(obj.CodeChallengeMethod) == 0 {
		obj.CodeChallengeMethod = "plain"
	}
}

func addDefaultingFuncs(scheme *runtime.Scheme) error {
	RegisterDefaults(scheme)
	return scheme.AddDefaultingFuncs(
		SetDefaults_OAuthAuthorizeToken,
	)
}
