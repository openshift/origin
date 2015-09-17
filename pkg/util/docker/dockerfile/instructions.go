package dockerfile

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/docker/docker/builder/command"
)

// A KeyValue can be used to build ordered lists of key-value pairs.
type KeyValue struct {
	Key   string
	Value string
}

// Env builds an ENV Dockerfile instruction from the mapping m. Keys and values
// are serialized as JSON strings to ensure compatibility with the Dockerfile
// parser.
func Env(m []KeyValue) (string, error) {
	return keyValueInstruction(command.Env, m)
}

// keyValueInstruction builds a Dockerfile instruction from the mapping m. Keys
// and values are serialized as JSON strings to ensure compatibility with the
// Dockerfile parser. Syntax:
//   COMMAND "KEY"="VALUE" "may"="contain spaces"
func keyValueInstruction(cmd string, m []KeyValue) (string, error) {
	s := []string{strings.ToUpper(cmd)}
	for _, kv := range m {
		// Marshal kv.Key and kv.Value as JSON strings to cover cases
		// like when the values contain spaces or special characters.
		k, err := json.Marshal(kv.Key)
		if err != nil {
			return "", err
		}
		v, err := json.Marshal(kv.Value)
		if err != nil {
			return "", err
		}
		s = append(s, fmt.Sprintf("%s=%s", k, v))
	}
	return strings.Join(s, " "), nil
}
