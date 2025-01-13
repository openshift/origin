package jsonpatch

import (
	"encoding/json"
	"fmt"

	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

type PatchOperation struct {
	Op    string      `json:"op,omitempty"`
	Path  string      `json:"path,omitempty"`
	Value interface{} `json:"value,omitempty"`
}

const (
	patchTestOperation   = "test"
	patchRemoveOperation = "remove"
)

type PatchSet struct {
	patches []PatchOperation
}

func New() *PatchSet {
	return &PatchSet{}
}

func (p *PatchSet) WithRemove(path string, test TestCondition) *PatchSet {
	p.WithTest(test.path, test.value)
	p.addOperation(patchRemoveOperation, path, nil)
	return p
}

func (p *PatchSet) WithTest(path string, value interface{}) *PatchSet {
	p.addOperation(patchTestOperation, path, value)
	return p
}

func (p *PatchSet) IsEmpty() bool {
	return len(p.patches) == 0
}

func (p *PatchSet) Marshal() ([]byte, error) {
	if err := p.validate(); err != nil {
		return nil, err
	}
	jsonBytes, err := json.Marshal(p.patches)
	if err != nil {
		return nil, err
	}
	return jsonBytes, nil
}

func (p *PatchSet) addOperation(op, path string, value interface{}) {
	patch := PatchOperation{
		Op:    op,
		Path:  path,
		Value: value,
	}
	p.patches = append(p.patches, patch)
}

func (p *PatchSet) validate() error {
	var errs []error
	for i, patch := range p.patches {
		if patch.Op == patchTestOperation {
			// testing resourceVersion is fragile
			// because it is likely to change frequently
			// instead, test against a different field
			// should be written.
			if patch.Path == "/metadata/resourceVersion" {
				errs = append(errs, fmt.Errorf("test operation at index: %d contains forbidden path: %q", i, patch.Path))
			}
		}
	}
	return utilerrors.NewAggregate(errs)
}

type TestCondition struct {
	path  string
	value interface{}
}

func NewTestCondition(path string, value interface{}) TestCondition {
	return TestCondition{path, value}
}
