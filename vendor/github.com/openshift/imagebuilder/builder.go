package imagebuilder

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	docker "github.com/fsouza/go-dockerclient"

	"github.com/docker/docker/builder/dockerfile/command"
	"github.com/docker/docker/builder/dockerfile/parser"
)

// Copy defines a copy operation required on the container.
type Copy struct {
	Src      string
	Dest     []string
	Download bool
}

// Run defines a run operation required in the container.
type Run struct {
	Shell bool
	Args  []string
}

type Executor interface {
	Copy(copies ...Copy) error
	Run(run Run, config docker.Config) error
	UnrecognizedInstruction(step *Step) error
}

type logExecutor struct{}

func (logExecutor) Copy(copies ...Copy) error {
	for _, c := range copies {
		log.Printf("COPY %s -> %v (download:%t)", c.Src, c.Dest, c.Download)
	}
	return nil
}

func (logExecutor) Run(run Run, config docker.Config) error {
	log.Printf("RUN %v %t (%v)", run.Args, run.Shell, config.Env)
	return nil
}

func (logExecutor) UnrecognizedInstruction(step *Step) error {
	log.Printf("Unknown instruction: %s", strings.ToUpper(step.Command))
	return nil
}

type noopExecutor struct{}

func (noopExecutor) Copy(copies ...Copy) error {
	return nil
}

func (noopExecutor) Run(run Run, config docker.Config) error {
	return nil
}

func (noopExecutor) UnrecognizedInstruction(step *Step) error {
	return nil
}

var (
	LogExecutor  = logExecutor{}
	NoopExecutor = noopExecutor{}
)

type Builder struct {
	RunConfig docker.Config

	Env    []string
	Args   map[string]string
	CmdSet bool
	Author string

	AllowedArgs map[string]bool

	PendingRuns   []Run
	PendingCopies []Copy

	Executor Executor
}

func NewBuilder() *Builder {
	args := make(map[string]bool)
	for k, v := range builtinAllowedBuildArgs {
		args[k] = v
	}
	return &Builder{
		Args:        make(map[string]string),
		AllowedArgs: args,
	}
}

// Step creates a new step from the current state.
func (b *Builder) Step() *Step {
	dst := make([]string, len(b.Env)+len(b.RunConfig.Env))
	copy(dst, b.Env)
	dst = append(dst, b.RunConfig.Env...)
	return &Step{Env: dst}
}

// Run executes a step, transforming the current builder and
// invoking any Copy or Run operations.
func (b *Builder) Run(step *Step, exec Executor) error {
	fn, ok := evaluateTable[step.Command]
	if !ok {
		return exec.UnrecognizedInstruction(step)
	}
	if err := fn(b, step.Args, step.Attrs, step.Original); err != nil {
		return err
	}

	copies := b.PendingCopies
	b.PendingCopies = nil
	runs := b.PendingRuns
	b.PendingRuns = nil

	if err := exec.Copy(copies...); err != nil {
		return err
	}
	for _, run := range runs {
		config := b.Config()
		if err := exec.Run(run, *config); err != nil {
			return err
		}
	}

	return nil
}

// RequiresStart returns true if a running container environment is necessary
// to invoke the provided commands
func (b *Builder) RequiresStart(node *parser.Node) bool {
	for _, child := range node.Children {
		if child.Value == command.Run {
			return true
		}
	}
	return false
}

// Config returns a snapshot of the current RunConfig intended for
// use with a container commit.
func (b *Builder) Config() *docker.Config {
	config := b.RunConfig
	if config.OnBuild == nil {
		config.OnBuild = []string{}
	}
	if config.Entrypoint == nil {
		config.Entrypoint = []string{}
	}
	config.Image = ""
	return &config
}

// Arguments returns the currently active arguments.
func (b *Builder) Arguments() []string {
	var envs []string
	for key, val := range b.Args {
		if _, ok := b.AllowedArgs[key]; ok {
			envs = append(envs, fmt.Sprintf("%s=%s", key, val))
		}
	}
	return envs
}

// ErrNoFROM is returned if the Dockerfile did not contain a FROM
// statement.
var ErrNoFROM = fmt.Errorf("no FROM statement found")

// From returns the image this dockerfile depends on, or an error
// if no FROM is found or if multiple FROM are specified. If a
// single from is found the passed node is updated with only
// the remaining statements.  The builder's RunConfig.Image field
// is set to the first From found, or left unchanged if already
// set.
func (b *Builder) From(node *parser.Node) (string, error) {
	children := SplitChildren(node, command.From)
	switch {
	case len(children) == 0:
		return "", ErrNoFROM
	case len(children) > 1:
		return "", fmt.Errorf("multiple FROM statements are not supported")
	default:
		step := b.Step()
		if err := step.Resolve(children[0]); err != nil {
			return "", err
		}
		if err := b.Run(step, NoopExecutor); err != nil {
			return "", err
		}
		return b.RunConfig.Image, nil
	}
}

