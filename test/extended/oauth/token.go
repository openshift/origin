package oauth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"

	"github.com/pborman/uuid"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/openshift/client-go/user/clientset/versioned"
	"k8s.io/apimachinery/pkg/api/errors"

	"k8s.io/client-go/rest"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	oauthv1 "github.com/openshift/api/oauth/v1"
	userv1 "github.com/openshift/api/user/v1"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-auth][Feature:OAuthServer] OAuth Authenticator", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLI("oauth-access-token-e2e-test")
	ctx := context.Background()

	g.It(fmt.Sprintf("accepts sha256 access tokens [apigroup:user.openshift.io][apigroup:oauth.openshift.io]"), g.Label("Size:M"), func() {
		user, err := oc.AdminUserClient().UserV1().Users().Create(ctx, &userv1.User{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name: "oauth-access-token-e2e-test-user-sha256",
			},
			FullName: "test user",
		}, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		oc.AddResourceToDelete(userv1.GroupVersion.WithResource("users"), user)

		g.By("creating a classic oauth access token")
		token := base64.RawURLEncoding.EncodeToString([]byte(uuid.New()))
		bs := sha256.Sum256([]byte(token))
		hash := base64.RawURLEncoding.EncodeToString(bs[:])
		classicTokenObject, err := oc.AdminOAuthClient().OauthV1().OAuthAccessTokens().Create(ctx, &oauthv1.OAuthAccessToken{
			ObjectMeta: metav1.ObjectMeta{
				Name: "sha256~" + hash[0:],
			},
			UserName:    user.Name,
			UserUID:     string(user.UID),
			ClientName:  "openshift-challenging-client",
			Scopes:      []string{"user:info"},
			RedirectURI: "https://127.0.0.1:12000/oauth/token/implicit",
		}, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		oc.AddResourceToDelete(oauthv1.GroupVersion.WithResource("oauthaccesstokens"), classicTokenObject)

		g.By("authenticating using the sha256 prefixed access token as bearer token")
		gotUser, err := whoamiWithToken("sha256~"+token, oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(gotUser.Name).To(o.Equal(user.Name))

		g.By("not-authenticating using a sha256 prefixed hash as bearer token")
		_, err = whoamiWithToken("sha256~"+hash[0:], oc)
		o.Expect(errors.IsUnauthorized(err)).To(o.BeTrue())

		g.By("not-authenticating using a non-prefixed token as bearer token")
		_, err = whoamiWithToken(token, oc)
		o.Expect(errors.IsUnauthorized(err)).To(o.BeTrue())

		g.By("not-authenticating using a non-prefixed hash as bearer token")
		_, err = whoamiWithToken(hash[0:], oc)
		o.Expect(errors.IsUnauthorized(err)).To(o.BeTrue())
	})
})

func whoamiWithToken(token string, oc *exutil.CLI) (*userv1.User, error) {
	bearerTokenConfig := rest.AnonymousClientConfig(oc.AdminConfig())
	bearerTokenConfig.BearerToken = token
	userClient, err := versioned.NewForConfig(bearerTokenConfig)
	o.Expect(err).NotTo(o.HaveOccurred())
	return userClient.UserV1().Users().Get(context.TODO(), "~", metav1.GetOptions{})
}
