package cli

import (
	"context"
	"os"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-cli] oc observe", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithoutNamespace("oc-observe").AsAdmin()

	g.It("works as expected", func() {
		g.By("Find out the clusterIP of the kubernetes.default service")
		kubernetesSVC, err := oc.AdminKubeClient().CoreV1().Services("default").Get(context.Background(), "kubernetes", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("basic scenarios")
		out, err := oc.Run("observe").Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("you must specify at least one argument containing the resource to observe"))

		out, err = oc.Run("observe").Args("serviceaccounts", "--once").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.Or(o.ContainSubstring("Sync ended"), o.ContainSubstring("Nothing to sync")))

		out, err = oc.Run("observe").Args("daemonsets", "--once").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.Or(o.ContainSubstring("Sync ended"), o.ContainSubstring("Nothing to sync, exiting immediately")))

		out, err = oc.Run("observe").Args("services", "--once", "--all-namespaces").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("default kubernetes"))

		out, err = oc.Run("observe").Args("services", "--once", "--all-namespaces", "--print-metrics-on-exit").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring(`observe_counts{type="Sync"}`))

		out, err = oc.Run("observe").Args("services", "--once", "--names", "echo").Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("--delete and --names must both be specified"))

		out, err = oc.Run("observe").Args("services", "--exit-after=1s").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("Shutting down after 1s ..."))

		out, err = oc.Run("observe").Args("services", "--exit-after=3s", "--all-namespaces", "--print-metrics-on-exit").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring(`observe_counts{type="Sync"}`))

		out, err = oc.Run("observe").Args("services", "--exit-after=3s", "--all-namespaces", "--names", "echo", "--names", "default/notfound", "--delete", "echo", "--delete", "remove").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("remove default notfound"))

		g.By("error counting")
		out, err = oc.Run("observe").Args("services", "--exit-after=1m", "--all-namespaces", "--maximum-errors=1", "--", "/bin/sh", "-c", "exit 1").Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("reached maximum error limit of 1, exiting"))

		out, err = oc.Run("observe").Args("services", "--exit-after=1m", "--all-namespaces", "--retry-on-exit-code=2", "--maximum-errors=1", "--loglevel=4", "--", "/bin/sh", "-c", "exit 2").Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("retrying command: exit status 2"))

		g.By("argument templates")
		out, err = oc.Run("observe").Args("services", "--once", "--all-namespaces", "--template='{ .spec.clusterIP }'").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.Or(o.ContainSubstring(kubernetesSVC.Spec.ClusterIP), o.ContainSubstring("fd02::1")))

		out, err = oc.Run("observe").Args("services", "--once", "--all-namespaces", "--template='{{ .spec.clusterIP }}'", "--output=go-template").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.Or(o.ContainSubstring(kubernetesSVC.Spec.ClusterIP), o.ContainSubstring("fd02::1")))

		out, err = oc.Run("observe").Args("services", "--once", "--all-namespaces", "--template='bad{ .missingkey }key'").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("badkey"))

		out, err = oc.Run("observe").Args("services", "--once", "--all-namespaces", "--template='bad{ .missingkey }key'", "--allow-missing-template-keys=false").Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("missingkey is not found"))

		out, err = oc.Run("observe").Args("services", "--once", "--all-namespaces", "--template='{{ .unknown }}'", "--output=go-template").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("default kubernetes"))

		out, err = oc.Run("observe").Args("services", "--once", "--all-namespaces", `--template='bad{{ or (.unknown) "" }}key'`, "--output=go-template").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("badkey"))

		out, err = oc.Run("observe").Args("services", "--once", "--all-namespaces", "--template='bad{{ .unknown }}key'", "--output=go-template", "--allow-missing-template-keys=false").Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("map has no entry for key"))

		g.By("event environment variables")
		o.Expect(os.Setenv("MYENV", "should_be_passed")).NotTo(o.HaveOccurred())
		out, err = oc.Run("observe").Args("services", "--once", "--all-namespaces", "--type-env-var=EVENT", "--", "/bin/sh", "-c", "echo $EVENT $MYENV").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("Sync should_be_passed"))
		o.Expect(os.Unsetenv("MYENV")).NotTo(o.HaveOccurred())
	})

	g.It("works as expected with cluster operators [apigroup:config.openshift.io]", func() {
		out, err := oc.Run("observe").Args("clusteroperators", "--once", "--listen-addr=:11252").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("kube-apiserver"))
	})
})
