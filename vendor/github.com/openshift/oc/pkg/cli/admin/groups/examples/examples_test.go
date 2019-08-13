package examples

import (
	"bytes"
	"io/ioutil"
	"testing"

	legacyconfigv1 "github.com/openshift/api/legacyconfig/v1"
	"github.com/openshift/library-go/pkg/config/helpers"
	"github.com/openshift/oc/pkg/helpers/groupsync/ldap"
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
		yamlConfig, err := ioutil.ReadFile("./../../../../../testdata/ldap/" + fixture)
		if err != nil {
			t.Errorf("could not read fixture at %q: %v", fixture, err)
			continue
		}

		uncast, err := helpers.ReadYAML(bytes.NewBuffer([]byte(yamlConfig)), legacyconfigv1.InstallLegacy)
		if err != nil {
			t.Error(err)
		}
		ldapConfig := uncast.(*legacyconfigv1.LDAPSyncConfig)

		if results := ldap.ValidateLDAPSyncConfig(ldapConfig); len(results.Errors) > 0 {
			t.Errorf("validation of fixture at %q failed with %d errors:", fixture, len(results.Errors))
			for _, err := range results.Errors {
				t.Error(err)
			}
		}
	}
}
