package main

import (
	"context"
	"math/rand"
	"strconv"
	"testing"

	"github.com/Microsoft/hcsshim/cmd/containerd-shim-runhcs-v1/options"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/runtime/v2/task"
	"github.com/containerd/typeurl"
)

func setupTaskServiceWithFakes(t *testing.T) (*service, *testShimTask, *testShimExec) {
	tid := strconv.Itoa(rand.Int())
	s := service{
		tid:       tid,
		isSandbox: false,
	}
	task := &testShimTask{
		id:    tid,
		exec:  newTestShimExec(tid, tid, 10),
		execs: make(map[string]*testShimExec),
	}
	secondExecID := strconv.Itoa(rand.Int())
	secondExec := newTestShimExec(tid, secondExecID, 101)
	task.execs[secondExecID] = secondExec
	s.taskOrPod.Store(task)
	return &s, task, secondExec
}

func Test_TaskShim_getTask_NotCreated_Error(t *testing.T) {
	s := service{
		tid:       t.Name(),
		isSandbox: false,
	}

	st, err := s.getTask(t.Name())

	verifyExpectedError(t, st, err, errdefs.ErrNotFound)
}

func Test_TaskShim_getTask_Created_DifferentID_Error(t *testing.T) {
	s, _, _ := setupTaskServiceWithFakes(t)

	st, err := s.getTask("thisidwontmatch")

	verifyExpectedError(t, st, err, errdefs.ErrNotFound)
}

func Test_TaskShim_getTask_Created_CorrectID_Success(t *testing.T) {
	s, t1, _ := setupTaskServiceWithFakes(t)

	st, err := s.getTask(t1.ID())
	if err != nil {
		t.Fatalf("should have not failed with error, got: %v", err)
	}
	if st != t1 {
		t.Fatal("should have returned a valid task")
	}
}

func Test_TaskShim_stateInternal_NoTask_Error(t *testing.T) {
	s := service{
		tid:       t.Name(),
		isSandbox: false,
	}

	resp, err := s.stateInternal(context.TODO(), &task.StateRequest{ID: t.Name()})

	verifyExpectedError(t, resp, err, errdefs.ErrNotFound)
}

func Test_TaskShim_stateInternal_InitTaskID_DifferentExecID_Error(t *testing.T) {
	s, t1, _ := setupTaskServiceWithFakes(t)

	resp, err := s.stateInternal(context.TODO(), &task.StateRequest{
		ID:     t1.ID(),
		ExecID: "thisshouldnotmatch",
	})

	verifyExpectedError(t, resp, err, errdefs.ErrNotFound)
}

func Test_TaskShim_stateInternal_InitTaskID_InitExecID_Success(t *testing.T) {
	s, t1, _ := setupTaskServiceWithFakes(t)

	resp, err := s.stateInternal(context.TODO(), &task.StateRequest{
		ID:     t1.ID(),
		ExecID: "",
	})
	if err != nil {
		t.Fatalf("should not have failed with error got: %v", err)
	}
	if resp == nil {
		t.Fatal("should have returned StateResponse")
	}
	if resp.ID != t1.ID() {
		t.Fatalf("StateResponse.ID expected '%s' got '%s'", t1.ID(), resp.ID)
	}
	if resp.ExecID != t1.ID() {
		t.Fatalf("StateResponse.ExecID expected '%s' got '%s'", t1.ID(), resp.ExecID)
	}
	if resp.Pid != uint32(t1.exec.pid) {
		t.Fatalf("should have returned init pid, got: %v", resp.Pid)
	}
}

