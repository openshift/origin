package ownership

import (
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	"k8s.io/kubernetes/pkg/util/wait"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("ownership", func() {
	defer g.GinkgoRecover()

	var (
		oc = exutil.NewCLI("ownership", exutil.KubeConfigPath())

		sampleReplicaSetFixture            = exutil.FixturePath("testdata", "ownership", "rs.yaml")
		sampleReplicationControllerFixture = exutil.FixturePath("testdata", "ownership", "rc.yaml")

		readyReplicasTimeout = 10 * time.Second
		removePodsTimeout    = 10 * time.Second

		waitForReadyReplicas = func(name, replicas string) error {
			defer g.GinkgoRecover()
			return wait.PollImmediate(time.Second, readyReplicasTimeout, func() (bool, error) {
				out, err := oc.Run("get").Args(name, "-o", "template", "--template", "{{.status.readyReplicas}}").Output()
				if err != nil {
					return false, err
				}
				return strings.TrimSpace(out) == replicas, nil
			})
		}

		getPods = func() []string {
			defer g.GinkgoRecover()
			out, err := oc.Run("get").Args("pods", "-l", "app=frontend", "-o", "template", "--template", "{{range .items}}{{.metadata.name}} {{end}}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			return strings.Split(strings.TrimSpace(out), " ")
		}

		waitForPodCount = func(count int) error {
			return wait.PollImmediate(time.Second, removePodsTimeout, func() (bool, error) {
				return len(getPods()) == count, nil
			})
		}

		podOwnerReference = func(name string) (string, string) {
			defer g.GinkgoRecover()
			out, err := oc.Run("get").Args("pod", name, "-o", "template", "--template", "{{range .metadata.ownerReferences}}{{.name}},{{.kind}}{{end}}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			refs := strings.Split(strings.TrimSpace(out), ",")
			// If this fails, it means the pod does not have ownerReference
			o.Expect(len(refs)).To(o.BeNumerically("==", 2))
			return refs[0], refs[1]
		}
	)

	g.Describe("when having same label for replica set and replication controller [Conformance]", func() {

		g.It("both should manage their pods independently", func() {
			g.By("replicaSet 'frontend' is created")
			_, err := oc.Run("create").Args("-f", sampleReplicaSetFixture).Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for replicaSet 'frontend' pods to be ready")
			err = waitForReadyReplicas("rs/frontend", "2")
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("verifying the replicaSet 'frontend' pods have correct ownerReferences")
			pods := getPods()
			o.Expect(len(pods)).To(o.BeNumerically("==", 2))
			for _, podName := range pods {
				_, kind := podOwnerReference(podName)
				o.Expect(kind).To(o.BeEquivalentTo("ReplicaSet"))
			}

			g.By("replicationController 'frontend' is created")
			_, err = oc.Run("create").Args("-f", sampleReplicationControllerFixture).Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for replicationController 'frontend' pods to be ready")
			err = waitForReadyReplicas("rc/frontend", "2")
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("verifying the replicationController 'frontend' pods have correct ownerReferences")
			oldPods := pods
			pods = getPods()
			o.Expect(len(pods)).To(o.BeNumerically("==", 4))
			for _, podName := range pods {
				skip := false
				for _, rsPod := range oldPods {
					if podName == rsPod {
						skip = true
					}
				}
				if skip {
					continue
				}
				_, kind := podOwnerReference(podName)
				o.Expect(kind).To(o.BeEquivalentTo("ReplicationController"))
			}

			g.By("deleting replicaset 'frontend'")
			_, err = oc.Run("delete").Args("rs/frontend").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			err = waitForPodCount(2)
			// If this timeout, it means either the pods were not removed for replicaSet or we
			// teared down the replication controller pods which should not happen.
			o.Expect(err).NotTo(o.HaveOccurred())
			// Only the replication controller pods should be left
			for _, podName := range getPods() {
				_, kind := podOwnerReference(podName)
				o.Expect(kind).To(o.BeEquivalentTo("ReplicationController"))
			}

			// Now do the same but delete the replication controller instead
			g.By("replicaSet 'frontend' is recreated")
			_, err = oc.Run("create").Args("-f", sampleReplicaSetFixture).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			g.By("waiting for replicaSet 'frontend' pods to be ready")
			err = waitForReadyReplicas("rs/frontend", "2")
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("deleting replication controller 'frontend'")
			_, err = oc.Run("delete").Args("rc/frontend").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			err = waitForPodCount(2)
			for _, podName := range getPods() {
				_, kind := podOwnerReference(podName)
				o.Expect(kind).To(o.BeEquivalentTo("ReplicaSet"))
			}

		})

	})
})
