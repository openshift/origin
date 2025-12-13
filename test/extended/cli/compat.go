package cli

import (
	"context"
	"time"

	"github.com/MakeNowJust/heredoc"
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	admissionapi "k8s.io/pod-security-admission/api"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-cli] oc", func() {
	defer g.GinkgoRecover()

	var oc *exutil.CLI
	var ns string

	g.AfterEach(func() {
		if g.CurrentSpecReport().Failed() && len(ns) > 0 {
			exutil.DumpPodLogsStartingWithInNamespace("", ns, oc)
		}
	})

	oc = exutil.NewCLIWithPodSecurityLevel("cli", admissionapi.LevelBaseline)

	g.It("can run inside of a busybox container [apigroup:image.openshift.io]", g.Label("Size:M"), func() {
		ns := oc.Namespace()
		cli := e2epod.PodClientNS(oc.KubeFramework(), ns)

		_, err := oc.KubeClient().RbacV1().RoleBindings(ns).Create(context.Background(), &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "edit-for-builder"},
			RoleRef:    rbacv1.RoleRef{Kind: "ClusterRole", Name: "edit"},
			Subjects: []rbacv1.Subject{
				{Kind: "ServiceAccount", Name: "builder"},
			},
		}, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		pod := cli.Create(context.TODO(), newShellPod(heredoc.Docf(`
			set -x

			# verify we can make API calls
			oc get secrets
			oc whoami
		`)))
		cli.WaitForSuccess(context.TODO(), pod.Name, 5*time.Minute)
	})

	g.It("can get list of nodes", g.Label("Size:S"), func() {
		oc := oc.AsAdmin()
		err := oc.Run("get").Args("nodes").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
	})
})
