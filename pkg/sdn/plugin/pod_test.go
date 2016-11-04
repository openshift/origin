package plugin

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openshift/origin/pkg/sdn/plugin/cniserver"

	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"

	kapi "k8s.io/kubernetes/pkg/api"
	kunversioned "k8s.io/kubernetes/pkg/api/unversioned"
	kcontainer "k8s.io/kubernetes/pkg/kubelet/container"
	kcontainertest "k8s.io/kubernetes/pkg/kubelet/container/testing"
	khostport "k8s.io/kubernetes/pkg/kubelet/network/hostport"
	utiltesting "k8s.io/kubernetes/pkg/util/testing"

	cnitypes "github.com/containernetworking/cni/pkg/types"
)

type operation struct {
	command   cniserver.CNICommand
	namespace string
	name      string
	cidr      string                // pod CIDR for add operation
	failStr   string                // error string for failing the operation
	request   *cniserver.PodRequest // filled in automatically from other info
	result    *cnitypes.Result
}

type expectedPod struct {
	// IP address to return for the pod's ADD operation
	cidr    string
	added   bool
	updated uint
	deleted bool
	errors  map[cniserver.CNICommand]string
}

type podTester struct {
	t        *testing.T
	testname string
	client   *http.Client

	// Holds list of expected pods and their IP address for the ADD operation
	pods map[string]*expectedPod
}

func newPodTester(t *testing.T, testname string, socketPath string) *podTester {
	client := &http.Client{
		Transport: &http.Transport{
			Dial: func(proto, addr string) (net.Conn, error) {
				return net.Dial("unix", socketPath)
			},
		},
	}

	return &podTester{
		t:        t,
		testname: testname,
		client:   client,
		pods:     make(map[string]*expectedPod),
	}
}

func ptPodKey(namespace, name string) string {
	return fmt.Sprintf("%s/%s", namespace, name)
}

func (pt *podTester) getExpectedPod(namespace, name string, command cniserver.CNICommand) (*expectedPod, error) {
	pod := pt.pods[ptPodKey(namespace, name)]
	if pod == nil {
		return nil, fmt.Errorf("pod not found!")
	} else if failStr, ok := pod.errors[command]; ok {
		return nil, fmt.Errorf(failStr)
	}
	return pod, nil
}

func (pt *podTester) addExpectedPod(t *testing.T, op *operation) {
	pk := ptPodKey(op.namespace, op.name)
	pod, ok := pt.pods[pk]
	if !ok {
		pod = &expectedPod{
			cidr:   op.cidr,
			errors: make(map[cniserver.CNICommand]string),
		}
		pt.pods[pk] = pod
	}
	if op.failStr != "" {
		pod.errors[op.command] = op.failStr
	}
}

func (pt *podTester) setup(req *cniserver.PodRequest) (*cnitypes.Result, *khostport.RunningPod, error) {
	pod, err := pt.getExpectedPod(req.PodNamespace, req.PodName, req.Command)
	if err != nil {
		return nil, nil, err
	} else if pod.added {
		return nil, nil, fmt.Errorf("pod already added!")
	}
	pod.added = true

	ip, ipnet, _ := net.ParseCIDR(pod.cidr)
	result := &cnitypes.Result{
		IP4: &cnitypes.IPConfig{
			IP: net.IPNet{
				IP:   ip,
				Mask: ipnet.Mask,
			},
		},
	}
	runningPod := &khostport.RunningPod{
		Pod: &kapi.Pod{
			TypeMeta: kunversioned.TypeMeta{
				Kind: "Pod",
			},
			ObjectMeta: kapi.ObjectMeta{
				Name:      req.PodName,
				Namespace: req.PodNamespace,
			},
			Spec: kapi.PodSpec{
				Containers: []kapi.Container{
					{
						Name:  "foobareasadfa",
						Image: "awesome-image",
					},
				},
			},
		},
		IP: ip,
	}

	return result, runningPod, nil
}

func (pt *podTester) update(req *cniserver.PodRequest) error {
	pod, err := pt.getExpectedPod(req.PodNamespace, req.PodName, req.Command)
	if err == nil {
		pod.updated += 1
	}
	return err
}

func (pt *podTester) teardown(req *cniserver.PodRequest) error {
	pod, err := pt.getExpectedPod(req.PodNamespace, req.PodName, req.Command)
	if err == nil {
		pod.deleted = true
	}
	return err
}

