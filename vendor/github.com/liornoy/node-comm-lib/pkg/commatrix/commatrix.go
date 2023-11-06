package commatrix

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/liornoy/node-comm-lib/pkg/client"
	"github.com/liornoy/node-comm-lib/pkg/consts"
	"github.com/liornoy/node-comm-lib/pkg/pointer"
)

type ComMatrix struct {
	Matrix []ComDetails
}

type ComDetails struct {
	Direction   string `json:"direction"`
	Protocol    string `json:"protocol"`
	Port        string `json:"port"`
	NodeRole    string `json:"nodeRole"`
	ServiceName string `json:"serviceName"`
	Required    bool   `json:"required"`
}

func (cd ComDetails) String() string {
	return fmt.Sprintf("%s,%s,%s,%s,%s,%v", cd.Direction, cd.Protocol, cd.Port, cd.NodeRole, cd.ServiceName, cd.Required)
}

func (cd ComDetails) ToEndpointSlice(endpointSliceName string, namespace string, nodeName string, labels map[string]string, port int) discoveryv1.EndpointSlice {
	endpointSlice := discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      endpointSliceName,
			Namespace: namespace,
			Labels:    labels,
		},
		Ports: []discoveryv1.EndpointPort{
			{
				Port:     pointer.Int32Ptr(int32(port)),
				Protocol: (*corev1.Protocol)(&cd.Protocol),
			},
		},
		Endpoints: []discoveryv1.Endpoint{
			{
				NodeName:  pointer.StrPtr(nodeName),
				Addresses: []string{consts.PlaceHolderIPAddress},
			},
		},
		AddressType: consts.DefaultAddressType,
	}

	return endpointSlice
}

func (m ComMatrix) String() string {
	var result strings.Builder
	for _, details := range m.Matrix {
		result.WriteString(details.String() + "\n")
	}

	return result.String()
}

func CreateComMatrix(cs *client.ClientSet, epSlices []discoveryv1.EndpointSlice) (ComMatrix, error) {
	if len(epSlices) == 0 {
		return ComMatrix{}, fmt.Errorf("failed to create ComMatrix: epSlices is empty")
	}

	nodes, err := cs.Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return ComMatrix{}, fmt.Errorf("failed to create ComMatrix: %w", err)
	}

	nodesRoles := GetNodesRoles(nodes)
	comDetails := make([]ComDetails, 0)

	for _, epSlice := range epSlices {
		cd := createComDetails(epSlice, nodesRoles)
		comDetails = append(comDetails, cd...)
	}

	cleanedComDetails := RemoveDups(comDetails)
	res := ComMatrix{Matrix: cleanedComDetails}

	return res, nil
}

func createComDetails(epSlice discoveryv1.EndpointSlice, nodesRoles map[string]string) []ComDetails {
	res := make([]ComDetails, 0)

	required := true
	if _, ok := epSlice.Labels["optional"]; ok {
		required = false
	}

	service := epSlice.Labels["kubernetes.io/service-name"]
	for _, endpoint := range epSlice.Endpoints {
		for _, p := range epSlice.Ports {
			comDetails := ComDetails{
				Direction:   "ingress",
				Protocol:    fmt.Sprint(*p.Protocol),
				Port:        fmt.Sprint(*p.Port),
				NodeRole:    nodesRoles[*endpoint.NodeName],
				ServiceName: service,
				Required:    required,
			}
			res = append(res, comDetails)
		}
	}

	return res
}

func (m ComMatrix) ToCSV() ([]byte, error) {
	out := make([]byte, 0)
	w := bytes.NewBuffer(out)
	csvwriter := csv.NewWriter(w)

	for _, cd := range m.Matrix {
		record := strings.Split(cd.String(), ",")
		err := csvwriter.Write(record)
		if err != nil {
			return nil, fmt.Errorf("failed to convert to CSV foramt: %w", err)
		}
	}
	csvwriter.Flush()

	return w.Bytes(), nil
}

func (m ComMatrix) ToJSON() ([]byte, error) {
	out, err := json.Marshal(m.Matrix)
	if err != nil {
		return nil, err
	}

	return out, nil
}

func RemoveDups(outPuts []ComDetails) []ComDetails {
	allKeys := make(map[string]bool)
	res := []ComDetails{}
	for _, item := range outPuts {
		str := fmt.Sprintf("%s-%s-%s", item.NodeRole, item.Port, item.Protocol)
		if _, value := allKeys[str]; !value {
			allKeys[str] = true
			res = append(res, item)
		}
	}

	return res
}

func GetNodesRoles(nodes *corev1.NodeList) map[string]string {
	res := make(map[string]string)

	for _, node := range nodes.Items {
		_, isWorker := node.Labels[consts.WorkerRole]
		_, isMaster := node.Labels[consts.MasterRole]
		if isMaster && isWorker {
			res[node.Name] = "master-worker"
			continue
		}
		if isMaster {
			res[node.Name] = "master"
		}
		if isWorker {
			res[node.Name] = "worker"
		}
	}

	return res
}

// Diff returns the diff ComMatrix while ignoring entries with ignored ports or that are not required.
func (m ComMatrix) Diff(other ComMatrix, ignorePorts map[string]bool) ComMatrix {
	diff := []ComDetails{}
	for _, cd1 := range m.Matrix {
		if !cd1.Required {
			continue
		}
		if _, ok := ignorePorts[cd1.Port]; ok {
			continue
		}
		found := false
		for _, cd2 := range other.Matrix {
			if cd1.Port == cd2.Port {
				found = true
				break
			}
		}
		if !found {
			diff = append(diff, cd1)
		}
	}

	return ComMatrix{Matrix: diff}
}

func (m ComMatrix) WriteTo(f *os.File) error {
	sorted := m.Matrix
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].ServiceName < sorted[j].ServiceName
	})

	for _, cd := range sorted {
		_, err := f.Write([]byte(fmt.Sprintln(cd)))
		if err != nil {
			return err
		}
	}

	return nil
}
