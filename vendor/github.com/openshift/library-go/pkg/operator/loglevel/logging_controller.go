package loglevel

import (
	"context"

	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
	operatorv1helpers "github.com/openshift/library-go/pkg/operator/v1helpers"
)

type LogLevelController struct {
	operatorClient operatorv1helpers.OperatorClient

	// for unit tests only
	setLogLevelFn func(operatorv1.LogLevel) error
	getLogLevelFn func() (operatorv1.LogLevel, bool)
}

// sets the klog level based on desired state
func NewClusterOperatorLoggingController(operatorClient operatorv1helpers.OperatorClient, recorder events.Recorder) factory.Controller {
	c := &LogLevelController{
		operatorClient: operatorClient,
		setLogLevelFn:  SetLogLEvel,
		getLogLevelFn:  GetLogLevel,
	}
	return factory.New().WithInformers(operatorClient.Informer()).WithSync(c.sync).ToController("LoggingSyncer", recorder)
}

// sync reacts to a change in prereqs by finding information that is required to match another value in the cluster. This
// must be information that is logically "owned" by another component.
func (c LogLevelController) sync(ctx context.Context, syncCtx factory.SyncContext) error {
	detailedSpec, _, _, err := c.operatorClient.GetOperatorState()
	if err != nil {
		return err
	}

	currentLogLevel, isUnknown := c.getLogLevelFn()
	desiredLogLevel := detailedSpec.OperatorLogLevel

	// if operator operatorSpec OperatorLogLevel is empty, default to "Normal"
	// TODO: This should be probably done by defaulting the CR field?
	if len(desiredLogLevel) == 0 {
		desiredLogLevel = operatorv1.Normal
	}

	// correct log level is set and it matches the expected log level from operator operatorSpec, do nothing.
	if !isUnknown && currentLogLevel == desiredLogLevel {
		return nil
	}

	// log level is not specified in operatorSpec and the log verbosity is not set (0), default the log level to V(2).
	if len(desiredLogLevel) == 0 {
		desiredLogLevel = currentLogLevel
	}

	// Set the new loglevel if the operator operatorSpec changed
	if err := c.setLogLevelFn(desiredLogLevel); err != nil {
		syncCtx.Recorder().Warningf("OperatorLogLevelChangeFailed", "Unable to change operator log level from %q to %q: %v", currentLogLevel, desiredLogLevel, err)
		return err
	}

	// Do not fire event on every restart.
	if isUnknown {
		return nil
	}

	syncCtx.Recorder().Eventf("OperatorLogLevelChange", "Operator log level changed from %q to %q", currentLogLevel, desiredLogLevel)
	return nil
}
