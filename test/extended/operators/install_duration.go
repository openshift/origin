package operators

import (
	"context"
	"fmt"

	"k8s.io/client-go/kubernetes"

	configv1 "github.com/openshift/api/config/v1"

	configclient "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
	"github.com/openshift/origin/pkg/test/ginkgo/result"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
)

var _ = g.Describe("[sig-arch] Managed cluster should", func() {
	defer g.GinkgoRecover()

	g.It("install quickly", func() {
		// read when bootstrapping completed
		kubeConfig, err := e2e.LoadConfig()
		o.Expect(err).ToNot(o.HaveOccurred())
		configClient, err := configclient.NewForConfig(kubeConfig)
		o.Expect(err).ToNot(o.HaveOccurred())
		clusterVersion, err := configClient.ClusterVersions().Get(context.Background(), "version", metav1.GetOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())

		// clusterVersion is one of the first created resources, use that as "when the install started"
		startTime := clusterVersion.CreationTimestamp
		// we determine completion time by looking at the last transition time for Available, which ends up with "Done applying 4.6.0-0.nightly-2020-09-12-230035"
		var installCompleteTime *metav1.Time
		for _, condition := range clusterVersion.Status.Conditions {
			if condition.Type != configv1.OperatorAvailable {
				continue
			}
			if condition.Status != configv1.ConditionTrue {
				continue
			}
			installCompleteTime = &condition.LastTransitionTime
			break
		}
		if installCompleteTime == nil {
			o.Expect(fmt.Errorf("install did not complete: CVO not ready %#v", clusterVersion.Status.Conditions)).ToNot(o.HaveOccurred())
		}
		installDuration := installCompleteTime.Sub(startTime.Time)

		// this is consumed after the process is done to fake a junit duration
		e2e.Logf("OVERRIDE_DURATION=%v", installDuration)
		result.Flakef("install took %0.2f minutes", installDuration.Minutes())
	})

	g.It("bootstrap quickly", func() {
		// read when bootstrapping completed
		kubeConfig, err := e2e.LoadConfig()
		o.Expect(err).ToNot(o.HaveOccurred())
		configClient, err := configclient.NewForConfig(kubeConfig)
		o.Expect(err).ToNot(o.HaveOccurred())
		clusterVersion, err := configClient.ClusterVersions().Get(context.Background(), "version", metav1.GetOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())

		// clusterVersion is one of the first created resources, use that as "when the install started"
		startTime := clusterVersion.CreationTimestamp

		kubeClient, err := kubernetes.NewForConfig(kubeConfig)
		o.Expect(err).ToNot(o.HaveOccurred())
		bootstrapCompleteConfigMap, err := kubeClient.CoreV1().ConfigMaps("kube-system").Get(context.Background(), "bootstrap", metav1.GetOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())
		bootstrapDuration := bootstrapCompleteConfigMap.CreationTimestamp.Sub(startTime.Time)

		// this is consumed after the process is done to fake a junit duration
		e2e.Logf("OVERRIDE_DURATION=%v", bootstrapDuration)
		result.Flakef("bootstrapping took %0.2f minutes", bootstrapDuration.Minutes())
	})
})
