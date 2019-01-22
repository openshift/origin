package app

import (
	"flag"
	"time"

	"github.com/mesos/mesos-go/api/v1/cmd"
	"github.com/mesos/mesos-go/api/v1/lib/encoding/codecs"
)

type Config struct {
	user                string
	name                string
	role                string
	url                 string
	codec               codec
	timeout             time.Duration
	failoverTimeout     time.Duration
	checkpoint          bool
	principal           string
	hostname            string
	labels              Labels
	server              server
	executor            string
	tasks               int
	verbose             bool
	taskCPU             float64
	taskMemory          float64
	execCPU             float64
	execMemory          float64
	reviveBurst         int
	reviveWait          time.Duration
	metrics             metrics
	resourceTypeMetrics bool
	maxRefuseSeconds    time.Duration
	jobRestartDelay     time.Duration
	summaryMetrics      bool
	execImage           string
	compression         bool
	credentials         credentials
	authMode            string
	gpuClusterCompat    bool
}

func (cfg *Config) AddFlags(fs *flag.FlagSet) {
	fs.StringVar(&cfg.user, "user", cfg.user, "Framework user to register with the Mesos master")
	fs.StringVar(&cfg.name, "name", cfg.name, "Framework name to register with the Mesos master")
	fs.StringVar(&cfg.role, "role", cfg.role, "Framework role to register with the Mesos master")
	fs.Var(&cfg.codec, "codec", "Codec to encode/decode scheduler API communications [protobuf, json]")
	fs.StringVar(&cfg.url, "url", cfg.url, "Mesos scheduler API URL")
	fs.DurationVar(&cfg.timeout, "timeout", cfg.timeout, "Mesos scheduler API connection timeout")
	fs.DurationVar(&cfg.failoverTimeout, "failoverTimeout", cfg.failoverTimeout, "Framework failover timeout (recover from scheduler failure)")
	fs.BoolVar(&cfg.checkpoint, "checkpoint", cfg.checkpoint, "Enable/disable agent checkpointing for framework tasks (recover from agent failure)")
	fs.StringVar(&cfg.principal, "principal", cfg.principal, "Framework principal with which to authenticate")
	fs.StringVar(&cfg.hostname, "hostname", cfg.hostname, "Framework hostname that is advertised to the master")
	fs.Var(&cfg.labels, "label", "Framework label, may be specified multiple times")
	fs.StringVar(&cfg.server.address, "server.address", cfg.server.address, "IP of artifact server")
	fs.IntVar(&cfg.server.port, "server.port", cfg.server.port, "Port of artifact server")
	fs.StringVar(&cfg.executor, "executor", cfg.executor, "Full path to executor binary")
	fs.IntVar(&cfg.tasks, "tasks", cfg.tasks, "Number of tasks to spawn")
	fs.BoolVar(&cfg.verbose, "verbose", cfg.verbose, "Verbose logging")
	fs.Float64Var(&cfg.taskCPU, "cpu", cfg.taskCPU, "CPU resources to consume per-task")
	fs.Float64Var(&cfg.taskMemory, "memory", cfg.taskMemory, "Memory resources (MB) to consume per-task")
	fs.Float64Var(&cfg.execCPU, "exec.cpu", cfg.execCPU, "CPU resources to consume per-executor")
	fs.Float64Var(&cfg.execMemory, "exec.memory", cfg.execMemory, "Memory resources (MB) to consume per-executor")
	fs.IntVar(&cfg.reviveBurst, "revive.burst", cfg.reviveBurst, "Number of revive messages that may be sent in a burst within revive-wait period")
	fs.DurationVar(&cfg.reviveWait, "revive.wait", cfg.reviveWait, "Wait this long to fully recharge revive-burst quota")
	fs.IntVar(&cfg.metrics.port, "metrics.port", cfg.metrics.port, "Port of metrics server (listens on server.address)")
	fs.StringVar(&cfg.metrics.path, "metrics.path", cfg.metrics.path, "URI path to metrics endpoint")
	fs.BoolVar(&cfg.resourceTypeMetrics, "resourceTypeMetrics", cfg.resourceTypeMetrics, "Collect scalar resource metrics per-type")
	fs.DurationVar(&cfg.maxRefuseSeconds, "maxRefuseSeconds", cfg.maxRefuseSeconds, "Max length of time to refuse future offers")
	fs.DurationVar(&cfg.jobRestartDelay, "jobRestartDelay", cfg.jobRestartDelay, "Duration between job (internal service) restarts between failures")
	fs.BoolVar(&cfg.summaryMetrics, "summaryMetrics", cfg.summaryMetrics, "Collect summary metrics for tasks launched per-offer-cycle, offer processing time, etc.")
	fs.StringVar(&cfg.execImage, "exec.image", cfg.execImage, "Name of the docker image to run the executor")
	fs.BoolVar(&cfg.compression, "compression", cfg.compression, "When true attempt to use compression for HTTP streams.")
	fs.StringVar(&cfg.credentials.username, "credentials.username", cfg.credentials.username, "Username for Mesos authentication")
	fs.StringVar(&cfg.credentials.password, "credentials.passwordFile", cfg.credentials.password, "Path to file that contains the password for Mesos authentication")
	fs.StringVar(&cfg.authMode, "authmode", cfg.authMode, "Method to use for Mesos authentication; specify '"+AuthModeBasic+"' for simple HTTP authentication")
	fs.BoolVar(&cfg.gpuClusterCompat, "gpuClusterCompat", cfg.gpuClusterCompat, "When true the framework will receive offers from agents w/ GPU resources.")
}

const AuthModeBasic = "basic"

func NewConfig() Config {
	return Config{
		user:             env("FRAMEWORK_USER", "root"),
		name:             env("FRAMEWORK_NAME", "example"),
		role:             env("FRAMEWORK_ROLE", "*"),
		url:              env("MESOS_MASTER_HTTP", "http://:5050/api/v1/scheduler"),
		codec:            codec{Codec: codecs.ByMediaType[codecs.MediaTypeProtobuf]},
		timeout:          envDuration("MESOS_CONNECT_TIMEOUT", "20s"),
		failoverTimeout:  envDuration("SCHEDULER_FAILOVER_TIMEOUT", "1000h"),
		checkpoint:       true,
		server:           server{address: env("LIBPROCESS_IP", "127.0.0.1")},
		tasks:            envInt("NUM_TASKS", "5"),
		taskCPU:          envFloat("TASK_CPU", "1"),
		taskMemory:       envFloat("TASK_MEMORY", "64"),
		execCPU:          envFloat("EXEC_CPU", "0.01"),
		execMemory:       envFloat("EXEC_MEMORY", "64"),
		reviveBurst:      envInt("REVIVE_BURST", "3"),
		reviveWait:       envDuration("REVIVE_WAIT", "1s"),
		maxRefuseSeconds: envDuration("MAX_REFUSE_SECONDS", "5s"),
		jobRestartDelay:  envDuration("JOB_RESTART_DELAY", "5s"),
		execImage:        env("EXEC_IMAGE", cmd.DockerImageTag),
		executor:         env("EXEC_BINARY", "/opt/example-executor"),
		metrics: metrics{
			port: envInt("PORT0", "64009"),
			path: env("METRICS_API_PATH", "/metrics"),
		},
		credentials: credentials{
			username: env("AUTH_USER", ""),
			password: env("AUTH_PASSWORD_FILE", ""),
		},
		authMode: env("AUTH_MODE", ""),
	}
}

type server struct {
	address string
	port    int
}

type metrics struct {
	port int
	path string
}

type credentials struct {
	username string
	password string
}
