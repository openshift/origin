package images

import (
	"fmt"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	e2e "k8s.io/kubernetes/test/e2e/framework"
	admissionapi "k8s.io/pod-security-admission/api"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-imageregistry][Serial] Image signature workflow", func() {
	defer g.GinkgoRecover()

	var (
		oc                 = exutil.NewCLIWithPodSecurityLevel("registry-signing", admissionapi.LevelBaseline)
		signerBuildFixture = exutil.FixturePath("testdata", "signer-buildconfig.yaml")
	)

	g.AfterEach(func() {
		if g.CurrentSpecReport().Failed() {
			exutil.DumpPodStates(oc)
			exutil.DumpConfigMapStates(oc)
			exutil.DumpPodLogsStartingWith("", oc)
		}
	})

	g.It("can push a signed image to openshift registry and verify it [apigroup:user.openshift.io][apigroup:image.openshift.io]", g.Label("Size:L"), func() {
		g.By("building a signer image that knows how to sign images")
		output, err := oc.Run("create").Args("-f", signerBuildFixture).Output()
		if err != nil {
			fmt.Fprintf(g.GinkgoWriter, "%s\n\n", output)
		}
		o.Expect(err).NotTo(o.HaveOccurred())
		err = exutil.WaitForAnImageStreamTag(oc, oc.Namespace(), "signer", "latest")
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("looking up the openshift registry URL")
		registryURL, err := GetDockerRegistryURL(oc)
		signerImage := fmt.Sprintf("%s/%s/signer:latest", registryURL, oc.Namespace())
		signedImage := fmt.Sprintf("%s/%s/signed:latest", registryURL, oc.Namespace())
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("obtaining bearer token for the test user")
		user := oc.Username()
		token, err := oc.Run("whoami").Args("-t").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("granting the image-signer role to test user")
		_, err = oc.AsAdmin().Run("adm").Args("policy", "add-cluster-role-to-user", "system:image-signer", oc.Username()).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		// TODO: The test user needs to be able to unlink the /dev/random which is owned by a
		// root. This cannot be done during image build time because the /dev is plugged into
		// container after it starts. This SCC could be avoided in the future when /dev/random
		// issue is fixed in Docker.
		g.By("granting the anyuid scc to test user")
		_, err = oc.AsAdmin().Run("adm").Args("policy", "add-scc-to-user", "anyuid", oc.Username()).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("preparing the image stream where the signed image will be pushed")
		_, err = oc.Run("create").Args("imagestream", "signed").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("granting the image-auditor role to test user")
		_, err = oc.AsAdmin().Run("adm").Args("policy", "add-cluster-role-to-user", "system:image-auditor", oc.Username()).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		pod, err := exutil.NewPodExecutor(oc, "sign-and-push", signerImage)
		o.Expect(err).NotTo(o.HaveOccurred())

		// Generate GPG key
		// Note that we need to replace the /dev/random with /dev/urandom to get more entropy
		// into container so we can successfully generate the GPG keypair.
		g.By("creating dummy GPG key")
		out, err := pod.Exec("rm -f /dev/random; ln -sf /dev/urandom /dev/random && " +
			"GNUPGHOME=/var/lib/origin/gnupg gpg2 --batch --gen-key --pinentry-mode=loopback --passphrase '' dummy_key.conf && " +
			"GNUPGHOME=/var/lib/origin/gnupg gpg2 --output=gnupg/pubring.gpg --export joe@foo.bar")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("keybox '/var/lib/origin/gnupg/pubring.kbx' created"))

		// Create kubeconfig for oc
		g.By("logging as a test user")
		out, err = pod.Exec("oc login https://$KUBERNETES_SERVICE_HOST:$KUBERNETES_SERVICE_PORT --token=" + token + " --certificate-authority=/run/secrets/kubernetes.io/serviceaccount/ca.crt")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("Logged in"))

		// Sign and copy the memcached image into target image stream tag
		g.By("signing a just-built image and pushing it into openshift registry")
		out, err = pod.Exec(strings.Join([]string{
			"GNUPGHOME=/var/lib/origin/gnupg",
			"skopeo", "--debug",
			"copy", "--sign-by", "joe@foo.bar",
			"--src-creds=" + user + ":" + token,
			"--dest-creds=" + user + ":" + token,

			// Expect to use /run/secrets/kubernetes.io/serviceaccount/ca.crt
			"--src-cert-dir=/run/secrets/kubernetes.io/serviceaccount",
			"--dest-cert-dir=/run/secrets/kubernetes.io/serviceaccount",

			"docker://" + signerImage,
			"docker://" + signedImage,
		}, " "))
		fmt.Fprintf(g.GinkgoWriter, "output: %s\n", out)
		o.Expect(err).NotTo(o.HaveOccurred())

		err = exutil.WaitForAnImageStreamTag(oc, oc.Namespace(), "signed", "latest")
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("obtaining the signed:latest image name")
		imageName, err := oc.Run("get").Args("istag", "signed:latest", "-o", "jsonpath={.image.metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("expecting the image to have unverified signature")
		out, err = oc.Run("describe").Args("istag", "signed:latest").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("%s", out)
		o.Expect(out).To(o.ContainSubstring("Unverified"))

		out, err = pod.Exec(strings.Join([]string{
			"GNUPGHOME=/var/lib/origin/gnupg",
			"oc", "adm",
			"verify-image-signature",
			"--insecure=true", // TODO: import the ca certificate into the signing pod
			"--loglevel=8",
			imageName,
			"--expected-identity=" + signedImage,
			"--save",
		}, " "))
		fmt.Fprintf(g.GinkgoWriter, "output: %s\n", out)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("identity is now confirmed"))

		g.By("checking the signature is present on the image and it is now verified")
		out, err = oc.Run("describe").Args("istag", "signed:latest").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("Verified"))
	})
})
