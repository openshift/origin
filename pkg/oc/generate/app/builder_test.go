package app

import (
	"reflect"
	"testing"

	imageapi "github.com/openshift/origin/pkg/image/apis/image"
)

func TestIsBuilderImage(t *testing.T) {
	tests := map[string]struct {
		image          *imageapi.DockerImage
		expectedReturn bool
	}{
		"nil image": {
			expectedReturn: false,
		},
		"nil config": {
			image:          &imageapi.DockerImage{Config: nil},
			expectedReturn: false,
		},
		"has label": {
			image: &imageapi.DockerImage{
				Config: &imageapi.DockerConfig{
					Labels: map[string]string{
						s2iScriptsLabel: "",
					},
				},
			},
			expectedReturn: true,
		},
		"has legacy environment STI_LOCATION": {
			image: &imageapi.DockerImage{
				Config: &imageapi.DockerConfig{
					Env: []string{"STI_LOCATION="},
				},
			},
			expectedReturn: true,
		},
		"has legacy environment STI_SCRIPTS_URL": {
			image: &imageapi.DockerImage{
				Config: &imageapi.DockerConfig{
					Env: []string{"STI_SCRIPTS_URL="},
				},
			},
			expectedReturn: true,
		},
		"has legacy environment STI_BUILDER": {
			image: &imageapi.DockerImage{
				Config: &imageapi.DockerConfig{
					Env: []string{"STI_BUILDER="},
				},
			},
			expectedReturn: true,
		},
		"not an sti builder": {
			image: &imageapi.DockerImage{
				Config: &imageapi.DockerConfig{},
			},
			expectedReturn: false,
		},
	}
	for name, test := range tests {
		ret := IsBuilderImage(test.image)
		if ret != test.expectedReturn {
			t.Errorf("%s expected %t", name, test.expectedReturn)
		}
	}
}

func TestIsBuilderStreamTag(t *testing.T) {
	tests := map[string]struct {
		stream         *imageapi.ImageStream
		tag            string
		expectedReturn bool
	}{
		"nil stream": {
			stream:         nil,
			tag:            "foo",
			expectedReturn: false,
		},
		"not a builder": {
			stream: &imageapi.ImageStream{
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"foo": {
							Annotations: map[string]string{
								"tags": "a,b,c",
							},
						},
					},
				},
			},
			tag:            "foo",
			expectedReturn: false,
		},
		"is a builder": {
			stream: &imageapi.ImageStream{
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"foo": {
							Annotations: map[string]string{
								"tags": "a,b,c,builder",
							},
						},
					},
				},
			},
			tag:            "foo",
			expectedReturn: true,
		},
	}
	for name, test := range tests {
		ret := IsBuilderStreamTag(test.stream, test.tag)
		if ret != test.expectedReturn {
			t.Errorf("%s expected %t", name, test.expectedReturn)
		}
	}
}

func TestIsBuilderMatch(t *testing.T) {
	tests := map[string]struct {
		match       *ComponentMatch
		expectedRet bool
	}{
		"match on image": {
			match: &ComponentMatch{
				Image: &imageapi.DockerImage{
					Config: &imageapi.DockerConfig{
						Labels: map[string]string{
							s2iScriptsLabel: "",
						},
					},
				},
			},
			expectedRet: true,
		},
		"match on stream": {
			match: &ComponentMatch{
				ImageStream: &imageapi.ImageStream{
					Spec: imageapi.ImageStreamSpec{
						Tags: map[string]imageapi.TagReference{
							"foo": {
								Annotations: map[string]string{
									"tags": "a,b,c,builder",
								},
							},
						},
					},
				},
				ImageTag: "foo",
			},
			expectedRet: true,
		},
		"no match": {
			match: &ComponentMatch{
				Image: &imageapi.DockerImage{
					Config: &imageapi.DockerConfig{},
				},
				ImageStream: &imageapi.ImageStream{
					Spec: imageapi.ImageStreamSpec{
						Tags: map[string]imageapi.TagReference{
							"foo": {
								Annotations: map[string]string{
									"tags": "a,b,c",
								},
							},
						},
					},
				},
				ImageTag: "foo",
			},
		},
	}
	for name, test := range tests {
		if ret := IsBuilderMatch(test.match); ret != test.expectedRet {
			t.Errorf("%s expected IsBuilderMatch to return %t", name, test.expectedRet)
		}
	}
}

