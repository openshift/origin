package shard

import (
	"context"
	"reflect"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	operatorv1 "github.com/openshift/api/operator/v1"
	exutil "github.com/openshift/origin/test/extended/util"
)

type Config struct {
	// FixturePath is the path to the ingresscontroller fixture.
	FixturePath string

	// Name is the name of the ingresscontroller
	Name string

	// Domain is the domain for the ingresscontroller to host
	Domain string

	// Type is the matchSelector
	Type string
}

var ingressControllerNonDefaultAvailableConditions = []operatorv1.OperatorCondition{
	{Type: operatorv1.IngressControllerAvailableConditionType, Status: operatorv1.ConditionTrue},
	{Type: operatorv1.LoadBalancerManagedIngressConditionType, Status: operatorv1.ConditionTrue},
	{Type: operatorv1.LoadBalancerReadyIngressConditionType, Status: operatorv1.ConditionTrue},
	{Type: operatorv1.DNSManagedIngressConditionType, Status: operatorv1.ConditionTrue},
	{Type: operatorv1.DNSReadyIngressConditionType, Status: operatorv1.ConditionTrue},
	{Type: "Admitted", Status: operatorv1.ConditionTrue},
}

func DeployNewRouterShard(oc *exutil.CLI, timeout time.Duration, cfg Config) (string, error) {
	jsonCfg, err := oc.AsAdmin().Run("process").Args("-f", cfg.FixturePath, "-p",
		"NAME="+cfg.Name,
		"NAMESPACE=openshift-ingress-operator",
		"DOMAIN="+cfg.Domain,
		"TYPE="+cfg.Type).OutputToFile("config.json")
	if err != nil {
		return "", err
	}

	if err := oc.AsAdmin().Run("create").Args("-f", jsonCfg, "--namespace=openshift-ingress-operator").Execute(); err != nil {
		return "", err
	}

	return jsonCfg, waitForIngressControllerCondition(oc, timeout, types.NamespacedName{Namespace: "openshift-ingress-operator", Name: oc.Namespace()}, ingressControllerNonDefaultAvailableConditions...)
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
