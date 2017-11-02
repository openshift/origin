package configuration

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"reflect"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"gopkg.in/yaml.v2"

	//"github.com/docker/distribution/registry/auth"
	"github.com/docker/distribution/configuration"
)

// Environment variables.
const (
	// DockerRegistryURLEnvVar is a mandatory environment variable name specifying url of internal docker
	// registry. All references to pushed images will be prefixed with its value.
	// DEPRECATED: Use the REGISTRY_OPENSHIFT_SERVER_ADDR instead.
	DockerRegistryURLEnvVar = "DOCKER_REGISTRY_URL"

	// OpenShiftDockerRegistryURLEnvVar is an optional environment that overrides the
	// DOCKER_REGISTRY_URL.
	// DEPRECATED: Use the REGISTRY_OPENSHIFT_SERVER_ADDR instead.
	OpenShiftDockerRegistryURLEnvVar = "REGISTRY_MIDDLEWARE_REPOSITORY_OPENSHIFT_DOCKERREGISTRYURL"

	// OpenShiftDefaultRegistryEnvVar overrides the DockerRegistryURLEnvVar as in OpenShift the
	// default registry URL is controlled by this environment variable.
	// DEPRECATED: Use the REGISTRY_OPENSHIFT_SERVER_ADDR instead.
	OpenShiftDefaultRegistryEnvVar = "OPENSHIFT_DEFAULT_REGISTRY"

	// EnforceQuotaEnvVar is a boolean environment variable that allows to turn quota enforcement on or off.
	// By default, quota enforcement is off. It overrides openshift middleware configuration option.
	// Recognized values are "true" and "false".
	// DEPRECATED: Use the REGISTRY_OPENSHIFT_QUOTA_ENABLED instead.
	EnforceQuotaEnvVar = "REGISTRY_MIDDLEWARE_REPOSITORY_OPENSHIFT_ENFORCEQUOTA"

	// ProjectCacheTTLEnvVar is an environment variable specifying an eviction timeout for project quota
	// objects. It takes a valid time duration string (e.g. "2m"). If empty, you get the default timeout. If
	// zero (e.g. "0m"), caching is disabled.
	// DEPRECATED: Use the REGISTRY_OPENSHIFT_CACHE_QUOTATTL instead.
	ProjectCacheTTLEnvVar = "REGISTRY_MIDDLEWARE_REPOSITORY_OPENSHIFT_PROJECTCACHETTL"

	// AcceptSchema2EnvVar is a boolean environment variable that allows to accept manifest schema v2
	// on manifest put requests.
	// DEPRECATED: Use the REGISTRY_OPENSHIFT_COMPATIBILITY_ACCEPTSCHEMA2 instead.
	AcceptSchema2EnvVar = "REGISTRY_MIDDLEWARE_REPOSITORY_OPENSHIFT_ACCEPTSCHEMA2"

	// BlobRepositoryCacheTTLEnvVar  is an environment variable specifying an eviction timeout for <blob
	// belongs to repository> entries. The higher the value, the faster queries but also a higher risk of
	// leaking a blob that is no longer tagged in given repository.
	// DEPRECATED: Use the REGISTRY_OPENSHIFT_CACHE_BLOBREPOSITORYTTL instead.
	BlobRepositoryCacheTTLEnvVar = "REGISTRY_MIDDLEWARE_REPOSITORY_OPENSHIFT_BLOBREPOSITORYCACHETTL"

	// Pullthrough is a boolean environment variable that controls whether pullthrough is enabled.
	// DEPRECATED: Use the REGISTRY_OPENSHIFT_PULLTHROUGH_ENABLED instead.
	PullthroughEnvVar = "REGISTRY_MIDDLEWARE_REPOSITORY_OPENSHIFT_PULLTHROUGH"

	// MirrorPullthrough is a boolean environment variable that controls mirroring of blobs on pullthrough.
	// DEPRECATED: Use the REGISTRY_OPENSHIFT_PULLTHROUGH_MIRROR instead.
	MirrorPullthroughEnvVar = "REGISTRY_MIDDLEWARE_REPOSITORY_OPENSHIFT_MIRRORPULLTHROUGH"

	RealmKey      = "realm"
	TokenRealmKey = "tokenrealm"

	middlewareName = "openshift"

	// Default values
	defaultBlobRepositoryCacheTTL = time.Minute * 10
	defaultProjectCacheTTL        = time.Minute
)

