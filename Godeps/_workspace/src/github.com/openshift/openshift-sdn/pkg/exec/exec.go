// Package exec provides an os/exec wrapper that can be overridden for testing
// purposes
package exec

import (
	"fmt"
	osexec "os/exec"
	"strings"

	"github.com/golang/glog"
)

var testMode bool

// LookPath looks for a program in $PATH and returns either the full path or an error
func LookPath(program string) (string, error) {
	if testMode {
		return testModeLookPath(program)
	}

	return osexec.LookPath(program)
}

// Exec executes a command with the given arguments and returns either the
// combined stdout+stdin, or an error.
func Exec(cmd string, args ...string) (string, error) {
	if testMode {
		return testModeExec(cmd, args...)
	}

	glog.V(5).Infof("[cmd] %s %s", cmd, strings.Join(args, " "))
	out, err := osexec.Command(cmd, args...).CombinedOutput()
	if err != nil {
		err = fmt.Errorf("%s failed: '%s %s': %v", cmd, cmd, strings.Join(args, " "), err)
	} else if glog.V(5) {
		lines := strings.Split(string(out), "\n")
		for i, line := range lines {
			if i < len(lines)-1 || len(line) > 0 {
				glog.V(5).Infof("[cmd]   => %s", line)
			}
		}
	}
	return string(out), err
}

// For testing

var testPrograms map[string]string

type TestResult struct {
	command string
	output  string
	err     error
}

var testResults []TestResult

// SetTestMode turns on "test mode". In test mode, the output of LookPath() and
// Exec() is determined by prior calls to AddTestProgram() and AddTestResult().
func SetTestMode() {
	testMode = true
	testPrograms = make(map[string]string)
}

// AddTestProgram takes the full path to a program and allows that program to be
// be found via LookPath().
func AddTestProgram(path string) {
	lastSlash := strings.LastIndex(path, "/")
	basename := path[lastSlash+1:]
	testPrograms[basename] = path
}

// AddTestResult tells exec to expect a call to Exec() with the given command
// line, and to return the given output or error in response. You must call
// AddTestResult() once for every time that Exec() will be called, in order.
func AddTestResult(command string, output string, err error) {
	testResults = append(testResults, TestResult{command, output, err})
}

func testModeLookPath(program string) (string, error) {
	path, ok := testPrograms[program]
	if !ok {
		return "", fmt.Errorf("Not found: %s", program)
	}
	return path, nil
}

func testModeExec(cmd string, args ...string) (string, error) {
	var command string
	if len(args) > 0 {
		command = cmd + " " + strings.Join(args, " ")
	} else {
		command = cmd
	}

	if len(testResults) == 0 {
		panic(fmt.Sprintf("Ran out of testResults executing: %s", command))
	}

	result := testResults[0]
	testResults = testResults[1:]
	if command != result.command {
		panic(fmt.Sprintf("Wrong exec command: expected %v, got %v", result.command, command))
	}
	return result.output, result.err
}
