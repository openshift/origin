package ingress

import (
	"context"
	"reflect"
	"time"

	"github.com/onsi/ginkgo/v2"
	operatorv1 "github.com/openshift/api/operator/v1"
	ingresscontroller "github.com/openshift/cluster-ingress-operator/pkg/operator/controller/ingress"
	exutil "github.com/openshift/origin/test/extended/util"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/upgrades"
)

type UpgradeTest struct {
	oc *exutil.CLI
}

var (
	availableConditionsForIngressControllerWithLoadBalancer = []operatorv1.OperatorCondition{
		{Type: operatorv1.IngressControllerAvailableConditionType, Status: operatorv1.ConditionTrue},
		{Type: operatorv1.LoadBalancerManagedIngressConditionType, Status: operatorv1.ConditionTrue},
		{Type: operatorv1.LoadBalancerReadyIngressConditionType, Status: operatorv1.ConditionTrue},
		{Type: ingresscontroller.IngressControllerAdmittedConditionType, Status: operatorv1.ConditionTrue},
	}
	invalidSourceRangesConditions = []operatorv1.OperatorCondition{
		{Type: operatorv1.OperatorStatusTypeProgressing, Status: operatorv1.ConditionTrue},
		{Type: ingresscontroller.IngressControllerEvaluationConditionsDetectedConditionType, Status: operatorv1.ConditionTrue},
	}
	validSourceRangesConditions = []operatorv1.OperatorCondition{
		{Type: operatorv1.OperatorStatusTypeProgressing, Status: operatorv1.ConditionFalse},
		{Type: ingresscontroller.IngressControllerEvaluationConditionsDetectedConditionType, Status: operatorv1.ConditionFalse},
	}
	icName  = types.NamespacedName{Namespace: DefaultOperatorNamespace, Name: "sourcerangesmigration"}
	svcName = "router-" + icName.Name
)

const (
	DefaultOperatorNamespace = "openshift-ingress-operator"
	DefaultOperandNamespace  = "openshift-ingress"
	allowedSourceRanges      = "0.0.0.0/0"
)

func (t *UpgradeTest) Name() string { return "ingress-source-ranges-migration" }
func (t *UpgradeTest) DisplayName() string {
	return "[sig-network-edge] Verify the migration from LoadBalancerSourceRanges annotation to the new AllowedSourceRanges API"
}

// Setup creates an IngressController that requests an LB,
// and sets the `service.beta.kubernetes.io/load-balancer-source-ranges` annotation
// on the resulting LoadBalancer-type service.
func (t *UpgradeTest) Setup(f *framework.Framework) {
	ginkgo.By("Setting up Ingress LoadBalancerSourceRanges migration test")
	oc := exutil.NewCLIWithFramework(f)
	t.oc = oc

	dnsConfig, err := oc.AdminConfigClient().ConfigV1().DNSes().Get(context.TODO(), "cluster", metav1.GetOptions{})
	framework.ExpectNoError(err)

	domain := icName.Name + "." + dnsConfig.Spec.BaseDomain
	createIngressController(oc, icName, domain)

	// Wait for the load balancer to be ready
	err = waitForIngressControllerCondition(oc, 5*time.Minute, icName, availableConditionsForIngressControllerWithLoadBalancer...)
	framework.ExpectNoError(err)

	lbService, err := getRouterService(oc, 5*time.Minute, svcName)
	framework.ExpectNoError(err)

	// Update load-balancer service with annotation
	lbService.Annotations[corev1.AnnotationLoadBalancerSourceRangesKey] = "127.0.0.0/8"
	_, err = oc.AdminKubeClient().CoreV1().Services(DefaultOperandNamespace).Update(context.Background(), lbService, metav1.UpdateOptions{})
	framework.ExpectNoError(err)
}

