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

	workers, err := parser.parseWorkerUpgradeSection()
	if err != nil {
		return nil, err
	}

	// Find health section manually for now
	remainingLines := parser.lines[parser.pos:]
	remainingText := strings.Join(remainingLines, "\n")

	updateHealthStart := strings.Index(remainingText, "= Update Health =")
	if updateHealthStart == -1 {
		return nil, errors.New("missing '= Update Health =' section in output")
	}

	healthSection := strings.TrimSpace(remainingText[updateHealthStart+len("= Update Health ="):])
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
	workerPoolsHeader       = regexp.MustCompile(`^WORKER POOL\s+ASSESSMENT\s+COMPLETION\s+STATUS$`)
)

func (p *parser) next() (string, bool) {
	if p.pos >= len(p.lines) {
		return "", true
	}

	line := strings.TrimSpace(p.lines[p.pos])
	p.pos++
	return line, false
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

func (p *parser) parseWorkerUpgradeSection() (*WorkersStatus, error) {
	p.eatEmptyLines()

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
		pools: pools,
		nodes: nodes,
	}, nil
}

func (p *parser) parseWorkerPools() ([]string, error) {
	p.eatEmptyLines()

	if err := p.eatRegex(workerPoolsHeader); err != nil {
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

	if err := p.eatRegex(nodesHeader); err != nil {
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
