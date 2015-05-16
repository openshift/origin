package v1beta1_test

import (
	"testing"

	knewer "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kolder "github.com/GoogleCloudPlatform/kubernetes/pkg/api/v1beta3"

	newer "github.com/openshift/origin/pkg/build/api"
	older "github.com/openshift/origin/pkg/build/api/v1beta1"
)

var Convert = knewer.Scheme.Convert

func TestSourceBuildStrategyOldToNewConversion(t *testing.T) {
	var actual newer.SourceBuildStrategy

	oldVersion := older.SourceBuildStrategy{
		BuilderImage: "testimage",
	}
	actual = newer.SourceBuildStrategy{}
	err := Convert(&oldVersion, &actual)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actual.From.Name != oldVersion.BuilderImage {
		t.Errorf("expected %v, actual %v", oldVersion.BuilderImage, actual.From.Name)
	}
	if actual.From.Kind != "DockerImage" {
		t.Errorf("expected %v, actual %v", "DockerImage", actual.From.Kind)
	}

	// default (ImageStream/ImageRepository) Kind
	oldVersion = older.SourceBuildStrategy{
		Clean: true,
		From: &kolder.ObjectReference{
			Name:      "name",
			Namespace: "namespace",
		},
		Tag: "tag",
	}
	actual = newer.SourceBuildStrategy{}
	err = Convert(&oldVersion, &actual)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actual.Incremental == oldVersion.Clean {
		t.Errorf("expected %v, actual %v", oldVersion.Clean, actual.Incremental)
	}
	if actual.From.Kind != "ImageStreamTag" {
		t.Errorf("expected %v, actual %v", "ImageStreamTag", actual.From.Kind)
	}
	if actual.From.Name != "name:tag" {
		t.Errorf("expected %v, actual %v", "name:tag", actual.From.Name)
	}
	if actual.From.Namespace != "namespace" {
		t.Errorf("expected %v, actual %v", "namespace", actual.From.Namespace)
	}

	// check Kind==ImageStream
	oldVersion = older.SourceBuildStrategy{
		From: &kolder.ObjectReference{
			Kind:      "ImageStream",
			Name:      "name",
			Namespace: "namespace",
		},
		Tag: "tag",
	}
	actual = newer.SourceBuildStrategy{}
	err = Convert(&oldVersion, &actual)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actual.From.Kind != "ImageStreamTag" {
		t.Errorf("expected %v, actual %v", "ImageStreamTag", actual.From.Kind)
	}
	if actual.From.Name != "name:tag" {
		t.Errorf("expected %v, actual %v", "name:tag", actual.From.Name)
	}
	if actual.From.Namespace != "namespace" {
		t.Errorf("expected %v, actual %v", "namespace", actual.From.Namespace)
	}

	// check Kind==ImageRepository
	oldVersion = older.SourceBuildStrategy{
		From: &kolder.ObjectReference{
			Kind:      "ImageRepository",
			Name:      "name",
			Namespace: "namespace",
		},
		Tag: "tag",
	}
	actual = newer.SourceBuildStrategy{}
	err = Convert(&oldVersion, &actual)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actual.From.Kind != "ImageStreamTag" {
		t.Errorf("expected %v, actual %v", "ImageStreamTag", actual.From.Kind)
	}
	if actual.From.Name != "name:tag" {
		t.Errorf("expected %v, actual %v", "name:tag", actual.From.Name)
	}
	if actual.From.Namespace != "namespace" {
		t.Errorf("expected %v, actual %v", "namespace", actual.From.Namespace)
	}

	// check default to tag to latest
	oldVersion = older.SourceBuildStrategy{
		From: &kolder.ObjectReference{
			Name:      "name",
			Namespace: "namespace",
		},
	}
	actual = newer.SourceBuildStrategy{}
	err = Convert(&oldVersion, &actual)
	if actual.From.Name != "name:latest" {
		t.Errorf("expected %v, actual %v", "name:latest", actual.From.Name)
	}
}

