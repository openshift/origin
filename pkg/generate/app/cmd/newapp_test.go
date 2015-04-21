package cmd

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/generate/app"
)

func TestAddArguments(t *testing.T) {
	tests := map[string]struct {
		args       []string
		env        util.StringList
		parms      util.StringList
		repos      util.StringList
		components util.StringList
		unknown    []string
	}{
		"components": {
			args:       []string{"one", "two+three", "four~five"},
			components: util.StringList{"one", "two+three", "four~five"},
			unknown:    []string{},
		},
		"source": {
			args:    []string{".", "./test/one/two/three", "/var/go/src/test", "git://server/repo.git"},
			repos:   util.StringList{".", "./test/one/two/three", "/var/go/src/test", "git://server/repo.git"},
			unknown: []string{},
		},
		"env": {
			args:    []string{"first=one", "second=two", "third=three"},
			env:     util.StringList{"first=one", "second=two", "third=three"},
			unknown: []string{},
		},
		"mix 1": {
			args:       []string{"git://server/repo.git", "mysql+ruby~git@test.server/repo.git", "env1=test", "ruby-helloworld-sample"},
			repos:      util.StringList{"git://server/repo.git"},
			components: util.StringList{"mysql+ruby~git@test.server/repo.git", "ruby-helloworld-sample"},
			env:        util.StringList{"env1=test"},
			unknown:    []string{},
		},
	}

	for n, c := range tests {
		a := AppConfig{}
		unknown := a.AddArguments(c.args)
		if !reflect.DeepEqual(a.Environment, c.env) {
			t.Errorf("%s: Different env variables. Expected: %v, Actual: %v", n, c.env, a.Environment)
		}
		if !reflect.DeepEqual(a.SourceRepositories, c.repos) {
			t.Errorf("%s: Different source repos. Expected: %v, Actual: %v", n, c.repos, a.SourceRepositories)
		}
		if !reflect.DeepEqual(a.Components, c.components) {
			t.Errorf("%s: Different components. Expected: %v, Actual: %v", n, c.components, a.Components)
		}
		if !reflect.DeepEqual(unknown, c.unknown) {
			t.Errorf("%s: Different unknown result. Expected: %v, Actual: %v", n, c.unknown, unknown)
		}
	}

}

