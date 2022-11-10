package authorization

import (
	"context"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/pod-security-admission/api"

	securityv1 "github.com/openshift/api/security/v1"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-auth][Feature:PodSecurity]", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithPodSecurityLevel("pod-security", api.LevelRestricted)

	g.It("restricted-v2 SCC should mutate empty securityContext to match restricted PSa profile", func() {
		pod, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).Create(context.Background(), &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "psa-testpod",
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:    "sleeper",
						Image:   "fedora:latest",
						Command: []string{"sleep"},
						Args:    []string{"infinity"},
					},
				},
			},
		}, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())                                                     // the pod passed admission
		o.Expect(pod.Annotations[securityv1.ValidatedSCCAnnotation]).To(o.Equal("restricted-v2")) // and the mutating SCC is restricted-v2
	})
})
