package serviceability

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/pkg/profile"
)

type Stop interface {
	Stop()
}

type stopper struct{}

func (stopper) Stop() {}

func Profile(mode string) Stop {
	var stop Stop
	switch mode {
	case "mem":
		stop = profileOnExit(profile.Start(profile.MemProfile, profile.ProfilePath("."), profile.NoShutdownHook, profile.Quiet))
	case "cpu":
		stop = profileOnExit(profile.Start(profile.CPUProfile, profile.ProfilePath("."), profile.NoShutdownHook, profile.Quiet))
	case "block":
		stop = profileOnExit(profile.Start(profile.BlockProfile, profile.ProfilePath("."), profile.NoShutdownHook, profile.Quiet))
	default:
		stop = stopper{}
	}
	return stop
}

func profileOnExit(s Stop) Stop {
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		<-c
		// Programs with more sophisticated signal handling
		// should ensure the Stop() function returned from
		// Start() is called during shutdown.
		// See http://godoc.org/github.com/pkg/profile
		s.Stop()

		os.Exit(1)
	}()
	return s
}
