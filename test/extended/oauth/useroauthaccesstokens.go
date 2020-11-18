package oauth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/rest"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/origin/test/extended/util"

	oauthv1 "github.com/openshift/api/oauth/v1"
	userv1 "github.com/openshift/api/user/v1"
	oauthv1client "github.com/openshift/client-go/oauth/clientset/versioned/typed/oauth/v1"
)

var _ = g.Describe("[sig-auth][Feature:OAuthAPIServer]", func() {
	defer g.GinkgoRecover()
	var oc = exutil.NewCLI("useroauthaccesstokens")

	g.It("test UserOAuthAccessTokens", func() {
		testCtx := context.Background()
		defer oc.TeardownProject()

		frantaToken := []byte(oc.Namespace() + "ifonlythiswasrandom") // for franta the user
		mirkaToken := []byte(oc.Namespace() + "nothingness")          // for mirka the user
		frantaTokenString := "sha256~" + string(frantaToken)
		mirkaTokenString := "sha256~" + string(mirkaToken)

		frantaSha := sha256.Sum256(frantaToken)
		mirkaSha := sha256.Sum256(mirkaToken)

		frantaObj, err := oc.AdminUserClient().UserV1().Users().Create(testCtx, &userv1.User{
			ObjectMeta: metav1.ObjectMeta{
				Name: "franta",
			},
		}, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		oc.AddResourceToDelete(userv1.GroupVersion.WithResource("users"), frantaObj.GetObjectMeta())

		mirkaObj, err := oc.AdminUserClient().UserV1().Users().Create(testCtx, &userv1.User{
			ObjectMeta: metav1.ObjectMeta{
				Name: "mirka",
			},
		}, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		oc.AddResourceToDelete(userv1.GroupVersion.WithResource("users"), mirkaObj.GetObjectMeta())

		tokens := []*oauthv1.OAuthAccessToken{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "sha256~" + base64.RawURLEncoding.EncodeToString(frantaSha[:]),
				},
				ClientName:  "openshift-challenging-client",
				ExpiresIn:   10000,
				RedirectURI: "https://test.testingstuff.test.test",
				UserName:    "franta",
				UserUID:     string(frantaObj.UID),
				Scopes:      []string{"user:full"},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "sha256~" + base64.RawURLEncoding.EncodeToString(mirkaSha[:]),
				},
				ClientName:  "openshift-challenging-client",
				ExpiresIn:   10000,
				RedirectURI: "https://test.testingstuff.test.test",
				UserName:    "mirka",
				UserUID:     string(mirkaObj.UID),
				Scopes:      []string{"user:full"},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "sha256~" + oc.Namespace() + "mirka0",
				},
				ClientName:  "openshift-browser-client",
				ExpiresIn:   10000,
				RedirectURI: "https://test.testingstuff.test.test",
				UserName:    "mirka",
				UserUID:     string(mirkaObj.UID),
				Scopes:      []string{"user:full"},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "sha256~" + oc.Namespace() + "franta0",
				},
				ClientName:  "openshift-browser-client",
				ExpiresIn:   10000,
				RedirectURI: "https://test.testingstuff.test.test",
				UserName:    "franta",
				UserUID:     string(frantaObj.UID),
				Scopes:      []string{"user:full"},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "sha256~" + oc.Namespace() + "franta1",
				},
				ClientName:  "openshift-browser-client",
				ExpiresIn:   10000,
				RedirectURI: "https://test.testingstuff.test.test",
				UserName:    "franta",
				UserUID:     string(frantaObj.UID),
				Scopes:      []string{"user:full"},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "sha256~" + oc.Namespace() + "pepa0",
				},
				ClientName:  "openshift-challenging-client",
				ExpiresIn:   10000,
				RedirectURI: "https://test.testingstuff.test.test",
				UserName:    "pepa",
				UserUID:     "pepauid",
				Scopes:      []string{"user:full"},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "sha256~" + oc.Namespace() + "tonda0",
				},
				ClientName:  "openshift-challenging-client",
				ExpiresIn:   10000,
				RedirectURI: "https://test.testingstuff.test.test",
				UserName:    "tonda",
				UserUID:     "tondauid",
				Scopes:      []string{"user:full"},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "sha256~" + oc.Namespace() + "franta2",
				},
				ClientName:  "openshift-challenging-client",
				ExpiresIn:   10000,
				RedirectURI: "https://test.testingstuff.test.test",
				UserName:    "franta",
				UserUID:     string(frantaObj.UID),
				Scopes:      []string{"user:full"},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: oc.Namespace() + "nonshatoken",
				},
				ClientName:  "openshift-challenging-client",
				ExpiresIn:   10000,
				RedirectURI: "https://test.testingstuff.test.test",
				UserName:    "franta",
				UserUID:     string(frantaObj.UID),
				Scopes:      []string{"user:full"},
			},
		}

		for _, t := range tokens {
			_, err := oc.AdminOAuthClient().OauthV1().OAuthAccessTokens().Create(testCtx, t, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			oc.AddResourceToDelete(oauthv1.GroupVersion.WithResource("oauthaccesstokens"), t.GetObjectMeta())
		}

		g.By("==== list tokens ====", func() { listTokens(oc, frantaTokenString, mirkaTokenString) })
		g.By("==== get a token ====", func() { getTokens(oc, frantaTokenString, mirkaTokenString) })
		g.By("==== delete a token ====", func() { deleteTokens(oc, frantaTokenString, mirkaTokenString) })
	})
})

