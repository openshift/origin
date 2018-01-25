package diagnostics

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"

	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/openshift/origin/pkg/client/config"
	"github.com/openshift/origin/pkg/oc/admin/diagnostics/diagnostics/util"
)

// use the base factory to return a raw config (not specific to a context)
func (o DiagnosticsOptions) buildRawConfig() (*clientcmdapi.Config, error) {
	kubeConfig, configErr := o.Factory.OpenShiftClientConfig().RawConfig()
	if configErr != nil {
		return nil, configErr
	}
	if len(kubeConfig.Contexts) == 0 {
		return nil, errors.New("No contexts found in config file.")
	}
	return &kubeConfig, nil
}

// determine if we even have a client config
func (o DiagnosticsOptions) detectClientConfig() (expected bool, detected bool) {
	if o.ClientFlags == nil {
		// options for client not provided, so it must not be expected.
		return false, false
	}
	o.Logger().Notice("CED2011", "Determining if client configuration exists for client/cluster diagnostics")
	confFlagName := config.OpenShiftConfigFlagName
	confFlagValue := o.ClientFlags.Lookup(confFlagName).Value.String()
	successfulLoad := false

	var foundPath string
	rules := config.NewOpenShiftClientConfigLoadingRules()
	paths := append([]string{confFlagValue}, rules.Precedence...)
	for index, path := range paths {
		errmsg := ""
		switch index {
		case 0:
			errmsg = fmt.Sprintf("--%s specified that client config should be at %s\n", confFlagName, path)
		case len(paths) - 1: // config in ~/.kube
		// no error message indicated if it is not there... user didn't say it would be
		default: // can be multiple paths from the env var in theory; all cases should go here
			if len(os.Getenv(config.OpenShiftConfigPathEnvVar)) != 0 {
				errmsg = fmt.Sprintf("Env var %s specified that client config could be at %s\n", config.OpenShiftConfigPathEnvVar, path)
			}
		}

		if o.canOpenConfigFile(path, errmsg) && len(foundPath) == 0 {
			successfulLoad = true
			foundPath = path
		}
	}
	if len(foundPath) > 0 {
		if len(confFlagValue) > 0 && confFlagValue != foundPath {
			// found config but not where --config said
			o.Logger().Error("DCli1001", fmt.Sprintf(`
The client configuration file was not found where the --%s flag indicated:
  %s
A config file was found at the following location:
  %s
If you wish to use this file for client configuration, you can specify it
with the --%[1]s flag, or just not specify the flag.
			`, confFlagName, confFlagValue, foundPath))
		}
	} else { // not found, check for master-generated ones to recommend
		if len(confFlagValue) > 0 {
			o.Logger().Error("DCli1002", fmt.Sprintf("Did not find config file where --%s=%s indicated", confFlagName, confFlagValue))
		}
		adminWarningF := `
No client config file was available; however, one exists at
    %[2]s
which may have been generated automatically by the master.
If you want to use this config, you should copy it to the
standard location (%[3]s),
or you can set the environment variable %[1]s:
    export %[1]s=%[2]s
If not, obtain a config file and place it in the standard
location for use by the client and diagnostics.
`
		// look for it in auto-generated locations when not found properly
		for _, path := range util.AdminKubeConfigPaths {
			msg := fmt.Sprintf("Looking for a possible client config at %s\n", path)
			if o.canOpenConfigFile(path, msg) {
				o.Logger().Warn("DCli1003", fmt.Sprintf(adminWarningF, config.OpenShiftConfigPathEnvVar, path, config.RecommendedHomeFile))
				break
			}
		}
	}
	return true, successfulLoad
}

// ----------------------------------------------------------
// Attempt to open file at path as client config
// If there is a problem and errmsg is set, log an error
func (o DiagnosticsOptions) canOpenConfigFile(path string, errmsg string) bool {
	var (
		file *os.File
		err  error
	)
	if len(path) == 0 { // empty param/envvar
		return false
	} else if file, err = os.Open(path); err == nil {
		o.Logger().Debug("DCli1004", fmt.Sprintf("Reading client config at %s", path))
	} else if len(errmsg) == 0 {
		o.Logger().Debug("DCli1005", fmt.Sprintf("Could not read client config at %s:\n%#v", path, err))
	} else if os.IsNotExist(err) {
		o.Logger().Debug("DCli1006", errmsg+"but that file does not exist.")
	} else if os.IsPermission(err) {
		o.Logger().Error("DCli1007", errmsg+"but lack permission to read that file.")
	} else {
		o.Logger().Error("DCli1008", fmt.Sprintf("%sbut there was an error opening it:\n%#v", errmsg, err))
	}
	if file == nil {
		return false
	}

	// file is open for reading
	defer file.Close()
	if buffer, err := ioutil.ReadAll(file); err != nil {
		o.Logger().Error("DCli1009", fmt.Sprintf("Unexpected error while reading client config file (%s): %v", path, err))
	} else if _, err := clientcmd.Load(buffer); err != nil {
		o.Logger().Error("DCli1010", fmt.Sprintf(`
Error reading YAML from client config file (%s):
  %v
This file may have been truncated or mis-edited.
Please fix, remove, or obtain a new client config`, file.Name(), err))
	} else {
		o.Logger().Info("DCli1011", fmt.Sprintf("Successfully read a client config file at '%s'", path))
		/* Note, we're not going to use this config file directly.
		 * Instead, we'll defer to the openshift client code to assimilate
		 * flags, env vars, and the potential hierarchy of config files
		 * into an actual configuration that the client uses.
		 * However, for diagnostic purposes, record the files we find.
		 */
		return true
	}
	return false
}
