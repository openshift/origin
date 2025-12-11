package security

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	kapiv1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/util/retry"
	rbacv1helpers "k8s.io/kubernetes/pkg/apis/rbac/v1"
	"k8s.io/kubernetes/test/e2e/framework"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	admissionapi "k8s.io/pod-security-admission/api"

	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/image"
)

const (
	supplementalGroupsPod = "supplemental-groups"
)

var _ = g.Describe("[sig-node] supplemental groups", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithPodSecurityLevel("sup-groups", admissionapi.LevelBaseline)
	ctx := context.Background()

	g.Describe("Ensure supplemental groups propagate to docker", func() {
		g.It("should propagate requested groups to the container [apigroup:security.openshift.io]", g.Label("Size:M"), func() {

			fsGroup := int64(1111)
			supGroup := int64(2222)

			projectName := oc.Namespace()
			sa := createServiceAccount(ctx, oc, projectName)
			supSubject := fmt.Sprintf("system:serviceaccount:%s:%s", projectName, sa.Name)
			createSupRoleOrDie(ctx, oc, sa)
			err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				// sa serves as a subject instead of the user
				_, err := oc.AsAdmin().Run("adm").Args("policy", "add-scc-to-user", "anyuid", "-z", sa.Name, "-n", projectName).Output()
				if exitErr, ok := err.(*exutil.ExitError); ok {
					if strings.HasPrefix(exitErr.StdErr, "Error from server (Conflict):") {
						// the retry.RetryOnConflict expects "conflict" error, let's provide it with one
						return errors.NewConflict(schema.GroupResource{}, "", err)
					}
				}
				return err
			})
			o.Expect(err).NotTo(o.HaveOccurred())
			supClient, _ := createClientFromServiceAccount(oc, sa)
			o.Expect(err).NotTo(o.HaveOccurred())

			// create a pod that is requesting supplemental groups.  We request specific sup groups
			// so that we can check for the exact values later and not rely on SCC allocation.
			g.By("creating a pod that requests supplemental groups")
			submittedPod := supGroupPod(fsGroup, supGroup)
			_, err = supClient.CoreV1().Pods(projectName).Create(context.Background(), submittedPod, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			defer supClient.CoreV1().Pods(projectName).Delete(context.Background(), submittedPod.Name, metav1.DeleteOptions{})

			// we should have been admitted with the groups that we requested but if for any
			// reason they are different we will fail.
			g.By("retrieving the pod and ensuring groups are set")
			retrievedPod, err := supClient.CoreV1().Pods(projectName).Get(context.Background(), submittedPod.Name, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(*retrievedPod.Spec.SecurityContext.FSGroup).To(o.Equal(*submittedPod.Spec.SecurityContext.FSGroup))
			o.Expect(retrievedPod.Spec.SecurityContext.SupplementalGroups).To(o.Equal(submittedPod.Spec.SecurityContext.SupplementalGroups))

			// wait for the pod to run, so we can inspect it.
			g.By("waiting for the pod to become running")
			err = e2epod.WaitForPodNameRunningInNamespace(context.TODO(), supClient, submittedPod.Name, projectName)
			o.Expect(err).NotTo(o.HaveOccurred())

			out, stderr, err := oc.Run("exec").Args(supplementalGroupsPod, "--as", supSubject, "--", "/usr/bin/id", "-G").Outputs()
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
					Name:            supplementalGroupsPod,
					Image:           image.ShellImage(),
					ImagePullPolicy: kapiv1.PullIfNotPresent,
					Command:         []string{"/bin/bash", "-c", "exec sleep infinity"},
				},
			},
		},
	}
}

func createSupRoleOrDie(ctx context.Context, oc *exutil.CLI, sa *kapiv1.ServiceAccount) {
	framework.Logf("Creating role")
	rule := rbacv1helpers.NewRule("get", "create", "update", "delete").Groups("").Resources("pods", "pods/exec").RuleOrDie()
	_, err := oc.AdminKubeClient().RbacV1().Roles(sa.Namespace).Create(
		ctx,
		&rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{Name: "supplemental-groups"},
			Rules:      []rbacv1.PolicyRule{rule},
		},
		metav1.CreateOptions{},
	)
	o.Expect(err).NotTo(o.HaveOccurred())

	framework.Logf("Creating rolebinding")
	_, err = oc.AdminKubeClient().RbacV1().RoleBindings(sa.Namespace).Create(ctx, &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    sa.Namespace,
			GenerateName: "supplemental-groups-",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind: "ServiceAccount",
				Name: sa.Name,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind: "Role",
			Name: "supplemental-groups",
		},
	}, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
}
