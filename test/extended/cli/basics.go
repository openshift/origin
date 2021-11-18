package cli

import (
	"os"
	"path/filepath"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
	k8simage "k8s.io/kubernetes/test/utils/image"
)

var _ = g.Describe("[sig-cli] oc basics", func() {
	defer g.GinkgoRecover()

	var (
		oc                   = exutil.NewCLI("oc-basics")
		cmdTestData          = exutil.FixturePath("testdata", "cmd", "test", "cmd", "testdata")
		mixedAPIVersionsFile = exutil.FixturePath("testdata", "mixed-api-versions.yaml")
		oauthAccessTokenFile = filepath.Join(cmdTestData, "oauthaccesstoken.yaml")
		templateFile         = filepath.Join(cmdTestData, "application-template-stibuild.json")
	)

	g.It("can create and interact with a list of resources", func() {
		file, err := replaceImageInFile(mixedAPIVersionsFile, "openshift/hello-openshift", k8simage.GetE2EImage(k8simage.EchoServer))
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

	g.It("can create deploymentconfig and clusterquota", func() {
		nginx := k8simage.GetE2EImage(k8simage.Nginx)
		tools := "image-registry.openshift-image-registry.svc:5000/openshift/tools:latest"

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

		// need admin here
		ocAdmin := oc.AsAdmin()
		err = ocAdmin.Run("create").Args("clusterquota", "limit-bob", "--project-label-selector=openshift.io/requestor=user-bob", "--hard=pods=10").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		err = ocAdmin.Run("delete").Args("clusterquota", "limit-bob").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It("can patch resources", func() {
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

	g.It("can describe an OAuth access token", func() {
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

	g.It("can process templates", func() {
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
})
