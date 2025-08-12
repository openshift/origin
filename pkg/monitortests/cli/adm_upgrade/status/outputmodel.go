package admupgradestatus

import (
	"errors"
	"fmt"
	"strings"
)

type UpgradeStatusOutput struct {
	rawOutput           string
	updating            bool
	controlPlaneSection string
	workersSection      string
	healthMessages      []string
}

func NewUpgradeStatusOutput(output string) (*UpgradeStatusOutput, error) {
	output = strings.TrimSpace(output)

	if output == "The cluster is not updating." {
		return &UpgradeStatusOutput{
			rawOutput: output,
			updating:  false,
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

	healthMessages := parseHealthMessages(healthSection)

	return &UpgradeStatusOutput{
		rawOutput:           output,
		updating:            true,
		controlPlaneSection: controlPlaneSection,
		workersSection:      workersSection,
		healthMessages:      healthMessages,
	}, nil
}

func (u *UpgradeStatusOutput) IsUpdating() bool {
	return u.updating
}

func (u *UpgradeStatusOutput) ControlPlane() string {
	return u.controlPlaneSection
}

func (u *UpgradeStatusOutput) Workers() string {
	return u.workersSection
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
