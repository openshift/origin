package security

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/fsouza/go-dockerclient"
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/test/e2e"

	testutil "github.com/openshift/origin/test/util"
)

var _ = g.Describe("security: supplemental groups", func() {
	defer g.GinkgoRecover()

	var (
		f = e2e.NewFramework("security-supgroups")
	)

	g.Describe("Ensure supplemental groups propagate to docker", func() {
		g.It("should propagate requested groups to the docker host config", func() {
			// Before running any of this test we need to first check that
			// the docker version being used supports the supplemental groups feature
			g.By("ensuring the feature is supported")
			dockerCli, err := testutil.NewDockerClient()
			o.Expect(err).NotTo(o.HaveOccurred())

			env, err := dockerCli.Version()
			o.Expect(err).NotTo(o.HaveOccurred(), "error getting docker environment")
			version := env.Get("Version")
			supports, err, requiredVersion := supportsSupplementalGroups(version)
			o.Expect(err).NotTo(o.HaveOccurred())

			if !supports {
				msg := fmt.Sprintf("skipping supplemental groups test, docker version %s does not meet required version %s", version, requiredVersion)
				g.Skip(msg)
			}

			// on to the real test
			fsGroup := int64(1111)
			supGroup := int64(2222)

			// create a pod that is requesting supplemental groups.  We request specific sup groups
			// so that we can check for the exact values later and not rely on SCC allocation.
			g.By("creating a pod that requests supplemental groups")
			submittedPod := supGroupPod(fsGroup, supGroup)
			_, err = f.Client.Pods(f.Namespace.Name).Create(submittedPod)
			o.Expect(err).NotTo(o.HaveOccurred())
			defer f.Client.Pods(f.Namespace.Name).Delete(submittedPod.Name, nil)

			// we should have been admitted with the groups that we requested but if for any
			// reason they are different we will fail.
			g.By("retrieving the pod and ensuring groups are set")
			retrievedPod, err := f.Client.Pods(f.Namespace.Name).Get(submittedPod.Name)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(*retrievedPod.Spec.SecurityContext.FSGroup).To(o.Equal(*submittedPod.Spec.SecurityContext.FSGroup))
			o.Expect(retrievedPod.Spec.SecurityContext.SupplementalGroups).To(o.Equal(submittedPod.Spec.SecurityContext.SupplementalGroups))

			// wait for the pod to run so we can inspect it.
			g.By("waiting for the pod to become running")
			err = f.WaitForPodRunning(submittedPod.Name)
			o.Expect(err).NotTo(o.HaveOccurred())

			// find the docker id of our running container.
			g.By("finding the docker container id on the pod")
			retrievedPod, err = f.Client.Pods(f.Namespace.Name).Get(submittedPod.Name)
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
			o.Expect(configHasGroup(fsGroup, dockerContainer.HostConfig)).To(o.Equal(true), fmt.Sprintf("fsGroup should exist on host config: %v", groupAdd))
			o.Expect(configHasGroup(supGroup, dockerContainer.HostConfig)).To(o.Equal(true), fmt.Sprintf("supGroup should exist on host config: %v", groupAdd))
		})

	})
})

// supportsSupplementalGroups does a check on the docker version to ensure it is at least
// 1.8.2.  This could still fail if the version does not have the /etc/groups patch
// but it will fail when launching the pod so this is as safe as we can get.
func supportsSupplementalGroups(dockerVersion string) (bool, error, string) {
	parts := strings.Split(dockerVersion, ".")

	var (
		requiredMajor   = 1
		requiredMinor   = 8
		requiredPatch   = 2
		requiredVersion = fmt.Sprintf("%d.%d.%d", requiredMajor, requiredMinor, requiredPatch)

		major       = 0
		minor       = 0
		patch       = 0
		err   error = nil
	)
	if len(parts) > 0 {
		major, err = strconv.Atoi(parts[0])
		if err != nil {
			return false, err, requiredVersion
		}
	}

	if len(parts) > 1 {
		minor, err = strconv.Atoi(parts[1])
		if err != nil {
			return false, err, requiredVersion
		}
	}

	if len(parts) > 2 {
		patch, err = strconv.Atoi(parts[2])
		if err != nil {
			return false, err, requiredVersion
		}
	}

	// requires at least 1.8.2
	if major > requiredMajor || (major == requiredMajor && minor > requiredMinor) ||
		(major == requiredMajor && minor == requiredMinor && patch >= requiredPatch) {
		return true, nil, requiredVersion
	}

	return false, nil, requiredVersion
}

// configHasGroup is a helper to ensure that a group is in the host config's addGroups field.
func configHasGroup(group int64, config *docker.HostConfig) bool {
	strGroup := strconv.FormatInt(group, 10)
	for _, g := range config.GroupAdd {
		if g == strGroup {
			return true
		}
	}
	return false
}

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
