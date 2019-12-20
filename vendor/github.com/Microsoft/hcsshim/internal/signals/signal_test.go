package signals

import (
	"fmt"
	"testing"

	"github.com/Microsoft/hcsshim/internal/guestrequest"
)

func Test_ValidateSigstr_LCOW_Empty_No_SignalsSupported(t *testing.T) {
	ret, err := ValidateSigstrLCOW("", false)
	if err != nil {
		t.Fatalf("should return nil error got: %v", err)
	}
	if ret != nil {
		t.Fatalf("should return nil signal for LCOW Terminate usage got: %+v", ret)
	}
}

func Test_ValidateSigstr_LCOW_Empty_SignalsSupported(t *testing.T) {
	ret, err := ValidateSigstrLCOW("", true)
	if err != nil {
		t.Fatalf("should return nil error got: %v", err)
	}
	if ret == nil {
		t.Fatal("expected non-nil ret")
	}
	if ret.Signal != sigTerm {
		t.Fatalf("expected signal: %v, got: %v", sigTerm, ret.Signal)
	}
}

func Test_ValidateSigstr_LCOW_ValidSigstr_No_SignalsSupported(t *testing.T) {
	cases := []string{"TERM", "15", "KILL", "9"}
	for _, c := range cases {
		ret, err := ValidateSigstrLCOW(c, false)
		if err != nil {
			t.Fatalf("expected nil err for signal: %v got: %v", c, err)
		}
		if ret != nil {
			t.Fatalf("expected nil ret for signal: %v got: %+v", c, ret)
		}
	}
}

func Test_ValidateSigstr_LCOW_SignalsSupported(t *testing.T) {
	// Test map entry by string name
	for k, v := range signalMapLcow {
		ret, err := ValidateSigstrLCOW(k, true)
		if err != nil {
			t.Fatalf("expected nil err for signal: %v got: %v", k, err)
		}
		if ret == nil {
			t.Fatalf("expected non-nil ret for signal: %v", k)
		}
		if ret.Signal != v {
			t.Fatalf("expected signal: %v, got: %v", v, ret.Signal)
		}
	}

	// Test map entry by string value
	for k, v := range signalMapLcow {
		ret, err := ValidateSigstrLCOW(fmt.Sprintf("%d", v), true)
		if err != nil {
			t.Fatalf("expected nil err for signal: %v got: %v", k, err)
		}
		if ret == nil {
			t.Fatalf("expected non-nil ret for signal: %v", k)
		}
		if ret.Signal != v {
			t.Fatalf("expected signal: %v, got: %v", v, ret.Signal)
		}
	}
}

func Test_ValidateSigstr_Invalid_LCOW_No_SignalsSupported(t *testing.T) {
	cases := []string{"90", "test"}
	for _, c := range cases {
		ret, err := ValidateSigstrLCOW(c, false)
		if err != ErrInvalidSignal {
			t.Fatalf("expected %v err for signal: %v got: %v", ErrInvalidSignal, c, err)
		}
		if ret != nil {
			t.Fatalf("expected nil ret for signal: %v got: %+v", c, ret)
		}
	}
}

func Test_ValidateSigstr_Invalid_LCOW_SignalsSupported(t *testing.T) {
	cases := []string{"90", "SIGTEST"}
	for _, c := range cases {
		ret, err := ValidateSigstrLCOW(c, true)
		if err != ErrInvalidSignal {
			t.Fatalf("expected %v err for signal: %v got: %v", ErrInvalidSignal, c, err)
		}
		if ret != nil {
			t.Fatalf("expected nil ret for signal: %v got: %+v", c, ret)
		}
	}
}

func Test_ValidateSigstr_WCOW_Empty_No_SignalsSupported(t *testing.T) {
	ret, err := ValidateSigstrWCOW("", false)
	if err != nil {
		t.Fatalf("should return nil error got: %v", err)
	}
	if ret != nil {
		t.Fatalf("should return nil signal for WCOW Terminate usage got: %+v", ret)
	}
}

func Test_ValidateSigstr_WCOW_Empty_SignalsSupported(t *testing.T) {
	ret, err := ValidateSigstrWCOW("", true)
	if err != nil {
		t.Fatalf("should return nil error got: %v", err)
	}
	if ret == nil {
		t.Fatal("expected non-nil ret")
	}
	if ret.Signal != guestrequest.SignalValueWCOWCtrlShutdown {
		t.Fatalf("expected signal: %v, got: %v", guestrequest.SignalValueWCOWCtrlShutdown, ret.Signal)
	}
}

func Test_ValidateSigstr_WCOW_ValidSigstr_No_SignalsSupported(t *testing.T) {
	cases := []string{"CtrlShutdown", "6", "TERM", "15", "KILL", "9"}
	for _, c := range cases {
		ret, err := ValidateSigstrWCOW(c, false)
		if err != nil {
			t.Fatalf("expected nil err for signal: %v got: %v", c, err)
		}
		if ret != nil {
			t.Fatalf("expected nil ret for signal: %v got: %+v", c, ret)
		}
	}
}

