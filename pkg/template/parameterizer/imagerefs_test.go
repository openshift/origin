package parameterizer

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/runtime"
	kapi "k8s.io/kubernetes/pkg/api"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	deployapi "github.com/openshift/origin/pkg/deploy/apis/apps"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
)

func TestMakeValidParameterName(t *testing.T) {
	tests := []struct {
		s string
		e string
	}{
		{
			s: "blah&blah1blah",
			e: "BLAH_BLAH1BLAH",
		},
		{
			s: "value*&*(test",
			e: "VALUE____TEST",
		},
		{
			s: "ABC",
			e: "ABC",
		},
		{
			s: "abc123",
			e: "ABC123",
		},
	}

	for _, test := range tests {
		actual := makeValidParameterName(test.s)
		if actual != test.e {
			t.Errorf("Expected: %q, Got: %q", test.e, actual)
		}
	}
}
func TestImageStreamTagParamName(t *testing.T) {
	name := "my-ruby:v1.0"
	expect := "MY_RUBY_IMAGE_STREAM_TAG"
	actual := imageStreamTagParamName(name)
	if actual != expect {
		t.Errorf("Expected: %q. Got %q", expect, actual)
	}
}
func TestImageStreamImageParamName(t *testing.T) {
	name := "test-1234@sha256:14f916134cf58676196409553c139f00f4e0c55bc92d364346c96ea1ef1ea825"
	expect := "TEST_1234_IMAGE_STREAM_IMAGE"
	actual := imageStreamImageParamName(name)
	if actual != expect {
		t.Errorf("Expected: %q. Got %q", expect, actual)
	}
}
func TestDockerImageRefParamName(t *testing.T) {
	name := "docker.io/openshift/origin:v3.6.0_alpha.1"
	expect := "ORIGIN_IMAGE_REF"
	actual := dockerImageRefParamName(name)
	if actual != expect {
		t.Errorf("Expected: %q. Got %q", expect, actual)
	}
}
func TestNamespaceParamName(t *testing.T) {
	name := "my-ruby:v1.0"
	expect := "MY_RUBY_IMAGE_STREAM_TAG_NS"
	actual := namespaceParamName(imageStreamTagParamName(name))
	if actual != expect {
		t.Errorf("Expected: %q. Got %q", expect, actual)
	}
}
func TestFormatParameter(t *testing.T) {
	name := "MY_PARAM"
	expect := "${MY_PARAM}"
	actual := formatParameter(name)
	if actual != expect {
		t.Errorf("Expected: %q. Got %q", expect, actual)
	}
}

type buildConfigBuilder buildapi.BuildConfig

func (bc *buildConfigBuilder) bc() *buildapi.BuildConfig {
	return (*buildapi.BuildConfig)(bc)
}

func objectRef(name, kind, namespace string) *kapi.ObjectReference {
	return &kapi.ObjectReference{
		Name:      name,
		Kind:      kind,
		Namespace: namespace,
	}
}

func (bc *buildConfigBuilder) withImageTrigger(name, kind, namespace string) *buildConfigBuilder {
	var ref *kapi.ObjectReference
	if len(name) > 0 {
		ref = objectRef(name, kind, namespace)
	}
	bc.Spec.Triggers = append(bc.Spec.Triggers, buildapi.BuildTriggerPolicy{
		Type: buildapi.ImageChangeBuildTriggerType,
		ImageChange: &buildapi.ImageChangeTrigger{
			From: ref,
		},
	})
	return bc
}

func (bc *buildConfigBuilder) withDockerStrategyFrom(name, kind, namespace string) *buildConfigBuilder {
	bc.Spec.CommonSpec.Strategy.DockerStrategy = &buildapi.DockerBuildStrategy{
		From: objectRef(name, kind, namespace),
	}
	return bc
}

func (bc *buildConfigBuilder) withSourceStrategyFrom(name, kind, namespace string) *buildConfigBuilder {
	ref := objectRef(name, kind, namespace)
	bc.Spec.CommonSpec.Strategy.SourceStrategy = &buildapi.SourceBuildStrategy{
		From: *ref,
	}
	return bc
}

func (bc *buildConfigBuilder) withCustomStrategyFrom(name, kind, namespace string) *buildConfigBuilder {
	ref := objectRef(name, kind, namespace)
	bc.Spec.CommonSpec.Strategy.CustomStrategy = &buildapi.CustomBuildStrategy{
		From: *ref,
	}
	return bc
}

