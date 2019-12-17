package main

import (
	"context"
	"time"

	"github.com/Microsoft/hcsshim/cmd/containerd-shim-runhcs-v1/options"
	"github.com/containerd/containerd/runtime/v2/task"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

// shimTaskPidPair groups a process pid to its execID if it was user generated.
type shimTaskPidPair struct {
	// Pid is the pid of the container process.
	Pid int
	// ExecID is the id of the exec if this container process was user
	// generated.
	ExecID string
}

type shimTask interface {
	// ID returns the original id used at `Create`.
	ID() string
	// CreateExec creates an additional exec within this task.
	//
	// If `req.ID==""` or `req.ID` is already a known exec this task MUST return
	// `errdefs.ErrAlreadyExists`
	//
	// If the init exec is no longer running this task MUST return
	// `errdefs.ErrFailedPrecondition`.
	CreateExec(ctx context.Context, req *task.ExecProcessRequest, s *specs.Process) error
	// GetExec returns an exec in this task that matches `eid`. If `eid == ""`
	// returns the init exec from the initial call to `Create`.
	//
	// If `eid` is not found this task MUST return `errdefs.ErrNotFound`.
	GetExec(eid string) (shimExec, error)
	// KillExec sends `signal` to the exec that matches `eid`. If `all==true`
	// `eid` MUST be empty and this task will send `signal` to all exec's in the
	// task and lastly send `signal` to the init exec.
	//
	// If `all == true && eid != ""` this task MUST return
	// `errdefs.ErrFailedPrecondition`.
	//
	// A call to `KillExec` is only valid when the exec is in the
	// `shimExecStateRunning, shimExecStateExited` states. If the exec is not in
	// this state this task MUST return `errdefs.ErrFailedPrecondition`. If
	// `eid=="" && all == false` all additional exec's must be in the
	// `shimExecStateExited` state.
	KillExec(ctx context.Context, eid string, signal uint32, all bool) error
	// DeleteExec deletes a `shimExec` in this `shimTask` that matches `eid`. If
	// `eid == ""` deletes the init `shimExec` AND this `shimTask`.
	//
	// If `eid` is not found `shimExec` MUST return `errdefs.ErrNotFound`.
	//
	// A call to `DeleteExec` is only valid in `shimExecStateCreated` and
	// `shimExecStateExited` states and MUST return
	// `errdefs.ErrFailedPrecondition` if not in these states. If `eid==""` all
	// additional exec's tracked by this task must also be in the
	// `shimExecStateExited` state.
	DeleteExec(ctx context.Context, eid string) (int, uint32, time.Time, error)
	// Pids returns all process pid's in this `shimTask` including ones not
	// created by the caller via a `CreateExec`.
	Pids(ctx context.Context) ([]options.ProcessDetails, error)
	// Waits for the the init task to complete.
	//
	// Note: If the `request.ExecID == ""` the caller should instead call `Wait`
	// rather than `exec.Wait` on the init exec. This is because  the lifetime
	// of the task is larger than just the init process and on shutdown we need
	// to wait for the container and potentially UVM before unblocking any event
	// based listeners or `Wait` based listeners.
	Wait(ctx context.Context) *task.StateResponse
}
