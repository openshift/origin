package builds

import (
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	kapiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	exutil "github.com/openshift/origin/test/extended/util"
	s2istatus "github.com/openshift/source-to-image/pkg/util/status"
)

var _ = g.Describe("[Feature:Builds][Conformance] s2i build with a root user image", func() {
	defer g.GinkgoRecover()

	var (
		oc = exutil.NewCLI("s2i-build-root", exutil.KubeConfigPath())
	)

	g.Context("", func() {
		g.BeforeEach(func() {
			exutil.DumpDockerInfo()
		})

		g.JustBeforeEach(func() {
			g.By("waiting for builder service account")
			err := exutil.WaitForBuilderAccount(oc.AdminKubeClient().Core().ServiceAccounts(oc.Namespace()))
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("creating a root build container")
			err = oc.Run("new-build").Args("-D", "FROM centos/nodejs-6-centos7\nUSER 0", "--name", "nodejsroot").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			err = exutil.WaitForABuild(oc.BuildClient().Build().Builds(oc.Namespace()), "nodejsroot-1", nil, nil, nil)
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.AfterEach(func() {
			if g.CurrentGinkgoTestDescription().Failed {
				exutil.DumpPodStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
			}
		})

		g.It("should create a root build and fail without a privileged SCC", func() {
			err := oc.Run("new-app").Args("nodejsroot~https://github.com/openshift/nodejs-ex", "--name", "nodejsfail").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			err = exutil.WaitForABuild(oc.BuildClient().Build().Builds(oc.Namespace()), "nodejsfail-1", nil, nil, nil)
			o.Expect(err).To(o.HaveOccurred())

			build, err := oc.BuildClient().Build().Builds(oc.Namespace()).Get("nodejsfail-1", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(build.Status.Phase).To(o.Equal(buildapi.BuildPhaseFailed))
			o.Expect(build.Status.Reason).To(o.BeEquivalentTo(s2istatus.ReasonPullBuilderImageFailed))
			o.Expect(build.Status.Message).To(o.BeEquivalentTo(s2istatus.ReasonMessagePullBuilderImageFailed))

			podname := build.Annotations[buildapi.BuildPodNameAnnotation]
			pod, err := oc.KubeClient().Core().Pods(oc.Namespace()).Get(podname, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			containers := make([]kapiv1.Container, len(pod.Spec.Containers)+len(pod.Spec.InitContainers))
			copy(containers, pod.Spec.Containers)
			copy(containers[len(pod.Spec.Containers):], pod.Spec.InitContainers)

			for _, c := range containers {
				env := map[string]string{}
				for _, e := range c.Env {
					env[e.Name] = e.Value
				}
				o.Expect(env["DROP_CAPS"]).To(o.Equal("KILL,MKNOD,SETGID,SETUID"))
				o.Expect(env["ALLOWED_UIDS"]).To(o.Equal("1-"))
			}
		})

		g.It("should create a root build and pass with a privileged SCC", func() {
			g.By("adding builder account to privileged SCC")
			err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				scc, err := oc.AdminSecurityClient().Security().SecurityContextConstraints().Get("privileged", metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				scc.Users = append(scc.Users, "system:serviceaccount:"+oc.Namespace()+":builder")
				_, err = oc.AdminSecurityClient().Security().SecurityContextConstraints().Update(scc)
				return err
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			err = oc.Run("new-app").Args("nodejsroot~https://github.com/openshift/nodejs-ex", "--name", "nodejspass").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			err = exutil.WaitForABuild(oc.BuildClient().Build().Builds(oc.Namespace()), "nodejspass-1", nil, nil, nil)
			o.Expect(err).NotTo(o.HaveOccurred())

			build, err := oc.BuildClient().Build().Builds(oc.Namespace()).Get("nodejspass-1", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			podname := build.Annotations[buildapi.BuildPodNameAnnotation]
			pod, err := oc.KubeClient().Core().Pods(oc.Namespace()).Get(podname, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			containers := make([]kapiv1.Container, len(pod.Spec.Containers)+len(pod.Spec.InitContainers))
			copy(containers, pod.Spec.Containers)
			copy(containers[len(pod.Spec.Containers):], pod.Spec.InitContainers)

			for _, c := range containers {
				env := map[string]string{}
				for _, e := range c.Env {
					env[e.Name] = e.Value
				}
				o.Expect(env).NotTo(o.HaveKey("DROP_CAPS"))
				o.Expect(env).NotTo(o.HaveKey("ALLOWED_UIDS"))
			}
		})
	})
})
