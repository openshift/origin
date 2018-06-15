package images

import (
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	kapiv1 "k8s.io/api/core/v1"
	kextensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Feature:ImageLookup][registry] Image policy", func() {
	defer g.GinkgoRecover()
	var oc = exutil.NewCLI("resolve-local-names", exutil.KubeConfigPath())
	one := int64(0)

	g.It("should update standard Kube object image fields when local names are on", func() {
		err := oc.Run("import-image").Args("busybox:latest", "--confirm").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.Run("set", "image-lookup").Args("busybox").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		tag, err := oc.ImageClient().Image().ImageStreamTags(oc.Namespace()).Get("busybox:latest", metav1.GetOptions{})
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
		rs, err := oc.KubeClient().ExtensionsV1beta1().ReplicaSets(oc.Namespace()).Create(&kextensionsv1beta1.ReplicaSet{
			ObjectMeta: metav1.ObjectMeta{Name: "resolve"},
			Spec: kextensionsv1beta1.ReplicaSetSpec{
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
		defer func() { oc.KubeClient().ExtensionsV1beta1().ReplicaSets(oc.Namespace()).Delete(rs.Name, nil) }()
	})

	g.It("should perform lookup when the pod has the resolve-names annotation", func() {
		err := oc.Run("import-image").Args("busybox:latest", "--confirm").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		tag, err := oc.ImageClient().Image().ImageStreamTags(oc.Namespace()).Get("busybox:latest", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		// pods should auto replace local references
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
		defer func() { oc.KubeClient().CoreV1().Pods(oc.Namespace()).Delete(pod.Name, nil) }()

		// replica sets should auto replace local references
		rs, err := oc.KubeClient().ExtensionsV1beta1().ReplicaSets(oc.Namespace()).Create(&kextensionsv1beta1.ReplicaSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "resolve",
				Annotations: map[string]string{
					"alpha.image.policy.openshift.io/resolve-names": "*",
				},
			},
			Spec: kextensionsv1beta1.ReplicaSetSpec{
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
		defer func() { oc.KubeClient().ExtensionsV1beta1().ReplicaSets(oc.Namespace()).Delete(rs.Name, nil) }()
	})
})
