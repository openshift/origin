package oauth

import (
	"context"
	"crypto/sha256"
	"io/ioutil"
	"os"
	"path"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"
	restclient "k8s.io/client-go/rest"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"

	configv1 "github.com/openshift/api/config/v1"
	userv1client "github.com/openshift/client-go/user/clientset/versioned"
	"github.com/openshift/library-go/pkg/crypto"
	exutil "github.com/openshift/origin/test/extended/util"
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

	g.It("has the correct token and certificate fallback semantics [apigroup:user.openshift.io]", func() {
		controlPlaneTopology, err := exutil.GetControlPlaneTopology(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		if *controlPlaneTopology == configv1.ExternalTopologyMode {
			e2eskipper.Skipf("External clusters do not contain a kube-control-plane-signer secret inside the cluster. The secret lives outside the cluster with the rest of the control plane.")
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
		o.Expect(ok).To(o.BeTrue())

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
		o.Expect(fooUserConfig).NotTo(o.BeNil())
		validToken := fooUserConfig.BearerToken
		o.Expect(validToken).ToNot(o.BeEmpty())

		adminConfig := oc.AdminConfig()
		validCert := adminConfig.TLSClientConfig

		// Cache certs in a map
		certs := map[string]restclient.TLSClientConfig{
			"validCert":   validCert,
			"invalidCert": invalidCert,
			"noCert":      noCert,
		}

		for name, test := range map[string]struct {
			token         string
			certKey       string // use "validCert", "invalidCert", or "noCert"
			expectedUser  string
			errorExpected bool
			errorString   string
		}{
			"valid token, valid cert": {
				token:        validToken,
				certKey:      "validCert",
				expectedUser: certUser, // If cert is valid, it takes precedence over token (because client certs are stronger auth).
			},

			"valid token, invalid cert": {
				token:        validToken,
				certKey:      "invalidCert",
				expectedUser: tokenUser,
			},

			"valid token, no cert": {
				token:        validToken,
				certKey:      "noCert",
				expectedUser: tokenUser,
			},

			"invalid token, valid cert": {
				token:        invalidToken,
				certKey:      "validCert",
				expectedUser: certUser,
			},

			"invalid token, invalid cert": {
				token:         invalidToken,
				certKey:       "invalidCert",
				errorExpected: true,
				errorString:   unauthorizedError,
			},

			"invalid token, no cert": {
				token:         invalidToken,
				certKey:       "noCert",
				errorExpected: true,
				errorString:   unauthorizedError,
			},

			"no token, valid cert": {
				token:        noToken,
				certKey:      "validCert",
				expectedUser: certUser,
			},

			"no token, invalid cert": {
				token:         noToken,
				certKey:       "invalidCert",
				errorExpected: true,
				errorString:   unauthorizedError,
			},

			"no token, no cert": {
				token:         noToken,
				certKey:       "noCert",
				errorExpected: true,
				errorString:   anonymousError,
			},
		} {
			g.By(name)

			// Skip if test requires validCert but kubeconfig doesn't have one
			if test.certKey == "validCert" && len(validCert.CertData) == 0 && validCert.CertFile == "" {
				e2e.Logf("Skipping test '%s': no available client certificate in kubeconfig", name)
				continue
			}

			adminConfig := restclient.AnonymousClientConfig(oc.AdminConfig())
			adminConfig.TLSClientConfig = certs[test.certKey]
			adminConfig.BearerToken = test.token
			adminConfig.CAData = oc.AdminConfig().CAData

			tokenPrefix := test.token
			if strings.HasPrefix(test.token, "sha256~") && len(test.token) > len("sha256~")+6 {
				tokenPrefix = test.token[:len("sha256~")+6] + "..."
			}
			e2e.Logf("Test case '%s': token='%s', using cert='%s'\n", name, tokenPrefix, test.certKey)
			if len(adminConfig.TLSClientConfig.CertData) > 0 {
				certs, err := crypto.CertsFromPEM(adminConfig.TLSClientConfig.CertData)
				if err != nil {
					e2e.Logf("Failed to parse cert: %v\n", err)
				} else {
					for i, cert := range certs {
						fingerprint := sha256.Sum256(cert.Raw)
						e2e.Logf("Cert[%d]: Subject=%s, Issuer=%s, SHA256=%x%s\n",
							i, cert.Subject.String(), cert.Issuer.String(), fingerprint[:3], "xxxxxx")
					}
				}
			} else {
				e2e.Logf("no available client certificate in kubeconfig\n")
			}

			userClient := userv1client.NewForConfigOrDie(adminConfig)
			user, err := userClient.UserV1().Users().Get(context.Background(), "~", metav1.GetOptions{})

			if test.errorExpected {
				o.Expect(err).ToNot(o.BeNil())
				o.Expect(err.Error()).To(o.Equal(test.errorString))
			} else {
				o.Expect(err).To(o.BeNil())
				o.Expect(user).ToNot(o.BeNil())
				o.Expect(user.Name).To(o.Equal(test.expectedUser))
			}
		}
	})
})
