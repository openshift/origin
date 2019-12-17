package main

import (
	"context"
	"time"

	"github.com/Microsoft/hcsshim/cmd/containerd-shim-runhcs-v1/options"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/runtime/v2/task"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

var _ = (shimTask)(&testShimTask{})

type testShimTask struct {
	id string

	exec  *testShimExec
	execs map[string]*testShimExec
}

func (tst *testShimTask) ID() string {
	return tst.id
}

func (tst *testShimTask) CreateExec(ctx context.Context, req *task.ExecProcessRequest, s *specs.Process) error {
	return errdefs.ErrNotImplemented
}

func (tst *testShimTask) GetExec(eid string) (shimExec, error) {
	if eid == "" {
		return tst.exec, nil
	}
	e, ok := tst.execs[eid]
	if ok {
		return e, nil
	}
	return nil, errdefs.ErrNotFound
}

func (tst *testShimTask) KillExec(ctx context.Context, eid string, signal uint32, all bool) error {
	e, err := tst.GetExec(eid)
	if err != nil {
		return err
	}
	return e.Kill(ctx, signal)
}

func (tst *testShimTask) DeleteExec(ctx context.Context, eid string) (int, uint32, time.Time, error) {
	e, err := tst.GetExec(eid)
	if err != nil {
		return 0, 0, time.Time{}, err
	}
	status := e.Status()
	if eid != "" {
		delete(tst.execs, eid)
	}
	return int(status.Pid), status.ExitStatus, status.ExitedAt, nil
}

func (tst *testShimTask) Pids(ctx context.Context) ([]options.ProcessDetails, error) {
	pairs := []options.ProcessDetails{
		{
			ProcessID: uint32(tst.exec.Pid()),
			ExecID:    tst.exec.ID(),
		},
	}
	for _, p := range tst.execs {
		pairs = append(pairs, options.ProcessDetails{
			ProcessID: uint32(p.pid),
			ExecID:    p.id,
		})
	}
	return pairs, nil
}

func (tst *testShimTask) Wait(ctx context.Context) *task.StateResponse {
	return tst.exec.Wait(ctx)
}
