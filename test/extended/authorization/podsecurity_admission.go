package authorization

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	rest "k8s.io/client-go/rest"
	psapi "k8s.io/pod-security-admission/api"
	"k8s.io/utils/pointer"

	securityv1 "github.com/openshift/api/security/v1"

	exutil "github.com/openshift/origin/test/extended/util"
)

var sleeperContainer = corev1.Container{
	Name:    "sleeper",
	Image:   "fedora:latest",
	Command: []string{"sleep"},
	Args:    []string{"infinity"},
}

var _ = g.Describe("[sig-auth][Feature:PodSecurity]", func() {
	defer g.GinkgoRecover()

	g.Context("with restricted level", func() {
		oc := exutil.NewCLIWithPodSecurityLevel("pod-security", psapi.LevelRestricted)

		g.It("restricted-v2 SCC should mutate empty securityContext to match restricted PSa profile", func() {
			pod, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).Create(context.Background(), &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "psa-testpod",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						sleeperContainer,
					},
				},
			}, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())                                                     // the pod passed admission
			o.Expect(pod.Annotations[securityv1.ValidatedSCCAnnotation]).To(o.Equal("restricted-v2")) // and the mutating SCC is restricted-v2
		})
	})
})

var _ = g.Describe("[sig-auth][Feature:PodSecurity][Feature:SCC]", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithPodSecurityLevel("pod-security-scc-mutation", psapi.LevelRestricted)

	g.It("creating pod controllers", func() {
		ns, err := oc.AdminKubeClient().CoreV1().Namespaces().Get(context.Background(), oc.Namespace(), metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		// we're interested only in warnings here
		ns.Labels[psapi.EnforceLevelLabel] = "privileged"
		ns.Labels[psapi.AuditLevelLabel] = "privileged"

		_, err = oc.AdminKubeClient().CoreV1().Namespaces().Update(context.Background(), ns, metav1.UpdateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		userConfig := rest.CopyConfig(oc.UserConfig())
		recorder := &HeaderRecordingRoundTripper{}
		userConfig.Wrap(recorder.Wrap)

		kubeClient, err := kubernetes.NewForConfig(userConfig)
		o.Expect(err).NotTo(o.HaveOccurred())

		privilegedSA, err := oc.AdminKubeClient().CoreV1().ServiceAccounts(oc.Namespace()).Create(
			context.Background(),
			&corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "privileted-sa-",
				},
			},
			metav1.CreateOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		_, err = oc.AdminKubeClient().RbacV1().RoleBindings(oc.Namespace()).Create(context.Background(),
			&v1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "privileged-scc-",
				},
				RoleRef: v1.RoleRef{
					APIGroup: "rbac.authorization.k8s.io",
					Kind:     "ClusterRole",
					Name:     "system:openshift:scc:privileged",
				},
				Subjects: []v1.Subject{
					{
						Kind:      "ServiceAccount",
						Name:      privilegedSA.Name,
						Namespace: oc.Namespace(),
					},
				},
			},
			metav1.CreateOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		for _, tt := range []struct {
			name                   string
			container              corev1.Container
			saName                 string
			expectWarningSubstring string
		}{
			{
				name:                   "SHOULD throw warnings if no SCC matches the pod template",
				container:              privilegedContainer(sleeperContainer),
				saName:                 "default",
				expectWarningSubstring: "would violate PodSecurity",
			},
			{
				name:                   "SHOULD throw warnings if SCC matches but will not mutate to the expected PSa profile",
				container:              privilegedContainer(sleeperContainer),
				saName:                 privilegedSA.Name,
				expectWarningSubstring: "would violate PodSecurity",
			},
			{
				name:      "SHOULD NOT throw warnings if an SCC CAN mutate their pods to match the effective PSa profile",
				saName:    "default",
				container: sleeperContainer,
			},
		} {
			g.By(tt.name, func() {
				recorder.ClearHeaders() // clear the recorded headers every time

				_, err = kubeClient.AppsV1().Deployments(oc.Namespace()).Create(context.Background(),
					&appsv1.Deployment{
						ObjectMeta: metav1.ObjectMeta{
							GenerateName: "psa-testdeploy",
						},
						Spec: appsv1.DeploymentSpec{
							Selector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"app": "psa-test",
								},
							},
							Template: corev1.PodTemplateSpec{
								ObjectMeta: metav1.ObjectMeta{
									Labels: map[string]string{
										"app": "psa-test",
									},
								},
								Spec: corev1.PodSpec{
									ServiceAccountName: tt.saName,
									Containers: []corev1.Container{
										tt.container,
									},
								},
							},
						},
					}, metav1.CreateOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				headers := recorder.RecordedHeaders()
				var warningFound bool
				var warningHeader string

				expectedSubstring := "PodSecurity"
				if len(tt.expectWarningSubstring) > 0 {
					expectedSubstring = tt.expectWarningSubstring
				}
				for _, h := range headers {
					if h == nil {
						continue
					}
					warningHeader = h.Get("Warning")
					if strings.Contains(warningHeader, expectedSubstring) {
						warningFound = true
						break
					}
				}

				if warningFound != (len(tt.expectWarningSubstring) > 0) {
					g.Fail(fmt.Sprintf("expected warning - %v. Found? %v; recorded headers:\n%v", len(tt.expectWarningSubstring) > 0, warningFound, headers))
				}
			})
		}
	})
})

func privilegedContainer(c corev1.Container) corev1.Container {
	ret := c.DeepCopy()
	if ret.SecurityContext == nil {
		ret.SecurityContext = &corev1.SecurityContext{}
	}
	ret.SecurityContext.Privileged = pointer.Bool(true)
	return *ret

}

type HeaderRecordingRoundTripper struct {
	delegate http.RoundTripper
	headers  []http.Header
}

func (rt *HeaderRecordingRoundTripper) Wrap(delegate http.RoundTripper) http.RoundTripper {
	rt.delegate = delegate
	return rt
}

func (rt *HeaderRecordingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := rt.delegate.RoundTrip(req)
	if resp != nil {
		rt.headers = append(rt.headers, resp.Header.Clone())
	} else {
		rt.headers = append(rt.headers, nil)
	}

	return resp, err
}

func (rt *HeaderRecordingRoundTripper) RecordedHeaders() []http.Header {
	return rt.headers
}

func (rt *HeaderRecordingRoundTripper) ClearHeaders() {
	rt.headers = []http.Header{}
}
