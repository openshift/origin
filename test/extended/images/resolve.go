package images

import (
	"context"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	appsv1 "github.com/openshift/api/apps/v1"
	configv1 "github.com/openshift/api/config/v1"
	kappsv1 "k8s.io/api/apps/v1"
	kbatchv1 "k8s.io/api/batch/v1"
	kapiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	k8simage "k8s.io/kubernetes/test/utils/image"
	admissionapi "k8s.io/pod-security-admission/api"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-imageregistry][Feature:ImageLookup] Image policy", func() {
	defer g.GinkgoRecover()
	var oc = exutil.NewCLIWithPodSecurityLevel("resolve-local-names", admissionapi.LevelBaseline)
	one := int64(0)
	ctx := context.Background()

	g.It("should update standard Kube object image fields when local names are on [apigroup:image.openshift.io]", g.Label("Size:L"), func() {
		hasImageRegistry, err := exutil.IsCapabilityEnabled(oc, configv1.ClusterVersionCapabilityImageRegistry)
		o.Expect(err).NotTo(o.HaveOccurred())

		err = oc.Run("tag").Args(k8simage.GetE2EImage(k8simage.BusyBox), "busybox:latest").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.Run("set", "image-lookup").Args("busybox").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		is, err := oc.ImageClient().ImageV1().ImageStreams(oc.Namespace()).Get(ctx, "busybox", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(is.Spec.LookupPolicy.Local).To(o.BeTrue())

		var internalImageReference string
		var lastErr error
		err = wait.PollImmediate(5*time.Second, 2*time.Minute, func() (bool, error) {
			tag, err := oc.ImageClient().ImageV1().ImageTags(oc.Namespace()).Get(ctx, "busybox:latest", metav1.GetOptions{})
			if err != nil || tag.Image == nil {
				lastErr = err
				return false, nil
			}
			internalImageReference = tag.Image.DockerImageReference
			return true, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("unable to wait for image to be imported: %v", lastErr))

		// pods should auto replace local references
		pod, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).Create(ctx, &kapiv1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "resolve"},
			Spec: kapiv1.PodSpec{
				TerminationGracePeriodSeconds: &one,
				RestartPolicy:                 kapiv1.RestartPolicyNever,
				Containers: []kapiv1.Container{
					{Name: "resolve", Image: "busybox:latest", Command: []string{"/bin/true"}},
					{Name: "resolve2", Image: "busybox:unknown", Command: []string{"/bin/true"}},
				},
			},
		}, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		if pod.Spec.Containers[0].Image != internalImageReference {
			g.Skip("default image resolution is not configured, can't verify pod resolution")
		}

		// When ImageRegistry is not present this should not get resolved and will remain
		// and Attempt resolve meaning it will not get changed by the admission plugin
		unknownImageSuffix := "busybox:unknown"
		if hasImageRegistry {
			unknownImageSuffix = "/" + oc.Namespace() + "/" + unknownImageSuffix
		}

		o.Expect(pod.Spec.Containers[0].Image).To(o.Equal(internalImageReference))
		o.Expect(pod.Spec.Containers[1].Image).To(o.HaveSuffix(unknownImageSuffix))
		defer func() { oc.KubeClient().CoreV1().Pods(oc.Namespace()).Delete(ctx, pod.Name, metav1.DeleteOptions{}) }()

		// replica sets should auto replace local references
		rs, err := oc.KubeClient().AppsV1().ReplicaSets(oc.Namespace()).Create(ctx, &kappsv1.ReplicaSet{
			ObjectMeta: metav1.ObjectMeta{Name: "resolve"},
			Spec: kappsv1.ReplicaSetSpec{
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
		}, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(rs.Spec.Template.Spec.Containers[0].Image).To(o.Equal(internalImageReference))
		defer func() {
			oc.KubeClient().AppsV1().ReplicaSets(oc.Namespace()).Delete(ctx, rs.Name, metav1.DeleteOptions{})
		}()

		// replica sets should auto replace local references on update
		rs, err = oc.KubeClient().AppsV1().ReplicaSets(oc.Namespace()).Patch(context.Background(), rs.Name, types.StrategicMergePatchType, []byte(`{"spec": {"template": {"spec": {"containers": [{"name": "resolve", "image": "busybox:latest"}]}}}}`), metav1.PatchOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(rs.Spec.Template.Spec.Containers[0].Image).To(o.Equal(internalImageReference))

		// daemon sets should auto replace local references
		ds, err := oc.KubeClient().AppsV1().DaemonSets(oc.Namespace()).Create(ctx, &kappsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{Name: "resolve"},
			Spec: kappsv1.DaemonSetSpec{
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
		}, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(ds.Spec.Template.Spec.Containers[0].Image).To(o.Equal(internalImageReference))
		defer func() {
			oc.KubeClient().AppsV1().DaemonSets(oc.Namespace()).Delete(ctx, ds.Name, metav1.DeleteOptions{})
		}()

		// daemon sets should auto replace local references on update
		ds, err = oc.KubeClient().AppsV1().DaemonSets(oc.Namespace()).Patch(context.Background(), ds.Name, types.StrategicMergePatchType, []byte(`{"spec": {"template": {"spec": {"containers": [{"name": "resolve", "image": "busybox:latest"}]}}}}`), metav1.PatchOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(ds.Spec.Template.Spec.Containers[0].Image).To(o.Equal(internalImageReference))

		// stateful sets should auto replace local references
		sts, err := oc.KubeClient().AppsV1().StatefulSets(oc.Namespace()).Create(ctx, &kappsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{Name: "resolve"},
			Spec: kappsv1.StatefulSetSpec{
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
		}, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sts.Spec.Template.Spec.Containers[0].Image).To(o.Equal(internalImageReference))
		defer func() {
			oc.KubeClient().AppsV1().StatefulSets(oc.Namespace()).Delete(ctx, sts.Name, metav1.DeleteOptions{})
		}()

		// stateful sets should auto replace local references on update
		sts, err = oc.KubeClient().AppsV1().StatefulSets(oc.Namespace()).Patch(context.Background(), sts.Name, types.StrategicMergePatchType, []byte(`{"spec": {"template": {"spec": {"containers": [{"name": "resolve", "image": "busybox:latest"}]}}}}`), metav1.PatchOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sts.Spec.Template.Spec.Containers[0].Image).To(o.Equal(internalImageReference))

		// deployments should auto replace local references
		dc, err := oc.AdminKubeClient().AppsV1().Deployments(oc.Namespace()).Create(ctx, &kappsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: "resolve"},
			Spec: kappsv1.DeploymentSpec{
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
		}, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(dc.Spec.Template.Spec.Containers[0].Image).To(o.Equal(internalImageReference))
		defer func() {
			oc.AdminKubeClient().AppsV1().Deployments(oc.Namespace()).Delete(ctx, dc.Name, metav1.DeleteOptions{})
		}()

		dc, err = oc.AdminKubeClient().AppsV1().Deployments(oc.Namespace()).Patch(context.Background(), dc.Name, types.StrategicMergePatchType, []byte(`{"spec": {"template": {"spec": {"containers": [{"name": "resolve", "image": "busybox:latest"}]}}}}`), metav1.PatchOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(dc.Spec.Template.Spec.Containers[0].Image).To(o.Equal(internalImageReference))
	})

	g.It("should update OpenShift object image fields when local names are on [apigroup:image.openshift.io][apigroup:apps.openshift.io]", g.Label("Size:L"), func() {
		err := oc.Run("tag").Args(k8simage.GetE2EImage(k8simage.BusyBox), "busybox:latest").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.Run("set", "image-lookup").Args("busybox").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		is, err := oc.ImageClient().ImageV1().ImageStreams(oc.Namespace()).Get(ctx, "busybox", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(is.Spec.LookupPolicy.Local).To(o.BeTrue())

		var internalImageReference string
		var lastErr error
		err = wait.PollImmediate(5*time.Second, 2*time.Minute, func() (bool, error) {
			tag, err := oc.ImageClient().ImageV1().ImageTags(oc.Namespace()).Get(ctx, "busybox:latest", metav1.GetOptions{})
			if err != nil || tag.Image == nil {
				lastErr = err
				return false, nil
			}
			internalImageReference = tag.Image.DockerImageReference
			return true, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("unable to wait for image to be imported: %v", lastErr))

		// deployment configs should auto replace local references
		dc, err := oc.AppsClient().AppsV1().DeploymentConfigs(oc.Namespace()).Create(ctx, &appsv1.DeploymentConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "resolve"},
			Spec: appsv1.DeploymentConfigSpec{
				Selector: map[string]string{"resolve": "true"},
				Template: &kapiv1.PodTemplateSpec{
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
		}, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(dc.Spec.Template.Spec.Containers[0].Image).To(o.Equal(internalImageReference))
		defer func() {
			oc.AppsClient().AppsV1().DeploymentConfigs(oc.Namespace()).Delete(ctx, dc.Name, metav1.DeleteOptions{})
		}()

		dc, err = oc.AppsClient().AppsV1().DeploymentConfigs(oc.Namespace()).Patch(context.Background(), dc.Name, types.StrategicMergePatchType, []byte(`{"spec": {"template": {"spec": {"containers": [{"name": "resolve", "image": "busybox:latest"}]}}}}`), metav1.PatchOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(dc.Spec.Template.Spec.Containers[0].Image).To(o.Equal(internalImageReference))
	})

	g.It("should perform lookup when the object has the resolve-names annotation [apigroup:image.openshift.io]", g.Label("Size:L"), func() {
		hasImageRegistry, err := exutil.IsCapabilityEnabled(oc, configv1.ClusterVersionCapabilityImageRegistry)
		o.Expect(err).NotTo(o.HaveOccurred())

		err = oc.Run("tag").Args(k8simage.GetE2EImage(k8simage.BusyBox), "busybox:latest").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		var internalImageReference string
		var lastErr error
		err = wait.PollImmediate(5*time.Second, 2*time.Minute, func() (bool, error) {
			tag, err := oc.ImageClient().ImageV1().ImageTags(oc.Namespace()).Get(ctx, "busybox:latest", metav1.GetOptions{})
			if err != nil || tag.Image == nil {
				lastErr = err
				return false, nil
			}
			internalImageReference = tag.Image.DockerImageReference
			return true, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("unable to wait for image to be imported: %v", lastErr))

		g.By("auto replacing local references on Pods")
		pod, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).Create(ctx, &kapiv1.Pod{
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
		}, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		if pod.Spec.Containers[0].Image != internalImageReference {
			g.Skip("default image resolution is not configured, can't verify pod resolution")
		}

		// When ImageRegistry is not present this should not get resolved and will remain
		// and Attempt resolve meaning it will not get changed by the admission plugin
		unknownImageSuffix := "busybox:unknown"
		if hasImageRegistry {
			unknownImageSuffix = "/" + oc.Namespace() + "/" + unknownImageSuffix
		}

		o.Expect(pod.Spec.Containers[0].Image).To(o.Equal(internalImageReference))
		o.Expect(pod.Spec.Containers[1].Image).To(o.HaveSuffix(unknownImageSuffix))

		g.By("auto replacing local references on ReplicaSets")
		rs, err := oc.KubeClient().AppsV1().ReplicaSets(oc.Namespace()).Create(ctx, &kappsv1.ReplicaSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "resolve",
				Annotations: map[string]string{
					"alpha.image.policy.openshift.io/resolve-names": "*",
				},
			},
			Spec: kappsv1.ReplicaSetSpec{
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
		}, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(rs.Spec.Template.Spec.Containers[0].Image).To(o.Equal(internalImageReference))

		g.By("auto replacing local references on Deployments")
		deployment, err := oc.KubeClient().AppsV1().Deployments(oc.Namespace()).Create(ctx, &kappsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name: "resolve",
				Annotations: map[string]string{
					"alpha.image.policy.openshift.io/resolve-names": "*",
				},
			},
			Spec: kappsv1.DeploymentSpec{
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
		}, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(deployment.Spec.Template.Spec.Containers[0].Image).To(o.Equal(internalImageReference))

		g.By("auto replacing local references on Deployments (in pod template)")
		deployment, err = oc.KubeClient().AppsV1().Deployments(oc.Namespace()).Create(ctx, &kappsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name: "resolve-template",
			},
			Spec: kappsv1.DeploymentSpec{
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
		}, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(deployment.Spec.Template.Spec.Containers[0].Image).To(o.Equal(internalImageReference))

		g.By("auto replacing local references on DaemonSets")
		daemonset, err := oc.KubeClient().AppsV1().DaemonSets(oc.Namespace()).Create(ctx, &kappsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "resolve",
				Annotations: map[string]string{
					"alpha.image.policy.openshift.io/resolve-names": "*",
				},
			},
			Spec: kappsv1.DaemonSetSpec{
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
		}, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(daemonset.Spec.Template.Spec.Containers[0].Image).To(o.Equal(internalImageReference))

		g.By("auto replacing local references on DaemonSets (in pod template)")
		daemonset, err = oc.KubeClient().AppsV1().DaemonSets(oc.Namespace()).Create(ctx, &kappsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "resolve-template",
			},
			Spec: kappsv1.DaemonSetSpec{
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
		}, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(daemonset.Spec.Template.Spec.Containers[0].Image).To(o.Equal(internalImageReference))

		g.By("auto replacing local references on StatefulSets")
		statefulset, err := oc.KubeClient().AppsV1().StatefulSets(oc.Namespace()).Create(ctx, &kappsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "resolve",
				Annotations: map[string]string{
					"alpha.image.policy.openshift.io/resolve-names": "*",
				},
			},
			Spec: kappsv1.StatefulSetSpec{
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
		}, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(statefulset.Spec.Template.Spec.Containers[0].Image).To(o.Equal(internalImageReference))

		g.By("auto replacing local references on StatefulSets (in pod template)")
		statefulset, err = oc.KubeClient().AppsV1().StatefulSets(oc.Namespace()).Create(ctx, &kappsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "resolve-template",
			},
			Spec: kappsv1.StatefulSetSpec{
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
		}, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(statefulset.Spec.Template.Spec.Containers[0].Image).To(o.Equal(internalImageReference))

		g.By("auto replacing local references on Jobs")
		job, err := oc.KubeClient().BatchV1().Jobs(oc.Namespace()).Create(ctx, &kbatchv1.Job{
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
		}, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(job.Spec.Template.Spec.Containers[0].Image).To(o.Equal(internalImageReference))

		g.By("auto replacing local references on Jobs (in pod template)")
		job, err = oc.KubeClient().BatchV1().Jobs(oc.Namespace()).Create(ctx, &kbatchv1.Job{
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
		}, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(job.Spec.Template.Spec.Containers[0].Image).To(o.Equal(internalImageReference))

		g.By("auto replacing local references on CronJobs")
		cronjob, err := oc.KubeClient().BatchV1().CronJobs(oc.Namespace()).Create(ctx, &kbatchv1.CronJob{
			ObjectMeta: metav1.ObjectMeta{
				Name: "resolve",
				Annotations: map[string]string{
					"alpha.image.policy.openshift.io/resolve-names": "*",
				},
			},
			Spec: kbatchv1.CronJobSpec{
				Schedule: "1 0 * * *",
				JobTemplate: kbatchv1.JobTemplateSpec{
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
		}, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(cronjob.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Image).To(o.Equal(internalImageReference))

		g.By("auto replacing local references on CronJobs (in pod template)")
		cronjob, err = oc.KubeClient().BatchV1().CronJobs(oc.Namespace()).Create(ctx, &kbatchv1.CronJob{
			ObjectMeta: metav1.ObjectMeta{
				Name: "resolve-template",
			},
			Spec: kbatchv1.CronJobSpec{
				Schedule: "1 0 * * *",
				JobTemplate: kbatchv1.JobTemplateSpec{
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
		}, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(cronjob.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Image).To(o.Equal(internalImageReference))
	})

	g.It("should perform lookup when the Deployment gets the resolve-names annotation later [apigroup:image.openshift.io]", g.Label("Size:M"), func() {
		imageReference := k8simage.GetE2EImage(k8simage.BusyBox)
		err := oc.Run("tag").Args(imageReference, "busybox:latest").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		var internalImageReference string
		var lastErr error
		err = wait.PollImmediate(5*time.Second, 2*time.Minute, func() (bool, error) {
			tag, err := oc.ImageClient().ImageV1().ImageTags(oc.Namespace()).Get(ctx, "busybox:latest", metav1.GetOptions{})
			if err != nil || tag.Image == nil {
				lastErr = err
				return false, nil
			}
			internalImageReference = tag.Image.DockerImageReference
			return true, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("unable to wait for image to be imported: %v", lastErr))

		deployment, err := oc.KubeClient().AppsV1().Deployments(oc.Namespace()).Create(ctx, &kappsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name: "resolve",
			},
			Spec: kappsv1.DeploymentSpec{
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
		}, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(deployment.Spec.Template.Spec.Containers[0].Image).To(o.Equal("busybox:latest"))

		g.By("auto replacing local references on Deployments when the annotation is added")
		if deployment.Annotations == nil {
			deployment.Annotations = map[string]string{}
		}
		deployment.Annotations["alpha.image.policy.openshift.io/resolve-names"] = "*"
		deployment, err = oc.KubeClient().AppsV1().Deployments(deployment.Namespace).Patch(ctx, deployment.Name, types.StrategicMergePatchType, []byte(`{"metadata": {"annotations": {"alpha.image.policy.openshift.io/resolve-names": "*"}}}`), metav1.PatchOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(deployment.ObjectMeta.Annotations["alpha.image.policy.openshift.io/resolve-names"]).To(o.Equal("*"))
		o.Expect(deployment.Spec.Template.Spec.Containers[0].Image).To(o.Equal(internalImageReference))
	})
})
