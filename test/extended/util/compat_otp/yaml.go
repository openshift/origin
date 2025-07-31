package compat_otp

import (
	"encoding/json"
	"io/ioutil"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

/*
YamlReplace define a YAML modification given.
Example:

	YamlReplace {
		Path: 'spec.template.spec.imagePullSecrets',
		Value: '- name: notmatch-secret',
	}
*/
type YamlReplace struct {
	Path  string // path to modify or create value
	Value string // a string literal or YAML string (ex. 'name: frontend') to be set under the given path
}

func convert(i interface{}) interface{} {
	switch x := i.(type) {
	case map[interface{}]interface{}:
		//A map with key and value using arbitrary value
		m2 := map[string]interface{}{}
		for k, v := range x {
			m2[k.(string)] = convert(v)
		}
		return m2

	case map[string]interface{}:
		//A map with string key and an arbitrary value
		m2 := map[string]interface{}{}
		for k, v := range x {
			m2[k] = convert(v)
		}
		return m2

	case []interface{}:
		// Arbitrary type value
		for i, v := range x {
			x[i] = convert(v)
		}
	}
	return i
}

/*
Yaml2Json converts yaml file to json format.
Example:

	util.Yaml2Json(string(yamlFile))
*/
func Yaml2Json(s string) (string, error) {
	var (
		body    interface{}
		errJson error
		b       []byte
	)
	if err := yaml.Unmarshal([]byte(s), &body); err != nil {
		e2e.Failf("Failed to unmarshal yaml with error: %v", err)
	}

	body = convert(body)

	if b, errJson = json.Marshal(body); errJson != nil {
		e2e.Failf("Failed to marshal json with error: %v", errJson)
	}
	return string(b), errJson
}

/*
ModifyYamlFileContent modify the content of YAML file given the file path and a list of YamlReplace struct.
Example:
ModifyYamlFileContent(file, []YamlReplace {

		{
			Path: 'spec.template.spec.imagePullSecrets',
			Value: '- name: notmatch-secret',
		},
	})
*/
func ModifyYamlFileContent(file string, replacements []YamlReplace) {
	input, err := ioutil.ReadFile(file)
	if err != nil {
		e2e.Failf("read file %s failed: %v", file, err)
	}

	var doc yaml.Node
	if err = yaml.Unmarshal(input, &doc); err != nil {
		e2e.Failf("unmarshal yaml for file %s failed: %v", file, err)
	}

	for _, replacement := range replacements {
		path := strings.Split(replacement.Path, ".")
		value := yaml.Node{
			Kind:  yaml.ScalarNode,
			Value: replacement.Value,
		}
		setYamlValue(&doc, path, value)
	}

	output, err := yaml.Marshal(doc.Content[0])
	if err != nil {
		e2e.Failf("marshal yaml for file %s failed: %v", file, err)
	}

	if err = ioutil.WriteFile(file, output, 0o755); err != nil {
		e2e.Failf("write file %s failed: %v", file, err)
	}
}

// setYamlValue set (or create if path not exist) a leaf yaml.Node according to given path
func setYamlValue(root *yaml.Node, path []string, value yaml.Node) {
	if len(path) == 0 {
		var valueParsed yaml.Node
		if err := yaml.Unmarshal([]byte(value.Value), &valueParsed); err == nil {
			*root = *valueParsed.Content[0]
		} else {
			*root = value
		}
		return
	}
	key := path[0]
	rest := path[1:]
	switch root.Kind {
	case yaml.DocumentNode:
		setYamlValue(root.Content[0], path, value)
	case yaml.MappingNode:
		for i := 0; i < len(root.Content); i += 2 {
			if root.Content[i].Value == key {
				setYamlValue(root.Content[i+1], rest, value)
				return
			}
		}
		// key not found
		root.Content = append(root.Content, &yaml.Node{
			Kind:  yaml.ScalarNode,
			Value: key,
		}, &yaml.Node{
			Kind: yaml.MappingNode,
		})
		l := len(root.Content)
		setYamlValue(root.Content[l-1], rest, value)
	case yaml.SequenceNode:
		index, err := strconv.Atoi(key)
		if err != nil {
			e2e.Failf("string to int failed: %v", err)
		}
		if index < len(root.Content) {
			setYamlValue(root.Content[index], rest, value)
		}
	}
}
