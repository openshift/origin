package cli

import (
	"fmt"
	"io/ioutil"
	"time"

	"k8s.io/apimachinery/pkg/watch"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/kubernetes/pkg/kubectl/scheme"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/origin/test/extended/util"
)

func readPodFixture(path string) (*corev1.Pod, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	content, err := kyaml.ToJSON(data)
	if err != nil {
		return nil, err
	}
	obj, err := runtime.Decode(scheme.Codecs.UniversalDecoder(corev1.SchemeGroupVersion), content)
	if err != nil {
		return nil, err
	}
	return obj.(*corev1.Pod), err
}

var _ = g.Describe("[cli] oc logs", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLI("cli-deployment", exutil.KubeConfigPath())

	g.JustBeforeEach(func() {
		// FIXME: remove this when https://github.com/openshift/origin/issues/20225 gets fixed
		g.By("waiting for default service account")
		err := exutil.WaitForServiceAccount(oc.KubeClient().CoreV1().ServiceAccounts(oc.Namespace()), "default")
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	var (
		echoPodFixture = exutil.FixturePath("testdata", "cli", "echo-pod.yaml")
	)

	g.It("should get all logs with --follow if the pod is gonna be terminated right after", func() {
		namespace := oc.Namespace()
		podName := "test-pod"

		pod, err := readPodFixture(echoPodFixture)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(pod.Name).To(o.Equal(podName))
		o.Expect(pod.Spec.RestartPolicy).To(o.Equal(corev1.RestartPolicyNever))

		pod, err = oc.KubeClient().CoreV1().Pods(namespace).Create(pod)
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("Created the pod")

		w, err := oc.KubeClient().CoreV1().Pods(namespace).Watch(metav1.SingleObject(pod.ObjectMeta))
		o.Expect(err).NotTo(o.HaveOccurred())

		e2e.Logf("Waiting for the pod to become running")
		event, err := watch.Until(5*time.Minute, w, func(event watch.Event) (bool, error) {
			switch event.Type {
			case watch.Modified:
				phase := event.Object.(*corev1.Pod).Status.Phase
				switch phase {
				case corev1.PodRunning:
					return true, nil
				case corev1.PodPending:
					return false, nil
				default:
					return true, fmt.Errorf("unexpected pod phase: %q", phase)
				}
			default:
				return true, fmt.Errorf("unexpected event: %#v", event)
			}
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		pod = event.Object.(*corev1.Pod)
		e2e.Logf("Observed pod as running")

		logCh := make(chan struct{})
		var out string
		go func() {
			var err error
			out, err = oc.Run("logs").Args("--follow", "pod/"+podName).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf("oc logs finished")
			logCh <- struct{}{}
		}()

		w, err = oc.KubeClient().CoreV1().Pods(namespace).Watch(metav1.SingleObject(pod.ObjectMeta))
		o.Expect(err).NotTo(o.HaveOccurred())

		e2e.Logf("Waiting for the pod to finish")
		_, err = watch.Until(5*time.Minute, w, func(event watch.Event) (bool, error) {
			switch event.Type {
			case watch.Modified:
				return event.Object.(*corev1.Pod).Status.Phase == corev1.PodSucceeded, nil
			default:
				return true, fmt.Errorf("unexpected event: %#v", event)
			}
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		e2e.Logf("Deleting the pod right after it finished")
		gracePeriod := int64(0)
		err = oc.KubeClient().CoreV1().Pods(namespace).Delete(pod.Name, &metav1.DeleteOptions{
			GracePeriodSeconds: &gracePeriod,
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		<-logCh
		e2e.Logf("out: %#v", out)
		// TODO: investigate why the last \n is stripped
		o.Expect(out).To(o.BeEquivalentTo("line 1\nline 2\nline 3"))
	})
})
