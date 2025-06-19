package baremetal

import (
	"context"
	"time"

	configv1 "github.com/openshift/api/config/v1"
	clusteroperatorhelpers "github.com/openshift/library-go/pkg/config/clusteroperator/v1helpers"

	"k8s.io/apimachinery/pkg/api/errors"

	e2e "k8s.io/kubernetes/test/e2e/framework"

	"k8s.io/apimachinery/pkg/util/wait"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	deleteInterval           = 1 * time.Second
	deleteWaitTimeout        = 3 * time.Minute
	restartInterval          = 10 * time.Second
	restartWaitTimeout       = 10 * time.Minute
	baremetalNamespace       = "openshift-machine-api"
	clusterBaremetalOperator = "cluster-baremetal-operator"
	metal3Deployment         = "metal3"
)

var _ = g.Describe("[sig-installer][Feature:baremetal][Serial] Baremetal platform should ensure [apigroup:config.openshift.io]", func() {
	defer g.GinkgoRecover()

	var (
		oc     = exutil.NewCLI("baremetal")
		helper *BaremetalTestHelper
	)

	g.BeforeEach(func() {
		skipIfNotBaremetal(oc)
		helper = NewBaremetalTestHelper(oc.AdminDynamicClient())
		helper.Setup()
	})

	g.AfterEach(func() {
		helper.DeleteAllExtraWorkers()
	})

	g.It("cluster baremetal operator and metal3 deployment return back healthy after they are deleted", func() {
		c, err := e2e.LoadClientset()
		o.Expect(err).ToNot(o.HaveOccurred())

		g.By("delete cluster baremetal operator")
		err = c.AppsV1().Deployments(baremetalNamespace).Delete(context.Background(), clusterBaremetalOperator, v1.DeleteOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		logrus.Infof("Event - delete CBO")

		g.By("wait until cluster baremetal operator is deleted")
		err = wait.PollImmediate(deleteInterval, deleteWaitTimeout, func() (bool, error) {
			_, err := c.AppsV1().Deployments(baremetalNamespace).Get(context.Background(), clusterBaremetalOperator, v1.GetOptions{})
			if err != nil {
				if errors.IsNotFound(err) {
					return true, nil
				}

				e2e.Logf("Error getting cluster baremetal operator deployment: %v", err)
				return false, nil
			}

			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		logrus.Infof("Event - wait until CBO is deleted")

		g.By("wait until cluster baremetal operator returns back healthy")
		err = wait.Poll(restartInterval, restartWaitTimeout, func() (bool, error) {
			dc, err := c.AppsV1().Deployments(baremetalNamespace).Get(context.Background(), clusterBaremetalOperator, v1.GetOptions{})
			if err != nil {
				if errors.IsNotFound(err) {
					return false, nil
				}

				e2e.Logf("Error getting cluster baremetal operator deployment: %v", err)
				return false, nil
			}

			if dc.Status.AvailableReplicas != 1 {
				return false, nil
			}

			baremetalCO, err := oc.AdminConfigClient().ConfigV1().ClusterOperators().Get(context.Background(), "baremetal", v1.GetOptions{})
			if err != nil {
				e2e.Logf("Error getting baremetal operator: %v", err)
				return false, nil
			}

			if clusteroperatorhelpers.IsStatusConditionFalse(baremetalCO.Status.Conditions, configv1.OperatorAvailable) {
				return false, nil
			}

			return true, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		logrus.Infof("Event - wait until CBO returns back healthy")

		g.By("delete metal3 deployment")
		err = c.AppsV1().Deployments(baremetalNamespace).Delete(context.Background(), metal3Deployment, v1.DeleteOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		logrus.Infof("Event - delete Metal3 Deployment")

		g.By("wait until metal3 deployment is deleted")
		err = wait.PollImmediate(deleteInterval, deleteWaitTimeout, func() (bool, error) {
			_, err := c.AppsV1().Deployments(baremetalNamespace).Get(context.Background(), metal3Deployment, v1.GetOptions{})
			if err != nil {
				if errors.IsNotFound(err) {
					return true, nil
				}

				e2e.Logf("Error getting metal3 deployment: %v", err)
				return false, nil
			}

			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		logrus.Infof("Event - wait until Metal3 Deployment is deleted")

		g.By("wait until metal3 deployment returns back healthy")
		err = wait.Poll(restartInterval, restartWaitTimeout, func() (bool, error) {
			dc, err := c.AppsV1().Deployments(baremetalNamespace).Get(context.Background(), metal3Deployment, v1.GetOptions{})
			if err != nil {
				if errors.IsNotFound(err) {
					return false, nil
				}

				e2e.Logf("Error getting metal3 deployment: %v", err)
				return false, nil
			}

			if dc.Status.AvailableReplicas != 1 {
				return false, nil
			}

			return true, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		logrus.Infof("Event - wait until Metal3 Deployment returns back healthy")

		g.By("verify that baremetal hosts are healthy and correct state")
		checkMetal3DeploymentHealthy(oc)
		e2e.Logf("Phase: verify that baremetal hosts are healthy and correct state took %s\n", time.Since(start7))
		logrus.Infof("Event - Verified that baremetal hosts are healthy and in correct state")
	})
})

func checkMetal3DeploymentHealthy(oc *exutil.CLI) {
	dc := oc.AdminDynamicClient()
	bmc := baremetalClient(dc)

	hosts, err := bmc.List(context.Background(), v1.ListOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(hosts.Items).ToNot(o.BeEmpty())

	for _, h := range hosts.Items {
		expectStringField(h, "baremetalhost", "status.operationalStatus").To(o.BeEquivalentTo("OK"))
		expectStringField(h, "baremetalhost", "status.provisioning.state").To(o.Or(o.BeEquivalentTo("provisioned"), o.BeEquivalentTo("externally provisioned")))
		expectBoolField(h, "baremetalhost", "spec.online").To(o.BeTrue())
	}
}