// Test periodically checks the service to verify that nothing updates the annotation
// or the LoadBalancerSourceRanges field on the service during the upgrade.
// Then, it verifies again when the upgrade is done that nothing has updated the annotation
// or LoadBalancerSourceRanges, and verifies that the operator has set Progressing=True and
// EvaluationConditionsDetected=True. It sets then AllowedSourceRanges, and verifies if LoadBalancerSourceRanges
// is set accordingly, and the annotation is removed. It finally verifies that the operator
// has set Progressing=False and EvaluationConditionsDetected=False.
func (t *UpgradeTest) Test(_ *framework.Framework, done <-chan struct{}, _ upgrades.UpgradeType) {
	ginkgo.By("Verifying nothing has updated the annotation or LoadBalancerSourceRanges field during upgrade")
UpgradeInProgress:
	for {
		select {
		case <-done:
			break UpgradeInProgress
		case <-time.After(30 * time.Second):
			err := waitForIngressControllerCondition(t.oc, 30*time.Second, icName, invalidSourceRangesConditions...)
			if err != nil {
				framework.Logf("Waiting for upgrade to finish")
				t.checkIfAnnotationOrFieldUpdated()
			} else {
				//This means ingress controller is already upgraded.
				// Verify again when the upgrade is done that nothing has updated the annotation or LoadBalancerSourceRanges
				t.checkIfAnnotationOrFieldUpdated()

				ic, err := t.oc.AdminOperatorClient().OperatorV1().IngressControllers(icName.Namespace).Get(context.Background(), icName.Name, metav1.GetOptions{})
				framework.ExpectNoError(err)

				// Set AllowedSourceRanges
				ic.Spec.EndpointPublishingStrategy.LoadBalancer.AllowedSourceRanges = []operatorv1.CIDR{allowedSourceRanges}
				_, err = t.oc.AdminOperatorClient().OperatorV1().IngressControllers(icName.Namespace).Update(context.Background(), ic, metav1.UpdateOptions{})
				framework.ExpectNoError(err)

				// Wait for the Progressing=False and EvaluationConditionsDetected=False conditions
				err = waitForIngressControllerCondition(t.oc, 5*time.Minute, icName, validSourceRangesConditions...)
				framework.ExpectNoError(err)
			}
		}
	}

	// Verify LoadBalancerSourceRanges is set and the annotation is removed
	lbService, err := getRouterService(t.oc, 5*time.Minute, svcName)
	framework.ExpectNoError(err)

	if err := wait.PollImmediate(time.Second, 5*time.Minute, func() (bool, error) {
		if len(lbService.Spec.LoadBalancerSourceRanges) == 1 && lbService.Spec.LoadBalancerSourceRanges[0] == allowedSourceRanges {
			if a, exists := lbService.Annotations[corev1.AnnotationLoadBalancerSourceRangesKey]; !exists || (exists && len(a) == 0) {
				return true, nil
			}
		}

		lbService, err = t.oc.AdminKubeClient().CoreV1().Services(DefaultOperandNamespace).Get(context.Background(), svcName, metav1.GetOptions{})
		if err != nil {
			framework.Logf("Get router service %s failed: %v, retrying...", svcName, err)
		}

		return false, nil
	}); err != nil {
		framework.Failf("expected LoadBalancerSourceRanges to be equal to AllowedSourceRanges and the annotation to be cleared")
	}

	// Verify again the Progressing=False and EvaluationConditionsDetected=False conditions
	err = waitForIngressControllerCondition(t.oc, 5*time.Minute, icName, validSourceRangesConditions...)
	framework.ExpectNoError(err)
}

// Teardown cleans up any objects that are created that
// aren't already cleaned up by the framework.
func (t *UpgradeTest) Teardown(_ *framework.Framework) {
	// rely on the namespace deletion to clean up everything
}

func (t *UpgradeTest) checkIfAnnotationOrFieldUpdated() {
	lbService, err := getRouterService(t.oc, 5*time.Minute, svcName)
	framework.ExpectNoError(err)

	if len(lbService.Spec.LoadBalancerSourceRanges) > 0 {
		framework.Failf("expected LoadBalancerSourceRanges not to change during upgrade")
	}

	a, ok := lbService.Annotations[corev1.AnnotationLoadBalancerSourceRangesKey]
	if !ok || a != "127.0.0.0/8" {
		framework.Failf("expected annotation not to change during upgrade")
	}
}

func createIngressController(oc *exutil.CLI, name types.NamespacedName, domain string) {
	repl := int32(1)
	ic := &operatorv1.IngressController{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: name.Namespace,
			Name:      name.Name,
		},
		Spec: operatorv1.IngressControllerSpec{
			Domain:   domain,
			Replicas: &repl,
			EndpointPublishingStrategy: &operatorv1.EndpointPublishingStrategy{
				Type: operatorv1.LoadBalancerServiceStrategyType,
			},
		},
	}

	_, err := oc.AdminOperatorClient().OperatorV1().IngressControllers(name.Namespace).Create(context.Background(), ic, metav1.CreateOptions{})
	framework.ExpectNoError(err)
}

func waitForIngressControllerCondition(oc *exutil.CLI, timeout time.Duration, name types.NamespacedName, conditions ...operatorv1.OperatorCondition) error {
	return wait.PollImmediate(3*time.Second, timeout, func() (bool, error) {
		ic, err := oc.AdminOperatorClient().OperatorV1().IngressControllers(name.Namespace).Get(context.Background(), name.Name, metav1.GetOptions{})
		if err != nil {
			framework.Logf("failed to get ingresscontroller %s/%s: %v, retrying...", name.Namespace, name.Name, err)
			return false, nil
		}
		expected := operatorConditionMap(conditions...)
		current := operatorConditionMap(ic.Status.Conditions...)
		met := conditionsMatchExpected(expected, current)
		if !met {
			framework.Logf("ingresscontroller %s/%s conditions not met; wanted %+v, got %+v, retrying...", name.Namespace, name.Name, expected, current)
		}
		return met, nil
	})
}

func operatorConditionMap(conditions ...operatorv1.OperatorCondition) map[string]string {
	conds := map[string]string{}
	for _, cond := range conditions {
		conds[cond.Type] = string(cond.Status)
	}
	return conds
}

func conditionsMatchExpected(expected, actual map[string]string) bool {
	filtered := map[string]string{}
	for k := range actual {
		if _, c := expected[k]; c {
			filtered[k] = actual[k]
		}
	}
	return reflect.DeepEqual(expected, filtered)
}

func getRouterService(oc *exutil.CLI, timeout time.Duration, name string) (*corev1.Service, error) {
	var svc *corev1.Service

	if err := wait.PollImmediate(time.Second, timeout, func() (bool, error) {
		o, err := oc.AdminKubeClient().CoreV1().Services(DefaultOperandNamespace).Get(context.Background(), name, metav1.GetOptions{})
		if err != nil {
			framework.Logf("Get router service %s failed: %v, retrying...", name, err)
			return false, nil
		}
		svc = o
		return true, nil
	}); err != nil {
		return nil, err
	}

	return svc, nil
}