var (
	// CurrentVersion is the most recent Version that can be parsed.
	CurrentVersion = configuration.MajorMinorVersion(1, 0)

	ErrUnsupportedVersion = errors.New("Unsupported openshift configuration version")
)

type openshiftConfig struct {
	Openshift Configuration
}

type Configuration struct {
	Version       configuration.Version `yaml:"version"`
	Metrics       Metrics               `yaml:"metrics"`
	Requests      Requests              `yaml:"requests"`
	Server        *Server               `yaml:"server"`
	Auth          *Auth                 `yaml:"auth"`
	Audit         *Audit                `yaml:"audit"`
	Cache         *Cache                `yaml:"cache"`
	Quota         *Quota                `yaml:"quota"`
	Pullthrough   *Pullthrough          `yaml:"pullthrough"`
	Compatibility *Compatibility        `yaml:"compatibility"`
}

type Metrics struct {
	Enabled bool   `yaml:"enabled"`
	Secret  string `yaml:"secret"`
}

type Requests struct {
	Read  RequestsLimits `yaml:"read"`
	Write RequestsLimits `yaml:"write"`
}

type RequestsLimits struct {
	MaxRunning     int           `yaml:"maxrunning"`
	MaxInQueue     int           `yaml:"maxinqueue"`
	MaxWaitInQueue time.Duration `yaml:"maxwaitinqueue"`
}

type Server struct {
	Addr string `yaml:"addr"`
}

type Auth struct {
	Realm      string `yaml:"realm"`
	TokenRealm string `yaml:"tokenrealm"`
}

type Audit struct {
	Enabled bool `yaml:"enabled"`
}

type Cache struct {
	BlobRepositoryTTL time.Duration `yaml:"blobrepositoryttl"`
	QuotaTTL          time.Duration `yaml:"quotattl"`
}

type Quota struct {
	Enabled bool `yaml:"enabled"`
}

type Pullthrough struct {
	Enabled bool `yaml:"enabled"`
	Mirror  bool `yaml:"mirror"`
}

type Compatibility struct {
	AcceptSchema2 bool `yaml:"acceptschema2"`
}

type versionInfo struct {
	Openshift struct {
		Version *configuration.Version
	}
}

// Parse parses an input configuration and returns docker configuration structure and
// openshift specific configuration.
// Environment variables may be used to override configuration parameters.
func Parse(rd io.Reader) (*configuration.Configuration, *Configuration, error) {
	in, err := ioutil.ReadAll(rd)
	if err != nil {
		return nil, nil, err
	}

	// We don't want to change the version from the environment variables.
	os.Unsetenv("REGISTRY_OPENSHIFT_VERSION")

	openshiftEnv, err := popEnv("REGISTRY_OPENSHIFT_")
	if err != nil {
		return nil, nil, err
	}

	dockerConfig, err := configuration.Parse(bytes.NewBuffer(in))
	if err != nil {
		return nil, nil, err
	}

	dockerEnv, err := popEnv("REGISTRY_")
	if err != nil {
		return nil, nil, err
	}
	if err := pushEnv(openshiftEnv); err != nil {
		return nil, nil, err
	}

	config := openshiftConfig{}

	vInfo := &versionInfo{}
	if err := yaml.Unmarshal(in, &vInfo); err != nil {
		return nil, nil, err
	}

	if vInfo.Openshift.Version != nil {
		if *vInfo.Openshift.Version != CurrentVersion {
			return nil, nil, ErrUnsupportedVersion
		}
	}

	p := configuration.NewParser("registry", []configuration.VersionedParseInfo{
		{
			Version: dockerConfig.Version,
			ParseAs: reflect.TypeOf(config),
			ConversionFunc: func(c interface{}) (interface{}, error) {
				return c, nil
			},
		},
	})

	if err = p.Parse(in, &config); err != nil {
		return nil, nil, err
	}
	if err := pushEnv(dockerEnv); err != nil {
		return nil, nil, err
	}

	if err := InitExtraConfig(dockerConfig, &config.Openshift); err != nil {
		return nil, nil, err
	}

	return dockerConfig, &config.Openshift, nil
}

type envVar struct {
	name  string
	value string
}

func popEnv(prefix string) ([]envVar, error) {
	var envVars []envVar

	for _, env := range os.Environ() {
		if !strings.HasPrefix(env, prefix) {
			continue
		}
		envParts := strings.SplitN(env, "=", 2)
		err := os.Unsetenv(envParts[0])
		if err != nil {
			return nil, err
		}

		envVars = append(envVars, envVar{envParts[0], envParts[1]})
	}

	return envVars, nil
}

