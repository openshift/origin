package authorization

import (
	"context"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	psapi "k8s.io/pod-security-admission/api"
	"k8s.io/utils/pointer"

	securityv1 "github.com/openshift/api/security/v1"
	"github.com/openshift/origin/pkg/test/ginkgo/result"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-auth][Feature:SCC][Early]", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithoutNamespace("working-scc-during-install")

	g.It("should not have pod creation failures during install", g.Label("Size:S"), func() {
		kubeClient := oc.AdminKubeClient()

		events, err := kubeClient.CoreV1().Events("").List(context.TODO(), metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		suiteStartTime := exutil.SuiteStartTime()
		var suppressPreTestFailure bool
		if t := exutil.LimitTestsToStartTime(); !t.IsZero() {
			suppressPreTestFailure = true
			if t.After(suiteStartTime) {
				suiteStartTime = t
			}
		}

		var preTestDenialStrings, duringTestDenialStrings []string

		for _, event := range events.Items {
			if !strings.Contains(event.Message, "unable to validate against any security context constraint") {
				continue
			}

			// try with a short summary we can actually read first
			denialString := fmt.Sprintf("%v for %v.%v/%v -n %v happened %d times", event.Message, event.InvolvedObject.Kind, event.InvolvedObject.APIVersion, event.InvolvedObject.Name, event.InvolvedObject.Namespace, event.Count)

			if event.EventTime.Time.Before(suiteStartTime) {
				// SCCs become accessible to serviceaccounts based on RBAC resources.  We could require that every operator
				// apply their RBAC in order with respect to their operands by checking SARs against every kube-apiserver endpoint
				// and ensuring that the "use" for an SCC comes back correctly, but that isn't very useful.
				// We don't want to delay pods for an excessive period of time, so we will catch those pods that take more
				// than five seconds to make it through SCC
				durationPodFailed := event.LastTimestamp.Sub(event.FirstTimestamp.Time)
				if durationPodFailed < 5*time.Second {
					continue
				}
				preTestDenialStrings = append(preTestDenialStrings, denialString)
			} else {
				// Tests are not allowed to not take SCC propagation time into account, and so every during test SCC failure
				// is a hard fail so that we don't allow bad tests to get checked in.
				duringTestDenialStrings = append(duringTestDenialStrings, denialString)
			}
		}

		if numFailingPods := len(preTestDenialStrings); numFailingPods > 0 {
			failMessage := fmt.Sprintf("%d pods failed before test on SCC errors\n%s\n", numFailingPods, strings.Join(preTestDenialStrings, "\n"))
			if suppressPreTestFailure {
				result.Flakef("pre-test environment had disruption and limited this test, suppressing failure: %s", failMessage)
			} else {
				g.Fail(failMessage)
			}
		}
		if numFailingPods := len(duringTestDenialStrings); numFailingPods > 0 {
			failMessage := fmt.Sprintf("%d pods failed during test on SCC errors\n%s\n", numFailingPods, strings.Join(duringTestDenialStrings, "\n"))
			g.Fail(failMessage)
		}
	})
})

var _ = g.Describe("[sig-auth][Feature:PodSecurity][Feature:SCC]", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithPodSecurityLevel("required-scc", psapi.LevelPrivileged)

	g.It("required-scc annotation is being applied to workloads", g.Label("Size:M"), func() {
		sccRole, err := oc.AdminKubeClient().RbacV1().ClusterRoles().Create(context.Background(),
			&rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "required-scc-",
				},
				Rules: []rbacv1.PolicyRule{
					{APIGroups: []string{"security.openshift.io"}, Resources: []string{"securitycontextconstraints"}, Verbs: []string{"use"}},
				},
			}, metav1.CreateOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		_, err = oc.AdminKubeClient().RbacV1().RoleBindings(oc.Namespace()).Create(context.Background(),
			&rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "required-scc-",
					Namespace:    oc.Namespace(),
				},
				RoleRef: rbacv1.RoleRef{
					APIGroup: "rbac.authorization.k8s.io",
					Kind:     "ClusterRole",
					Name:     sccRole.Name,
				},
				Subjects: []rbacv1.Subject{
					{
						Kind:     "User",
						Name:     oc.Username(),
						APIGroup: "rbac.authorization.k8s.io",
					},
				},
			},
			metav1.CreateOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		newPod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "required-scc-testpod",
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:    "sleeper",
						Image:   "fedora:latest",
						Command: []string{"sleep"},
						Args:    []string{"infinity"},
						SecurityContext: &corev1.SecurityContext{
							RunAsNonRoot: pointer.Bool(true),
							Capabilities: &corev1.Capabilities{
								Drop: []corev1.Capability{
									"ALL",
								},
							},
						},
					},
				},
				SecurityContext: &corev1.PodSecurityContext{
					RunAsNonRoot: pointer.Bool(true),
				},
			},
		}

		pod, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).Create(context.Background(), newPod, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		// pod has permissions to use all SCCs and matches the "anyuid" SCC because it specifies RunAsNonRoot=true.
		o.Expect(pod.Annotations[securityv1.ValidatedSCCAnnotation]).To(o.Equal("anyuid"))

		// we pin the SCC to a concrete SCC using the required annotation.
		newPod.Annotations = map[string]string{securityv1.RequiredSCCAnnotation: "restricted-v2"}
		pod, err = oc.KubeClient().CoreV1().Pods(oc.Namespace()).Create(context.Background(), newPod, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(pod.Annotations[securityv1.ValidatedSCCAnnotation]).To(o.Equal("restricted-v2"))
	})

	g.It("SCC admission fails for incorrect/non-existent required-scc annotation", g.Label("Size:M"), func() {
		sccPod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "required-scc-testpod",
				Annotations: map[string]string{
					securityv1.RequiredSCCAnnotation: "non_existent_scc", // Set an annotation to a non-existent SCC.
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:    "sleeper",
						Image:   "fedora:latest",
						Command: []string{"sleep"},
						Args:    []string{"infinity"},
						SecurityContext: &corev1.SecurityContext{
							RunAsNonRoot: pointer.Bool(true),
							Capabilities: &corev1.Capabilities{
								Drop: []corev1.Capability{
									"ALL",
								},
							},
						},
					},
				},
				SecurityContext: &corev1.PodSecurityContext{
					RunAsNonRoot: pointer.Bool(true),
				},
			},
		}

		// Attempt to create the pod and expect an error due to the non-existent SCC.
		_, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).Create(context.Background(), sccPod, metav1.CreateOptions{})
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(err.Error()).To(o.MatchRegexp(`required .*non_existent_scc.* not found`))

		// Set an annotation to an existing SCC but without the required permissions.
		sccPod.Annotations[securityv1.RequiredSCCAnnotation] = "privileged"

		// Attempt to create the pod again and expect an error due to the lack of permission.
		_, err = oc.KubeClient().CoreV1().Pods(oc.Namespace()).Create(context.Background(), sccPod, metav1.CreateOptions{})
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(err.Error()).To(o.ContainSubstring("Forbidden: not usable by user or serviceaccount"))
	})
})
