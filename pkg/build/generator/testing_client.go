package generator

import (
	"context"

	buildv1 "github.com/openshift/api/build/v1"
	imagev1 "github.com/openshift/api/image/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestingClient is an implementation of the GeneratorClient interface
type TestingClient struct {
	GetBuildConfigFunc      func(ctx context.Context, name string, options *metav1.GetOptions) (*buildv1.BuildConfig, error)
	UpdateBuildConfigFunc   func(ctx context.Context, buildConfig *buildv1.BuildConfig) error
	GetBuildFunc            func(ctx context.Context, name string, options *metav1.GetOptions) (*buildv1.Build, error)
	CreateBuildFunc         func(ctx context.Context, build *buildv1.Build) error
	UpdateBuildFunc         func(ctx context.Context, build *buildv1.Build) error
	GetImageStreamFunc      func(ctx context.Context, name string, options *metav1.GetOptions) (*imagev1.ImageStream, error)
	GetImageStreamImageFunc func(ctx context.Context, name string, options *metav1.GetOptions) (*imagev1.ImageStreamImage, error)
	GetImageStreamTagFunc   func(ctx context.Context, name string, options *metav1.GetOptions) (*imagev1.ImageStreamTag, error)
}

// GetBuildConfig retrieves a named build config
func (c TestingClient) GetBuildConfig(ctx context.Context, name string, options *metav1.GetOptions) (*buildv1.BuildConfig, error) {
	return c.GetBuildConfigFunc(ctx, name, options)
}

// UpdateBuildConfig updates a named build config
func (c TestingClient) UpdateBuildConfig(ctx context.Context, buildConfig *buildv1.BuildConfig) error {
	return c.UpdateBuildConfigFunc(ctx, buildConfig)
}

// GetBuild retrieves a build
func (c TestingClient) GetBuild(ctx context.Context, name string, options *metav1.GetOptions) (*buildv1.Build, error) {
	return c.GetBuildFunc(ctx, name, options)
}

// CreateBuild creates a new build
func (c TestingClient) CreateBuild(ctx context.Context, build *buildv1.Build) error {
	return c.CreateBuildFunc(ctx, build)
}

// UpdateBuild updates a build
func (c TestingClient) UpdateBuild(ctx context.Context, build *buildv1.Build) error {
	return c.UpdateBuildFunc(ctx, build)
}

// GetImageStream retrieves a named image stream
func (c TestingClient) GetImageStream(ctx context.Context, name string, options *metav1.GetOptions) (*imagev1.ImageStream, error) {
	return c.GetImageStreamFunc(ctx, name, options)
}

// GetImageStreamImage retrieves an image stream image
func (c TestingClient) GetImageStreamImage(ctx context.Context, name string, options *metav1.GetOptions) (*imagev1.ImageStreamImage, error) {
	return c.GetImageStreamImageFunc(ctx, name, options)
}

// GetImageStreamTag retrieves and image stream tag
func (c TestingClient) GetImageStreamTag(ctx context.Context, name string, options *metav1.GetOptions) (*imagev1.ImageStreamTag, error) {
	return c.GetImageStreamTagFunc(ctx, name, options)
}
