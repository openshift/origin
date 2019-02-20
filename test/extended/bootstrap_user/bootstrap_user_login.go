package bootstrap_user

import (
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	"github.com/openshift/origin/pkg/oauthserver/server/crypto"

	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	exutil "github.com/openshift/origin/test/extended/util"
	"golang.org/x/crypto/bcrypt"
)

var _ = g.Describe("The bootstrap user", func() {
	defer g.GinkgoRecover()
	var oc *exutil.CLI
	var originalPasswordHash []byte
	secretExists := true
	recorder := events.NewInMemoryRecorder("")
	oc = exutil.NewCLI("bootstrap-login", exutil.KubeConfigPath())
	g.It("should successfully login with password decoded from kubeadmin secret [Flaky]", func() {
		// We aren't testing that the installer has created this secret here, instead,
		// we create it/apply new data/restore it after (if it existed, or delete it if
		// it didn't.  Here, we are only testing the oauth flow
		// of authenticating/creating the special kube:admin user.
		// Testing that the installer properly generated the password/secret is the
		// responsibility of the installer.
		secret, err := oc.AsAdmin().KubeClient().CoreV1().Secrets("kube-system").Get("kubeadmin", metav1.GetOptions{})
		if err != nil {
			if !kerrors.IsNotFound(err) {
				o.Expect(err).NotTo(o.HaveOccurred())
			} else {
				secretExists = false
			}
		}
		if secretExists {
			// not validating secret here, but it should have this if it's there
			originalPasswordHash = secret.Data["kubeadmin"]
		}
		password, passwordHash, err := generatePassword()
		o.Expect(err).NotTo(o.HaveOccurred())
		kubeadminSecret := generateSecret(passwordHash)
		_, _, err = resourceapply.ApplySecret(oc.AsAdmin().KubeClient().CoreV1(), recorder, kubeadminSecret)
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("logging in as kubeadmin user")
		out, err := oc.Run("login").Args("-u", "kubeadmin", "-p", password).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("Login successful"))
		user, err := oc.Run("whoami").Args().Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(user).To(o.ContainSubstring("kube:admin"))
		//Now, restore cluster to original state
		if secretExists {
			originalKubeadminSecret := generateSecret(originalPasswordHash)
			e2e.Logf("restoring original kubeadmin user")
			_, _, err = resourceapply.ApplySecret(oc.AsAdmin().KubeClient().CoreV1(), recorder, originalKubeadminSecret)
			o.Expect(err).NotTo(o.HaveOccurred())
		} else {
			err := oc.AsAdmin().KubeClient().CoreV1().Secrets("kube-system").Delete("kubeadmin", &metav1.DeleteOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
		}
	})
})

func generatePassword() (string, []byte, error) {
	password := crypto.Random256BitsString()
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", nil, err
	}
	return password, bytes, nil
}

func generateSecret(data []byte) *v1.Secret {
	return &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubeadmin",
			Namespace: "kube-system",
		},
		Data: map[string][]byte{
			"kubeadmin": data,
		},
	}
}
