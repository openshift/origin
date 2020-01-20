package csi

import (
	"context"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	coreclient "k8s.io/client-go/kubernetes/typed/core/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	configclient "github.com/openshift/client-go/config/clientset/versioned"
)

var _ = g.Describe("[sig-storage] [CSICertification] CSI driver", func() {
	defer g.GinkgoRecover()

	g.It("should be running on an OpenShift cluster", func() {
		cfg, err := e2e.LoadConfig()
		o.Expect(err).NotTo(o.HaveOccurred())
		c := configclient.NewForConfigOrDie(cfg)

		coreclient, err := e2e.LoadClientset()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("checking for the Cluster Version Operator")
		waitForCVO(coreclient.CoreV1().Namespaces())

		_, err = c.ConfigV1().ClusterVersions().Get(context.TODO(), "version", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
	})
})

func waitForCVO(c coreclient.NamespaceInterface) {
	err := wait.PollImmediate(time.Second, time.Minute, func() (bool, error) {
		_, err := c.Get(context.TODO(), "openshift-cluster-version", metav1.GetOptions{})
		if err == nil {
			return true, nil
		}
		e2e.Logf("Unable to get CVO namespace: %v", err)
		return false, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
}
