package run

import (
	"github.com/openshift/origin/pkg/cmd/experimental/diagnostics/options"
	"github.com/openshift/origin/pkg/cmd/server/start"
	"github.com/openshift/origin/pkg/diagnostics/client"
	"github.com/openshift/origin/pkg/diagnostics/discovery"
	"github.com/openshift/origin/pkg/diagnostics/log"
	"github.com/openshift/origin/pkg/diagnostics/systemd"
	"github.com/openshift/origin/pkg/diagnostics/types/diagnostic"
	"os"
	"strings"
)

func Diagnose(opts *options.AllDiagnosticsOptions) {
	// start output to a log
	dopts := opts.DiagOptions
	logger, _ := log.NewLogger(dopts.DiagLevel, dopts.DiagFormat, dopts.Output.Get())
	// start discovery
	if env := RunDiscovery(opts, logger); env != nil { // discovery result can veto continuing
		allDiags := make(map[string]map[string]diagnostic.Diagnostic)
		// now we will figure out what diagnostics to run based on discovery.
		for area := range env.WillCheck {
			switch area {
			case discovery.ClientTarget:
				allDiags["client"] = client.Diagnostics
			case discovery.MasterTarget, discovery.NodeTarget:
				allDiags["systemd"] = systemd.Diagnostics
			}
		}
		if list := opts.DiagOptions.Diagnostics; len(*list) > 0 {
			// just run a specific (set of) diagnostic(s)
			for _, arg := range *list {
				parts := strings.SplitN(arg, ".", 2)
				if len(parts) < 2 {
					env.Log.Noticef("noDiag", `There is no such diagnostic "%s"`, arg)
					continue
				}
				area, name := parts[0], parts[1]
				if diagnostics, exists := allDiags[area]; !exists {
					env.Log.Noticef("noDiag", `There is no such diagnostic "%s"`, arg)
				} else if diag, exists := diagnostics[name]; !exists {
					env.Log.Noticef("noDiag", `There is no such diagnostic "%s"`, arg)
				} else {
					RunDiagnostic(area, name, diag, env)
				}
			}
		} else {
			// TODO: run all of these in parallel but ensure sane output
			for area, diagnostics := range allDiags {
				for name, diag := range diagnostics {
					RunDiagnostic(area, name, diag, env)
				}
			}
		}
	}
	logger.Summary()
	logger.Finish()
	if logger.ErrorsSeen() {
		os.Exit(255)
	}
}

// ----------------------------------------------------------
// Examine system and return findings in an Environment
func RunDiscovery(adOpts *options.AllDiagnosticsOptions, logger *log.Logger) *discovery.Environment {
	logger.Notice("discBegin", "Beginning discovery of environment")
	env := discovery.NewEnvironment(adOpts, logger)
	env.DiscoverOperatingSystem()
	if adOpts.MasterDiagOptions != nil || adOpts.NodeDiagOptions != nil {
		env.DiscoverSystemd()
	}
	if mdOpts := adOpts.MasterDiagOptions; mdOpts != nil {
		if mdOpts.MasterStartOptions == nil {
			mdOpts.MasterStartOptions = &start.MasterOptions{ConfigFile: adOpts.MasterConfigPath}
			// leaving MasterArgs nil signals it has to be a master config file or nothing.
		} else if adOpts.MasterConfigPath != "" {
			mdOpts.MasterStartOptions.ConfigFile = adOpts.MasterConfigPath
		}
		env.DiscoverMaster()
	}
	if ndOpts := adOpts.NodeDiagOptions; ndOpts != nil {
		if ndOpts.NodeStartOptions == nil {
			ndOpts.NodeStartOptions = &start.NodeOptions{ConfigFile: adOpts.NodeConfigPath}
			// no NodeArgs signals it has to be a node config file or nothing.
		} else if adOpts.NodeConfigPath != "" {
			ndOpts.NodeStartOptions.ConfigFile = adOpts.NodeConfigPath
		}
		env.DiscoverNode()
	}
	if cdOpts := adOpts.ClientDiagOptions; cdOpts != nil {
		env.DiscoverClient()
		env.ReadClientConfigFiles() // so user knows where config is coming from (or not)
		env.ConfigClient()
	}
	checkAny := false
	for _, check := range env.WillCheck {
		checkAny = checkAny || check
	}
	if !checkAny {
		logger.Error("discNoChecks", "Cannot find any OpenShift configuration. Please specify which component or configuration you wish to troubleshoot.")
		return nil
	}
	return env
}

func RunDiagnostic(area string, name string, diag diagnostic.Diagnostic, env *discovery.Environment) {
	defer func() {
		// recover from diagnostics that panic so others can still run
		if r := recover(); r != nil {
			env.Log.Errorf("diagPanic", "Diagnostic '%s' crashed; this is usually a bug in either diagnostics or OpenShift. Stack trace:\n%+v", name, r)
		}
	}()
	if diag.Condition != nil {
		if skip, reason := diag.Condition(env); skip {
			if reason == "" {
				env.Log.Noticem("diagSkip", log.Msg{"area": area, "name": name, "diag": diag.Description,
					"tmpl": "Skipping diagnostic: {{.area}}.{{.name}}\nDescription: {{.diag}}"})
			} else {
				env.Log.Noticem("diagSkip", log.Msg{"area": area, "name": name, "diag": diag.Description, "reason": reason,
					"tmpl": "Skipping diagnostic: {{.area}}.{{.name}}\nDescription: {{.diag}}\nBecause: {{.reason}}"})
			}
			return
		}
	}
	env.Log.Noticem("diagRun", log.Msg{"area": area, "name": name, "diag": diag.Description,
		"tmpl": "Running diagnostic: {{.area}}.{{.name}}\nDescription: {{.diag}}"})
	diag.Run(env)
}
