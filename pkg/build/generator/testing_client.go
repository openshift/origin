package generator

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
)

// TestingClient is an implementation of the GeneratorClient interface
type TestingClient struct {
	GetBuildConfigFunc      func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*buildapi.BuildConfig, error)
	UpdateBuildConfigFunc   func(ctx apirequest.Context, buildConfig *buildapi.BuildConfig) error
	GetBuildFunc            func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*buildapi.Build, error)
	CreateBuildFunc         func(ctx apirequest.Context, build *buildapi.Build) error
	UpdateBuildFunc         func(ctx apirequest.Context, build *buildapi.Build) error
	GetImageStreamFunc      func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*imageapi.ImageStream, error)
	GetImageStreamImageFunc func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*imageapi.ImageStreamImage, error)
	GetImageStreamTagFunc   func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*imageapi.ImageStreamTag, error)
}

// GetBuildConfig retrieves a named build config
func (c TestingClient) GetBuildConfig(ctx apirequest.Context, name string, options *metav1.GetOptions) (*buildapi.BuildConfig, error) {
	return c.GetBuildConfigFunc(ctx, name, options)
}

// UpdateBuildConfig updates a named build config
func (c TestingClient) UpdateBuildConfig(ctx apirequest.Context, buildConfig *buildapi.BuildConfig) error {
	return c.UpdateBuildConfigFunc(ctx, buildConfig)
}

// GetBuild retrieves a build
func (c TestingClient) GetBuild(ctx apirequest.Context, name string, options *metav1.GetOptions) (*buildapi.Build, error) {
	return c.GetBuildFunc(ctx, name, options)
}

// CreateBuild creates a new build
func (c TestingClient) CreateBuild(ctx apirequest.Context, build *buildapi.Build) error {
	return c.CreateBuildFunc(ctx, build)
}

// UpdateBuild updates a build
func (c TestingClient) UpdateBuild(ctx apirequest.Context, build *buildapi.Build) error {
	return c.UpdateBuildFunc(ctx, build)
}

// GetImageStream retrieves a named image stream
func (c TestingClient) GetImageStream(ctx apirequest.Context, name string, options *metav1.GetOptions) (*imageapi.ImageStream, error) {
	return c.GetImageStreamFunc(ctx, name, options)
}

// GetImageStreamImage retrieves an image stream image
func (c TestingClient) GetImageStreamImage(ctx apirequest.Context, name string, options *metav1.GetOptions) (*imageapi.ImageStreamImage, error) {
	return c.GetImageStreamImageFunc(ctx, name, options)
}

// GetImageStreamTag retrieves and image stream tag
func (c TestingClient) GetImageStreamTag(ctx apirequest.Context, name string, options *metav1.GetOptions) (*imageapi.ImageStreamTag, error) {
	return c.GetImageStreamTagFunc(ctx, name, options)
}
