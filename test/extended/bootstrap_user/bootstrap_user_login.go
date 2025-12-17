package bootstrap_user

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"golang.org/x/crypto/bcrypt"

	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	clocktesting "k8s.io/utils/clock/testing"

	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-auth][Feature:BootstrapUser] The bootstrap user", func() {
	defer g.GinkgoRecover()

	// since login mutates the current kubeconfig we want to use NewCLI
	// as that will give each one of our test runs a new config via SetupProject
	oc := exutil.NewCLI("bootstrap-login")

	g.It("should successfully login with password decoded from kubeadmin secret [Disruptive]", g.Label("Size:M"), func() {
		var originalPasswordHash []byte
		secretExists := true
		recorder := events.NewInMemoryRecorder("", clocktesting.NewFakePassiveClock(time.Now()))

		// always restore cluster to original state at the end
		defer func() {
			if secretExists {
				originalKubeadminSecret := generateSecret(originalPasswordHash)
				e2e.Logf("restoring original kubeadmin user")
				_, _, err := resourceapply.ApplySecret(context.TODO(), oc.AsAdmin().KubeClient().CoreV1(), recorder, originalKubeadminSecret)
				o.Expect(err).NotTo(o.HaveOccurred())
				return
			}

			err := oc.AsAdmin().KubeClient().CoreV1().Secrets("kube-system").Delete(context.Background(), "kubeadmin", metav1.DeleteOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
		}()

		// We aren't testing that the installer has created this secret here, instead,
		// we create it/apply new data/restore it after (if it existed, or delete it if
		// it didn't.  Here, we are only testing the oauth flow
		// of authenticating/creating the special kube:admin user.
		// Testing that the installer properly generated the password/secret is the
		// responsibility of the installer.
		secret, err := oc.AsAdmin().KubeClient().CoreV1().Secrets("kube-system").Get(context.Background(), "kubeadmin", metav1.GetOptions{})
		if kerrors.IsNotFound(err) {
			secretExists = false
			err = nil // ignore not found
		}
		o.Expect(err).NotTo(o.HaveOccurred())
		if secretExists {
			// not validating secret here, but it should have this if it's there
			originalPasswordHash = secret.Data["kubeadmin"]
		}

		password, passwordHash, err := generatePassword()
		o.Expect(err).NotTo(o.HaveOccurred())
		kubeadminSecret := generateSecret(passwordHash)
		_, _, err = resourceapply.ApplySecret(context.TODO(), oc.AsAdmin().KubeClient().CoreV1(), recorder, kubeadminSecret)
		o.Expect(err).NotTo(o.HaveOccurred())

		e2e.Logf("logging in as kubeadmin user")
		err = wait.Poll(10*time.Second, 5*time.Minute, func() (done bool, err error) {
			out, err := oc.Run("login").Args("-u", "kubeadmin").InputString(password + "\n").Output()
			if err != nil {
				e2e.Logf("oc login for bootstrap user failed: %s", strings.Replace(err.Error(), password, "<redacted>", -1))
				return false, nil
			}
			if !strings.Contains(out, "Login successful") {
				e2e.Logf("oc login output did not contain success message:\n%s", strings.Replace(out, password, "<redacted>", -1))
				return false, nil
			}
			return true, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		user, err := oc.Run("whoami").Args().Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(user).To(o.ContainSubstring("kube:admin"))
	})
})

func generatePassword() (string, []byte, error) {
	// these are all copied, but we could hardcode a single one if we liked.
	password := random256BitsString()
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

// RandomBits returns a random byte slice with at least the requested bits of entropy.
// Callers should avoid using a value less than 256 unless they have a very good reason.
func randomBits(bits int) []byte {
	size := bits / 8
	if bits%8 != 0 {
		size++
	}
	b := make([]byte, size)
	if _, err := rand.Read(b); err != nil {
		panic(err) // rand should never fail
	}
	return b
}

// RandomBitsString returns a random string with at least the requested bits of entropy.
// It uses RawURLEncoding to ensure we do not get / characters or trailing ='s.
func randomBitsString(bits int) string {
	return base64.RawURLEncoding.EncodeToString(randomBits(bits))
}

// Random256BitsString is a convenience function for calling RandomBitsString(256).
// Callers that need a random string should use this function unless they have a
// very good reason to need a different amount of entropy.
func random256BitsString() string {
	// 32 bytes (256 bits) = 43 base64-encoded characters
	return randomBitsString(256)
}
