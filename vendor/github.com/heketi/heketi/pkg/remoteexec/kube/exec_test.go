//
// Copyright (c) 2018 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package kube

import (
	"os"
	"strings"
	"testing"

	"github.com/heketi/tests"

	rex "github.com/heketi/heketi/pkg/remoteexec"
)

// test timeout options
var tto = TimeoutOptions{1, false}

// TestExecCommands tests running commands on an actual container.
// To be honest, this is a bit hokey but was an expedient way to actually
// test some of the code in this package. To use, build the tests with
// 'go test -c' and copy the binary to a working heketi pod. Then set
// the environment variable to something like:
//    KUBE_EXEC_TEST_CTL='incluster||daemonset|default|node2|glusterfs-node'
// This is the closest to how heketi currently runs most of the time.
// It derives the k8s connection from "in cluster", uses a "daemonset"
// lookup in the "default" namespace on "node2" with a label of
// "glusterfs-node". Other look up strings can be used, please read the
// source. Currently only incluster configuration is supported, in theory
// the 1st string could indicate out-of-cluster and the 2nd any additional
// connection params.
// (Told you it was hokey)
func TestExecCommands(t *testing.T) {
	testControl := os.Getenv("KUBE_EXEC_TEST_CTL")
	if testControl == "" {
		t.Skipf("Test will not run with empty KUBE_EXEC_TEST_CTL env var.")
	}
	v := strings.Split(testControl, "|")
	if len(v) < 4 {
		t.Skipf("Too few subsections in KUBE_EXEC_TEST_CTL env var. Got: %v",
			testControl)
	}
	if v[0] != "incluster" {
		t.Skipf("Unexpected kube config in KUBE_EXEC_TEST_CTL env var. Got: %v",
			testControl)
	}

	var (
		tp  TargetPod
		err error
		l   = &dummyLogger{}
		// extracted from test control string
		targetValue = v[2]
		targetNs    = v[3]
	)
	kc, err := NewKubeConn(l)
	switch targetValue {
	case "pod":
		t.Logf("Using given pod name: %v", v[4])
		tp = TargetPod{
			target: target{
				Namespace: targetNs,
			},
			PodName: v[4],
		}
	case "label":
		t.Logf("Using label: %v=%v", v[4], v[5])
		tgt := TargetLabel{}
		tgt.Namespace = targetNs
		tgt.Key = v[4]
		tgt.Value = v[5]
		tp, err = tgt.GetTargetPod(kc)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	case "daemonset":
		t.Logf("Using daemonset: %v, %v", v[4], v[5])
		tgt := TargetDaemonSet{}
		tgt.Namespace = targetNs
		tgt.Host = v[4]
		tgt.Selector = v[5]
		tp, err = tgt.GetTargetPod(kc)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	default:
		t.Fatalf("Invalid target value: %v", targetValue)
	}
	tc, err := tp.FirstContainer(kc)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	t.Run("simple", func(t *testing.T) { testExecSimple(t, kc, tc) })
	t.Run("simple3", func(t *testing.T) { testExecSimple3(t, kc, tc) })
	t.Run("stopEarly", func(t *testing.T) { testExecStopEarly(t, kc, tc) })
}

func testExecSimple(t *testing.T, kc *KubeConn, tc TargetContainer) {
	r, err := ExecCommands(kc, tc, rex.ToCmds([]string{"ls /"}), tto)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(r) == 1)
}

func testExecSimple3(t *testing.T, kc *KubeConn, tc TargetContainer) {
	cmds := []string{
		"ls /proc",
		"true",
		"false",
	}
	r, err := ExecCommands(kc, tc, rex.ToCmds(cmds), tto)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(r) == 3)
	tests.Assert(t, r[0].Ok(), "expected r0 OK")
	tests.Assert(t, r[1].Ok(), "expected r1 OK")
	tests.Assert(t, !r[2].Ok(), "expected r2 not OK")
}

func testExecStopEarly(t *testing.T, kc *KubeConn, tc TargetContainer) {
	cmds := []string{
		"false",
		"ls /proc",
		"true",
	}
	r, err := ExecCommands(kc, tc, rex.ToCmds(cmds), tto)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(r) == 3)
	tests.Assert(t, !r[0].Ok(), "expected r0 not OK")
	tests.Assert(t, !r[1].Ok(), "expected r1 not OK")
	tests.Assert(t, !r[2].Ok(), "expected r2 not OK")
	tests.Assert(t, r[0].Completed, "expected r0 completed")
	tests.Assert(t, !r[1].Completed, "expected r1 not completed")
	tests.Assert(t, !r[2].Completed, "expected r2 not completed")
}
