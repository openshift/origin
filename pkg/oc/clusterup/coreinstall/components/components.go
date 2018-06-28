package components

import (
	"encoding/json"
	"io"

	"k8s.io/apimachinery/pkg/util/sets"
)

// Components lists the components that were enabled on cluster up run on empty base directory.
type Components struct {
	Enabled []string `json:"components"`
}

func (c *Components) Add(component string) {
	s := sets.NewString(c.Enabled...)
	if s.Has(component) {
		return
	}
	c.Enabled = append(c.Enabled, component)
}

// NewComponentsEnabled initialize the Components
func NewComponentsEnabled(component ...string) *Components {
	return &Components{Enabled: component}
}

// ReadComponentsEnabled reads the components enabled from the JSON file
func ReadComponentsEnabled(r io.Reader) (*Components, error) {
	c := Components{}
	err := json.NewDecoder(r).Decode(&c)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// WriteComponentsEnabled writes the components enabled to a JSON file
func WriteComponentsEnabled(w io.Writer, c *Components) error {
	return json.NewEncoder(w).Encode(c)
}
