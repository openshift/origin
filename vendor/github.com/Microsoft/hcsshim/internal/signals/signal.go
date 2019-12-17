package signals

import (
	"errors"
	"strconv"
	"strings"

	"github.com/Microsoft/hcsshim/internal/guestrequest"
)

var (
	// ErrInvalidSignal is the standard error for an invalid signal for a given
	// flavor of container WCOW/LCOW.
	ErrInvalidSignal = errors.New("invalid signal value")
)

// ValidateSigstrLCOW validates that `sigstr` is an acceptable signal for LCOW
// based on `signalsSupported`.
//
// `sigstr` may either be the text name or integer value of the signal.
//
// If `signalsSupported==false` we verify that only SIGTERM/SIGKILL are sent.
// All other signals are not supported on downlevel platforms.
func ValidateSigstrLCOW(sigstr string, signalsSupported bool) (*guestrequest.SignalProcessOptionsLCOW, error) {
	// All flavors including legacy default to SIGTERM on LCOW CtrlC on Windows
	if sigstr == "" {
		if signalsSupported {
			return &guestrequest.SignalProcessOptionsLCOW{Signal: sigTerm}, nil
		}
		return nil, nil
	}

	signal, err := strconv.Atoi(sigstr)
	if err == nil {
		return ValidateLCOW(signal, signalsSupported)
	}

	sigstr = strings.ToUpper(sigstr)
	if !signalsSupported {
		// If signals arent supported we just validate that its a known signal.
		// We already return 0 since we only supported a platform Kill() at that
		// time.
		switch sigstr {
		case "TERM", "KILL":
			return nil, nil
		default:
			return nil, ErrInvalidSignal
		}
	}

	// Match signal string name
	for k, v := range signalMapLcow {
		if sigstr == k {
			return &guestrequest.SignalProcessOptionsLCOW{Signal: v}, nil
		}
	}
	return nil, ErrInvalidSignal
}

// ValidateSigstrWCOW validates that `sigstr` is an acceptable signal for WCOW
// based on `signalsSupported`.
//
// `sigstr` may either be the text name or integer value of the signal.
//
// If `signalsSupported==false` we verify that only SIGTERM/SIGKILL and
// CTRLSHUTDOWN are sent. All other signals are not supported on downlevel
// platforms.
//
// By default WCOW orchestrators may still use Linux SIGTERM and SIGKILL
// semantics which will be properly translated to CTRLSHUTDOWN and `Terminate`.
// To detect when WCOW needs to `Terminate` the return signal will be `nil` and
// the return error will be `nil`.
func ValidateSigstrWCOW(sigstr string, signalsSupported bool) (*guestrequest.SignalProcessOptionsWCOW, error) {
	// All flavors including legacy default to SIGTERM on LCOW CtrlC on Windows
	if sigstr == "" {
		if signalsSupported {
			return &guestrequest.SignalProcessOptionsWCOW{Signal: guestrequest.SignalValueWCOWCtrlShutdown}, nil
		}
		return nil, nil
	}

	signal, err := strconv.Atoi(sigstr)
	if err == nil {
		return ValidateWCOW(signal, signalsSupported)
	}

	sigstr = strings.ToUpper(sigstr)
	if !signalsSupported {
		// If signals arent supported we just validate that its a known signal.
		// We already return 0 since we only supported a platform Kill() at that
		// time.
		switch sigstr {
		// Docker sends a UNIX term in the supported Windows Signal map.
		case "CTRLSHUTDOWN", "TERM", "KILL":
			return nil, nil
		default:
			return nil, ErrInvalidSignal
		}
	} else {
		// Docker sends the UNIX signal name or value. Convert them to the
		// correct Windows signals.

		var signalString guestrequest.SignalValueWCOW
		switch sigstr {
		case "CTRLC":
			signalString = guestrequest.SignalValueWCOWCtrlC
		case "CTRLBREAK":
			signalString = guestrequest.SignalValueWCOWCtrlBreak
		case "CTRLCLOSE":
			signalString = guestrequest.SignalValueWCOWCtrlClose
		case "CTRLLOGOFF":
			signalString = guestrequest.SignalValueWCOWCtrlLogOff
		case "CTRLSHUTDOWN", "TERM":
			// SIGTERM is most like CtrlShutdown on Windows convert it here.
			signalString = guestrequest.SignalValueWCOWCtrlShutdown
		case "KILL":
			return nil, nil
		default:
			return nil, ErrInvalidSignal
		}

		return &guestrequest.SignalProcessOptionsWCOW{Signal: signalString}, nil
	}
}

// ValidateLCOW validates that `signal` is an acceptable signal for LCOW based
// on `signalsSupported`.
//
// If `signalsSupported==false` we verify that only SIGTERM/SIGKILL are sent.
// All other signals are not supported on downlevel platforms.
func ValidateLCOW(signal int, signalsSupported bool) (*guestrequest.SignalProcessOptionsLCOW, error) {
	if !signalsSupported {
		// If signals arent supported we just validate that its a known signal.
		// We already return 0 since we only supported a platform Kill() at that
		// time.
		switch signal {
		case ctrlShutdown, sigTerm, sigKill:
			return nil, nil
		default:
			return nil, ErrInvalidSignal
		}
	}

	// Match signal by value
	for _, v := range signalMapLcow {
		if signal == v {
			return &guestrequest.SignalProcessOptionsLCOW{Signal: signal}, nil
		}
	}
	return nil, ErrInvalidSignal
}

// ValidateWCOW validates that `signal` is an acceptable signal for WCOW based
// on `signalsSupported`.
//
// If `signalsSupported==false` we verify that only SIGTERM/SIGKILL and
// CTRLSHUTDOWN are sent. All other signals are not supported on downlevel
// platforms.
//
// By default WCOW orchestrators may still use Linux SIGTERM and SIGKILL
// semantics which will be properly translated to CTRLSHUTDOWN and `Terminate`.
// To detect when WCOW needs to `Terminate` the return signal will be `nil` and
// the return error will be `nil`.
func ValidateWCOW(signal int, signalsSupported bool) (*guestrequest.SignalProcessOptionsWCOW, error) {
	if !signalsSupported {
		// If signals arent supported we just validate that its a known signal.
		// We already return 0 since we only supported a platform Kill() at that
		// time.
		switch signal {
		// Docker sends a UNIX term in the supported Windows Signal map.
		case ctrlShutdown, sigTerm, sigKill:
			return nil, nil
		default:
			return nil, ErrInvalidSignal
		}
	} else {
		// Docker sends the UNIX signal name or value. Convert them to the
		// correct Windows signals.

		var signalString guestrequest.SignalValueWCOW
		switch signal {
		case ctrlC:
			signalString = guestrequest.SignalValueWCOWCtrlC
		case ctrlBreak:
			signalString = guestrequest.SignalValueWCOWCtrlBreak
		case ctrlClose:
			signalString = guestrequest.SignalValueWCOWCtrlClose
		case ctrlLogOff:
			signalString = guestrequest.SignalValueWCOWCtrlLogOff
		case ctrlShutdown, sigTerm:
			// sigTerm is most like CtrlShutdown on Windows convert it here.
			signalString = guestrequest.SignalValueWCOWCtrlShutdown
		case sigKill:
			return nil, nil
		default:
			return nil, ErrInvalidSignal
		}

		return &guestrequest.SignalProcessOptionsWCOW{Signal: signalString}, nil
	}
}
