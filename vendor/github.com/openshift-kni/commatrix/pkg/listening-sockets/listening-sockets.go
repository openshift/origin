package listeningsockets

import (
	"encoding/json"
	"fmt"
	"net"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"

	"github.com/openshift-kni/commatrix/pkg/client"
	"github.com/openshift-kni/commatrix/pkg/consts"
	"github.com/openshift-kni/commatrix/pkg/mcp"
	"github.com/openshift-kni/commatrix/pkg/types"
	"github.com/openshift-kni/commatrix/pkg/utils"
)

const (
	localAddrPortFieldIdx = 3
	interval              = time.Millisecond * 500
	duration              = time.Second * 5
)

type ConnectionCheck struct {
	*client.ClientSet
	podUtils    utils.UtilsInterface
	destDir     string
	nodeToGroup map[string]string
}

func NewCheck(c *client.ClientSet, podUtils utils.UtilsInterface, destDir string) (*ConnectionCheck, error) {
	// Try MCP-based resolution first
	if nodeToPool, err := mcp.ResolveNodeToPool(c); err == nil {
		return &ConnectionCheck{
			ClientSet:   c,
			podUtils:    podUtils,
			destDir:     destDir,
			nodeToGroup: nodeToPool,
		}, nil
	}

	// Fallback: build node->group map (HyperShift or clusters without MCP): prefer NodePool label, else role
	nodeToGroup, err := types.BuildNodeToGroupMap(c)
	if err != nil {
		return nil, err
	}
	return &ConnectionCheck{
		ClientSet:   c,
		podUtils:    podUtils,
		destDir:     destDir,
		nodeToGroup: nodeToGroup,
	}, nil
}

func (cc *ConnectionCheck) GenerateSS(namespace string) (*types.ComMatrix, []byte, []byte, error) {
	var ssOutTCP, ssOutUDP []byte
	nodesComDetails := []types.ComDetails{}

	nLock := &sync.Mutex{}
	g := new(errgroup.Group)
	for nodeName := range cc.nodeToGroup {
		name := nodeName
		g.Go(func() error {
			debugPod, err := cc.podUtils.CreatePodOnNode(name, namespace, consts.DefaultDebugPodImage, []string{})
			if err != nil {
				return err
			}

			err = cc.podUtils.WaitForPodStatus(namespace, debugPod, corev1.PodRunning)
			if err != nil {
				return err
			}

			defer func() {
				err := cc.podUtils.DeletePod(debugPod)
				if err != nil {
					log.Warningf("failed cleaning debug pod %s: %v", debugPod.Name, err)
				}
			}()

			group := cc.nodeToGroup[name]
			cds, ssTCP, ssUDP, err := cc.createSSOutputFromNode(debugPod, group)
			if err != nil {
				return err
			}
			nLock.Lock()
			defer nLock.Unlock()
			ssTCPLine := fmt.Sprintf("node: %s\n%s\n", name, string(ssTCP))
			ssUDPLine := fmt.Sprintf("node: %s\n%s\n", name, string(ssUDP))

			nodesComDetails = append(nodesComDetails, cds...)
			ssOutTCP = append(ssOutTCP, []byte(ssTCPLine)...)
			ssOutUDP = append(ssOutUDP, []byte(ssUDPLine)...)
			return nil
		})
	}

	err := g.Wait()
	if err != nil {
		return nil, nil, nil, err
	}

	ssComMat := types.ComMatrix{Matrix: nodesComDetails}
	ssComMat.SortAndRemoveDuplicates()

	return &ssComMat, ssOutTCP, ssOutUDP, nil
}

func (cc *ConnectionCheck) WriteSSRawFiles(ssOutTCP, ssOutUDP []byte) error {
	err := cc.podUtils.WriteFile(path.Join(cc.destDir, consts.SSRawTCP), ssOutTCP)
	if err != nil {
		return fmt.Errorf("failed writing to file: %s", err)
	}

	err = cc.podUtils.WriteFile(path.Join(cc.destDir, consts.SSRawUDP), ssOutUDP)
	if err != nil {
		return fmt.Errorf("failed writing to file: %s", err)
	}

	return nil
}

