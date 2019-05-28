package cmd

import (
	"os"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/klog"
)

// SiblingCommand returns a sibling command to the given command
func SiblingCommand(cmd *cobra.Command, name string) []string {
	c := cmd.Parent()
	command := []string{}
	for c != nil {
		klog.V(5).Infof("Found parent command: %s", c.Name())
		command = append([]string{c.Name()}, command...)
		c = c.Parent()
	}
	// Replace the root command with what was actually used
	// in the command line
	klog.V(4).Infof("Setting root command to: %s", os.Args[0])
	command[0] = os.Args[0]

	// Append the sibling command
	command = append(command, name)
	klog.V(4).Infof("The sibling command is: %s", strings.Join(command, " "))

	return command
}
