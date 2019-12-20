// +build functional

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Microsoft/go-winio"
	"github.com/containerd/containerd/runtime/v2/task"
	"github.com/containerd/ttrpc"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/generate"
)

func createStartCommand(t *testing.T) (*exec.Cmd, *bytes.Buffer, *bytes.Buffer) {
	return createStartCommandWithID(t, t.Name())
}

func createStartCommandWithID(t *testing.T, id string) (*exec.Cmd, *bytes.Buffer, *bytes.Buffer) {
	bundleDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("failed to create bundle with: %v", err)
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed os.Getwd() with: %v", err)
	}
	cmd := exec.Command(
		filepath.Join(wd, "containerd-shim-runhcs-v1.exe"),
		"--namespace", t.Name(),
		"--address", "need-a-real-one",
		"--publish-binary", "need-a-real-one",
		"--id", id,
		"start",
	)
	cmd.Dir = bundleDir
	outb := bytes.Buffer{}
	errb := bytes.Buffer{}
	cmd.Stdout = &outb
	cmd.Stderr = &errb
	return cmd, &outb, &errb
}

func cleanupTestBundle(t *testing.T, dir string) {
	err := os.RemoveAll(dir)
	if err != nil {
		t.Errorf("failed removing test bundle with: %v", err)
	}
}

func writeBundleConfig(t *testing.T, dir string, cfg *specs.Spec) {
	cf, err := os.Create(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatalf("failed to create config.json with error: %v", err)
	}
	err = json.NewEncoder(cf).Encode(cfg)
	if err != nil {
		cf.Close()
		t.Fatalf("failed to encode config.json with error: %v", err)
	}
	cf.Close()
}

func verifyStartCommandSuccess(t *testing.T, expectedNamespace, expectedID string, cmd *exec.Cmd, stdout, stderr *bytes.Buffer) {
	err := cmd.Run()
	if err != nil {
		t.Fatalf("expected `start` command to succeed failed with: %v, stdout: %v, stderr: %v", err, stdout.String(), stderr.String())
	}
	sout := stdout.String()
	serr := stderr.String()

	expectedStdout := fmt.Sprintf("\\\\.\\pipe\\ProtectedPrefix\\Administrators\\containerd-shim-%s-%s", expectedNamespace, expectedID)
	if !strings.HasPrefix(sout, expectedStdout) {
		t.Fatalf("expected stdout to start with: %s, got: %s, %s", expectedStdout, sout, serr)
	}
	if serr != "" {
		t.Fatalf("expected stderr to be empty got: %s", serr)
	}
	// Connect and shutdown the serve shim.
	c, err := winio.DialPipe(sout, nil)
	if err != nil {
		t.Fatalf("failed to connect to hosting shim at: %s, with: %v", sout, err)
	}
	cl := ttrpc.NewClient(c)
	cl.OnClose(func() { c.Close() })
	tc := task.NewTaskClient(cl)
	ctx := context.Background()
	req := &task.ShutdownRequest{ID: expectedID, Now: true}
	_, err = tc.Shutdown(ctx, req)

	cl.Close()
	c.Close()
	if err != nil && !strings.HasPrefix(err.Error(), "ttrpc: client shutting down: ttrpc: closed") {
		t.Fatalf("failed to shutdown shim with: %v", err)
	}
}

func Test_Start_No_Bundle_Config(t *testing.T) {
	cmd, stdout, stderr := createStartCommand(t)
	defer cleanupTestBundle(t, cmd.Dir)

	expectedStdout := ""
	expectedStderr := fmt.Sprintf(
		"open %s: The system cannot find the file specified.",
		filepath.Join(cmd.Dir, "config.json"))

	err := cmd.Run()
	verifyGlobalCommandFailure(
		t,
		expectedStdout, stdout.String(),
		expectedStderr, stderr.String(),
		err)
}

