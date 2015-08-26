package util

import (
	"strings"

	"github.com/openshift/origin/pkg/client"
)

// GetUserResources queries the server for user resources.
func GetUserResources(c *client.Client) ([]string, error) {
	result := c.RESTClient.Get().AbsPath("/userresources").Do()
	if result.Error() != nil {
		return []string{}, result.Error()
	}
	raw, _ := result.Raw()
	resourceString := string(raw)
	return strings.Split(resourceString, ","), nil
}

// IsBuildEnabled returns true if both builds and buildConfigs are enabled in the resources slice.
func IsBuildEnabled(resources []string) bool {
	builds := false
	buildConfigs := false

	for _, s := range resources {
		if s == "builds" {
			builds = true
		}
		if s == "buildConfigs" {
			buildConfigs = true
		}
	}

	return builds && buildConfigs
}
