//
// Copyright (c) 2018 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package stack

import (
	"fmt"
	"testing"

	"github.com/heketi/tests"

	"github.com/heketi/heketi/executors"
	"github.com/heketi/heketi/executors/mockexec"
)

func TestNewExecutorStack(t *testing.T) {
	es := NewExecutorStack(
		NewExecutorStack(),
		NewExecutorStack())
	tests.Assert(t, len(es.executors) == 2,
		"expected len(es.executors) == 2, got:", len(es.executors))
}

func TestNewExecutorStackMock(t *testing.T) {
	m1, _ := mockexec.NewMockExecutor()
	m2, _ := mockexec.NewMockExecutor()
	es := NewExecutorStack(m1, m2)
	tests.Assert(t, len(es.executors) == 2,
		"expected len(es.executors) == 2, got:", len(es.executors))
}

func TestGlusterdCheck(t *testing.T) {
	m1, _ := mockexec.NewMockExecutor()
	m2, _ := mockexec.NewMockExecutor()
	es := NewExecutorStack(m1, m2)
	tests.Assert(t, len(es.executors) == 2,
		"expected len(es.executors) == 2, got:", len(es.executors))

	err := es.GlusterdCheck("foo")
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	m1.MockGlusterdCheck = func(h string) error {
		return fmt.Errorf("F1")
	}

	err = es.GlusterdCheck("foo")
	tests.Assert(t, err.Error() == "F1", "expected err == F1, got:", err)

	m1.MockGlusterdCheck = func(h string) error {
		return nil
	}
	m2.MockGlusterdCheck = func(h string) error {
		return fmt.Errorf("F2")
	}

	err = es.GlusterdCheck("foo")
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	es.CheckAllGlusterd = true
	err = es.GlusterdCheck("foo")
	tests.Assert(t, err.Error() == "F2", "expected err == F2, got:", err)

	m1.MockGlusterdCheck = func(h string) error {
		return executors.NotSupportedError
	}
	m2.MockGlusterdCheck = func(h string) error {
		return executors.NotSupportedError
	}
	err = es.GlusterdCheck("foo")
	tests.Assert(t, err == executors.NotSupportedError, "expected err == NotSupportedError, got:", err)
}

func TestPeerProbe(t *testing.T) {
	m1, _ := mockexec.NewMockExecutor()
	m2, _ := mockexec.NewMockExecutor()
	es := NewExecutorStack(m1, m2)
	tests.Assert(t, len(es.executors) == 2,
		"expected len(es.executors) == 2, got:", len(es.executors))

	err := es.PeerProbe("foo", "bar")
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	m2.MockPeerProbe = func(h, n string) error {
		return fmt.Errorf("E2")
	}

	err = es.PeerProbe("foo", "bar")
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	m1.MockPeerProbe = func(h, n string) error {
		return NotSupportedError
	}

	err = es.PeerProbe("foo", "bar")
	tests.Assert(t, err.Error() == "E2", "expected err == E2, got:", err)

	m2.MockPeerProbe = func(h, n string) error {
		return NotSupportedError
	}
	err = es.PeerProbe("foo", "bar")
	tests.Assert(t, err == NotSupportedError, "expected err == NotSupportedError, got:", err)
}

func TestPeerDetach(t *testing.T) {
	m1, _ := mockexec.NewMockExecutor()
	m2, _ := mockexec.NewMockExecutor()
	es := NewExecutorStack(m1, m2)
	tests.Assert(t, len(es.executors) == 2,
		"expected len(es.executors) == 2, got:", len(es.executors))

	err := es.PeerDetach("foo", "bar")
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	m2.MockPeerDetach = func(h, n string) error {
		return fmt.Errorf("E2")
	}

	err = es.PeerDetach("foo", "bar")
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	m1.MockPeerDetach = func(h, n string) error {
		return NotSupportedError
	}

	err = es.PeerDetach("foo", "bar")
	tests.Assert(t, err.Error() == "E2", "expected err == E2, got:", err)

	m2.MockPeerDetach = func(h, n string) error {
		return NotSupportedError
	}
	err = es.PeerDetach("foo", "bar")
	tests.Assert(t, err == NotSupportedError, "expected err == NotSupportedError, got:", err)
}

