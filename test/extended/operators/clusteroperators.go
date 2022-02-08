package operators

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/synthetictests"

	"k8s.io/client-go/util/retry"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	s "github.com/onsi/gomega/gstruct"
	t "github.com/onsi/gomega/types"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kube-openapi/pkg/util/sets"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	config "github.com/openshift/api/config/v1"
	configclient "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
)

var _ = g.Describe("[sig-arch] ClusterOperators", func() {
	defer g.GinkgoRecover()

	var clusterOperators []config.ClusterOperator
	var clusterOperatorsClient configclient.ClusterOperatorInterface

	whitelistNoNamespace := sets.NewString(
		"cloud-credential",
		"image-registry",
		"machine-api",
		"marketplace",
		"network",
		"operator-lifecycle-manager",
		"operator-lifecycle-manager-catalog",
		"support",
	)
	whitelistNoOperatorConfig := sets.NewString(
		"cloud-credential",
		"cluster-autoscaler",
		"machine-api",
		"machine-config",
		"marketplace",
		"network",
		"operator-lifecycle-manager",
		"operator-lifecycle-manager-catalog",
		"support",
	)

	g.BeforeEach(func() {
		kubeConfig, err := e2e.LoadConfig()
		o.Expect(err).ToNot(o.HaveOccurred())
		client, err := configclient.NewForConfig(kubeConfig)
		o.Expect(err).ToNot(o.HaveOccurred())
		clusterOperatorsClient = client.ClusterOperators()
		clusterOperatorsList, err := clusterOperatorsClient.List(context.Background(), metav1.ListOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())
		clusterOperators = clusterOperatorsList.Items
	})

	g.It("should reconcile the messages and update last transition time", func() {
		operatorOriginalConditions := map[string][]config.ClusterOperatorStatusCondition{}
		operatorsToReconcileMessage := sets.NewString()
		interestingConditionTypes := sets.NewString("Available", "Degraded", "Progressing", "Upgradeable")

		messageToReconcile := "operator is expected to reconcile this message"
		for _, clusterOperator := range clusterOperators {
			updateErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				newClusterOperator, err := clusterOperatorsClient.Get(context.TODO(), clusterOperator.Name, metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				if err != nil {
					return err
				}
				clusterOperatorCopy := newClusterOperator.DeepCopy()
				for i := range clusterOperatorCopy.Status.Conditions {
					newCondition := clusterOperatorCopy.Status.Conditions[i]
					if !interestingConditionTypes.Has(string(newCondition.Type)) {
						continue
					}
					operatorOriginalConditions[newClusterOperator.Name] = append(operatorOriginalConditions[clusterOperator.Name], clusterOperatorCopy.Status.Conditions[i])
					newCondition.Message = messageToReconcile
					clusterOperatorCopy.Status.Conditions[i] = newCondition
				}
				_, err = clusterOperatorsClient.UpdateStatus(context.TODO(), clusterOperatorCopy, metav1.UpdateOptions{})
				return err
			})
			// if we failed to update single cluster operator, this test failed
			if updateErr != nil {
				o.Expect(updateErr).ToNot(o.HaveOccurred())
				return
			}
			operatorsToReconcileMessage.Insert(clusterOperator.Name)
		}

		for {
			time.Sleep(300 * time.Millisecond) // prevent hot-looping

			for _, clusterOperator := range clusterOperators {
				reconciledClusterOperator, err := clusterOperatorsClient.Get(context.TODO(), clusterOperator.Name, metav1.GetOptions{})
				if err != nil {
					e2e.Logf("failed to get %q operator: %w", clusterOperator.Name, err)
				}

				hasMessageToReconcile := false
				for _, c := range reconciledClusterOperator.Status.Conditions {
					if !interestingConditionTypes.Has(string(c.Type)) {
						continue
					}
					// check if the condition message has changed, if not go to next operator and return to this one in next iteration
					if c.Message == messageToReconcile {
						e2e.Logf("operator %s condition %s has not reconciled ...", clusterOperator.Name, c.Type)
						hasMessageToReconcile = true
					}
				}
				if !hasMessageToReconcile {
					operatorsToReconcileMessage.Delete(clusterOperator.Name)
				}
			}

			select {
			case <-time.After(60 * time.Second):
				messages := []string{}
				for _, operatorName := range operatorsToReconcileMessage.List() {
					component := synthetictests.GetBugzillaComponentForOperator(operatorName)
					messages = append(messages, fmt.Sprintf("[bz-%s] cluster operator %s failed to reconcile condition message after it changed", component, operatorName))
				}
				o.Expect(fmt.Errorf(strings.Join(messages, "\n"))).ToNot(o.HaveOccurred())
				return
			default:
				// all operators reconciled their messages
				if operatorsToReconcileMessage.Len() == 0 {
					return
				}
			}
		}
	})

	g.Context("should define", func() {
		g.Specify("at least one namespace in their lists of related objects", func() {
			for _, clusterOperator := range clusterOperators {
				if !whitelistNoNamespace.Has(clusterOperator.Name) {
					o.Expect(clusterOperator.Status.RelatedObjects).To(o.ContainElement(isNamespace()), "ClusterOperator: %s", clusterOperator.Name)
				}
			}
		})
		g.Specify("at least one related object that is not a namespace", func() {
			for _, clusterOperator := range clusterOperators {
				if !whitelistNoOperatorConfig.Has(clusterOperator.Name) {
					o.Expect(clusterOperator.Status.RelatedObjects).To(o.ContainElement(o.Not(isNamespace())), "ClusterOperator: %s", clusterOperator.Name)
				}
			}
		})

	})
})

func isNamespace() t.GomegaMatcher {
	return s.MatchFields(s.IgnoreExtras|s.IgnoreMissing, s.Fields{
		"Resource": o.Equal("namespaces"),
		"Group":    o.Equal(""),
	})
}
