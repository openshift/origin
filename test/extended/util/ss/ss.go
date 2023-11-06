package ss

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	exutil "github.com/openshift/origin/test/extended/util"
	corev1 "k8s.io/api/core/v1"

	"github.com/liornoy/node-comm-lib/pkg/consts"
	"github.com/liornoy/node-comm-lib/pkg/nodes"
	"github.com/liornoy/node-comm-lib/pkg/types"
)

const localAddrPortFieldIdx = 3

var (
	// TcpSSFilterFn is a function variable in Go that filters entries from the 'ss' command output.
	// It takes an entry from the 'ss' command output and returns true if the entry represents a TCP port in the listening state.
	tcpSSFilterFn = func(s string) bool {
		return strings.Contains(s, "127.0.0") || !strings.Contains(s, "LISTEN")
	}
	// UdpSSFilterFn is a function variable in Go that filters entries from the 'ss' command output.
	// It takes an entry from the 'ss' command output and returns true if the entry represents a UDP port in the listening state.
	udpSSFilterFn = func(s string) bool {
		return strings.Contains(s, "127.0.0") || !strings.Contains(s, "ESTAB")
	}
)

func ToComDetails(oc *exutil.CLI, node *corev1.Node, tcpfile, udpfile *os.File) ([]types.ComDetails, error) {
	ssTCPCommand := "ss -anplt"
	ssUDPCommand := "ss -anplu"
	res := []types.ComDetails{}
	f := oc.KubeFramework()

	ssOutTCP, err := exutil.ExecCommandOnMachineConfigDaemon(f.ClientSet, oc, node, []string{
		"sh", "-c", ssTCPCommand})
	if err != nil {
		return nil, fmt.Errorf("failed to exec ss: %w", err)
	}

	ssOutUDP, err := exutil.ExecCommandOnMachineConfigDaemon(f.ClientSet, oc, node, []string{
		"sh", "-c", ssUDPCommand})
	if err != nil {
		return nil, fmt.Errorf("failed to exec ss: %w", err)
	}

	ssOutFilteredTCP := filterStrings(tcpSSFilterFn, splitByLines(ssOutTCP))
	ssOutFilteredUDP := filterStrings(udpSSFilterFn, splitByLines(ssOutUDP))

	_, err = tcpfile.Write([]byte(fmt.Sprintf("node: %s\n%s", node.Name, strings.Join(ssOutFilteredTCP, "\n"))))
	if err != nil {
		return nil, fmt.Errorf("failed writing to file: %s", err)
	}
	_, err = tcpfile.Write([]byte(fmt.Sprintf("node: %s\n%s", node.Name, strings.Join(ssOutFilteredUDP, "\n"))))
	if err != nil {
		return nil, fmt.Errorf("failed writing to file: %s", err)
	}

	tcpComDetails, err := toComDetails(ssOutFilteredTCP, "TCP", node)
	if err != nil {
		return nil, err
	}
	udpComDetails, err := toComDetails(ssOutFilteredUDP, "UDP", node)
	if err != nil {
		return nil, err
	}

	res = append(res, udpComDetails...)
	res = append(res, tcpComDetails...)

	return res, nil
}

func splitByLines(str string) []string {
	return strings.Split(str, "\n")
}

func toComDetails(ssOutput []string, protocol string, node *corev1.Node) ([]types.ComDetails, error) {
	res := make([]types.ComDetails, 0)
	nodeRoles := nodes.GetRoles(node)

	for _, ssEntry := range ssOutput {
		cd, err := parseComDetail(ssEntry)
		if err != nil {
			return nil, err
		}

		cd.Protocol = protocol
		cd.NodeRole = nodeRoles
		cd.Optional = false
		res = append(res, *cd)
	}

	return res, nil
}

func filterStrings(filterOutFn func(string) bool, strs []string) []string {
	res := make([]string, 0)
	for _, s := range strs {
		if filterOutFn(s) {
			continue
		}

		res = append(res, s)
	}

	return res
}

func parseComDetail(ssEntry string) (*types.ComDetails, error) {
	serviceName, err := extractServiceName(ssEntry)
	if err != nil {
		return nil, err
	}

	fields := strings.Fields(ssEntry)
	portIdx := strings.LastIndex(fields[localAddrPortFieldIdx], ":")
	port := fields[localAddrPortFieldIdx][portIdx+1:]

	return &types.ComDetails{
		Direction: consts.IngressLabel,
		Port:      port,
		Service:   serviceName,
		Optional:  false}, nil
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