func (bc *buildConfigBuilder) withOutputTo(name, kind, namespace string) *buildConfigBuilder {
	bc.Spec.CommonSpec.Output.To = objectRef(name, kind, namespace)
	return bc
}

func (bc *buildConfigBuilder) withImageSourceFrom(name, kind, namespace string) *buildConfigBuilder {
	ref := objectRef(name, kind, namespace)
	bc.Spec.CommonSpec.Source.Images = append(bc.Spec.CommonSpec.Source.Images, buildapi.ImageSource{
		From: *ref,
	})
	return bc
}

type deploymentConfigBuilder deployapi.DeploymentConfig

func (dc *deploymentConfigBuilder) dc() *deployapi.DeploymentConfig {
	return (*deployapi.DeploymentConfig)(dc)
}

func (dc *deploymentConfigBuilder) withImageTrigger(name, kind, namespace string, containers []string) *deploymentConfigBuilder {
	ref := objectRef(name, kind, namespace)
	dc.Spec.Triggers = append(dc.Spec.Triggers, deployapi.DeploymentTriggerPolicy{
		Type: deployapi.DeploymentTriggerOnImageChange,
		ImageChangeParams: &deployapi.DeploymentTriggerImageChangeParams{
			ContainerNames: containers,
			From:           *ref,
		},
	})
	return dc
}

func (dc *deploymentConfigBuilder) withContainerImageRef(container, ref string) *deploymentConfigBuilder {
	if dc.Spec.Template == nil {
		dc.Spec.Template = &kapi.PodTemplateSpec{}
	}
	dc.Spec.Template.Spec.Containers = append(dc.Spec.Template.Spec.Containers, kapi.Container{
		Name:  container,
		Image: ref,
	})
	return dc
}

func buildConfig(name string) *buildConfigBuilder {
	bc := (*buildConfigBuilder)(&buildapi.BuildConfig{})
	bc.Name = name
	return bc
}

func deploymentConfig(name string) *deploymentConfigBuilder {
	dc := (*deploymentConfigBuilder)(&deployapi.DeploymentConfig{})
	dc.Name = name
	return dc
}

func imageStream(name string) *imageapi.ImageStream {
	is := &imageapi.ImageStream{}
	is.Name = name
	return is
}

func asJSON(obj runtime.Object) string {
	str, err := json.Marshal(obj)
	if err != nil {
		return fmt.Sprintf("%#v", obj)
	}
	return string(str)
}
func TestIncludesImageStream(t *testing.T) {
	objs := []runtime.Object{
		buildConfig("test1").bc(),
		buildConfig("test2").bc(),
		deploymentConfig("test3").dc(),
		deploymentConfig("test3").dc(),
		imageStream("is1"),
		imageStream("is2"),
	}
	tests := []struct {
		ref    string
		expect bool
	}{
		{
			ref:    "is:v1",
			expect: false,
		},
		{
			ref:    "is1:latest",
			expect: true,
		},
		{
			ref:    "is2:alpha.0",
			expect: true,
		},
	}
	for _, test := range tests {
		actual := includesImageStream(imageStreamTagImageStream(test.ref), objs)
		if actual != test.expect {
			t.Errorf("unexpected: %q: %v", test.ref, actual)
		}
	}
}
func TestIsParameter(t *testing.T) {
	tests := []struct {
		v string // string to test
		m bool   // true if expected to match
	}{
		{
			v: "test}}}",
			m: false,
		},
		{
			v: "${hello}",
			m: true,
		},
		{
			v: "test{one}two",
			m: false,
		},
		{
			v: "${test",
			m: false,
		},
		{
			v: "${name}image",
			m: true,
		},
		{
			v: "image${name}",
			m: true,
		},
		{
			v: "${{number}}",
			m: true,
		},
	}

	for _, test := range tests {
		actual := isParameter(test.v)
		if actual != test.m {
			t.Errorf("Unexpected result: value: %q, actual: %v", test.v, actual)
		}
	}
}

func validateExpectedParams(t *testing.T, testName string, expected map[string]string, params Params) {
	if len(params) != len(expected) {
		t.Errorf("%s: unexpected params. Expected: %#v, Got: %#v", testName, expected, params)
		return
	}

	for name, value := range expected {
		p, found := params[name]
		if !found {
			t.Errorf("%s: param %s not found in %#v", testName, name, params)
			continue
		}
		if p.Value != value {
			t.Errorf("%s: parameter %s value does not match. Expected %q, Got %q", testName, name, value, p.Value)
		}
	}
}

