package networking

import (
	"context"
	"fmt"
	"regexp"
	"time"

	networkapi "github.com/openshift/api/network/v1"
	networkclient "github.com/openshift/client-go/network/clientset/versioned/typed/network/v1"
	"github.com/openshift/library-go/pkg/network/networkutils"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/image"

	kapiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	frameworkpod "k8s.io/kubernetes/test/e2e/framework/pod"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("[sig-network] multicast", func() {
	oc := exutil.NewCLI("multicast")

	// The subnet plugin should block all multicast. The multitenant and networkpolicy
	// plugins should implement multicast in the way that we test. For third-party
	// plugins, the behavior is unspecified and we should not run either test.

	InPluginContext([]string{networkutils.SingleTenantPluginName},
		func() {
			It("should block multicast traffic", func() {
				oc := exutil.NewCLI("multicast")
				f := oc.KubeFramework()
				Expect(testMulticast(f, oc)).NotTo(Succeed())
			})
		},
	)

	InPluginContext([]string{networkutils.MultiTenantPluginName, networkutils.NetworkPolicyPluginName},
		func() {
			It("should block multicast traffic in namespaces where it is disabled", func() {
				f := oc.KubeFramework()
				Expect(testMulticast(f, oc)).NotTo(Succeed())
			})
			It("should allow multicast traffic in namespaces where it is enabled", func() {
				f := oc.KubeFramework()
				makeNamespaceMulticastEnabled(oc, f.Namespace)
				Expect(testMulticast(f, oc)).To(Succeed())
			})
		},
	)
})

func makeNamespaceMulticastEnabled(oc *exutil.CLI, ns *kapiv1.Namespace) {
	clientConfig := oc.AdminConfig()
	networkClient := networkclient.NewForConfigOrDie(clientConfig)
	var netns *networkapi.NetNamespace
	var err error
	err = wait.Poll(time.Second, 2*time.Minute, func() (bool, error) {
		netns, err = networkClient.NetNamespaces().Get(context.Background(), ns.Name, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		return true, nil
	})
	expectNoError(err)
	if netns.Annotations == nil {
		netns.Annotations = make(map[string]string, 1)
	}
	netns.Annotations[networkapi.MulticastEnabledAnnotation] = "true"
	_, err = networkClient.NetNamespaces().Update(context.Background(), netns, metav1.UpdateOptions{})
	expectNoError(err)
}

// We run 'omping -c 5 -T 60 -q -q ${ip1} ${ip2} ${ip3}' in each pod:
//   -c 5  : exchange 5 multicast packets with each peer and then exit
//   -T 60 : time out and exit after 60 seconds no matter what
//   -q -q : extra quiet, only print final status
//
// (Since we need to pass all three pod IPs to each omping command, we launch the pods
// with the command "sleep 1000" first and then use "oc exec" to run omping.)
//
// The output should look like:
//
//   10.130.0.3 :   unicast, xmt/rcv/%loss = 5/5/0%, min/avg/max/std-dev = 0.046/0.046/0.046/0.000
//   10.130.0.3 : multicast, xmt/rcv/%loss = 5/5/0%, min/avg/max/std-dev = 0.068/0.068/0.068/0.000
//   10.129.0.2 :   unicast, xmt/rcv/%loss = 5/5/0%, min/avg/max/std-dev = 0.066/0.066/0.066/0.000
//   10.129.0.2 : multicast, xmt/rcv/%loss = 5/5/0%, min/avg/max/std-dev = 0.095/0.095/0.095/0.000
//
// However, network congestion may cause some packets to be dropped. We only consider the
// test to have failed if we see "multicast, xmt/rcv/%loss = 5/0/100%" in the output (ie,
// at least one of the pods was completely unable to communicate via multicast).

func testMulticast(f *e2e.Framework, oc *exutil.CLI) error {
	makeNamespaceScheduleToAllNodes(f)

	// We launch 3 pods total; pod[0] and pod[1] will end up on node[0], and pod[2]
	// will end up on node[1], ensuring we test both intra- and inter-node multicast
	var nodes [2]*kapiv1.Node
	var err error
	nodes[0], nodes[1], err = findAppropriateNodes(f, DIFFERENT_NODE)
	if err != nil {
		return err
	}

	var pod, ip, out [3]string
	var errs [3]error
	var ch [3]chan struct{}
	var failMatch [3]*regexp.Regexp

	for i := range pod {
		pod[i] = fmt.Sprintf("multicast-%d", i+1)
		err := launchTestMulticastPod(f, nodes[i/2].Name, pod[i])
		expectNoError(err)
		var zero int64
		defer f.ClientSet.CoreV1().Pods(f.Namespace.Name).Delete(context.Background(), pod[i], metav1.DeleteOptions{GracePeriodSeconds: &zero})
	}

	for i := range pod {
		ip[i], errs[i] = waitForTestMulticastPod(f, pod[i])
		expectNoError(errs[i])
		failMatch[i] = regexp.MustCompile(ip[i] + ".*multicast.*/100%")
		ch[i] = make(chan struct{})
	}

	for i := range pod {
		i := i
		go func() {
			out[i], errs[i] = oc.Run("exec").Args(pod[i], "--", "omping", "-c", "5", "-T", "60", "-q", "-q", ip[0], ip[1], ip[2]).Output()
			close(ch[i])
		}()
	}
	for i := range pod {
		<-ch[i]
		if errs[i] != nil {
			return errs[i]
		}
		for j := range pod {
			if i != j {
				if failMatch[j].MatchString(out[i]) {
					e2e.Logf("Pod %d failed to send multicast to pod %d:\n%s", i+1, j+1, out[i])
					return fmt.Errorf("pod %d failed to send multicast to pod %d", i+1, j+1)
				}
			}
		}
	}

	return nil
}

func launchTestMulticastPod(f *e2e.Framework, nodeName string, podName string) error {
	contName := fmt.Sprintf("%s-container", podName)
	pod := &kapiv1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind: "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: podName,
		},
		Spec: kapiv1.PodSpec{
			Containers: []kapiv1.Container{
				{
					Name:    contName,
					Image:   image.LocationFor("docker.io/openshift/test-multicast:latest"),
					Command: []string{"sleep", "1000"},
				},
			},
			NodeName:      nodeName,
			RestartPolicy: kapiv1.RestartPolicyNever,
		},
	}
	_, err := f.ClientSet.CoreV1().Pods(f.Namespace.Name).Create(context.Background(), pod, metav1.CreateOptions{})
	return err
}

func waitForTestMulticastPod(f *e2e.Framework, podName string) (string, error) {
	podIP := ""
	err := frameworkpod.WaitForPodCondition(f.ClientSet, f.Namespace.Name, podName, "running", podStartTimeout, func(pod *kapiv1.Pod) (bool, error) {
		podIP = pod.Status.PodIP
		return (podIP != "" && pod.Status.Phase != kapiv1.PodPending), nil
	})
	return podIP, err
}
