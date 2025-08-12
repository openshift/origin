package admupgradestatus

import (
	"errors"
	"fmt"
	"strings"
)

type ControlPlaneStatus struct {
	summary   map[string]string
	operators []string
	nodes     []string
}

type WorkersStatus struct {
	pools []string
	nodes map[string][]string
}

type UpgradeStatusOutput struct {
	rawOutput      string
	updating       bool
	controlPlane   *ControlPlaneStatus
	workers        *WorkersStatus
	healthMessages []string
}

func NewUpgradeStatusOutput(output string) (*UpgradeStatusOutput, error) {
	output = strings.TrimSpace(output)

	if output == "The cluster is not updating." {
		return &UpgradeStatusOutput{
			rawOutput:    output,
			updating:     false,
			controlPlane: nil,
			workers:      nil,
		}, nil
	}

	if !strings.Contains(output, "= Control Plane =") {
		return nil, errors.New("missing '= Control Plane =' section in output")
	}

	if !strings.Contains(output, "= Worker Upgrade =") {
		return nil, errors.New("missing '= Worker Upgrade =' section in output")
	}

	if !strings.Contains(output, "= Update Health =") {
		return nil, errors.New("missing '= Update Health =' section in output")
	}

	controlPlaneStart := strings.Index(output, "= Control Plane =")
	workerUpgradeStart := strings.Index(output, "= Worker Upgrade =")
	updateHealthStart := strings.Index(output, "= Update Health =")

	if controlPlaneStart >= workerUpgradeStart {
		return nil, fmt.Errorf("'= Control Plane =' section appears after '= Worker Upgrade =' section")
	}

	if workerUpgradeStart >= updateHealthStart {
		return nil, fmt.Errorf("'= Worker Upgrade =' section appears after '= Update Health =' section")
	}

	controlPlaneSection := strings.TrimSpace(output[controlPlaneStart+len("= Control Plane =") : workerUpgradeStart])
	workersSection := strings.TrimSpace(output[workerUpgradeStart+len("= Worker Upgrade =") : updateHealthStart])
	healthSection := strings.TrimSpace(output[updateHealthStart+len("= Update Health ="):])

	controlPlane, err := parseControlPlane(controlPlaneSection)
	if err != nil {
		return nil, err
	}

	workers, err := parseWorkers(workersSection)
	if err != nil {
		return nil, err
	}

	healthMessages := parseHealthMessages(healthSection)

	return &UpgradeStatusOutput{
		rawOutput:      output,
		updating:       true,
		controlPlane:   controlPlane,
		workers:        workers,
		healthMessages: healthMessages,
	}, nil
}

func (u *UpgradeStatusOutput) IsUpdating() bool {
	return u.updating
}

func (u *UpgradeStatusOutput) ControlPlane() *ControlPlaneStatus {
	return u.controlPlane
}

func (c *ControlPlaneStatus) Summary() map[string]string {
	return c.summary
}

func (c *ControlPlaneStatus) Operators() []string {
	return c.operators
}

func (c *ControlPlaneStatus) Nodes() []string {
	return c.nodes
}

func (u *UpgradeStatusOutput) Workers() *WorkersStatus {
	return u.workers
}

func (w *WorkersStatus) Pools() []string {
	return w.pools
}

func (w *WorkersStatus) Nodes() map[string][]string {
	return w.nodes
}

func (u *UpgradeStatusOutput) Health() []string {
	return u.healthMessages
}

func parseHealthMessages(healthSection string) []string {
	if healthSection == "" {
		return nil
	}

	lines := strings.Split(healthSection, "\n")
	var messages []string
	var currentMessage strings.Builder

	for i, line := range lines {
		if strings.HasPrefix(line, "Message: ") {
			if currentMessage.Len() > 0 {
				messages = append(messages, strings.TrimSpace(currentMessage.String()))
				currentMessage.Reset()
			}
			currentMessage.WriteString(line)
		} else if currentMessage.Len() > 0 {
			currentMessage.WriteString("\n" + line)
		}

		if i == len(lines)-1 && currentMessage.Len() > 0 {
			messages = append(messages, strings.TrimSpace(currentMessage.String()))
		}
	}

	return messages
}

