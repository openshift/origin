package oauth

import (
	"context"
	"io/ioutil"
	"os"
	"path"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/client-go/rest"
	restclient "k8s.io/client-go/rest"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"

	userv1client "github.com/openshift/client-go/user/clientset/versioned"
	"github.com/openshift/library-go/pkg/crypto"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/ibmcloud"
)

const (
	invalidToken      = "invalid"
	noToken           = ""
	certUser          = "system:admin"
	tokenUser         = "foouser"
	unauthorizedError = "Unauthorized"
	anonymousError    = `users.user.openshift.io "~" is forbidden: User "system:anonymous" cannot get resource "users" in API group "user.openshift.io" at the cluster scope`
)

var _ = g.Describe("[sig-auth][Feature:OAuthServer] OAuth server", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLI("oauth")

	g.It("has the correct token and certificate fallback semantics", func() {
		if e2e.TestContext.Provider == ibmcloud.ProviderName {
			e2eskipper.Skipf("IBM ROKS clusters do not contain a kube-control-plane-signer secret inside the cluster. The secret lives outside the cluster with the rest of the control plane.")
		}

		var (
			// We have to generate this dynamically in order to have an invalid cert signed by a signer with the same name as the valid CA
			invalidCert = restclient.TLSClientConfig{}
			noCert      = restclient.TLSClientConfig{}
		)

		// make a client cert signed by a fake CA with the same name as the real CA.
		// this is needed to get the go client to actually send the cert to the server,
		// since the server advertises the signer name it requires
		fakecadir, err := ioutil.TempDir("", "fakeca")
		o.Expect(err).NotTo(o.HaveOccurred())
		defer os.RemoveAll(fakecadir)

		// openssl s_client shows the kube-control-plane-signer CA name sent as one of the acceptable client CAs, so use that.
		realCASecret, err := oc.AsAdmin().KubeClient().CoreV1().Secrets("openshift-kube-apiserver-operator").Get(context.Background(), "kube-control-plane-signer", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		realCAPEM, ok := realCASecret.Data["tls.crt"]
		o.Expect(ok).NotTo(o.BeFalse())

		realCA, err := crypto.CertsFromPEM(realCAPEM)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(len(realCA)).To(o.Equal(1))

		fakeCA, err := crypto.MakeSelfSignedCA(
			path.Join(fakecadir, "fakeca.crt"),
			path.Join(fakecadir, "fakeca.key"),
			path.Join(fakecadir, "fakeca.serial"),
			realCA[0].Subject.String(),
			100,
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		clientCertConfig, err := fakeCA.MakeClientCertificate(
			path.Join(fakecadir, "fakeclient.crt"),
			path.Join(fakecadir, "fakeclient.key"),
			&user.DefaultInfo{Name: "fakeuser"},
			365*2, /* 2 years */
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		invalidCert.CertData, invalidCert.KeyData, err = clientCertConfig.GetPEMBytes()
		o.Expect(err).NotTo(o.HaveOccurred())

		fooUserConfig := oc.GetClientConfigForUser(tokenUser)
		o.Expect(err).NotTo(o.HaveOccurred())
		validToken := fooUserConfig.BearerToken
		o.Expect(validToken).ToNot(o.BeEmpty())

		adminConfig := oc.AdminConfig()
		validCert := adminConfig.TLSClientConfig

		for name, test := range map[string]struct {
			token         string
			cert          rest.TLSClientConfig
			expectedUser  string
			errorExpected bool
			errorString   string
		}{
			"valid token, valid cert": {
				token:        validToken,
				cert:         validCert,
				expectedUser: certUser,
			},
			"valid token, invalid cert": {
				token:        validToken,
				cert:         invalidCert,
				expectedUser: tokenUser,
			},
			"valid token, no cert": {
				token:        validToken,
				cert:         noCert,
				expectedUser: tokenUser,
			},
			"invalid token, valid cert": {
				token:        invalidToken,
				cert:         validCert,
				expectedUser: certUser,
			},
			"invalid token, invalid cert": {
				token:         invalidToken,
				cert:          invalidCert,
				errorExpected: true,
				errorString:   unauthorizedError,
			},
			"invalid token, no cert": {
				token:         invalidToken,
				cert:          noCert,
				errorExpected: true,
				errorString:   unauthorizedError,
			},
			"no token, valid cert": {
				token:        noToken,
				cert:         validCert,
				expectedUser: certUser,
			},
			"no token, invalid cert": {
				token:         noToken,
				cert:          invalidCert,
				errorExpected: true,
				errorString:   unauthorizedError,
			},
			"no token, no cert": {
				token:         noToken,
				cert:          noCert,
				errorExpected: true,
				errorString:   anonymousError,
			},
		} {
			g.By(name)
			adminConfig := restclient.AnonymousClientConfig(oc.AdminConfig())
			adminConfig.BearerToken = test.token
			adminConfig.TLSClientConfig = test.cert
			adminConfig.CAData = oc.AdminConfig().CAData

			userClient := userv1client.NewForConfigOrDie(adminConfig)
			user, err := userClient.UserV1().Users().Get(context.Background(), "~", metav1.GetOptions{})

			if test.errorExpected {
				o.Expect(err).ToNot(o.BeNil())
				o.Expect(err.Error()).To(o.Equal(test.errorString))
			} else {
				o.Expect(user).ToNot(o.BeNil())
				o.Expect(user.Name).To(o.Equal(test.expectedUser))
				o.Expect(err).To(o.BeNil())
			}
		}
	})
})
