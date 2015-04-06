package lifecycle

import (
	"fmt"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
)

// Status represents the status of a lifecycle action, and should be treated
// as the source of truth by controllers.
type Status string

const (
	// Pending means the action has not yet executed.
	Pending Status = "Pending"
	// Running means the action is currently executing.
	Running Status = "Running"
	// Complete means the action completed successfully, taking into account
	// failure policy.
	Complete Status = "Complete"
	// Failed means the action failed to execute, taking into account failure
	// policy.
	Failed Status = "Failed"
)

// Context informs the manager which Lifecycle point is being handled.
type Context string

const (
	// Pre refers to Lifecycle.Pre
	Pre Context = "Pre"
	// Post refers to Lifecycle.Post
	Post Context = "Post"
)

// Interface provides deployment controllers with a way to execute and track
// lifecycle actions.
//
// This interface abstracts action policy handling; users should assume
// Complete and Failed are terminal states, and that any request to retry has
// already been accounted for. Users should not attempt to retry Failed
// actions.
//
// Users should not be concerned with whether a given lifecycle action is
// actually defined on a deployment; calls to execute non-existent actions
// will no-op, and status for non-existent actions will appear to be Complete.
type Interface interface {
	// Execute executes the deployment lifecycle action for the given context.
	// If no action is defined, Execute should return nil.
	Execute(context Context, deployment *kapi.ReplicationController) error
	// Status returns the status of the lifecycle action for the deployment. If
	// no action is defined for the given context, Status returns Complete. If
	// the action finished, either Complete or Failed is returned depending on
	// the failure policy associated with the action (for example, if the action
	// failed but the policy is set to ignore failures, Complete is returned
	// instead of Failed).
	//
	// If the status couldn't be determined, an error is returned.
	Status(context Context, deployment *kapi.ReplicationController) (Status, error)
}

// Plugin knows how to execute lifecycle handlers and report their status.
//
// Plugins are expected to report actual status, NOT policy based status.
type Plugin interface {
	// CanHandle should return true if the plugin knows how to execute handler.
	CanHandle(handler *deployapi.Handler) bool
	// Execute executes handler in the given context for deployment.
	Execute(context Context, handler *deployapi.Handler, deployment *kapi.ReplicationController, config *deployapi.DeploymentConfig) error
	// Status should report the actual status of the action without taking into
	// account failure policies.
	Status(context Context, handler *deployapi.Handler, deployment *kapi.ReplicationController) Status
}

// LifecycleManager implements a pluggable lifecycle.Interface which handles
// the high level details of lifecyle action execution such as decoding
// DeploymentConfigs and implementing the lifecycle.Interface contract for
// policy based status reporting using the actual status returned from
// plugins.
type LifecycleManager struct {
	// Plugins execute specific handler instances.
	Plugins []Plugin
	// DecodeConfig knows how to decode the deploymentConfig from a deployment's annotations.
	DecodeConfig func(deployment *kapi.ReplicationController) (*deployapi.DeploymentConfig, error)
}

var _ Interface = &LifecycleManager{}

// Execute implements Interface.
func (m *LifecycleManager) Execute(context Context, deployment *kapi.ReplicationController) error {
	// Decode the config
	config, err := m.DecodeConfig(deployment)
	if err != nil {
		return err
	}

	// If there's no handler, no-op
	handler := handlerFor(context, config)
	if handler == nil {
		return nil
	}

	plugin, err := m.pluginFor(handler)
	if err != nil {
		return err
	}

	return plugin.Execute(context, handler, deployment, config)
}

// Status implements Interface.
func (m *LifecycleManager) Status(context Context, deployment *kapi.ReplicationController) (Status, error) {
	// Decode the config
	config, err := m.DecodeConfig(deployment)
	if err != nil {
		return "", nil
	}

	handler := handlerFor(context, config)
	if handler == nil {
		return Complete, nil
	}

	plugin, err := m.pluginFor(handler)
	if err != nil {
		return "", err
	}

	status := plugin.Status(context, handler, deployment)
	if status == Failed && handler.FailurePolicy == deployapi.IgnoreHandlerFailurePolicy {
		status = Complete
	}
	return status, nil
}

// pluginFor finds a plugin which knows how to deal with handler.
func (m *LifecycleManager) pluginFor(handler *deployapi.Handler) (Plugin, error) {
	for _, plugin := range m.Plugins {
		if plugin.CanHandle(handler) {
			return plugin, nil
		}
	}

	return nil, fmt.Errorf("no plugin registered for handler: %#v", handler)
}

// handlerFor finds any handler in config for the given context.
func handlerFor(context Context, config *deployapi.DeploymentConfig) *deployapi.Handler {
	if config.Template.Strategy.Lifecycle == nil {
		return nil
	}

	// Find any right handler given the context
	var handler *deployapi.Handler
	switch context {
	case Pre:
		handler = config.Template.Strategy.Lifecycle.Pre
	case Post:
		handler = config.Template.Strategy.Lifecycle.Post
	}
	return handler
}
