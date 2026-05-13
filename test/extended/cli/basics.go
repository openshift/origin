package cli

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	k8simage "k8s.io/kubernetes/test/utils/image"
	admissionapi "k8s.io/pod-security-admission/api"

	exutil "github.com/openshift/origin/test/extended/util"
	exutilimage "github.com/openshift/origin/test/extended/util/image"
)

var (
	serverPattern  = regexp.MustCompile(`http(s)?:\/\/.*\:[0-9]+`)
	consolePattern = regexp.MustCompile(`http(s)?:\/\/.*`)
)

var _ = g.Describe("[sig-cli] oc basics", func() {
	defer g.GinkgoRecover()

	var (
		oc                   = exutil.NewCLIWithPodSecurityLevel("oc-basics", admissionapi.LevelBaseline)
		cmdTestData          = exutil.FixturePath("testdata", "cmd", "test", "cmd", "testdata")
		mixedAPIVersionsFile = exutil.FixturePath("testdata", "mixed-api-versions.yaml")
		oauthAccessTokenFile = filepath.Join(cmdTestData, "oauthaccesstoken.yaml")
		templateFile         = filepath.Join(cmdTestData, "application-template-mix.json")
	)

	g.It("can create and interact with a list of resources", func() {
		file, err := replaceImageInFile(mixedAPIVersionsFile, "openshift/hello-openshift", k8simage.GetE2EImage(k8simage.NginxNew))
		o.Expect(err).NotTo(o.HaveOccurred())
		defer os.Remove(file)

		err = oc.Run("create").Args("-f", file).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		out, err := oc.Run("get").Args("-f", file, "-o", "yaml").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("v1-job"))

		err = oc.Run("label").Args("-f", file, "mylabel=a").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		err = oc.Run("annotate").Args("-f", file, "myannotation=b").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		out, err = oc.Run("get").Args("-f", file, `--output=jsonpath="{..metadata.labels.mylabel}"`).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("a a a a"))

		out, err = oc.Run("get").Args("-f", file, `--output=jsonpath="{..metadata.annotations.myannotation}"`).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("b b b b"))

		err = oc.Run("delete").Args("-f", file).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It("can create deploymentconfig and clusterquota [apigroup:apps.openshift.io]", func() {
		nginx := k8simage.GetE2EImage(k8simage.Nginx)
		tools := exutilimage.ShellImage()

		err := oc.Run("create").Args("dc", "my-nginx", "--image="+nginx).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		err = oc.Run("delete").Args("dc", "my-nginx").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		err = oc.Run("create").Args("dc", "test", "--image="+tools).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		err = oc.Run("run").Args("test2", "--image="+tools, "--restart=Never").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		err = oc.Run("create").Args("job", "test3", "--image="+tools).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		err = oc.Run("delete").Args("dc/test", "pod/test2", "job/test3").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		// validate dry-run creation of resourcequota
		out, err := oc.Run("create").Args("quota", "quota", "--dry-run").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("resourcequota/quota created (dry run)"))

		// need admin here
		ocAdmin := oc.AsAdmin()
		err = ocAdmin.Run("create").Args("clusterquota", "limit-bob", "--project-label-selector=openshift.io/requestor=user-bob", "--hard=pods=10").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		err = ocAdmin.Run("delete").Args("clusterquota", "limit-bob").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It("can patch resources [apigroup:user.openshift.io]", func() {
		// need admin here
		ocAdmin := oc.AsAdmin()

		err := ocAdmin.Run("adm").Args("groups", "new", "patch-group").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		err = ocAdmin.Run("patch").Args("group", "patch-group", "-p", `users: ["myuser"]`, "--loglevel=8").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		out, err := ocAdmin.Run("get").Args("group", "patch-group", "-o", "yaml").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("myuser"))

		err = ocAdmin.Run("patch").Args("group", "patch-group", "-p", `users: []`, "--loglevel=8").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		// apply same patch twice results in "not patched" text
		out, err = ocAdmin.Run("patch").Args("group", "patch-group", "-p", `users: []`).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring(`patched (no change)`))

		// applying an invalid patch results in exit code 1 with error text
		out, err = ocAdmin.Run("patch").Args("group", "patch-group", "-p", `users: ""`).Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring(`cannot restore slice from string`))

		out, err = ocAdmin.Run("get").Args("group", "patch-group", "-o", "yaml").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("users: []"))

		err = ocAdmin.Run("delete").Args("group", "patch-group").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It("can describe an OAuth access token [apigroup:oauth.openshift.io]", func() {
		// need admin here
		ocAdmin := oc.AsAdmin()

		err := ocAdmin.Run("create").Args("-f", oauthAccessTokenFile).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		out, err := ocAdmin.Run("describe").Args("oauthaccesstoken", "sha256~efaca6fab897953ffcb4f857eb5cbf2cf3a4c33f1314b4922656303426b1cfc9").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("efaca6fab897953ffcb4f857eb5cbf2cf3a4c33f1314b4922656303426b1cfc9"))

		err = ocAdmin.Run("delete").Args("oauthaccesstoken", "sha256~efaca6fab897953ffcb4f857eb5cbf2cf3a4c33f1314b4922656303426b1cfc9").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It("can output expected --dry-run text", func() {
		out, err := oc.Run("create").Args("deploymentconfig", "--dry-run", "foo", "--image=bar", "-o", "name").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("deploymentconfig.apps.openshift.io/foo"))

		out, err = oc.Run("run").Args("--dry-run", "foo", "--image=bar", "-o", "name", "--restart=Never").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("pod/foo"))

		out, err = oc.Run("create").Args("job", "--dry-run", "foo", "--image=bar", "-o", "name").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("job.batch/foo"))
	})

	g.It("can process templates [apigroup:template.openshift.io]", func() {
		name := filepath.Join(os.TempDir(), "template.json")

		out, err := oc.Run("process").Args("-f", templateFile, "-l", "name=mytemplate").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		err = os.WriteFile(name, []byte(out), 0744)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer os.Remove(name)

		err = oc.Run("create").Args("-f", name).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		err = oc.Run("delete").Args("all", "-l", "name=mytemplate").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It("can get version information from API", func() {
		kubeCA := oc.UserConfig().CAData
		transport := http.DefaultTransport

		if len(kubeCA) > 0 {
			rootCAs, err := x509.SystemCertPool()
			o.Expect(err).NotTo(o.HaveOccurred())

			if rootCAs == nil {
				rootCAs = x509.NewCertPool()
			}

			ok := rootCAs.AppendCertsFromPEM(kubeCA)
			o.Expect(ok).To(o.BeTrue())

			transport = &http.Transport{
				TLSClientConfig: &tls.Config{
					RootCAs: rootCAs,
				},
				Proxy: http.ProxyFromEnvironment,
			}
		}

		hc := http.Client{
			Transport: transport,
		}

		host := oc.UserConfig().Host
		o.Expect(host).NotTo(o.BeEmpty())

		resp, err := hc.Get(host + "/version")
		o.Expect(err).NotTo(o.HaveOccurred())
		defer resp.Body.Close()

		o.Expect(resp.StatusCode).To(o.Equal(http.StatusOK))

		body, err := io.ReadAll(resp.Body)
		o.Expect(err).NotTo(o.HaveOccurred())

		version := map[string]string{}
		err = json.Unmarshal(body, &version)
		o.Expect(err).NotTo(o.HaveOccurred())

		o.Expect(version["major"]).NotTo(o.BeEmpty())
		o.Expect(version["minor"]).NotTo(o.BeEmpty())
		o.Expect(version["gitVersion"]).NotTo(o.BeEmpty())
		o.Expect(version["gitTreeState"]).NotTo(o.BeEmpty())
		o.Expect(version["buildDate"]).NotTo(o.BeEmpty())
		o.Expect(version["goVersion"]).NotTo(o.BeEmpty())
		o.Expect(version["compiler"]).NotTo(o.BeEmpty())
		o.Expect(version["platform"]).NotTo(o.BeEmpty())
	})

	g.It("can get version information from CLI", func() {
		out, err := oc.Run("version").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("Client Version: "))
		o.Expect(out).To(o.ContainSubstring("Kubernetes Version: "))
	})

	g.It("can show correct whoami result", func() {
		out, err := oc.Run("whoami").Args("--show-server").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		found := serverPattern.MatchString(out)
		o.Expect(found).To(o.BeTrue())
	})

	g.It("can show correct whoami result with console", func() {
		out, err := oc.Run("whoami").Args("--show-console").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		found := consolePattern.MatchString(out)
		o.Expect(found).To(o.BeTrue())
	})
})
