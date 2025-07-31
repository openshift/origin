// © Broadcom. All Rights Reserved.
// The term “Broadcom” refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: Apache-2.0

package types

type HasFault interface {
	Fault() BaseMethodFault
}

func IsFileNotFound(err error) bool {
	if f, ok := err.(HasFault); ok {
		switch f.Fault().(type) {
		case *FileNotFound:
			return true
		}
	}

	return false
}

func IsAlreadyExists(err error) bool {
	if f, ok := err.(HasFault); ok {
		switch f.Fault().(type) {
		case *AlreadyExists:
			return true
		}
	}

	return false
}

// HasLocalizedMethodFault is any type that has a LocalizedMethodFault.
type HasLocalizedMethodFault interface {

	// GetLocalizedMethodFault returns the LocalizedMethodFault instance.
	GetLocalizedMethodFault() *LocalizedMethodFault
}

// GetLocalizedMethodFault returns this LocalizedMethodFault.
func (f *LocalizedMethodFault) GetLocalizedMethodFault() *LocalizedMethodFault {
	return f
}
