// Copyright (c) 2012-2018 Ugorji Nwoke. All rights reserved.
// Use of this source code is governed by a MIT license found in the LICENSE file.

// +build !go1.11

package main

import (
	"go/build"
)

func pkgPath(dir string) (string, error) {
	pkg, err := build.Default.ImportDir(dir, build.AllowBinary)
	if err != nil {
		return "", err
	}
	return pkg.ImportPath, nil
}
