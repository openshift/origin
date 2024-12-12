// Copyright 2022 Red Hat, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package parse contains a function for parsing unit contents shared between
// multiple config versions.
package parse

import (
	"fmt"
	"strings"

	"github.com/coreos/go-systemd/v22/unit"
)

// ParseUnitContents parses the content of a given unit
func ParseUnitContents(content *string) ([]*unit.UnitOption, error) {
	if content == nil {
		return []*unit.UnitOption{}, nil
	}
	c := strings.NewReader(*content)
	opts, err := unit.Deserialize(c)
	if err != nil {
		return nil, fmt.Errorf("invalid unit content: %s", err)
	}
	return opts, nil
}
