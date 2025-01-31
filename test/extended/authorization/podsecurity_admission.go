package authorization

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	v1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/storage/names"
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

var _ = g.Describe("[sig-auth][Feature:PodSecurity][Feature:LabelSyncer][Feature:ForceHistoricalOwnership]", func() {
	defer g.GinkgoRecover()

	ctx := context.Background()
	oc := exutil.NewCLIWithoutNamespace("pod-security-managed-labels-historical")

	cpc := "cluster-policy-controller"
	psaLabelSyncer := "pod-security-admission-label-synchronization-controller"
	createMgr := cpc
	updateMgr := names.SimpleNameGenerator.GenerateName("test-client-")
	labelSyncLabel := "security.openshift.io/scc.podSecurityLabelSync"
	psaLabels := []string{
		psapi.AuditLevelLabel,
		psapi.AuditVersionLabel,
		psapi.WarnLevelLabel,
		psapi.WarnVersionLabel,
	}

	g.It("forcing PSa label historical ownership", func() {
		for _, testCase := range []struct {
			name                                   string
			podSecurityLabelSync                   string
			expectedManagerForPSaLabelsAfterCreate string
			expectedManagerForPSaLabelsAfterUpdate string
		}{
			{"labelsyncer-off", "false", cpc, updateMgr},
			{"labelsyncer-on", "true", psaLabelSyncer, psaLabelSyncer},
			{"labelsyncer-undef", "", psaLabelSyncer, updateMgr},

			// openshift- prefixed namespaces; special case when podSecurityLabelSync label is undefined
			{"openshift-e2e-labelsyncer-off", "false", cpc, updateMgr},
			{"openshift-e2e-labelsyncer-on", "true", psaLabelSyncer, psaLabelSyncer},
			{"openshift-e2e-labelsyncer-undef", "", cpc, updateMgr},
		} {
			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: names.SimpleNameGenerator.GenerateName(testCase.name + "-"),
					Labels: map[string]string{
						psapi.AuditLevelLabel:   "restricted",
						psapi.AuditVersionLabel: "v1.24",
						psapi.WarnLevelLabel:    "restricted",
						psapi.WarnVersionLabel:  "v1.24",
					},
				},
			}

			if testCase.podSecurityLabelSync != "" {
				ns.ObjectMeta.Labels[labelSyncLabel] = testCase.podSecurityLabelSync
			}

			ns, err := oc.AdminKubeClient().CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{FieldManager: createMgr})
			o.Expect(err).NotTo(o.HaveOccurred())
			ns, err = waitAndGetNamespace(ctx, oc, ns)
			oc.AddResourceToDelete(corev1.SchemeGroupVersion.WithResource("namespaces"), ns)

			managers, err := managerPerLabel(ns.ObjectMeta.ManagedFields)
			o.Expect(err).NotTo(o.HaveOccurred())

			if ns.ObjectMeta.Labels[labelSyncLabel] != "" {
				o.Expect(managers[labelSyncLabel]).To(o.Equal(cpc))
			}

			for _, label := range psaLabels {
				o.Expect(managers[label]).To(o.Equal(testCase.expectedManagerForPSaLabelsAfterCreate))
			}

			ns.ObjectMeta.Labels[psapi.AuditLevelLabel] = "privileged"
			ns.ObjectMeta.Labels[psapi.AuditVersionLabel] = "v1.25"
			ns.ObjectMeta.Labels[psapi.WarnLevelLabel] = "privileged"
			ns.ObjectMeta.Labels[psapi.WarnVersionLabel] = "v1.25"
			ns, err = oc.AdminKubeClient().CoreV1().Namespaces().Update(ctx, ns, metav1.UpdateOptions{FieldManager: updateMgr})
			o.Expect(err).NotTo(o.HaveOccurred())
			ns, err = waitAndGetNamespace(ctx, oc, ns)
			o.Expect(err).NotTo(o.HaveOccurred())

			managers, err = managerPerLabel(ns.ObjectMeta.ManagedFields)
			o.Expect(err).NotTo(o.HaveOccurred())

			if ns.ObjectMeta.Labels[labelSyncLabel] != "" {
				o.Expect(managers[labelSyncLabel]).To(o.Equal(cpc))
			}

			for _, label := range psaLabels {
				o.Expect(managers[label]).To(o.Equal(testCase.expectedManagerForPSaLabelsAfterUpdate))
			}
		}
	})
})