type fakeHost struct {
	runtime kcontainer.Runtime
}

func newFakeHost() *fakeHost {
	return &fakeHost{
		runtime: &kcontainertest.FakeRuntime{
			AllPodList: []*kcontainertest.FakePod{},
		},
	}
}

func (fnh *fakeHost) GetPodByName(name, namespace string) (*kapi.Pod, bool) {
	return nil, false
}

func (fnh *fakeHost) GetKubeClient() clientset.Interface {
	return nil
}

func (fnh *fakeHost) GetRuntime() kcontainer.Runtime {
	return fnh.runtime
}

type podcheck struct {
	namespace   string
	name        string
	updateCount uint
}

func TestPodManager(t *testing.T) {
	tmpDir, err := utiltesting.MkTmpdir("cniserver")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	socketPath := filepath.Join(tmpDir, "cni-server.sock")

	testcases := map[string]struct {
		operations []*operation
		checks     []*podcheck
	}{
		"ADD+DEL one pod": {
			operations: []*operation{
				{
					command:   cniserver.CNI_ADD,
					namespace: "namespace1",
					name:      "pod1",
					cidr:      "10.1.2.4/24",
				},
				{
					command:   cniserver.CNI_DEL,
					namespace: "namespace1",
					name:      "pod1",
				},
			},
			checks: []*podcheck{
				{
					namespace:   "namespace1",
					name:        "pod1",
					updateCount: 0,
				},
			},
		},
		"ADD+UPDATE+DEL many pod": {
			operations: []*operation{
				{
					command:   cniserver.CNI_ADD,
					namespace: "namespace1",
					name:      "pod1",
					cidr:      "10.1.2.4/24",
				},
				{
					command:   cniserver.CNI_UPDATE,
					namespace: "namespace1",
					name:      "pod1",
				},
				{
					command:   cniserver.CNI_ADD,
					namespace: "namespace2",
					name:      "pod2",
					cidr:      "10.1.2.3/24",
				},
				{
					command:   cniserver.CNI_ADD,
					namespace: "namespace3",
					name:      "pod3",
					cidr:      "10.1.2.2/24",
				},
				{
					command:   cniserver.CNI_UPDATE,
					namespace: "namespace2",
					name:      "pod2",
				},
				{
					command:   cniserver.CNI_DEL,
					namespace: "namespace1",
					name:      "pod1",
				},
				{
					command:   cniserver.CNI_UPDATE,
					namespace: "namespace2",
					name:      "pod2",
				},
				{
					command:   cniserver.CNI_DEL,
					namespace: "namespace3",
					name:      "pod3",
				},
				{
					command:   cniserver.CNI_DEL,
					namespace: "namespace2",
					name:      "pod2",
				},
				{
					command:   cniserver.CNI_DEL,
					namespace: "namespace2",
					name:      "pod2",
				},
			},
			checks: []*podcheck{
				{
					namespace:   "namespace1",
					name:        "pod1",
					updateCount: 1,
				},
				{
					namespace:   "namespace2",
					name:        "pod2",
					updateCount: 2,
				},
				{
					namespace:   "namespace3",
					name:        "pod3",
					updateCount: 0,
				},
			},
		},
		"ADD error": {
			operations: []*operation{
				{
					command:   cniserver.CNI_ADD,
					namespace: "namespace1",
					name:      "pod1",
					cidr:      "10.1.2.5/24",
					failStr:   "fail hard",
				},
			},
		},
		"UPDATE error": {
			operations: []*operation{
				{
					command:   cniserver.CNI_ADD,
					namespace: "namespace2",
					name:      "pod2",
					cidr:      "10.1.2.5/24",
				},
				{
					command:   cniserver.CNI_UPDATE,
					namespace: "namespace2",
					name:      "pod2",
					failStr:   "fail harder",
				},
			},
		},
		"DEL error": {
			operations: []*operation{
				{
					command:   cniserver.CNI_ADD,
					namespace: "namespace3",
					name:      "pod3",
					cidr:      "10.1.2.5/24",
				},
				{
					command:   cniserver.CNI_DEL,
					namespace: "namespace3",
					name:      "pod3",
					failStr:   "fail like a rock",
				},
			},
		},
		"unknown command": {
			operations: []*operation{
				{
					command:   cniserver.CNICommand("foobar!"),
					namespace: "namespace3",
					name:      "pod3",
					failStr:   "unhandled CNI request foobar!",
				},
			},
		},
	}

	for k, tc := range testcases {
		podTester := newPodTester(t, k, socketPath)
		podManager := newDefaultPodManager(newFakeHost())
		podManager.podHandler = podTester
		podManager.Start(socketPath)

		// Add pods to our expected pod list before kicking off the
		// actual pod setup to ensure we don't concurrently access
		// our pod map from different goroutines
		for _, op := range tc.operations {
			podTester.addExpectedPod(t, op)
		}

		for _, op := range tc.operations {
			op.request = &cniserver.PodRequest{
				Command:      op.command,
				PodNamespace: op.namespace,
				PodName:      op.name,
				ContainerId:  "asd;lfkajsdflkajfs",
				Netns:        "/some/network/namespace",
				Result:       make(chan *cniserver.PodResult),
			}
			podManager.addRequest(op.request)
		}

		for _, op := range tc.operations {
			result := podManager.waitRequest(op.request)
			if op.failStr != "" {
				if result.Err == nil {
					t.Fatalf("[%s] unexpected %v result success", k, op)
				}
				if !strings.HasPrefix(fmt.Sprintf("%v", result.Err), op.failStr) {
					t.Fatalf("[%s] unexpected %v error: %v", k, op, result.Err)
				}
			} else {
				if result.Err != nil {
					t.Fatalf("[%s] unexpected %v result error %v", k, op, result.Err)
				}

				if op.command == cniserver.CNI_ADD {
					if result.Response == nil {
						t.Fatalf("[%s] unexpected %v nil result response", k, op)
					}
					var cniResult *cnitypes.Result
					if err := json.Unmarshal(result.Response, &cniResult); err != nil {
						t.Fatalf("[%s] unexpected error unmarshalling CNI result '%s': %v", k, string(result.Response), err)
					}
					if cniResult.IP4.IP.String() != op.cidr {
						t.Fatalf("[%s] expected ADD IP %s but got %s", k, op.cidr, cniResult.IP4.IP.String())
					}
				}
			}
		}

		// Verify pod operations performed as requested
		for _, check := range tc.checks {
			pod, err := podTester.getExpectedPod(check.namespace, check.name, "")
			if err != nil {
				t.Fatalf("[%s] expected pod %v: %v", k, check, err)
			}

			if len(pod.errors) > 0 {
				// expected error; don't check operations
				continue
			}

			if !pod.added {
				t.Fatalf("[%s] added pod %v not marked added", k, check)
			}
			if pod.updated != check.updateCount {
				t.Fatalf("[%s] pod %v update count wrong, got %v", k, check, pod.updated)
			}
			if !pod.deleted {
				t.Fatalf("[%s] expected pod %v to be deleted", k, check)
			}
			// Make sure it's gone from the podManager too
			if podManager.getPod(&cniserver.PodRequest{
				PodNamespace: check.namespace,
				PodName:      check.name,
			}) != nil {
				t.Fatalf("[%s] expected pod %v to be deleted from podManager", k, check)
			}
		}
	}
}

// Test a direct pod update, not through the CNIServer, like the node process
// currently does due to lack of a standard CNI update command
func TestDirectPodUpdate(t *testing.T) {
	tmpDir, err := utiltesting.MkTmpdir("cniserver")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	socketPath := filepath.Join(tmpDir, "cni-server.sock")

	podTester := newPodTester(t, "update", socketPath)
	podManager := newDefaultPodManager(newFakeHost())
	podManager.podHandler = podTester
	podManager.Start(socketPath)

	op := &operation{
		command:   cniserver.CNI_UPDATE,
		namespace: "foobarnamespace",
		name:      "foobarname",
	}
	podTester.addExpectedPod(t, op)

	req := &cniserver.PodRequest{
		Command:      op.command,
		PodNamespace: op.namespace,
		PodName:      op.name,
		ContainerId:  "asdfasdfasdfaf",
		Result:       make(chan *cniserver.PodResult),
	}

	// Send request and wait for the result
	if _, err = podManager.handleCNIRequest(req); err != nil {
		t.Fatalf("failed to update pod: %v", err)
	}
}
