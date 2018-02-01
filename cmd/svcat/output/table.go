/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package output

import (
	"io"

	"github.com/olekukonko/tablewriter"
)

// NewListTable builds a table formatted to list a set of results.
func NewListTable(w io.Writer) *tablewriter.Table {
	t := tablewriter.NewWriter(w)
	t.SetBorder(false)
	t.SetColumnSeparator(" ")
	return t
}

// NewDetailsTable builds a table formatted to list details for a single result.
func NewDetailsTable(w io.Writer) *tablewriter.Table {
	t := tablewriter.NewWriter(w)
	t.SetBorder(false)
	t.SetColumnSeparator(" ")

	// tablewriter wraps based on "ragged text", not max column width
	// which is great for tables but isn't efficient for detailed views
	t.SetAutoWrapText(false)

	return t
}
