package networking

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	imageutils "k8s.io/kubernetes/test/utils/image"

	"k8s.io/kubernetes/test/e2e/framework"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
	admissionapi "k8s.io/pod-security-admission/api"

	"github.com/openshift/origin/test/extended/util"
	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	nodeTCPPort = 9000
	nodeUDPPort = 9999
)

var _ = ginkgo.Describe("[sig-network] Internal connectivity", func() {
	f := framework.NewDefaultFramework("nettest")
	// TODO(sur): verify if privileged is really necessary in a follow-up
	f.NamespacePodSecurityLevel = admissionapi.LevelPrivileged
	oc := exutil.NewCLIWithoutNamespace("nettest").AsAdmin()

	ginkgo.It("for TCP and UDP on ports 9000-9999 is allowed [Serial:Self]", ginkgo.Label("Size:L"), func() {
		e2eskipper.SkipUnlessNodeCountIsAtLeast(2)

		namespace := f.Namespace.Name
		clientSet := f.ClientSet
		clientConfig := f.ClientConfig()

		// SCC privileged is needed for host networked pods
		_, err := runOcWithRetry(oc.AsAdmin(), "adm", "policy", "add-scc-to-user", "privileged", fmt.Sprintf("system:serviceaccount:%s:default", namespace))
		o.Expect(err).NotTo(o.HaveOccurred())

		one := int64(0)
		runAsUser := int64(0)
		privileged := true
		ds := &appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "webserver",
				Namespace: namespace,
			},
			Spec: appsv1.DaemonSetSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"apps": "webserver",
					},
				},
				Template: v1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"apps": "webserver",
						},
					},
					Spec: v1.PodSpec{
						Tolerations: []v1.Toleration{
							{
								Key:      "node-role.kubernetes.io/master",
								Operator: v1.TolerationOpExists,
								Effect:   v1.TaintEffectNoSchedule,
							},
						},
						HostNetwork:                   true,
						TerminationGracePeriodSeconds: &one,
						Containers: []v1.Container{
							{
								Name:    "webserver",
								Image:   imageutils.GetE2EImage(imageutils.Agnhost),
								Command: []string{"/agnhost", "netexec", fmt.Sprintf("--http-port=%v", nodeTCPPort), fmt.Sprintf("--udp-port=%v", nodeUDPPort)},
								Ports: []v1.ContainerPort{
									{Name: "tcp", ContainerPort: nodeTCPPort},
									{Name: "udp", ContainerPort: nodeUDPPort},
								},
								ReadinessProbe: &v1.Probe{
									InitialDelaySeconds: 10,
									ProbeHandler: v1.ProbeHandler{
										HTTPGet: &v1.HTTPGetAction{
											Port: intstr.FromInt(nodeTCPPort),
										},
									},
								},
								SecurityContext: &v1.SecurityContext{
									Privileged: &privileged,
									RunAsUser:  &runAsUser,
									Capabilities: &v1.Capabilities{
										Add: []v1.Capability{"NET_RAW"},
									},
								},
							},
						},
					},
				},
			},
		}
		name := ds.Name
		ds, err = clientSet.AppsV1().DaemonSets(namespace).Create(context.Background(), ds, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		err = wait.PollImmediate(5*time.Second, 5*time.Minute, func() (bool, error) {
			ds, err = clientSet.AppsV1().DaemonSets(namespace).Get(context.Background(), name, metav1.GetOptions{})
			if err != nil {
				framework.Logf("unable to retrieve daemonset: %v", err)
				return false, nil
			}
			if ds.Status.ObservedGeneration != ds.Generation || ds.Status.NumberAvailable == 0 || ds.Status.NumberAvailable != ds.Status.DesiredNumberScheduled {
				framework.Logf("waiting for daemonset: %#v", ds.Status)
				return false, nil
			}
			return true, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("daemonset ready: %#v", ds.Status)

		pods, err := clientSet.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{LabelSelector: labels.Set(ds.Spec.Selector.MatchLabels).String()})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(len(pods.Items)).To(o.Equal(int(ds.Status.NumberAvailable)), fmt.Sprintf("%#v", pods.Items))

		// verify connectivity across pairs of pods in parallel
		// TODO: on large clusters this is O(N^2), we could potentially sample or split by topology
		var testFns []func() error
		protocols := []v1.Protocol{v1.ProtocolTCP, v1.ProtocolUDP}
		ports := []int{nodeTCPPort, nodeUDPPort}
		for j := range pods.Items {
			for i := range pods.Items {
				if i == j {
					continue
				}
				for k := range protocols {
					func(i, j, k int) {
						testFns = append(testFns, func() error {
							from := pods.Items[j]
							to := pods.Items[i]
							protocol := protocols[k]
							testingMsg := fmt.Sprintf("[%s: %s -> %s:%d]", protocol, from.Spec.NodeName, to.Spec.NodeName, ports[k])
							testMsg := fmt.Sprintf("%s-from-%s-to-%s", "hello", from.Status.PodIP, to.Status.PodIP)
							command, err := testRemoteConnectivityCommand(protocol, "localhost:"+strconv.Itoa(nodeTCPPort), to.Status.HostIP, ports[k], testMsg)
							if err != nil {
								return fmt.Errorf("test of %s failed: %v", testingMsg, err)
							}
							res, err := util.ExecInPodWithResult(clientSet.CoreV1(), clientConfig, from.Namespace, from.Name, "webserver", []string{"/bin/sh", "-cex", strings.Join(command, " ")})
							if err != nil {
								return fmt.Errorf("test of %s failed: %v", testingMsg, err)
							}
							if res != `{"responses":["`+testMsg+`"]}` {
								return fmt.Errorf("test of %s failed, unexpected response: %s", testingMsg, res)
							}
							return nil
						})
					}(i, j, k)
				}
			}
		}
		errs := ParallelTest(6, testFns)
		o.Expect(errs).To(o.Equal([]error(nil)))
	})
})