func TestValidate(t *testing.T) {
	tests := map[string]struct {
		cfg                 AppConfig
		componentValues     []string
		sourceRepoLocations []string
		env                 map[string]string
		parms               map[string]string
	}{
		"components": {
			cfg: AppConfig{
				Components: util.StringList{"one", "two", "three/four"},
			},
			componentValues:     []string{"one", "two", "three/four"},
			sourceRepoLocations: []string{},
			env:                 map[string]string{},
			parms:               map[string]string{},
		},
		"sourcerepos": {
			cfg: AppConfig{
				SourceRepositories: []string{".", "/test/var/src", "https://server/repo.git"},
			},
			componentValues:     []string{},
			sourceRepoLocations: []string{".", "/test/var/src", "https://server/repo.git"},
			env:                 map[string]string{},
			parms:               map[string]string{},
		},
		"envs": {
			cfg: AppConfig{
				Environment: util.StringList{"one=first", "two=second", "three=third"},
			},
			componentValues:     []string{},
			sourceRepoLocations: []string{},
			env:                 map[string]string{"one": "first", "two": "second", "three": "third"},
			parms:               map[string]string{},
		},
		"component+source": {
			cfg: AppConfig{
				Components: util.StringList{"one~https://server/repo.git"},
			},
			componentValues:     []string{"one"},
			sourceRepoLocations: []string{"https://server/repo.git"},
			env:                 map[string]string{},
			parms:               map[string]string{},
		},
		"components+source": {
			cfg: AppConfig{
				Components: util.StringList{"mysql+ruby~git://github.com/namespace/repo.git"},
			},
			componentValues:     []string{"mysql", "ruby"},
			sourceRepoLocations: []string{"git://github.com/namespace/repo.git"},
			env:                 map[string]string{},
			parms:               map[string]string{},
		},
		"components+parms": {
			cfg: AppConfig{
				Components:         util.StringList{"ruby-helloworld-sample"},
				TemplateParameters: util.StringList{"one=first", "two=second"},
			},
			componentValues:     []string{"ruby-helloworld-sample"},
			sourceRepoLocations: []string{},
			env:                 map[string]string{},
			parms: map[string]string{
				"one": "first",
				"two": "second",
			},
		},
	}

	for n, c := range tests {
		cr, repos, env, parms, err := c.cfg.validate()
		if err != nil {
			t.Errorf("%s: Unexpected error: %v", n, err)
		}
		compValues := []string{}
		for _, r := range cr {
			compValues = append(compValues, r.Input().Value)
		}
		if !reflect.DeepEqual(c.componentValues, compValues) {
			t.Errorf("%s: Component values don't match. Expected: %v, Got: %v", n, c.componentValues, compValues)
		}
		repoLocations := []string{}
		for _, r := range repos {
			repoLocations = append(repoLocations, r.String())
		}
		if !reflect.DeepEqual(c.sourceRepoLocations, repoLocations) {
			t.Errorf("%s: Repository locations don't match. Expected: %v, Got: %v", n, c.sourceRepoLocations, repoLocations)
		}
		if len(env) != len(c.env) {
			t.Errorf("%s: Environment variables don't match. Expected: %v, Got: %v", n, c.env, env)
		}
		for e, v := range env {
			if c.env[e] != v {
				t.Errorf("%s: Environment variables don't match. Expected: %v, Got: %v", n, c.env, env)
				break
			}
		}
		if len(parms) != len(c.parms) {
			t.Errorf("%s: Template parameters don't match. Expected: %v, Got: %v", n, c.parms, parms)
		}
		for p, v := range parms {
			if c.parms[p] != v {
				t.Errorf("%s: Template parameters don't match. Expected: %v, Got: %v", n, c.parms, parms)
				break
			}
		}
	}
}

func TestBuildTemplates(t *testing.T) {
	tests := map[string]struct {
		templateName string
		namespace    string
		parms        map[string]string
	}{
		"simple": {
			templateName: "first-stored-template",
			namespace:    "default",
			parms:        map[string]string{},
		},
	}

	for n, c := range tests {
		appCfg := AppConfig{}
		appCfg.SetOpenShiftClient(&client.Fake{}, c.namespace)
		appCfg.AddArguments([]string{c.templateName})
		appCfg.TemplateParameters = util.StringList{}
		for k, v := range c.parms {
			appCfg.TemplateParameters.Set(fmt.Sprintf("%v=%v", k, v))
		}

		components, _, _, parms, err := appCfg.validate()
		if err != nil {
			t.Errorf("%s: Unexpected error: %v", n, err)
		}
		err = appCfg.resolve(components)
		if err != nil {
			t.Errorf("%s: Unexpected error: %v", n, err)
		}
		_, err = appCfg.buildTemplates(components, app.Environment(parms))
		if err != nil {
			t.Errorf("%s: Unexpected error: %v", n, err)
		}
		for _, component := range components {
			match := component.Input().Match
			if !match.IsTemplate() {
				t.Errorf("%s: Expected template match, got: %v", n, match)
			}
			if c.templateName != match.Name {
				t.Errorf("%s: Expected template name %q, got: %q", n, c.templateName, match.Name)
			}
			if len(parms) != len(c.parms) {
				t.Errorf("%s: Template parameters don't match. Expected: %v, Got: %v", n, c.parms, parms)
			}
			for p, v := range parms {
				if c.parms[p] != v {
					t.Errorf("%s: Template parameters don't match. Expected: %v, Got: %v", n, c.parms, parms)
					break
				}
			}
		}
	}
}
