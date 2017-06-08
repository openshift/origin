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

package controller

import (
	"encoding/json"
)

// TODO: does this need to move to another package?

// serialize converts values of any type to a []byte, suitable for addition to
// a k8s Secret's Data field (which is a map[string][]byte). Generally, this
// serialization is performed using json.Unmarshal(), which easily handles
// most primitives, but will also, conveniently, handle the scenario where the
// item being serialized is a map[string]interface{} or []interface{}-- both
// of which are possible since an OSB broker's response to a binding request
// might contain credentials that are composed of arbitrarily complex JSON.
// These would have been unmarshalled to map[string]interface{} or
// []interface{} when they were received and need to be re-marshalled here
// before they can be added to a Secret. (Note that the consumer would need to
// know what to do with those bytes that contain arbitrarily complex JSON. The
// controller's only concern is accurately passing along all credentials
// received from the OSB broker.) The common case where this strategy doesn't
// work is that of string values. Using json.Unmarshal() to serialize string
// values results in a []byte where the leading and trailing bytes represent
// double quotes. Consequently, this would lead to consumers receiving a
// credential like `"password"` where the credential from the OSB broker was
// actually `password`. It's easy to see that's bad, so an alternative
// strategy is used for serializing string values to avoid introducing those
// errant quotes.
func serialize(value interface{}) ([]byte, error) {
	if strVal, ok := value.(string); ok {
		return []byte(strVal), nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return data, nil
}
