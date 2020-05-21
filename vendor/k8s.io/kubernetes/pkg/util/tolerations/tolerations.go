/*
Copyright 2017 The Kubernetes Authors.

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

package tolerations

import (
	"github.com/golang/glog"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	api "k8s.io/kubernetes/pkg/apis/core"
)

type key struct {
	tolerationKey string
	effect        api.TaintEffect
}

// VerifyAgainstWhitelist checks if the provided tolerations
// satisfy the provided whitelist and returns true, otherwise returns false
func VerifyAgainstWhitelist(tolerations []api.Toleration, whitelist []api.Toleration) bool {
	if len(whitelist) == 0 || len(tolerations) == 0 {
		return true
	}

next:
	for _, t := range tolerations {
		for _, w := range whitelist {
			if isSuperset(w, t) {
				continue next
			}
		}
		return false
	}
	return true
}

// MergeTolerations merges two sets of tolerations into one
// it does not check for conflicts
func MergeTolerations(first []api.Toleration, second []api.Toleration) []api.Toleration {
	all := append(first, second...)
	var merged []api.Toleration

next:
	for i, t := range all {
		for _, t2 := range merged {
			if isSuperset(t2, t) {
				continue next // t is redundant; ignore it
			}
		}
		if i+1 < len(all) {
			for _, t2 := range all[i+1:] {
				// If the tolerations are equal, prefer the first.
				if !apiequality.Semantic.DeepEqual(&t, &t2) && isSuperset(t2, t) {
					continue next // t is redundant; ignore it
				}
			}
		}
		merged = append(merged, t)
	}

	return merged
}

// isSuperset checks whether ss tolerates a superset of t.
func isSuperset(ss, t api.Toleration) bool {
	if apiequality.Semantic.DeepEqual(&t, &ss) {
		return true
	}

	if t.Key != ss.Key &&
		// An empty key with Exists operator means match all keys & values.
		(ss.Key != "" || ss.Operator != api.TolerationOpExists) {
		return false
	}

	// An empty effect means match all effects.
	if t.Effect != ss.Effect && ss.Effect != "" {
		return false
	}

	if ss.Effect == api.TaintEffectNoExecute {
		if ss.TolerationSeconds != nil {
			if t.TolerationSeconds == nil ||
				*t.TolerationSeconds > *ss.TolerationSeconds {
				return false
			}
		}
	}

	switch ss.Operator {
	case api.TolerationOpEqual, "": // empty operator means Equal
		return t.Operator == api.TolerationOpEqual && t.Value == ss.Value
	case api.TolerationOpExists:
		return true
	default:
		glog.Errorf("Unknown toleration operator: %s", ss.Operator)
		return false
	}
}

// EqualTolerations returns true if two sets of tolerations are equal, otherwise false
// it assumes no duplicates in individual set of tolerations
func EqualTolerations(first []api.Toleration, second []api.Toleration) bool {
	if len(first) != len(second) {
		return false
	}

	firstMap := ConvertTolerationToAMap(first)
	secondMap := ConvertTolerationToAMap(second)

	for k1, v1 := range firstMap {
		if v2, ok := secondMap[k1]; !ok || !AreEqual(v1, v2) {
			return false
		}
	}
	return true
}

// ConvertTolerationToAMap converts toleration list into a map[string]api.Toleration
func ConvertTolerationToAMap(in []api.Toleration) map[key]api.Toleration {
	out := map[key]api.Toleration{}
	for i := range in {
		out[key{in[i].Key, in[i].Effect}] = in[i]
	}
	return out
}

// AreEqual checks if two provided tolerations are equal or not.
func AreEqual(first, second api.Toleration) bool {
	if first.Key == second.Key &&
		first.Operator == second.Operator &&
		first.Value == second.Value &&
		first.Effect == second.Effect &&
		AreTolerationSecondsEqual(first.TolerationSeconds, second.TolerationSeconds) {
		return true
	}
	return false
}

// AreTolerationSecondsEqual checks if two provided TolerationSeconds are equal or not.
func AreTolerationSecondsEqual(ts1, ts2 *int64) bool {
	if ts1 == ts2 {
		return true
	}
	if ts1 != nil && ts2 != nil && *ts1 == *ts2 {
		return true
	}
	return false
}