func Test_Start_Invalid_Bundle_Config(t *testing.T) {
	cmd, stdout, stderr := createStartCommand(t)
	defer cleanupTestBundle(t, cmd.Dir)

	// Write an empty file with isnt a valid json struct.
	cf, err := os.Create(filepath.Join(cmd.Dir, "config.json"))
	if err != nil {
		t.Fatalf("failed to create config.json with error: %v", err)
	}
	cf.Close()

	expectedStdout := ""
	expectedStderr := "failed to deserialize valid OCI spec"

	err = cmd.Run()
	verifyGlobalCommandFailure(
		t,
		expectedStdout, stdout.String(),
		expectedStderr, stderr.String(),
		err)
}

func Test_Start_NoPod_Config(t *testing.T) {
	cmd, stdout, stderr := createStartCommand(t)
	defer cleanupTestBundle(t, cmd.Dir)

	g, err := generate.New("windows")
	if err != nil {
		t.Fatalf("failed to generate Windows config with error: %v", err)
	}
	writeBundleConfig(t, cmd.Dir, g.Config)

	verifyStartCommandSuccess(t, t.Name(), t.Name(), cmd, stdout, stderr)
}

func Test_Start_Pod_Config(t *testing.T) {
	cmd, stdout, stderr := createStartCommand(t)
	defer cleanupTestBundle(t, cmd.Dir)

	g, err := generate.New("windows")
	if err != nil {
		t.Fatalf("failed to generate Windows config with error: %v", err)
	}
	// Setup the POD annotations
	g.AddAnnotation("io.kubernetes.cri.container-type", "sandbox")
	g.AddAnnotation("io.kubernetes.cri.sandbox-id", t.Name())

	writeBundleConfig(t, cmd.Dir, g.Config)

	verifyStartCommandSuccess(t, t.Name(), t.Name(), cmd, stdout, stderr)
}

func Test_Start_Container_InPod_Config(t *testing.T) {
	// Create the POD
	podID := t.Name() + "-POD"
	pcmd, _, _ := createStartCommandWithID(t, podID)
	defer cleanupTestBundle(t, pcmd.Dir)

	pg, perr := generate.New("windows")
	if perr != nil {
		t.Fatalf("failed to generate Windows config with error: %v", perr)
	}

	pg.AddAnnotation("io.kubernetes.cri.container-type", "sandbox")
	pg.AddAnnotation("io.kubernetes.cri.sandbox-id", podID)

	writeBundleConfig(t, pcmd.Dir, pg.Config)

	perr = pcmd.Run()
	if perr != nil {
		t.Fatalf("failed to start pod container shim with err: %v", perr)
	}

	// Create the Workload container
	wcmd, wstdout, wstderr := createStartCommand(t)
	defer cleanupTestBundle(t, wcmd.Dir)

	wg, werr := generate.New("windows")
	if werr != nil {
		t.Fatalf("failed to generate Windows config with error: %v", werr)
	}

	// Setup the POD Workload container annotations
	wg.AddAnnotation("io.kubernetes.cri.container-type", "container")
	wg.AddAnnotation("io.kubernetes.cri.sandbox-id", podID)

	writeBundleConfig(t, wcmd.Dir, wg.Config)

	verifyStartCommandSuccess(t, t.Name(), podID, wcmd, wstdout, wstderr)
}

func Test_Start_Container_InPod_Config_PodShim_Gone(t *testing.T) {
	cmd, stdout, stderr := createStartCommand(t)
	defer cleanupTestBundle(t, cmd.Dir)

	g, err := generate.New("windows")
	if err != nil {
		t.Fatalf("failed to generate Windows config with error: %v", err)
	}

	podID := "POD-TEST"
	// Setup the POD Workload container annotations
	g.AddAnnotation("io.kubernetes.cri.container-type", "container")
	g.AddAnnotation("io.kubernetes.cri.sandbox-id", podID)

	writeBundleConfig(t, cmd.Dir, g.Config)

	expectedStdout := ""
	expectedStderr := "failed to connect to hosting shim"

	err = cmd.Run()
	verifyGlobalCommandFailure(
		t,
		expectedStdout, stdout.String(),
		expectedStderr, stderr.String(),
		err)
}