// FromImage updates the builder to use the provided image (resetting RunConfig
// and recording the image environment), and updates the node with any ONBUILD
// statements extracted from the parent image.
func (b *Builder) FromImage(image *docker.Image, node *parser.Node) error {
	SplitChildren(node, command.From)

	b.RunConfig = *image.Config
	b.Env = b.RunConfig.Env
	b.RunConfig.Env = nil

	// Check to see if we have a default PATH, note that windows won't
	// have one as its set by HCS
	if runtime.GOOS != "windows" && !hasEnvName(b.Env, "PATH") {
		b.RunConfig.Env = append(b.RunConfig.Env, "PATH="+defaultPathEnv)
	}

	// Join the image onbuild statements into node
	if image.Config == nil || len(image.Config.OnBuild) == 0 {
		return nil
	}
	extra, err := parser.Parse(bytes.NewBufferString(strings.Join(image.Config.OnBuild, "\n")))
	if err != nil {
		return err
	}
	for _, child := range extra.Children {
		switch strings.ToUpper(child.Value) {
		case "ONBUILD":
			return fmt.Errorf("Chaining ONBUILD via `ONBUILD ONBUILD` isn't allowed")
		case "MAINTAINER", "FROM":
			return fmt.Errorf("%s isn't allowed as an ONBUILD trigger", child.Value)
		}
	}
	node.Children = append(extra.Children, node.Children...)
	// Since we've processed the OnBuild statements, clear them from the runconfig state.
	b.RunConfig.OnBuild = nil
	return nil
}

// SplitChildren removes any children with the provided value from node
// and returns them as an array. node.Children is updated.
func SplitChildren(node *parser.Node, value string) []*parser.Node {
	var split []*parser.Node
	var children []*parser.Node
	for _, child := range node.Children {
		if child.Value == value {
			split = append(split, child)
		} else {
			children = append(children, child)
		}
	}
	node.Children = children
	return split
}

// StepFunc is invoked with the result of a resolved step.
type StepFunc func(*Builder, []string, map[string]bool, string) error

var evaluateTable = map[string]StepFunc{
	command.Env:        env,
	command.Label:      label,
	command.Maintainer: maintainer,
	command.Add:        add,
	command.Copy:       dispatchCopy, // copy() is a go builtin
	command.From:       from,
	command.Onbuild:    onbuild,
	command.Workdir:    workdir,
	command.Run:        run,
	command.Cmd:        cmd,
	command.Entrypoint: entrypoint,
	command.Expose:     expose,
	command.Volume:     volume,
	command.User:       user,
	// TODO: use the public constants for these when we update dockerfile/
	commandStopSignal: stopSignal,
	commandArg:        arg,
}

// builtinAllowedBuildArgs is list of built-in allowed build args
var builtinAllowedBuildArgs = map[string]bool{
	"HTTP_PROXY":  true,
	"http_proxy":  true,
	"HTTPS_PROXY": true,
	"https_proxy": true,
	"FTP_PROXY":   true,
	"ftp_proxy":   true,
	"NO_PROXY":    true,
	"no_proxy":    true,
}

// ParseDockerIgnore returns a list of the excludes in the .dockerignore file.
// extracted from fsouza/go-dockerclient.
func ParseDockerignore(root string) ([]string, error) {
	var excludes []string
	ignore, err := ioutil.ReadFile(filepath.Join(root, ".dockerignore"))
	if err != nil && !os.IsNotExist(err) {
		return excludes, fmt.Errorf("error reading .dockerignore: '%s'", err)
	}
	return strings.Split(string(ignore), "\n"), nil
}

// ExportEnv creates an export statement for a shell that contains all of the
// provided environment.
func ExportEnv(env []string) string {
	if len(env) == 0 {
		return ""
	}
	out := "export"
	for _, e := range env {
		out += " " + BashQuote(e)
	}
	return out + "; "
}

// BashQuote escapes the provided string and surrounds it with double quotes.
// TODO: verify that these are all we have to escape.
func BashQuote(env string) string {
	out := []rune{'"'}
	for _, r := range env {
		switch r {
		case '$', '\\', '"':
			out = append(out, '\\', r)
		default:
			out = append(out, r)
		}
	}
	out = append(out, '"')
	return string(out)
}
