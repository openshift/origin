// © Broadcom. All Rights Reserved.
// The term “Broadcom” refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: Apache-2.0

package list

import (
	"path"
	"strings"
)

func ToParts(p string) []string {
	p = path.Clean(p)
	if p == "/" {
		return []string{}
	}

	if len(p) > 0 {
		// Prefix ./ if relative
		if p[0] != '/' && p[0] != '.' {
			p = "./" + p
		}
	}

	ps := strings.Split(p, "/")
	if ps[0] == "" {
		// Start at root
		ps = ps[1:]
	}

	return ps
}
