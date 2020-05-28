package networking

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/scheme"
	coreclientset "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/kubernetes/test/e2e/framework"
	e2enetwork "k8s.io/kubernetes/test/e2e/framework/network"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"

	"github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
)

const (
	nodeTCPPort = 9000
	nodeUDPPort = 9999
)

var _ = ginkgo.Describe("[sig-network] Internal connectivity", func() {
	f := framework.NewDefaultFramework("k8s-nettest")

	ginkgo.It("for TCP and UDP on ports 9000-9999 is allowed", func() {
		e2eskipper.SkipUnlessNodeCountIsAtLeast(2)
		clientConfig := f.ClientConfig()

		one := int64(0)
		ds := &appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "webserver",
				Namespace: f.Namespace.Name,
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
								Image:   e2enetwork.NetexecImageName,
								Command: []string{"/bin/bash", "-c", fmt.Sprintf("#!/bin/bash\napk add -q --update tcpdump\n./agnhost netexec --http-port=%v --udp-port=%v &\nexec tcpdump -i any port %v or port %v -n", nodeTCPPort, nodeUDPPort, nodeTCPPort, nodeUDPPort)},
								Ports: []v1.ContainerPort{
									{Name: "tcp", ContainerPort: nodeTCPPort},
									{Name: "udp", ContainerPort: nodeUDPPort},
								},
								ReadinessProbe: &v1.Probe{
									InitialDelaySeconds: 10,
									Handler: v1.Handler{
										HTTPGet: &v1.HTTPGetAction{
											Port: intstr.FromInt(nodeTCPPort),
										},
									},
								},
							},
						},
					},
				},
			},
		}
		name := ds.Name
		ds, err := f.ClientSet.AppsV1().DaemonSets(f.Namespace.Name).Create(context.Background(), ds, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		err = wait.PollImmediate(5*time.Second, 5*time.Minute, func() (bool, error) {
			ds, err = f.ClientSet.AppsV1().DaemonSets(f.Namespace.Name).Get(context.Background(), name, metav1.GetOptions{})
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

		pods, err := f.ClientSet.CoreV1().Pods(f.Namespace.Name).List(context.Background(), metav1.ListOptions{LabelSelector: labels.Set(ds.Spec.Selector.MatchLabels).String()})
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
							res, err := commandResult(f.ClientSet.CoreV1(), clientConfig, from.Namespace, from.Name, "webserver", []string{"/bin/sh", "-cex", strings.Join(command, " ")})
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
		errs := parallelTest(6, testFns)
		o.Expect(errs).To(o.Equal([]error(nil)))
	})
})

// parallelTest runs the provided fns in parallel with at most workers and returns an array of all
// non nil errors.
func parallelTest(workers int, fns []func() error) []error {
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
		return nil, fmt.Errorf("nc does not support protocol %s", protocol)
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

// commandContents fetches the result of invoking a command in the provided container from stdout.
func commandResult(podClient coreclientset.CoreV1Interface, podRESTConfig *rest.Config, ns, name, containerName string, command []string) (string, error) {
	u := podClient.RESTClient().Post().Resource("pods").Namespace(ns).Name(name).SubResource("exec").VersionedParams(&v1.PodExecOptions{
		Container: containerName,
		Stdout:    true,
		Stderr:    true,
		Command:   command,
	}, scheme.ParameterCodec).URL()

	e, err := remotecommand.NewSPDYExecutor(podRESTConfig, "POST", u)
	if err != nil {
		return "", fmt.Errorf("could not initialize a new SPDY executor: %v", err)
	}
	buf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	if err := e.Stream(remotecommand.StreamOptions{
		Stdout: buf,
		Stdin:  nil,
		Stderr: errBuf,
	}); err != nil {
		framework.Logf("exec error: %s", errBuf.String())
		return "", err
	}
	return buf.String(), nil
}
