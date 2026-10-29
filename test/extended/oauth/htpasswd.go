package oauth

import (
	"context"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	admissionapi "k8s.io/pod-security-admission/api"

	osinv1 "github.com/openshift/api/osin/v1"

	exutil "github.com/openshift/origin/test/extended/util"
	utiloauth "github.com/openshift/origin/test/extended/util/oauthserver"
)

var (
	scheme  = runtime.NewScheme()
	codecs  = serializer.NewCodecFactory(scheme)
	encoder = codecs.LegacyCodec(osinv1.GroupVersion) // TODO I think there is a better way to do this
)

func init() {
	utilruntime.Must(osinv1.Install(scheme))
}

var _ = g.Describe("[sig-auth][Feature:HTPasswdAuth] HTPasswd IDP", func() {
	var oc = exutil.NewCLIWithPodSecurityLevel("htpasswd-idp", admissionapi.LevelBaseline)

	g.It("should successfully configure htpasswd and be responsive [apigroup:user.openshift.io][apigroup:route.openshift.io]", func() {
		newTokenReqOpts, cleanup, err := deployOAuthServer(oc)
		defer cleanup()
		o.Expect(err).ToNot(o.HaveOccurred())
		tokenReqOpts := newTokenReqOpts("testuser", "password")
		e2e.Logf("got the OAuth server address: %s", tokenReqOpts.Issuer)
		token, err := tokenReqOpts.RequestToken()
		o.Expect(err).ToNot(o.HaveOccurred())
		defer func() {
			oc.AdminUserClient().UserV1().Users().Delete(context.Background(), "testuser", metav1.DeleteOptions{})
			oc.AdminUserClient().UserV1().Identities().Delete(context.Background(), "htpasswd:testuser", metav1.DeleteOptions{})
		}()
		tokenUser, err := utiloauth.GetUserForToken(oc.AdminConfig(), token, "testuser")
		o.Expect(err).ToNot(o.HaveOccurred())
		o.Expect(tokenUser.Name).To(o.Equal("testuser"))
	})

	// Regression test: a second password provider plus whitespace in a provider name used
	// to break the oauth server.
	g.It("should not panic when multiple htpasswd identity providers have whitespace in their names [apigroup:user.openshift.io][apigroup:route.openshift.io]", func() {
		const (
			username         = "testuser43587"
			idpWithSpaces    = "idp with spaces"
			idpWithoutSpaces = "idp-without-spaces"
		)
		newTokenReqOpts, cleanup, err := deployOAuthServerWithHTPasswd(oc, username, idpWithSpaces, idpWithoutSpaces)
		defer cleanup()
		o.Expect(err).ToNot(o.HaveOccurred(), "oauth-server did not come up with a htpasswd identity provider whose name contains whitespace")

		resetHTPasswdUser := func() {
			oc.AdminUserClient().UserV1().Identities().Delete(context.Background(), idpWithSpaces+":"+username, metav1.DeleteOptions{})
			oc.AdminUserClient().UserV1().Identities().Delete(context.Background(), idpWithoutSpaces+":"+username, metav1.DeleteOptions{})
			oc.AdminUserClient().UserV1().Users().Delete(context.Background(), username, metav1.DeleteOptions{})
		}
		resetHTPasswdUser()
		defer resetHTPasswdUser()

		token, err := newTokenReqOpts(username, "password").RequestToken()
		o.Expect(err).ToNot(o.HaveOccurred())
		tokenUser, err := utiloauth.GetUserForToken(oc.AdminConfig(), token, username)
		o.Expect(err).ToNot(o.HaveOccurred())
		o.Expect(tokenUser.Name).To(o.Equal(username))

		// proves the spacey provider did the login, not the control provider
		identity, err := oc.AdminUserClient().UserV1().Identities().Get(context.Background(), idpWithSpaces+":"+username, metav1.GetOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())
		o.Expect(identity.ProviderName).To(o.Equal(idpWithSpaces))
		o.Expect(identity.ProviderUserName).To(o.Equal(username))
		o.Expect(identity.User.Name).To(o.Equal(username))
	})
})
