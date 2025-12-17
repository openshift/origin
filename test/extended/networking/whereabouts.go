package networking

import (
	"context"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	admissionapi "k8s.io/pod-security-admission/api"

	frameworkpod "k8s.io/kubernetes/test/e2e/framework/pod"
)

var _ = g.Describe("[sig-network][Feature:Whereabouts]", func() {

	oc := exutil.NewCLIWithPodSecurityLevel("whereabouts-e2e", admissionapi.LevelBaseline)

	// Whereabouts is already installed in Origin. These tests aims to verify the integrity of the installation.

	g.It("should use whereabouts net-attach-def to limit IP ranges for newly created pods [apigroup:k8s.cni.cncf.io]", g.Label("Size:M"), func() {
		var err error

		f := oc.KubeFramework()
		podName := "whereabouts-pod-"
		ns := f.Namespace.Name

		g.By("creating a whereabouts net-attach-def using bridgeCNI")
		nad_yaml := exutil.FixturePath("testdata", "net-attach-defs", "whereabouts-nad.yml")
		g.By(fmt.Sprintf("calling oc create -f %s", nad_yaml))
		err = oc.AsAdmin().Run("create").Args("-f", nad_yaml).Execute()
		o.Expect(err).NotTo(o.HaveOccurred(), "created net-attach-def")

		g.By("launching pods with annotations to use the net-attach-def")
		annotation := map[string]string{
			"k8s.v1.cni.cncf.io/networks": "wa-conf",
		}
		// First three pods should come up without issue.
		testPod := frameworkpod.CreateExecPodOrFail(context.TODO(), f.ClientSet, ns, podName, func(pod *v1.Pod) {
			pod.ObjectMeta.Annotations = annotation
		})
		testPod2 := frameworkpod.CreateExecPodOrFail(context.TODO(), f.ClientSet, ns, podName, func(pod *v1.Pod) {
			pod.ObjectMeta.Annotations = annotation
		})
		testPod3 := frameworkpod.CreateExecPodOrFail(context.TODO(), f.ClientSet, ns, podName, func(pod *v1.Pod) {
			pod.ObjectMeta.Annotations = annotation
		})
		// Fourth pod should not come up.
		fmt.Println("...creating 4th exec pod, expecting failure due to IPAM")
		ExecPodSpec := e2epod.NewAgnhostPod(ns, "", nil, nil, nil)
		ExecPodSpec.ObjectMeta.GenerateName = podName
		ExecPodSpec.ObjectMeta.Annotations = annotation
		f.ClientSet.CoreV1().Pods(ns).Create(context.TODO(), ExecPodSpec, metav1.CreateOptions{})
		time.Sleep(30 * time.Second)

		g.By("checking that additional pods do not come up when range is exhausted")
		output, err := oc.AsAdmin().Run("get").Args("pods").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		getPodArr := strings.Split(output, "\n")
		failedPodCount := 0
		for _, v := range getPodArr {
			if strings.Contains(v, "ContainerCreating") {
				failedPodCount++
				if failedPodCount > 1 {
					break
				}
			}
		}
		o.Expect(failedPodCount).To(o.Equal(1))
		fmt.Println(output)

		g.By("checking that successfully started pods are within IP range")
		// pod 1 ip check
		output, err = oc.AsAdmin().Run("exec").Args(testPod.Name, "--", "ip", "a").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		fmt.Println(output)
		o.Expect(output).Should(o.ContainSubstring("192.168.2.228"))
		// pod 1 annotation check
		output, err = oc.AsAdmin().Run("describe").Args("pod", testPod.Name).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		fmt.Println(output)
		o.Expect(output).Should(o.ContainSubstring("192.168.2.228"))
		// pod 2 ip check
		output, err = oc.AsAdmin().Run("exec").Args(testPod2.Name, "--", "ip", "a").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		fmt.Println(output)
		o.Expect(output).Should(o.ContainSubstring("192.168.2.229"))
		// pod 2 annotation check
		output, err = oc.AsAdmin().Run("describe").Args("pod", testPod2.Name).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		fmt.Println(output)
		o.Expect(output).Should(o.ContainSubstring("192.168.2.229"))
		// pod 3 ip check
		output, err = oc.AsAdmin().Run("exec").Args(testPod3.Name, "--", "ip", "a").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		fmt.Println(output)
		o.Expect(output).Should(o.ContainSubstring("192.168.2.230"))
		// pod 3 annotation check
		output, err = oc.AsAdmin().Run("describe").Args("pod", testPod3.Name).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		fmt.Println(output)
		o.Expect(output).Should(o.ContainSubstring("192.168.2.230"))

	})

	g.It("should assign unique IP addresses to each pod in the event of a race condition case [apigroup:k8s.cni.cncf.io]", g.Label("Size:M"), func() {
		// steps for the test
		// 1. create sleepy pod
		// 2. create awake pod
		// 3. check if both pods have unique ip
		var err error

		f := oc.KubeFramework()
		podName := "whereabouts-pod-"
		ns := f.Namespace.Name

		g.By("creating a whereabouts net-attach-def that invokes sleep")
		nad_yaml := exutil.FixturePath("testdata", "net-attach-defs", "whereabouts-race-sleepy.yml")
		g.By(fmt.Sprintf("calling oc create -f %s", nad_yaml))
		err = oc.AsAdmin().Run("create").Args("-f", nad_yaml).Execute()
		o.Expect(err).NotTo(o.HaveOccurred(), "created net-attach-def")

		g.By("creating a whereabouts net-attach-def that does not invoke sleep")
		nad_yaml = exutil.FixturePath("testdata", "net-attach-defs", "whereabouts-race-awake.yml")
		g.By(fmt.Sprintf("calling oc create -f %s", nad_yaml))
		err = oc.AsAdmin().Run("create").Args("-f", nad_yaml).Execute()
		o.Expect(err).NotTo(o.HaveOccurred(), "created net-attach-def")

		g.By("launching pods with annotations to use the net-attach-defs")
		annotation := map[string]string{
			"k8s.v1.cni.cncf.io/networks": "wa-sleepy-conf",
		}
		sleepyPod := frameworkpod.CreateExecPodOrFail(context.TODO(), f.ClientSet, ns, podName, func(pod *v1.Pod) {
			pod.ObjectMeta.Annotations = annotation
			pod.ObjectMeta.Name = "sleepy-pod"
		})

		annotation = map[string]string{
			"k8s.v1.cni.cncf.io/networks": "wa-awake-conf",
		}
		awakePod := frameworkpod.CreateExecPodOrFail(context.TODO(), f.ClientSet, ns, podName, func(pod *v1.Pod) {
			pod.ObjectMeta.Annotations = annotation
			pod.ObjectMeta.Name = "awake-pod"
		})

		g.By("checking that both pods are running with unique IP addresses")
		pod1_ip, err := oc.AsAdmin().Run("exec").Args(sleepyPod.Name, "--", "ip", "a").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		fmt.Println(pod1_ip)

		pod2_ip, err := oc.AsAdmin().Run("exec").Args(awakePod.Name, "--", "ip", "a").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		fmt.Println(pod2_ip)

		pod1_desc, err := oc.AsAdmin().Run("describe").Args("pod", sleepyPod.Name).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		fmt.Println(pod1_desc)

		pod2_desc, err := oc.AsAdmin().Run("describe").Args("pod", awakePod.Name).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		fmt.Println(pod2_desc)

		// check that IP addresses for both pods are unique.
		o.Expect(pod1_ip).ShouldNot(o.Equal(pod2_ip))
		o.Expect(pod1_desc).ShouldNot(o.Equal(pod2_desc))

	})
})