func TestParameterizeRef(t *testing.T) {
	tests := []struct {
		name           string
		ref            *kapi.ObjectReference
		objs           []runtime.Object
		expectedParams map[string]string
		expectedRef    *kapi.ObjectReference
	}{
		{
			name: "imagestreamtag simple ref",
			ref: &kapi.ObjectReference{
				Name: "ruby:v1.0",
				Kind: "ImageStreamTag",
			},
			expectedParams: map[string]string{
				"RUBY_IMAGE_STREAM_TAG": "ruby:v1.0",
			},
			expectedRef: &kapi.ObjectReference{
				Name: "${RUBY_IMAGE_STREAM_TAG}",
				Kind: "ImageStreamTag",
			},
		},
		{
			name: "imagestreamtag with namespace",
			ref: &kapi.ObjectReference{
				Name:      "my-stream:v1.0",
				Kind:      "ImageStreamTag",
				Namespace: "myns",
			},
			expectedParams: map[string]string{
				"MY_STREAM_IMAGE_STREAM_TAG":    "my-stream:v1.0",
				"MY_STREAM_IMAGE_STREAM_TAG_NS": "myns",
			},
			expectedRef: &kapi.ObjectReference{
				Name:      "${MY_STREAM_IMAGE_STREAM_TAG}",
				Kind:      "ImageStreamTag",
				Namespace: "${MY_STREAM_IMAGE_STREAM_TAG_NS}",
			},
		},
		{
			name: "imagestreamtag with referenced imagestream",
			ref: &kapi.ObjectReference{
				Name: "my-stream:v1.0",
				Kind: "ImageStreamTag",
			},
			objs: []runtime.Object{
				imageStream("my-stream"),
			},
			expectedRef: &kapi.ObjectReference{
				Name: "my-stream:v1.0",
				Kind: "ImageStreamTag",
			},
		},
		{
			name: "parameterized imagestreamtag",
			ref: &kapi.ObjectReference{
				Name: "${IST_PARAM}",
				Kind: "ImageStreamTag",
			},
			expectedRef: &kapi.ObjectReference{
				Name: "${IST_PARAM}",
				Kind: "ImageStreamTag",
			},
		},
		{
			name: "imagestreamimage simple ref",
			ref: &kapi.ObjectReference{
				Name: "my-stream@sha256:14f916134cf58676196409553c139f00f4e0c55bc92d364346c96ea1ef1ea825",
				Kind: "ImageStreamImage",
			},
			expectedParams: map[string]string{
				"MY_STREAM_IMAGE_STREAM_IMAGE": "my-stream@sha256:14f916134cf58676196409553c139f00f4e0c55bc92d364346c96ea1ef1ea825",
			},
			expectedRef: &kapi.ObjectReference{
				Name: "${MY_STREAM_IMAGE_STREAM_IMAGE}",
				Kind: "ImageStreamImage",
			},
		},
		{
			name: "imagestreamimage with namespace",
			ref: &kapi.ObjectReference{
				Name:      "my-stream@sha256:14f916134cf58676196409553c139f00f4e0c55bc92d364346c96ea1ef1ea825",
				Kind:      "ImageStreamImage",
				Namespace: "myns",
			},
			expectedParams: map[string]string{
				"MY_STREAM_IMAGE_STREAM_IMAGE":    "my-stream@sha256:14f916134cf58676196409553c139f00f4e0c55bc92d364346c96ea1ef1ea825",
				"MY_STREAM_IMAGE_STREAM_IMAGE_NS": "myns",
			},
			expectedRef: &kapi.ObjectReference{
				Name:      "${MY_STREAM_IMAGE_STREAM_IMAGE}",
				Kind:      "ImageStreamImage",
				Namespace: "${MY_STREAM_IMAGE_STREAM_IMAGE_NS}",
			},
		},
		{
			name: "imagestreamimage with referenced imagestream",
			ref: &kapi.ObjectReference{
				Name: "my-stream@sha256:14f916134cf58676196409553c139f00f4e0c55bc92d364346c96ea1ef1ea825",
				Kind: "ImageStreamImage",
			},
			objs: []runtime.Object{
				imageStream("my-stream"),
			},
			expectedRef: &kapi.ObjectReference{
				Name: "my-stream@sha256:14f916134cf58676196409553c139f00f4e0c55bc92d364346c96ea1ef1ea825",
				Kind: "ImageStreamImage",
			},
		},
		{
			name: "dockerimage",
			ref: &kapi.ObjectReference{
				Name: "openshift/origin:v3.5",
				Kind: "DockerImage",
			},
			expectedParams: map[string]string{
				"ORIGIN_IMAGE_REF": "openshift/origin:v3.5",
			},
			expectedRef: &kapi.ObjectReference{
				Name: "${ORIGIN_IMAGE_REF}",
				Kind: "DockerImage",
			},
		},
	}
	for _, test := range tests {
		params := Params{}
		parameterizeRef(test.ref, test.objs, params)

		if !reflect.DeepEqual(*test.ref, *test.expectedRef) {
			t.Errorf("%s: expected ref: %#v, got: %#v", test.name, test.expectedRef, test.ref)
		}
		validateExpectedParams(t, test.name, test.expectedParams, params)
	}
}

