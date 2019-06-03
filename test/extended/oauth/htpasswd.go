package oauth

import (
	"path/filepath"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Serial][Suite:openshift/oauth/serial] Htpasswd identity provider", func() {
	defer g.GinkgoRecover()
	var (
		oc                    = exutil.NewCLI("htpasswd-configure", exutil.KubeConfigPath())
		testDataBaseDir       = exutil.FixturePath("testdata", "oauth_idp")
		oauthHtpasswdFixture  = filepath.Join(testDataBaseDir, "oauth-htpasswd.yaml")
		htpasswdSecretFixture = filepath.Join(testDataBaseDir, "htpasswd-secret.yaml")
		htpasswdSecretName    = "htpasswd"
		oauthNoIdpFixture     = filepath.Join(testDataBaseDir, "oauth-noidp.yaml")
	)
	g.AfterEach(func() {
		err := oc.AsAdmin().KubeClient().CoreV1().Secrets("openshift-config").Delete(htpasswdSecretName, &metav1.DeleteOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("removing htpasswd idp from oauth")
		err = oc.AsAdmin().Run("replace").Args("-f", oauthNoIdpFixture).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = waitForAuthOperatorAvailable(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		// cleanup created user/identity
		err = oc.AsAdmin().Run("delete").Args("identities", "htpasswd:testuser").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().Run("delete").Args("user", "testuser").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It("should successfully be configured", func() {
		e2e.Logf("configuring htpasswd")
		oc.SetNamespace("openshift-config")
		err := oc.AsAdmin().Run("apply").Args("-f", htpasswdSecretFixture).Execute()
		err = oc.AsAdmin().Run("replace").Args("-f", oauthHtpasswdFixture).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = waitForAuthOperatorAvailable(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("logging in as htpasswd user testuser")
		err = wait.Poll(2*time.Second, 30*time.Second, func() (done bool, err error) {
			out, err := oc.Run("login").Args("-u", "testuser").InputString("password" + "\n").Output()
			if err != nil {
				e2e.Logf("waiting for oc login as htpasswd configured user to succeed")
				return false, nil
			}
			if !strings.Contains(out, "Login successful") {
				e2e.Logf("oc login output did not contain success message:\n%s", strings.Replace(out, "password", "<redacted>", -1))
				return false, nil
			}
			e2e.Logf("oc login -u testuser succeeded")
			return true, nil
		})
		user, err := oc.Run("whoami").Args().Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(user).To(o.ContainSubstring("testuser"))
	})
})
