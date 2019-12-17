// +build functional

package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func runGlobalCommand(t *testing.T, args []string) (string, string, error) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed os.Getwd() with: %v", err)
	}
	cmd := exec.Command(
		filepath.Join(wd, "containerd-shim-runhcs-v1.exe"),
		args...,
	)

	outb := bytes.Buffer{}
	errb := bytes.Buffer{}
	cmd.Stdout = &outb
	cmd.Stderr = &errb
	err = cmd.Run()
	return outb.String(), errb.String(), err
}

func verifyGlobalCommandSuccess(t *testing.T, expectedStdout, stdout, expectedStderr, stderr string, runerr error) {
	if runerr != nil {
		t.Fatalf("expected no error got stdout: '%s', stderr: '%s', err: '%v'", stdout, stderr, runerr)
	}

	verifyGlobalCommandOut(t, expectedStdout, stdout, expectedStderr, stderr)
}

func verifyGlobalCommandFailure(t *testing.T, expectedStdout, stdout, expectedStderr, stderr string, runerr error) {
	if runerr == nil || runerr.Error() != "exit status 1" {
		t.Fatalf("expected error: 'exit status 1', got: '%v'", runerr)
	}

	verifyGlobalCommandOut(t, expectedStdout, stdout, expectedStderr, stderr)
}

func verifyGlobalCommandOut(t *testing.T, expectedStdout, stdout, expectedStderr, stderr string) {
	// stdout verify
	if expectedStdout == "" && expectedStdout != stdout {
		t.Fatalf("expected stdout empty got: %s", stdout)
	} else if !strings.HasPrefix(stdout, expectedStdout) {
		t.Fatalf("expected stdout to begin with: %s, got: %s", expectedStdout, stdout)
	}

	// stderr verify
	if expectedStderr == "" && expectedStderr != stderr {
		t.Fatalf("expected stderr empty got: %s", stderr)
	} else if !strings.HasPrefix(stderr, expectedStderr) {
		t.Fatalf("expected stderr to begin with: %s, got: %s", expectedStderr, stderr)
	}
}

func Test_Global_Command_No_Namespace(t *testing.T) {
	stdout, stderr, err := runGlobalCommand(
		t,
		[]string{})
	verifyGlobalCommandFailure(
		t,
		"namespace is required\n", stdout,
		"namespace is required\n", stderr,
		err)
}

func Test_Global_Command_No_Address(t *testing.T) {
	stdout, stderr, err := runGlobalCommand(
		t,
		[]string{
			"--namespace", t.Name(),
		})
	verifyGlobalCommandFailure(
		t,
		"address is required\n", stdout,
		"address is required\n", stderr,
		err)
}

func Test_Global_Command_No_PublishBinary(t *testing.T) {
	stdout, stderr, err := runGlobalCommand(
		t,
		[]string{
			"--namespace", t.Name(),
			"--address", t.Name(),
		})
	verifyGlobalCommandFailure(
		t,
		"publish-binary is required\n", stdout,
		"publish-binary is required\n", stderr,
		err)
}

func Test_Global_Command_No_ID(t *testing.T) {
	stdout, stderr, err := runGlobalCommand(
		t,
		[]string{
			"--namespace", t.Name(),
			"--address", t.Name(),
			"--publish-binary", t.Name(),
		})
	verifyGlobalCommandFailure(
		t,
		"id is required\n", stdout,
		"id is required\n", stderr,
		err)
}

func Test_Global_Command_No_Command(t *testing.T) {
	stdout, stderr, err := runGlobalCommand(
		t,
		[]string{
			"--namespace", t.Name(),
			"--address", t.Name(),
			"--publish-binary", t.Name(),
			"--id", t.Name(),
		})
	verifyGlobalCommandSuccess(
		t,
		"NAME:\n", stdout,
		"", stderr,
		err)
}
