package oauth

import (
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	osinv1 "github.com/openshift/api/osin/v1"
	exutil "github.com/openshift/origin/test/extended/util"
	utiloauth "github.com/openshift/origin/test/extended/util/oauthserver"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	scheme  = runtime.NewScheme()
	codecs  = serializer.NewCodecFactory(scheme)
	encoder = codecs.LegacyCodec(osinv1.GroupVersion) // TODO I think there is a better way to do this
)

func init() {
	utilruntime.Must(osinv1.Install(scheme))
}

var _ = g.Describe("[Suite:openshift/oauth/htpasswd] HTPasswd IDP", func() {
	defer g.GinkgoRecover()
	var (
		oc = exutil.NewCLI("htpasswd-idp", exutil.KubeConfigPath())
	)

	g.It("should successfully configure htpasswd and be responsive", func() {
		secrets := []corev1.Secret{{
			ObjectMeta: metav1.ObjectMeta{
				Name: "htpasswd-secret",
			},
			Data: map[string][]byte{
				"htpasswd": []byte("testuser:$2y$05$pOYBCbJ1RXr.vDzPXdyTxuE96Nojc9dNI9R3QjkWUj2t/Ae/jmFy."), // userinfo testuser:password
			},
		}}

		htpasswdProvider, err := utiloauth.GetRawExtensionForOsinProvider(
			&osinv1.HTPasswdPasswordIdentityProvider{File: utiloauth.GetPathFromConfigMapSecretName("htpasswd-secret", "htpasswd")},
		)
		o.Expect(err).ToNot(o.HaveOccurred())
		htpasswdConfig := osinv1.IdentityProvider{
			Name:            "htpasswd",
			UseAsChallenger: true,
			UseAsLogin:      true,
			MappingMethod:   "claim",
			Provider:        *htpasswdProvider,
		}
		tokenReqOpts, cleanup, err := utiloauth.DeployOAuthServer(oc, []osinv1.IdentityProvider{htpasswdConfig}, nil, secrets)
		defer cleanup()
		o.Expect(err).ToNot(o.HaveOccurred())
		e2e.Logf("got the OAuth server address: %s", tokenReqOpts.Issuer)

		token, err := utiloauth.RequestTokenForUser(tokenReqOpts, "testuser", "password")
		defer func() {
			oc.AdminUserClient().UserV1().Users().Delete("testuser", &metav1.DeleteOptions{})
			oc.AdminUserClient().UserV1().Identities().Delete("htpasswd:testuser", &metav1.DeleteOptions{})
		}()
		o.Expect(err).ToNot(o.HaveOccurred())

		tokenUser, err := utiloauth.GetUserForToken(oc.AdminConfig(), token, "testuser")
		o.Expect(err).ToNot(o.HaveOccurred())
		o.Expect(tokenUser.Name).To(o.Equal("testuser"))
	})
})

func encodeOrDie(obj runtime.Object) []byte {
	bytes, err := runtime.Encode(encoder, obj)
	if err != nil {
		panic(err) // indicates static generated code is broken, unrecoverable
	}
	return bytes
}