func TestIsGeneratorJobImage(t *testing.T) {
	tests := map[string]struct {
		image       *imageapi.DockerImage
		expectedRet bool
	}{
		"nil image": {expectedRet: false},
		"nil config": {
			image: &imageapi.DockerImage{
				Config: nil,
			},
			expectedRet: false,
		},
		"match": {
			image: &imageapi.DockerImage{
				Config: &imageapi.DockerConfig{
					Labels: map[string]string{
						labelGenerateJob: "true",
					},
				},
			},
			expectedRet: true,
		},
		"no match": {
			image: &imageapi.DockerImage{
				Config: &imageapi.DockerConfig{},
			},
			expectedRet: false,
		},
	}
	for name, test := range tests {
		if ret := isGeneratorJobImage(test.image); ret != test.expectedRet {
			t.Errorf("%s expected isGeneratorJobImage to return %t", name, test.expectedRet)
		}
	}
}

func TestIsGeneratorJobImageStreamTag(t *testing.T) {
	tests := map[string]struct {
		stream      *imageapi.ImageStream
		tag         string
		expectedRet bool
	}{
		"nil stream": {
			stream:      nil,
			expectedRet: false,
		},
		"match": {
			stream: &imageapi.ImageStream{
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"foo": {
							Annotations: map[string]string{
								labelGenerateJob: "true",
							},
						},
					},
				},
			},
			tag:         "foo",
			expectedRet: true,
		},
		"no match": {
			stream: &imageapi.ImageStream{
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"foo": {
							Annotations: map[string]string{},
						},
					},
				},
			},
			tag:         "foo",
			expectedRet: false,
		},
	}
	for name, test := range tests {
		if ret := isGeneratorJobImageStreamTag(test.stream, test.tag); ret != test.expectedRet {
			t.Errorf("%s expected isGeneratorJobImageStreamTag to return %t", name, test.expectedRet)
		}
	}
}

func TestParseGenerateTokenAs(t *testing.T) {
	foo := "foo"
	tests := map[string]struct {
		value              string
		expectedTokenInput *TokenInput
		expectedErr        string
	}{
		"invalid value for label": {
			value:       "foo",
			expectedErr: "unrecognized value for label",
		},
		"env label good": {
			value: "env:foo",
			expectedTokenInput: &TokenInput{
				Env: &foo,
			},
		},
		"env label less than two parts": {
			value:       "env",
			expectedErr: "expected 'env:<NAME>' or not set",
		},
		"env label name empty": {
			value:       "env: ",
			expectedErr: "expected 'env:<NAME>' but name was empty",
		},
		"file label good": {
			value: "file:foo",
			expectedTokenInput: &TokenInput{
				File: &foo,
			},
		},
		"file label less than two parts": {
			value:       "file",
			expectedErr: "expected 'file:<PATH>' or not set",
		},
		"file label name empty": {
			value:       "file: ",
			expectedErr: "expected 'file:<PATH>' but path was empty",
		},
		"service account": {
			value: "serviceaccount:foo",
			expectedTokenInput: &TokenInput{
				ServiceAccount: true,
			},
		},
	}
	for name, test := range tests {
		tokenInput, err := parseGenerateTokenAs(test.value)
		if !reflect.DeepEqual(tokenInput, test.expectedTokenInput) {
			t.Errorf("%s expected token input to be %#v but got %#v", name, test.expectedTokenInput, tokenInput)
		}
		checkError(err, test.expectedErr, name, t)
	}
}
