package main

import (
	"context"
	"testing"
	"time"

	containerd_v1_types "github.com/containerd/containerd/api/types/task"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/runtime/v2/task"
	"github.com/pkg/errors"
)

func verifyWcowPodSandboxExecStatus(t *testing.T, wasStarted bool, es containerd_v1_types.Status, status *task.StateResponse) {
	if status.ID != t.Name() {
		t.Fatalf("expected id: '%s' got: '%s'", t.Name(), status.ID)
	}
	if status.ExecID != t.Name() {
		t.Fatalf("expected execid: '%s' got: '%s'", t.Name(), status.ExecID)
	}
	if status.Bundle != t.Name() {
		t.Fatalf("expected bundle: '%s' got: '%s'", t.Name(), status.Bundle)
	}
	var expectedPid uint32
	if wasStarted && es != containerd_v1_types.StatusCreated {
		expectedPid = 1
	}
	if status.Pid != expectedPid {
		t.Fatalf("expected pid: '%d' got: '%d'", expectedPid, status.Pid)
	}
	if status.Status != es {
		t.Fatalf("expected status: '%s' got: '%s'", es, status.Status)
	}
	if status.Stdin != "" {
		t.Fatalf("expected stdin: '' got: '%s'", status.Stdin)
	}
	if status.Stdout != "" {
		t.Fatalf("expected stdout: '' got: '%s'", status.Stdout)
	}
	if status.Stderr != "" {
		t.Fatalf("expected stderr: '' got: '%s'", status.Stderr)
	}
	if status.Terminal {
		t.Fatalf("expected terminal: 'false' got: '%v'", status.Terminal)
	}
	var expectedExitStatus uint32
	switch es {
	case containerd_v1_types.StatusCreated, containerd_v1_types.StatusRunning:
		expectedExitStatus = 255
	case containerd_v1_types.StatusStopped:
		if !wasStarted {
			expectedExitStatus = 1
		} else {
			expectedExitStatus = 0
		}
	}
	if status.ExitStatus != expectedExitStatus {
		t.Fatalf("expected exitstatus: '%d' got: '%d'", expectedExitStatus, status.ExitStatus)
	}
	if es != containerd_v1_types.StatusStopped {
		if !status.ExitedAt.IsZero() {
			t.Fatalf("expected exitedat: '%v' got: '%v'", time.Time{}, status.ExitedAt)
		}
	} else {
		if status.ExitedAt.IsZero() {
			t.Fatalf("expected exitedat: > '%v' got: '%v'", time.Time{}, status.ExitedAt)
		}
	}
}

func Test_newWcowPodSandboxExec(t *testing.T) {
	wpse := newWcowPodSandboxExec(context.TODO(), fakePublisher, t.Name(), t.Name())

	verifyWcowPodSandboxExecStatus(t, false, containerd_v1_types.StatusCreated, wpse.Status())
}

func Test_newWcowPodSandboxExec_ID(t *testing.T) {
	wpse := newWcowPodSandboxExec(context.TODO(), fakePublisher, t.Name(), t.Name())

	if wpse.ID() != t.Name() {
		t.Fatalf("expected ID: '%s' got: '%s", t.Name(), wpse.ID())
	}
}

func Test_newWcowPodSandboxExec_Pid(t *testing.T) {
	wpse := newWcowPodSandboxExec(context.TODO(), fakePublisher, t.Name(), t.Name())

	if wpse.Pid() != 0 {
		t.Fatalf("expected created pid: '0' got: '%d", wpse.Pid())
	}

	// Start it
	err := wpse.Start(context.TODO())
	if err != nil {
		t.Fatalf("should not have failed to start got: %v", err)
	}

	if wpse.Pid() != 1 {
		t.Fatalf("expected running pid: '1' got: '%d", wpse.Pid())
	}

	// Stop it
	err = wpse.Kill(context.TODO(), 0x0)
	if err != nil {
		t.Fatalf("should not have failed to stop got: %v", err)
	}

	if wpse.Pid() != 1 {
		t.Fatalf("expected stopped pid: '1' got: '%d", wpse.Pid())
	}
}

