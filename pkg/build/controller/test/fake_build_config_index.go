package test

import (
	buildapi "github.com/openshift/origin/pkg/build/api"
	oscache "github.com/openshift/origin/pkg/client/cache"
)

type FakeBuildConfigIndex struct {
	Build *buildapi.BuildConfig
	Err   error
}

func NewFakeBuildConfigIndex(build *buildapi.BuildConfig) oscache.StoreToBuildConfigLister {
	return &FakeBuildConfigIndex{Build: build}
}

func (i *FakeBuildConfigIndex) List() ([]*buildapi.BuildConfig, error) {
	return []*buildapi.BuildConfig{i.Build}, nil
}

func (i *FakeBuildConfigIndex) GetConfigsForImageStreamTrigger(namespace, name string) ([]*buildapi.BuildConfig, error) {
	return []*buildapi.BuildConfig{i.Build}, nil
}
