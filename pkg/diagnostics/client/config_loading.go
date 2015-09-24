package client

import (
	"fmt"
	"io/ioutil"
	"os"

	flag "github.com/spf13/pflag"
	"k8s.io/kubernetes/pkg/client/unversioned/clientcmd"

	"github.com/openshift/origin/pkg/cmd/cli/config"
	"github.com/openshift/origin/pkg/diagnostics/types"
)

// ConfigLoading is a little special in that it is run separately as a precondition
// in order to determine whether we can run other dependent diagnostics.
type ConfigLoading struct {
	ConfFlagName   string
	ClientFlags    *flag.FlagSet
	successfulLoad bool // set if at least one file loaded
}

func (d *ConfigLoading) Name() string {
	return "ConfigLoading"
}

func (d *ConfigLoading) Description() string {
	return "Try to load client config file(s) and report what happens"
}

func (d *ConfigLoading) CanRun() (bool, error) {
	return true, nil
}

func (d *ConfigLoading) SuccessfulLoad() bool {
	return d.successfulLoad
}

func (d *ConfigLoading) Check() types.DiagnosticResult {
	r := types.NewDiagnosticResult("ConfigLoading")
	confFlagValue := d.ClientFlags.Lookup(d.ConfFlagName).Value.String()

	var foundPath string
	rules := config.NewOpenShiftClientConfigLoadingRules()
	paths := append([]string{confFlagValue}, rules.Precedence...)
	for index, path := range paths {
		errmsg := ""
		switch index {
		case 0:
			errmsg = fmt.Sprintf("--%s specified that client config should be at %s\n", d.ConfFlagName, path)
		case len(paths) - 1: // config in ~/.kube
		// no error message indicated if it is not there... user didn't say it would be
		default: // can be multiple paths from the env var in theory; all cases should go here
			if len(os.Getenv(config.OpenShiftConfigPathEnvVar)) != 0 {
				errmsg = fmt.Sprintf("Env var %s specified that client config could be at %s\n", config.OpenShiftConfigPathEnvVar, path)
			}
		}

		if d.canOpenConfigFile(path, errmsg, r) && foundPath == "" {
			d.successfulLoad = true
			foundPath = path
		}
	}
	if foundPath != "" {
		if confFlagValue != "" && confFlagValue != foundPath {
			// found config but not where --config said
			r.Error("DCli1001", nil, fmt.Sprintf(`
The client configuration file was not found where the --%s flag indicated:
  %s
A config file was found at the following location:
  %s
If you wish to use this file for client configuration, you can specify it
with the --%[1]s flag, or just not specify the flag.
			`, d.ConfFlagName, confFlagValue, foundPath))
		}
	} else { // not found, check for master-generated ones to recommend
		if confFlagValue != "" {
			r.Error("DCli1002", nil, fmt.Sprintf("Did not find config file where --%s=%s indicated", d.ConfFlagName, confFlagValue))
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
		adminPaths := []string{
			"/etc/openshift/master/admin.kubeconfig",           // enterprise
			"/openshift.local.config/master/admin.kubeconfig",  // origin systemd
			"./openshift.local.config/master/admin.kubeconfig", // origin binary
		}
		// look for it in auto-generated locations when not found properly
		for _, path := range adminPaths {
			msg := fmt.Sprintf("Looking for a possible client config at %s\n", path)
			if d.canOpenConfigFile(path, msg, r) {
				r.Warn("DCli1003", nil, fmt.Sprintf(adminWarningF, config.OpenShiftConfigPathEnvVar, path, config.RecommendedHomeFile))
				break
			}
		}
	}
	return r
}

// ----------------------------------------------------------
// Attempt to open file at path as client config
// If there is a problem and errmsg is set, log an error
func (d ConfigLoading) canOpenConfigFile(path string, errmsg string, r types.DiagnosticResult) bool {
	var file *os.File
	var err error
	if path == "" { // empty param/envvar
		return false
	} else if file, err = os.Open(path); err == nil {
		r.Debug("DCli1004", fmt.Sprintf("Reading client config at %s", path))
	} else if errmsg == "" {
		r.Debug("DCli1005", fmt.Sprintf("Could not read client config at %s:\n%#v", path, err))
	} else if os.IsNotExist(err) {
		r.Debug("DCli1006", errmsg+"but that file does not exist.")
	} else if os.IsPermission(err) {
		r.Error("DCli1007", err, errmsg+"but lack permission to read that file.")
	} else {
		r.Error("DCli1008", err, fmt.Sprintf("%sbut there was an error opening it:\n%#v", errmsg, err))
	}
	if file != nil { // it is open for reading
		defer file.Close()
		if buffer, err := ioutil.ReadAll(file); err != nil {
			r.Error("DCli1009", err, fmt.Sprintf("Unexpected error while reading client config file (%s): %v", path, err))
		} else if _, err := clientcmd.Load(buffer); err != nil {
			r.Error("DCli1010", err, fmt.Sprintf(`
Error reading YAML from client config file (%s):
  %v
This file may have been truncated or mis-edited.
Please fix, remove, or obtain a new client config`, file.Name(), err))
		} else {
			r.Info("DCli1011", fmt.Sprintf("Successfully read a client config file at '%s'", path))
			/* Note, we're not going to use this config file directly.
			 * Instead, we'll defer to the openshift client code to assimilate
			 * flags, env vars, and the potential hierarchy of config files
			 * into an actual configuration that the client uses.
			 * However, for diagnostic purposes, record the files we find.
			 */
			return true
		}
	}
	return false
}
