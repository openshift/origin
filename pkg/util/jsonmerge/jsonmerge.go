package jsonmerge

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/evanphx/json-patch"
	"github.com/golang/glog"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/yaml"
)

// Delta represents a change between two JSON documents.
type Delta struct {
	original []byte
	edit     []byte
}

// NewDelta accepts two JSON or YAML documents and calculates the difference
// between them.  It returns a Delta object which can be used to resolve
// conflicts against a third version with a common parent, or an error
// if either document is in error.
func NewDelta(from, to []byte) (*Delta, error) {
	d := &Delta{}
	before, err := yaml.ToJSON(from)
	if err != nil {
		return nil, err
	}
	after, err := yaml.ToJSON(to)
	if err != nil {
		return nil, err
	}
	diff, err := jsonpatch.CreateMergePatch(before, after)
	if err != nil {
		return nil, err
	}
	glog.V(6).Infof("Patch created from:\n%s\n%s\n%s", string(before), string(after), string(diff))
	d.original = before
	d.edit = diff
	return d, nil
}

// Apply attempts to apply the changes described by Delta onto latest,
// returning an error if the changes cannot be applied cleanly.
// IsConflicting will be true if the changes overlap, otherwise a
// generic error will be returned.
func (d *Delta) Apply(latest []byte) ([]byte, error) {
	base, err := yaml.ToJSON(latest)
	if err != nil {
		return nil, err
	}
	changes, err := jsonpatch.CreateMergePatch(d.original, base)
	if err != nil {
		return nil, err
	}
	diff1 := make(map[string]interface{})
	if err := json.Unmarshal(d.edit, &diff1); err != nil {
		return nil, err
	}
	diff2 := make(map[string]interface{})
	if err := json.Unmarshal(changes, &diff2); err != nil {
		return nil, err
	}
	glog.V(6).Infof("Testing for conflict between:\n%s\n%s", string(d.edit), string(changes))
	if overlap(diff1, diff2) {
		return nil, ErrConflict
	}
	return jsonpatch.MergePatch(base, d.edit)
}

// IsConflicting returns true if the provided error indicates a
// conflict exists between the original changes and the applied
// changes.
func IsConflicting(err error) bool {
	return err == ErrConflict
}

var ErrConflict = fmt.Errorf("changes are in conflict")

func overlap(a, b interface{}) bool {
	switch u := a.(type) {
	case map[string]interface{}:
		switch v := b.(type) {
		case map[string]interface{}:
			for key, left := range u {
				if right, ok := v[key]; ok && overlap(left, right) {
					return true
				}
			}
			return false
		default:
			return true
		}
	case []interface{}:
		switch v := b.(type) {
		case []interface{}:
			if len(u) != len(v) {
				return true
			}
			for i := range u {
				if overlap(u[i], v[i]) {
					return true
				}
			}
			return false
		default:
			return true
		}
	case string, float64, bool, int, int64, nil:
		return !reflect.DeepEqual(a, b)
	default:
		panic(fmt.Sprintf("unknown type: %v", reflect.TypeOf(u)))
	}
}
