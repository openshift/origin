package admupgradestatus

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

type ControlPlaneStatus struct {
	Updated      bool
	Summary      map[string]string
	Operators    []string
	NodesUpdated bool
	Nodes        []string
}

type WorkersStatus struct {
	Pools []string
	Nodes map[string][]string
}

type Health struct {
	Detailed bool
	Messages []string
}

type upgradeStatusOutput struct {
	rawOutput    string
	updating     bool
	controlPlane *ControlPlaneStatus
	workers      *WorkersStatus
	health       *Health
}

var unableToFetchAlerts = regexp.MustCompile(`^Unable to fetch alerts.*`)

func newUpgradeStatusOutput(output string) (*upgradeStatusOutput, error) {
	output = strings.TrimSpace(output)

	if output == "The cluster is not updating." {
		return &upgradeStatusOutput{
			rawOutput:    output,
			updating:     false,
			controlPlane: nil,
			workers:      nil,
		}, nil
	}

	lines := strings.Split(output, "\n")
	parser := &parser{lines: lines, pos: 0}

	if parser.tryRegex(unableToFetchAlerts) {
		_ = parser.eatRegex(unableToFetchAlerts)
	}

	controlPlane, err := parser.parseControlPlaneSection()
	if err != nil {
		return nil, err
	}

	workers, err := parser.parseWorkerUpgradeSection()
	if err != nil {
		return nil, err
	}

	health, err := parser.parseHealthSection()
	if err != nil {
		return nil, err
	}

	return &upgradeStatusOutput{
		rawOutput:    output,
		updating:     true,
		controlPlane: controlPlane,
		workers:      workers,
		health:       health,
	}, nil
}

type parser struct {
	lines []string
	pos   int
}

var (
	updatingOperatorsHeaderPattern = regexp.MustCompile(`^NAME\s+SINCE\s+REASON\s+MESSAGE$`)
	nodesHeaderPattern             = regexp.MustCompile(`^NAME\s+ASSESSMENT\s+PHASE\s+VERSION\s+EST\s+MESSAGE$`)
	workerPoolsHeaderPattern       = regexp.MustCompile(`^WORKER POOL\s+ASSESSMENT\s+COMPLETION\s+STATUS$`)
	healthHeaderPattern            = regexp.MustCompile(`^SINCE\s+LEVEL\s+IMPACT\s+MESSAGE$`)

	workerUpgradeHeaderPattern      = regexp.MustCompile(`^= Worker Upgrade =$`)
	controlPlaneUpdatedPattern      = regexp.MustCompile(`^Update to .* successfully completed at .*$`)
	controlPlaneNodesUpdatedPattern = regexp.MustCompile(`^All control plane nodes successfully updated to .*`)
)

type nextOption int

const (
	preserveLeadingWhitespace nextOption = iota
)

func (p *parser) next(opts ...nextOption) (string, bool) {
	if p.pos >= len(p.lines) {
		return "", true
	}

	line := p.lines[p.pos]
	p.pos++

	// Check if we should preserve leading whitespace
	preserveLeading := false
	for _, opt := range opts {
		if opt == preserveLeadingWhitespace {
			preserveLeading = true
			break
		}
	}

	if preserveLeading {
		return strings.TrimRight(line, " \t\r\n"), false
	} else {
		return strings.TrimSpace(line), false
	}
}

func (p *parser) eatEmptyLines() {
	for {
		line, done := p.next()
		if done {
			return
		}
		if line != "" {
			p.pos--
			return
		}
	}
}

func (p *parser) tryRegex(what *regexp.Regexp) bool {
	line, done := p.next()
	p.pos--

	return !done && what.MatchString(line)
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

	var status ControlPlaneStatus

	if p.tryRegex(controlPlaneUpdatedPattern) {
		_ = p.eatRegex(controlPlaneUpdatedPattern)
		status.Updated = true
		p.eatEmptyLines()
		if err := p.eatRegex(controlPlaneNodesUpdatedPattern); err != nil {
			return nil, fmt.Errorf("expected 'All control plane nodes successfully updated to' message, got: %w", err)
		}
		status.NodesUpdated = true

		return &status, nil
	}

	summary, err := p.parseControlPlaneSummary()
	if err != nil {
		return nil, err
	}
	status.Summary = summary

	operators, err := p.parseControlPlaneOperators()
	if err != nil {
		return nil, err
	}
	status.Operators = operators

	p.eatEmptyLines()

	if p.tryRegex(controlPlaneNodesUpdatedPattern) {
		_ = p.eatRegex(controlPlaneNodesUpdatedPattern)
		status.NodesUpdated = true
	} else {
		nodes, err := p.parseControlPlaneNodes()
		if err != nil {
			return nil, err
		}
		status.Nodes = nodes
	}

	return &status, nil
}