func TestSourceBuildStrategyNewToOldConversion(t *testing.T) {
	var actual older.SourceBuildStrategy

	newVersion := newer.SourceBuildStrategy{
		From: &knewer.ObjectReference{
			Kind:      "DockerImage",
			Name:      "name",
			Namespace: "namespace",
		},
	}
	actual = older.SourceBuildStrategy{}
	err := Convert(&newVersion, &actual)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actual.BuilderImage != newVersion.From.Name {
		t.Errorf("expected %v, actual %v", newVersion.From.Name, actual.BuilderImage)
	}
	if actual.Image != newVersion.From.Name {
		t.Errorf("expected %v, actual %v", newVersion.From.Name, actual.Image)
	}
	if actual.From != nil {
		t.Errorf("expected %v, actual %v", nil, actual.From.Kind)
	}

	// ImageStreamTag, convert to ImageStream+tag
	newVersion = newer.SourceBuildStrategy{
		From: &knewer.ObjectReference{
			Kind:      "ImageStreamTag",
			Name:      "name:tag",
			Namespace: "namespace",
		},
	}
	actual = older.SourceBuildStrategy{}
	err = Convert(&newVersion, &actual)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actual.From.Kind != "ImageStream" {
		t.Errorf("expected %v, actual %v", "", actual.From.Kind)
	}
	if actual.From.Name != "name" {
		t.Errorf("expected %v, actual %v", "name", actual.From.Name)
	}
	if actual.From.Namespace != "namespace" {
		t.Errorf("expected %v, actual %v", "namespace", actual.From.Namespace)
	}
	if actual.Tag != "tag" {
		t.Errorf("expected %v, actual %v", "tag", actual.Tag)
	}

	// ImageStreamImage, convert to ImageStream+tag
	newVersion = newer.SourceBuildStrategy{
		From: &knewer.ObjectReference{
			Kind:      "ImageStreamImage",
			Name:      "name@id",
			Namespace: "namespace",
		},
	}

	actual = older.SourceBuildStrategy{}
	err = Convert(&newVersion, &actual)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actual.From.Kind != "ImageStreamImage" {
		t.Errorf("expected %v, actual %v", "", actual.From.Kind)
	}
	if actual.From.Name != "name@id" {
		t.Errorf("expected %v, actual %v", "name", actual.From.Name)
	}
	if actual.From.Namespace != "namespace" {
		t.Errorf("expected %v, actual %v", "namespace", actual.From.Namespace)
	}
	if actual.Tag != "" {
		t.Errorf("expected |%v|, actual |%v|", "", actual.Tag)
	}

	// ImageStream, unchanged
	newVersion = newer.SourceBuildStrategy{
		From: &knewer.ObjectReference{
			Kind:      "ImageStream",
			Name:      "name",
			Namespace: "namespace",
		},
	}
	actual = older.SourceBuildStrategy{}
	err = Convert(&newVersion, &actual)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actual.From.Kind != "ImageStream" {
		t.Errorf("expected %v, actual %v", "", actual.From.Kind)
	}
	if actual.From.Name != "name" {
		t.Errorf("expected %v, actual %v", "name", actual.From.Name)
	}
	if actual.From.Namespace != "namespace" {
		t.Errorf("expected %v, actual %v", "namespace", actual.From.Namespace)
	}
	if actual.Tag != "" {
		t.Errorf("expected %v, actual %v", "tag", actual.Tag)
	}

	// ImageStream(default), unchanged
	newVersion = newer.SourceBuildStrategy{
		From: &knewer.ObjectReference{
			Name:      "name",
			Namespace: "namespace",
		},
	}
	actual = older.SourceBuildStrategy{}
	err = Convert(&newVersion, &actual)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actual.From.Kind != "ImageStream" {
		t.Errorf("expected %v, actual %v", "", actual.From.Kind)
	}
	if actual.From.Name != "name" {
		t.Errorf("expected %v, actual %v", "name", actual.From.Name)
	}
	if actual.From.Namespace != "namespace" {
		t.Errorf("expected %v, actual %v", "namespace", actual.From.Namespace)
	}
	if actual.Tag != "" {
		t.Errorf("expected %v, actual %v", "tag", actual.Tag)
	}

}

