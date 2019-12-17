package main

import (
	"context"
	"math/rand"
	"strconv"
	"sync"
	"testing"

	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/runtime/v2/task"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

var _ = (shimPod)(&testShimPod{})

type testShimPod struct {
	id string

	tasks sync.Map
}

func (tsp *testShimPod) ID() string {
	return tsp.id
}

func (tsp *testShimPod) CreateTask(ctx context.Context, req *task.CreateTaskRequest, s *specs.Spec) (shimTask, error) {
	return nil, errdefs.ErrNotImplemented
}

func (tsp *testShimPod) GetTask(tid string) (shimTask, error) {
	v, loaded := tsp.tasks.Load(tid)
	if loaded {
		return v.(shimTask), nil
	}
	return nil, errdefs.ErrNotFound
}

func (tsp *testShimPod) KillTask(ctx context.Context, tid, eid string, signal uint32, all bool) error {
	s, err := tsp.GetTask(tid)
	if err != nil {
		return err
	}
	return s.KillExec(ctx, eid, signal, all)
}

// Pod tests

func setupTestPodWithFakes(t *testing.T) (*pod, *testShimTask) {
	st := &testShimTask{
		id:    t.Name(),
		exec:  newTestShimExec(t.Name(), t.Name(), 10),
		execs: make(map[string]*testShimExec),
	}
	// Add a 2nd exec
	seid := strconv.Itoa(rand.Int())
	st.execs[seid] = newTestShimExec(t.Name(), seid, int(rand.Int31()))
	p := &pod{
		id:          t.Name(),
		sandboxTask: st,
	}
	return p, st
}

func setupTestTaskInPod(t *testing.T, p *pod) *testShimTask {
	tid := strconv.Itoa(rand.Int())
	wt := &testShimTask{
		id:   tid,
		exec: newTestShimExec(tid, tid, int(rand.Int31())),
	}
	p.workloadTasks.Store(wt.id, wt)
	return wt
}

func Test_pod_ID(t *testing.T) {
	p := pod{id: t.Name()}
	id := p.ID()
	if id != t.Name() {
		t.Fatalf("pod should of returned ID: %s, got: %s", t.Name(), id)
	}
}

func Test_pod_GetTask_SandboxID(t *testing.T) {
	p, st := setupTestPodWithFakes(t)
	t1, err := p.GetTask(t.Name())
	if err != nil {
		t.Fatalf("should not have failed, got: %v", err)
	}
	if t1 != st {
		t.Fatal("should have returned sandbox task")
	}
}

func Test_pod_GetTask_WorkloadID_NotCreated_Error(t *testing.T) {
	p, _ := setupTestPodWithFakes(t)
	t1, err := p.GetTask("thisshouldnotmatch")

	verifyExpectedError(t, t1, err, errdefs.ErrNotFound)
}

func Test_pod_GetTask_WorkloadID_Created_Success(t *testing.T) {
	p, _ := setupTestPodWithFakes(t)
	t2 := setupTestTaskInPod(t, p)

	resp, err := p.GetTask(t2.ID())
	if err != nil {
		t.Fatalf("should not have failed, got: %v", err)
	}
	if resp != t2 {
		t.Fatal("should have returned workload task")
	}
}

func Test_pod_KillTask_UnknownTaskID_Error(t *testing.T) {
	p, _ := setupTestPodWithFakes(t)
	err := p.KillTask(context.TODO(), "thisshouldnotmatch", "", 0xf, false)

	verifyExpectedError(t, nil, err, errdefs.ErrNotFound)
}

func Test_pod_KillTask_SandboxID_UnknownExecID_Error(t *testing.T) {
	p, _ := setupTestPodWithFakes(t)
	err := p.KillTask(context.TODO(), t.Name(), "thisshouldnotmatch", 0xf, false)

	verifyExpectedError(t, nil, err, errdefs.ErrNotFound)
}

func Test_pod_KillTask_SandboxID_InitExecID_Success(t *testing.T) {
	p, _ := setupTestPodWithFakes(t)
	err := p.KillTask(context.TODO(), t.Name(), "", 0xf, false)
	if err != nil {
		t.Fatalf("should not have failed, got: %v", err)
	}
}

func Test_pod_KillTask_SandboxID_InitExecID_All_Success(t *testing.T) {
	p, _ := setupTestPodWithFakes(t)
	// Add two workload tasks
	setupTestTaskInPod(t, p)
	setupTestTaskInPod(t, p)
	err := p.KillTask(context.TODO(), t.Name(), "", 0xf, true)
	if err != nil {
		t.Fatalf("should not have failed, got: %v", err)
	}
}

func Test_pod_KillTask_SandboxID_2ndExecID_Success(t *testing.T) {
	p, t1 := setupTestPodWithFakes(t)
	for k := range t1.execs {
		err := p.KillTask(context.TODO(), t.Name(), k, 0xf, false)
		if err != nil {
			t.Fatalf("should not have failed, got: %v", err)
		}
	}
}

func Test_pod_KillTask_SandboxID_2ndExecID_All_Error(t *testing.T) {
	p, t1 := setupTestPodWithFakes(t)
	for k := range t1.execs {
		err := p.KillTask(context.TODO(), t.Name(), k, 0xf, true)

		verifyExpectedError(t, nil, err, errdefs.ErrFailedPrecondition)
	}
}

func Test_pod_KillTask_WorkloadID_InitExecID_Success(t *testing.T) {
	p, _ := setupTestPodWithFakes(t)
	t1 := setupTestTaskInPod(t, p)

	err := p.KillTask(context.TODO(), t1.ID(), "", 0xf, false)
	if err != nil {
		t.Fatalf("should not have failed, got: %v", err)
	}
}

func Test_pod_KillTask_WorkloadID_InitExecID_All_Success(t *testing.T) {
	p, _ := setupTestPodWithFakes(t)
	t1 := setupTestTaskInPod(t, p)

	err := p.KillTask(context.TODO(), t1.ID(), "", 0xf, true)
	if err != nil {
		t.Fatalf("should not have failed, got: %v", err)
	}
}

func Test_pod_KillTask_WorkloadID_2ndExecID_Success(t *testing.T) {
	p, _ := setupTestPodWithFakes(t)
	t1 := setupTestTaskInPod(t, p)

	for k := range t1.execs {
		err := p.KillTask(context.TODO(), t1.ID(), k, 0xf, false)
		if err != nil {
			t.Fatalf("should not have failed, got: %v", err)
		}
	}
}

func Test_pod_KillTask_WorkloadID_2ndExecID_All_Error(t *testing.T) {
	p, _ := setupTestPodWithFakes(t)
	t1 := setupTestTaskInPod(t, p)

	for k := range t1.execs {
		err := p.KillTask(context.TODO(), t1.ID(), k, 0xf, true)

		verifyExpectedError(t, nil, err, errdefs.ErrFailedPrecondition)
	}
}