func TestDeviceSetup(t *testing.T) {
	m1, _ := mockexec.NewMockExecutor()
	m2, _ := mockexec.NewMockExecutor()
	es := NewExecutorStack(m1, m2)
	tests.Assert(t, len(es.executors) == 2,
		"expected len(es.executors) == 2, got:", len(es.executors))

	_, err := es.DeviceSetup("foo", "bar", "v", true)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	m2.MockDeviceSetup = func(h, n, v string, d bool) (*executors.DeviceInfo, error) {
		return nil, fmt.Errorf("E2")
	}

	_, err = es.DeviceSetup("foo", "bar", "v", true)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	m1.MockDeviceSetup = func(h, n, v string, d bool) (*executors.DeviceInfo, error) {
		return nil, NotSupportedError
	}

	_, err = es.DeviceSetup("foo", "bar", "v", false)
	tests.Assert(t, err.Error() == "E2", "expected err == E2, got:", err)

	m2.MockDeviceSetup = func(h, n, v string, d bool) (*executors.DeviceInfo, error) {
		return nil, NotSupportedError
	}
	_, err = es.DeviceSetup("foo", "bar", "v", false)
	tests.Assert(t, err == NotSupportedError, "expected err == NotSupportedError, got:", err)
}

func TestGetDeviceInfo(t *testing.T) {
	m1, _ := mockexec.NewMockExecutor()
	m2, _ := mockexec.NewMockExecutor()
	es := NewExecutorStack(m1, m2)
	tests.Assert(t, len(es.executors) == 2,
		"expected len(es.executors) == 2, got:", len(es.executors))

	_, err := es.GetDeviceInfo("foo", "bar", "v")
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	m2.MockGetDeviceInfo = func(h, d, v string) (*executors.DeviceInfo, error) {
		return nil, fmt.Errorf("E2")
	}

	_, err = es.GetDeviceInfo("foo", "bar", "v")
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	m1.MockGetDeviceInfo = func(h, d, v string) (*executors.DeviceInfo, error) {
		return nil, NotSupportedError
	}

	_, err = es.GetDeviceInfo("foo", "bar", "v")
	tests.Assert(t, err != nil, "expected err != nil")
	tests.Assert(t, err.Error() == "E2", "expected err == E2, got:", err)

	m2.MockGetDeviceInfo = func(h, d, v string) (*executors.DeviceInfo, error) {
		return nil, NotSupportedError
	}
	_, err = es.GetDeviceInfo("foo", "bar", "v")
	tests.Assert(t, err == NotSupportedError, "expected err == NotSupportedError, got:", err)
}

func TestDeviceTeardown(t *testing.T) {
	m1, _ := mockexec.NewMockExecutor()
	m2, _ := mockexec.NewMockExecutor()
	es := NewExecutorStack(m1, m2)
	tests.Assert(t, len(es.executors) == 2,
		"expected len(es.executors) == 2, got:", len(es.executors))

	err := es.DeviceTeardown("foo", "bar", "v")
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	m2.MockDeviceTeardown = func(h, n, v string) error {
		return fmt.Errorf("E2")
	}

	err = es.DeviceTeardown("foo", "bar", "v")
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	m1.MockDeviceTeardown = func(h, n, v string) error {
		return NotSupportedError
	}

	err = es.DeviceTeardown("foo", "bar", "v")
	tests.Assert(t, err.Error() == "E2", "expected err == E2, got:", err)

	m2.MockDeviceTeardown = func(h, n, v string) error {
		return NotSupportedError
	}
	err = es.DeviceTeardown("foo", "bar", "v")
	tests.Assert(t, err == NotSupportedError, "expected err == NotSupportedError, got:", err)
}

func TestBrickCreate(t *testing.T) {
	m1, _ := mockexec.NewMockExecutor()
	m2, _ := mockexec.NewMockExecutor()
	es := NewExecutorStack(m1, m2)
	tests.Assert(t, len(es.executors) == 2,
		"expected len(es.executors) == 2, got:", len(es.executors))

	br := &executors.BrickRequest{}

	_, err := es.BrickCreate("foo", br)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	m2.MockBrickCreate = func(host string, brick *executors.BrickRequest) (*executors.BrickInfo, error) {
		return nil, fmt.Errorf("E2")
	}

	_, err = es.BrickCreate("foo", br)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	m1.MockBrickCreate = func(host string, brick *executors.BrickRequest) (*executors.BrickInfo, error) {
		return nil, NotSupportedError
	}

	_, err = es.BrickCreate("foo", br)
	tests.Assert(t, err.Error() == "E2", "expected err == E2, got:", err)

	m2.MockBrickCreate = func(host string, brick *executors.BrickRequest) (*executors.BrickInfo, error) {
		return nil, NotSupportedError
	}
	_, err = es.BrickCreate("foo", br)
	tests.Assert(t, err == NotSupportedError, "expected err == NotSupportedError, got:", err)
}

