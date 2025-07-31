// © Broadcom. All Rights Reserved.
// The term “Broadcom” refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: Apache-2.0

package task

import "github.com/vmware/govmomi/vim25/types"

type Error struct {
	*types.LocalizedMethodFault
	Description *types.LocalizableMessage
}

// Error returns the task's localized fault message.
func (e Error) Error() string {
	return e.LocalizedMethodFault.LocalizedMessage
}

func (e Error) Fault() types.BaseMethodFault {
	return e.LocalizedMethodFault.Fault
}