func (p *parser) parseControlPlaneSummary() (map[string]string, error) {
	p.eatEmptyLines()

	summary := map[string]string{}
	for {
		line, done := p.next()
		if done || line == "" {
			break
		}

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

	if err := p.eatRegex(updatingOperatorsHeaderPattern); err != nil {
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

	if err := p.eatRegex(nodesHeaderPattern); err != nil {
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

func (p *parser) parseWorkerUpgradeSection() (*WorkersStatus, error) {
	p.eatEmptyLines()

	if !p.tryRegex(workerUpgradeHeaderPattern) {
		return nil, nil
	}

	if err := p.eat("= Worker Upgrade ="); err != nil {
		return nil, err
	}

	pools, err := p.parseWorkerPools()
	if err != nil {
		return nil, err
	}

	nodes, err := p.parseWorkerPoolNodes()
	if err != nil {
		return nil, err
	}

	return &WorkersStatus{
		Pools: pools,
		Nodes: nodes,
	}, nil
}

func (p *parser) parseWorkerPools() ([]string, error) {
	p.eatEmptyLines()

	if err := p.eatRegex(workerPoolsHeaderPattern); err != nil {
		return nil, fmt.Errorf("expected Worker Upgrade table header: %w", err)
	}

	var pools []string
	for {
		line, done := p.next()
		if done || line == "" {
			break
		}

		pools = append(pools, line)
	}

	if len(pools) == 0 {
		return nil, errors.New("no worker pools found in Worker Upgrade section")
	}

	return pools, nil
}

func (p *parser) parseWorkerPoolNodes() (map[string][]string, error) {
	nodes := make(map[string][]string)

	for {
		p.eatEmptyLines()

		name, entries, err := p.tryParseWorkerNodeTable()
		if err != nil {
			return nil, err
		}

		if name == "" {
			break
		}

		nodes[name] = entries
	}

	if len(nodes) == 0 {
		return nil, errors.New("no worker pool nodes found in Worker Upgrade section")
	}

	return nodes, nil
}

func (p *parser) tryParseWorkerNodeTable() (string, []string, error) {
	p.eatEmptyLines()

	line, done := p.next()
	if done {
		return "", nil, errors.New("expected 'Worker Pool Nodes:' section but reached end of input")
	}
	if !strings.HasPrefix(line, "Worker Pool Nodes:") {
		p.pos-- // put it back
		return "", nil, nil
	}

	name := strings.TrimPrefix(line, "Worker Pool Nodes: ")

	if err := p.eatRegex(nodesHeaderPattern); err != nil {
		return "", nil, fmt.Errorf("expected worker pool nodes table header for pool '%s': %w", name, err)
	}

	// Read node entries
	var nodeEntries []string
	for {
		line, done := p.next()
		if done || line == "" {
			break
		}

		nodeEntries = append(nodeEntries, line)
	}

	if len(nodeEntries) == 0 {
		return "", nil, fmt.Errorf("no nodes found for worker pool '%s'", name)
	}

	return name, nodeEntries, nil
}

func (p *parser) parseHealthSection() (*Health, error) {
	p.eatEmptyLines()

	if err := p.eat("= Update Health ="); err != nil {
		return nil, err
	}

	var health Health

	line, done := p.next()
	if done {
		return nil, errors.New("expected 'Update Health' section but reached end of input")
	}

	var getMessage func() (string, error)
	if strings.HasPrefix(line, "Message: ") {
		getMessage = p.parseHealthMessage
		health.Detailed = true
		p.pos--
	} else if healthHeaderPattern.MatchString(line) {
		getMessage = p.parseHealthMessageLine
	} else {
		return nil, fmt.Errorf("expected 'Update Health' to start with either a table header or a 'Message: ' line, got %s", line)
	}

	for {
		message, err := getMessage()
		if err != nil {
			return nil, err
		}

		if message == "" {
			// No more messages
			break
		}

		health.Messages = append(health.Messages, message)
	}

	if len(health.Messages) == 0 {
		return nil, errors.New("no health messages found in Update Health section")
	}

	return &health, nil
}

func (p *parser) parseHealthMessageLine() (string, error) {
	line, _ := p.next()
	return line, nil
}

func (p *parser) parseHealthMessage() (string, error) {
	var messageBuilder strings.Builder

	line, done := p.next()
	if done {
		return "", nil // No more input
	}

	if !strings.HasPrefix(line, "Message: ") {
		return "", fmt.Errorf("expected health message to start with 'Message: ', got: %s", line)
	}

	messageBuilder.WriteString(line)

	// Read continuation lines until we hit the next "Message: " or end of input
	for {
		line, done := p.next(preserveLeadingWhitespace)
		if done {
			break
		}

		if line == "" {
			peek, done := p.next()
			if done {
				break
			}
			p.pos--
			if strings.HasPrefix(peek, "Message: ") {
				break
			}
		}

		messageBuilder.WriteString("\n" + line)
	}

	return strings.TrimSpace(messageBuilder.String()), nil
}