var _ = g.Describe("[sig-auth][Feature:PodSecurity][Feature:LabelSyncer]", func() {
	defer g.GinkgoRecover()

	ctx := context.Background()
	oc := exutil.NewCLIWithoutNamespace("pod-security-managed-labels")

	sccToPSa := map[string]string{
		"privileged":    "privileged",
		"restricted":    "baseline",
		"restricted-v2": "restricted",
	}

	g.It("updating PSa labels", func() {
		for _, testCase := range []struct {
			name                          string
			podSecurityLabelSync          string
			expectedPSaLevelAfterCreate   string
			expectedPSaVersionAfterCreate string
			userPSaLevel                  string
			userPSaVersion                string
			expectedPSaLevelAfterUpdate   string
			expectedPSaVersionAfterUpdate string
			sccChangesPSa                 bool
		}{
			{"restricted-labelsyncer-off", "false", "", "", "restricted", "v1.25", "restricted", "v1.25", false},
			{"baseline-labelsyncer-off", "false", "", "", "baseline", "v1.25", "baseline", "v1.25", false},
			{"privileged-labelsyncer-off", "false", "", "", "privileged", "v1.25", "privileged", "v1.25", false},

			{"restricted-labelsyncer-on", "true", "restricted", "v1.24", "restricted", "v1.25", "restricted", "v1.24", true},
			{"baseline-labelsyncer-on", "true", "restricted", "v1.24", "baseline", "v1.25", "restricted", "v1.24", true},
			{"privileged-labelsyncer-on", "true", "restricted", "v1.24", "privileged", "v1.25", "restricted", "v1.24", true},

			{"restricted-labelsyncer-undef", "", "restricted", "v1.24", "restricted", "v1.25", "restricted", "v1.25", true}, // sccChangesPSa = true because the field hasn't been taken over as the update did not change the label
			{"baseline-labelsyncer-undef", "", "restricted", "v1.24", "baseline", "v1.25", "baseline", "v1.25", false},
			{"privileged-labelsyncer-undef", "", "restricted", "v1.24", "privileged", "v1.25", "privileged", "v1.25", false},

			// openshift- prefixed namespaces; special case when podSecurityLabelSync label is undefined
			{"openshift-e2e-restricted-labelsyncer-off", "false", "", "", "restricted", "v1.25", "restricted", "v1.25", false},
			{"openshift-e2e-baseline-labelsyncer-off", "false", "", "", "baseline", "v1.25", "baseline", "v1.25", false},
			{"openshift-e2e-privileged-labelsyncer-off", "false", "", "", "privileged", "v1.25", "privileged", "v1.25", false},

			{"openshift-e2e-restricted-labelsyncer-on", "true", "restricted", "v1.24", "restricted", "v1.25", "restricted", "v1.24", true},
			{"openshift-e2e-baseline-labelsyncer-on", "true", "restricted", "v1.24", "baseline", "v1.25", "restricted", "v1.24", true},
			{"openshift-e2e-privileged-labelsyncer-on", "true", "restricted", "v1.24", "privileged", "v1.25", "restricted", "v1.24", true},

			{"openshift-e2e-restricted-labelsyncer-undef", "", "", "", "restricted", "v1.25", "restricted", "v1.25", false},
			{"openshift-e2e-baseline-labelsyncer-undef", "", "", "", "baseline", "v1.25", "baseline", "v1.25", false},
			{"openshift-e2e-privileged-labelsyncer-undef", "", "", "", "privileged", "v1.25", "privileged", "v1.25", false},
		} {
			g.By(fmt.Sprintf("test updating PSa labels for ns: %s", testCase.name), func() {
				ns := &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name:   names.SimpleNameGenerator.GenerateName(testCase.name + "-"),
						Labels: map[string]string{},
					},
				}

				if testCase.podSecurityLabelSync != "" {
					ns.ObjectMeta.Labels["security.openshift.io/scc.podSecurityLabelSync"] = testCase.podSecurityLabelSync
				}

				// create the test namespace without any PSa labels; podSecurityLabelSync label according to each test case
				ns, err := oc.AdminKubeClient().CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				ns, err = waitAndGetNamespace(ctx, oc, ns)
				o.Expect(err).NotTo(o.HaveOccurred())
				oc.AddResourceToDelete(corev1.SchemeGroupVersion.WithResource("namespaces"), ns)

				// assert expectations after creation depending on the podSecurityLabelSync label
				o.Expect(ns.ObjectMeta.Labels[psapi.AuditLevelLabel]).To(o.Equal(testCase.expectedPSaLevelAfterCreate))
				o.Expect(ns.ObjectMeta.Labels[psapi.AuditVersionLabel]).To(o.Equal(testCase.expectedPSaVersionAfterCreate))
				o.Expect(ns.ObjectMeta.Labels[psapi.WarnLevelLabel]).To(o.Equal(testCase.expectedPSaLevelAfterCreate))
				o.Expect(ns.ObjectMeta.Labels[psapi.WarnVersionLabel]).To(o.Equal(testCase.expectedPSaVersionAfterCreate))

				// test overwriting the PSa labels
				ns.ObjectMeta.Labels[psapi.AuditLevelLabel] = testCase.userPSaLevel
				ns.ObjectMeta.Labels[psapi.AuditVersionLabel] = testCase.userPSaVersion
				ns.ObjectMeta.Labels[psapi.WarnLevelLabel] = testCase.userPSaLevel
				ns.ObjectMeta.Labels[psapi.WarnVersionLabel] = testCase.userPSaVersion
				ns, err = oc.AdminKubeClient().CoreV1().Namespaces().Update(ctx, ns, metav1.UpdateOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				ns, err = waitAndGetNamespace(ctx, oc, ns)
				o.Expect(err).NotTo(o.HaveOccurred())

				// assert expectations after the update based on the podSecurityLabelSync label
				o.Expect(ns.ObjectMeta.Labels[psapi.AuditLevelLabel]).To(o.Equal(testCase.expectedPSaLevelAfterUpdate))
				o.Expect(ns.ObjectMeta.Labels[psapi.AuditVersionLabel]).To(o.Equal(testCase.expectedPSaVersionAfterUpdate))
				o.Expect(ns.ObjectMeta.Labels[psapi.WarnLevelLabel]).To(o.Equal(testCase.expectedPSaLevelAfterUpdate))
				o.Expect(ns.ObjectMeta.Labels[psapi.WarnVersionLabel]).To(o.Equal(testCase.expectedPSaVersionAfterUpdate))

				// add a role to a SA
				role, err := bindRoleToSA(ctx, oc, ns.Name)
				o.Expect(err).NotTo(o.HaveOccurred())

				// set some SCCs on the role and test PSa label outcome
				for scc, psaLabel := range sccToPSa {
					err = setSCCPermissions(ctx, oc, role, scc)
					o.Expect(err).NotTo(o.HaveOccurred())

					ns, err = waitAndGetNamespace(ctx, oc, ns)
					o.Expect(err).NotTo(o.HaveOccurred())

					expectedLevelLabel := testCase.expectedPSaLevelAfterUpdate
					if testCase.sccChangesPSa {
						// SCC assignment yields PSa label change by the labelsyncer
						expectedLevelLabel = psaLabel
					}
					o.Expect(ns.ObjectMeta.Labels[psapi.AuditLevelLabel]).To(o.Equal(expectedLevelLabel))
					o.Expect(ns.ObjectMeta.Labels[psapi.WarnLevelLabel]).To(o.Equal(expectedLevelLabel))
				}
			})
		}
	})
})