func TestDockerBuildStrategyOldToNewConversion(t *testing.T) {
	var actual newer.DockerBuildStrategy
	oldVersion := older.DockerBuildStrategy{
		NoCache: true,
	}
	err := Convert(&oldVersion, &actual)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actual.NoCache != oldVersion.NoCache {
		t.Errorf("expected %v, actual %v", oldVersion.NoCache, actual.NoCache)
	}
	if actual.From != nil {
		t.Errorf("expected %v, actual %v", nil, actual.From)
	}

	oldVersion = older.DockerBuildStrategy{
		BaseImage: "testimage",
	}
	actual = newer.DockerBuildStrategy{}
	err = Convert(&oldVersion, &actual)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actual.From.Name != oldVersion.BaseImage {
		t.Errorf("expected %v, actual %v", oldVersion.BaseImage, actual.From.Name)
	}
	if actual.From.Kind != "DockerImage" {
		t.Errorf("expected %v, actual %v", "DockerImage", actual.From.Kind)
	}

	// default (ImageStream/ImageRepository) Kind
	oldVersion = older.DockerBuildStrategy{
		From: &kolder.ObjectReference{
			Name:      "name",
			Namespace: "namespace",
		},
		Tag: "tag",
	}
	actual = newer.DockerBuildStrategy{}
	err = Convert(&oldVersion, &actual)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actual.From.Kind != "ImageStreamTag" {
		t.Errorf("expected %v, actual %v", "ImageStreamTag", actual.From.Kind)
	}
	if actual.From.Name != "name:tag" {
		t.Errorf("expected %v, actual %v", "name:tag", actual.From.Name)
	}
	if actual.From.Namespace != "namespace" {
		t.Errorf("expected %v, actual %v", "namespace", actual.From.Namespace)
	}

	// check Kind==ImageStream
	oldVersion = older.DockerBuildStrategy{
		From: &kolder.ObjectReference{
			Kind:      "ImageStream",
			Name:      "name",
			Namespace: "namespace",
		},
		Tag: "tag",
	}
	actual = newer.DockerBuildStrategy{}
	err = Convert(&oldVersion, &actual)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actual.From.Kind != "ImageStreamTag" {
		t.Errorf("expected %v, actual %v", "ImageStreamTag", actual.From.Kind)
	}
	if actual.From.Name != "name:tag" {
		t.Errorf("expected %v, actual %v", "name:tag", actual.From.Name)
	}
	if actual.From.Namespace != "namespace" {
		t.Errorf("expected %v, actual %v", "namespace", actual.From.Namespace)
	}

	// check Kind==ImageRepository
	oldVersion = older.DockerBuildStrategy{
		From: &kolder.ObjectReference{
			Kind:      "ImageRepository",
			Name:      "name",
			Namespace: "namespace",
		},
		Tag: "tag",
	}
	actual = newer.DockerBuildStrategy{}
	err = Convert(&oldVersion, &actual)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actual.From.Kind != "ImageStreamTag" {
		t.Errorf("expected %v, actual %v", "ImageStreamTag", actual.From.Kind)
	}
	if actual.From.Name != "name:tag" {
		t.Errorf("expected %v, actual %v", "name:tag", actual.From.Name)
	}
	if actual.From.Namespace != "namespace" {
		t.Errorf("expected %v, actual %v", "namespace", actual.From.Namespace)
	}

	// check default to tag to latest
	oldVersion = older.DockerBuildStrategy{
		From: &kolder.ObjectReference{
			Name:      "name",
			Namespace: "namespace",
		},
	}
	actual = newer.DockerBuildStrategy{}
	err = Convert(&oldVersion, &actual)
	if actual.From.Name != "name:latest" {
		t.Errorf("expected %v, actual %v", "name:latest", actual.From.Name)
	}
}

