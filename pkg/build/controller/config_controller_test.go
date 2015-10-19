package controller

import (
	"fmt"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"

	buildapi "github.com/openshift/origin/pkg/build/api"
)

func TestHandleBuildConfig(t *testing.T) {
	tests := []struct {
		name              string
		bc                *buildapi.BuildConfig
		expectBuild       bool
		withBuilds        bool
		failFirstDelete   bool
		instantiatorError bool
		expectErr         bool
	}{
		{
			name:        "build config with no config change trigger",
			bc:          baseBuildConfig(),
			expectBuild: false,
		},
		{
			name:        "build config with non-zero last version",
			bc:          buildConfigWithNonZeroLastVersion(),
			expectBuild: false,
		},
		{
			name:        "build config with config change trigger",
			bc:          buildConfigWithConfigChangeTrigger(),
			expectBuild: true,
		},
		{
			name:              "instantiator error",
			bc:                buildConfigWithConfigChangeTrigger(),
			instantiatorError: true,
			expectErr:         true,
		},
		{
			name:              "build config with non-zero deletion timestamp",
			bc:                buildConfigWithNonZeroDeletionTimestamp(),
			instantiatorError: true,
		},
		{
			name:              "build config with non-zero deletion timestamp with builds",
			bc:                buildConfigWithNonZeroDeletionTimestamp(),
			instantiatorError: true,
			withBuilds:        true,
		},
		{
			name:              "build config with non-zero deletion timestamp with builds and first deletion failure",
			bc:                buildConfigWithNonZeroDeletionTimestamp(),
			instantiatorError: true,
			withBuilds:        true,
			failFirstDelete:   true,
			expectErr:         true,
		},
	}

	for _, tc := range tests {
		client := &testBuildConfigClient{
			err: tc.instantiatorError,
		}
		buildClient := &testBuildClient{
			withBuilds:      tc.withBuilds,
			failFirstDelete: tc.failFirstDelete,
		}
		controller := &BuildConfigController{
			BuildConfigInstantiator: client,
			BuildConfigUpdater:      client,
			BuildConfigDeleter:      client,
			BuildDeleter:            buildClient,
			BuildLister:             buildClient,
		}
		err := controller.HandleBuildConfig(tc.bc)
		if err != nil {
			if !tc.expectErr {
				t.Errorf("%s: unexpected error: %v", tc.name, err)
			}
			continue
		}
		if err == nil && tc.expectErr {
			t.Errorf("%s: expected error, but got none", tc.name)
			continue
		}
		if tc.expectBuild && len(client.requestName) == 0 {
			t.Errorf("%s: expected a build to be started.", tc.name)
		}
		if !tc.expectBuild && len(client.requestName) > 0 {
			t.Errorf("%s: did not expect a build to be started.", tc.name)
		}
		if tc.withBuilds && buildClient.deleteCount == 0 {
			t.Errorf("%s: buildConfig had builds, but delete was not called", tc.name)
		}
		if !tc.withBuilds && buildClient.deleteCount > 0 {
			t.Errorf("%s: delete was called even though there are no builds", tc.name)
		}
		if tc.failFirstDelete {
			if buildClient.deleteCount != 1 {
				t.Errorf("%s: delete was called even though there are no builds", tc.name)
				continue
			}
			err := controller.HandleBuildConfig(tc.bc)
			if err != nil {
				t.Errorf("%s: unexpected error on second try", tc.name)
			}
		}
	}

}

type testBuildConfigClient struct {
	requestName string
	err         bool
}

func (c *testBuildConfigClient) Instantiate(namespace string, request *buildapi.BuildRequest) (*buildapi.Build, error) {
	c.requestName = request.Name
	if c.err {
		return nil, fmt.Errorf("error")
	}
	return &buildapi.Build{}, nil
}

func (c *testBuildConfigClient) Update(buildConfig *buildapi.BuildConfig) error {
	return nil
}

func (c *testBuildConfigClient) Delete(namespace, name string) error {
	return nil
}

type testBuildClient struct {
	withBuilds      bool
	failFirstDelete bool
	deleteCount     int
}

func (c *testBuildClient) Delete(namespace, name string) error {
	c.deleteCount++
	if c.failFirstDelete && c.deleteCount == 1 {
		return fmt.Errorf("planned error")
	}
	return nil
}

func (c *testBuildClient) List(namespace string, label labels.Selector, field fields.Selector) (*buildapi.BuildList, error) {
	items := []buildapi.Build{}
	if c.withBuilds {
		items = append(items, buildapi.Build{
			ObjectMeta: kapi.ObjectMeta{
				Name:      "build",
				Namespace: "ns",
			},
		})
	}
	builds := &buildapi.BuildList{
		Items: items,
	}
	return builds, nil
}

func baseBuildConfig() *buildapi.BuildConfig {
	bc := &buildapi.BuildConfig{}
	bc.Name = "testBuildConfig"
	bc.Spec.BuildSpec.Strategy.Type = buildapi.SourceBuildStrategyType
	bc.Spec.BuildSpec.Strategy.SourceStrategy = &buildapi.SourceBuildStrategy{}
	bc.Spec.BuildSpec.Strategy.SourceStrategy.From.Name = "builderimage:latest"
	bc.Spec.BuildSpec.Strategy.SourceStrategy.From.Kind = "ImageStreamTag"
	return bc
}

func buildConfigWithConfigChangeTrigger() *buildapi.BuildConfig {
	bc := baseBuildConfig()
	configChangeTrigger := buildapi.BuildTriggerPolicy{}
	configChangeTrigger.Type = buildapi.ConfigChangeBuildTriggerType
	bc.Spec.Triggers = append(bc.Spec.Triggers, configChangeTrigger)
	return bc
}

func buildConfigWithNonZeroLastVersion() *buildapi.BuildConfig {
	bc := buildConfigWithConfigChangeTrigger()
	bc.Status.LastVersion = 1
	return bc
}

func buildConfigWithNonZeroDeletionTimestamp() *buildapi.BuildConfig {
	bc := baseBuildConfig()
	now := unversioned.Now()
	bc.DeletionTimestamp = &now
	return bc
}
