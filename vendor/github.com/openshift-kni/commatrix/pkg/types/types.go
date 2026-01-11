package types

import (
	"bytes"
	"cmp"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	"github.com/gocarina/gocsv"

	configv1 "github.com/openshift/api/config/v1"
	corev1 "k8s.io/api/core/v1"

	rtclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/openshift-kni/commatrix/pkg/consts"
	"github.com/openshift-kni/commatrix/pkg/utils"
)

var SupportedPlatforms = []configv1.PlatformType{
	configv1.AWSPlatformType,
	configv1.BareMetalPlatformType,
	configv1.NonePlatformType,
}

type Deployment int

const (
	SNO Deployment = iota
	Standard
)

const (
	FormatJSON = "json"
	FormatYAML = "yaml"
	FormatCSV  = "csv"
	FormatNFT  = "nft"
)

type ComMatrix struct {
	Matrix []ComDetails
}

type ComDetails struct {
	Direction string `json:"direction" yaml:"direction" csv:"Direction"`
	Protocol  string `json:"protocol" yaml:"protocol" csv:"Protocol"`
	Port      int    `json:"port" yaml:"port" csv:"Port"`
	Namespace string `json:"namespace" yaml:"namespace" csv:"Namespace"`
	Service   string `json:"service" yaml:"service" csv:"Service"`
	Pod       string `json:"pod" yaml:"pod" csv:"Pod"`
	Container string `json:"container" yaml:"container" csv:"Container"`
	NodeGroup string `json:"nodeGroup" yaml:"nodeGroup" csv:"NodeGroup"`
	Optional  bool   `json:"optional" yaml:"optional" csv:"Optional"`
}

type ContainerInfo struct {
	Containers []struct {
		Labels struct {
			ContainerName string `json:"io.kubernetes.container.name"`
			PodName       string `json:"io.kubernetes.pod.name"`
			PodNamespace  string `json:"io.kubernetes.pod.namespace"`
		} `json:"labels"`
	} `json:"containers"`
}

func (m *ComMatrix) ToCSV() ([]byte, error) {
	out := make([]byte, 0)
	w := bytes.NewBuffer(out)
	csvwriter := csv.NewWriter(w)

	err := gocsv.MarshalCSV(&m.Matrix, csvwriter)
	if err != nil {
		return nil, err
	}

	csvwriter.Flush()

	return w.Bytes(), nil
}

func (m *ComMatrix) ToJSON() ([]byte, error) {
	out, err := json.MarshalIndent(m.Matrix, "", "    ")
	if err != nil {
		return nil, err
	}

	return out, nil
}

func (m *ComMatrix) ToYAML() ([]byte, error) {
	out, err := yaml.Marshal(m)
	if err != nil {
		return nil, err
	}

	return out, nil
}

func (m *ComMatrix) String() string {
	var result strings.Builder
	for _, details := range m.Matrix {
		result.WriteString(details.String() + "\n")
	}

	return result.String()
}

func (m *ComMatrix) WriteMatrixToFileByType(utilsHelpers utils.UtilsInterface, fileNamePrefix, format string, deployment Deployment, destDir string) error {
	if format == FormatNFT {
		pools := m.SeparateMatrixByGroup()
		for poolName, mat := range pools {
			if len(mat.Matrix) == 0 {
				continue
			}
			if err := mat.writeMatrixToFile(utilsHelpers, fileNamePrefix+"-"+poolName, format, destDir); err != nil {
				return err
			}
		}
		return nil
	}

	err := m.writeMatrixToFile(utilsHelpers, fileNamePrefix, format, destDir)
	if err != nil {
		return err
	}
	return nil
}

func (m *ComMatrix) print(format string) ([]byte, error) {
	switch format {
	case FormatJSON:
		return m.ToJSON()
	case FormatCSV:
		return m.ToCSV()
	case FormatYAML:
		return m.ToYAML()
	case FormatNFT:
		return m.ToNFTables()
	default:
		return nil, fmt.Errorf("invalid format: %s. Please specify json, csv, yaml, or nft", format)
	}
}

// SeparateMatrixByGroup groups matrix entries by their group name (stored in NodeGroup).
func (m *ComMatrix) SeparateMatrixByGroup() map[string]ComMatrix {
	res := make(map[string]ComMatrix)
	for _, entry := range m.Matrix {
		pool := entry.NodeGroup
		if pool == "" {
			continue
		}
		cm := res[pool]
		cm.Matrix = append(cm.Matrix, entry)
		res[pool] = cm
	}
	return res
}

func (m *ComMatrix) writeMatrixToFile(utilsHelpers utils.UtilsInterface, fileName, format string, destDir string) error {
	res, err := m.print(format)
	if err != nil {
		return err
	}

	comMatrixFileName := filepath.Join(destDir, fmt.Sprintf("%s.%s", fileName, format))
	return utilsHelpers.WriteFile(comMatrixFileName, res)
}

func (m *ComMatrix) Contains(cd ComDetails) bool {
	for _, cd1 := range m.Matrix {
		if cd1.Equals(cd) {
			return true
		}
	}

	return false
}