func TestDockerBuildStrategyNewToOldConversion(t *testing.T) {
	var actual older.DockerBuildStrategy

	newVersion := newer.DockerBuildStrategy{
		From: &knewer.ObjectReference{
			Kind:      "DockerImage",
			Name:      "name",
			Namespace: "namespace",
		},
	}
	actual = older.DockerBuildStrategy{}
	err := Convert(&newVersion, &actual)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actual.BaseImage != newVersion.From.Name {
		t.Errorf("expected %v, actual %v", newVersion.From.Name, actual.BaseImage)
	}
	if actual.Image != newVersion.From.Name {
		t.Errorf("expected %v, actual %v", newVersion.From.Name, actual.Image)
	}
	if actual.From != nil {
		t.Errorf("expected %v, actual %v", nil, actual.From.Kind)
	}

	// ImageStreamTag, convert to ImageStream+tag
	newVersion = newer.DockerBuildStrategy{
		From: &knewer.ObjectReference{
			Kind:      "ImageStreamTag",
			Name:      "name:tag",
			Namespace: "namespace",
		},
	}
	actual = older.DockerBuildStrategy{}
	err = Convert(&newVersion, &actual)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actual.From.Kind != "ImageStream" {
		t.Errorf("expected %v, actual %v", "", actual.From.Kind)
	}
	if actual.From.Name != "name" {
		t.Errorf("expected %v, actual %v", "name", actual.From.Name)
	}
	if actual.From.Namespace != "namespace" {
		t.Errorf("expected %v, actual %v", "namespace", actual.From.Namespace)
	}
	if actual.Tag != "tag" {
		t.Errorf("expected %v, actual %v", "tag", actual.Tag)
	}

	// ImageStreamImage, convert to ImageStream+tag
	newVersion = newer.DockerBuildStrategy{
		From: &knewer.ObjectReference{
			Kind:      "ImageStreamImage",
			Name:      "name@id",
			Namespace: "namespace",
		},
	}

	actual = older.DockerBuildStrategy{}
	err = Convert(&newVersion, &actual)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actual.From.Kind != "ImageStreamImage" {
		t.Errorf("expected %v, actual %v", "", actual.From.Kind)
	}
	if actual.From.Name != "name@id" {
		t.Errorf("expected %v, actual %v", "name", actual.From.Name)
	}
	if actual.From.Namespace != "namespace" {
		t.Errorf("expected %v, actual %v", "namespace", actual.From.Namespace)
	}
	if actual.Tag != "" {
		t.Errorf("expected |%v|, actual |%v|", "", actual.Tag)
	}

	// ImageStream, unchanged
	newVersion = newer.DockerBuildStrategy{
		From: &knewer.ObjectReference{
			Kind:      "ImageStream",
			Name:      "name",
			Namespace: "namespace",
		},
	}
	actual = older.DockerBuildStrategy{}
	err = Convert(&newVersion, &actual)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actual.From.Kind != "ImageStream" {
		t.Errorf("expected %v, actual %v", "", actual.From.Kind)
	}
	if actual.From.Name != "name" {
		t.Errorf("expected %v, actual %v", "name", actual.From.Name)
	}
	if actual.From.Namespace != "namespace" {
		t.Errorf("expected %v, actual %v", "namespace", actual.From.Namespace)
	}
	if actual.Tag != "" {
		t.Errorf("expected %v, actual %v", "tag", actual.Tag)
	}

	// ImageStream(default), unchanged
	newVersion = newer.DockerBuildStrategy{
		From: &knewer.ObjectReference{
			Name:      "name",
			Namespace: "namespace",
		},
	}
	actual = older.DockerBuildStrategy{}
	err = Convert(&newVersion, &actual)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actual.From.Kind != "ImageStream" {
		t.Errorf("expected %v, actual %v", "", actual.From.Kind)
	}
	if actual.From.Name != "name" {
		t.Errorf("expected %v, actual %v", "name", actual.From.Name)
	}
	if actual.From.Namespace != "namespace" {
		t.Errorf("expected %v, actual %v", "namespace", actual.From.Namespace)
	}
	if actual.Tag != "" {
		t.Errorf("expected %v, actual %v", "tag", actual.Tag)
	}

}