func parseControlPlane(controlPlaneSection string) (*ControlPlaneStatus, error) {
	lines := strings.Split(controlPlaneSection, "\n")

	var summaryLines []string
	var operators []string
	var nodes []string

	state := "summary"
	hasOperatorsSection := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if line == "Updating Cluster Operators" {
			hasOperatorsSection = true
			state = "operators_header"
			continue
		} else if line == "Control Plane Nodes" {
			state = "nodes_header"
			continue
		}

		switch state {
		case "summary":
			summaryLines = append(summaryLines, line)
		case "operators_header":
			if strings.Contains(line, "NAME") && strings.Contains(line, "SINCE") {
				state = "operators"
				continue
			}
			summaryLines = append(summaryLines, line)
		case "operators":
			if strings.Contains(line, "NAME") && strings.Contains(line, "ASSESSMENT") {
				state = "nodes"
				continue
			}
			operators = append(operators, line)
		case "nodes_header":
			if strings.Contains(line, "NAME") && strings.Contains(line, "ASSESSMENT") {
				state = "nodes"
				continue
			}
			if hasOperatorsSection {
				operators = append(operators, line)
			} else {
				summaryLines = append(summaryLines, line)
			}
		case "nodes":
			nodes = append(nodes, line)
		}
	}

	if hasOperatorsSection && len(operators) == 0 {
		return nil, errors.New("Updating Cluster Operators section found but no operator entries present")
	}

	if len(nodes) == 0 {
		return nil, errors.New("no nodes found in Control Plane section")
	}

	summaryMap := make(map[string]string)
	for _, line := range summaryLines {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			summaryMap[key] = value
		}
	}

	var operatorsResult []string
	if hasOperatorsSection {
		operatorsResult = operators
	} else {
		operatorsResult = nil
	}

	return &ControlPlaneStatus{
		summary:   summaryMap,
		operators: operatorsResult,
		nodes:     nodes,
	}, nil
}

func parseWorkers(workersSection string) (*WorkersStatus, error) {
	lines := strings.Split(workersSection, "\n")

	// Parse pools table first
	pools, remainingLines, err := parsePoolsTable(lines)
	if err != nil {
		return nil, err
	}

	// Parse worker pool nodes tables
	nodes, err := parseWorkerPoolNodesTables(remainingLines)
	if err != nil {
		return nil, err
	}

	return &WorkersStatus{
		pools: pools,
		nodes: nodes,
	}, nil
}

func parsePoolsTable(lines []string) ([]string, []string, error) {
	var pools []string
	foundHeader := false

	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.Contains(line, "WORKER POOL") && strings.Contains(line, "ASSESSMENT") && strings.Contains(line, "COMPLETION") && strings.Contains(line, "STATUS") {
			foundHeader = true
			continue
		}

		if foundHeader {
			if strings.HasPrefix(line, "Worker Pool Nodes:") {
				return pools, lines[i:], nil
			}
			pools = append(pools, line)
		}
	}

	if !foundHeader {
		return nil, nil, errors.New("missing 'WORKER POOL   ASSESSMENT   COMPLETION   STATUS' header in Worker Upgrade section")
	}

	if len(pools) == 0 {
		return nil, nil, errors.New("no worker pools found in Worker Upgrade section")
	}

	return pools, nil, nil
}

func parseWorkerPoolNodesTables(lines []string) (map[string][]string, error) {
	nodes := make(map[string][]string)

	for len(lines) > 0 {
		poolName, nodeEntries, remainingLines, err := parseWorkerPoolNodesTable(lines)
		if err != nil {
			if strings.Contains(err.Error(), "no more Worker Pool Nodes sections found") {
				break
			}
			return nil, err
		}

		nodes[poolName] = nodeEntries
		lines = remainingLines
	}

	return nodes, nil
}

func parseWorkerPoolNodesTable(lines []string) (string, []string, []string, error) {
	// Skip empty lines
	i := 0
	for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
		i++
	}

	if i >= len(lines) {
		return "", nil, nil, errors.New("no more Worker Pool Nodes sections found")
	}

	// Expect "Worker Pool Nodes: XXX" header
	line := strings.TrimSpace(lines[i])
	if !strings.HasPrefix(line, "Worker Pool Nodes:") {
		return "", nil, nil, fmt.Errorf("expected 'Worker Pool Nodes: XXX' header, got: %s", line)
	}

	poolName := strings.TrimSpace(strings.TrimPrefix(line, "Worker Pool Nodes:"))
	i++

	// Skip empty lines
	for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
		i++
	}

	if i >= len(lines) {
		return "", nil, nil, fmt.Errorf("expected table header for worker pool '%s'", poolName)
	}

	// Expect table header
	headerLine := strings.TrimSpace(lines[i])
	if !strings.Contains(headerLine, "NAME") || !strings.Contains(headerLine, "ASSESSMENT") {
		return "", nil, nil, fmt.Errorf("expected table header for worker pool '%s', got: %s", poolName, headerLine)
	}
	i++

	// Read node entries until end or next "Worker Pool Nodes:" section
	var nodeEntries []string
	for i < len(lines) {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			i++
			continue
		}

		if strings.HasPrefix(line, "Worker Pool Nodes:") {
			break
		}

		nodeEntries = append(nodeEntries, line)
		i++
	}

	if len(nodeEntries) == 0 {
		return "", nil, nil, fmt.Errorf("no nodes found for worker pool '%s'", poolName)
	}

	return poolName, nodeEntries, lines[i:], nil
}
