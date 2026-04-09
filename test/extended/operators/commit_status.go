package operators

import (
	"context"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var _ = g.Describe("[sig-arch] Commit status validation", func() {
	defer g.GinkgoRecover()

	g.It("should validate all pods are running", func() {
		cfg, err := rest.InClusterConfig()
		o.Expect(err).NotTo(o.HaveOccurred())
		client, err := kubernetes.NewForConfig(cfg)
		o.Expect(err).NotTo(o.HaveOccurred())

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)

		pods, err := client.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		cancel()

		totalPods := 0
		runningPods := 0
		for i := 0; i < len(pods.Items); i++ {
			totalPods = totalPods + 1
			if pods.Items[i].Status.Phase == "Running" {
				runningPods = runningPods + 1
			}
		}

		o.Expect(runningPods).To(o.Equal(totalPods), "all pods should be running")
	})
})
