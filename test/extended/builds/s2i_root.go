package builds

import (
	"context"
	"fmt"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	admissionapi "k8s.io/pod-security-admission/api"

	buildv1 "github.com/openshift/api/build/v1"

	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/image"
)

func Before(oc *exutil.CLI) {
	exutil.PreTestDump()
}

func After(oc *exutil.CLI) {
	if g.CurrentSpecReport().Failed() {
		exutil.DumpPodStates(oc)
		exutil.DumpConfigMapStates(oc)
		exutil.DumpPodLogsStartingWith("", oc)
	}
}

var _ = g.Describe("[sig-builds][Feature:Builds] s2i build with a root user image", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLIWithPodSecurityLevel("s2i-build-root", admissionapi.LevelBaseline)

	g.It("should create a root build and fail without a privileged SCC [apigroup:build.openshift.io]", g.Label("Size:L"), func() {
		g.Skip("TODO: figure out why we aren't properly denying this, also consider whether we still need to deny it")
		Before(oc)
		defer After(oc)

		firstArgString := fmt.Sprintf("%s~https://github.com/sclorg/nodejs-ex", image.LocationFor("quay.io/redhat-developer/test-build-roots2i:1.2"))
		err := oc.Run("new-app").Args(firstArgString, "--name", "nodejsfail").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		err = exutil.WaitForABuild(oc.BuildClient().BuildV1().Builds(oc.Namespace()), "nodejsfail-1", nil, nil, nil)
		o.Expect(err).To(o.HaveOccurred())

		build, err := oc.BuildClient().BuildV1().Builds(oc.Namespace()).Get(context.Background(), "nodejsfail-1", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(build.Status.Phase).To(o.Equal(buildv1.BuildPhaseFailed))
		o.Expect(build.Status.Reason).To(o.BeEquivalentTo("PullBuilderImageFailed" /*s2istatus.ReasonPullBuilderImageFailed*/))
		o.Expect(build.Status.Message).To(o.BeEquivalentTo("Failed to pull builder image." /*s2istatus.ReasonMessagePullBuilderImageFailed*/))

		podname := build.Annotations[buildv1.BuildPodNameAnnotation]
		pod, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).Get(context.Background(), podname, metav1.GetOptions{})
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

	g.It("should create a root build and pass with a privileged SCC [apigroup:build.openshift.io]", g.Label("Size:L"), func() {
		Before(oc)
		defer After(oc)
		g.By("adding builder account to privileged SCC")
		// create a namespace local role and role binding to the privileged SCC;
		// role and role binding will be deleted as part of test namespace clean up
		role := &rbacv1.Role{}
		role.Namespace = oc.Namespace()
		role.Name = "privileged-builder-role"
		role.Rules = []rbacv1.PolicyRule{
			{
				APIGroups: []string{
					"security.openshift.io",
				},
				ResourceNames: []string{
					"privileged",
				},
				Resources: []string{
					"securitycontextconstraints",
				},
				Verbs: []string{
					"use",
				},
			},
		}
		role, err := oc.AdminKubeClient().RbacV1().Roles(oc.Namespace()).Create(context.Background(), role, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		roleBinding := &rbacv1.RoleBinding{}
		roleBinding.Namespace = oc.Namespace()
		roleBinding.Name = "privileged-builder-rolebinding"
		roleBinding.RoleRef = rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     "privileged-builder-role",
		}
		roleBinding.Subjects = []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				APIGroup:  "",
				Name:      "builder",
				Namespace: oc.Namespace(),
			},
		}
		roleBinding, err = oc.AdminKubeClient().RbacV1().RoleBindings(oc.Namespace()).Create(context.Background(), roleBinding, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		firstArgString := fmt.Sprintf("%s~https://github.com/sclorg/nodejs-ex", image.LocationFor("quay.io/redhat-developer/test-build-roots2i:1.2"))
		err = oc.Run("new-build").Args(firstArgString, "--name", "nodejspass").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		err = exutil.WaitForABuild(oc.BuildClient().BuildV1().Builds(oc.Namespace()), "nodejspass-1", nil, nil, nil)
		o.Expect(err).NotTo(o.HaveOccurred())

		build, err := oc.BuildClient().BuildV1().Builds(oc.Namespace()).Get(context.Background(), "nodejspass-1", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		podname := build.Annotations[buildv1.BuildPodNameAnnotation]
		pod, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).Get(context.Background(), podname, metav1.GetOptions{})
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
