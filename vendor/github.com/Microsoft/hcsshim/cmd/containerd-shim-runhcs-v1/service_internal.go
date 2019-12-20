package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	runhcsopts "github.com/Microsoft/hcsshim/cmd/containerd-shim-runhcs-v1/options"
	"github.com/Microsoft/hcsshim/internal/oci"
	containerd_v1_types "github.com/containerd/containerd/api/types/task"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/runtime/v2/task"
	"github.com/containerd/typeurl"
	google_protobuf1 "github.com/gogo/protobuf/types"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

var empty = &google_protobuf1.Empty{}

// getPod returns the pod this shim is tracking or else returns `nil`. It is the
// callers responsibility to verify that `s.isSandbox == true` before calling
// this method.
//
//
// If `pod==nil` returns `errdefs.ErrFailedPrecondition`.
func (s *service) getPod() (shimPod, error) {
	raw := s.taskOrPod.Load()
	if raw == nil {
		return nil, errors.Wrapf(errdefs.ErrFailedPrecondition, "task with id: '%s' must be created first", s.tid)
	}
	return raw.(shimPod), nil
}

// getTask returns a task matching `tid` or else returns `nil`. This properly
// handles a task in a pod or a singular task shim.
//
// If `tid` is not found will return `errdefs.ErrNotFound`.
func (s *service) getTask(tid string) (shimTask, error) {
	raw := s.taskOrPod.Load()
	if raw == nil {
		return nil, errors.Wrapf(errdefs.ErrNotFound, "task with id: '%s' not found", tid)
	}
	if s.isSandbox {
		p := raw.(shimPod)
		return p.GetTask(tid)
	}
	// When its not a sandbox only the init task is a valid id.
	if s.tid != tid {
		return nil, errors.Wrapf(errdefs.ErrNotFound, "task with id: '%s' not found", tid)
	}
	return raw.(shimTask), nil
}

func (s *service) stateInternal(ctx context.Context, req *task.StateRequest) (*task.StateResponse, error) {
	t, err := s.getTask(req.ID)
	if err != nil {
		return nil, err
	}
	e, err := t.GetExec(req.ExecID)
	if err != nil {
		return nil, err
	}
	return e.Status(), nil
}

func (s *service) createInternal(ctx context.Context, req *task.CreateTaskRequest) (*task.CreateTaskResponse, error) {
	setupDebuggerEvent()

	var shimOpts *runhcsopts.Options
	if req.Options != nil {
		v, err := typeurl.UnmarshalAny(req.Options)
		if err != nil {
			return nil, err
		}
		shimOpts = v.(*runhcsopts.Options)
	}

	if shimOpts != nil && shimOpts.Debug {
		logrus.SetLevel(logrus.DebugLevel)
	}

	var spec specs.Spec
	f, err := os.Open(filepath.Join(req.Bundle, "config.json"))
	if err != nil {
		return nil, err
	}
	if err := json.NewDecoder(f).Decode(&spec); err != nil {
		f.Close()
		return nil, err
	}
	f.Close()

	spec = oci.UpdateSpecFromOptions(spec, shimOpts)

	if len(req.Rootfs) == 0 {
		// If no mounts are passed via the snapshotter its the callers full
		// responsibility to manage the storage. Just move on without affecting
		// the config.json at all.
		if spec.Windows == nil || len(spec.Windows.LayerFolders) < 2 {
			return nil, errors.Wrap(errdefs.ErrFailedPrecondition, "no Windows.LayerFolders found in oci spec")
		}
	} else if len(req.Rootfs) != 1 {
		return nil, errors.Wrap(errdefs.ErrFailedPrecondition, "Rootfs does not contain exactly 1 mount for the root file system")
	} else {
		m := req.Rootfs[0]
		if m.Type != "windows-layer" && m.Type != "lcow-layer" {
			return nil, errors.Wrapf(errdefs.ErrFailedPrecondition, "unsupported mount type '%s'", m.Type)
		}

		// parentLayerPaths are passed in layerN, layerN-1, ..., layer 0
		//
		// The OCI spec expects:
		//   layerN, layerN-1, ..., layer0, scratch
		var parentLayerPaths []string
		for _, option := range m.Options {
			if strings.HasPrefix(option, mount.ParentLayerPathsFlag) {
				err := json.Unmarshal([]byte(option[len(mount.ParentLayerPathsFlag):]), &parentLayerPaths)
				if err != nil {
					return nil, errors.Wrapf(errdefs.ErrFailedPrecondition, "failed to unmarshal parent layer paths from mount: %v", err)
				}
			}
		}

		if m.Type == "lcow-layer" {
			// If we are creating LCOW make sure that spec.Windows is filled out before
			// appending layer folders.
			if spec.Windows == nil {
				spec.Windows = &specs.Windows{}
			}
			if spec.Windows.HyperV == nil {
				spec.Windows.HyperV = &specs.WindowsHyperV{}
			}
		} else if spec.Windows.HyperV == nil {
			// This is a Windows Argon make sure that we have a Root filled in.
			if spec.Root == nil {
				spec.Root = &specs.Root{}
			}
		}

		// Append the parents
		spec.Windows.LayerFolders = append(spec.Windows.LayerFolders, parentLayerPaths...)
		// Append the scratch
		spec.Windows.LayerFolders = append(spec.Windows.LayerFolders, m.Source)
	}

	if req.Terminal && req.Stderr != "" {
		return nil, errors.Wrap(errdefs.ErrFailedPrecondition, "if using terminal, stderr must be empty")
	}

	resp := &task.CreateTaskResponse{}
	s.cl.Lock()
	if s.isSandbox {
		pod, err := s.getPod()
		if err == nil {
			// The POD sandbox was previously created. Unlock and forward to the POD
			s.cl.Unlock()
			t, err := pod.CreateTask(ctx, req, &spec)
			if err != nil {
				return nil, err
			}
			e, _ := t.GetExec("")
			resp.Pid = uint32(e.Pid())
			return resp, nil
		}
		pod, err = createPod(ctx, s.events, req, &spec)
		if err != nil {
			s.cl.Unlock()
			return nil, err
		}
		t, _ := pod.GetTask(req.ID)
		e, _ := t.GetExec("")
		resp.Pid = uint32(e.Pid())
		s.taskOrPod.Store(pod)
	} else {
		t, err := newHcsStandaloneTask(ctx, s.events, req, &spec)
		if err != nil {
			s.cl.Unlock()
			return nil, err
		}
		e, _ := t.GetExec("")
		resp.Pid = uint32(e.Pid())
		s.taskOrPod.Store(t)
	}
	s.cl.Unlock()
	return resp, nil
}

