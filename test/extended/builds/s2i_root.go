package builds

import (
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"

	buildv1 "github.com/openshift/api/build/v1"
	exutil "github.com/openshift/origin/test/extended/util"
)

func Before(oc *exutil.CLI) {
	exutil.PreTestDump()
}

func After(oc *exutil.CLI) {
	if g.CurrentGinkgoTestDescription().Failed {
		exutil.DumpPodStates(oc)
		exutil.DumpConfigMapStates(oc)
		exutil.DumpPodLogsStartingWith("", oc)
	}
}

var _ = g.Describe("[sig-builds][Feature:Builds] s2i build with a root user image", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLI("s2i-build-root", exutil.KubeConfigPath())

	g.It("should create a root build and fail without a privileged SCC", func() {
		g.Skip("TODO: figure out why we aren't properly denying this, also consider whether we still need to deny it")
		Before(oc)
		defer After(oc)

		err := oc.Run("new-app").Args("docker.io/openshift/test-build-roots2i~https://github.com/sclorg/nodejs-ex", "--name", "nodejsfail").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		err = exutil.WaitForABuild(oc.BuildClient().BuildV1().Builds(oc.Namespace()), "nodejsfail-1", nil, nil, nil)
		o.Expect(err).To(o.HaveOccurred())

		build, err := oc.BuildClient().BuildV1().Builds(oc.Namespace()).Get("nodejsfail-1", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(build.Status.Phase).To(o.Equal(buildv1.BuildPhaseFailed))
		o.Expect(build.Status.Reason).To(o.BeEquivalentTo("PullBuilderImageFailed" /*s2istatus.ReasonPullBuilderImageFailed*/))
		o.Expect(build.Status.Message).To(o.BeEquivalentTo("Failed to pull builder image." /*s2istatus.ReasonMessagePullBuilderImageFailed*/))

		podname := build.Annotations[buildv1.BuildPodNameAnnotation]
		pod, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).Get(podname, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		containers := make([]corev1.Container, len(pod.Spec.Containers)+len(pod.Spec.InitContainers))
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
		Before(oc)
		defer After(oc)
		g.By("adding builder account to privileged SCC")
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			scc, err := oc.AdminSecurityClient().SecurityV1().SecurityContextConstraints().Get("privileged", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			scc.Users = append(scc.Users, "system:serviceaccount:"+oc.Namespace()+":builder")
			_, err = oc.AdminSecurityClient().SecurityV1().SecurityContextConstraints().Update(scc)
			return err
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		err = oc.Run("new-build").Args("docker.io/openshift/test-build-roots2i~https://github.com/sclorg/nodejs-ex", "--name", "nodejspass").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		err = exutil.WaitForABuild(oc.BuildClient().BuildV1().Builds(oc.Namespace()), "nodejspass-1", nil, nil, nil)
		o.Expect(err).NotTo(o.HaveOccurred())

		build, err := oc.BuildClient().BuildV1().Builds(oc.Namespace()).Get("nodejspass-1", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		podname := build.Annotations[buildv1.BuildPodNameAnnotation]
		pod, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).Get(podname, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		containers := make([]corev1.Container, len(pod.Spec.Containers)+len(pod.Spec.InitContainers))
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
