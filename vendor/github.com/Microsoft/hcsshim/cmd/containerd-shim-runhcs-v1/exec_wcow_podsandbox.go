package main

import (
	"context"
	"sync"
	"time"

	eventstypes "github.com/containerd/containerd/api/events"
	containerd_v1_types "github.com/containerd/containerd/api/types/task"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/runtime"
	"github.com/containerd/containerd/runtime/v2/task"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func newWcowPodSandboxExec(ctx context.Context, events publisher, tid, bundle string) *wcowPodSandboxExec {
	logrus.WithFields(logrus.Fields{
		"tid": tid,
		"eid": tid, // Init exec ID is always same as Task ID
	}).Debug("newWcowPodSandboxExec")

	wpse := &wcowPodSandboxExec{
		events:     events,
		tid:        tid,
		bundle:     bundle,
		state:      shimExecStateCreated,
		exitStatus: 255, // By design for non-exited process status.
		exited:     make(chan struct{}),
	}
	return wpse
}

var _ = (shimExec)(&wcowPodSandboxExec{})

// wcowPodSandboxExec is a special exec type that actually holds no real
// resources. The WCOW model has two services the HCS/HNS that actually hold
// open any shared namespace outside of the container/process itself. So we
// don't actually need to create them in the case of a POD for the sandbox
// container.
//
// Note: This is only true today because CRI defines an API that allows for no
// query/state/exec/io operations against the POD sandbox container itself. If
// this changes we need to rethink this.
//
// Note 2: This is only true today because Windows contains only a shared
// `NetworkNamespace` controlled via CNI in the CRI layer and held open by the
// HNS and not by any container runtime attribute. If we ever have a shared
// namespace that requires a container we will have to rethink this.
type wcowPodSandboxExec struct {
	events publisher
	// tid is the task id of the container hosting this process.
	//
	// This MUST be treated as read only in the lifetime of the exec.
	tid string
	// bundle is typically the on disk path to the folder containing the
	// `process.json` describing this process. In the `wcowPodSandboxExec` this
	// will always be the init exec and thus the process is described in the
	// `config.json`.
	//
	// This MUST be treated as read only in the lifetime of the exec.
	bundle string

	// sl is the state lock that MUST be held to safely read/write any of the
	// following members.
	sl         sync.Mutex
	state      shimExecState
	pid        int
	exitStatus uint32
	exitedAt   time.Time

	// exited is a wait block which waits async for the process to exit.
	exited chan struct{}
}

func (wpse *wcowPodSandboxExec) ID() string {
	return wpse.tid
}

func (wpse *wcowPodSandboxExec) Pid() int {
	wpse.sl.Lock()
	defer wpse.sl.Unlock()
	return wpse.pid
}

func (wpse *wcowPodSandboxExec) State() shimExecState {
	wpse.sl.Lock()
	defer wpse.sl.Unlock()
	return wpse.state
}

func (wpse *wcowPodSandboxExec) Status() *task.StateResponse {
	wpse.sl.Lock()
	defer wpse.sl.Unlock()

	var s containerd_v1_types.Status
	switch wpse.state {
	case shimExecStateCreated:
		s = containerd_v1_types.StatusCreated
	case shimExecStateRunning:
		s = containerd_v1_types.StatusRunning
	case shimExecStateExited:
		s = containerd_v1_types.StatusStopped
	}

	return &task.StateResponse{
		ID:         wpse.tid,
		ExecID:     wpse.tid, // Init exec ID is always same as Task ID
		Bundle:     wpse.bundle,
		Pid:        uint32(wpse.pid),
		Status:     s,
		Stdin:      "", // NilIO
		Stdout:     "", // NilIO
		Stderr:     "", // NilIO
		Terminal:   false,
		ExitStatus: wpse.exitStatus,
		ExitedAt:   wpse.exitedAt,
	}
}

func (wpse *wcowPodSandboxExec) Start(ctx context.Context) error {
	logrus.WithFields(logrus.Fields{
		"tid": wpse.tid,
		"eid": wpse.tid, // Init exec ID is always same as Task ID
	}).Debug("wcowPodSandboxExec::Start")

	wpse.sl.Lock()
	defer wpse.sl.Unlock()
	if wpse.state != shimExecStateCreated {
		return newExecInvalidStateError(wpse.tid, wpse.tid, wpse.state, "start")
	}
	// Transition the state
	wpse.state = shimExecStateRunning
	wpse.pid = 1 // Fake but init pid is always 1

	// Publish the task start event. We mever have an exec for the WCOW
	// PodSandbox.
	wpse.events(
		runtime.TaskStartEventTopic,
		&eventstypes.TaskStart{
			ContainerID: wpse.tid,
			Pid:         uint32(wpse.pid),
		})

	return nil
}

func (wpse *wcowPodSandboxExec) Kill(ctx context.Context, signal uint32) error {
	logrus.WithFields(logrus.Fields{
		"tid":    wpse.tid,
		"eid":    wpse.tid, // Init exec ID is always same as Task ID
		"signal": signal,
	}).Debug("wcowPodSandboxExec::Kill")

	wpse.sl.Lock()
	defer wpse.sl.Unlock()
	switch wpse.state {
	case shimExecStateCreated:
		wpse.state = shimExecStateExited
		wpse.exitStatus = 1
		wpse.exitedAt = time.Now()
		close(wpse.exited)
		return nil
	case shimExecStateRunning:
		// TODO: Should we verify that the signal would of killed the WCOW Process?
		wpse.state = shimExecStateExited
		wpse.exitStatus = 0
		wpse.exitedAt = time.Now()

		// NOTE: We do not support a non `init` exec for this "fake" init
		// process. Skip any exited event which will be sent by the task.

		close(wpse.exited)
		return nil
	case shimExecStateExited:
		return errors.Wrapf(errdefs.ErrNotFound, "exec: '%s' in task: '%s' not found", wpse.tid, wpse.tid)
	default:
		return newExecInvalidStateError(wpse.tid, wpse.tid, wpse.state, "kill")
	}
}

func (wpse *wcowPodSandboxExec) ResizePty(ctx context.Context, width, height uint32) error {
	logrus.WithFields(logrus.Fields{
		"tid":    wpse.tid,
		"eid":    wpse.tid, // Init exec ID is always same as Task ID
		"width":  width,
		"height": height,
	}).Debug("wcowPodSandboxExec::ResizePty")

	wpse.sl.Lock()
	defer wpse.sl.Unlock()
	if wpse.state != shimExecStateRunning {
		return newExecInvalidStateError(wpse.tid, wpse.tid, wpse.state, "resizepty")
	}
	// We will never have IO for a sandbox container so we wont have a tty
	// either.
	return errors.Wrapf(errdefs.ErrFailedPrecondition, "exec: '%s' in task: '%s' is not a tty", wpse.tid, wpse.tid)
}

func (wpse *wcowPodSandboxExec) CloseIO(ctx context.Context, stdin bool) error {
	logrus.WithFields(logrus.Fields{
		"tid":   wpse.tid,
		"eid":   wpse.tid, // Init exec ID is always same as Task ID
		"stdin": stdin,
	}).Debug("wcowPodSandboxExec::CloseIO")

	return nil
}

func (wpse *wcowPodSandboxExec) Wait(ctx context.Context) *task.StateResponse {
	logrus.WithFields(logrus.Fields{
		"tid": wpse.tid,
		"eid": wpse.tid, // Init exec ID is always same as Task ID
	}).Debug("wcowPodSandboxExec::Wait")

	<-wpse.exited
	return wpse.Status()
}

func (wpse *wcowPodSandboxExec) ForceExit(status int) {
	wpse.sl.Lock()
	defer wpse.sl.Unlock()
	if wpse.state != shimExecStateExited {
		// Avoid logging the force if we already exited gracefully
		logrus.WithFields(logrus.Fields{
			"tid":    wpse.tid,
			"eid":    wpse.tid,
			"status": status,
		}).Debug("hcsExec::ForceExit")

		wpse.state = shimExecStateExited
		wpse.exitStatus = 1
		wpse.exitedAt = time.Now()

		// NOTE: We do not support a non `init` exec for this "fake" init
		// process. Skip any exited event which will be sent by the task.

		close(wpse.exited)
	}
}