func pushEnv(environ []envVar) error {
	for _, env := range environ {
		if err := os.Setenv(env.name, env.value); err != nil {
			return err
		}
	}
	return nil
}

func setDefaultMiddleware(config *configuration.Configuration) {
	// Default to openshift middleware for relevant types
	// This allows custom configs based on old default configs to continue to work
	if config.Middleware == nil {
		config.Middleware = map[string][]configuration.Middleware{}
	}
	for _, middlewareType := range []string{"registry", "repository", "storage"} {
		found := false
		for _, middleware := range config.Middleware[middlewareType] {
			if middleware.Name != middlewareName {
				continue
			}
			if middleware.Disabled {
				log.Errorf("wrong configuration detected, openshift %s middleware should not be disabled in the config file", middlewareType)
				middleware.Disabled = false
			}
			found = true
			break
		}
		if found {
			continue
		}
		config.Middleware[middlewareType] = append(config.Middleware[middlewareType], configuration.Middleware{
			Name: middlewareName,
		})
	}
	// TODO(legion) This check breaks the tests. Uncomment when the tests will be able to work with auth middleware.
	/*
		authType := config.Auth.Type()
		if authType != middlewareName {
			log.Errorf("wrong configuration detected, registry should use openshift auth controller: %v", authType)
			config.Auth = make(configuration.Auth)
			config.Auth[middlewareName] = make(configuration.Parameters)
		}
	*/
}

func getServerAddr(options configuration.Parameters) (registryAddr string, err error) {
	found := false

	if len(registryAddr) == 0 {
		registryAddr, found = os.LookupEnv(OpenShiftDefaultRegistryEnvVar)
		if found {
			log.Infof("DEPRECATED: %q is deprecated, use the 'REGISTRY_OPENSHIFT_SERVER_ADDR' instead", OpenShiftDefaultRegistryEnvVar)
		}
	}

	if len(registryAddr) == 0 {
		registryAddr, found = os.LookupEnv(DockerRegistryURLEnvVar)
		if found {
			log.Infof("DEPRECATED: %q is deprecated, use the 'REGISTRY_OPENSHIFT_SERVER_ADDR' instead", DockerRegistryURLEnvVar)
		}
	}

	if len(registryAddr) == 0 {
		// Legacy configuration
		registryAddr, err = getStringOption(OpenShiftDockerRegistryURLEnvVar, "dockerregistryurl", registryAddr, options)
		if err != nil {
			return
		}
	}

	// TODO: This is a fallback to assuming there is a service named 'docker-registry'. This
	// might change in the future and we should make this configurable.
	if len(registryAddr) == 0 {
		if len(os.Getenv("DOCKER_REGISTRY_SERVICE_HOST")) > 0 && len(os.Getenv("DOCKER_REGISTRY_SERVICE_PORT")) > 0 {
			registryAddr = os.Getenv("DOCKER_REGISTRY_SERVICE_HOST") + ":" + os.Getenv("DOCKER_REGISTRY_SERVICE_PORT")
		} else {
			err = fmt.Errorf("REGISTRY_OPENSHIFT_SERVER_ADDR variable must be set when running outside of Kubernetes cluster")
			return
		}
	}

	return
}

func migrateServerSection(cfg *Configuration, options configuration.Parameters) (err error) {
	if cfg.Server != nil {
		return
	}
	cfg.Server = &Server{}
	cfg.Server.Addr, err = getServerAddr(options)
	if err != nil {
		err = fmt.Errorf("configuration error in openshift.server.addr: %v", err)
	}
	return
}

func migrateQuotaSection(cfg *Configuration, options configuration.Parameters) (err error) {
	defEnabled := false

	if cfg.Quota != nil {
		options = configuration.Parameters{}
		defEnabled = cfg.Quota.Enabled
	} else {
		cfg.Quota = &Quota{}
	}

	cfg.Quota.Enabled, err = getBoolOption(EnforceQuotaEnvVar, "enforcequota", defEnabled, options)
	if err != nil {
		err = fmt.Errorf("configuration error in openshift.quota.enabled: %v", err)
	}
	return
}