func (s *service) startInternal(ctx context.Context, req *task.StartRequest) (*task.StartResponse, error) {
	t, err := s.getTask(req.ID)
	if err != nil {
		return nil, err
	}
	e, err := t.GetExec(req.ExecID)
	if err != nil {
		return nil, err
	}
	err = e.Start(ctx)
	if err != nil {
		return nil, err
	}
	return &task.StartResponse{
		Pid: uint32(e.Pid()),
	}, nil
}

func (s *service) deleteInternal(ctx context.Context, req *task.DeleteRequest) (*task.DeleteResponse, error) {
	// TODO: JTERRY75 we need to send this to the POD for isSandbox

	t, err := s.getTask(req.ID)
	if err != nil {
		return nil, err
	}
	pid, exitStatus, exitedAt, err := t.DeleteExec(ctx, req.ExecID)
	if err != nil {
		return nil, err
	}
	// TODO: We should be removing the task after this right?
	return &task.DeleteResponse{
		Pid:        uint32(pid),
		ExitStatus: exitStatus,
		ExitedAt:   exitedAt,
	}, nil
}

func (s *service) pidsInternal(ctx context.Context, req *task.PidsRequest) (*task.PidsResponse, error) {
	t, err := s.getTask(req.ID)
	if err != nil {
		return nil, err
	}
	pids, err := t.Pids(ctx)
	if err != nil {
		return nil, err
	}
	processes := make([]*containerd_v1_types.ProcessInfo, len(pids))
	for i, p := range pids {
		a, err := typeurl.MarshalAny(&p)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to marshal ProcessDetails for process: %s, task: %s", p.ExecID, req.ID)
		}
		proc := &containerd_v1_types.ProcessInfo{
			Pid:  p.ProcessID,
			Info: a,
		}
		processes[i] = proc
	}
	return &task.PidsResponse{
		Processes: processes,
	}, nil
}

func (s *service) pauseInternal(ctx context.Context, req *task.PauseRequest) (*google_protobuf1.Empty, error) {
	/*
		s.events <- cdevent{
			topic: runtime.TaskPausedEventTopic,
			event: &eventstypes.TaskPaused{
				req.ID,
			},
		}
	*/
	return nil, errdefs.ErrNotImplemented
}

func (s *service) resumeInternal(ctx context.Context, req *task.ResumeRequest) (*google_protobuf1.Empty, error) {
	/*
		s.events <- cdevent{
			topic: runtime.TaskResumedEventTopic,
			event: &eventstypes.TaskResumed{
				req.ID,
			},
		}
	*/
	return nil, errdefs.ErrNotImplemented
}

