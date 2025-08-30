package ginkgo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/openshift-eng/openshift-tests-extension/pkg/dbtime"
	"github.com/openshift-eng/openshift-tests-extension/pkg/extension/extensiontests"
)

func SpawnProcessToRunTest(ctx context.Context, testName string, timeout time.Duration) *extensiontests.ExtensionTestResult {
	// longerCtx is used to backstop the process, but leave termination up to us if possible to allow a double interrupt
	longerCtx, longerCancel := context.WithTimeout(ctx, timeout+15*time.Minute)
	defer longerCancel()
	timeoutCtx, shorterCancel := context.WithTimeout(longerCtx, timeout)
	defer shorterCancel()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	command := exec.CommandContext(longerCtx, os.Args[0], "run-test", "--output=json", testName)
	command.Stdout = stdout
	command.Stderr = stderr

	start := time.Now()
	err := command.Start()
	if err != nil {
		fmt.Fprintf(stderr, "Command Start Error: %v\n", err)
		return newTestResult(testName, extensiontests.ResultFailed, start, time.Now(), stdout, stderr)
	}

	go func() {
		select {
		// interrupt tests after timeout, and abort if they don't complete quick enough
		case <-time.After(timeout):
			if command.Process != nil {
				// we're not going to do anything with the err
				_ = command.Process.Signal(syscall.SIGINT)
			}
			// if the process appears to be hung a significant amount of time after the timeout
			// send an ABRT so we get a stack dump
			select {
			case <-time.After(time.Minute):
				if command.Process != nil {
					// we're not going to do anything with the err
					_ = command.Process.Signal(syscall.SIGABRT)
				}
			case <-timeoutCtx.Done():
				if command.Process != nil {
					_ = command.Process.Signal(syscall.SIGABRT)
				}
			}
		case <-timeoutCtx.Done():
			if command.Process != nil {
				_ = command.Process.Signal(syscall.SIGINT)
			}
		}
	}()

	result := extensiontests.ResultFailed
	cmdErr := command.Wait()

	subcommandResult, parseErr := newTestResultFromOutput(stdout)
	if parseErr == nil {
		// even if we have a cmdErr, if we were able to parse the result, trust the output
		return subcommandResult
	}

	fmt.Fprintf(stderr, "Command Error: %v\n", cmdErr)
	fmt.Fprintf(stderr, "Deserializaion Error: %v\n", parseErr)
	return newTestResult(testName, result, start, time.Now(), stdout, stderr)
}

func newTestResultFromOutput(stdout *bytes.Buffer) (*extensiontests.ExtensionTestResult, error) {
	if len(stdout.Bytes()) == 0 {
		return nil, errors.New("no output from command")
	}

	// when the command runs correctly, we get json or json slice output
	retArray := []extensiontests.ExtensionTestResult{}
	if arrayItemErr := json.Unmarshal(stdout.Bytes(), &retArray); arrayItemErr == nil {
		if len(retArray) != 1 {
			return nil, errors.New("expected 1 result, got %v results")
		}
		return &retArray[0], nil
	}

	// when the command runs correctly, we get json output
	ret := &extensiontests.ExtensionTestResult{}
	if singleItemErr := json.Unmarshal(stdout.Bytes(), ret); singleItemErr != nil {
		return nil, singleItemErr
	}

	return ret, nil
}

func newTestResult(name string, result extensiontests.Result, start, end time.Time, stdout, stderr *bytes.Buffer) *extensiontests.ExtensionTestResult {
	duration := end.Sub(start)
	dbStart := dbtime.DBTime(start)
	dbEnd := dbtime.DBTime(start)
	ret := &extensiontests.ExtensionTestResult{
		Name:      name,
		Lifecycle: "", // lifecycle is completed one level above this.
		Duration:  int64(duration),
		StartTime: &dbStart,
		EndTime:   &dbEnd,
		Result:    result,
		Details:   nil,
	}

	if stdout != nil && stderr != nil {
		stdoutStr := stdout.String()
		stderrStr := stderr.String()

		ret.Output = fmt.Sprintf("STDOUT:\n%s\n\nSTDERR:\n%s\n", stdoutStr, stderrStr)

		// try to choose the best summary
		switch {
		case len(stderrStr) > 0 && len(stderrStr) < 5000:
			ret.Error = stderrStr
		case len(stderrStr) > 0 && len(stderrStr) >= 5000:
			ret.Error = stderrStr[len(stderrStr)-5000:]

		case len(stdoutStr) > 0 && len(stdoutStr) < 5000:
			ret.Error = stdoutStr
		case len(stdoutStr) > 0 && len(stdoutStr) >= 5000:
			ret.Error = stdoutStr[len(stdoutStr)-5000:]
		}
	}

	return ret
}
