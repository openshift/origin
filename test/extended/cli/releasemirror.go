package cli

import (
	"time"

	"github.com/MakeNowJust/heredoc"
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	exutil "github.com/openshift/origin/test/extended/util"
	configclientv1 "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
)

const (
	imageStreamName        = "release-mirror"
	imageStreamTagName     = "release"
	imageStreamNameTagName = imageStreamName + ":" + imageStreamTagName
	podTimeout             = 5 * time.Minute
)

var _ = g.Describe("[cli] oc adm release mirror", func() {
	defer g.GinkgoRecover()
	var ns string
	var oc *exutil.CLI
	g.AfterEach(func() {
		if g.CurrentGinkgoTestDescription().Failed && len(ns) > 0 {
			exutil.DumpPodLogsStartingWithInNamespace("", ns, oc)
		}
	})
	oc = exutil.NewCLI("oc-adm-release-mirror", exutil.KubeConfigPath()).AsAdmin()

	g.It("runs successfully", func() {
		ns = oc.Namespace()
		cli := oc.KubeFramework().PodClient()

		//Get ClusterVersion from the cluster running this job
		clusterVersion, err := configclientv1.NewForConfigOrDie(oc.AdminConfig()).ClusterVersions().Get("version", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		//Get the image url for the current cluster
		image := clusterVersion.Status.Desired.Image

		// Grab the pull secret for the registry used to create the cluster
		imagePullSecret, err := oc.KubeFramework().ClientSet.CoreV1().Secrets("openshift-config").Get("pull-secret", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		//adm release mirror requires permissions to ImageStreamImage
		_, err = oc.KubeClient().RbacV1().RoleBindings(ns).Create(&rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "edit-for-builder"},
			RoleRef:    rbacv1.RoleRef{Kind: "ClusterRole", Name: "edit"},
			Subjects: []rbacv1.Subject{
				{Kind: "ServiceAccount", Name: "builder"},
			},
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		pod := cli.Create(cliPodWithPullSecret(oc, imagePullSecret.Data[".dockerconfigjson"], heredoc.Docf(`
				set -x
				oc create imagestream %[2]s
				oc adm release mirror --from="%[1]s" --to-image-stream="%[2]s" -a /secret/.dockerconfigjson
			`, image, imageStreamName)))

		cli.WaitForSuccess(pod.Name, podTimeout)

		//Get the ImageStreamTag object of the created ImageStream release mirror
		istag, err := oc.ImageClient().ImageV1().ImageStreamTags(ns).Get(imageStreamNameTagName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		//Confirm that the ImageStreamTag.Tag.From.Name is equal to the current cluster desired image
		o.Expect(istag.Tag.From.Name).To(o.Equal(image))
	})
})

func generateSecret(name string, ns string, data []byte) *v1.Secret {
	return &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Data: map[string][]byte{
			v1.DockerConfigJsonKey: data,
		},
	}
}

func cliPodWithPullSecret(cli *exutil.CLI, secret []byte, shell string) *v1.Pod {
	var pullSecretName = "pull-secret"
	pullSecret := generateSecret(pullSecretName, cli.Namespace(), secret)

	_, err := cli.KubeClient().CoreV1().Secrets(cli.Namespace()).Create(pullSecret)
	o.Expect(err).NotTo(o.HaveOccurred())

	cliImage, _ := exutil.FindCLIImage(cli)

	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pod-with-pull-secret-test",
		},
		Spec: v1.PodSpec{
			// so we have permission to push and pull to the registry
			ServiceAccountName: "builder",
			RestartPolicy:      v1.RestartPolicyNever,
			Containers: []v1.Container{
				{
					Name:    "test",
					Image:   cliImage,
					Command: []string{"/bin/bash", "-c", "set -euo pipefail; " + shell},
					Env: []v1.EnvVar{
						{
							Name:  "HOME",
							Value: "/secret",
						},
					},
					VolumeMounts: []v1.VolumeMount{
						{
							Name:      "pull-secret",
							MountPath: "/secret/.dockerconfigjson",
							SubPath:   v1.DockerConfigJsonKey,
						},
					},
				},
			},
			Volumes: []v1.Volume{
				{
					Name: "pull-secret",
					VolumeSource: v1.VolumeSource{
						Secret: &v1.SecretVolumeSource{
							SecretName: pullSecretName,
						},
					},
				},
			},
		},
	}
}