func (cc *ConnectionCheck) createSSOutputFromNode(debugPod *corev1.Pod, group string) ([]types.ComDetails, []byte, []byte, error) {
	ssOutTCP, err := cc.podUtils.RunCommandOnPod(debugPod, []string{"/bin/sh", "-c", "ss -anpltH"})
	if err != nil {
		return nil, nil, nil, err
	}
	ssOutUDP, err := cc.podUtils.RunCommandOnPod(debugPod, []string{"/bin/sh", "-c", "ss -anpluH"})
	if err != nil {
		return nil, nil, nil, err
	}

	loopbackIPs := cc.getLoopbackIPs(debugPod)
	ssOutFilteredTCP := filterEntries(splitByLines(ssOutTCP), loopbackIPs)
	ssOutFilteredUDP := filterEntries(splitByLines(ssOutUDP), loopbackIPs)

	tcpComDetails := cc.toComDetails(debugPod, ssOutFilteredTCP, "TCP", group)
	udpComDetails := cc.toComDetails(debugPod, ssOutFilteredUDP, "UDP", group)

	res := []types.ComDetails{}
	res = append(res, udpComDetails...)
	res = append(res, tcpComDetails...)

	return res, ssOutTCP, ssOutUDP, nil
}

func splitByLines(bytes []byte) []string {
	str := string(bytes)
	return strings.Split(str, "\n")
}

func (cc *ConnectionCheck) toComDetails(debugPod *corev1.Pod, ssOutput []string, protocol string, pool string) []types.ComDetails {
	res := make([]types.ComDetails, 0)

	for _, ssEntry := range ssOutput {
		cd := parseComDetail(ssEntry)

		containerName, nameSpace, podName := "", "", ""
		containerInfo, err := cc.getContainerInfo(debugPod, ssEntry)
		if err != nil {
			log.Debugf("failed to identify container for ss entry: %serr: %s", ssEntry, err)
		} else {
			containerName = containerInfo.Containers[0].Labels.ContainerName
			nameSpace = containerInfo.Containers[0].Labels.PodNamespace
			podName = containerInfo.Containers[0].Labels.PodName
		}

		cd.Container = containerName
		cd.Namespace = nameSpace
		cd.Pod = podName
		cd.Protocol = protocol
		cd.NodeGroup = pool
		cd.Optional = false
		res = append(res, *cd)
	}
	return res
}

// getContainerInfo receives an ss entry and gets the ContainerInfo obj of the container exposing this port.
func (cc *ConnectionCheck) getContainerInfo(debugPod *corev1.Pod, ssEntry string) (*types.ContainerInfo, error) {
	pid, err := extractPID(ssEntry)
	if err != nil {
		return nil, err
	}

	containerID, err := cc.extractContainerID(debugPod, pid)
	if err != nil {
		return nil, err
	}

	containerInfo, err := cc.extractContainerInfo(debugPod, containerID)
	if err != nil {
		return nil, err
	}

	return containerInfo, nil
}

// extractContainerID receives a PID of a container, and returns its CRI-O ID.
func (cc *ConnectionCheck) extractContainerID(debugPod *corev1.Pod, pid string) (string, error) {
	cmd := fmt.Sprintf("cat /proc/%s/cgroup", pid)
	out, err := cc.podUtils.RunCommandOnPod(debugPod, []string{"/bin/sh", "-c", cmd})
	if err != nil {
		return "", err
	}
	re := regexp.MustCompile(`crio-([0-9a-fA-F]+)\.scope`)

	match := re.FindStringSubmatch(string(out))

	if len(match) < 2 {
		return "", fmt.Errorf("container ID not found node:%s  pid: %s", debugPod.Spec.NodeName, pid)
	}

	containerID := match[1]
	return containerID, nil
}

