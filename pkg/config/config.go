package config

import (
	"encoding/json"
	"fmt"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

	clientapi "github.com/openshift/origin/pkg/cmd/client/api"
)

// Apply creates and manages resources defined in the Config. It wont stop on
// error, but it will finish the job and then return list of errors.
//
// TODO: Return the output for each resource on success, so the client can
//       print it out.
func Apply(data []byte, storage clientapi.ClientMappings) (errs errors.ErrorList) {

	// Unmarshal the Config JSON manually instead of using runtime.Decode()
	conf := struct {
		Items []json.RawMessage `json:"items" yaml:"items"`
	}{}
	if err := json.Unmarshal(data, &conf); err != nil {
		return append(errs, fmt.Errorf("Unable to parse Config: %v", err))
	}

	if len(conf.Items) == 0 {
		return append(errs, fmt.Errorf("Config.items is empty"))
	}

	for i, item := range conf.Items {
		if item == nil || (len(item) == 4 && string(item) == "null") {
			errs = append(errs, fmt.Errorf("Config.items[%v] is null", i))
			continue
		}

		_, kind, err := runtime.VersionAndKind(item)
		if err != nil {
			errs = append(errs, fmt.Errorf("Config.items[%v]: %v", i, err))
			continue
		}

		if kind == "" {
			errs = append(errs, fmt.Errorf("Config.items[%v] has an empty 'kind'", i))
			continue
		}

		client, path, err := getClientAndPath(kind, storage)
		if err != nil {
			errs = append(errs, fmt.Errorf("Config.items[%v]: %v", i, err))
			continue
		}
		if client == nil {
			errs = append(errs, fmt.Errorf("Config.items[%v]: Invalid client for 'kind=%v'", i, kind))
			continue
		}

		jsonResource, err := item.MarshalJSON()
		if err != nil {
			errs = append(errs, err)
			continue
		}

		request := client.Verb("POST").Path(path).Body(jsonResource)
		_, err = request.Do().Get()
		if err != nil {
			errs = append(errs, fmt.Errorf("Failed to create Config.items[%v] of 'kind=%v': %v", i, kind, err))
		}
	}

	return
}

// getClientAndPath returns the RESTClient and path defined for a given
// resource kind. Returns an error when no RESTClient is found.
func getClientAndPath(kind string, mappings clientapi.ClientMappings) (clientapi.RESTClient, string, error) {
	for k, m := range mappings {
		if m.Kind == kind {
			return m.Client, k, nil
		}
	}
	return nil, "", fmt.Errorf("No client found for 'kind=%v'", kind)
}

// reportError provides a human-readable error message that include the Config
// item JSON representation.
func reportError(item interface{}, message string) error {
	itemJSON, _ := json.Marshal(item)
	return fmt.Errorf(message+": %s", string(itemJSON))
}