func listTokens(oc *exutil.CLI, frantaTokenString, mirkaTokenString string) {
	e2e.Logf("franta: %s, mirka: %s", frantaTokenString, mirkaTokenString)

	testCtx := context.Background()

	tests := []struct {
		name            string
		userToken       string
		userName        string
		fieldSelector   fields.Selector
		labelSelector   labels.Selector
		expectedResults int
		expectedError   string
	}{
		{
			name:            "happy path",
			userToken:       mirkaTokenString,
			userName:        "mirka",
			expectedResults: 2,
		},
		{
			name:          "invalid field selector taken from oauthaccesstokens to match another username",
			userToken:     frantaTokenString,
			fieldSelector: fields.OneTermEqualSelector("userName", "pepa"),
			expectedError: "is not a known field selector",
		},
		{
			name:            "single-equal field selector to get own tokens by client",
			userToken:       frantaTokenString,
			userName:        "franta",
			fieldSelector:   fields.OneTermEqualSelector("clientName", "openshift-browser-client"),
			expectedResults: 2,
		},
		{
			name:            "set label selector to get own tokens and of others",
			userToken:       mirkaTokenString,
			userName:        "mirka",
			labelSelector:   parseLabelSelectorOrDie("randomLabel notin (mirka,franta)"),
			expectedResults: 2,
		},
	}

	for _, tc := range tests {
		g.By(tc.name, func() {
			userConfig := rest.AnonymousClientConfig(oc.AdminConfig())
			userConfig.BearerToken = tc.userToken

			tokenClient, err := oauthv1client.NewForConfig(userConfig)
			o.Expect(err).NotTo(o.HaveOccurred())

			lopts := metav1.ListOptions{}
			if tc.fieldSelector != nil {
				lopts.FieldSelector = tc.fieldSelector.String()
			}
			if tc.labelSelector != nil {
				lopts.LabelSelector = tc.labelSelector.String()
			}
			tokenList, err := tokenClient.UserOAuthAccessTokens().List(testCtx, lopts)
			if len(tc.expectedError) != 0 {
				o.Expect(err).NotTo(o.BeNil())
				o.Expect(err.Error()).To(o.ContainSubstring(tc.expectedError))
			} else {
				o.Expect(err).NotTo(o.HaveOccurred())
			}

			o.Expect(len(tokenList.Items)).To(o.Equal(tc.expectedResults), fmt.Sprintf("unexpected number of results, expected %d, got %d: %v", len(tokenList.Items), tc.expectedResults, tokenList.Items))
			for _, t := range tokenList.Items {
				o.Expect(t.UserName).To(o.Equal(tc.userName))
			}

		})
	}
}