func Test_newWcowPodSandboxExec_State(t *testing.T) {
	wpse := newWcowPodSandboxExec(context.TODO(), fakePublisher, t.Name(), t.Name())

	if wpse.State() != shimExecStateCreated {
		t.Fatalf("expected state: '%s' got: '%s", shimExecStateCreated, wpse.State())
	}

	// Start it
	err := wpse.Start(context.TODO())
	if err != nil {
		t.Fatalf("should not have failed to start got: %v", err)
	}

	if wpse.State() != shimExecStateRunning {
		t.Fatalf("expected state: '%s' got: '%s", shimExecStateRunning, wpse.State())
	}

	// Stop it
	err = wpse.Kill(context.TODO(), 0x0)
	if err != nil {
		t.Fatalf("should not have failed to stop got: %v", err)
	}

	if wpse.State() != shimExecStateExited {
		t.Fatalf("expected state: '%s' got: '%s", shimExecStateExited, wpse.State())
	}
}

func Test_newWcowPodSandboxExec_Status(t *testing.T) {
	wpse := newWcowPodSandboxExec(context.TODO(), fakePublisher, t.Name(), t.Name())

	verifyWcowPodSandboxExecStatus(t, false, containerd_v1_types.StatusCreated, wpse.Status())

	// Start it
	err := wpse.Start(context.TODO())
	if err != nil {
		t.Fatalf("should not have failed to start got: %v", err)
	}

	verifyWcowPodSandboxExecStatus(t, true, containerd_v1_types.StatusRunning, wpse.Status())

	// Stop it
	err = wpse.Kill(context.TODO(), 0x0)
	if err != nil {
		t.Fatalf("should not have failed to stop got: %v", err)
	}

	verifyWcowPodSandboxExecStatus(t, true, containerd_v1_types.StatusStopped, wpse.Status())
}

func Test_newWcowPodSandboxExec_Start(t *testing.T) {
	wpse := newWcowPodSandboxExec(context.TODO(), fakePublisher, t.Name(), t.Name())

	// Start it
	err := wpse.Start(context.TODO())
	if err != nil {
		t.Fatalf("should not have failed to start got: %v", err)
	}
	if wpse.State() != shimExecStateRunning {
		t.Fatalf("should of transitioned to running state")
	}

	// Call start again
	err = wpse.Start(context.TODO())
	verifyExpectedError(t, nil, err, errdefs.ErrFailedPrecondition)
}

func Test_newWcowPodSandboxExec_Kill_Created(t *testing.T) {
	wpse := newWcowPodSandboxExec(context.TODO(), fakePublisher, t.Name(), t.Name())

	// Kill it in the created state
	err := wpse.Kill(context.TODO(), 0x0)
	if err != nil {
		t.Fatalf("should not have failed to kill got: %v", err)
	}
	if wpse.State() != shimExecStateExited {
		t.Fatalf("should of transitioned to exited state")
	}

	// Call Kill again
	err = wpse.Kill(context.TODO(), 0x0)
	if errors.Cause(err) != errdefs.ErrNotFound {
		t.Fatalf("Kill should fail with `ErrNotFound` in the exited state got: %v", err)
	}
}

func Test_newWcowPodSandboxExec_Kill_Started(t *testing.T) {
	wpse := newWcowPodSandboxExec(context.TODO(), fakePublisher, t.Name(), t.Name())

	// Start it
	err := wpse.Start(context.TODO())
	if err != nil {
		t.Fatalf("should not have failed to start got: %v", err)
	}

	// Kill it in the started state
	err = wpse.Kill(context.TODO(), 0x0)
	if err != nil {
		t.Fatalf("should not have failed to kill got: %v", err)
	}
	if wpse.State() != shimExecStateExited {
		t.Fatalf("should of transitioned to exited state")
	}

	// Call Kill again
	err = wpse.Kill(context.TODO(), 0x0)
	if errors.Cause(err) != errdefs.ErrNotFound {
		t.Fatalf("Kill should fail with `ErrNotFound` in the exited state got: %v", err)
	}
}

