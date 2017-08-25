package internalversion

import (
	apps "github.com/openshift/origin/pkg/deploy/apis/apps"
)

type DeploymentConfigExpansion interface {
	Instantiate(request *apps.DeploymentRequest) (*apps.DeploymentConfig, error)
}

// Instantiate instantiates a new build from build config returning new object or an error
func (c *deploymentConfigs) Instantiate(request *apps.DeploymentRequest) (*apps.DeploymentConfig, error) {
	result := &apps.DeploymentConfig{}
	resp := c.client.Post().Namespace(c.ns).Resource("deploymentConfigs").Name(request.Name).SubResource("instantiate").Body(request).Do()
	var statusCode int
	if resp.StatusCode(&statusCode); statusCode == 204 {
		return nil, nil
	}
	err := resp.Into(result)
	return result, err
}
