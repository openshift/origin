// © Broadcom. All Rights Reserved.
// The term “Broadcom” refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: Apache-2.0

package progress

// Report defines the interface for types that can deliver progress reports.
// Examples include uploads/downloads in the http client and the task info
// field in the task managed object.
type Report interface {
	Percentage() float32
	Detail() string
	Error() error
}
