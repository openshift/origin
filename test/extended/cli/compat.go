package cli

import (
	"context"
	"time"

	"github.com/MakeNowJust/heredoc"
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	kapiv1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	exutil "github.com/openshift/origin/test/extended/util"
)

func cliPod(cli *exutil.CLI, shell string) *kapiv1.Pod {
	cliImage, _ := exutil.FindCLIImage(cli)

	return &kapiv1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cli-test",
		},
		Spec: kapiv1.PodSpec{
			RestartPolicy: kapiv1.RestartPolicyNever,
			Containers: []kapiv1.Container{
				{
					Name:    "test",
					Image:   cliImage,
					Command: []string{"/bin/bash", "-c", "set -euo pipefail; " + shell},
					Env: []kapiv1.EnvVar{
						{
							Name:  "HOME",
							Value: "/tmp",
						},
					},
				},
			},
		},
	}
}

func cliPodWithImage(cli *exutil.CLI, image string, shell string) *kapiv1.Pod {
	cliImage, _ := exutil.FindCLIImage(cli)

	return &kapiv1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cli-test",
		},
		Spec: kapiv1.PodSpec{
			ServiceAccountName: "builder",
			RestartPolicy:      kapiv1.RestartPolicyNever,
			Volumes: []kapiv1.Volume{
				{
					Name: "bin",
					VolumeSource: kapiv1.VolumeSource{
						EmptyDir: &kapiv1.EmptyDirVolumeSource{},
					},
				},
			},
			InitContainers: []kapiv1.Container{
				{
					Name:    "cli",
					Image:   cliImage,
					Command: []string{"/bin/bash", "-c", "set -e; cp /usr/bin/oc /tmp/bin/"},
					Env: []kapiv1.EnvVar{
						{
							Name:  "HOME",
							Value: "/tmp",
						},
					},
					VolumeMounts: []kapiv1.VolumeMount{
						{
							Name:      "bin",
							MountPath: "/tmp/bin",
						},
					},
				},
			},
			Containers: []kapiv1.Container{
				{
					Name:    "test",
					Image:   cliImage,
					Command: []string{"/bin/bash", "-c", "set -euo pipefail; export PATH=/tmp/bin:$PATH; " + shell},
					Env: []kapiv1.EnvVar{
						{
							Name:  "HOME",
							Value: "/tmp",
						},
					},
					VolumeMounts: []kapiv1.VolumeMount{
						{
							Name:      "bin",
							MountPath: "/tmp/bin",
						},
					},
				},
			},
		},
	}
}

var _ = g.Describe("[sig-cli] CLI", func() {
	defer g.GinkgoRecover()

	var oc *exutil.CLI
	var ns string

	g.AfterEach(func() {
		if g.CurrentGinkgoTestDescription().Failed && len(ns) > 0 {
			exutil.DumpPodLogsStartingWithInNamespace("", ns, oc)
		}
	})

	oc = exutil.NewCLI("cli")

	g.It("can run inside of a busybox container", func() {
		ns = oc.Namespace()
		cli := oc.KubeFramework().PodClient()

		_, err := oc.KubeClient().RbacV1().RoleBindings(ns).Create(context.Background(), &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "edit-for-builder"},
			RoleRef:    rbacv1.RoleRef{Kind: "ClusterRole", Name: "edit"},
			Subjects: []rbacv1.Subject{
				{Kind: "ServiceAccount", Name: "builder"},
			},
		}, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		pod := cli.Create(cliPodWithImage(oc, "busybox:latest", heredoc.Docf(`
			set -x

			# verify we can make API calls
			oc get secrets
			oc whoami
		`)))
		cli.WaitForSuccess(pod.Name, 5*time.Minute)
	})
})
