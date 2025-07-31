// © Broadcom. All Rights Reserved.
// The term “Broadcom” refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: Apache-2.0

package mo

// Entity is the interface that is implemented by all managed objects
// that extend ManagedEntity.
type Entity interface {
	Reference
	Entity() *ManagedEntity
}
