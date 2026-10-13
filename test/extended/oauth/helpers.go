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

func deployOAuthServer(oc *util.CLI) (oauthserver.NewRequestTokenOptionsFunc, func(), error) {
	// secret containing htpasswd "file": `htpasswd -cbB htpasswd.tmp testuser password`
	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "htpasswd"},
		Data: map[string][]byte{
			"htpasswd": []byte("testuser:$2y$05$0Fk2s.0FbLy0FZ82JAqajOV/kbT/wqKX5/QFKgps6J69J2jY6r5ZG"),
		},
	}
	// provider config
	providerConfig, err := oauthserver.GetRawExtensionForOsinProvider(&osinv1.HTPasswdPasswordIdentityProvider{
		File: oauthserver.GetPathFromConfigMapSecretName(secret.Name, "htpasswd"),
	})
	o.Expect(err).ToNot(o.HaveOccurred())
	// identity provider
	identityProvider := osinv1.IdentityProvider{
		Name:            "htpasswd",
		MappingMethod:   "claim",
		Provider:        *providerConfig,
		UseAsChallenger: true,
		UseAsLogin:      true,
	}

	// deploy oauth server
	return oauthserver.DeployOAuthServer(oc, []osinv1.IdentityProvider{identityProvider}, nil, []corev1.Secret{secret})
}

func getOAuthWellKnownData(oc *util.CLI) *oauthdiscovery.OauthAuthorizationServerMetadata {
	metadataJSON, err := oc.Run("get").Args("--raw", "/.well-known/oauth-authorization-server").Output()
	o.Expect(err).NotTo(o.HaveOccurred())

	metadata := &oauthdiscovery.OauthAuthorizationServerMetadata{}
	err = json.Unmarshal([]byte(metadataJSON), metadata)
	o.Expect(err).NotTo(o.HaveOccurred())

	return metadata
}
