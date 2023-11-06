package commatrix

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/liornoy/node-comm-lib/pkg/client"
	"github.com/liornoy/node-comm-lib/pkg/commatrix"
	"github.com/liornoy/node-comm-lib/pkg/consts"
	"github.com/liornoy/node-comm-lib/pkg/ss"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	exutil "github.com/openshift/origin/test/extended/util"
)

func GenerateSSComMatrix(oc *exutil.CLI, artifactsDir string) (commatrix.ComMatrix, error) {
	comDetails := make([]commatrix.ComDetails, 0)

	nodes, err := oc.KubeClient().CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return commatrix.ComMatrix{}, err
	}

	nodesRoles := commatrix.GetNodesRoles(nodes)
	for _, n := range nodes.Items {
		tcpOutput, err := exutil.DebugNodeRetryWithOptionsAndChroot(oc, n.Name, "openshift-cluster-node-tuning-operator", "ss", "-anplt")
		if err != nil {
			return commatrix.ComMatrix{}, err
		}

		udpOutput, err := exutil.DebugNodeRetryWithOptionsAndChroot(oc, n.Name, "openshift-cluster-node-tuning-operator", "ss", "-anplu")
		if err != nil {
			return commatrix.ComMatrix{}, err
		}

		err = os.WriteFile(filepath.Join(artifactsDir, fmt.Sprintf("%s-ss-tcp", n.Name)), []byte(tcpOutput), 0644)
		if err != nil {
			return commatrix.ComMatrix{}, err
		}

		err = os.WriteFile(filepath.Join(artifactsDir, fmt.Sprintf("%s-ss-udp", n.Name)), []byte(udpOutput), 0644)
		if err != nil {
			return commatrix.ComMatrix{}, err
		}

		tcpComDetails := ss.ToComDetails(string(tcpOutput), nodesRoles[n.Name], "TCP")
		comDetails = append(comDetails, tcpComDetails...)

		udpComDetails := ss.ToComDetails(string(udpOutput), nodesRoles[n.Name], "UDP")
		comDetails = append(comDetails, udpComDetails...)
	}

	comDetails = commatrix.RemoveDups(comDetails)
	return commatrix.ComMatrix{Matrix: comDetails}, nil
}

func CreateHostServiceSlices(cs *client.ClientSet) error {
	nodes, err := cs.Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	nodeNameToNodeRole := commatrix.GetNodesRoles(nodes)
	nodeRoleToNodeName := ReverseMap(nodeNameToNodeRole)

	slices, err := CustomHostServicesDefinion()
	if err != nil {
		return err
	}

	for _, s := range slices {
		endpointSlice, err := ComDetailsToEPSlice(&s, nodeRoleToNodeName)
		if err != nil {
			return err
		}

		_, err = cs.EndpointSlices("default").Create(context.TODO(), &endpointSlice, metav1.CreateOptions{})
		if err != nil && !errors.IsAlreadyExists(err) {
			return err
		}
	}

	return nil
}

func ReverseMap(m map[string]string) map[string]string {
	n := make(map[string]string, len(m))
	for k, v := range m {
		n[v] = k
	}
	return n
}

func ComDetailsToEPSlice(cd *commatrix.ComDetails, nodeRoleToNodeName map[string]string) (discoveryv1.EndpointSlice, error) {
	port, err := strconv.ParseInt(cd.Port, 10, 32)
	if err != nil {
		return discoveryv1.EndpointSlice{}, err
	}
	name := fmt.Sprintf("commatrix-test-%s-%s-%s", cd.ServiceName, cd.NodeRole, cd.Port)

	nodeName := nodeRoleToNodeName[cd.NodeRole]
	// On SNO we want the single node name for either master/worker ComDetails.
	if len(nodeRoleToNodeName) == 1 {
		nodeName = nodeRoleToNodeName["master-worker"]
	}

	labels := map[string]string{
		consts.IngressLabel:          "",
		"kubernetes.io/service-name": cd.ServiceName,
	}
	if !cd.Required {
		labels[consts.OptionalLabel] = consts.OptionalTrue
	}

	endpointSlice := cd.ToEndpointSlice(name, "default", nodeName, labels, int(port))

	return endpointSlice, nil
}

