package admupgradestatus

import (
	"errors"
	"fmt"
	"regexp"
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

	lines := strings.Split(output, "\n")
	parser := &parser{lines: lines, pos: 0}

	controlPlane, err := parser.parseControlPlaneSection()
	if err != nil {
		return nil, err
	}

	// Find worker and health sections manually for now (only control plane uses recursive descent)
	remainingLines := parser.lines[parser.pos:]
	remainingText := strings.Join(remainingLines, "\n")

	workerUpgradeStart := strings.Index(remainingText, "= Worker Upgrade =")
	updateHealthStart := strings.Index(remainingText, "= Update Health =")

	if workerUpgradeStart == -1 {
		return nil, errors.New("missing '= Worker Upgrade =' section in output")
	}

	if updateHealthStart == -1 {
		return nil, errors.New("missing '= Update Health =' section in output")
	}

	if workerUpgradeStart >= updateHealthStart {
		return nil, fmt.Errorf("'= Worker Upgrade =' section appears after '= Update Health =' section")
	}

	workersSection := strings.TrimSpace(remainingText[workerUpgradeStart+len("= Worker Upgrade =") : updateHealthStart])
	healthSection := strings.TrimSpace(remainingText[updateHealthStart+len("= Update Health ="):])

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

type parser struct {
	lines []string
	pos   int
}

var (
	updatingOperatorsHeader = regexp.MustCompile(`^NAME\s+SINCE\s+REASON\s+MESSAGE$`)
	nodesHeader             = regexp.MustCompile(`^NAME\s+ASSESSMENT\s+PHASE\s+VERSION\s+EST\s+MESSAGE$`)
)

func (p *parser) next() (string, bool) {
	if p.pos >= len(p.lines) {
		return "", true
	}

	line := strings.TrimSpace(p.lines[p.pos])
	p.pos++
	return line, false
}

func (p *parser) eatEmptyLines() error {
	for {
		line, done := p.next()
		if done {
			return errors.New("reached end of input while expecting empty lines")
		}
		if line != "" {
			p.pos--
			return nil
		}
	}
}

func (p *parser) eat(what string) error {
	line, done := p.next()
	if done {
		return fmt.Errorf("expected '%s' but reached end of input", what)
	}

	if line != what {
		return fmt.Errorf("expected '%s' but got '%s'", what, line)
	}

	return nil
}

func (p *parser) eatRegex(what *regexp.Regexp) error {
	line, done := p.next()
	if done {
		return fmt.Errorf("expected '%s' but reached end of input", what)
	}

	if !what.MatchString(line) {
		return fmt.Errorf("expected '%s' but got '%s'", what, line)
	}

	return nil
}

func (p *parser) parseControlPlaneSection() (*ControlPlaneStatus, error) {
	if err := p.eat("= Control Plane ="); err != nil {
		return nil, err
	}

	summary, err := p.parseControlPlaneSummary()
	if err != nil {
		return nil, err
	}

	operators, err := p.parseControlPlaneOperators()
	if err != nil {
		return nil, err
	}

	nodes, err := p.parseControlPlaneNodes()
	if err != nil {
		return nil, err
	}

	return &ControlPlaneStatus{
		summary:   summary,
		operators: operators,
		nodes:     nodes,
	}, nil
}

func (p *parser) parseControlPlaneSummary() (map[string]string, error) {
	p.eatEmptyLines()

	summary := map[string]string{}
	for {
		line, done := p.next()
		if done || line == "" {
			break
		}

		// Expect lines in the format "Key: Value"
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("expected 'Key: Value' format, got: %s", line)
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		summary[key] = value
	}

	if len(summary) == 0 {
		return nil, errors.New("found no entries in control plane summary section")
	}

	return summary, nil
}

func (p *parser) parseControlPlaneOperators() ([]string, error) {
	p.eatEmptyLines()

	if line, _ := p.next(); line != "Updating Cluster Operators" {
		// section is optional, put back the line and return nil
		p.pos--
		return nil, nil
	}

	if err := p.eatRegex(updatingOperatorsHeader); err != nil {
		return nil, fmt.Errorf("expected Updating Cluster Operators table header, got: %w", err)
	}

	var operators []string

	for {
		line, done := p.next()
		if done || line == "" {
			break
		}

		operators = append(operators, line)
	}

	if len(operators) == 0 {
		return nil, errors.New("found no entries in Updating Cluster Operators section")
	}

	return operators, nil
}

func (p *parser) parseControlPlaneNodes() ([]string, error) {
	p.eatEmptyLines()

	if p.eat("Control Plane Nodes") != nil {
		return nil, errors.New("expected 'Control Plane Nodes' section")
	}

	if err := p.eatRegex(nodesHeader); err != nil {
		return nil, fmt.Errorf("expected Control Plane Nodes table header: %w", err)
	}

	var nodes []string
	for {
		line, done := p.next()
		if done || line == "" {
			break
		}

		nodes = append(nodes, line)
	}

	if len(nodes) == 0 {
		return nil, errors.New("no nodes found in Control Plane Nodes section")
	}

	return nodes, nil
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