func TestParameterizeBuildConfig(t *testing.T) {
	tests := []struct {
		name         string
		bc           *buildapi.BuildConfig
		objs         []runtime.Object
		expectParams map[string]string
		expectBC     *buildapi.BuildConfig
	}{
		{
			name: "build config with trigger",
			bc: buildConfig("testbc").
				withImageTrigger("ruby:v2.0", "ImageStreamTag", "").
				bc(),
			expectParams: map[string]string{"RUBY_IMAGE_STREAM_TAG": "ruby:v2.0"},
			expectBC: buildConfig("testbc").
				withImageTrigger("${RUBY_IMAGE_STREAM_TAG}", "ImageStreamTag", "").
				bc(),
		},
		{
			name: "build config with image source",
			bc: buildConfig("testbc").
				withImageSourceFrom("python:v3", "ImageStreamTag", "").
				bc(),
			expectParams: map[string]string{"PYTHON_IMAGE_STREAM_TAG": "python:v3"},
			expectBC: buildConfig("testbc").
				withImageSourceFrom("${PYTHON_IMAGE_STREAM_TAG}", "ImageStreamTag", "").
				bc(),
		},
		{
			name: "build config with docker strategy from",
			bc: buildConfig("testbc").
				withDockerStrategyFrom("myimage@sha256:14f916134cf58676196409553c139f00f4e0c55bc92d364346c96ea1ef1ea825", "ImageStreamImage", "project2").
				bc(),
			expectParams: map[string]string{
				"MYIMAGE_IMAGE_STREAM_IMAGE":    "myimage@sha256:14f916134cf58676196409553c139f00f4e0c55bc92d364346c96ea1ef1ea825",
				"MYIMAGE_IMAGE_STREAM_IMAGE_NS": "project2",
			},
			expectBC: buildConfig("testbc").
				withDockerStrategyFrom("${MYIMAGE_IMAGE_STREAM_IMAGE}", "ImageStreamImage", "${MYIMAGE_IMAGE_STREAM_IMAGE_NS}").
				bc(),
		},
		{
			name: "output ref to docker image",
			bc: buildConfig("testbc").
				withOutputTo("openshift/origin:v1.4.1", "DockerImage", "").
				bc(),
			expectParams: map[string]string{"ORIGIN_IMAGE_REF": "openshift/origin:v1.4.1"},
			expectBC: buildConfig("testbc").
				withOutputTo("${ORIGIN_IMAGE_REF}", "DockerImage", "").
				bc(),
		},
		{
			name: "build config with multiple refs to the same image",
			bc: buildConfig("testbc").
				withImageTrigger("mystream:tag", "ImageStreamTag", "").
				withCustomStrategyFrom("mystream:tag", "ImageStreamTag", "").
				withImageSourceFrom("mystream:tag", "ImageStreamTag", "").
				withOutputTo("mystream:tag", "ImageStreamTag", "").
				bc(),
			expectParams: map[string]string{"MYSTREAM_IMAGE_STREAM_TAG": "mystream:tag"},
			expectBC: buildConfig("testbc").
				withImageTrigger("${MYSTREAM_IMAGE_STREAM_TAG}", "ImageStreamTag", "").
				withCustomStrategyFrom("${MYSTREAM_IMAGE_STREAM_TAG}", "ImageStreamTag", "").
				withImageSourceFrom("${MYSTREAM_IMAGE_STREAM_TAG}", "ImageStreamTag", "").
				withOutputTo("${MYSTREAM_IMAGE_STREAM_TAG}", "ImageStreamTag", "").
				bc(),
		},
		{
			name: "build config with image stream refs to included imagestream",
			bc: buildConfig("testbc").
				withImageTrigger("mystream:tag", "ImageStreamTag", "").
				withCustomStrategyFrom("mystream:tag", "ImageStreamTag", "").
				withImageSourceFrom("mystream:tag", "ImageStreamTag", "").
				withOutputTo("mystream:tag", "ImageStreamTag", "").
				bc(),
			objs: []runtime.Object{
				imageStream("mystream"),
			},
			expectParams: map[string]string{},
			expectBC: buildConfig("testbc").
				withImageTrigger("mystream:tag", "ImageStreamTag", "").
				withCustomStrategyFrom("mystream:tag", "ImageStreamTag", "").
				withImageSourceFrom("mystream:tag", "ImageStreamTag", "").
				withOutputTo("mystream:tag", "ImageStreamTag", "").
				bc(),
		},
	}
	for _, test := range tests {
		params := Params{}
		parameterizeBuildConfig(test.bc, test.objs, params)
		validateExpectedParams(t, test.name, test.expectParams, params)
		if !equality.Semantic.DeepEqual(*test.bc, *test.expectBC) {
			t.Errorf("expected: %#v, got: %#v", asJSON(test.expectBC), asJSON(test.bc))
		}
	}
}
func TestParameterizeDeploymentConfig(t *testing.T) {
	tests := []struct {
		name         string
		dc           *deployapi.DeploymentConfig
		objs         []runtime.Object
		expectParams map[string]string
		expectDC     *deployapi.DeploymentConfig
	}{
		{
			name: "dc with image trigger",
			dc: deploymentConfig("testdc").
				withImageTrigger("php:2", "ImageStreamTag", "", nil).
				dc(),
			expectParams: map[string]string{"PHP_IMAGE_STREAM_TAG": "php:2"},
			expectDC: deploymentConfig("testdc").
				withImageTrigger("${PHP_IMAGE_STREAM_TAG}", "ImageStreamTag", "", nil).
				dc(),
		},
		{
			name: "dc with pod image ref",
			dc: deploymentConfig("testdc").
				withContainerImageRef("container1", "openshift/hello:v1").
				dc(),
			expectParams: map[string]string{"HELLO_IMAGE_REF": "openshift/hello:v1"},
			expectDC: deploymentConfig("testdc").
				withContainerImageRef("container1", "${HELLO_IMAGE_REF}").
				dc(),
		},
		{
			name: "dc with trigger that includes container",
			dc: deploymentConfig("testdc").
				withImageTrigger("php:2", "ImageStreamTag", "", []string{"mycontainer"}).
				withContainerImageRef("mycontainer", "openshift/hello:v1").
				dc(),
			expectParams: map[string]string{"PHP_IMAGE_STREAM_TAG": "php:2"},
			expectDC: deploymentConfig("testdc").
				withImageTrigger("${PHP_IMAGE_STREAM_TAG}", "ImageStreamTag", "", []string{"mycontainer"}).
				withContainerImageRef("mycontainer", "openshift/hello:v1").
				dc(),
		},
		{
			name: "dc with trigger and separate container",
			dc: deploymentConfig("testdc").
				withImageTrigger("php:2", "ImageStreamTag", "", []string{"anothercontainer"}).
				withContainerImageRef("mycontainer", "openshift/hello:v1").
				dc(),
			expectParams: map[string]string{"PHP_IMAGE_STREAM_TAG": "php:2", "HELLO_IMAGE_REF": "openshift/hello:v1"},
			expectDC: deploymentConfig("testdc").
				withImageTrigger("${PHP_IMAGE_STREAM_TAG}", "ImageStreamTag", "", []string{"anothercontainer"}).
				withContainerImageRef("mycontainer", "${HELLO_IMAGE_REF}").
				dc(),
		},
		{
			name: "dc with trigger and separate container and included imagestream",
			dc: deploymentConfig("testdc").
				withImageTrigger("php:2", "ImageStreamTag", "", []string{"anothercontainer"}).
				withContainerImageRef("mycontainer", "openshift/hello:v1").
				dc(),
			objs: []runtime.Object{
				imageStream("php"),
			},
			expectParams: map[string]string{"HELLO_IMAGE_REF": "openshift/hello:v1"},
			expectDC: deploymentConfig("testdc").
				withImageTrigger("php:2", "ImageStreamTag", "", []string{"anothercontainer"}).
				withContainerImageRef("mycontainer", "${HELLO_IMAGE_REF}").
				dc(),
		},
	}
	for _, test := range tests {
		params := Params{}
		parameterizeDeploymentConfig(test.dc, test.objs, params)
		validateExpectedParams(t, test.name, test.expectParams, params)
		if !equality.Semantic.DeepEqual(*test.dc, *test.expectDC) {
			t.Errorf("expected: %s, got: %s", asJSON(test.expectDC), asJSON(test.dc))
		}
	}
}
