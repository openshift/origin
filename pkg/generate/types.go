package generate

import (
	"fmt"
	"strings"

	"github.com/golang/glog"
)

type Tester interface {
	Has(dir string) (string, bool, error)
}

type Strategy int

const (
	StrategyUnspecified Strategy = iota
	StrategySource
	StrategyDocker
	StrategyPipeline
)

func (s Strategy) String() string {
	switch s {
	case StrategyUnspecified:
		return ""
	case StrategySource:
		return "source"
	case StrategyDocker:
		return "Docker"
	case StrategyPipeline:
		return "pipeline"
	}
	glog.Error("unknown strategy")
	return ""
}

func (s Strategy) Type() string {
	return "strategy"
}

func (s *Strategy) Set(str string) error {
	switch strings.ToLower(str) {
	case "":
		*s = StrategyUnspecified
	case "docker":
		*s = StrategyDocker
	case "pipeline":
		*s = StrategyPipeline
	case "source":
		*s = StrategySource
	default:
		return fmt.Errorf("invalid strategy: %s. Must be 'docker', 'pipeline' or 'source'.", str)
	}
	return nil
}