func Test_TaskShim_stateInternal_InitTaskID_2ndExecID_Success(t *testing.T) {
	s, t1, e2 := setupTaskServiceWithFakes(t)

	resp, err := s.stateInternal(context.TODO(), &task.StateRequest{
		ID:     t1.ID(),
		ExecID: e2.ID(),
	})
	if err != nil {
		t.Fatalf("should not have failed with error got: %v", err)
	}
	if resp == nil {
		t.Fatal("should have returned StateResponse")
	}
	if resp.ID != t1.ID() {
		t.Fatalf("StateResponse.ID expected '%s' got '%s'", t1.ID(), resp.ID)
	}
	if resp.ExecID != e2.ID() {
		t.Fatalf("StateResponse.ExecID expected '%s' got '%s'", e2.ID(), resp.ExecID)
	}
	if resp.Pid != uint32(t1.execs[e2.ID()].pid) {
		t.Fatalf("should have returned 2nd exec pid, got: %v", resp.Pid)
	}
}

// TODO: Test_TaskShim_createInternal_*

func Test_TaskShim_startInternal_NoTask_Error(t *testing.T) {
	s := service{
		tid:       t.Name(),
		isSandbox: true,
	}

	resp, err := s.startInternal(context.TODO(), &task.StartRequest{ID: t.Name()})

	verifyExpectedError(t, resp, err, errdefs.ErrNotFound)
}

func Test_TaskShim_startInternal_ValidTask_DifferentExecID_Error(t *testing.T) {
	s, t1, _ := setupTaskServiceWithFakes(t)

	resp, err := s.startInternal(context.TODO(), &task.StartRequest{
		ID:     t1.ID(),
		ExecID: "thisshouldnotmatch",
	})

	verifyExpectedError(t, resp, err, errdefs.ErrNotFound)
}

func Test_TaskShim_startInternal_InitTaskID_InitExecID_Success(t *testing.T) {
	s, t1, _ := setupTaskServiceWithFakes(t)

	resp, err := s.startInternal(context.TODO(), &task.StartRequest{
		ID:     t1.ID(),
		ExecID: "",
	})
	if err != nil {
		t.Fatalf("should not have failed with error got: %v", err)
	}
	if resp == nil {
		t.Fatal("should have returned StartResponse")
	}
	if resp.Pid != uint32(t1.exec.pid) {
		t.Fatal("should have returned init pid")
	}
}

func Test_TaskShim_startInternal_InitTaskID_2ndExecID_Success(t *testing.T) {
	s, t1, e2 := setupTaskServiceWithFakes(t)

	resp, err := s.startInternal(context.TODO(), &task.StartRequest{
		ID:     t1.ID(),
		ExecID: e2.ID(),
	})
	if err != nil {
		t.Fatalf("should not have failed with error got: %v", err)
	}
	if resp == nil {
		t.Fatal("should have returned StartResponse")
	}
	if resp.Pid != uint32(t1.execs[e2.ID()].pid) {
		t.Fatal("should have returned 2nd pid")
	}
}

func Test_TaskShim_deleteInternal_NoTask_Error(t *testing.T) {
	s := service{
		tid:       t.Name(),
		isSandbox: true,
	}

	resp, err := s.deleteInternal(context.TODO(), &task.DeleteRequest{ID: t.Name()})

	verifyExpectedError(t, resp, err, errdefs.ErrNotFound)
}

func Test_TaskShim_deleteInternal_ValidTask_DifferentExecID_Error(t *testing.T) {
	s, t1, _ := setupTaskServiceWithFakes(t)

	resp, err := s.deleteInternal(context.TODO(), &task.DeleteRequest{
		ID:     t1.ID(),
		ExecID: "thisshouldnotmatch",
	})

	verifyExpectedError(t, resp, err, errdefs.ErrNotFound)
}

func Test_TaskShim_deleteInternal_InitTaskID_InitExecID_Success(t *testing.T) {
	s, t1, _ := setupTaskServiceWithFakes(t)

	resp, err := s.deleteInternal(context.TODO(), &task.DeleteRequest{
		ID:     t1.ID(),
		ExecID: "",
	})
	if err != nil {
		t.Fatalf("should not have failed with error got: %v", err)
	}
	if resp == nil {
		t.Fatal("should have returned DeleteResponse")
	}
	if resp.Pid != uint32(t1.exec.pid) {
		t.Fatal("should have returned init pid")
	}
}

