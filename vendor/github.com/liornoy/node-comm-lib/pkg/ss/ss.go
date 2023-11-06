package ss

import (
	"bufio"
	"strings"

	"github.com/liornoy/node-comm-lib/pkg/commatrix"
)

func ToComDetails(ssOutput string, role string, protocol string) []commatrix.ComDetails {
	res := make([]commatrix.ComDetails, 0)
	reader := strings.NewReader(ssOutput)
	scanner := bufio.NewScanner(reader)

	for scanner.Scan() {
		line := scanner.Text()

		if skipSSline(line, protocol) {
			continue
		}

		comDetail := defineComDetail(line, protocol, role)
		res = append(res, comDetail)
	}

	return res
}

func skipSSline(line, protocol string) bool {
	fields := strings.Fields(line)

	shouldSkip := strings.Contains(line, "127.0.0") ||
		protocol == "TCP" && !strings.Contains(line, "LISTEN") ||
		protocol == "UDP" && !strings.Contains(line, "ESTAB") ||
		len(fields) != 6

	return shouldSkip
}

func defineComDetail(line string, protocol string, role string) commatrix.ComDetails {
	optionalProcesses := map[string]bool{
		"rpcbind":   false,
		"sshd":      false,
		"rpc.statd": false,
	}
	fields := strings.Fields(line)
	processes := getStrBetweenDoubleQuotes(fields[5])
	mainProcess := processes[0]

	idx := strings.LastIndex(fields[3], ":")
	port := fields[3][idx+1:]

	required := true
	if _, ok := optionalProcesses[mainProcess]; ok {
		required = false
	}

	return commatrix.ComDetails{
		Direction:   "ingress",
		Protocol:    protocol,
		Port:        port,
		NodeRole:    role,
		ServiceName: mainProcess,
		Required:    required}
}

func getStrBetweenDoubleQuotes(s string) []string {
	res := make([]string, 0)
	for idx, endIdx := 0, 0; strings.Contains(s, "\""); s = s[idx+endIdx+2:] {
		idx = strings.Index(s, "\"")
		endIdx = strings.Index(s[idx+1:], "\"")
		res = append(res, s[idx+1:idx+1+endIdx])
	}

	return res
}