// creates a service account, a role & rolebinding for the test and binds it to the SA
func bindRoleToSA(ctx context.Context, oc *exutil.CLI, nsName string) (*v1.Role, error) {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{Name: "sa-" + nsName, Namespace: nsName},
	}

	role := &v1.Role{
		ObjectMeta: metav1.ObjectMeta{Name: "role-" + nsName, Namespace: nsName},
	}

	roleBinding := &v1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "rb-" + nsName, Namespace: nsName},
		Subjects: []v1.Subject{
			{Kind: v1.ServiceAccountKind, Name: sa.ObjectMeta.Name},
		},
		RoleRef: v1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     role.ObjectMeta.Name,
		},
	}

	_, err := oc.AdminKubeClient().CoreV1().ServiceAccounts(nsName).Create(ctx, sa, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	oc.AddResourceToDelete(corev1.SchemeGroupVersion.WithResource("serviceaccounts"), sa)

	_, err = oc.AdminKubeClient().RbacV1().Roles(nsName).Create(ctx, role, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	oc.AddResourceToDelete(rbacv1.SchemeGroupVersion.WithResource("roles"), role)

	_, err = oc.AdminKubeClient().RbacV1().RoleBindings(nsName).Create(ctx, roleBinding, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	oc.AddResourceToDelete(rbacv1.SchemeGroupVersion.WithResource("rolebindings"), roleBinding)

	return role, nil
}

// updates a role to use a specific SCC
func setSCCPermissions(ctx context.Context, oc *exutil.CLI, role *v1.Role, scc string) error {
	role.Rules = []v1.PolicyRule{
		{
			APIGroups:     []string{"security.openshift.io"},
			ResourceNames: []string{scc},
			Resources:     []string{"securitycontextconstraints"},
			Verbs:         []string{"use"},
		},
	}

	_, err := oc.AdminKubeClient().RbacV1().Roles(role.ObjectMeta.Namespace).Update(ctx, role, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	return nil
}

// waitAndGetNamespace waits for the labelsyncer to sync the namespace; it polls until it detects a change in the
// namespace object, or until a timeout is reached
func waitAndGetNamespace(ctx context.Context, oc *exutil.CLI, ns *corev1.Namespace) (*corev1.Namespace, error) {
	var updatedNS *corev1.Namespace
	err := wait.PollUntilContextTimeout(ctx, 1*time.Second, 30*time.Second, true, func(ctx context.Context) (bool, error) {
		var getErr error
		updatedNS, getErr = oc.AdminKubeClient().CoreV1().Namespaces().Get(ctx, ns.Name, metav1.GetOptions{})
		if getErr != nil {
			return false, getErr
		}

		return !reflect.DeepEqual(ns, updatedNS), nil
	})

	// no change detected within the timeout; do not return any error
	if errors.Is(err, context.DeadlineExceeded) {
		return updatedNS, nil
	}

	return updatedNS, err
}

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

func managerPerLabel(managedFields []metav1.ManagedFieldsEntry) (map[string]string, error) {
	result := map[string]string{}
	for _, field := range managedFields {
		labels, err := managedLabels(field)
		if err != nil {
			return nil, err
		}

		for _, label := range labels.UnsortedList() {
			result[label] = field.Manager
		}
	}

	return result, nil
}

func managedLabels(fieldsEntry metav1.ManagedFieldsEntry) (sets.Set[string], error) {
	managedUnstructured := map[string]interface{}{}
	err := json.Unmarshal(fieldsEntry.FieldsV1.Raw, &managedUnstructured)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal managed fields: %w", err)
	}

	labels, found, err := unstructured.NestedMap(managedUnstructured, "f:metadata", "f:labels")
	if err != nil {
		return nil, fmt.Errorf("failed to get labels from the managed fields: %w", err)
	}

	ret := sets.New[string]()
	if !found {
		return ret, nil
	}

	for l := range labels {
		ret.Insert(strings.Replace(l, "f:", "", 1))
	}

	return ret, nil
}
