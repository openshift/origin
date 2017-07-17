/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package util

import (
	"errors"
	"strings"

	"k8s.io/gengo/types"
)

var supportedTags = []string{
	"genclient",
	"genclient:nonNamespaced",
	"genclient:noVerbs",
	"genclient:onlyVerbs",
	"genclient:skipVerbs",
	"genclient:noStatus",
}

var SupportedVerbs = []string{
	"create",
	"update",
	"updateStatus",
	"delete",
	"deleteCollection",
	"get",
	"list",
	"watch",
	"patch",
}

type Tags struct {
	// +genclient
	GenerateClient bool
	// +genclient:nonNamespaced
	NonNamespaced bool
	// +genclient:noStatus
	NoStatus bool
	// +genclient:noVerbs
	NoVerbs bool
	// +genclient:onlyVerbs=create,delete
	OnlyVerbs []string
	// +genclient:skipVerbs=get,update
	SkipVerbs []string
}

// ParseClientGenTags parse the provided genclient tags and validates that no unknown
// tags are provided.
func ParseClientGenTags(lines []string) (Tags, error) {
	ret := Tags{}
	values := types.ExtractCommentTags("+", lines)
	_, ret.GenerateClient = values["genclient"]
	_, ret.NonNamespaced = values["genclient:nonNamespaced"]
	_, ret.NoVerbs = values["genclient:noVerbs"]
	_, ret.NoStatus = values["genclient:noStatus"]
	if v, exists := values["genclient:onlyVerbs"]; exists {
		ret.OnlyVerbs = strings.Split(v[0], ",")
	}
	if v, exists := values["genclient:skipVerbs"]; exists {
		ret.SkipVerbs = strings.Split(v[0], ",")
	}
	return ret, validateClientGenTags(values)
}

// validateTags validates that only supported genclient tags were provided.
func validateClientGenTags(values map[string][]string) error {
	for _, k := range supportedTags {
		delete(values, k)
	}
	for key := range values {
		if strings.HasPrefix(key, "genclient") {
			return errors.New("unknown tag detected: " + key)
		}
	}
	return nil
}
