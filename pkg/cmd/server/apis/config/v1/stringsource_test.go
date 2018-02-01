package v1

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"

	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
)

func TestStringSourceUnmarshaling(t *testing.T) {
	codec := serializer.NewCodecFactory(configapi.Scheme).LegacyCodec(SchemeGroupVersion)

	testcases := map[string]struct {
		JSON           string
		ExpectedObject configapi.StringSource
		ExpectedError  string
	}{
		"bool": {
			JSON:           `true`,
			ExpectedObject: configapi.StringSource{},
			ExpectedError:  "cannot unmarshal",
		},
		"number": {
			JSON:           `1`,
			ExpectedObject: configapi.StringSource{},
			ExpectedError:  "cannot unmarshal",
		},

		"empty string": {
			JSON:           `""`,
			ExpectedObject: configapi.StringSource{},
			ExpectedError:  "",
		},
		"string": {
			JSON:           `"foo"`,
			ExpectedObject: configapi.StringSource{StringSourceSpec: configapi.StringSourceSpec{Value: "foo"}},
			ExpectedError:  "",
		},

		"empty struct": {
			JSON:           `{}`,
			ExpectedObject: configapi.StringSource{},
			ExpectedError:  "",
		},
		"struct value": {
			JSON:           `{"value":"foo"}`,
			ExpectedObject: configapi.StringSource{StringSourceSpec: configapi.StringSourceSpec{Value: "foo"}},
			ExpectedError:  "",
		},
		"struct env": {
			JSON:           `{"env":"foo"}`,
			ExpectedObject: configapi.StringSource{StringSourceSpec: configapi.StringSourceSpec{Env: "foo"}},
			ExpectedError:  "",
		},
		"struct file": {
			JSON:           `{"file":"foo"}`,
			ExpectedObject: configapi.StringSource{StringSourceSpec: configapi.StringSourceSpec{File: "foo"}},
			ExpectedError:  "",
		},
		"struct file+keyFile": {
			JSON:           `{"file":"foo","keyFile":"bar"}`,
			ExpectedObject: configapi.StringSource{StringSourceSpec: configapi.StringSourceSpec{File: "foo", KeyFile: "bar"}},
			ExpectedError:  "",
		},
	}

	for k, tc := range testcases {
		// Wrap in a dummy object we can deserialize
		input := fmt.Sprintf(`{"kind":"GitHubIdentityProvider","apiVersion":"v1","clientSecret":%s}`, tc.JSON)
		obj, err := runtime.Decode(codec, []byte(input))
		if len(tc.ExpectedError) > 0 && (err == nil || !strings.Contains(err.Error(), tc.ExpectedError)) {
			t.Errorf("%s: expected error containing %q, got %q", k, tc.ExpectedError, err.Error())
		}
		if len(tc.ExpectedError) == 0 && err != nil {
			t.Errorf("%s: got unexpected error: %v", k, err)
		}
		if err != nil {
			continue
		}
		githubProvider, ok := obj.(*configapi.GitHubIdentityProvider)
		if !ok {
			t.Errorf("%s: wrapper object was not a GitHubIdentityProvider as expected: %#v", k, obj)
			continue
		}
		if !reflect.DeepEqual(tc.ExpectedObject, githubProvider.ClientSecret) {
			t.Errorf("%s: expected\n%#v\ngot\n%#v", k, tc.ExpectedObject, githubProvider.ClientSecret)
		}
	}
}

func TestStringSourceMarshaling(t *testing.T) {
	codec := serializer.NewCodecFactory(configapi.Scheme).LegacyCodec(SchemeGroupVersion)

	testcases := map[string]struct {
		Object       configapi.StringSource
		ExpectedJSON string
	}{
		"empty string": {
			Object:       configapi.StringSource{},
			ExpectedJSON: `""`,
		},
		"string": {
			Object:       configapi.StringSource{StringSourceSpec: configapi.StringSourceSpec{Value: "foo"}},
			ExpectedJSON: `"foo"`,
		},
		"struct value+keyFile": {
			Object:       configapi.StringSource{StringSourceSpec: configapi.StringSourceSpec{Value: "foo", KeyFile: "bar"}},
			ExpectedJSON: `{"value":"foo","env":"","file":"","keyFile":"bar"}`,
		},
		"struct env": {
			Object:       configapi.StringSource{StringSourceSpec: configapi.StringSourceSpec{Env: "foo"}},
			ExpectedJSON: `{"value":"","env":"foo","file":"","keyFile":""}`,
		},
		"struct file": {
			Object:       configapi.StringSource{StringSourceSpec: configapi.StringSourceSpec{File: "foo"}},
			ExpectedJSON: `{"value":"","env":"","file":"foo","keyFile":""}`,
		},
		"struct file+keyFile": {
			Object:       configapi.StringSource{StringSourceSpec: configapi.StringSourceSpec{File: "foo", KeyFile: "bar"}},
			ExpectedJSON: `{"value":"","env":"","file":"foo","keyFile":"bar"}`,
		},
	}

	for k, tc := range testcases {
		provider := &configapi.GitHubIdentityProvider{ClientSecret: tc.Object}

		json, err := runtime.Encode(codec, provider)
		if err != nil {
			t.Errorf("%s: unexpected error: %v", k, err)
		}

		// Wrap in a dummy JSON from the surrounding object
		input := fmt.Sprintf(`{"kind":"GitHubIdentityProvider","apiVersion":"v1","clientID":"","clientSecret":%s,"organizations":null,"teams":null}`, tc.ExpectedJSON)
		if strings.TrimSpace(string(json)) != input {
			t.Log(len(input), len(json))
			t.Errorf("%s: expected\n%s\ngot\n%s", k, input, string(json))
		}
	}
}
