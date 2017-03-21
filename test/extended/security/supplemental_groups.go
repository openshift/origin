package security

import (
	"fmt"
	"strconv"
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	kapi "k8s.io/kubernetes/pkg/api"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	testutil "github.com/openshift/origin/test/util"
)

var _ = g.Describe("[security] supplemental groups", func() {
	defer g.GinkgoRecover()

	var (
		f = e2e.NewDefaultFramework("security-supgroups")
	)

	g.Describe("[Conformance]Ensure supplemental groups propagate to docker", func() {
		g.It("should propagate requested groups to the docker host config [local]", func() {
			g.By("getting the docker client")
			dockerCli, err := testutil.NewDockerClient()
			o.Expect(err).NotTo(o.HaveOccurred())

			fsGroup := int64(1111)
			supGroup := int64(2222)

			// create a pod that is requesting supplemental groups.  We request specific sup groups
			// so that we can check for the exact values later and not rely on SCC allocation.
			g.By("creating a pod that requests supplemental groups")
			submittedPod := supGroupPod(fsGroup, supGroup)
			_, err = f.ClientSet.Core().Pods(f.Namespace.Name).Create(submittedPod)
			o.Expect(err).NotTo(o.HaveOccurred())
			defer f.ClientSet.Core().Pods(f.Namespace.Name).Delete(submittedPod.Name, nil)

			// we should have been admitted with the groups that we requested but if for any
			// reason they are different we will fail.
			g.By("retrieving the pod and ensuring groups are set")
			retrievedPod, err := f.ClientSet.Core().Pods(f.Namespace.Name).Get(submittedPod.Name)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(*retrievedPod.Spec.SecurityContext.FSGroup).To(o.Equal(*submittedPod.Spec.SecurityContext.FSGroup))
			o.Expect(retrievedPod.Spec.SecurityContext.SupplementalGroups).To(o.Equal(submittedPod.Spec.SecurityContext.SupplementalGroups))

			// wait for the pod to run so we can inspect it.
			g.By("waiting for the pod to become running")
			err = f.WaitForPodRunning(submittedPod.Name)
			o.Expect(err).NotTo(o.HaveOccurred())

			// find the docker id of our running container.
			g.By("finding the docker container id on the pod")
			retrievedPod, err = f.ClientSet.Core().Pods(f.Namespace.Name).Get(submittedPod.Name)
			o.Expect(err).NotTo(o.HaveOccurred())
			containerID, err := getContainerID(retrievedPod)
			o.Expect(err).NotTo(o.HaveOccurred())

			// now check the host config of the container which should have been updated by the
			// kubelet.  If that is good then ensure we have the groups we expected.
			g.By("inspecting the container")
			dockerContainer, err := dockerCli.InspectContainer(containerID)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("ensuring the host config has GroupAdd")
			groupAdd := dockerContainer.HostConfig.GroupAdd
			o.Expect(groupAdd).ToNot(o.BeEmpty(), fmt.Sprintf("groupAdd on host config was %v", groupAdd))

			g.By("ensuring the groups are set")
			group := strconv.FormatInt(fsGroup, 10)
			o.Expect(groupAdd).To(o.ContainElement(group), fmt.Sprintf("fsGroup %v should exist on host config: %v", fsGroup, groupAdd))

			group = strconv.FormatInt(supGroup, 10)
			o.Expect(groupAdd).To(o.ContainElement(group), fmt.Sprintf("supGroup %v should exist on host config: %v", supGroup, groupAdd))
		})

	})
})

// getContainerID is a helper to parse the docker container id from a status.
func getContainerID(p *kapi.Pod) (string, error) {
	for _, status := range p.Status.ContainerStatuses {
		if len(status.ContainerID) > 0 {
			containerID := strings.Replace(status.ContainerID, "docker://", "", -1)
			return containerID, nil
		}
	}
	return "", fmt.Errorf("unable to find container id on pod")
}

// supGroupPod generates the pod requesting supplemental groups.
func supGroupPod(fsGroup int64, supGroup int64) *kapi.Pod {
	return &kapi.Pod{
		ObjectMeta: kapi.ObjectMeta{
			Name: "supplemental-groups",
		},
		Spec: kapi.PodSpec{
			SecurityContext: &kapi.PodSecurityContext{
				FSGroup:            &fsGroup,
				SupplementalGroups: []int64{supGroup},
			},
			Containers: []kapi.Container{
				{
					Name:  "supplemental-groups",
					Image: "openshift/origin-pod",
				},
			},
		},
	}
}