func TestCustomBuildStrategyOldToNewConversion(t *testing.T) {
	var actual newer.CustomBuildStrategy
	oldVersion := older.CustomBuildStrategy{
		ExposeDockerSocket: true,
	}
	err := Convert(&oldVersion, &actual)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actual.ExposeDockerSocket != oldVersion.ExposeDockerSocket {
		t.Errorf("expected %v, actual %v", oldVersion.ExposeDockerSocket, actual.ExposeDockerSocket)
	}
	if actual.From != nil {
		t.Errorf("expected %v, actual %v", nil, actual.From)
	}

	oldVersion = older.CustomBuildStrategy{
		Image: "testimage",
	}
	actual = newer.CustomBuildStrategy{}
	err = Convert(&oldVersion, &actual)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actual.From.Name != oldVersion.Image {
		t.Errorf("expected %v, actual %v", oldVersion.Image, actual.From.Name)
	}
	if actual.From.Kind != "DockerImage" {
		t.Errorf("expected %v, actual %v", "DockerImage", actual.From.Kind)
	}

	// default (ImageStream/ImageRepository) Kind
	oldVersion = older.CustomBuildStrategy{
		From: &kolder.ObjectReference{
			Name:      "name",
			Namespace: "namespace",
		},
		Tag: "tag",
	}
	actual = newer.CustomBuildStrategy{}
	err = Convert(&oldVersion, &actual)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actual.From.Kind != "ImageStreamTag" {
		t.Errorf("expected %v, actual %v", "ImageStreamTag", actual.From.Kind)
	}
	if actual.From.Name != "name:tag" {
		t.Errorf("expected %v, actual %v", "name:tag", actual.From.Name)
	}
	if actual.From.Namespace != "namespace" {
		t.Errorf("expected %v, actual %v", "namespace", actual.From.Namespace)
	}

	// check Kind==ImageStream
	oldVersion = older.CustomBuildStrategy{
		From: &kolder.ObjectReference{
			Kind:      "ImageStream",
			Name:      "name",
			Namespace: "namespace",
		},
		Tag: "tag",
	}
	actual = newer.CustomBuildStrategy{}
	err = Convert(&oldVersion, &actual)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actual.From.Kind != "ImageStreamTag" {
		t.Errorf("expected %v, actual %v", "ImageStreamTag", actual.From.Kind)
	}
	if actual.From.Name != "name:tag" {
		t.Errorf("expected %v, actual %v", "name:tag", actual.From.Name)
	}
	if actual.From.Namespace != "namespace" {
		t.Errorf("expected %v, actual %v", "namespace", actual.From.Namespace)
	}

	// check Kind==ImageRepository
	oldVersion = older.CustomBuildStrategy{
		From: &kolder.ObjectReference{
			Kind:      "ImageRepository",
			Name:      "name",
			Namespace: "namespace",
		},
		Tag: "tag",
	}
	actual = newer.CustomBuildStrategy{}
	err = Convert(&oldVersion, &actual)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actual.From.Kind != "ImageStreamTag" {
		t.Errorf("expected %v, actual %v", "ImageStreamTag", actual.From.Kind)
	}
	if actual.From.Name != "name:tag" {
		t.Errorf("expected %v, actual %v", "name:tag", actual.From.Name)
	}
	if actual.From.Namespace != "namespace" {
		t.Errorf("expected %v, actual %v", "namespace", actual.From.Namespace)
	}

	// check default to tag to latest
	oldVersion = older.CustomBuildStrategy{
		From: &kolder.ObjectReference{
			Name:      "name",
			Namespace: "namespace",
		},
	}
	actual = newer.CustomBuildStrategy{}
	err = Convert(&oldVersion, &actual)
	if actual.From.Name != "name:latest" {
		t.Errorf("expected %v, actual %v", "name:latest", actual.From.Name)
	}
}