func (m *ComMatrix) ToNFTables() ([]byte, error) {
	var tcpPorts []string
	var udpPorts []string
	for _, line := range m.Matrix {
		if line.Protocol == "TCP" {
			tcpPorts = append(tcpPorts, fmt.Sprint(line.Port))
		} else if line.Protocol == "UDP" {
			udpPorts = append(udpPorts, fmt.Sprint(line.Port))
		}
	}

	tcpPortsStr := strings.Join(tcpPorts, ", ")
	udpPortsStr := strings.Join(udpPorts, ", ")

	result := fmt.Sprintf(`#!/usr/sbin/nft -f
      table inet openshift_filter {
          chain OPENSHIFT {
					type filter hook input priority 1; policy accept;
			
					# Allow loopback traffic
					iif lo accept
			
					# Allow established and related traffic
					ct state established,related accept
			
					# Allow ICMP on ipv4
					ip protocol icmp accept
					# Allow ICMP on ipv6
					ip6 nexthdr ipv6-icmp accept
			
					# Allow specific TCP and UDP ports
					tcp dport { %s } accept
					udp dport { %s } accept
			
					# Logging and default drop
					log prefix "firewall " drop
				  }
			    }`, tcpPortsStr, udpPortsStr)

	return []byte(result), nil
}

// SortAndRemoveDuplicates removes duplicates in the matrix and sort it.
func (m *ComMatrix) SortAndRemoveDuplicates() {
	allKeys := make(map[string]bool)
	res := []ComDetails{}
	for _, item := range m.Matrix {
		str := fmt.Sprintf("%s-%d-%s", item.NodeGroup, item.Port, item.Protocol)
		if _, value := allKeys[str]; !value {
			allKeys[str] = true
			res = append(res, item)
		}
	}
	m.Matrix = res

	slices.SortFunc(m.Matrix, func(a, b ComDetails) int {
		res := cmp.Compare(a.NodeGroup, b.NodeGroup)
		if res != 0 {
			return res
		}

		res = cmp.Compare(a.Protocol, b.Protocol)
		if res != 0 {
			return res
		}

		return cmp.Compare(a.Port, b.Port)
	})
}

func (cd ComDetails) String() string {
	return fmt.Sprintf("%s,%s,%d,%s,%s,%s,%s,%s,%v", cd.Direction, cd.Protocol, cd.Port, cd.Namespace, cd.Service, cd.Pod, cd.Container, cd.NodeGroup, cd.Optional)
}

func (cd ComDetails) Equals(other ComDetails) bool {
	strComDetail1 := fmt.Sprintf("%s-%d-%s", cd.NodeGroup, cd.Port, cd.Protocol)
	strComDetail2 := fmt.Sprintf("%s-%d-%s", other.NodeGroup, other.Port, other.Protocol)

	return strComDetail1 == strComDetail2
}

func GetComMatrixHeadersByFormat(format string) (string, error) {
	typ := reflect.TypeOf(ComDetails{})

	var tagsList []string
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		tag := field.Tag.Get(format)
		if tag == "" {
			return "", fmt.Errorf("field %v has no tag of format %s", field, format)
		}
		tagsList = append(tagsList, tag)
	}

	return strings.Join(tagsList, ","), nil
}

func GetNodeRole(node *corev1.Node) (string, error) {
	if _, ok := node.Labels[consts.RoleLabel+"master"]; ok {
		return "master", nil
	}

	if _, ok := node.Labels[consts.RoleLabel+"control-plane"]; ok {
		return "master", nil
	}

	if _, ok := node.Labels[consts.RoleLabel+"worker"]; ok {
		return "worker", nil
	}

	for label := range node.Labels {
		if strings.HasPrefix(label, consts.RoleLabel) {
			return strings.TrimPrefix(label, consts.RoleLabel), nil
		}
	}

	return "", fmt.Errorf("unable to determine role for node %s", node.Name)
}

// BuildNodeToGroupMap builds a node->group map for clusters without MCP:
// - Prefer HyperShift NodePool label when present.
// - Otherwise fall back to Kubernetes node role derived from labels.
func BuildNodeToGroupMap(c rtclient.Client) (map[string]string, error) {
	nodeList := &corev1.NodeList{}
	if err := c.List(context.TODO(), nodeList); err != nil {
		return nil, err
	}
	nodeToGroup := make(map[string]string, len(nodeList.Items))
	for _, node := range nodeList.Items {
		if np, ok := node.Labels["hypershift.openshift.io/nodePool"]; ok && np != "" {
			nodeToGroup[node.Name] = np
			continue
		}
		role, err := GetNodeRole(&node)
		if err != nil {
			return nil, err
		}
		nodeToGroup[node.Name] = role
	}
	return nodeToGroup, nil
}

func ParseToComDetailsList(content []byte, format string) ([]ComDetails, error) {
	var comDetails []ComDetails
	switch format {
	case FormatJSON:
		if err := json.Unmarshal(content, &comDetails); err != nil {
			return nil, err
		}
	case FormatYAML:
		if err := yaml.Unmarshal(content, &comDetails); err != nil {
			return nil, err
		}
	case FormatCSV:
		if err := gocsv.UnmarshalBytes(content, &comDetails); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("invalid value for format must be (json,yaml,csv)")
	}
	return comDetails, nil
}
