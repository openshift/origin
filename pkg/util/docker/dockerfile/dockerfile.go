package dockerfile

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/docker/docker/builder/dockerfile/command"
	"github.com/docker/docker/builder/dockerfile/parser"
	"github.com/fsouza/go-dockerclient"
)

var portRangeRegexp = regexp.MustCompile(`^(\d+)-(\d+)$`)
var argSplitRegexp = regexp.MustCompile(`^([a-zA-Z_]+[a-zA-Z0-9_]*)=(.*)$`)

// FindAll returns the indices of all children of node such that
// node.Children[i].Value == cmd. Valid values for cmd are defined in the
// package github.com/docker/docker/builder/dockerfile/command.
func FindAll(node *parser.Node, cmd string) []int {
	if node == nil {
		return nil
	}
	var indices []int
	for i, child := range node.Children {
		if child != nil && child.Value == cmd {
			indices = append(indices, i)
		}
	}
	return indices
}

// InsertInstructions inserts instructions starting from the pos-th child of
// node, moving other children as necessary. The instructions should be valid
// Dockerfile instructions. InsertInstructions mutates node in-place, and the
// final state of node is equivalent to what parser.Parse would return if the
// original Dockerfile represented by node contained the instructions at the
// specified position pos. If the returned error is non-nil, node is guaranteed
// to be unchanged.
func InsertInstructions(node *parser.Node, pos int, instructions string) error {
	if node == nil {
		return fmt.Errorf("cannot insert instructions in a nil node")
	}
	if pos < 0 || pos > len(node.Children) {
		return fmt.Errorf("pos %d out of range [0, %d]", pos, len(node.Children)-1)
	}
	newChild, err := parser.Parse(strings.NewReader(instructions))
	if err != nil {
		return err
	}
	// InsertVector pattern (https://github.com/golang/go/wiki/SliceTricks)
	node.Children = append(node.Children[:pos], append(newChild.AST.Children, node.Children[pos:]...)...)
	return nil
}

// LastBaseImage takes a Dockerfile root node and returns the base image
// declared in the last FROM instruction.
func LastBaseImage(node *parser.Node) string {
	baseImages := baseImages(node)
	if len(baseImages) == 0 {
		return ""
	}
	return baseImages[len(baseImages)-1]
}

// baseImages takes a Dockerfile root node and returns a list of all base images
// declared in the Dockerfile. Each base image is the argument of a FROM
// instruction.
func baseImages(node *parser.Node) []string {
	var images []string
	for _, pos := range FindAll(node, command.From) {
		images = append(images, nextValues(node.Children[pos])...)
	}
	return images
}

func evalRange(port string) string {
	m, match := match(portRangeRegexp, port)
	if !match {
		return port
	}
	_, err := strconv.ParseUint(m[1], 10, 16)
	if err != nil {
		return port
	}
	return m[1]
}

func evalPorts(exposedPorts []string, node *parser.Node, from, to int) []string {
	shlex := NewShellLex('\\')
	shlex.ProcessWord("w", []string{})
	envs := evalVars(node, from, to, exposedPorts, shlex)
	ports := make([]string, 0, len(exposedPorts))
	for _, p := range exposedPorts {
		dp := docker.Port(p)
		port := dp.Port()
		port, err := shlex.ProcessWord(port, envs)
		if err != nil {
			//keep the exposed port as was
			port = dp.Port()
		}
		port = evalRange(port)
		if strings.Contains(p, `/`) {
			p = port + `/` + dp.Proto()
		} else {
			p = port
		}
		ports = append(ports, p)
	}
	return ports
}

// LastExposedPorts takes a Dockerfile root node and returns a list of ports
// exposed in the last image built by the Dockerfile, i.e., only the EXPOSE
// instructions after the last FROM instruction are considered.
//
// It also evaluates the following scenarios
// 1) env variable - evaluate from ENV and ARG with default value
// 2) port range - adding the lowest port from range
func LastExposedPorts(node *parser.Node) []string {
	exposedPorts, exposeIndices := exposedPorts(node)
	if len(exposedPorts) == 0 || len(exposeIndices) == 0 {
		return nil
	}
	lastExposed := exposedPorts[len(exposedPorts)-1]
	froms := FindAll(node, command.From)
	from := froms[len(froms)-1]
	to := exposeIndices[len(exposeIndices)-1]
	return evalPorts(lastExposed, node, from, to)
}

// exposedPorts takes a Dockerfile root node and returns a list of all ports
// exposed in the Dockerfile, grouped by images that this Dockerfile produces.
// The number of port lists returned is the number of images produced by this
// Dockerfile, which is the same as the number of FROM instructions.
func exposedPorts(node *parser.Node) ([][]string, []int) {
	var allPorts [][]string
	var ports []string
	froms := FindAll(node, command.From)
	exposes := FindAll(node, command.Expose)
	for i, j := len(froms)-1, len(exposes)-1; i >= 0; i-- {
		for ; j >= 0 && exposes[j] > froms[i]; j-- {
			ports = append(nextValues(node.Children[exposes[j]]), ports...)
		}
		allPorts = append([][]string{ports}, allPorts...)
		ports = nil
	}
	return allPorts, exposes
}

// nextValues returns a slice of values from the next nodes following node. This
// roughly translates to the arguments to the Docker builder instruction
// represented by node.
func nextValues(node *parser.Node) []string {
	if node == nil {
		return nil
	}
	var values []string
	for next := node.Next; next != nil; next = next.Next {
		values = append(values, next.Value)
	}
	return values
}

func match(r *regexp.Regexp, str string) ([]string, bool) {
	m := r.FindStringSubmatch(str)
	return m, len(m) == r.NumSubexp()+1
}

func containsVars(ports []string) bool {
	for _, p := range ports {
		if strings.Contains(p, `$`) {
			return true
		}
	}
	return false
}

// evalVars is a best effort evaluation of ENV and ARG labels in a dockerfile
// in order to provide support for variables in EXPOSE label
// It returns list of evaluated variables as used by ShellLex - ["var=val", ...]
func evalVars(n *parser.Node, from, to int, ports []string, shlex *ShellLex) []string {
	envs := make([]string, 0)
	if !containsVars(ports) {
		return envs
	}
	for i := from; i < to; i++ {
		switch n.Children[i].Value {
		case command.Arg:
			args := nextValues(n.Children[i])
			if len(args) == 1 {
				//silently skip ARG without default value
				if _, match := match(argSplitRegexp, args[0]); match {
					processed, err := shlex.ProcessWord(args[0], envs)
					if err == nil {
						envs = append(envs, processed)
					}
				}
			}
		case command.Env:
			args := nextValues(n.Children[i])
			for j := 0; j < len(args)-1; j += 2 {
				processed, err := shlex.ProcessWord(args[j+1], envs)
				if err == nil {
					envs = append(envs, args[j]+"="+processed)
				}
			}
		}
	}
	return envs
}
