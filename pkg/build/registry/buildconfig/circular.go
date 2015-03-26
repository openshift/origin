package buildconfig

import (
	"errors"

	"github.com/openshift/origin/pkg/build/api"
)

// deps contains all image repositories mapped to their dependencies
var deps = make(map[string]map[string]string)

// circularDeps checks for circular dependencies in build configurations
func circularDeps(config *api.BuildConfig) bool {
	if config.Parameters.Output.To == nil || len(config.Triggers) == 0 {
		return false
	}

	// Create output spec
	if len(config.Parameters.Output.To.Namespace) == 0 {
		config.Parameters.Output.To.Namespace = config.Namespace
	}
	if len(config.Parameters.Output.Tag) == 0 {
		config.Parameters.Output.Tag = "latest"
	}
	to := config.Parameters.Output.To.Namespace + "/" + config.Parameters.Output.To.Name + ":" + config.Parameters.Output.Tag

	for _, tr := range config.Triggers {
		if tr.ImageChange == nil {
			continue
		}

		// Create input spec
		if len(tr.ImageChange.From.Namespace) == 0 {
			tr.ImageChange.From.Namespace = config.Namespace
		}
		if len(tr.ImageChange.Tag) == 0 {
			tr.ImageChange.Tag = "latest"
		}
		from := tr.ImageChange.From.Namespace + "/" + tr.ImageChange.From.Name + ":" + tr.ImageChange.Tag

		// Check for any conflicts in dependencies
		if err := findConflicts(from, to); err != nil {
			return true
		}
		// The trigger is valid, add the relationship in deps
		if _, ok := deps[from]; !ok {
			deps[from] = make(map[string]string)
		}
		fromDeps := deps[from]
		fromDeps[to] = config.Name
	}

	return false
}

// findConflicts traces if there are any conflicts in the dependencies
// of From and To in a build configuration
func findConflicts(from, to string) error {
	toDeps := deps[to]

	// Check the dependencies of the dependency we want to add
	for toDep := range toDeps {
		if from == toDep {
			// From already depends on To
			return errors.New("circular dependencies in buildConfigs")
		}
		if err := findConflicts(from, toDep); err != nil {
			// From already depends on another repo that depends on To
			return err
		}
	}
	return nil
}

func deleteFromDeps(id string) {
	for _, dep := range deps {
		for repo, cfgName := range dep {
			if cfgName == id {
				delete(dep, repo)
			}
		}
	}
}
