package editor

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/docker/docker/pkg/term"
	"github.com/golang/glog"
)

const (
	// sorry, blame Git
	defaultEditor = "vi"
	defaultShell  = "/bin/bash"
)

type Editor struct {
	Args  []string
	Shell bool
}

// NewDefaultEditor creates an Editor that uses the OS environment to
// locate the editor program, looking at OSC_EDITOR, GIT_EDITOR, and
// EDITOR in order to find the proper command line. If the provided
// editor has no spaces, or no quotes, it is treated as a bare command
// to be loaded. Otherwise, the string will be passed to the user's
// shell for execution.
func NewDefaultEditor() Editor {
	args, shell := defaultEnvEditor()
	return Editor{
		Args:  args,
		Shell: shell,
	}
}

func defaultEnvShell() []string {
	shell := os.Getenv("SHELL")
	if len(shell) == 0 {
		shell = defaultShell
	}
	return []string{shell, "-c"}
}

func defaultEnvEditor() ([]string, bool) {
	editor := os.Getenv("OSC_EDITOR")
	if len(editor) == 0 {
		editor = os.Getenv("GIT_EDITOR")
	}
	if len(editor) == 0 {
		editor = os.Getenv("EDITOR")
	}
	if len(editor) == 0 {
		editor = defaultEditor
	}
	if !strings.Contains(editor, " ") {
		return []string{editor}, false
	}
	if !strings.ContainsAny(editor, "\"'\\") {
		return strings.Split(editor, " "), false
	}
	// rather than parse the shell arguments ourselves, punt to the shell
	shell := defaultEnvShell()
	return append(shell, editor), true
}

func (e Editor) args(path string) []string {
	args := make([]string, len(e.Args))
	copy(args, e.Args)
	if e.Shell {
		last := args[len(args)-1]
		args[len(args)-1] = fmt.Sprintf("%s %q", last, path)
	} else {
		args = append(args, path)
	}
	return args
}

// Launch opens the described or returns an error. The TTY will be protected, and
// SIGQUIT, SIGTERM, and SIGINT will all be trapped.
func (e Editor) Launch(path string) error {
	if len(e.Args) == 0 {
		return fmt.Errorf("no editor defined, can't open %s", path)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	args := e.args(abs)
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	glog.V(5).Infof("Opening file with editor %v", args)
	if err := withSafeTTYAndInterrupts(os.Stdin, cmd.Run); err != nil {
		if err, ok := err.(*exec.Error); ok {
			if err.Err == exec.ErrNotFound {
				return fmt.Errorf("unable to launch the editor %q", strings.Join(e.Args, " "))
			}
		}
		return fmt.Errorf("there was a problem with the editor %q", strings.Join(e.Args, " "))
	}
	return nil
}

// LaunchTempFile reads the provided stream into a temporary file in the given directory
// and file prefix, and then invokes Launch with the path of that file. It will return
// the contents of the file after launch, any errors that occur, and the path of the
// temporary file so the caller can clean it up as needed.
func (e Editor) LaunchTempFile(dir, prefix string, r io.Reader) ([]byte, string, error) {
	f, err := ioutil.TempFile(dir, prefix)
	if err != nil {
		return nil, "", err
	}
	defer f.Close()
	path := f.Name()
	if _, err := io.Copy(f, r); err != nil {
		os.Remove(path)
		return nil, path, err
	}
	if err := e.Launch(path); err != nil {
		return nil, path, err
	}
	bytes, err := ioutil.ReadFile(path)
	return bytes, path, err
}

// withSafeTTYAndInterrupts invokes the provided function after the terminal
// state has been stored, and then on any error or termination attempts to
// restore the terminal state to its prior behavior. It also eats signals
// for the duration of the function.
func withSafeTTYAndInterrupts(r io.Reader, fn func() error) error {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, childSignals...)
	defer signal.Stop(ch)

	if file, ok := r.(*os.File); ok {
		inFd := file.Fd()
		if term.IsTerminal(inFd) {
			state, err := term.SaveState(inFd)
			if err != nil {
				return err
			}
			go func() {
				if _, ok := <-ch; !ok {
					return
				}
				term.RestoreTerminal(inFd, state)
			}()
			defer term.RestoreTerminal(inFd, state)
			return fn()
		}
	}
	return fn()
}
