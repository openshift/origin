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
}

// sets the klog level based on desired state
func NewClusterOperatorLoggingController(operatorClient operatorv1helpers.OperatorClient, recorder events.Recorder) factory.Controller {
	c := &LogLevelController{operatorClient: operatorClient}
	return factory.New().WithInformers(operatorClient.Informer()).WithSync(c.sync).ToController("LoggingSyncer", recorder)
}

// sync reacts to a change in prereqs by finding information that is required to match another value in the cluster. This
// must be information that is logically "owned" by another component.
func (c LogLevelController) sync(ctx context.Context, syncCtx factory.SyncContext) error {
	detailedSpec, _, _, err := c.operatorClient.GetOperatorState()
	if err != nil {
		return err
	}

	currentLogLevel := CurrentLogLevel()
	desiredLogLevel := detailedSpec.OperatorLogLevel

	if len(desiredLogLevel) == 0 {
		desiredLogLevel = operatorv1.Normal
	}

	// When the current loglevel is the desired one, do nothing
	if currentLogLevel == desiredLogLevel {
		return nil
	}

	// Set the new loglevel if the operator spec changed
	if err := SetVerbosityValue(desiredLogLevel); err != nil {
		syncCtx.Recorder().Warningf("OperatorLoglevelChangeFailed", "Unable to change operator log level from %q to %q: %v", currentLogLevel, desiredLogLevel, err)
		return err
	}

	syncCtx.Recorder().Eventf("OperatorLoglevelChange", "Operator log level changed from %q to %q", currentLogLevel, desiredLogLevel)
	return nil
}
