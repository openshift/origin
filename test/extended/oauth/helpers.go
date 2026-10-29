package oauth

import (
	"encoding/json"

	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	osinv1 "github.com/openshift/api/osin/v1"
	"github.com/openshift/library-go/pkg/oauth/oauthdiscovery"

	"github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/oauthserver"
)

// htpasswdBcryptHash is the bcrypt hash for the password "password",
const htpasswdBcryptHash = "$2y$05$0Fk2s.0FbLy0FZ82JAqajOV/kbT/wqKX5/QFKgps6J69J2jY6r5ZG"

func deployOAuthServer(oc *util.CLI) (oauthserver.NewRequestTokenOptionsFunc, func(), error) {
	return deployOAuthServerWithHTPasswd(oc, "testuser", "htpasswd")
}

// deployOAuthServerWithHTPasswd deploys an OAuth server with one htpasswd provider per name.
func deployOAuthServerWithHTPasswd(oc *util.CLI, username string, idpNames ...string) (oauthserver.NewRequestTokenOptionsFunc, func(), error) {
	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "htpasswd"},
		Data: map[string][]byte{
			"htpasswd": []byte(username + ":" + htpasswdBcryptHash),
		},
	}
	// provider config
	providerConfig, err := oauthserver.GetRawExtensionForOsinProvider(&osinv1.HTPasswdPasswordIdentityProvider{
		File: oauthserver.GetPathFromConfigMapSecretName(secret.Name, "htpasswd"),
	})
	o.Expect(err).ToNot(o.HaveOccurred())

	identityProviders := make([]osinv1.IdentityProvider, 0, len(idpNames))
	for i, name := range idpNames {
		identityProviders = append(identityProviders, osinv1.IdentityProvider{
			Name:          name,
			MappingMethod: "claim",
			Provider:      *providerConfig,
			// only the first provider logs users in, so we know where
			// the identity comes from (see the comment above)
			UseAsChallenger: i == 0,
			UseAsLogin:      i == 0,
		})
	}

	// deploy oauth server
	return oauthserver.DeployOAuthServer(oc, identityProviders, nil, []corev1.Secret{secret})
}

func getOAuthWellKnownData(oc *util.CLI) *oauthdiscovery.OauthAuthorizationServerMetadata {
	metadataJSON, err := oc.Run("get").Args("--raw", "/.well-known/oauth-authorization-server").Output()
	o.Expect(err).NotTo(o.HaveOccurred())

	metadata := &oauthdiscovery.OauthAuthorizationServerMetadata{}
	err = json.Unmarshal([]byte(metadataJSON), metadata)
	o.Expect(err).NotTo(o.HaveOccurred())

	return metadata
}