func TestBrickDestroy(t *testing.T) {
	m1, _ := mockexec.NewMockExecutor()
	m2, _ := mockexec.NewMockExecutor()
	es := NewExecutorStack(m1, m2)
	tests.Assert(t, len(es.executors) == 2,
		"expected len(es.executors) == 2, got:", len(es.executors))

	br := &executors.BrickRequest{}

	_, err := es.BrickDestroy("foo", br)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	m2.MockBrickDestroy = func(host string, brick *executors.BrickRequest) (bool, error) {
		return false, fmt.Errorf("E2")
	}

	_, err = es.BrickDestroy("foo", br)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	m1.MockBrickDestroy = func(host string, brick *executors.BrickRequest) (bool, error) {
		return false, NotSupportedError
	}

	_, err = es.BrickDestroy("foo", br)
	tests.Assert(t, err.Error() == "E2", "expected err == E2, got:", err)

	m2.MockBrickDestroy = func(host string, brick *executors.BrickRequest) (bool, error) {
		return false, NotSupportedError
	}
	_, err = es.BrickDestroy("foo", br)
	tests.Assert(t, err == NotSupportedError, "expected err == NotSupportedError, got:", err)
}

func TestVolumeCreate(t *testing.T) {
	m1, _ := mockexec.NewMockExecutor()
	m2, _ := mockexec.NewMockExecutor()
	es := NewExecutorStack(m1, m2)
	tests.Assert(t, len(es.executors) == 2,
		"expected len(es.executors) == 2, got:", len(es.executors))

	br := &executors.VolumeRequest{}

	_, err := es.VolumeCreate("foo", br)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	m2.MockVolumeCreate = func(host string, brick *executors.VolumeRequest) (*executors.Volume, error) {
		return nil, fmt.Errorf("E2")
	}

	_, err = es.VolumeCreate("foo", br)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	m1.MockVolumeCreate = func(host string, brick *executors.VolumeRequest) (*executors.Volume, error) {
		return nil, NotSupportedError
	}

	_, err = es.VolumeCreate("foo", br)
	tests.Assert(t, err.Error() == "E2", "expected err == E2, got:", err)

	m2.MockVolumeCreate = func(host string, brick *executors.VolumeRequest) (*executors.Volume, error) {
		return nil, NotSupportedError
	}
	_, err = es.VolumeCreate("foo", br)
	tests.Assert(t, err == NotSupportedError, "expected err == NotSupportedError, got:", err)
}

func TestVolumeDestroy(t *testing.T) {
	m1, _ := mockexec.NewMockExecutor()
	m2, _ := mockexec.NewMockExecutor()
	es := NewExecutorStack(m1, m2)
	tests.Assert(t, len(es.executors) == 2,
		"expected len(es.executors) == 2, got:", len(es.executors))

	err := es.VolumeDestroy("foo", "v")
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	m2.MockVolumeDestroy = func(h, v string) error {
		return fmt.Errorf("E2")
	}

	err = es.VolumeDestroy("foo", "v")
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	m1.MockVolumeDestroy = func(h, v string) error {
		return NotSupportedError
	}

	err = es.VolumeDestroy("foo", "v")
	tests.Assert(t, err.Error() == "E2", "expected err == E2, got:", err)

	m2.MockVolumeDestroy = func(h, v string) error {
		return NotSupportedError
	}
	err = es.VolumeDestroy("foo", "v")
	tests.Assert(t, err == NotSupportedError, "expected err == NotSupportedError, got:", err)
}

func TestVolumeDestroyCheck(t *testing.T) {
	m1, _ := mockexec.NewMockExecutor()
	m2, _ := mockexec.NewMockExecutor()
	es := NewExecutorStack(m1, m2)
	tests.Assert(t, len(es.executors) == 2,
		"expected len(es.executors) == 2, got:", len(es.executors))

	err := es.VolumeDestroyCheck("foo", "v")
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	m2.MockVolumeDestroyCheck = func(h, v string) error {
		return fmt.Errorf("E2")
	}

	err = es.VolumeDestroyCheck("foo", "v")
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	m1.MockVolumeDestroyCheck = func(h, v string) error {
		return NotSupportedError
	}

	err = es.VolumeDestroyCheck("foo", "v")
	tests.Assert(t, err.Error() == "E2", "expected err == E2, got:", err)

	m2.MockVolumeDestroyCheck = func(h, v string) error {
		return NotSupportedError
	}
	err = es.VolumeDestroyCheck("foo", "v")
	tests.Assert(t, err == NotSupportedError, "expected err == NotSupportedError, got:", err)
}