func getTokens(oc *exutil.CLI, frantaTokenString, mirkaTokenString string) {
	testCtx := context.Background()

	tests := []struct {
		name          string
		userToken     string
		userName      string
		getTokenName  string
		expectedError bool
	}{
		{
			name:         "get own token",
			userToken:    mirkaTokenString,
			userName:     "mirka",
			getTokenName: "sha256~" + oc.Namespace() + "mirka0",
		},
		{
			name:          "get someone else's token",
			userToken:     mirkaTokenString,
			getTokenName:  "sha256~" + oc.Namespace() + "franta0",
			expectedError: true,
		},
	}

	for _, tc := range tests {
		g.By(tc.name, func() {
			userConfig := rest.AnonymousClientConfig(oc.AdminConfig())
			userConfig.BearerToken = tc.userToken

			tokenClient, err := oauthv1client.NewForConfig(userConfig)
			o.Expect(err).NotTo(o.HaveOccurred())

			token, err := tokenClient.UserOAuthAccessTokens().Get(testCtx, tc.getTokenName, metav1.GetOptions{})
			if tc.expectedError {
				if !errors.IsNotFound(err) {
					g.Fail(fmt.Sprintf("expected to not find any tokens, instead got err: %v and result: %v", err, token))
				}
			} else {
				var tokenList []oauthv1.OAuthAccessToken
				tokens, lerr := oc.AdminOauthClient().OauthV1().OAuthAccessTokens().List(testCtx, metav1.ListOptions{})
				if lerr == nil {
					tokenList = tokens.Items
				}
				o.Expect(err).NotTo(o.HaveOccurred(), "tokens dump: %v", tokenList)
			}

			o.Expect(token.UserName).To(o.Equal(tc.userName))

		})
	}
}

func deleteTokens(oc *exutil.CLI, frantaTokenString, mirkaTokenString string) {
	testCtx := context.Background()

	tests := []struct {
		name          string
		userToken     string
		userName      string
		getTokenName  string
		expectedError bool
	}{
		{
			name:          "delete someone else's token",
			userToken:     mirkaTokenString,
			getTokenName:  "sha256~" + oc.Namespace() + "franta0",
			expectedError: true,
		},
		{
			name:         "delete an own token",
			userToken:    frantaTokenString,
			userName:     "mirka",
			getTokenName: "sha256~" + oc.Namespace() + "franta0",
		},
	}

	for _, tc := range tests {
		g.By(tc.name, func() {
			userConfig := rest.AnonymousClientConfig(oc.AdminConfig())
			userConfig.BearerToken = tc.userToken

			tokenClient, err := oauthv1client.NewForConfig(userConfig)
			o.Expect(err).NotTo(o.HaveOccurred())

			err = tokenClient.UserOAuthAccessTokens().Delete(testCtx, tc.getTokenName, metav1.DeleteOptions{})
			if tc.expectedError {
				if !errors.IsNotFound(err) {
					g.Fail(fmt.Sprintf("expected to not delete any tokens, instead got err: %v", err))
				}
			} else {
				var tokenList []oauthv1.OAuthAccessToken
				tokens, lerr := oc.AdminOauthClient().OauthV1().OAuthAccessTokens().List(testCtx, metav1.ListOptions{})
				if lerr == nil {
					tokenList = tokens.Items
				}
				o.Expect(err).NotTo(o.HaveOccurred(), "tokens dump: %v", tokenList)
			}

		})
	}
}

func parseLabelSelectorOrDie(s string) labels.Selector {
	selector, err := labels.Parse(s)
	if err != nil {
		panic(err)
	}
	return selector
}
