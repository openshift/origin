package ss

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"

	"github.com/openshift-kni/commatrix/consts"
	"github.com/openshift-kni/commatrix/debug"
	"github.com/openshift-kni/commatrix/nodes"
	"github.com/openshift-kni/commatrix/types"
)

const (
	localAddrPortFieldIdx = 3
	interval              = time.Millisecond * 500
	duration              = time.Second * 5
)

func CreateSSOutputFromNode(debugPod *debug.DebugPod, node *corev1.Node) (res []types.ComDetails, ssOutTCP, ssOutUDP []byte, err error) {
	ssOutTCP, err = debugPod.ExecWithRetry("ss -anpltH", interval, duration)
	if err != nil {
		return nil, nil, nil, err
	}
	ssOutUDP, err = debugPod.ExecWithRetry("ss -anpluH", interval, duration)
	if err != nil {
		return nil, nil, nil, err
	}

	ssOutFilteredTCP := filterEntries(splitByLines(ssOutTCP))
	ssOutFilteredUDP := filterEntries(splitByLines(ssOutUDP))

	tcpComDetails := toComDetails(debugPod, ssOutFilteredTCP, "TCP", node)
	udpComDetails := toComDetails(debugPod, ssOutFilteredUDP, "UDP", node)

	res = []types.ComDetails{}
	res = append(res, udpComDetails...)
	res = append(res, tcpComDetails...)

	return res, ssOutTCP, ssOutUDP, nil
}

func splitByLines(bytes []byte) []string {
	str := string(bytes)
	return strings.Split(str, "\n")
}

func toComDetails(debugPod *debug.DebugPod, ssOutput []string, protocol string, node *corev1.Node) []types.ComDetails {
	res := make([]types.ComDetails, 0)
	nodeRoles := nodes.GetRole(node)

	for _, ssEntry := range ssOutput {
		cd := parseComDetail(ssEntry)

		name, err := getContainerName(debugPod, ssEntry)
		if err != nil {
			log.Debugf("failed to identify container for ss entry: %serr: %s", ssEntry, err)
		}

		cd.Container = name
		cd.Protocol = protocol
		cd.NodeRole = nodeRoles
		cd.Optional = false
		res = append(res, *cd)
	}

	return res
}

// getContainerName receives an ss entry and gets the name of the container exposing this port.
func getContainerName(debugPod *debug.DebugPod, ssEntry string) (string, error) {
	pid, err := extractPID(ssEntry)
	if err != nil {
		return "", err
	}

	containerID, err := extractContainerID(debugPod, pid)
	if err != nil {
		return "", err
	}

	res, err := extractContainerName(debugPod, containerID)
	if err != nil {
		return "", err
	}

	return res, nil
}

// extractPID receives an ss entry and returns the PID number of it.
func extractPID(ssEntry string) (string, error) {
	re := regexp.MustCompile(`pid=(\d+)`)

	match := re.FindStringSubmatch(ssEntry)

	if len(match) < 2 {
		return "", fmt.Errorf("PID not found in the input string")
	}

	pid := match[1]
	return pid, nil
}

// extractContainerID receives a PID of a container, and returns its CRI-O ID.
func extractContainerID(debugPod *debug.DebugPod, pid string) (string, error) {
	cmd := fmt.Sprintf("cat /proc/%s/cgroup", pid)
	out, err := debugPod.ExecWithRetry(cmd, interval, duration)
	if err != nil {
		return "", err
	}
	re := regexp.MustCompile(`crio-([0-9a-fA-F]+)\.scope`)

	match := re.FindStringSubmatch(string(out))

	if len(match) < 2 {
		return "", fmt.Errorf("container ID not found node:%s  pid: %s", debugPod.NodeName, pid)
	}

	containerID := match[1]
	return containerID, nil
}

// extractContainerName receives CRI-O container ID and returns the container's name.
func extractContainerName(debugPod *debug.DebugPod, containerID string) (string, error) {
	type ContainerInfo struct {
		Containers []struct {
			Labels struct {
				ContainerName string `json:"io.kubernetes.container.name"`
				PodName       string `json:"io.kubernetes.pod.name"`
				PodNamespace  string `json:"io.kubernetes.pod.namespace"`
			} `json:"labels"`
		} `json:"containers"`
	}
	containerInfo := &ContainerInfo{}
	cmd := fmt.Sprintf("crictl ps -o json --id %s", containerID)

	out, err := debugPod.ExecWithRetry(cmd, interval, duration)
	if err != nil {
		return "", err
	}

	err = json.Unmarshal(out, &containerInfo)
	if err != nil {
		return "", err
	}
	if len(containerInfo.Containers) != 1 {
		return "", fmt.Errorf("failed extracting pod info, got %d results expected 1. got output:\n%s", len(containerInfo.Containers), string(out))
	}

	containerName := containerInfo.Containers[0].Labels.ContainerName

	return containerName, nil
}

func filterEntries(ssEntries []string) []string {
	res := make([]string, 0)
	for _, s := range ssEntries {
		if strings.Contains(s, "127.0.0") || strings.Contains(s, "::1") || s == "" {
			continue
		}

		res = append(res, s)
	}

	return res
}

func parseComDetail(ssEntry string) *types.ComDetails {
	serviceName, err := extractServiceName(ssEntry)
	if err != nil {
		log.Debugf(err.Error())
	}

	fields := strings.Fields(ssEntry)
	portIdx := strings.LastIndex(fields[localAddrPortFieldIdx], ":")
	portStr := fields[localAddrPortFieldIdx][portIdx+1:]

	port, err := strconv.Atoi(portStr)
	if err != nil {
		log.Debugf(err.Error())
		return nil
	}

	return &types.ComDetails{
		Direction: consts.IngressLabel,
		Port:      port,
		Service:   serviceName,
		Optional:  false}
}

func extractServiceName(ssEntry string) (string, error) {
	re := regexp.MustCompile(`users:\(\("(?P<servicename>[^"]+)"`)

	match := re.FindStringSubmatch(ssEntry)

	if len(match) < 2 {
		return "", fmt.Errorf("service name not found in the input string: %s", ssEntry)
	}

	serviceName := match[re.SubexpIndex("servicename")]

	return serviceName, nil
}
