package oci

import (
	"testing"

	"github.com/opencontainers/runtime-spec/specs-go"
)

func Test_IsLCOW_WCOW(t *testing.T) {
	s := &specs.Spec{
		Windows: &specs.Windows{},
	}
	if IsLCOW(s) {
		t.Fatal("should not have returned LCOW spec for WCOW config")
	}
}

func Test_IsLCOW_WCOW_Isolated(t *testing.T) {
	s := &specs.Spec{
		Windows: &specs.Windows{
			HyperV: &specs.WindowsHyperV{},
		},
	}
	if IsLCOW(s) {
		t.Fatal("should not have returned LCOW spec for WCOW isolated config")
	}
}

func Test_IsLCOW_Success(t *testing.T) {
	s := &specs.Spec{
		Linux: &specs.Linux{},
		Windows: &specs.Windows{
			HyperV: &specs.WindowsHyperV{},
		},
	}
	if !IsLCOW(s) {
		t.Fatal("should have returned LCOW spec")
	}
}

func Test_IsLCOW_NoWindows_Success(t *testing.T) {
	s := &specs.Spec{
		Linux: &specs.Linux{},
	}
	if !IsLCOW(s) {
		t.Fatal("should have returned LCOW spec")
	}
}

func Test_IsLCOW_Neither(t *testing.T) {
	s := &specs.Spec{}

	if IsLCOW(s) {
		t.Fatal("should not have returned LCOW spec for neither config")
	}
}

func Test_IsWCOW_Success(t *testing.T) {
	s := &specs.Spec{
		Windows: &specs.Windows{},
	}
	if !IsWCOW(s) {
		t.Fatal("should have returned WCOW spec for WCOW config")
	}
}

func Test_IsWCOW_Isolated_Success(t *testing.T) {
	s := &specs.Spec{
		Windows: &specs.Windows{
			HyperV: &specs.WindowsHyperV{},
		},
	}
	if !IsWCOW(s) {
		t.Fatal("should have returned WCOW spec for WCOW isolated config")
	}
}

func Test_IsWCOW_LCOW(t *testing.T) {
	s := &specs.Spec{
		Linux: &specs.Linux{},
		Windows: &specs.Windows{
			HyperV: &specs.WindowsHyperV{},
		},
	}
	if IsWCOW(s) {
		t.Fatal("should not have returned WCOW spec for LCOW config")
	}
}

func Test_IsWCOW_LCOW_NoWindows_Success(t *testing.T) {
	s := &specs.Spec{
		Linux: &specs.Linux{},
	}
	if IsWCOW(s) {
		t.Fatal("should not have returned WCOW spec for LCOW config")
	}
}

func Test_IsWCOW_Neither(t *testing.T) {
	s := &specs.Spec{}

	if IsWCOW(s) {
		t.Fatal("should not have returned WCOW spec for neither config")
	}
}

func Test_IsIsolated_WCOW(t *testing.T) {
	s := &specs.Spec{
		Windows: &specs.Windows{},
	}
	if IsIsolated(s) {
		t.Fatal("should not have returned isolated for WCOW config")
	}
}

func Test_IsIsolated_WCOW_Isolated(t *testing.T) {
	s := &specs.Spec{
		Windows: &specs.Windows{
			HyperV: &specs.WindowsHyperV{},
		},
	}
	if !IsIsolated(s) {
		t.Fatal("should have returned isolated for WCOW isolated config")
	}
}

func Test_IsIsolated_LCOW(t *testing.T) {
	s := &specs.Spec{
		Linux: &specs.Linux{},
		Windows: &specs.Windows{
			HyperV: &specs.WindowsHyperV{},
		},
	}
	if !IsIsolated(s) {
		t.Fatal("should have returned isolated for LCOW config")
	}
}

func Test_IsIsolated_LCOW_NoWindows(t *testing.T) {
	s := &specs.Spec{
		Linux: &specs.Linux{},
	}
	if !IsIsolated(s) {
		t.Fatal("should have returned isolated for LCOW config")
	}
}

func Test_IsIsolated_Neither(t *testing.T) {
	s := &specs.Spec{}

	if IsIsolated(s) {
		t.Fatal("should have not have returned isolated for neither config")
	}
}