func Test_ValidateSigstr_WCOW_SignalsSupported(t *testing.T) {
	type testcase struct {
		value  string
		result guestrequest.SignalValueWCOW
	}
	cases := []testcase{
		{
			"CtrlC",
			guestrequest.SignalValueWCOWCtrlC,
		},
		{
			"0",
			guestrequest.SignalValueWCOWCtrlC,
		},
		{
			"CtrlBreak",
			guestrequest.SignalValueWCOWCtrlBreak,
		},
		{
			"1",
			guestrequest.SignalValueWCOWCtrlBreak,
		},
		{
			"CtrlClose",
			guestrequest.SignalValueWCOWCtrlClose,
		},
		{
			"2",
			guestrequest.SignalValueWCOWCtrlClose,
		},
		{
			"CtrlLogOff",
			guestrequest.SignalValueWCOWCtrlLogOff,
		},
		{
			"5",
			guestrequest.SignalValueWCOWCtrlLogOff,
		},
		{
			"CtrlShutdown",
			guestrequest.SignalValueWCOWCtrlShutdown,
		},
		{
			"6",
			guestrequest.SignalValueWCOWCtrlShutdown,
		},
		{
			"TERM",
			guestrequest.SignalValueWCOWCtrlShutdown,
		},
		{
			"15",
			guestrequest.SignalValueWCOWCtrlShutdown,
		},
		{
			"KILL",
			guestrequest.SignalValueWCOW("<invalid>"),
		},
		{
			"9",
			guestrequest.SignalValueWCOW("<invalid>"),
		},
	}
	for _, c := range cases {
		ret, err := ValidateSigstrWCOW(c.value, true)
		if err != nil {
			t.Fatalf("expected nil err for signal: %v got: %v", c.value, err)
		}
		if c.result == guestrequest.SignalValueWCOW("<invalid>") {
			if ret != nil {
				t.Fatalf("expected nil ret for signal: %v got: %+v", c.value, ret)
			}
		} else {
			if ret == nil {
				t.Fatalf("expected non-nil ret for signal: %v", c.value)
			}
		}
	}
}

func Test_ValidateSigstr_Invalid_WCOW_No_SignalsSupported(t *testing.T) {
	cases := []string{"2", "test"}
	for _, c := range cases {
		ret, err := ValidateSigstrWCOW(c, false)
		if err != ErrInvalidSignal {
			t.Fatalf("expected %v err for signal: %v got: %v", ErrInvalidSignal, c, err)
		}
		if ret != nil {
			t.Fatalf("expected nil ret for signal: %v got: %+v", c, ret)
		}
	}
}

func Test_ValidateSigstr_Invalid_WCOW_SignalsSupported(t *testing.T) {
	cases := []string{"20", "CtrlTest"}
	for _, c := range cases {
		ret, err := ValidateSigstrWCOW(c, true)
		if err != ErrInvalidSignal {
			t.Fatalf("expected %v err for signal: %v got: %v", ErrInvalidSignal, c, err)
		}
		if ret != nil {
			t.Fatalf("expected nil ret for signal: %v got: %+v", c, ret)
		}
	}
}

func Test_ValidateLCOW_SignalsSupported(t *testing.T) {
	for _, v := range signalMapLcow {
		ret, err := ValidateLCOW(v, true)
		if err != nil {
			t.Fatalf("expected nil err for signal: %v got: %v", v, err)
		}
		if ret == nil {
			t.Fatalf("expected non-nil ret for signal: %v", v)
		}
		if ret.Signal != v {
			t.Fatalf("expected signal: %v got: %v", v, ret.Signal)
		}
	}
}

func Test_ValidateWCOW_SignalsSupported(t *testing.T) {
	type testcase struct {
		value  int
		result guestrequest.SignalValueWCOW
	}
	cases := []testcase{
		{
			ctrlC,
			guestrequest.SignalValueWCOWCtrlC,
		},
		{
			ctrlBreak,
			guestrequest.SignalValueWCOWCtrlBreak,
		},
		{
			ctrlClose,
			guestrequest.SignalValueWCOWCtrlClose,
		},
		{
			ctrlLogOff,
			guestrequest.SignalValueWCOWCtrlLogOff,
		},
		{
			ctrlShutdown,
			guestrequest.SignalValueWCOWCtrlShutdown,
		},
		{
			sigTerm,
			guestrequest.SignalValueWCOWCtrlShutdown,
		},
		{
			sigKill,
			guestrequest.SignalValueWCOW("<invalid>"),
		},
	}
	for _, c := range cases {
		ret, err := ValidateWCOW(c.value, true)
		if err != nil {
			t.Fatalf("expected nil err for signal: %v got: %v", c.value, err)
		}
		if c.result == guestrequest.SignalValueWCOW("<invalid>") {
			if ret != nil {
				t.Fatalf("expected nil ret for signal: %v got: %+v", c.value, ret)
			}
		} else {
			if ret == nil {
				t.Fatalf("expected non-nil ret for signal: %v", c.value)
			}
			if ret.Signal != c.result {
				t.Fatalf("expected signal: %v, got: %v", c.result, ret.Signal)
			}
		}
	}
}
