package project

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	log "github.com/golang/glog"
)

// ServiceState holds the state of a service.
type ServiceState string

// State definitions
var (
	StateExecuted = ServiceState("executed")
	StateUnknown  = ServiceState("unknown")
)

// Error definitions
var (
	ErrRestart     = errors.New("Restart execution")
	ErrUnsupported = errors.New("UnsupportedOperation")
)

// Event holds project-wide event informations.
type Event struct {
	EventType   EventType
	ServiceName string
	Data        map[string]string
}

// AddEnvironmentLookUp adds mechanism for extracting environment
// variables, from operating system or .env file
func AddEnvironmentLookUp(context *Context) error {
	if context.ResourceLookup == nil {
		context.ResourceLookup = &FileResourceLookup{}
	}

	if context.EnvironmentLookup == nil {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		context.EnvironmentLookup = &ComposableEnvLookup{
			Lookups: []EnvironmentLookup{
				&EnvfileLookup{
					Path: filepath.Join(cwd, ".env"),
				},
				&OsEnvLookup{},
			},
		}
	}
	return nil
}

// NewProject create a new project with the specified context.
func NewProject(context *Context) *Project {
	p := &Project{
		context: context,
		Configs: make(map[string]*ServiceConfig),
	}

	context.Project = p

	return p
}

// Parse populates project information based on its context. It sets up the name,
// the composefile and the composebytes (the composefile content).
func (p *Project) Parse() error {
	err := p.context.open()
	if err != nil {
		return err
	}

	p.Name = p.context.ProjectName

	p.Files = p.context.ComposeFiles

	if len(p.Files) == 1 && p.Files[0] == "-" {
		p.Files = []string{"."}
	}

	if p.context.ComposeBytes != nil {
		for i, composeBytes := range p.context.ComposeBytes {
			file := ""
			if i < len(p.context.ComposeFiles) {
				file = p.Files[i]
			}
			if err := p.load(file, composeBytes); err != nil {
				return err
			}
		}
	}

	return nil
}

// CreateService creates a service with the specified name based. It there
// is no config in the project for this service, it will return an error.
func (p *Project) CreateService(name string) (Service, error) {
	existing, ok := p.Configs[name]
	if !ok {
		return nil, fmt.Errorf("Failed to find service: %s", name)
	}

	// Copy because we are about to modify the environment
	config := *existing

	if p.context.EnvironmentLookup != nil {
		parsedEnv := make([]string, 0, len(config.Environment.Slice()))

		for _, env := range config.Environment.Slice() {
			parts := strings.SplitN(env, "=", 2)
			if len(parts) > 1 && parts[1] != "" {
				parsedEnv = append(parsedEnv, env)
				continue
			} else {
				env = parts[0]
			}

			for _, value := range p.context.EnvironmentLookup.Lookup(env, name, &config) {
				parsedEnv = append(parsedEnv, value)
			}
		}

		config.Environment = NewMaporEqualSlice(parsedEnv)
	}

	return p.context.ServiceFactory.Create(p, name, &config)
}

// AddConfig adds the specified service config for the specified name.
func (p *Project) AddConfig(name string, config *ServiceConfig) error {
	p.Configs[name] = config
	p.reload = append(p.reload, name)

	return nil
}

// Load loads the specified byte array (the composefile content) and adds the
// service configuration to the project.
// FIXME is it needed ?
func (p *Project) Load(bytes []byte) error {
	return p.load("", bytes)
}

func (p *Project) load(file string, bytes []byte) error {
	configs := make(map[string]*ServiceConfig)
	configs, err := mergeProject(p, file, bytes)
	if err != nil {
		log.Errorf("Could not parse config for project %s : %v", p.Name, err)
		return err
	}

	for name, config := range configs {
		err := p.AddConfig(name, config)
		if err != nil {
			return err
		}
	}

	return nil
}