func Test_TaskShim_deleteInternal_InitTaskID_2ndExecID_Success(t *testing.T) {
	s, t1, e2 := setupTaskServiceWithFakes(t)

	// capture the t2 task as it will be deleted
	t2t := t1.execs[e2.ID()]

	resp, err := s.deleteInternal(context.TODO(), &task.DeleteRequest{
		ID:     t1.ID(),
		ExecID: e2.ID(),
	})
	if err != nil {
		t.Fatalf("should not have failed with error got: %v", err)
	}
	if resp == nil {
		t.Fatal("should have returned DeleteResponse")
	}
	if resp.Pid != uint32(t2t.pid) {
		t.Fatal("should have returned 2nd pid")
	}
	if _, ok := t1.execs[e2.ID()]; ok {
		t.Fatal("should have deleted the 2nd exec")
	}
}

func Test_TaskShim_pidsInternal_NoTask_Error(t *testing.T) {
	s := service{
		tid:       t.Name(),
		isSandbox: true,
	}

	resp, err := s.pidsInternal(context.TODO(), &task.PidsRequest{ID: t.Name()})

	verifyExpectedError(t, resp, err, errdefs.ErrNotFound)
}

func Test_TaskShim_pidsInternal_InitTaskID_Success(t *testing.T) {
	s, t1, e2 := setupTaskServiceWithFakes(t)

	resp, err := s.pidsInternal(context.TODO(), &task.PidsRequest{ID: t1.ID()})
	if err != nil {
		t.Fatalf("should not have failed with error got: %v", err)
	}
	if resp == nil {
		t.Fatal("should have returned PidsResponse")
	}
	if len(resp.Processes) != 2 {
		t.Fatalf("should have returned len(processes) == 1, got: %v", len(resp.Processes))
	}
	if resp.Processes[0].Pid != uint32(t1.exec.pid) {
		t.Fatal("should have returned init pid")
	}
	if resp.Processes[0].Info == nil {
		t.Fatal("should have returned init pid info")
	}
	if resp.Processes[1].Pid != uint32(t1.execs[e2.ID()].pid) {
		t.Fatal("should have returned 2nd pid")
	}
	if resp.Processes[1].Info == nil {
		t.Fatal("should have returned 2nd pid info")
	}
	u, err := typeurl.UnmarshalAny(resp.Processes[1].Info)
	if err != nil {
		t.Fatalf("failed to unmarshal 2nd pid info, err: %v", err)
	}
	pi := u.(*options.ProcessDetails)
	if pi.ExecID != e2.ID() {
		t.Fatalf("should have returned 2nd pid ExecID, got: %v", pi.ExecID)
	}
}

func Test_TaskShim_pauseInternal_Error(t *testing.T) {
	s := service{
		tid:       t.Name(),
		isSandbox: true,
	}

	resp, err := s.pauseInternal(context.TODO(), &task.PauseRequest{ID: t.Name()})

	verifyExpectedError(t, resp, err, errdefs.ErrNotImplemented)
}

func Test_TaskShim_resumeInternal_Error(t *testing.T) {
	s := service{
		tid:       t.Name(),
		isSandbox: true,
	}

	resp, err := s.resumeInternal(context.TODO(), &task.ResumeRequest{ID: t.Name()})

	verifyExpectedError(t, resp, err, errdefs.ErrNotImplemented)
}

func Test_TaskShim_checkpointInternal_Error(t *testing.T) {
	s := service{
		tid:       t.Name(),
		isSandbox: true,
	}

	resp, err := s.checkpointInternal(context.TODO(), &task.CheckpointTaskRequest{ID: t.Name()})

	verifyExpectedError(t, resp, err, errdefs.ErrNotImplemented)
}

func Test_TaskShim_killInternal_NoTask_Error(t *testing.T) {
	s := service{
		tid:       t.Name(),
		isSandbox: true,
	}

	resp, err := s.killInternal(context.TODO(), &task.KillRequest{ID: t.Name()})

	verifyExpectedError(t, resp, err, errdefs.ErrNotFound)
}