func (s *service) checkpointInternal(ctx context.Context, req *task.CheckpointTaskRequest) (*google_protobuf1.Empty, error) {
	return nil, errdefs.ErrNotImplemented
}

func (s *service) killInternal(ctx context.Context, req *task.KillRequest) (*google_protobuf1.Empty, error) {
	if s.isSandbox {
		pod, err := s.getPod()
		if err != nil {
			return nil, errors.Wrapf(errdefs.ErrNotFound, "%v: task with id: '%s' not found", err, req.ID)
		}
		// Send it to the POD and let it cascade on its own through all tasks.
		err = pod.KillTask(ctx, req.ID, req.ExecID, req.Signal, req.All)
		if err != nil {
			return nil, err
		}
		return empty, nil
	}
	t, err := s.getTask(req.ID)
	if err != nil {
		return nil, err
	}
	// Send it to the task and let it cascade on its own through all exec's
	err = t.KillExec(ctx, req.ExecID, req.Signal, req.All)
	if err != nil {
		return nil, err
	}
	return empty, nil
}

func (s *service) execInternal(ctx context.Context, req *task.ExecProcessRequest) (*google_protobuf1.Empty, error) {
	t, err := s.getTask(req.ID)
	if err != nil {
		return nil, err
	}
	if req.Terminal && req.Stderr != "" {
		return nil, errors.Wrap(errdefs.ErrFailedPrecondition, "if using terminal, stderr must be empty")
	}
	var spec specs.Process
	if err := json.Unmarshal(req.Spec.Value, &spec); err != nil {
		return nil, errors.Wrap(err, "request.Spec was not oci process")
	}
	err = t.CreateExec(ctx, req, &spec)
	if err != nil {
		return nil, err
	}
	return empty, nil
}

func (s *service) resizePtyInternal(ctx context.Context, req *task.ResizePtyRequest) (*google_protobuf1.Empty, error) {
	t, err := s.getTask(req.ID)
	if err != nil {
		return nil, err
	}
	e, err := t.GetExec(req.ExecID)
	if err != nil {
		return nil, err
	}
	err = e.ResizePty(ctx, req.Width, req.Height)
	if err != nil {
		return nil, err
	}
	return empty, nil
}

func (s *service) closeIOInternal(ctx context.Context, req *task.CloseIORequest) (*google_protobuf1.Empty, error) {
	t, err := s.getTask(req.ID)
	if err != nil {
		return nil, err
	}
	e, err := t.GetExec(req.ExecID)
	if err != nil {
		return nil, err
	}
	err = e.CloseIO(ctx, req.Stdin)
	if err != nil {
		return nil, err
	}
	return empty, nil
}

func (s *service) updateInternal(ctx context.Context, req *task.UpdateTaskRequest) (*google_protobuf1.Empty, error) {
	return nil, errdefs.ErrNotImplemented
}

func (s *service) waitInternal(ctx context.Context, req *task.WaitRequest) (*task.WaitResponse, error) {
	t, err := s.getTask(req.ID)
	if err != nil {
		return nil, err
	}
	var state *task.StateResponse
	if req.ExecID != "" {
		e, err := t.GetExec(req.ExecID)
		if err != nil {
			return nil, err
		}
		state = e.Wait(ctx)
	} else {
		state = t.Wait(ctx)
	}
	return &task.WaitResponse{
		ExitStatus: state.ExitStatus,
		ExitedAt:   state.ExitedAt,
	}, nil
}

func (s *service) statsInternal(ctx context.Context, req *task.StatsRequest) (*task.StatsResponse, error) {
	return nil, errdefs.ErrNotImplemented
}

func (s *service) connectInternal(ctx context.Context, req *task.ConnectRequest) (*task.ConnectResponse, error) {
	// We treat the shim/task as the same pid on the Windows host.
	pid := uint32(os.Getpid())
	return &task.ConnectResponse{
		ShimPid: pid,
		TaskPid: pid,
	}, nil
}

func (s *service) shutdownInternal(ctx context.Context, req *task.ShutdownRequest) (*google_protobuf1.Empty, error) {
	// Because a pod shim hosts multiple tasks only the init task can issue the
	// shutdown request.
	if req.ID != s.tid {
		return empty, nil
	}

	if req.Now {
		os.Exit(0)
	}
	// TODO: JTERRY75 if we dont use `now` issue a Shutdown to the ttrpc
	// connection to drain any active requests.
	os.Exit(0)
	return empty, nil
}
