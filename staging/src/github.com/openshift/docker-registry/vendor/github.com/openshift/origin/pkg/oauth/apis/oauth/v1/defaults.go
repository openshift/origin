package v1

func SetDefaults_OAuthAuthorizeToken(obj *OAuthAuthorizeToken) {
	if len(obj.CodeChallenge) > 0 && len(obj.CodeChallengeMethod) == 0 {
		obj.CodeChallengeMethod = "plain"
	}
}
