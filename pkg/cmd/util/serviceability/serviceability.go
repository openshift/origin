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

type stopper struct {
	profile bool
}

func (stopper) Stop() {
}

func Profile(mode string) Stop {
	var stop Stop
	switch mode {
	case "mem":
		stop = profileOnExit(profile.Start(profile.MemProfile, profile.ProfilePath("."), profile.NoShutdownHook))
	case "cpu":
		stop = profileOnExit(profile.Start(profile.CPUProfile, profile.ProfilePath("."), profile.NoShutdownHook))
	case "block":
		stop = profileOnExit(profile.Start(profile.BlockProfile, profile.ProfilePath("."), profile.NoShutdownHook))
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

		s.Stop()

		os.Exit(1)
	}()
	return s
}