func Test_TaskShim_killInternal_InitTaskID_DifferentExecID_Error(t *testing.T) {
	s, t1, _ := setupTaskServiceWithFakes(t)

	resp, err := s.killInternal(context.TODO(), &task.KillRequest{
		ID:     t1.ID(),
		ExecID: "thisshouldnotmatch",
	})

	verifyExpectedError(t, resp, err, errdefs.ErrNotFound)
}

func Test_TaskShim_killInternal_InitTaskID_InitExecID_Success(t *testing.T) {
	s, t1, _ := setupTaskServiceWithFakes(t)

	resp, err := s.killInternal(context.TODO(), &task.KillRequest{
		ID:     t1.ID(),
		ExecID: "",
	})
	if err != nil {
		t.Fatalf("should not have failed with error got: %v", err)
	}
	if resp == nil {
		t.Fatal("should have returned KillResponse")
	}
}

func Test_TaskShim_killInternal_InitTaskID_2ndExecID_Success(t *testing.T) {
	s, t1, e2 := setupTaskServiceWithFakes(t)

	resp, err := s.killInternal(context.TODO(), &task.KillRequest{
		ID:     t1.ID(),
		ExecID: e2.ID(),
	})
	if err != nil {
		t.Fatalf("should not have failed with error got: %v", err)
	}
	if resp == nil {
		t.Fatal("should have returned KillResponse")
	}
}

// TODO: Test_TaskShim_execInternal_*

func Test_TaskShim_resizePtyInternal_NoTask_Error(t *testing.T) {
	s := service{
		tid:       t.Name(),
		isSandbox: true,
	}

	resp, err := s.resizePtyInternal(context.TODO(), &task.ResizePtyRequest{ID: t.Name()})

	verifyExpectedError(t, resp, err, errdefs.ErrNotFound)
}

func Test_TaskShim_resizePtyInternal_InitTaskID_DifferentExecID_Error(t *testing.T) {
	s, t1, _ := setupTaskServiceWithFakes(t)

	resp, err := s.resizePtyInternal(context.TODO(), &task.ResizePtyRequest{
		ID:     t1.ID(),
		ExecID: "thisshouldnotmatch",
	})

	verifyExpectedError(t, resp, err, errdefs.ErrNotFound)
}

func Test_TaskShim_resizePtyInternal_InitTaskID_InitExecID_Success(t *testing.T) {
	s, t1, _ := setupTaskServiceWithFakes(t)

	resp, err := s.resizePtyInternal(context.TODO(), &task.ResizePtyRequest{
		ID:     t1.ID(),
		ExecID: "",
	})
	if err != nil {
		t.Fatalf("should not have failed with error got: %v", err)
	}
	if resp == nil {
		t.Fatal("should have returned ResizePtyResponse")
	}
}

func Test_TaskShim_resizePtyInternal_InitTaskID_2ndExecID_Success(t *testing.T) {
	s, t1, e2 := setupTaskServiceWithFakes(t)

	resp, err := s.resizePtyInternal(context.TODO(), &task.ResizePtyRequest{
		ID:     t1.ID(),
		ExecID: e2.ID(),
	})
	if err != nil {
		t.Fatalf("should not have failed with error got: %v", err)
	}
	if resp == nil {
		t.Fatal("should have returned ResizePtyResponse")
	}
}

func Test_TaskShim_closeIOInternal_NoTask_Error(t *testing.T) {
	s := service{
		tid:       t.Name(),
		isSandbox: true,
	}

	resp, err := s.closeIOInternal(context.TODO(), &task.CloseIORequest{ID: t.Name()})

	verifyExpectedError(t, resp, err, errdefs.ErrNotFound)
}

