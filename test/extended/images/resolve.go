package images

import (
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	kbatchv1 "k8s.io/api/batch/v1"
	kbatchv1beta1 "k8s.io/api/batch/v1beta1"
	kapiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Feature:ImageLookup][registry][Conformance] Image policy", func() {
	defer g.GinkgoRecover()
	var oc = exutil.NewCLI("resolve-local-names", exutil.KubeConfigPath())
	one := int64(0)

	g.It("should update standard Kube object image fields when local names are on", func() {
		err := oc.Run("import-image").Args("busybox:latest", "--confirm").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.Run("set", "image-lookup").Args("busybox").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		tag, err := oc.ImageClient().ImageV1().ImageStreamTags(oc.Namespace()).Get("busybox:latest", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(tag.LookupPolicy.Local).To(o.BeTrue())

		// pods should auto replace local references
		pod, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).Create(&kapiv1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "resolve"},
			Spec: kapiv1.PodSpec{
				TerminationGracePeriodSeconds: &one,
				RestartPolicy:                 kapiv1.RestartPolicyNever,
				Containers: []kapiv1.Container{
					{Name: "resolve", Image: "busybox:latest", Command: []string{"/bin/true"}},
					{Name: "resolve2", Image: "busybox:unknown", Command: []string{"/bin/true"}},
				},
			},
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		if pod.Spec.Containers[0].Image != tag.Image.DockerImageReference {
			g.Skip("default image resolution is not configured, can't verify pod resolution")
		}

		o.Expect(pod.Spec.Containers[0].Image).To(o.Equal(tag.Image.DockerImageReference))
		o.Expect(pod.Spec.Containers[1].Image).To(o.HaveSuffix("/" + oc.Namespace() + "/busybox:unknown"))
		defer func() { oc.KubeClient().CoreV1().Pods(oc.Namespace()).Delete(pod.Name, nil) }()

		// replica sets should auto replace local references
		rs, err := oc.KubeClient().AppsV1().ReplicaSets(oc.Namespace()).Create(&appsv1.ReplicaSet{
			ObjectMeta: metav1.ObjectMeta{Name: "resolve"},
			Spec: appsv1.ReplicaSetSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"resolve": "true"},
				},
				Template: kapiv1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{"resolve": "true"},
					},
					Spec: kapiv1.PodSpec{
						TerminationGracePeriodSeconds: &one,
						Containers: []kapiv1.Container{
							{Name: "resolve", Image: "busybox:latest", Command: []string{"/bin/true"}},
						},
					},
				},
			},
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(rs.Spec.Template.Spec.Containers[0].Image).To(o.Equal(tag.Image.DockerImageReference))
		defer func() { oc.KubeClient().AppsV1().ReplicaSets(oc.Namespace()).Delete(rs.Name, nil) }()
	})

	g.It("should perform lookup when the object has the resolve-names annotation", func() {
		err := oc.Run("import-image").Args("busybox:latest", "--confirm").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		tag, err := oc.ImageClient().ImageV1().ImageStreamTags(oc.Namespace()).Get("busybox:latest", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("auto replacing local references on Pods")
		pod, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).Create(&kapiv1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: "resolve",
				Annotations: map[string]string{
					"alpha.image.policy.openshift.io/resolve-names": "*",
				},
			},
			Spec: kapiv1.PodSpec{
				TerminationGracePeriodSeconds: &one,
				RestartPolicy:                 kapiv1.RestartPolicyNever,
				Containers: []kapiv1.Container{
					{Name: "resolve", Image: "busybox:latest", Command: []string{"/bin/true"}},
					{Name: "resolve2", Image: "busybox:unknown", Command: []string{"/bin/true"}},
				},
			},
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		if pod.Spec.Containers[0].Image != tag.Image.DockerImageReference {
			g.Skip("default image resolution is not configured, can't verify pod resolution")
		}

		o.Expect(pod.Spec.Containers[0].Image).To(o.Equal(tag.Image.DockerImageReference))
		o.Expect(pod.Spec.Containers[1].Image).To(o.HaveSuffix("/" + oc.Namespace() + "/busybox:unknown"))

		g.By("auto replacing local references on ReplicaSets")
		rs, err := oc.KubeClient().AppsV1().ReplicaSets(oc.Namespace()).Create(&appsv1.ReplicaSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "resolve",
				Annotations: map[string]string{
					"alpha.image.policy.openshift.io/resolve-names": "*",
				},
			},
			Spec: appsv1.ReplicaSetSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"resolve": "true"},
				},
				Template: kapiv1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"resolve": "true"}},
					Spec: kapiv1.PodSpec{
						TerminationGracePeriodSeconds: &one,
						Containers: []kapiv1.Container{
							{Name: "resolve", Image: "busybox:latest", Command: []string{"/bin/true"}},
						},
					},
				},
			},
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(rs.Spec.Template.Spec.Containers[0].Image).To(o.Equal(tag.Image.DockerImageReference))

		g.By("auto replacing local references on Deployments")
		deployment, err := oc.KubeClient().AppsV1().Deployments(oc.Namespace()).Create(&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name: "resolve",
				Annotations: map[string]string{
					"alpha.image.policy.openshift.io/resolve-names": "*",
				},
			},
			Spec: appsv1.DeploymentSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"resolve": "true"},
				},
				Template: kapiv1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"resolve": "true"}},
					Spec: kapiv1.PodSpec{
						TerminationGracePeriodSeconds: &one,
						Containers: []kapiv1.Container{
							{Name: "resolve", Image: "busybox:latest", Command: []string{"/bin/true"}},
						},
					},
				},
			},
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(deployment.Spec.Template.Spec.Containers[0].Image).To(o.Equal(tag.Image.DockerImageReference))

		g.By("auto replacing local references on Deployments (in pod template)")
		deployment, err = oc.KubeClient().AppsV1().Deployments(oc.Namespace()).Create(&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name: "resolve-template",
			},
			Spec: appsv1.DeploymentSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"resolve-template": "true"},
				},
				Template: kapiv1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{"resolve-template": "true"},
						Annotations: map[string]string{
							"alpha.image.policy.openshift.io/resolve-names": "*",
						},
					},
					Spec: kapiv1.PodSpec{
						TerminationGracePeriodSeconds: &one,
						Containers: []kapiv1.Container{
							{Name: "resolve", Image: "busybox:latest", Command: []string{"/bin/true"}},
						},
					},
				},
			},
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(deployment.Spec.Template.Spec.Containers[0].Image).To(o.Equal(tag.Image.DockerImageReference))

		g.By("auto replacing local references on DaemonSets")
		daemonset, err := oc.KubeClient().AppsV1().DaemonSets(oc.Namespace()).Create(&appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "resolve",
				Annotations: map[string]string{
					"alpha.image.policy.openshift.io/resolve-names": "*",
				},
			},
			Spec: appsv1.DaemonSetSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"resolve": "true"},
				},
				Template: kapiv1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{"resolve": "true"},
					},
					Spec: kapiv1.PodSpec{
						TerminationGracePeriodSeconds: &one,
						Containers: []kapiv1.Container{
							{Name: "resolve", Image: "busybox:latest", Command: []string{"/bin/true"}},
						},
					},
				},
			},
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(daemonset.Spec.Template.Spec.Containers[0].Image).To(o.Equal(tag.Image.DockerImageReference))

		g.By("auto replacing local references on DaemonSets (in pod template)")
		daemonset, err = oc.KubeClient().AppsV1().DaemonSets(oc.Namespace()).Create(&appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "resolve-template",
			},
			Spec: appsv1.DaemonSetSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"resolve-template": "true"},
				},
				Template: kapiv1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{"resolve-template": "true"},
						Annotations: map[string]string{
							"alpha.image.policy.openshift.io/resolve-names": "*",
						},
					},
					Spec: kapiv1.PodSpec{
						TerminationGracePeriodSeconds: &one,
						Containers: []kapiv1.Container{
							{Name: "resolve", Image: "busybox:latest", Command: []string{"/bin/true"}},
						},
					},
				},
			},
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(daemonset.Spec.Template.Spec.Containers[0].Image).To(o.Equal(tag.Image.DockerImageReference))

		g.By("auto replacing local references on StatefulSets")
		statefulset, err := oc.KubeClient().AppsV1().StatefulSets(oc.Namespace()).Create(&appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "resolve",
				Annotations: map[string]string{
					"alpha.image.policy.openshift.io/resolve-names": "*",
				},
			},
			Spec: appsv1.StatefulSetSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"resolve": "true"},
				},
				Template: kapiv1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{"resolve": "true"},
					},
					Spec: kapiv1.PodSpec{
						TerminationGracePeriodSeconds: &one,
						Containers: []kapiv1.Container{
							{Name: "resolve", Image: "busybox:latest", Command: []string{"/bin/true"}},
						},
					},
				},
			},
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(statefulset.Spec.Template.Spec.Containers[0].Image).To(o.Equal(tag.Image.DockerImageReference))

		g.By("auto replacing local references on StatefulSets (in pod template)")
		statefulset, err = oc.KubeClient().AppsV1().StatefulSets(oc.Namespace()).Create(&appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "resolve-template",
			},
			Spec: appsv1.StatefulSetSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"resolve-template": "true"},
				},
				Template: kapiv1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{"resolve-template": "true"},
						Annotations: map[string]string{
							"alpha.image.policy.openshift.io/resolve-names": "*",
						},
					},
					Spec: kapiv1.PodSpec{
						TerminationGracePeriodSeconds: &one,
						Containers: []kapiv1.Container{
							{Name: "resolve", Image: "busybox:latest", Command: []string{"/bin/true"}},
						},
					},
				},
			},
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(statefulset.Spec.Template.Spec.Containers[0].Image).To(o.Equal(tag.Image.DockerImageReference))

		g.By("auto replacing local references on Jobs")
		job, err := oc.KubeClient().BatchV1().Jobs(oc.Namespace()).Create(&kbatchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name: "resolve",
				Annotations: map[string]string{
					"alpha.image.policy.openshift.io/resolve-names": "*",
				},
			},
			Spec: kbatchv1.JobSpec{
				Template: kapiv1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{"resolve": "true"},
					},
					Spec: kapiv1.PodSpec{
						TerminationGracePeriodSeconds: &one,
						RestartPolicy:                 kapiv1.RestartPolicyNever,
						Containers: []kapiv1.Container{
							{Name: "resolve", Image: "busybox:latest", Command: []string{"/bin/true"}},
						},
					},
				},
			},
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(job.Spec.Template.Spec.Containers[0].Image).To(o.Equal(tag.Image.DockerImageReference))

		g.By("auto replacing local references on Jobs (in pod template)")
		job, err = oc.KubeClient().BatchV1().Jobs(oc.Namespace()).Create(&kbatchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name: "resolve-template",
			},
			Spec: kbatchv1.JobSpec{
				Template: kapiv1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							"alpha.image.policy.openshift.io/resolve-names": "*",
						},
					},
					Spec: kapiv1.PodSpec{
						TerminationGracePeriodSeconds: &one,
						RestartPolicy:                 kapiv1.RestartPolicyNever,
						Containers: []kapiv1.Container{
							{Name: "resolve", Image: "busybox:latest", Command: []string{"/bin/true"}},
						},
					},
				},
			},
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(job.Spec.Template.Spec.Containers[0].Image).To(o.Equal(tag.Image.DockerImageReference))

		g.By("auto replacing local references on CronJobs")
		cronjob, err := oc.KubeClient().BatchV1beta1().CronJobs(oc.Namespace()).Create(&kbatchv1beta1.CronJob{
			ObjectMeta: metav1.ObjectMeta{
				Name: "resolve",
				Annotations: map[string]string{
					"alpha.image.policy.openshift.io/resolve-names": "*",
				},
			},
			Spec: kbatchv1beta1.CronJobSpec{
				Schedule: "1 0 * * *",
				JobTemplate: kbatchv1beta1.JobTemplateSpec{
					Spec: kbatchv1.JobSpec{
						Template: kapiv1.PodTemplateSpec{
							Spec: kapiv1.PodSpec{
								TerminationGracePeriodSeconds: &one,
								RestartPolicy:                 kapiv1.RestartPolicyNever,
								Containers: []kapiv1.Container{
									{Name: "resolve", Image: "busybox:latest", Command: []string{"/bin/true"}},
								},
							},
						},
					},
				},
			},
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(cronjob.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Image).To(o.Equal(tag.Image.DockerImageReference))

		g.By("auto replacing local references on CronJobs (in pod template)")
		cronjob, err = oc.KubeClient().BatchV1beta1().CronJobs(oc.Namespace()).Create(&kbatchv1beta1.CronJob{
			ObjectMeta: metav1.ObjectMeta{
				Name: "resolve-template",
			},
			Spec: kbatchv1beta1.CronJobSpec{
				Schedule: "1 0 * * *",
				JobTemplate: kbatchv1beta1.JobTemplateSpec{
					Spec: kbatchv1.JobSpec{
						Template: kapiv1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Annotations: map[string]string{
									"alpha.image.policy.openshift.io/resolve-names": "*",
								},
							},
							Spec: kapiv1.PodSpec{
								TerminationGracePeriodSeconds: &one,
								RestartPolicy:                 kapiv1.RestartPolicyNever,
								Containers: []kapiv1.Container{
									{Name: "resolve", Image: "busybox:latest", Command: []string{"/bin/true"}},
								},
							},
						},
					},
				},
			},
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(cronjob.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Image).To(o.Equal(tag.Image.DockerImageReference))
	})

	g.It("should perform lookup when the Deployment gets the resolve-names annotation later", func() {
		err := oc.Run("import-image").Args("busybox:latest", "--confirm").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		tag, err := oc.ImageClient().ImageV1().ImageStreamTags(oc.Namespace()).Get("busybox:latest", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		deployment, err := oc.KubeClient().AppsV1().Deployments(oc.Namespace()).Create(&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name: "resolve",
			},
			Spec: appsv1.DeploymentSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"resolve": "true"},
				},
				Template: kapiv1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{"resolve": "true"},
					},
					Spec: kapiv1.PodSpec{
						TerminationGracePeriodSeconds: &one,
						Containers: []kapiv1.Container{
							{Name: "resolve", Image: "busybox:latest", Command: []string{"/bin/true"}},
						},
					},
				},
			},
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(deployment.Spec.Template.Spec.Containers[0].Image).To(o.Equal("busybox:latest"))

		g.By("auto replacing local references on Deployments when the annotation is added")
		if deployment.Annotations == nil {
			deployment.Annotations = map[string]string{}
		}
		deployment.Annotations["alpha.image.policy.openshift.io/resolve-names"] = "*"
		deployment, err = oc.KubeClient().AppsV1().Deployments(deployment.Namespace).Patch(deployment.Name, types.StrategicMergePatchType, []byte(`{"metadata": {"annotations": {"alpha.image.policy.openshift.io/resolve-names": "*"}}}`))
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(deployment.ObjectMeta.Annotations["alpha.image.policy.openshift.io/resolve-names"]).To(o.Equal("*"))
		o.Expect(deployment.Spec.Template.Spec.Containers[0].Image).To(o.Equal(tag.Image.DockerImageReference))
	})
})
