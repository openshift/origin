// Copyright (c) 2012-2018 Ugorji Nwoke. All rights reserved.
// Use of this source code is governed by a MIT license found in the LICENSE file.

// +build go1.11

package main

import (
	"fmt"

	"golang.org/x/tools/go/packages"
)

func pkgPath(dir string) (string, error) {
	pkgs, err := packages.Load(&packages.Config{Dir: dir}, ".")
	if err != nil {
		return "", err
	}
	if len(pkgs) != 1 {
		return "", fmt.Errorf("Could not read package (%d package found)", len(pkgs))
	}
	pkg := pkgs[0]
	return pkg.PkgPath, nil
}