func Test_newWcowPodSandboxExec_ResizePty(t *testing.T) {
	wpse := newWcowPodSandboxExec(context.TODO(), fakePublisher, t.Name(), t.Name())

	// Resize in created state
	err := wpse.ResizePty(context.TODO(), 10, 10)
	verifyExpectedError(t, nil, err, errdefs.ErrFailedPrecondition)

	// Start it
	err = wpse.Start(context.TODO())
	if err != nil {
		t.Fatalf("should not have failed to start got: %v", err)
	}

	err = wpse.ResizePty(context.TODO(), 10, 10)
	verifyExpectedError(t, nil, err, errdefs.ErrFailedPrecondition)

	// Stop it
	err = wpse.Kill(context.TODO(), 0x0)
	if err != nil {
		t.Fatalf("should not have failed to stop got: %v", err)
	}

	err = wpse.ResizePty(context.TODO(), 10, 10)
	verifyExpectedError(t, nil, err, errdefs.ErrFailedPrecondition)
}

func Test_newWcowPodSandboxExec_CloseIO(t *testing.T) {
	wpse := newWcowPodSandboxExec(context.TODO(), fakePublisher, t.Name(), t.Name())

	// Resize in created state
	err := wpse.CloseIO(context.TODO(), true)
	if err != nil {
		t.Fatalf("should not have failed CloseIO in created state got: %v", err)
	}

	// Start it
	err = wpse.Start(context.TODO())
	if err != nil {
		t.Fatalf("should not have failed to start got: %v", err)
	}

	err = wpse.CloseIO(context.TODO(), true)
	if err != nil {
		t.Fatalf("should not have failed CloseIO in running state got: %v", err)
	}

	// Stop it
	err = wpse.Kill(context.TODO(), 0x0)
	if err != nil {
		t.Fatalf("should not have failed to stop got: %v", err)
	}

	err = wpse.CloseIO(context.TODO(), true)
	if err != nil {
		t.Fatalf("should not have failed CloseIO in exited state got: %v", err)
	}
}

func Test_newWcowPodSandboxExec_Wait_Created(t *testing.T) {
	wpse := newWcowPodSandboxExec(context.TODO(), fakePublisher, t.Name(), t.Name())

	waitExit := make(chan *task.StateResponse, 1)
	defer close(waitExit)

	// Issue the wait in the created state
	go func() {
		waitExit <- wpse.Wait(context.TODO())
	}()

	now := time.Now()
	err := wpse.Kill(context.TODO(), 0x0)
	if err != nil {
		t.Fatalf("should not have failed to kill got: %v", err)
	}

	status := <-waitExit
	verifyWcowPodSandboxExecStatus(t, false, containerd_v1_types.StatusStopped, status)
	if status.ExitedAt.Before(now) {
		t.Fatal("exit should not have unblocked previous to kill")
	}

	// Verify the wait in the exited state doesnt block.
	verifyWcowPodSandboxExecStatus(t, false, containerd_v1_types.StatusStopped, wpse.Wait(context.TODO()))
}

func Test_newWcowPodSandboxExec_Wait_Started(t *testing.T) {
	wpse := newWcowPodSandboxExec(context.TODO(), fakePublisher, t.Name(), t.Name())

	waitExit := make(chan *task.StateResponse, 1)
	defer close(waitExit)

	// Issue the wait in the created state
	go func() {
		waitExit <- wpse.Wait(context.TODO())
	}()

	err := wpse.Start(context.TODO())
	if err != nil {
		t.Fatalf("should not have failed to start got: %v", err)
	}

	now := time.Now()
	err = wpse.Kill(context.TODO(), 0x0)
	if err != nil {
		t.Fatalf("should not have failed to kill got: %v", err)
	}

	status := <-waitExit
	verifyWcowPodSandboxExecStatus(t, true, containerd_v1_types.StatusStopped, status)
	if status.ExitedAt.Before(now) {
		t.Fatal("exit should not have unblocked previous to kill")
	}

	// Verify the wait in the exited state doesnt block.
	verifyWcowPodSandboxExecStatus(t, true, containerd_v1_types.StatusStopped, wpse.Wait(context.TODO()))
}
