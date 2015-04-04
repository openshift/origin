package discovery

import (
	mconfigapilatest "github.com/openshift/origin/pkg/cmd/server/api/latest"
	"github.com/openshift/origin/pkg/cmd/server/start"
)

const StandardNodeConfPath string = "/etc/openshift/node/node-config.yaml"

func (env *Environment) DiscoverNode() {
	// first, determine if we even have a node config
	options := env.Options.NodeDiagOptions
	if env.Options.NodeConfigPath != "" { // specified node conf, it has to load or we choke
		options.NodeStartOptions.NodeArgs = start.NewDefaultNodeArgs() // and don't set any args
		if env.tryNodeConfig(true) {
			env.WillCheck[NodeTarget] = true
		}
	} else { // user did not indicate config file
		env.Log.Debug("discNCnofile", "No node config file specified")
		if !options.MustCheck {
			// general command, user couldn't indicate server flags;
			// look for node config in standard location(s)
			env.tryStandardNodeConfig() // or give up.
		} else { // assume user provided flags like actual node.
			env.tryNodeConfig(true)
			env.WillCheck[NodeTarget] = true // regardless
		}
	}
	if !env.WillCheck[NodeTarget] {
		env.Log.Notice("discNCnone", "No node config found; node diagnostics will not be performed.")
	}
}

func (env *Environment) tryNodeConfig(errOnFail bool) bool /* worked */ {
	options := env.Options.NodeDiagOptions.NodeStartOptions
	//pretty.Println("nodeconfig options are:", options)
	logOnFail := env.Log.Debugf
	if errOnFail {
		logOnFail = env.Log.Errorf
	}
	if err := options.Complete(); err != nil {
		logOnFail("discNCstart", "Could not read node config options: (%T) %[1]v", err)
		return false
	} else if err = options.Validate([]string{}); err != nil {
		logOnFail("discNCstart", "Could not read node config options: (%T) %[1]v", err)
		return false
	}
	var err error
	if path := options.ConfigFile; path != "" {
		env.Log.Debugf("discNCfile", "Looking for node config file at '%s'", path)
		if env.NodeConfig, err = mconfigapilatest.ReadAndResolveNodeConfig(path); err != nil {
			logOnFail("discNCfail", "Could not read node config file '%s':\n(%T) %[2]v", path, err)
			return false
		}
		env.Log.Infof("discNCfound", "Found a node config file:\n%[1]s", path)
		return true
	} else {
		if env.NodeConfig, err = options.NodeArgs.BuildSerializeableNodeConfig(); err != nil {
			logOnFail("discNCopts", "Could not build a node config from flags:\n(%T) %[1]v", err)
			return false
		}
		env.Log.Infof("discNCfound", "No node config file, using any flags for configuration.")
	}
	return false
}

func (env *Environment) tryStandardNodeConfig() bool /*worked*/ {
	env.Log.Debug("discNCnoflags", "No node config flags specified, will try standard config location")
	options := env.Options.NodeDiagOptions.NodeStartOptions
	options.ConfigFile = StandardNodeConfPath
	options.NodeArgs = start.NewDefaultNodeArgs()
	if env.tryNodeConfig(false) {
		env.Log.Debug("discNCdefault", "Using node config file at "+StandardNodeConfPath)
		env.WillCheck[NodeTarget] = true
		return true
	} else { // otherwise, we just don't do node diagnostics
		env.Log.Debugf("discNCnone", "Not using node config file at "+StandardNodeConfPath+" - will not do node diagnostics.")
	}
	return false
}
