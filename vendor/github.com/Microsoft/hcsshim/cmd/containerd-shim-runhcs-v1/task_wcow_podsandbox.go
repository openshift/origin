package main

import (
	"context"
	"sync"
	"time"

	"github.com/Microsoft/hcsshim/cmd/containerd-shim-runhcs-v1/options"
	"github.com/Microsoft/hcsshim/internal/hcs"
	"github.com/Microsoft/hcsshim/internal/uvm"
	eventstypes "github.com/containerd/containerd/api/events"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/runtime"
	"github.com/containerd/containerd/runtime/v2/task"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// newWcowPodSandboxTask creates a fake WCOW task with a fake WCOW `init`
// process as a performance optimization rather than creating an actual
// container and process since it is not needed to hold open any namespaces like
// the equivalent on Linux.
//
// It is assumed that this is the only fake WCOW task and that this task owns
// `parent`. When the fake WCOW `init` process exits via `Signal` `parent` will
// be forcibly closed by this task.
func newWcowPodSandboxTask(ctx context.Context, events publisher, id, bundle string, parent *uvm.UtilityVM) shimTask {
	logrus.WithFields(logrus.Fields{
		"tid": id,
	}).Debug("newWcowPodSandboxTask")

	wpst := &wcowPodSandboxTask{
		events: events,
		id:     id,
		init:   newWcowPodSandboxExec(ctx, events, id, bundle),
		host:   parent,
		closed: make(chan struct{}),
	}
	if parent != nil {
		// We have (and own) a parent UVM. Listen for its exit and forcibly
		// close this task. This is not expected but in the event of a UVM crash
		// we need to handle this case.
		go func() {
			werr := parent.Wait()
			if werr != nil && !hcs.IsAlreadyClosed(werr) {
				logrus.WithFields(logrus.Fields{
					"tid":           id,
					logrus.ErrorKey: werr,
				}).Error("newWcowPodSandboxTask - UVM Wait failed")
			}
			// The UVM came down. Force transition the init task (if it wasn't
			// already) to unblock any waiters since the platform wont send any
			// events for this fake process.
			wpst.init.ForceExit(1)

			// Close the host and event the exit.
			wpst.close()
		}()
	}
	// In the normal case the `Signal` call from the caller killed this fake
	// init process.
	go func() {
		// Wait for it to exit on its own
		wpst.init.Wait(context.Background())

		// Close the host and event the exit
		wpst.close()
	}()
	return wpst
}

var _ = (shimTask)(&wcowPodSandboxTask{})

// wcowPodSandboxTask is a special task type that actually holds no real
// resources due to various design differences between Linux/Windows.
//
// For more information on why we can have this stub and in what invariant cases
// it makes sense please see `wcowPodExec`.
//
// Note: If this is a Hypervisor Isolated WCOW sandbox then we do actually track
// the lifetime of the UVM for a WCOW POD but the UVM will have no WCOW
// container/exec init representing the actual POD Sandbox task.
type wcowPodSandboxTask struct {
	events publisher
	// id is the id of this task when it is created.
	//
	// It MUST be treated as read only in the liftetime of the task.
	id string
	// init is the init process of the container.
	//
	// Note: the invariant `container state == init.State()` MUST be true. IE:
	// if the init process exits the container as a whole and all exec's MUST
	// exit.
	//
	// It MUST be treated as read only in the lifetime of the task.
	init *wcowPodSandboxExec
	// host is the hosting VM for this task if hypervisor isolated. If
	// `host==nil` this is an Argon task so no UVM cleanup is required.
	host *uvm.UtilityVM

	closed    chan struct{}
	closeOnce sync.Once
}

func (wpst *wcowPodSandboxTask) ID() string {
	return wpst.id
}

func (wpst *wcowPodSandboxTask) CreateExec(ctx context.Context, req *task.ExecProcessRequest, s *specs.Process) error {
	logrus.WithFields(logrus.Fields{
		"tid": wpst.id,
		"eid": req.ID,
	}).Debug("wcowPodSandboxTask::CreateExec")

	return errors.Wrap(errdefs.ErrNotImplemented, "WCOW Pod task should never issue exec")
}

func (wpst *wcowPodSandboxTask) GetExec(eid string) (shimExec, error) {
	if eid == "" {
		return wpst.init, nil
	}
	// Cannot exec in an a WCOW sandbox container so all non-init calls fail here.
	return nil, errors.Wrapf(errdefs.ErrNotFound, "exec: '%s' in task: '%s' not found", eid, wpst.id)
}

func (wpst *wcowPodSandboxTask) KillExec(ctx context.Context, eid string, signal uint32, all bool) error {
	logrus.WithFields(logrus.Fields{
		"tid":    wpst.id,
		"eid":    eid,
		"signal": signal,
		"all":    all,
	}).Debug("wcowPodSandboxTask::KillExec")

	e, err := wpst.GetExec(eid)
	if err != nil {
		return err
	}
	if all && eid != "" {
		return errors.Wrapf(errdefs.ErrFailedPrecondition, "cannot signal all for non-empty exec: '%s'", eid)
	}
	err = e.Kill(ctx, signal)
	if err != nil {
		return err
	}
	return nil
}

func (wpst *wcowPodSandboxTask) DeleteExec(ctx context.Context, eid string) (int, uint32, time.Time, error) {
	logrus.WithFields(logrus.Fields{
		"tid": wpst.id,
		"eid": eid,
	}).Debug("wcowPodSandboxTask::DeleteExec")

	e, err := wpst.GetExec(eid)
	if err != nil {
		return 0, 0, time.Time{}, err
	}
	switch state := e.State(); state {
	case shimExecStateCreated:
		e.ForceExit(0)
	case shimExecStateRunning:
		return 0, 0, time.Time{}, newExecInvalidStateError(wpst.id, eid, state, "delete")
	}
	status := e.Status()

	// Publish the deleted event
	wpst.events(
		runtime.TaskDeleteEventTopic,
		&eventstypes.TaskDelete{
			ContainerID: wpst.id,
			ID:          eid,
			Pid:         status.Pid,
			ExitStatus:  status.ExitStatus,
			ExitedAt:    status.ExitedAt,
		})

	return int(status.Pid), status.ExitStatus, status.ExitedAt, nil
}

func (wpst *wcowPodSandboxTask) Pids(ctx context.Context) ([]options.ProcessDetails, error) {
	logrus.WithFields(logrus.Fields{
		"tid": wpst.id,
	}).Debug("wcowPodSandboxTask::Pids")

	return []options.ProcessDetails{
		{
			ProcessID: uint32(wpst.init.Pid()),
			ExecID:    wpst.init.ID(),
		},
	}, nil
}

func (wpst *wcowPodSandboxTask) Wait(ctx context.Context) *task.StateResponse {
	<-wpst.closed
	return wpst.init.Wait(ctx)
}

// close safely closes the hosting UVM. Because of the specialty of this task it
// is assumed that this is always the owner of `wpst.host`. Once closed and all
// resources released it events the `runtime.TaskExitEventTopic` for all
// upstream listeners.
//
// This call is idempotent and safe to call multiple times.
func (wpst *wcowPodSandboxTask) close() {
	wpst.closeOnce.Do(func() {
		logrus.WithFields(logrus.Fields{
			"tid": wpst.id,
		}).Debug("wcowPodSandboxTask::close")

		if wpst.host != nil {
			if err := wpst.host.Close(); !hcs.IsAlreadyClosed(err) {
				logrus.WithFields(logrus.Fields{
					"tid":           wpst.id,
					logrus.ErrorKey: err,
				}).Error("wcowPodSandboxTask::close - failed host vm shutdown")
			}
		}
		// Send the `init` exec exit notification always.
		exit := wpst.init.Status()
		wpst.events(
			runtime.TaskExitEventTopic,
			&eventstypes.TaskExit{
				ContainerID: wpst.id,
				ID:          exit.ID,
				Pid:         uint32(exit.Pid),
				ExitStatus:  exit.ExitStatus,
				ExitedAt:    exit.ExitedAt,
			})
		close(wpst.closed)
	})
}
