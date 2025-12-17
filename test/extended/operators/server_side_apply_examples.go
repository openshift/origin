package operators

import (
	"context"

	"github.com/davecgh/go-spew/spew"
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	configv1 "github.com/openshift/api/config/v1"
	applyconfigv1 "github.com/openshift/client-go/config/applyconfigurations/config/v1"
	exutil "github.com/openshift/origin/test/extended/util"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	applymetav1 "k8s.io/client-go/applyconfigurations/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	imageutils "k8s.io/kubernetes/test/utils/image"
	admissionapi "k8s.io/pod-security-admission/api"
)

var _ = g.Describe("[sig-apimachinery]", func() {

	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithPodSecurityLevel("server-side-apply-examples", admissionapi.LevelPrivileged)
	fieldManager := metav1.ApplyOptions{
		FieldManager: "e2e=test",
	}

	g.Describe("server-side-apply should function properly", func() {
		g.It("should clear fields when they are no longer being applied on CRDs", g.Label("Size:M"), func() {
			ctx := context.Background()
			isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
			o.Expect(err).NotTo(o.HaveOccurred())
			if isMicroShift {
				g.Skip("microshift lacks the API")
			}

			_, err = oc.AdminConfigClient().ConfigV1().ClusterOperators().Create(ctx, &configv1.ClusterOperator{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-instance",
				},
			}, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			defer oc.AdminConfigClient().ConfigV1().ClusterOperators().Delete(ctx, "test-instance", metav1.DeleteOptions{})

			addFirstCondition := applyconfigv1.ClusterOperator("test-instance").
				WithStatus(applyconfigv1.ClusterOperatorStatus().
					WithConditions(applyconfigv1.ClusterOperatorStatusCondition().
						WithType("FirstType").
						WithStatus(configv1.ConditionTrue).
						WithReason("Dummy").
						WithMessage("No Value").
						WithLastTransitionTime(metav1.Now()),
					),
				)
			_, err = oc.AdminConfigClient().ConfigV1().ClusterOperators().ApplyStatus(ctx, addFirstCondition, fieldManager)
			o.Expect(err).NotTo(o.HaveOccurred())

			currInstance, err := oc.AdminConfigClient().ConfigV1().ClusterOperators().Get(ctx, "test-instance", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			if !containsCondition(currInstance.Status.Conditions, "FirstType") {
				framework.Logf("got conditions: %v", spew.Sdump(currInstance.Status.Conditions))
				g.Fail("missing FirstType condition")
			}

			addJustSecondCondition := applyconfigv1.ClusterOperator("test-instance").
				WithStatus(applyconfigv1.ClusterOperatorStatus().
					WithConditions(applyconfigv1.ClusterOperatorStatusCondition().
						WithType("SecondType").
						WithStatus(configv1.ConditionTrue).
						WithReason("Dummy").
						WithMessage("No Value").
						WithLastTransitionTime(metav1.Now()),
					),
				)
			_, err = oc.AdminConfigClient().ConfigV1().ClusterOperators().ApplyStatus(ctx, addJustSecondCondition, fieldManager)
			o.Expect(err).NotTo(o.HaveOccurred())

			currInstance, err = oc.AdminConfigClient().ConfigV1().ClusterOperators().Get(ctx, "test-instance", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			if !containsCondition(currInstance.Status.Conditions, "SecondType") {
				g.Fail("missing SecondType condition")
			}
			if containsCondition(currInstance.Status.Conditions, "FirstType") {
				g.Fail("has FirstType condition unexpectedly")
			}
		})

		g.It("should clear fields when they are no longer being applied in FeatureGates [apigroup:config.openshift.io]", g.Label("Size:M"), func() {
			ctx := context.Background()
			isSelfManagedHA, err := exutil.IsSelfManagedHA(ctx, oc.AdminConfigClient())
			o.Expect(err).NotTo(o.HaveOccurred())
			isSingleNode, err := exutil.IsSingleNode(ctx, oc.AdminConfigClient())
			o.Expect(err).NotTo(o.HaveOccurred())
			if !isSelfManagedHA && !isSingleNode {
				g.Skip("only SelfManagedHA and SingleNode have mutable FeatureGates")
			}

			condTrue := metav1.ConditionTrue
			now := metav1.Now()
			type1 := "FirstType"
			type2 := "SecondType"
			dummyReason := "Dummy"
			dummyMsg := "No Value"

			addFirstCondition := applyconfigv1.FeatureGate("cluster").
				WithStatus(applyconfigv1.FeatureGateStatus().
					WithConditions(
						&applymetav1.ConditionApplyConfiguration{
							Type:               &type1,
							Status:             &condTrue,
							LastTransitionTime: &now,
							Reason:             &dummyReason,
							Message:            &dummyMsg,
						},
					),
				)
			_, err = oc.AdminConfigClient().ConfigV1().FeatureGates().ApplyStatus(ctx, addFirstCondition, fieldManager)
			o.Expect(err).NotTo(o.HaveOccurred())

			currInstance, err := oc.AdminConfigClient().ConfigV1().FeatureGates().Get(ctx, "cluster", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			if !containsMetaCondition(currInstance.Status.Conditions, "FirstType") {
				framework.Logf("got conditions: %v", spew.Sdump(currInstance.Status.Conditions))
				g.Fail("missing FirstType condition")
			}

			addJustSecondCondition := applyconfigv1.FeatureGate("cluster").
				WithStatus(applyconfigv1.FeatureGateStatus().
					WithConditions(
						&applymetav1.ConditionApplyConfiguration{
							Type:               &type2,
							Status:             &condTrue,
							LastTransitionTime: &now,
							Reason:             &dummyReason,
							Message:            &dummyMsg,
						},
					),
				)
			_, err = oc.AdminConfigClient().ConfigV1().FeatureGates().ApplyStatus(ctx, addJustSecondCondition, fieldManager)
			o.Expect(err).NotTo(o.HaveOccurred())

			currInstance, err = oc.AdminConfigClient().ConfigV1().FeatureGates().Get(ctx, "cluster", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			if !containsMetaCondition(currInstance.Status.Conditions, "SecondType") {
				g.Fail("missing SecondType condition")
			}
			if containsMetaCondition(currInstance.Status.Conditions, "FirstType") {
				g.Fail("has FirstType condition unexpectedly")
			}
		})

		g.It("should clear fields when they are no longer being applied in built-in APIs", g.Label("Size:M"), func() {
			ctx := context.Background()

			_, err := oc.AdminKubeClient().CoreV1().Pods(oc.Namespace()).Create(ctx, pausePod("test-instance"), metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			defer oc.AdminKubeClient().CoreV1().Pods(oc.Namespace()).Delete(ctx, "test-instance", metav1.DeleteOptions{})

			addFirstCondition := applycorev1.Pod("test-instance", oc.Namespace()).
				WithStatus(applycorev1.PodStatus().
					WithConditions(applycorev1.PodCondition().
						WithType("FirstType").
						WithStatus(corev1.ConditionTrue).
						WithReason("Dummy").
						WithMessage("No Value").
						WithLastTransitionTime(metav1.Now()),
					),
				)
			_, err = oc.AdminKubeClient().CoreV1().Pods(oc.Namespace()).ApplyStatus(ctx, addFirstCondition, fieldManager)
			o.Expect(err).NotTo(o.HaveOccurred())

			currInstance, err := oc.AdminKubeClient().CoreV1().Pods(oc.Namespace()).Get(ctx, "test-instance", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			if !containsPodCondition(currInstance.Status.Conditions, "FirstType") {
				framework.Logf("got conditions: %v", spew.Sdump(currInstance.Status.Conditions))
				g.Fail("missing FirstType condition")
			}

			addJustSecondCondition := applycorev1.Pod("test-instance", oc.Namespace()).
				WithStatus(applycorev1.PodStatus().
					WithConditions(applycorev1.PodCondition().
						WithType("SecondType").
						WithStatus(corev1.ConditionTrue).
						WithReason("Dummy").
						WithMessage("No Value").
						WithLastTransitionTime(metav1.Now()),
					),
				)
			_, err = oc.AdminKubeClient().CoreV1().Pods(oc.Namespace()).ApplyStatus(ctx, addJustSecondCondition, fieldManager)
			o.Expect(err).NotTo(o.HaveOccurred())

			currInstance, err = oc.AdminKubeClient().CoreV1().Pods(oc.Namespace()).Get(ctx, "test-instance", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			if !containsPodCondition(currInstance.Status.Conditions, "SecondType") {
				g.Fail("missing SecondType condition")
			}
			if containsPodCondition(currInstance.Status.Conditions, "FirstType") {
				g.Fail("has FirstType condition unexpectedly")
			}
		})
	})
})

func containsCondition(podConditions []configv1.ClusterOperatorStatusCondition, name string) bool {
	for _, curr := range podConditions {
		if string(curr.Type) == name {
			return true
		}
	}
	return false
}

func containsMetaCondition(podConditions []metav1.Condition, name string) bool {
	for _, curr := range podConditions {
		if string(curr.Type) == name {
			return true
		}
	}
	return false
}

func containsPodCondition(podConditions []corev1.PodCondition, name string) bool {
	for _, curr := range podConditions {
		if string(curr.Type) == name {
			return true
		}
	}
	return false
}

func pausePod(name string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: corev1.PodSpec{
			SecurityContext: e2epod.GetRestrictedPodSecurityContext(),
			Containers: []corev1.Container{
				{
					Name:            "pause-container",
					Image:           imageutils.GetPauseImageName(),
					SecurityContext: e2epod.GetRestrictedContainerSecurityContext(),
				},
			},
		},
	}

}
