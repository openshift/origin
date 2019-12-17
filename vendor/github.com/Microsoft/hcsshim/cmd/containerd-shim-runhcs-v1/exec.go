package main

import (
	"context"

	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/runtime/v2/task"
	"github.com/pkg/errors"
)

type shimExecState string

const (
	shimExecStateCreated shimExecState = "created"
	shimExecStateRunning shimExecState = "running"
	shimExecStateExited  shimExecState = "exited"
)

// shimExec is an interface that represents a single process created by a user
// within a container (task).
//
// For WCOW process isolated containers the process will be on the same machine
// as this shim.
//
// For WCOW hypervisor isolated or LCOW containers the process will be viewed
// and proxied to the remote UtilityVM hosting the process.
type shimExec interface {
	// ID returns the original id of this exec.
	ID() string
	// Pid returns the pid of the exec process.
	//
	// A call to `Pid` is valid in any `State()`.
	Pid() int
	// State returns the current state of this exec process.
	//
	// A call to `State` is valid in any `State()`.
	State() shimExecState
	// Status returns the current full status of this exec process.
	//
	// A call to `Status` is valid in any `State()`. Note that for `State() ==
	// shimExecStateRunning` this exec process MUST return `ExitStatus=255` and
	// `time.IsZero(ExitedAt)==true` by convention.
	Status() *task.StateResponse
	// Start starts the exec process.
	//
	// If the exec process has already been started this exec MUST return
	// `errdefs.ErrFailedPrecondition`.
	Start(ctx context.Context) error
	// Kill sends `signal` to this exec process.
	//
	// If `State() != shimExecStateRunning` this exec MUST return
	// `errdefs.ErrFailedPrecondition`.
	//
	// If `State() == shimExecStateExited` this exec MUST return `errdefs.ErrNotFound`.
	Kill(ctx context.Context, signal uint32) error
	// ResizePty resizes the tty of this exec process.
	//
	// If this exec is not a tty this exec MUST return
	// `errdefs.ErrFailedPrecondition`.
	//
	// If `State() != shimExecStateRunning` this exec MUST return
	// `errdefs.ErrFailedPrecondition`.
	ResizePty(ctx context.Context, width, height uint32) error
	// CloseIO closes `stdin` if open.
	//
	// A call to `CloseIO` is valid in any `State()` and MUST not return an
	// error for duplicate calls.
	CloseIO(ctx context.Context, stdin bool) error
	// Wait waits for this exec process to exit and returns the state of the
	// exit information.
	//
	// A call to `Wait` is valid in any `State()`. Note that if this exec
	// process is already in the `State() == shimExecStateExited` state, `Wait`
	// MUST return immediately with the original exit state.
	Wait(ctx context.Context) *task.StateResponse
	// ForceExit forcibly terminates the exec, sets the exit status to `status`,
	// and unblocks all waiters.
	//
	// This call is idempotent and safe to call even on an already exited exec
	// in which case it does nothing.
	//
	// `ForceExit` is safe to call in any `State()`.
	ForceExit(status int)
}

func newExecInvalidStateError(tid, eid string, state shimExecState, op string) error {
	return errors.Wrapf(
		errdefs.ErrFailedPrecondition,
		"exec: '%s' in task: '%s' is in invalid state: '%s' for %s",
		eid,
		tid,
		state,
		op)
}