func TestCustomBuildStrategyNewToOldConversion(t *testing.T) {
	var actual older.CustomBuildStrategy

	newVersion := newer.CustomBuildStrategy{
		From: &knewer.ObjectReference{
			Kind:      "DockerImage",
			Name:      "name",
			Namespace: "namespace",
		},
	}
	actual = older.CustomBuildStrategy{}
	err := Convert(&newVersion, &actual)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actual.Image != newVersion.From.Name {
		t.Errorf("expected %v, actual %v", newVersion.From.Name, actual.Image)
	}
	if actual.From != nil {
		t.Errorf("expected %v, actual %v", nil, actual.From)
	}

	// ImageStreamTag, convert to ImageStream+tag
	newVersion = newer.CustomBuildStrategy{
		From: &knewer.ObjectReference{
			Kind:      "ImageStreamTag",
			Name:      "name:tag",
			Namespace: "namespace",
		},
	}
	actual = older.CustomBuildStrategy{}
	err = Convert(&newVersion, &actual)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actual.From.Kind != "ImageStream" {
		t.Errorf("expected %v, actual %v", "", actual.From.Kind)
	}
	if actual.From.Name != "name" {
		t.Errorf("expected %v, actual %v", "name", actual.From.Name)
	}
	if actual.From.Namespace != "namespace" {
		t.Errorf("expected %v, actual %v", "namespace", actual.From.Namespace)
	}
	if actual.Tag != "tag" {
		t.Errorf("expected %v, actual %v", "tag", actual.Tag)
	}

	// ImageStreamImage, convert to ImageStream+tag
	newVersion = newer.CustomBuildStrategy{
		From: &knewer.ObjectReference{
			Kind:      "ImageStreamImage",
			Name:      "name@id",
			Namespace: "namespace",
		},
	}

	actual = older.CustomBuildStrategy{}
	err = Convert(&newVersion, &actual)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actual.From.Kind != "ImageStreamImage" {
		t.Errorf("expected %v, actual %v", "", actual.From.Kind)
	}
	if actual.From.Name != "name@id" {
		t.Errorf("expected %v, actual %v", "name", actual.From.Name)
	}
	if actual.From.Namespace != "namespace" {
		t.Errorf("expected %v, actual %v", "namespace", actual.From.Namespace)
	}
	if actual.Tag != "" {
		t.Errorf("expected |%v|, actual |%v|", "", actual.Tag)
	}

	// ImageStream, unchanged
	newVersion = newer.CustomBuildStrategy{
		From: &knewer.ObjectReference{
			Kind:      "ImageStream",
			Name:      "name",
			Namespace: "namespace",
		},
	}
	actual = older.CustomBuildStrategy{}
	err = Convert(&newVersion, &actual)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actual.From.Kind != "ImageStream" {
		t.Errorf("expected %v, actual %v", "", actual.From.Kind)
	}
	if actual.From.Name != "name" {
		t.Errorf("expected %v, actual %v", "name", actual.From.Name)
	}
	if actual.From.Namespace != "namespace" {
		t.Errorf("expected %v, actual %v", "namespace", actual.From.Namespace)
	}
	if actual.Tag != "" {
		t.Errorf("expected %v, actual %v", "tag", actual.Tag)
	}

	// ImageStream(default), unchanged
	newVersion = newer.CustomBuildStrategy{
		From: &knewer.ObjectReference{
			Name:      "name",
			Namespace: "namespace",
		},
	}
	actual = older.CustomBuildStrategy{}
	err = Convert(&newVersion, &actual)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actual.From.Kind != "ImageStream" {
		t.Errorf("expected %v, actual %v", "", actual.From.Kind)
	}
	if actual.From.Name != "name" {
		t.Errorf("expected %v, actual %v", "name", actual.From.Name)
	}
	if actual.From.Namespace != "namespace" {
		t.Errorf("expected %v, actual %v", "namespace", actual.From.Namespace)
	}
	if actual.Tag != "" {
		t.Errorf("expected %v, actual %v", "tag", actual.Tag)
	}

}

func TestContextDirConversion(t *testing.T) {
	var actual newer.BuildParameters
	oldVersion := older.BuildParameters{
		Strategy: older.BuildStrategy{
			Type: older.DockerBuildStrategyType,
			DockerStrategy: &older.DockerBuildStrategy{
				ContextDir: "contextDir",
			},
		},
	}
	err := Convert(&oldVersion, &actual)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actual.Source.ContextDir != oldVersion.Strategy.DockerStrategy.ContextDir {
		t.Errorf("expected %v, actual %v", oldVersion.Strategy.DockerStrategy.ContextDir, actual.Source.ContextDir)
	}
}
