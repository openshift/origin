// Copyright 2020 CoreOS, Inc.
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

package unit

// UnitEntry is a single line entry in a Unit file.
type UnitEntry struct {
	Name  string
	Value string
}

// UnitSection is a section in a Unit file. The section name
// and a list of entries in that section.
type UnitSection struct {
	Section string
	Entries []*UnitEntry
}

// String implements the stringify interface for UnitEntry
func (u *UnitEntry) String() string {
	return "{Name: " + u.Name + ", " + "Value: " + u.Value + "}"
}

// String implements the stringify interface for UnitSection
func (s *UnitSection) String() string {
	result := "{Section: " + s.Section
	for _, e := range s.Entries {
		result += e.String()
	}

	result += "}"
	return result
}
