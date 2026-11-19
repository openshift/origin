package oauth

import (
	"context"
	"fmt"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
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

	// Regression test: a second password provider plus whitespace in a provider name used to break the oauth server.
	g.It("should not panic when multiple htpasswd identity providers have whitespace in their names [apigroup:user.openshift.io][apigroup:route.openshift.io]", func() {
		const (
			username         = "testuser43587"
			idpWithSpaces    = "idp with spaces"
			idpWithoutSpaces = "idp-without-spaces"
		)
		newTokenReqOpts, cleanup, err := deployOAuthServerWithHTPasswd(oc, username, idpWithSpaces, idpWithoutSpaces)
		defer cleanup()
		o.Expect(err).ToNot(o.HaveOccurred(), "oauth-server did not come up with a htpasswd identity provider whose name contains whitespace")

		// leftovers from earlier runs would claim-map to the stale user and
		// mask which provider performed the login, so reset before and after
		resetHTPasswdUser := func() error {
			userClient := oc.AdminUserClient().UserV1()
			var errs []error
			for _, identityName := range []string{idpWithSpaces + ":" + username, idpWithoutSpaces + ":" + username} {
				if err := userClient.Identities().Delete(context.Background(), identityName, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
					errs = append(errs, fmt.Errorf("deleting identity %q: %w", identityName, err))
				}
			}
			if err := userClient.Users().Delete(context.Background(), username, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
				errs = append(errs, fmt.Errorf("deleting user %q: %w", username, err))
			}
			return utilerrors.NewAggregate(errs)
		}
		o.Expect(resetHTPasswdUser()).ToNot(o.HaveOccurred(), "failed to reset htpasswd user %q and its identities before the test", username)
		defer func() {
			o.Expect(resetHTPasswdUser()).ToNot(o.HaveOccurred(), "failed to clean up htpasswd user %q and its identities after the test", username)
		}()

		token, err := newTokenReqOpts(username, "password").RequestToken()
		o.Expect(err).ToNot(o.HaveOccurred(), "failed to request token for htpasswd user %s", username)
		tokenUser, err := utiloauth.GetUserForToken(oc.AdminConfig(), token, username)
		o.Expect(err).ToNot(o.HaveOccurred(), "failed to get user for token of htpasswd user %s", username)
		o.Expect(tokenUser.Name).To(o.Equal(username), "expected token to authenticate as htpasswd user %s", username)

		// proves the spacey provider did the login, not the control provider
		identity, err := oc.AdminUserClient().UserV1().Identities().Get(context.Background(), idpWithSpaces+":"+username, metav1.GetOptions{})
		o.Expect(err).ToNot(o.HaveOccurred(), "failed to get identity %s:%s", idpWithSpaces, username)
		o.Expect(identity.ProviderName).To(o.Equal(idpWithSpaces), "expected the login to go through provider %q", idpWithSpaces)
		o.Expect(identity.ProviderUserName).To(o.Equal(username), "expected identity provider user name to be %s", username)
		o.Expect(identity.User.Name).To(o.Equal(username), "expected identity to be mapped to user %s", username)
	})
})
