// Copyright 2019 Red Hat, Inc
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
// limitations under the License.)

package report

import (
	"fmt"

	"github.com/coreos/vcontext/path"
	"github.com/coreos/vcontext/tree"
)

// EntryKind represents an Entry's severity.
type EntryKind interface {
	String() string
	IsFatal() bool
}

// Report is a collection of information from validating a struct.
type Report struct {
	Entries []Entry
}

// Merge adds the entries from child to r.
func (r *Report) Merge(child Report) {
	r.Entries = append(r.Entries, child.Entries...)
}

// getDeepestNode returns the deepest node matching the context.
func getDeepestNode(n tree.Node, c path.ContextPath) tree.Node {
	if child, err := n.Get(c); err != nil {
		return getDeepestNode(n, c.Pop())
	} else {
		return child
	}
}

// Correlate takes a node tree and populates the markers in the report's entries
// based on the entries' context.
func (r *Report) Correlate(n tree.Node) {
	for i, e := range r.Entries {
		r.Entries[i].Marker = getDeepestNode(n, e.Context).GetMarker()
	}
}

// IsFatal returns true if any entries are fatal.
func (r Report) IsFatal() bool {
	for _, e := range r.Entries {
		if e.Kind.IsFatal() {
			return true
		}
	}
	return false
}

func (r Report) String() string {
	str := ""
	for _, e := range r.Entries {
		str += e.String() + "\n"
	}
	return str
}

// Entry represents one error or message from validation.
type Entry struct {
	// Kind is the severity of the message.
	Kind    EntryKind
	Message string

	// Context is the logical location of the error.
	Context path.ContextPath

	// Marker is the literal location in a json or yaml blob of the error.
	Marker tree.Marker
}

func (e Entry) String() string {
	at := ""
	switch {
	case e.Marker.StartP != nil && e.Context.Len() != 0:
		at = fmt.Sprintf(" at %s, %s", e.Context.String(), e.Marker.String())
	case e.Marker.StartP != nil:
		at = fmt.Sprintf(" at %s", e.Marker.String())
	case e.Context.Len() != 0:
		at = fmt.Sprintf(" at %s", e.Context.String())
	}

	return fmt.Sprintf("%s%s: %s", e.Kind.String(), at, e.Message)
}

// Kind is a default set of EntryKind.
type Kind int

const (
	Error Kind = iota
	Warn  Kind = iota
	Info  Kind = iota
)

func (k Kind) String() string {
	switch k {
	case Error:
		return "error"
	case Warn:
		return "warning"
	case Info:
		return "info"
	default:
		return "unknown severity"
	}
}

func (k Kind) IsFatal() bool {
	return k == Error
}

func (r *Report) AddOn(c path.ContextPath, err error, k EntryKind) {
	if err == nil {
		return
	}
	r.Entries = append(r.Entries, Entry{
		Message: err.Error(),
		Context: c.Copy(),
		Kind:    k,
	})
}

// AddOnError adds err to report with kind "Error" if err is not nil.
func (r *Report) AddOnError(c path.ContextPath, err error) {
	r.AddOn(c, err, Error)

}

// AddOnWarn adds err to report with kind "Warning" if err is not nil.
func (r *Report) AddOnWarn(c path.ContextPath, err error) {
	r.AddOn(c, err, Warn)
}

// AddOnInfo adds err to report with kind "Info" if err is not nil.
func (r *Report) AddOnInfo(c path.ContextPath, err error) {
	r.AddOn(c, err, Info)
}
