package discovery

import (
	kclientcmdapi "github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd/api"
	"github.com/openshift/origin/pkg/cmd/experimental/diagnostics/options"
	mconfigapi "github.com/openshift/origin/pkg/cmd/server/api"
	osclientcmd "github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/diagnostics/log"
	"github.com/openshift/origin/pkg/diagnostics/types"
)

// One env instance is created and filled in by discovery.
// Then it should be considered immutable while diagnostics use it.
type Environment struct {
	// the options that were set by command invocation
	Options *options.AllDiagnosticsOptions

	// used to print discovery and diagnostic logs
	Log *log.Logger

	// do we have enough config to diagnose master,node,client?
	WillCheck map[Target]bool

	// general system info
	HasBash      bool                         // for non-Linux clients, will not have bash...
	HasSystemd   bool                         // not even all Linux has systemd
	SystemdUnits map[string]types.SystemdUnit // list of relevant units present on system

	// outcome from looking for executables
	OscPath          string
	OscVersion       types.Version
	OpenshiftPath    string
	OpenshiftVersion types.Version

	// saved results from client discovery
	ClientConfigPath    string                          // first client config file found, if any
	ClientConfigRaw     *kclientcmdapi.Config           // available to analyze ^^
	OsConfig            *kclientcmdapi.Config           // actual merged client configuration
	FactoryForContext   map[string]*osclientcmd.Factory // one for each known context
	AccessForContext    map[string]*ContextAccess       // one for each context that has access to anything
	ClusterAdminFactory *osclientcmd.Factory            // factory we will use for cluster-admin access (could easily be nil)

	// saved results from master discovery
	MasterConfig *mconfigapi.MasterConfig // actual config determined from flags/file

	// saved results from node discovery
	NodeConfig *mconfigapi.NodeConfig // actual config determined from flags/file
}

type ContextAccess struct {
	Projects     []string
	ClusterAdmin bool // has access to see stuff only cluster-admin should
}

func NewEnvironment(opts *options.AllDiagnosticsOptions, logger *log.Logger) *Environment {
	return &Environment{
		Options:           opts,
		Log:               logger,
		SystemdUnits:      make(map[string]types.SystemdUnit),
		WillCheck:         make(map[Target]bool),
		FactoryForContext: make(map[string]*osclientcmd.Factory),
		AccessForContext:  make(map[string]*ContextAccess),
	}
}

// helpful translator
func (env *Environment) DefaultFactory() *osclientcmd.Factory {
	if env.FactoryForContext != nil && env.OsConfig != nil { // no need to panic if missing...
		return env.FactoryForContext[env.OsConfig.CurrentContext]
	}
	return nil
}

type Target string

const (
	ClientTarget Target = "client"
	MasterTarget Target = "master"
	NodeTarget   Target = "node"
)
