package operators

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	// corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/kube-openapi/pkg/util/sets"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	config "github.com/openshift/api/config/v1"
	configclient "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
)

var _ = g.Describe("[sig-arch] Managed cluster should recover", func() {
	defer g.GinkgoRecover()

	g.It("when operator-owned objects are deleted [Disruptive][apigroup:config.openshift.io]", g.Label("Size:L"), func() {
		daemonsets := false
		deployments := false
		secrets := true
		serviceAccountSecrets := false

		c, err := e2e.LoadClientset()
		o.Expect(err).ToNot(o.HaveOccurred())
		skipUnlessCVO(c.CoreV1().Namespaces())
		kubeConfig, err := e2e.LoadConfig()
		o.Expect(err).ToNot(o.HaveOccurred())
		configClient, err := configclient.NewForConfig(kubeConfig)
		o.Expect(err).ToNot(o.HaveOccurred())

		// remember the initial set of operators
		operators, err := configClient.ClusterOperators().List(context.Background(), metav1.ListOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())

		// delete all daemonsets in openshift-* namespaces
		all, err := c.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())
		for _, ns := range all.Items {
			switch {
			case !strings.HasPrefix(ns.Name, "openshift-"):
				continue
			case ns.Name == "openshift-cluster-version", ns.Name == "openshift-config":
				continue
			}
			one := int64(1)
			background := metav1.DeletePropagationBackground
			if deployments {
				g.By(fmt.Sprintf("Deleting deployments in namespace %s", ns.Name))
				err := c.AppsV1().Deployments(ns.Name).DeleteCollection(context.Background(), metav1.DeleteOptions{GracePeriodSeconds: &one, PropagationPolicy: &background}, metav1.ListOptions{})
				o.Expect(err).ToNot(o.HaveOccurred())
			}
			if daemonsets {
				g.By(fmt.Sprintf("Deleting daemonsets in namespace %s", ns.Name))
				err := c.AppsV1().DaemonSets(ns.Name).DeleteCollection(context.Background(), metav1.DeleteOptions{GracePeriodSeconds: &one, PropagationPolicy: &background}, metav1.ListOptions{})
				o.Expect(err).ToNot(o.HaveOccurred())
			}
			if secrets {
				g.By(fmt.Sprintf("Deleting non-generated secrets in namespace %s", ns.Name))
				secrets, err := c.CoreV1().Secrets(ns.Name).List(context.Background(), metav1.ListOptions{})
				o.Expect(err).ToNot(o.HaveOccurred())
				for _, secret := range secrets.Items {
					if !serviceAccountSecrets && len(secret.Annotations["kubernetes.io/service-account.name"]) > 0 {
						continue
					}
					err := c.CoreV1().Secrets(secret.Namespace).Delete(context.Background(), secret.Name, metav1.DeleteOptions{})
					o.Expect(err).ToNot(o.HaveOccurred())
				}
			}
		}

		// delete all cluster operators
		g.By("Deleting all cluster operators")
		foreground := metav1.DeletePropagationForeground
		err = configClient.ClusterOperators().DeleteCollection(context.Background(), metav1.DeleteOptions{PropagationPolicy: &foreground}, metav1.ListOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())

		g.By("Waiting for all cluster operators to recover")
		var currentOperators *config.ClusterOperatorList
		var healthy []config.ClusterOperator
		err = wait.PollImmediate(5*time.Second, 20*time.Minute, func() (bool, error) {
			currentOperators, err = configClient.ClusterOperators().List(context.Background(), metav1.ListOptions{})
			o.Expect(err).ToNot(o.HaveOccurred())

			healthy = nil
			for _, operator := range currentOperators.Items {
				if hasCondition(&operator, config.OperatorAvailable, config.ConditionTrue) && hasCondition(&operator, config.OperatorDegraded, config.ConditionFalse) {
					healthy = append(healthy, operator)
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
			o.Expect(err).ToNot(o.HaveOccurred(), fmt.Sprintf("Operators never became available: %s\n%s", strings.Join(operatorNames(operators.Items).Difference(operatorNames(healthy)).List(), ", "), buf.String()))
		}
	})
})

func conditionStatus(operator *config.ClusterOperator, name config.ClusterStatusConditionType) config.ConditionStatus {
	for _, condition := range operator.Status.Conditions {
		if name != condition.Type {
			continue
		}
		return condition.Status
	}
	return ""
}

func hasCondition(operator *config.ClusterOperator, name config.ClusterStatusConditionType, status config.ConditionStatus) bool {
	for _, condition := range operator.Status.Conditions {
		if name != condition.Type {
			continue
		}
		return condition.Status == status
	}
	return false
}

func operatorNames(operators []config.ClusterOperator) sets.String {
	names := sets.NewString()
	for _, operator := range operators {
		names.Insert(operator.Name)
	}
	return names
}
