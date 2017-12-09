package security

import (
	"fmt"
	"strconv"
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	kapiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/util/retry"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	supplementalGroupsPod = "supplemental-groups"
)

var _ = g.Describe("[security] supplemental groups", func() {
	defer g.GinkgoRecover()

	var (
		oc = exutil.NewCLI("sup-groups", exutil.KubeConfigPath())
		f  = oc.KubeFramework()
	)

	g.Describe("[Conformance]Ensure supplemental groups propagate to docker", func() {
		g.It("should propagate requested groups to the container [local]", func() {

			fsGroup := int64(1111)
			supGroup := int64(2222)

			err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				_, err := oc.AsAdmin().Run("adm").Args("policy", "add-scc-to-user", "anyuid", oc.Username()).Output()
				if exitErr, ok := err.(*exutil.ExitError); ok {
					if strings.HasPrefix(exitErr.StdErr, "Error from server (Conflict):") {
						// the retry.RetryOnConflict expects "conflict" error, let's provide it with one
						return errors.NewConflict(schema.GroupResource{}, "", err)
					}
				}
				return err
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			// create a pod that is requesting supplemental groups.  We request specific sup groups
			// so that we can check for the exact values later and not rely on SCC allocation.
			g.By("creating a pod that requests supplemental groups")
			submittedPod := supGroupPod(fsGroup, supGroup)
			_, err = f.ClientSet.CoreV1().Pods(f.Namespace.Name).Create(submittedPod)
			o.Expect(err).NotTo(o.HaveOccurred())
			defer f.ClientSet.CoreV1().Pods(f.Namespace.Name).Delete(submittedPod.Name, nil)

			// we should have been admitted with the groups that we requested but if for any
			// reason they are different we will fail.
			g.By("retrieving the pod and ensuring groups are set")
			retrievedPod, err := f.ClientSet.CoreV1().Pods(f.Namespace.Name).Get(submittedPod.Name, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(*retrievedPod.Spec.SecurityContext.FSGroup).To(o.Equal(*submittedPod.Spec.SecurityContext.FSGroup))
			o.Expect(retrievedPod.Spec.SecurityContext.SupplementalGroups).To(o.Equal(submittedPod.Spec.SecurityContext.SupplementalGroups))

			// wait for the pod to run so we can inspect it.
			g.By("waiting for the pod to become running")
			err = f.WaitForPodRunning(submittedPod.Name)
			o.Expect(err).NotTo(o.HaveOccurred())

			out, stderr, err := oc.Run("exec").Args("-p", supplementalGroupsPod, "--", "/usr/bin/id", "-G").Outputs()
			if err != nil {
				logs, _ := oc.Run("logs").Args(supplementalGroupsPod).Output()
				e2e.Failf("Failed to get groups: \n%q, %q, pod logs: \n%q", out, stderr, logs)
			}
			o.Expect(err).NotTo(o.HaveOccurred())

			split := strings.Split(out, " ")
			o.Expect(split).ToNot(o.BeEmpty(), fmt.Sprintf("no groups in pod: %v", out))
			group := strconv.FormatInt(fsGroup, 10)
			o.Expect(split).To(o.ContainElement(group), fmt.Sprintf("fsGroup %v should exist in pod's groups: %v", fsGroup, out))
			group = strconv.FormatInt(supGroup, 10)
			o.Expect(split).To(o.ContainElement(group), fmt.Sprintf("supGroup %v should exist in pod's groups: %v", supGroup, out))
		})

	})
})

// supGroupPod generates the pod requesting supplemental groups.
func supGroupPod(fsGroup int64, supGroup int64) *kapiv1.Pod {
	return &kapiv1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: supplementalGroupsPod,
		},
		Spec: kapiv1.PodSpec{
			SecurityContext: &kapiv1.PodSecurityContext{
				FSGroup:            &fsGroup,
				SupplementalGroups: []int64{supGroup},
			},
			Containers: []kapiv1.Container{
				{
					Name:  supplementalGroupsPod,
					Image: "openshift/origin-pod",
				},
			},
		},
	}
}
