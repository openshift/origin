// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package product

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strings"

	"github.com/hashicorp/go-version"
	"github.com/hashicorp/hc-install/internal/build"
)

var consulVersionOutputRe = regexp.MustCompile(`Consul ` + simpleVersionRe)

var Consul = Product{
	Name: "consul",
	BinaryName: func() string {
		if runtime.GOOS == "windows" {
			return "consul.exe"
		}
		return "consul"
	},
	GetVersion: func(ctx context.Context, path string) (*version.Version, error) {
		v, err := consulJsonVersion(ctx, path)
		if err == nil {
			return v, nil
		}

		// JSON output was added in 1.9.0
		// See https://github.com/hashicorp/consul/pull/8268
		// We assume that error implies older version.
		return legacyConsulVersion(ctx, path)
	},
	BuildInstructions: &BuildInstructions{
		GitRepoURL:    "https://github.com/hashicorp/consul.git",
		PreCloneCheck: &build.GoIsInstalled{},
		Build:         &build.GoBuild{},
	},
}

type consulJsonVersionOutput struct {
	Version *version.Version `json:"Version"`
}

func consulJsonVersion(ctx context.Context, path string) (*version.Version, error) {
	cmd := exec.CommandContext(ctx, path, "version", "-format=json")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}

	var vOut consulJsonVersionOutput
	err = json.Unmarshal(out, &vOut)
	if err != nil {
		return nil, err
	}

	return vOut.Version, nil
}

func legacyConsulVersion(ctx context.Context, path string) (*version.Version, error) {
	cmd := exec.CommandContext(ctx, path, "version")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	stdout := strings.TrimSpace(string(out))

	submatches := consulVersionOutputRe.FindStringSubmatch(stdout)
	if len(submatches) != 2 {
		return nil, fmt.Errorf("unexpected number of version matches %d for %s", len(submatches), stdout)
	}
	v, err := version.NewVersion(submatches[1])
	if err != nil {
		return nil, fmt.Errorf("unable to parse version %q: %w", submatches[1], err)
	}

	return v, err
}