// extractContainerInfo receives CRI-O container ID and returns the container's Info obj.
func (cc *ConnectionCheck) extractContainerInfo(debugPod *corev1.Pod, containerID string) (*types.ContainerInfo, error) {
	containerInfo := &types.ContainerInfo{}
	cmd := fmt.Sprintf("crictl ps -o json --id %s", containerID)
	out, err := cc.podUtils.RunCommandOnPod(debugPod, []string{"chroot", "/host", "/bin/sh", "-c", cmd})
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(out, &containerInfo)
	if err != nil {
		return nil, err
	}
	if len(containerInfo.Containers) != 1 {
		return nil, fmt.Errorf("failed extracting pod info, got %d results expected 1. got output:\n%s", len(containerInfo.Containers), string(out))
	}

	return containerInfo, nil
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

func filterEntries(ssEntries []string, loopbackIPs map[string]bool) []string {
	res := make([]string, 0)
	for _, s := range ssEntries {
		if s == "" {
			continue
		}
		if isLoopbackEntry(s, loopbackIPs) {
			continue
		}

		res = append(res, s)
	}

	return res
}

// isLoopbackEntry checks if an ss entry is listening on a loopback interface or its aliases.
func isLoopbackEntry(ssEntry string, loopbackIPs map[string]bool) bool {
	// Fast path for obvious loopback addresses
	if strings.Contains(ssEntry, "127.0.0") {
		return true
	}

	fields := strings.Fields(ssEntry)
	if len(fields) <= localAddrPortFieldIdx {
		return false
	}

	// Extract the local address from the ss entry (format typically "IP:PORT" or "[IPv6]:PORT")
	localAddrPort := fields[localAddrPortFieldIdx]
	localAddrPort = strings.Trim(localAddrPort, "[]")
	lastColonIdx := strings.LastIndex(localAddrPort, ":")
	if lastColonIdx == -1 {
		return false
	}
	ipStr := localAddrPort[:lastColonIdx]

	// Parse the IP address
	ip := net.ParseIP(ipStr)
	if ip == nil {
		// Fallback: simple checks
		return strings.Contains(ssEntry, "127.") || strings.Contains(ssEntry, "::1")
	}

	// Standard loopback range
	if ip.IsLoopback() {
		return true
	}

	// Check discovered loopback aliases
	if loopbackIPs[ip.String()] {
		return true
	}

	// IPv6 canonical without zone
	if ip.To4() == nil {
		if idx := strings.Index(ip.String(), "%"); idx != -1 {
			if loopbackIPs[ip.String()[:idx]] {
				return true
			}
		}
	}

	return false
}

func (cc *ConnectionCheck) getLoopbackIPs(debugPod *corev1.Pod) map[string]bool {
	type addr struct {
		Local string `json:"local"`
	}
	type iface struct {
		AddrInfo []addr `json:"addr_info"`
	}
	out, err := cc.podUtils.RunCommandOnPod(debugPod, []string{"chroot", "/host", "/bin/sh", "-c", "ip -j addr show lo"})
	if err != nil {
		return map[string]bool{}
	}
	var parsed []iface
	if json.Unmarshal(out, &parsed) != nil {
		return map[string]bool{}
	}
	ips := make(map[string]bool)
	for _, it := range parsed {
		for _, ai := range it.AddrInfo {
			if ai.Local != "" {
				ips[ai.Local] = true
			}
		}
	}
	return ips
}

func parseComDetail(ssEntry string) *types.ComDetails {
	serviceName, err := extractServiceName(ssEntry)
	if err != nil {
		log.Debug(err.Error())
	}

	fields := strings.Fields(ssEntry)
	portIdx := strings.LastIndex(fields[localAddrPortFieldIdx], ":")
	portStr := fields[localAddrPortFieldIdx][portIdx+1:]

	port, err := strconv.Atoi(portStr)
	if err != nil {
		log.Debug(err.Error())
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