// ParallelTest runs the provided fns in parallel with at most workers and returns an array of all
// non nil errors.
func ParallelTest(workers int, fns []func() error) []error {
	var wg sync.WaitGroup
	work := make(chan func() error, workers)
	results := make(chan error, workers)

	go func() {
		for _, fn := range fns {
			work <- fn
			wg.Add(1)
		}
		close(work)
		wg.Wait()
		close(results)
	}()

	for i := 0; i < workers; i++ {
		go func() {
			for fn := range work {
				results <- fn()
				wg.Done()
			}
		}()
	}

	var errs []error
	for err := range results {
		if err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

func testRemoteConnectivityCommand(protocol v1.Protocol, localHostPort, host string, port int, echoMessage string) ([]string, error) {
	var protocolType string
	var dialCommand string
	switch protocol {
	case v1.ProtocolTCP:
		protocolType = "http"
		dialCommand = fmt.Sprintf("echo?msg=%s", echoMessage)
	case v1.ProtocolUDP:
		protocolType = "udp"
		dialCommand = fmt.Sprintf("echo%%20%s", echoMessage)
	default:
		return nil, fmt.Errorf("curl does not support protocol %s", protocol)
	}

	//func (config *NetworkingTestConfig) DialFromContainer(protocol, dialCommand, containerIP, targetIP string, containerHTTPPort, targetPort, maxTries, minTries int, expectedResponses sets.String) {
	// The current versions of curl included in CentOS and RHEL distros
	// misinterpret square brackets around IPv6 as globbing, so use the -g
	// argument to disable globbing to handle the IPv6 case.
	command := []string{
		"curl", "-g", "-q", "-s",
		fmt.Sprintf("'http://%s/dial?request=%s&protocol=%s&host=%s&port=%d&tries=1'",
			localHostPort,
			dialCommand,
			protocolType,
			host,
			port),
	}
	return command, nil
}

func testConnectivityCommand(protocol v1.Protocol, host string, port, timeout int) ([]string, error) {
	command := []string{
		"nc",
		"-vz",
		"-w", strconv.Itoa(timeout),
	}
	switch protocol {
	case v1.ProtocolTCP:
		command = append(command, "-t")
	case v1.ProtocolUDP:
		command = append(command, "-u")
	default:
		return nil, fmt.Errorf("nc does not support protocol %s", protocol)
	}
	command = append(command, host, strconv.Itoa(port))
	return command, nil
}
