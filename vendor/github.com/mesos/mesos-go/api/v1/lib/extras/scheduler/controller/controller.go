package controller

import (
	"context"

	"github.com/mesos/mesos-go/api/v1/lib"
	"github.com/mesos/mesos-go/api/v1/lib/encoding"
	"github.com/mesos/mesos-go/api/v1/lib/scheduler"
	"github.com/mesos/mesos-go/api/v1/lib/scheduler/calls"
	"github.com/mesos/mesos-go/api/v1/lib/scheduler/events"
)

type (
	// Option modifies a Config, returns an Option that acts as an "undo"
	Option func(*Config) Option

	// Config is an opaque controller configuration. Properties are configured by applying Option funcs.
	Config struct {
		frameworkIDFunc        func() string
		handler                events.Handler
		registrationTokens     <-chan struct{}
		subscriptionTerminated func(error)
		initSuppressRoles      []string
	}
)

// WithInitiallySuppressedRoles sets the "suppressed_roles" field of the SUBSCRIBE call
// that's issued to Mesos for each (re-)subscription attempt.
func WithInitiallySuppressedRoles(r []string) Option {
	return func(c *Config) Option {
		old := c.initSuppressRoles
		c.initSuppressRoles = r
		return WithInitiallySuppressedRoles(old)
	}
}

// WithEventHandler sets the consumer of scheduler events. The controller's internal event processing
// loop is aborted if a Handler returns a non-nil error, after which the controller may attempt
// to re-register (subscribe) with Mesos.
func WithEventHandler(handler events.Handler) Option {
	return func(c *Config) Option {
		old := c.handler
		c.handler = handler
		return WithEventHandler(old)
	}
}

// WithFrameworkID sets a fetcher for the current Mesos-assigned framework ID. Frameworks are expected to
// track this ID (that comes from Mesos, in a SUBSCRIBED event).
// frameworkIDFunc is optional; nil tells the controller to always register as a new framework
// for each subscription attempt.
func WithFrameworkID(frameworkIDFunc func() string) Option {
	return func(c *Config) Option {
		old := c.frameworkIDFunc
		c.frameworkIDFunc = frameworkIDFunc
		return WithFrameworkID(old)
	}
}

// WithSubscriptionTerminated sets a handler that is invoked at the end of every subscription cycle; the
// given error may be nil if no error occurred. subscriptionTerminated is optional; if nil then errors are
// swallowed.
func WithSubscriptionTerminated(handler func(error)) Option {
	return func(c *Config) Option {
		old := c.subscriptionTerminated
		c.subscriptionTerminated = handler
		return WithSubscriptionTerminated(old)
	}
}

// WithRegistrationTokens limits the rate at which a framework (re)registers with Mesos.
// A non-nil chan should yield a struct{} in order to allow the framework registration process to continue.
// When nil, there is no backoff delay between re-subscription attempts.
// A closed chan disables re-registration and terminates the Run control loop.
func WithRegistrationTokens(registrationTokens <-chan struct{}) Option {
	return func(c *Config) Option {
		old := c.registrationTokens
		c.registrationTokens = registrationTokens
		return WithRegistrationTokens(old)
	}
}

func (c *Config) tryFrameworkID() (result string) {
	if c.frameworkIDFunc != nil {
		result = c.frameworkIDFunc()
	}
	return
}

func isDone(ctx context.Context) (result bool) {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}

// Run executes a control loop that registers a framework with Mesos and processes the scheduler events
// that flow through the subscription. Upon disconnection, if the current configuration reports "not done"
// then the controller will attempt to re-register the framework and continue processing events.
func Run(ctx context.Context, framework *mesos.FrameworkInfo, caller calls.Caller, options ...Option) (lastErr error) {
	var config Config
	for _, opt := range options {
		if opt != nil {
			opt(&config)
		}
	}
	if config.handler == nil {
		config.handler = DefaultHandler
	}
	subscribe := calls.Subscribe(framework)
	subscribe.Subscribe.SuppressedRoles = config.initSuppressRoles
	for !isDone(ctx) {
		frameworkID := config.tryFrameworkID()
		if framework.GetFailoverTimeout() > 0 && frameworkID != "" {
			subscribe.With(calls.SubscribeTo(frameworkID))
		}
		if config.registrationTokens != nil {
			select {
			case _, ok := <-config.registrationTokens:
				if !ok {
					// re-registration canceled, exit Run loop
					return
				}
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		resp, err := caller.Call(ctx, subscribe)
		lastErr = processSubscription(ctx, config, resp, err)
		if config.subscriptionTerminated != nil {
			config.subscriptionTerminated(lastErr)
		}
	}
	return
}

func processSubscription(ctx context.Context, config Config, resp mesos.Response, err error) error {
	if resp != nil {
		defer resp.Close()
	}
	if err == nil {
		err = eventLoop(ctx, config, resp)
	}
	return err
}

// eventLoop returns the framework ID received by mesos (if any); callers should check for a
// framework ID regardless of whether error != nil.
func eventLoop(ctx context.Context, config Config, eventDecoder encoding.Decoder) (err error) {
	for err == nil && !isDone(ctx) {
		var e scheduler.Event
		if err = eventDecoder.Decode(&e); err == nil {
			err = config.handler.HandleEvent(ctx, &e)
		}
	}
	return err
}

// DefaultHandler is invoked when no other handlers have been defined for the controller.
// The current implementation does nothing.
// TODO(jdef) a smarter default impl would decline all offers so as to avoid resource hoarding.
const DefaultHandler = events.NoopHandler
