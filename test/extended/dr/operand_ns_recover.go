package dr

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/kube-openapi/pkg/util/sets"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	config "github.com/openshift/api/config/v1"
	configclient "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
)

var _ = g.Describe("[sig-arch][Disruptive] Managed cluster should recover", func() {
	defer g.GinkgoRecover()
	// Delete clusteroperator-owned operand ns, ensure cluster recovery
	// TODO: Add 'openshift-multus' ns, skip due to it takes too long to terminate/return
	g.It("[Feature:ClusterOperatorRecovery][Slow] when operand namespaces deleted", func() {
		ctx := context.Background()
		c, err := e2e.LoadClientset()
		o.Expect(err).NotTo(o.HaveOccurred())

		// List of clusteroperators that have operand namespaces
		operandNS := []string{
			"openshift-etcd",
			//"openshift-multus",
			"openshift-authentication",
			"openshift-dns",
			"openshift-service-ca",
			"openshift-console",
			"openshift-kube-controller-manager",
			"openshift-kube-scheduler",
			"openshift-kube-storage-version-migrator",
			"openshift-controller-manager",
			"openshift-apiserver",
			"openshift-kube-apiserver",
		}
		kubeConfig, err := e2e.LoadConfig()
		o.Expect(err).ToNot(o.HaveOccurred())
		configClient, err := configclient.NewForConfig(kubeConfig)
		o.Expect(err).ToNot(o.HaveOccurred())
		// remember the initial set of operators
		operators, err := configClient.ClusterOperators().List(context.Background(), metav1.ListOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())

		for _, ns := range operandNS {
			err = c.CoreV1().Namespaces().Delete(ctx, ns, *metav1.NewDeleteOptions(1))
			o.Expect(err).NotTo(o.HaveOccurred())
			err = wait.PollImmediate(2*time.Second, 10*time.Minute, func() (bool, error) {
				proj, err := c.CoreV1().Namespaces().Get(ctx, ns, metav1.GetOptions{})
				if err != nil || proj.Status.Phase != corev1.NamespaceActive {
					e2e.Logf("waiting for operand namespace %s to become Active", ns)
					return false, nil
				}
				e2e.Logf("namespace %s is Active", ns)
				return true, nil
			})
			o.Expect(err).ToNot(o.HaveOccurred())
		}
		// could waitForOperatorsToSettle() but kube-apiserver recovery time is too long
		err = allClusterOperatorsHealthy(operators, configClient)
		o.Expect(err).ToNot(o.HaveOccurred())
	})
})

func hasCondition(operator *config.ClusterOperator, name config.ClusterStatusConditionType, status config.ConditionStatus) bool {
	for _, condition := range operator.Status.Conditions {
		if name != condition.Type {
			continue
		}
		return condition.Status == status
	}
	return false
}

func isHealthy(operator config.ClusterOperator) bool {
	if hasCondition(&operator, config.OperatorAvailable, config.ConditionTrue) &&
		hasCondition(&operator, config.OperatorDegraded, config.ConditionFalse) &&
		hasCondition(&operator, config.OperatorProgressing, config.ConditionFalse) {
		return true
	}
	// for sake of e2e limit, kube-apiserver Progressing=True ok
	if operator.Name == "kube-apiserver" &&
		hasCondition(&operator, config.OperatorAvailable, config.ConditionTrue) &&
		hasCondition(&operator, config.OperatorDegraded, config.ConditionFalse) {
		return true
	}
	return false
}

func allClusterOperatorsHealthy(operators *config.ClusterOperatorList, configClient *configclient.ConfigV1Client) error {
	// After deleting operand namespaces and recovering, check that all clusteroperators are left in good state.
	var currentOperators *config.ClusterOperatorList
	var healthy []config.ClusterOperator
	var err error
	err = wait.PollImmediate(5*time.Second, 30*time.Minute, func() (bool, error) {
		currentOperators, err = configClient.ClusterOperators().List(context.Background(), metav1.ListOptions{})
		if err != nil {
			return false, err
		}
		healthy = nil
		for _, operator := range currentOperators.Items {
			if isHealthy(operator) {
				healthy = append(healthy, operator)
			} else {
				e2e.Logf("%s clusteroperator is not healthy", operator.Name)
			}

		}
		if len(healthy) < len(operators.Items) {
			return false, nil
		}
		if len(healthy) > len(operators.Items) {
			return false, fmt.Errorf("found %s operators that were not in original set", strings.Join(operatorNames(healthy).Difference(operatorNames(operators.Items)).List(), ", "))
		}
		if !operatorNames(healthy).Equal(operatorNames(operators.Items)) {
			return false, fmt.Errorf("found %s operators that were not in original set", strings.Join(operatorNames(healthy).Difference(operatorNames(operators.Items)).List(), ", "))
		}
		return true, nil
	})
	if err != nil {
		buf := &bytes.Buffer{}
		w := tabwriter.NewWriter(buf, 0, 1, 1, ' ', 0)
		for _, operator := range currentOperators.Items {
			fmt.Fprintf(w, "%s\t%s\t%s\n", operator.Name, conditionStatus(&operator, config.OperatorAvailable), conditionStatus(&operator, config.OperatorDegraded))
		}
		w.Flush()
		return fmt.Errorf("Operators never became available: %s\n%s", strings.Join(operatorNames(operators.Items).Difference(operatorNames(healthy)).List(), ", "), buf.String())
	}
	return nil
}

func conditionStatus(operator *config.ClusterOperator, name config.ClusterStatusConditionType) config.ConditionStatus {
	for _, condition := range operator.Status.Conditions {
		if name != condition.Type {
			continue
		}
		return condition.Status
	}
	return ""
}

func operatorNames(operators []config.ClusterOperator) sets.String {
	names := sets.NewString()
	for _, operator := range operators {
		names.Insert(operator.Name)
	}
	return names
}
