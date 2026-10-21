package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestConnectionWatchCommandSelection(t *testing.T) {
	t.Parallel()
	for _, testCase := range []struct {
		name string
		args []string
		want bool
	}{
		{name: "command", args: []string{"disruption", "watch-established-connections"}, want: true},
		{name: "persistent flag before commands", args: []string{"--v", "2", "disruption", "watch-established-connections"}, want: true},
		{name: "persistent flag between commands", args: []string{"disruption", "-v=2", "watch-established-connections"}, want: true},
		{name: "persistent flag after commands", args: []string{"disruption", "watch-established-connections", "-v", "2"}, want: true},
		{name: "attached short flag", args: []string{"-v2", "disruption", "watch-established-connections"}, want: true},
		{name: "decimal leading zero", args: []string{"--v=08", "disruption", "watch-established-connections"}, want: true},
		{name: "negative verbosity", args: []string{"disruption", "watch-established-connections", "--v=-1"}, want: true},
		{name: "watch flag before commands", args: []string{"--namespace", "example", "disruption", "watch-established-connections"}, want: true},
		{name: "watch flags", args: []string{"disruption", "watch-established-connections", "--namespace=example", "--probe-timeout", "2s"}, want: true},
		{name: "help", args: []string{"disruption", "watch-established-connections", "--help"}, want: true},
		{name: "empty"},
		{name: "parent", args: []string{"disruption"}},
		{name: "sibling", args: []string{"disruption", "poll-service"}},
		{name: "other command arguments", args: []string{"run", "disruption", "watch-established-connections"}},
		{name: "flag values", args: []string{"--namespace", "disruption", "watch-established-connections"}},
		{name: "leaf as flag value", args: []string{"disruption", "--namespace", "watch-established-connections"}},
		{name: "other command flag value", args: []string{"version", "--output", "disruption", "watch-established-connections"}},
		{name: "after flag terminator", args: []string{"--", "disruption", "watch-established-connections"}},
		{name: "invalid verbosity", args: []string{"--v", "disruption", "disruption", "watch-established-connections"}},
		{name: "hex verbosity rejected", args: []string{"--v=0x2", "disruption", "watch-established-connections"}},
		{name: "out of range verbosity", args: []string{"--v=2147483648", "disruption", "watch-established-connections"}},
		{name: "unknown flag", args: []string{"--unknown=value", "disruption", "watch-established-connections"}},
		{name: "invalid duration", args: []string{"disruption", "watch-established-connections", "--probe-timeout=bad"}},
		{name: "missing flag value", args: []string{"disruption", "watch-established-connections", "--namespace"}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			if got := isConnectionWatchCommand(testCase.args); got != testCase.want {
				t.Errorf("isConnectionWatchCommand(%q) = %v, want %v", testCase.args, got, testCase.want)
			}
		})
	}
}

func TestConnectionWatchEntrypointStreams(t *testing.T) {
	t.Parallel()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, testCase := range []struct {
		name    string
		args    []string
		version bool
	}{
		{name: "normal execution", args: []string{"disruption", "watch-established-connections"}},
		{name: "leading verbosity", args: []string{"--v=2", "disruption", "watch-established-connections"}},
		{name: "interleaved verbosity", args: []string{"disruption", "-v", "2", "watch-established-connections"}},
		{name: "trailing verbosity", args: []string{"disruption", "watch-established-connections", "-v2"}},
		{name: "decimal leading zero", args: []string{"--v=08", "disruption", "watch-established-connections"}},
		{name: "unrelated command unchanged", args: []string{"version"}, version: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			command := exec.CommandContext(ctx, executable, append([]string{"-test.run=^TestOpenShiftTestsEntrypoint$", "--"}, testCase.args...)...)
			command.Env = append(os.Environ(), "ORIGIN_TEST_ENTRYPOINT=1", "KUBE_TEST_REPO_LIST=", "OPENSHIFT_PROFILE=", "KUBECONFIG="+filepath.Join(t.TempDir(), "missing-kubeconfig"))
			var stdout, stderr bytes.Buffer
			command.Stdout, command.Stderr = &stdout, &stderr
			err := command.Run()
			if testCase.version {
				if err != nil || !strings.Contains(stdout.String(), "openshift-tests version") || !strings.Contains(stdout.String(), "test specs") {
					t.Fatalf("unrelated command changed: error=%v stdout=%s stderr=%s", err, &stdout, &stderr)
				}
				return
			}
			var exitError *exec.ExitError
			if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
				t.Fatalf("expected required-option exit 1, got %v; stderr: %s", err, &stderr)
			}
			if stdout.Len() != 0 {
				t.Errorf("unexpected stdout (%d bytes): %s", stdout.Len(), &stdout)
			}
			if !strings.Contains(stderr.String(), "--my-node-name is required") {
				t.Errorf("missing required-option diagnostic: %s", &stderr)
			}
			if !strings.Contains(stderr.String(), "test specs") {
				t.Errorf("missing extension initialization diagnostics: %s", &stderr)
			}
		})
	}
}

func TestOpenShiftTestsEntrypoint(t *testing.T) {
	if os.Getenv("ORIGIN_TEST_ENTRYPOINT") != "1" {
		return
	}
	for index, argument := range os.Args {
		if argument == "--" {
			os.Args = append(os.Args[:1], os.Args[index+1:]...)
			main()
			os.Exit(0)
		}
	}
	t.Fatal("missing entrypoint argument separator")
}
