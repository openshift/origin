// © Broadcom. All Rights Reserved.
// The term “Broadcom” refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: Apache-2.0

package mo

import "github.com/vmware/govmomi/vim25/types"

// Reference is the interface that is implemented by all the managed objects
// defined in this package. It specifies that these managed objects have a
// function that returns the managed object reference to themselves.
type Reference interface {
	Reference() types.ManagedObjectReference
}
