package admin

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/util/sets"
)

const (
	PhaseKey     = "createkey"
	PhaseCSR     = "createcsr"
	PhaseSign    = "signcsr"
	PhaseVerify  = "verify"
	PhasePackage = "package"
)

func BindPhaseOptions(phases *[]string, flags *pflag.FlagSet, prefix string) {
	flags.StringSliceVar(phases, prefix+"phases", *phases, "Comma delimited list of "+strings.Join(AllPhases, ","))
}

func NewDefaultPhaseOptions() []string {
	return AllPhases
}

func ValidatePhases(phases []string) error {
	if len(phases) == 0 {
		return errors.New("at least one phase must be provided")
	}
	if !ValidPhases.HasAll(phases...) {
		return fmt.Errorf("invalid phases: %v", sets.NewString(phases...).Difference(ValidPhases).List())
	}
	return nil
}

var (
	AllPhases   = []string{PhaseKey, PhaseCSR, PhaseSign, PhaseVerify, PhasePackage}
	ValidPhases = sets.NewString(AllPhases...)
)

func hasPhase(phase string, phases []string) bool {
	return sets.NewString(phases...).Has(phase)
}
