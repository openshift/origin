package discovery // client

import (
	"fmt"
	"github.com/openshift/origin/pkg/diagnostics/log"
	"github.com/openshift/origin/pkg/diagnostics/types"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// ----------------------------------------------------------
// Look for 'osc' and 'openshift' executables
func (env *Environment) DiscoverClient() error {
	var err error
	f := env.Options.ClientDiagOptions.Factory
	if config, err := f.OpenShiftClientConfig.RawConfig(); err != nil {
		env.Log.Errorf("discCCstart", "Could not read client config: (%T) %[1]v", err)
	} else {
		env.OsConfig = &config
		env.FactoryForContext[config.CurrentContext] = f
	}
	env.Log.Debug("discSearchExec", "Searching for executables in path:\n  "+strings.Join(filepath.SplitList(os.Getenv("PATH")), "\n  ")) //TODO for non-Linux OS
	env.OscPath = env.findExecAndLog("osc")
	if env.OscPath != "" {
		env.OscVersion, err = getExecVersion(env.OscPath, env.Log)
	}
	env.OpenshiftPath = env.findExecAndLog("openshift")
	if env.OpenshiftPath != "" {
		env.OpenshiftVersion, err = getExecVersion(env.OpenshiftPath, env.Log)
	}
	if env.OpenshiftVersion.NonZero() && env.OscVersion.NonZero() && !env.OpenshiftVersion.Eq(env.OscVersion) {
		env.Log.Warnm("discVersionMM", log.Msg{"osV": env.OpenshiftVersion.GoString(), "oscV": env.OscVersion.GoString(),
			"text": fmt.Sprintf("'openshift' version %#v does not match 'osc' version %#v; update or remove the lower version", env.OpenshiftVersion, env.OscVersion)})
	}
	return err
}

// ----------------------------------------------------------
// Look for a specific executable and log what happens
func (env *Environment) findExecAndLog(cmd string) string {
	if path := findExecFor(cmd); path != "" {
		env.Log.Infom("discExecFound", log.Msg{"command": cmd, "path": path, "tmpl": "Found '{{.command}}' at {{.path}}"})
		return path
	} else {
		env.Log.Warnm("discExecNoPath", log.Msg{"command": cmd, "tmpl": "No '{{.command}}' executable was found in your path"})
	}
	return ""
}

// ----------------------------------------------------------
// Look in the path for an executable
func findExecFor(cmd string) string {
	path, err := exec.LookPath(cmd)
	if err == nil {
		return path
	}
	if runtime.GOOS == "windows" {
		path, err = exec.LookPath(cmd + ".exe")
		if err == nil {
			return path
		}
	}
	return ""
}

// ----------------------------------------------------------
// Invoke executable's "version" command to determine version
func getExecVersion(path string, logger *log.Logger) (version types.Version, err error) {
	cmd := exec.Command(path, "version")
	var out []byte
	out, err = cmd.CombinedOutput()
	if err == nil {
		var name string
		var x, y, z int
		if scanned, err := fmt.Sscanf(string(out), "%s v%d.%d.%d", &name, &x, &y, &z); scanned > 1 {
			version = types.Version{x, y, z}
			logger.Infom("discVersion", log.Msg{"tmpl": "version of {{.command}} is {{.version}}", "command": name, "version": version.GoString()})
		} else {
			logger.Errorf("discVersErr", `
Expected version output from '%s version'
Could not parse output received:
%v
Error was: %#v`, path, string(out), err)
		}
	} else {
		switch err.(type) {
		case *exec.Error:
			logger.Errorf("discVersErr", "error in executing '%v version': %v", path, err)
		case *exec.ExitError:
			logger.Errorf("discVersErr", `
Executed '%v version' which exited with an error code.
This version is likely old or broken.
Error was '%v';
Output was:
%v`, path, err.Error(), log.LimitLines(string(out), 5))
		default:
			logger.Errorf("discVersErr", "executed '%v version' but an error occurred:\n%v\nOutput was:\n%v", path, err, string(out))
		}
	}
	return version, err
}
