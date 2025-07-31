// © Broadcom. All Rights Reserved.
// The term “Broadcom” refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: Apache-2.0

package types

import (
	"reflect"
	"strings"
)

var (
	t = map[string]reflect.Type{}

	// minAPIVersionForType is used to lookup the minimum API version for which
	// a type is valid.
	minAPIVersionForType = map[string]string{}

	// minAPIVersionForEnumValue is used to lookup the minimum API version for
	// which an enum value is valid.
	minAPIVersionForEnumValue = map[string]map[string]string{}
)

func Add(name string, kind reflect.Type) {
	t[name] = kind
}

func AddMinAPIVersionForType(name, minAPIVersion string) {
	minAPIVersionForType[name] = minAPIVersion
}

func AddMinAPIVersionForEnumValue(enumName, enumValue, minAPIVersion string) {
	if v, ok := minAPIVersionForEnumValue[enumName]; ok {
		v[enumValue] = minAPIVersion
	} else {
		minAPIVersionForEnumValue[enumName] = map[string]string{
			enumValue: minAPIVersion,
		}
	}
}

type Func func(string) (reflect.Type, bool)

func TypeFunc() Func {
	return func(name string) (reflect.Type, bool) {
		typ, ok := t[name]
		if !ok {
			// The /sdk endpoint does not prefix types with the namespace,
			// but extension endpoints, such as /pbm/sdk do.
			name = strings.TrimPrefix(name, "vim25:")
			typ, ok = t[name]
		}
		return typ, ok
	}
}
