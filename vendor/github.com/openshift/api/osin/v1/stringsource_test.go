package v1

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	configv1 "github.com/openshift/api/config/v1"
)

var testScheme = runtime.NewScheme()

func init() {
	utilruntime.Must(Install(testScheme))
}

func TestStringSourceUnmarshaling(t *testing.T) {
	codec := serializer.NewCodecFactory(testScheme).LegacyCodec(GroupVersion)

	testcases := map[string]struct {
		JSON           string
		ExpectedObject configv1.StringSource
		ExpectedError  string
	}{
		"bool": {
			JSON:           `true`,
			ExpectedObject: configv1.StringSource{},
			ExpectedError:  "cannot unmarshal",
		},
		"number": {
			JSON:           `1`,
			ExpectedObject: configv1.StringSource{},
			ExpectedError:  "cannot unmarshal",
		},

		"empty string": {
			JSON:           `""`,
			ExpectedObject: configv1.StringSource{},
			ExpectedError:  "",
		},
		"string": {
			JSON:           `"foo"`,
			ExpectedObject: configv1.StringSource{StringSourceSpec: configv1.StringSourceSpec{Value: "foo"}},
			ExpectedError:  "",
		},

		"empty struct": {
			JSON:           `{}`,
			ExpectedObject: configv1.StringSource{},
			ExpectedError:  "",
		},
		"struct value": {
			JSON:           `{"value":"foo"}`,
			ExpectedObject: configv1.StringSource{StringSourceSpec: configv1.StringSourceSpec{Value: "foo"}},
			ExpectedError:  "",
		},
		"struct env": {
			JSON:           `{"env":"foo"}`,
			ExpectedObject: configv1.StringSource{StringSourceSpec: configv1.StringSourceSpec{Env: "foo"}},
			ExpectedError:  "",
		},
		"struct file": {
			JSON:           `{"file":"foo"}`,
			ExpectedObject: configv1.StringSource{StringSourceSpec: configv1.StringSourceSpec{File: "foo"}},
			ExpectedError:  "",
		},
		"struct file+keyFile": {
			JSON:           `{"file":"foo","keyFile":"bar"}`,
			ExpectedObject: configv1.StringSource{StringSourceSpec: configv1.StringSourceSpec{File: "foo", KeyFile: "bar"}},
			ExpectedError:  "",
		},
	}

	for k, tc := range testcases {
		t.Run(k, func(t *testing.T) {
			// Wrap in a dummy object we can deserialize
			input := fmt.Sprintf(`{"kind":"GitHubIdentityProvider","apiVersion":"osin.config.openshift.io/v1","clientSecret":%s}`, tc.JSON)
			githubProvider := &GitHubIdentityProvider{}
			err := runtime.DecodeInto(codec, []byte(input), githubProvider)
			if len(tc.ExpectedError) > 0 && (err == nil || !strings.Contains(err.Error(), tc.ExpectedError)) {
				t.Errorf("%s: expected error containing %q, got %q", k, tc.ExpectedError, err.Error())
			}
			if len(tc.ExpectedError) == 0 && err != nil {
				t.Errorf("%s: got unexpected error: %v", k, err)
			}
			if err != nil {
				return
			}
			if !reflect.DeepEqual(tc.ExpectedObject, githubProvider.ClientSecret) {
				t.Errorf("%s: expected\n%#v\ngot\n%#v", k, tc.ExpectedObject, githubProvider.ClientSecret)
			}
		})
	}
}

func TestStringSourceMarshaling(t *testing.T) {
	codec := serializer.NewCodecFactory(testScheme).LegacyCodec(GroupVersion)

	testcases := map[string]struct {
		Object       configv1.StringSource
		ExpectedJSON string
	}{
		"empty string": {
			Object:       configv1.StringSource{},
			ExpectedJSON: `""`,
		},
		"string": {
			Object:       configv1.StringSource{StringSourceSpec: configv1.StringSourceSpec{Value: "foo"}},
			ExpectedJSON: `"foo"`,
		},
		"struct value+keyFile": {
			Object:       configv1.StringSource{StringSourceSpec: configv1.StringSourceSpec{Value: "foo", KeyFile: "bar"}},
			ExpectedJSON: `{"value":"foo","env":"","file":"","keyFile":"bar"}`,
		},
		"struct env": {
			Object:       configv1.StringSource{StringSourceSpec: configv1.StringSourceSpec{Env: "foo"}},
			ExpectedJSON: `{"value":"","env":"foo","file":"","keyFile":""}`,
		},
		"struct file": {
			Object:       configv1.StringSource{StringSourceSpec: configv1.StringSourceSpec{File: "foo"}},
			ExpectedJSON: `{"value":"","env":"","file":"foo","keyFile":""}`,
		},
		"struct file+keyFile": {
			Object:       configv1.StringSource{StringSourceSpec: configv1.StringSourceSpec{File: "foo", KeyFile: "bar"}},
			ExpectedJSON: `{"value":"","env":"","file":"foo","keyFile":"bar"}`,
		},
	}

	for k, tc := range testcases {
		provider := &GitHubIdentityProvider{ClientSecret: tc.Object}

		json, err := runtime.Encode(codec, provider)
		if err != nil {
			t.Errorf("%s: unexpected error: %v", k, err)
		}

		// Wrap in a dummy JSON from the surrounding object
		input := fmt.Sprintf(`{"kind":"GitHubIdentityProvider","apiVersion":"osin.config.openshift.io/v1","clientID":"","clientSecret":%s,"organizations":null,"teams":null,"hostname":"","ca":""}`, tc.ExpectedJSON)
		if strings.TrimSpace(string(json)) != input {
			t.Log(len(input), len(json))
			t.Errorf("%s: expected\n%s\ngot\n%s", k, input, string(json))
		}
	}
}
