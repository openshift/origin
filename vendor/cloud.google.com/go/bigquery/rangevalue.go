// Copyright 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package bigquery

// RangeValue represents a continuous RANGE of values of a given element
// type.  The supported element types for RANGE are currently the BigQuery
// DATE, DATETIME, and TIMESTAMP, types.
type RangeValue struct {
	// The start value of the range.  A missing value represents an
	// unbounded start.
	Start Value `json:"start"`

	// The end value of the range.  A missing value represents an
	// unbounded end.
	End Value `json:"end"`
}