func migrateCacheSection(cfg *Configuration, options configuration.Parameters) (err error) {
	defBlobRepositoryTTL := defaultBlobRepositoryCacheTTL
	defQuotaTTL := defaultProjectCacheTTL

	if cfg.Cache != nil {
		options = configuration.Parameters{}
		defBlobRepositoryTTL = cfg.Cache.BlobRepositoryTTL
		defQuotaTTL = cfg.Cache.QuotaTTL
	} else {
		cfg.Cache = &Cache{}
	}

	cfg.Cache.BlobRepositoryTTL, err = getDurationOption(BlobRepositoryCacheTTLEnvVar, "blobrepositorycachettl", defBlobRepositoryTTL, options)
	if err != nil {
		err = fmt.Errorf("configuration error in openshift.cache.blobrepositoryttl: %v", err)
		return
	}
	cfg.Cache.QuotaTTL, err = getDurationOption(ProjectCacheTTLEnvVar, "projectcachettl", defQuotaTTL, options)
	if err != nil {
		err = fmt.Errorf("configuration error in openshift.cache.quotattl: %v", err)
	}
	return
}

func migratePullthroughSection(cfg *Configuration, options configuration.Parameters) (err error) {
	defEnabled := true
	defMirror := true

	if cfg.Pullthrough != nil {
		options = configuration.Parameters{}
		defEnabled = cfg.Pullthrough.Enabled
		defMirror = cfg.Pullthrough.Mirror
	} else {
		cfg.Pullthrough = &Pullthrough{}
	}

	cfg.Pullthrough.Enabled, err = getBoolOption(PullthroughEnvVar, "pullthrough", defEnabled, options)
	if err != nil {
		err = fmt.Errorf("configuration error in openshift.pullthrough.enabled: %v", err)
		return
	}
	cfg.Pullthrough.Mirror, err = getBoolOption(MirrorPullthroughEnvVar, "mirrorpullthrough", defMirror, options)
	if err != nil {
		err = fmt.Errorf("configuration error in openshift.pullthrough.mirror: %v", err)
	}
	return
}

func migrateCompatibilitySection(cfg *Configuration, options configuration.Parameters) (err error) {
	defAcceptSchema2 := true

	if cfg.Compatibility != nil {
		options = configuration.Parameters{}
		defAcceptSchema2 = cfg.Compatibility.AcceptSchema2
	} else {
		cfg.Compatibility = &Compatibility{}
	}

	cfg.Compatibility.AcceptSchema2, err = getBoolOption(AcceptSchema2EnvVar, "acceptschema2", defAcceptSchema2, options)
	if err != nil {
		err = fmt.Errorf("configuration error in openshift.compatibility.acceptschema2: %v", err)
	}
	return
}

func migrateMiddleware(dockercfg *configuration.Configuration, cfg *Configuration) (err error) {
	var repoMiddleware *configuration.Middleware
	for _, middleware := range dockercfg.Middleware["repository"] {
		if middleware.Name == middlewareName {
			repoMiddleware = &middleware
			break
		}
	}
	if repoMiddleware == nil {
		repoMiddleware = &configuration.Middleware{
			Name:    middlewareName,
			Options: make(configuration.Parameters),
		}
	}

	if cfg.Auth == nil {
		cfg.Auth = &Auth{}
		cfg.Auth.Realm, err = getStringOption("", RealmKey, "origin", dockercfg.Auth.Parameters())
		if err != nil {
			err = fmt.Errorf("configuration error in openshift.auth.realm: %v", err)
			return
		}
		cfg.Auth.TokenRealm, err = getStringOption("", TokenRealmKey, "", dockercfg.Auth.Parameters())
		if err != nil {
			err = fmt.Errorf("configuration error in openshift.auth.tokenrealm: %v", err)
			return
		}
	}
	if cfg.Audit == nil {
		cfg.Audit = &Audit{}
		authParameters := dockercfg.Auth.Parameters()
		if audit, ok := authParameters["audit"]; ok {
			auditOptions := make(map[string]interface{})

			for k, v := range audit.(map[interface{}]interface{}) {
				if s, ok := k.(string); ok {
					auditOptions[s] = v
				}
			}

			cfg.Audit.Enabled, err = getBoolOption("", "enabled", false, auditOptions)
			if err != nil {
				err = fmt.Errorf("configuration error in openshift.audit.enabled: %v", err)
				return
			}
		}
	}
	for _, migrator := range []func(*Configuration, configuration.Parameters) error{
		migrateServerSection,
		migrateCacheSection,
		migrateQuotaSection,
		migratePullthroughSection,
		migrateCompatibilitySection,
	} {
		err = migrator(cfg, repoMiddleware.Options)
		if err != nil {
			return
		}
	}
	return nil
}

func InitExtraConfig(dockercfg *configuration.Configuration, cfg *Configuration) error {
	setDefaultMiddleware(dockercfg)
	return migrateMiddleware(dockercfg, cfg)
}
