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

	g.It("should successfully configure htpasswd and be responsive [apigroup:user.openshift.io][apigroup:route.openshift.io]", g.Label("Size:L"), func() {
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
})
