package test

import (
	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	buildlister "github.com/openshift/origin/pkg/build/generated/listers/build/internalversion"
	"k8s.io/apimachinery/pkg/labels"
)

type FakeBuildConfigIndex struct {
	Build *buildapi.BuildConfig
	Err   error
}

func NewFakeBuildConfigIndex(build *buildapi.BuildConfig) buildlister.BuildConfigLister {
	return &FakeBuildConfigIndex{Build: build}
}

func (i *FakeBuildConfigIndex) List(label labels.Selector) ([]*buildapi.BuildConfig, error) {
	return []*buildapi.BuildConfig{i.Build}, nil
}

func (i *FakeBuildConfigIndex) BuildConfigs(ns string) buildlister.BuildConfigNamespaceLister {
	return i
}

func (i *FakeBuildConfigIndex) Get(name string) (*buildapi.BuildConfig, error) {
	return i.Build, nil
}
