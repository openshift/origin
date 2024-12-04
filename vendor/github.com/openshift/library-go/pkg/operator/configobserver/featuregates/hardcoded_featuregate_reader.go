package featuregates

import (
	"context"
	"fmt"

	configv1 "github.com/openshift/api/config/v1"
)

type hardcodedFeatureGateAccess struct {
	enabled  []configv1.FeatureGateName
	disabled []configv1.FeatureGateName
	readErr  error

	initialFeatureGatesObserved chan struct{}
}

// NewHardcodedFeatureGateAccess returns a FeatureGateAccess that is always initialized and always
// returns the provided feature gates.
func NewHardcodedFeatureGateAccess(enabled, disabled []configv1.FeatureGateName) FeatureGateAccess {
	initialFeatureGatesObserved := make(chan struct{})
	close(initialFeatureGatesObserved)
	c := &hardcodedFeatureGateAccess{
		enabled:                     enabled,
		disabled:                    disabled,
		initialFeatureGatesObserved: initialFeatureGatesObserved,
	}

	return c
}

// NewHardcodedFeatureGateAccessForTesting returns a FeatureGateAccess that returns stub responses
// using caller-supplied values.
func NewHardcodedFeatureGateAccessForTesting(enabled, disabled []configv1.FeatureGateName, initialFeatureGatesObserved chan struct{}, readErr error) FeatureGateAccess {
	return &hardcodedFeatureGateAccess{
		enabled:                     enabled,
		disabled:                    disabled,
		initialFeatureGatesObserved: initialFeatureGatesObserved,
		readErr:                     readErr,
	}
}

func (c *hardcodedFeatureGateAccess) SetChangeHandler(featureGateChangeHandlerFn FeatureGateChangeHandlerFunc) {
	// ignore
}

func (c *hardcodedFeatureGateAccess) Run(ctx context.Context) {
	// ignore
}

func (c *hardcodedFeatureGateAccess) InitialFeatureGatesObserved() <-chan struct{} {
	return c.initialFeatureGatesObserved
}

func (c *hardcodedFeatureGateAccess) AreInitialFeatureGatesObserved() bool {
	select {
	case <-c.InitialFeatureGatesObserved():
		return true
	default:
		return false
	}
}

func (c *hardcodedFeatureGateAccess) CurrentFeatureGates() (FeatureGate, error) {
	return NewFeatureGate(c.enabled, c.disabled), c.readErr
}

// NewHardcodedFeatureGateAccessFromFeatureGate returns a FeatureGateAccess that is static and initialised from
// a populated FeatureGate status.
// If the desired version is missing, this will return an error.
func NewHardcodedFeatureGateAccessFromFeatureGate(featureGate *configv1.FeatureGate, desiredVersion string) (FeatureGateAccess, error) {
	features, err := featuresFromFeatureGate(featureGate, desiredVersion)
	if err != nil {
		return nil, fmt.Errorf("unable to determine features: %w", err)
	}

	return NewHardcodedFeatureGateAccess(features.Enabled, features.Disabled), nil
}
