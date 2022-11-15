package shard

import (
	"context"
	"reflect"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	utilpointer "k8s.io/utils/pointer"

	operatorv1 "github.com/openshift/api/operator/v1"
	exutil "github.com/openshift/origin/test/extended/util"
)

type Config struct {
	// Domain is the domain for the ingresscontroller to host
	Domain string

	// Type is the matchSelector
	Type string
}

var ingressControllerNonDefaultAvailableConditions = []operatorv1.OperatorCondition{
	{Type: operatorv1.IngressControllerAvailableConditionType, Status: operatorv1.ConditionTrue},
	{Type: operatorv1.LoadBalancerManagedIngressConditionType, Status: operatorv1.ConditionTrue},
	{Type: operatorv1.LoadBalancerReadyIngressConditionType, Status: operatorv1.ConditionTrue},
	{Type: "Admitted", Status: operatorv1.ConditionTrue},
}

func DeployNewRouterShard(oc *exutil.CLI, timeout time.Duration, cfg Config) (*operatorv1.IngressController, error) {
	ingressCtrl := &operatorv1.IngressController{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cfg.Type,
			Namespace: "openshift-ingress-operator",
			Annotations: map[string]string{
				"ingress.operator.openshift.io/default-enable-http2": "true",
			},
		},
		Spec: operatorv1.IngressControllerSpec{
			Replicas: utilpointer.Int32(1),
			Domain:   cfg.Domain,
			EndpointPublishingStrategy: &operatorv1.EndpointPublishingStrategy{
				Type: operatorv1.LoadBalancerServiceStrategyType,
			},
			NodePlacement: &operatorv1.NodePlacement{
				NodeSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"node-role.kubernetes.io/worker": "",
					},
				},
			},
			NamespaceSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"type": cfg.Type,
				},
			},
		},
	}
	_, err := oc.AdminOperatorClient().OperatorV1().IngressControllers(ingressCtrl.Namespace).Create(context.Background(), ingressCtrl, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	return ingressCtrl, waitForIngressControllerCondition(oc, timeout, types.NamespacedName{Namespace: ingressCtrl.Namespace, Name: ingressCtrl.Name}, ingressControllerNonDefaultAvailableConditions...)
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
		if _, comparable := expected[k]; comparable {
			filtered[k] = actual[k]
		}
	}
	return reflect.DeepEqual(expected, filtered)
}

func waitForIngressControllerCondition(oc *exutil.CLI, timeout time.Duration, name types.NamespacedName, conditions ...operatorv1.OperatorCondition) error {
	return wait.PollImmediate(3*time.Second, timeout, func() (bool, error) {
		ic, err := oc.AdminOperatorClient().OperatorV1().IngressControllers(name.Namespace).Get(context.Background(), name.Name, metav1.GetOptions{})
		if err != nil {
			e2e.Logf("failed to get ingresscontroller %s/%s: %v, retrying...", name.Namespace, name.Name, err)
			return false, nil
		}
		expected := operatorConditionMap(conditions...)
		current := operatorConditionMap(ic.Status.Conditions...)
		met := conditionsMatchExpected(expected, current)
		if !met {
			e2e.Logf("ingresscontroller %s/%s conditions not met; wanted %+v, got %+v, retrying...", name.Namespace, name.Name, expected, current)
		}
		return met, nil
	})
}
