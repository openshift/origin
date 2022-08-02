package jobs

import (
	"context"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/utils/image"
	admissionapi "k8s.io/pod-security-admission/api"
)

var _ = g.Describe("[sig-apps][Feature:Jobs]", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLIWithPodSecurityLevel("job-controller", admissionapi.LevelBaseline)

	g.It("Users should be able to create and run a job in a user project", func() {

		name := "simplev1"
		labels := fmt.Sprintf("app=%s", name)

		g.By("creating a job from...")
		_, err := oc.KubeClient().BatchV1().Jobs(oc.Namespace()).Create(context.Background(), createJob(name, oc.Namespace()), metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("waiting for a pod...")
		podNames, err := exutil.WaitForPods(oc.KubeClient().CoreV1().Pods(oc.Namespace()), exutil.ParseLabelsOrDie(labels), exutil.CheckPodIsSucceeded, 1, 3*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(len(podNames)).Should(o.Equal(1))

		g.By("waiting for a job...")
		err = exutil.WaitForAJob(oc.KubeClient().BatchV1().Jobs(oc.Namespace()), name, 2*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("checking job status...")
		jobs, err := oc.KubeClient().BatchV1().Jobs(oc.Namespace()).List(context.Background(), metav1.ListOptions{LabelSelector: exutil.ParseLabelsOrDie(labels).String()})
		o.Expect(err).NotTo(o.HaveOccurred())

		o.Expect(len(jobs.Items)).Should(o.Equal(1))
		job := jobs.Items[0]
		o.Expect(len(job.Status.Conditions)).Should(o.Equal(1))
		o.Expect(job.Status.Conditions[0].Type).Should(o.Equal(batchv1.JobComplete))

		g.By("removing a job...")
		err = oc.Run("delete").Args(fmt.Sprintf("job/%s", name)).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

	})
})

func createJob(name, namespace string) *batchv1.Job {
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
					Labels: map[string]string{
						"app": name,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:    "simplev1",
							Image:   image.GetE2EImage(image.Agnhost),
							Command: []string{"/bin/sh", "-c", "exit 0"},
						},
					},
					RestartPolicy: corev1.RestartPolicyNever,
				},
			},
		},
	}
}