func Test_TaskShim_closeIOInternal_InitTaskID_DifferentExecID_Error(t *testing.T) {
	s, t1, _ := setupTaskServiceWithFakes(t)

	resp, err := s.closeIOInternal(context.TODO(), &task.CloseIORequest{
		ID:     t1.ID(),
		ExecID: "thisshouldnotmatch",
	})

	verifyExpectedError(t, resp, err, errdefs.ErrNotFound)
}

func Test_TaskShim_closeIOInternal_InitTaskID_InitExecID_Success(t *testing.T) {
	s, t1, _ := setupTaskServiceWithFakes(t)

	resp, err := s.closeIOInternal(context.TODO(), &task.CloseIORequest{
		ID:     t1.ID(),
		ExecID: "",
	})
	if err != nil {
		t.Fatalf("should not have failed with error got: %v", err)
	}
	if resp == nil {
		t.Fatal("should have returned CloseIOResponse")
	}
}

func Test_TaskShim_closeIOInternal_InitTaskID_2ndExecID_Success(t *testing.T) {
	s, t1, e2 := setupTaskServiceWithFakes(t)

	resp, err := s.closeIOInternal(context.TODO(), &task.CloseIORequest{
		ID:     t1.ID(),
		ExecID: e2.ID(),
	})
	if err != nil {
		t.Fatalf("should not have failed with error got: %v", err)
	}
	if resp == nil {
		t.Fatal("should have returned CloseIOResponse")
	}
}

func Test_TaskShim_updateInternal_Error(t *testing.T) {
	s := service{
		tid:       t.Name(),
		isSandbox: true,
	}

	resp, err := s.updateInternal(context.TODO(), &task.UpdateTaskRequest{ID: t.Name()})

	verifyExpectedError(t, resp, err, errdefs.ErrNotImplemented)
}

func Test_TaskShim_waitInternal_NoTask_Error(t *testing.T) {
	s := service{
		tid:       t.Name(),
		isSandbox: true,
	}

	resp, err := s.waitInternal(context.TODO(), &task.WaitRequest{ID: t.Name()})

	verifyExpectedError(t, resp, err, errdefs.ErrNotFound)
}

func Test_TaskShim_waitInternal_InitTaskID_DifferentExecID_Error(t *testing.T) {
	s, t1, _ := setupTaskServiceWithFakes(t)

	resp, err := s.waitInternal(context.TODO(), &task.WaitRequest{
		ID:     t1.ID(),
		ExecID: "thisshouldnotmatch",
	})

	verifyExpectedError(t, resp, err, errdefs.ErrNotFound)
}

func Test_TaskShim_waitInternal_InitTaskID_InitExecID_Success(t *testing.T) {
	s, t1, _ := setupTaskServiceWithFakes(t)

	resp, err := s.waitInternal(context.TODO(), &task.WaitRequest{
		ID:     t1.ID(),
		ExecID: "",
	})
	if err != nil {
		t.Fatalf("should not have failed with error got: %v", err)
	}
	if resp == nil {
		t.Fatal("should have returned WaitResponse")
	}
	if resp.ExitStatus != t1.exec.Status().ExitStatus {
		t.Fatal("should have returned exit status for init")
	}
}

func Test_TaskShim_waitInternal_InitTaskID_2ndExecID_Success(t *testing.T) {
	s, t1, e2 := setupTaskServiceWithFakes(t)

	resp, err := s.waitInternal(context.TODO(), &task.WaitRequest{
		ID:     t1.ID(),
		ExecID: e2.ID(),
	})
	if err != nil {
		t.Fatalf("should not have failed with error got: %v", err)
	}
	if resp == nil {
		t.Fatal("should have returned WaitResponse")
	}
	if resp.ExitStatus != t1.execs[e2.ID()].Status().ExitStatus {
		t.Fatal("should have returned exit status for init")
	}
}

func Test_TaskShim_statsInternal_Error(t *testing.T) {
	s := service{
		tid:       t.Name(),
		isSandbox: true,
	}

	resp, err := s.statsInternal(context.TODO(), &task.StatsRequest{ID: t.Name()})

	verifyExpectedError(t, resp, err, errdefs.ErrNotImplemented)
}
