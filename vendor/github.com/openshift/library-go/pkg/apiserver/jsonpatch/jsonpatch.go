package jsonpatch

import (
	"encoding/json"
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

func (p *PatchSet) Marshal() ([]byte, error) {
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
	if op == patchTestOperation {
		p.patches = append([]PatchOperation{patch}, p.patches...)
		return
	}
	p.patches = append(p.patches, patch)
}

type TestCondition struct {
	path  string
	value interface{}
}

func NewTestCondition(path string, value interface{}) TestCondition {
	return TestCondition{path, value}
}
