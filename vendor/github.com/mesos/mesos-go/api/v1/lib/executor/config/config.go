package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	mesostime "github.com/mesos/mesos-go/api/v1/lib/time"
)

type Config struct {
	// FrameworkID of the scheduler needed as part of the SUBSCRIBE call
	FrameworkID string
	// ExecutorID of the executor needed as part of the SUBSCRIBE call
	ExecutorID string
	// Directory is the path to the working directory for the executor on the host filesystem (deprecated).
	Directory string
	// Sandbox is the path to the mapped sandbox inside of the container (determined by the agent flag
	// `sandbox_directory`) for either mesos container with image or docker container. For the case of
	// command task without image specified, it is the path to the sandbox on the host filesystem, which is
	// identical to `MESOS_DIRECTORY`. `MESOS_DIRECTORY` is always the sandbox on the host filesystem.
	Sandbox string
	// AgentEndpoint is the endpoint i.e. ip:port to be used by the executor to connect
	// to the agent
	AgentEndpoint string
	// ExecutorShutdownGracePeriod is the amount of time the agent would wait for an
	// executor to shut down (e.g., 60 secs, 3mins etc.) after sending a SHUTDOWN event
	ExecutorShutdownGracePeriod time.Duration

	// Checkpoint is set i.e. when framework checkpointing is enabled; if set then
	// RecoveryTimeout and SubscriptionBackoffMax are also set.
	Checkpoint bool
	// The total duration that the executor should spend retrying before shutting itself
	// down when it is disconnected from the agent (e.g., 15mins, 5secs etc.)
	RecoveryTimeout time.Duration
	// The maximum backoff duration to be used by the executor between two retries when
	// disconnected (e.g., 250ms, 1mins etc.)
	SubscriptionBackoffMax time.Duration
}

type EnvError struct {
	Reasons []error
	Message string
}

func (ee *EnvError) Error() string { return fmt.Sprintf("%s: %v", ee.Message, ee.Reasons) }

// FromEnv returns a configuration generated from MESOS_xyz environment variables.
func FromEnv() (Config, error) { return fromEnv(os.Getenv) }

func fromEnv(getter func(string) string) (Config, error) {
	ee := EnvError{Message: "illegal configuration in process environment"}
	required := func(name string) string {
		value := getter(name)
		if value == "" {
			ee.Reasons = append(ee.Reasons, errors.New("missing environment variable: "+name))
		}
		return value
	}
	requiredDuration := func(name string) time.Duration {
		stringValue := required(name)
		if stringValue != "" {
			d, err := mesostime.ParseDuration(stringValue)
			if err != nil {
				ee.Reasons = append(ee.Reasons, err)
			} else {
				return d
			}
		}
		return 0
	}
	c := Config{
		FrameworkID:                 required("MESOS_FRAMEWORK_ID"),
		ExecutorID:                  required("MESOS_EXECUTOR_ID"),
		Directory:                   required("MESOS_DIRECTORY"),
		Sandbox:                     required("MESOS_SANDBOX"),
		AgentEndpoint:               required("MESOS_AGENT_ENDPOINT"),
		ExecutorShutdownGracePeriod: requiredDuration("MESOS_EXECUTOR_SHUTDOWN_GRACE_PERIOD"),
	}
	checkpoint, err := envBool("MESOS_CHECKPOINT", getter)
	if err != nil {
		ee.Reasons = append(ee.Reasons, err)
	}
	if checkpoint {
		c.Checkpoint = true
		c.RecoveryTimeout = requiredDuration("MESOS_RECOVERY_TIMEOUT")
		c.SubscriptionBackoffMax = requiredDuration("MESOS_SUBSCRIPTION_BACKOFF_MAX")
	}
	if len(ee.Reasons) == 0 {
		return c, nil
	}
	return Config{}, &ee
}

func envBool(name string, getter func(string) string) (b bool, err error) {
	value := getter(name)
	if value != "" {
		b, err = strconv.ParseBool(value)
	}
	return
}