func CustomHostServicesDefinion() ([]commatrix.ComDetails, error) {
	var res []commatrix.ComDetails
	var hostServiceEndpointSlicesJSON = `[
		{
			"protocol":    "TCP",
			"port":        "2380",
			"nodeRole":    "master",
			"serviceName": "etcd",
			"required": true
		},
		{
			"protocol":    "TCP",
			"port":        "9978",
			"nodeRole":    "master",
			"serviceName": "etcd",
			"required": true
		},
		{
			"protocol":    "TCP",
			"port":        "9980",
			"nodeRole":    "master",
			"serviceName": "etcd",
			"required": true
		},
		{
			"protocol":    "TCP",
			"port":        "9107",
			"nodeRole":    "worker",
			"serviceName": "ovnkube",
			"required": true
		},
		{
			"protocol":    "TCP",
			"port":        "10250",
			"nodeRole":    "worker",
			"serviceName": "kubelet",
			"required": true
		},
		{
			"protocol":    "TCP",
			"port":        "22",
			"nodeRole":    "worker",
			"serviceName": "sshd",
			"required": false
		},
		{
			"protocol":    "TCP",
			"port":        "111",
			"nodeRole":    "worker",
			"serviceName": "systemd",
			"required": true
		},
		{
			"protocol":    "TCP",
			"port":        "10256",
			"nodeRole":    "worker",
			"serviceName": "ovnkube",
			"required": true
		},
		{
			"protocol":    "TCP",
			"port":        "10250",
			"nodeRole":    "master",
			"serviceName": "kubelet",
			"required": true
		},
		{
			"protocol":    "TCP",
			"port":        "111",
			"nodeRole":    "master",
			"serviceName": "systemd",
			"required": true
		},
		{
			"protocol":    "TCP",
			"port":        "22",
			"nodeRole":    "master",
			"serviceName": "sshd",
			"required": false
		},
		{
			"protocol":    "TCP",
			"port":        "9107",
			"nodeRole":    "master",
			"serviceName": "ovnkube",
			"required": true
		},
		{
			"protocol":    "TCP",
			"port":        "10357",
			"nodeRole":    "master",
			"serviceName": "cluster-policy",
			"required": true
		},
		{
			"protocol":    "TCP",
			"port":        "10256",
			"nodeRole":    "master",
			"serviceName": "ovnkube",
			"required": true
		},
		{
			"protocol":    "TCP",
			"port":        "9191",
			"nodeRole":    "master",
			"serviceName": "machine-approve",
			"required": true
		},   
		{
			"protocol":    "TCP",
			"port":        "6388",
			"nodeRole":    "master",
			"serviceName": "metal3-state",
			"required": false
		},   
		{
			"protocol":    "TCP",
			"port":        "22624",
			"nodeRole":    "master",
			"serviceName": "machine-config",
			"required": true
		},   
		{
			"protocol":    "TCP",
			"port":        "22623",
			"nodeRole":    "master",
			"serviceName": "machine-config",
			"required": true
		},   
		{
			"protocol":    "TCP",
			"port":        "29445",
			"nodeRole":    "master",
			"serviceName": "haproxy",
			"required": false
		},   
		{
			"protocol":    "TCP",
			"port":        "9444",
			"nodeRole":    "master",
			"serviceName": "haproxy",
			"required": false
		},   
		{
			"protocol":    "TCP",
			"port":        "9445",
			"nodeRole":    "master",
			"serviceName": "haproxy",
			"required": false
		},   
		{
			"protocol":    "TCP",
			"port":        "9258",
			"nodeRole":    "master",
			"serviceName": "cluster-control",
			"required": true
		},   
		{
			"protocol":    "TCP",
			"port":        "17697",
			"nodeRole":    "master",
			"serviceName": "cluster-kube-ap",
			"required": true
		},   
		{
			"protocol":    "TCP",
			"port":        "6080",
			"nodeRole":    "master",
			"serviceName": "cluster-kube-ap",
			"required": true
		},   
		{
			"protocol":    "TCP",
			"port":        "5050",
			"nodeRole":    "master",
			"serviceName": "httpd",
			"required": false
		}
	]
	`
	err := json.Unmarshal([]byte(hostServiceEndpointSlicesJSON), &res)
	if err != nil {
		return nil, err
	}

	return res, nil
}
