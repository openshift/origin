package operators

import (
	"context"
	"fmt"
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	operatorv1 "github.com/openshift/api/operator/v1"
	applyoperatorv1 "github.com/openshift/client-go/operator/applyconfigurations/operator/v1"
	exutil "github.com/openshift/origin/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	admissionapi "k8s.io/pod-security-admission/api"
	"math/rand"
)

var _ = g.Describe("[sig-apimachinery]", func() {

	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithPodSecurityLevel("server-side-apply-zero-diff", admissionapi.LevelPrivileged)

	g.Describe("server-side-apply zero diff detection", func() {
		g.It("should not update when the existing values have not changed", func() {
			ctx := context.Background()
			isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
			o.Expect(err).NotTo(o.HaveOccurred())
			if isMicroShift {
				g.Skip("microshift lacks the API")
			}

			instanceName := fmt.Sprintf("test-instance-%d", rand.Int31())
			gvr := operatorv1.GroupVersion.WithResource("openshiftapiservers")
			client := oc.AdminDynamicClient().Resource(gvr)
			dynamicOperatorClient := &dynamicOperatorClient{
				gvk:                operatorv1.GroupVersion.WithKind("OpenShiftAPIServer"),
				configName:         instanceName,
				client:             client,
				extractApplySpec:   convertOperatorSpecToStaticPodOperatorSpec(extractOperatorSpec),
				extractApplyStatus: convertOperatorStatusToStaticPodOperatorStatus(extractOperatorStatus),
			}

			creatingApply := applyoperatorv1.OperatorSpec().WithLogLevel(operatorv1.Debug)
			err = dynamicOperatorClient.ApplyOperatorSpec(ctx, "creator", creatingApply)
			o.Expect(err).NotTo(o.HaveOccurred())
			defer client.Delete(ctx, "test-instance", metav1.DeleteOptions{})

			firstConditionInitial := applyoperatorv1.OperatorStatus().
				WithConditions(applyoperatorv1.OperatorCondition().
					WithType("First").
					WithStatus(operatorv1.ConditionTrue).
					WithReason("Error").
					WithMessage("Whatever"))
			action, err := dynamicOperatorClient.ApplyOperatorStatus(ctx, "first-condition-setter", firstConditionInitial)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(action).To(o.Equal("modification done"))

			firstConditionIdenticalToInitial := applyoperatorv1.OperatorStatus().
				WithConditions(applyoperatorv1.OperatorCondition().
					WithType("First").
					WithStatus(operatorv1.ConditionTrue).
					WithReason("Error").
					WithMessage("Whatever"))
			action, err = dynamicOperatorClient.ApplyOperatorStatus(ctx, "first-condition-setter", firstConditionIdenticalToInitial)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(action).To(o.Equal("nothing to apply"))
		})

	})
})
