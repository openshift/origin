package config

import (
	"encoding/json"
	"fmt"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	clientapi "github.com/openshift/origin/pkg/cmd/client/api"
)

// configJSON stores the raw Config JSON representation
// TODO: Replace this with configapi.Config when it handles the unregistred types.
type configJSON struct {
	Items []interface{} `json:"items" yaml:"items"`
}

// Apply creates and manages resources defined in the Config. It wont stop on
// error, but it will finish the job and then return list of errors.
func Apply(data []byte, storage clientapi.ClientMappings) (errs errors.ErrorList) {

	// Unmarshal the Config JSON using default json package instead of
	// api.Decode()
	conf := configJSON{}
	if err := json.Unmarshal(data, &conf); err != nil {
		return append(errs, fmt.Errorf("Unable to parse Config: %v", err))
	}

	for _, item := range conf.Items {
		kind, itemId, parseErrs := parseKindAndId(item)
		if len(parseErrs) != 0 {
			errs = append(errs, parseErrs...)
			continue
		}

		client, path := getClientAndPath(kind, storage)
		if client == nil {
			errs = append(errs, fmt.Errorf("The resource %s is not a known type - unable to create %s", kind, itemId))
			continue
		}

		// Serialize the single Config item back into JSON
		itemJson, _ := json.Marshal(item)

		request := client.Verb("POST").Path(path).Body(itemJson)
		_, err := request.Do().Get()
		if err != nil {
			errs = append(errs, fmt.Errorf("[%s#%s] Failed to create: %v", kind, itemId, err))
		}
	}

	return
}

// getClientAndPath returns the RESTClient and path defined for given resource
// kind.
func getClientAndPath(kind string, mappings clientapi.ClientMappings) (client clientapi.RESTClient, path string) {
	for k, m := range mappings {
		if k == kind {
			return m.Client, k
		}
	}
	return
}

// parseKindAndId extracts the 'kind' and 'id' fields from the Config item JSON
// and report errors if these fields are missing.
func parseKindAndId(item interface{}) (kind, id string, errs errors.ErrorList) {
	itemMap := item.(map[string]interface{})

	kind, ok := itemMap["kind"].(string)
	if !ok {
		errs = append(errs, reportError(item, "Missing 'kind' field for Config item"))
	}

	id, ok = itemMap["id"].(string)
	if !ok {
		errs = append(errs, reportError(item, "Missing 'id' field for Config item"))
	}

	return
}

// reportError provides a human-readable error message that include the Config
// item JSON representation.
func reportError(item interface{}, message string) error {
	itemJson, _ := json.Marshal(item)
	return fmt.Errorf(message+": %s", string(itemJson))
}
