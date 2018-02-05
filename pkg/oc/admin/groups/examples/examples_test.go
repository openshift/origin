package examples

import (
	"io/ioutil"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/openshift/origin/pkg/cmd/server/apis/config"
	_ "github.com/openshift/origin/pkg/cmd/server/apis/config/install"
	configapilatest "github.com/openshift/origin/pkg/cmd/server/apis/config/latest"
	"github.com/openshift/origin/pkg/cmd/server/apis/config/validation"
)

func TestLDAPSyncConfigFixtures(t *testing.T) {
	fixtures := []string{}

	// build a list of common configurations for all schemas
	schemas := []string{"rfc2307", "ad", "augmented-ad"}
	for _, schema := range schemas {
		fixtures = append(fixtures, schema+"/sync-config.yaml")
		fixtures = append(fixtures, schema+"/sync-config-dn-everywhere.yaml")
		fixtures = append(fixtures, schema+"/sync-config-partially-user-defined.yaml")
		fixtures = append(fixtures, schema+"/sync-config-user-defined.yaml")
		fixtures = append(fixtures, schema+"/sync-config-paging.yaml")
	}
	fixtures = append(fixtures, "rfc2307/sync-config-tolerating.yaml")

	for _, fixture := range fixtures {
		var config config.LDAPSyncConfig

		yamlConfig, err := ioutil.ReadFile("./../../../../../test/extended/authentication/ldap/" + fixture)
		if err != nil {
			t.Errorf("could not read fixture at %q: %v", fixture, err)
			continue
		}

		jsonConfig, err := yaml.ToJSON(yamlConfig)
		if err != nil {
			t.Errorf("could not convert YAML fixture at %q to JSON: %v", fixture, err)
			continue
		}

		if err := runtime.DecodeInto(configapilatest.Codec, jsonConfig, &config); err != nil {
			t.Errorf("could not deocde fixture at %q into internal type: %v", fixture, err)
			continue
		}

		if results := validation.ValidateLDAPSyncConfig(&config); len(results.Errors) > 0 {
			t.Errorf("validation of fixture at %q failed with %d errors:", fixture, len(results.Errors))
			for _, err := range results.Errors {
				t.Error(err)
			}
		}
	}
}
