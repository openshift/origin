// Copyright 2014 go-dockerclient authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package docker

import (
	"time"
)

type Image struct {
	ID              string    `yaml:"id" json:"id"`
	Parent          string    `yaml:"parent,omitempty" json:"parent,omitempty"`
	Comment         string    `yaml:"comment,omitempty" json:"comment,omitempty"`
	Created         time.Time `yaml:"created,omitempty" json:"created"`
	Container       string    `yaml:"container,omitempty" json:"container,omitempty"`
	ContainerConfig Config    `yaml:"containerconfig,omitempty" json:"containerconfig,omitempty"`
	DockerVersion   string    `yaml:"dockerversion,omitempty" json:"dockerversion,omitempty"`
	Author          string    `yaml:"author,omitempty" json:"author,omitempty"`
	Config          *Config   `yaml:"config",omitempty" json:"config,omitempty"`
	Architecture    string    `yaml:"architecture,omitempty" json:"architecture,omitempty"`
	Size            int64     `yaml:"size,omitempty" json:"size,omitempty"`
}
