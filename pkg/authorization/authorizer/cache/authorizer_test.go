package cache

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"k8s.io/kubernetes/pkg/auth/authorizer"
	"k8s.io/kubernetes/pkg/auth/user"
	"k8s.io/kubernetes/pkg/util/sets"
)

func TestAuthorizer(t *testing.T) {
	_, _ = NewAuthorizer(nil, time.Minute, 1000)
}

func TestCacheKey(t *testing.T) {
	tests := map[string]struct {
		Attrs authorizer.Attributes

		ExpectedKey string
		ExpectedErr bool
	}{
		"empty": {
			Attrs:       authorizer.AttributesRecord{},
			ExpectedKey: `{"apiGroup":"","apiVersion":"","name":"","namespace":"","path":"","readOnly":false,"resource":"","resourceRequest":false,"subresource":"","verb":""}`,
		},
		"full": {
			Attrs: authorizer.AttributesRecord{
				User:            &user.DefaultInfo{Name: "me", Groups: []string{"group1", "group2"}},
				Verb:            "v",
				Namespace:       "myns",
				APIVersion:      "av",
				APIGroup:        "ag",
				Resource:        "r",
				Subresource:     "sub",
				Name:            "rn",
				ResourceRequest: true,
				Path:            "/abc",
			},
			ExpectedKey: `{"apiGroup":"ag","apiVersion":"av","groups":["group1","group2"],"name":"rn","namespace":"myns","path":"/abc","readOnly":false,"resource":"r","resourceRequest":true,"scopes":null,"subresource":"sub","user":"me","verb":"v"}`,
		},
	}

	for k, tc := range tests {
		key, err := cacheKey(tc.Attrs)
		if tc.ExpectedErr != (err != nil) {
			t.Errorf("%s: expected err=%v, got %v", k, tc.ExpectedErr, err)
		}
		if tc.ExpectedKey != key {
			t.Errorf("%s: expected key=%v, got %v", k, tc.ExpectedKey, key)
		}
	}
}

func TestCacheKeyFields(t *testing.T) {
	keyJSON, err := cacheKey(authorizer.AttributesRecord{User: &user.DefaultInfo{Name: "me"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	keyMap := map[string]interface{}{}
	if err := json.Unmarshal([]byte(keyJSON), &keyMap); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	keys := sets.NewString()
	for k := range keyMap {
		keys.Insert(strings.ToLower(k))
	}

	attrType := reflect.TypeOf((*authorizer.AttributesRecord)(nil)).Elem()
	for i := 0; i < attrType.NumMethod(); i++ {
		name := attrType.Method(i).Name
		name = strings.TrimPrefix(name, "Get")
		name = strings.TrimPrefix(name, "Is")
		name = strings.ToLower(name)
		if !keys.Has(name) {
			t.Errorf("computed cache is missing an entry for %s", attrType.Method(i).Name)
		}
	}
}
