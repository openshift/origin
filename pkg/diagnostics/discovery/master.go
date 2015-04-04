package discovery

import (
	mconfigapilatest "github.com/openshift/origin/pkg/cmd/server/api/latest"
	"github.com/openshift/origin/pkg/cmd/server/start"
)

const StandardMasterConfPath string = "/etc/openshift/master/master-config.yaml"

func (env *Environment) DiscoverMaster() {
	// first, determine if we even have a master config
	options := env.Options.MasterDiagOptions
	if env.Options.MasterConfigPath != "" { // specified master conf, it has to load or we choke
		options.MasterStartOptions.MasterArgs = start.NewDefaultMasterArgs() // and don't set any args
		if env.tryMasterConfig(true) {
			env.WillCheck[MasterTarget] = true
		}
	} else { // user did not indicate config file
		env.Log.Debug("discMCnofile", "No top-level --master-config file specified")
		if !options.MustCheck {
			// general command, user couldn't indicate server flags;
			// look for master config in standard location(s)
			env.tryStandardMasterConfig() // or give up.
		} else { // assume user provided flags like actual master.
			env.tryMasterConfig(true)
			env.WillCheck[MasterTarget] = true // regardless
		}
	}
	if !env.WillCheck[MasterTarget] {
		env.Log.Notice("discMCnone", "No master config found; master diagnostics will not be performed.")
	}
}

func (env *Environment) tryMasterConfig(errOnFail bool) bool /* worked? */ {
	options := env.Options.MasterDiagOptions.MasterStartOptions
	logOnFail := env.Log.Debugf
	if errOnFail {
		logOnFail = env.Log.Errorf
	}
	if err := options.Complete(); err != nil {
		logOnFail("discMCstart", "Could not read master config options: (%T) %[1]v", err)
		return false
	} else if err = options.Validate([]string{}); err != nil {
		logOnFail("discMCstart", "Could not read master config options: (%T) %[1]v", err)
		return false
	}
	var err error
	if path := options.ConfigFile; path != "" {
		env.Log.Debugf("discMCfile", "Looking for master config file at '%s'", path)
		if env.MasterConfig, err = mconfigapilatest.ReadAndResolveMasterConfig(path); err != nil {
			logOnFail("discMCfail", "Could not read master config file '%s':\n(%T) %[2]v", path, err)
			return false
		}
		env.Log.Infof("discMCfound", "Found a master config file:\n%[1]s", path)
		return true
	} else {
		if env.MasterConfig, err = options.MasterArgs.BuildSerializeableMasterConfig(); err != nil {
			logOnFail("discMCopts", "Could not build a master config from flags:\n(%T) %[1]v", err)
			return false
		}
		env.Log.Infof("discMCfound", "No master config file, using any flags for configuration.")
	}
	return false
}

func (env *Environment) tryStandardMasterConfig() bool /* worked? */ {
	env.Log.Debug("discMCnoflags", "No master config flags specified, will try standard config location")
	options := env.Options.MasterDiagOptions.MasterStartOptions
	options.ConfigFile = StandardMasterConfPath
	options.MasterArgs = start.NewDefaultMasterArgs()
	if env.tryMasterConfig(false) {
		env.Log.Debug("discMCdefault", "Using master config file at "+StandardMasterConfPath)
		env.WillCheck[MasterTarget] = true
		return true
	} else { // otherwise, we just don't do master diagnostics
		env.Log.Debugf("discMCnone", "Not using master config file at "+StandardMasterConfPath+" - will not do master diagnostics.")
	}
	return false
}
