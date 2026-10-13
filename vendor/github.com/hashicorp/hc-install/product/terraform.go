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

var terraformVersionOutputRe = regexp.MustCompile(`Terraform ` + simpleVersionRe)

var Terraform = Product{
	Name: "terraform",
	BinaryName: func() string {
		if runtime.GOOS == "windows" {
			return "terraform.exe"
		}
		return "terraform"
	},
	GetVersion: func(ctx context.Context, path string) (*version.Version, error) {
		v, err := terraformJsonVersion(ctx, path)
		if err == nil {
			return v, nil
		}

		// JSON output was added in 0.13.0
		// See https://github.com/hashicorp/terraform/pull/25252
		// We assume that error implies older version.
		return legacyTerraformVersion(ctx, path)
	},
	BuildInstructions: &BuildInstructions{
		GitRepoURL:    "https://github.com/hashicorp/terraform.git",
		PreCloneCheck: &build.GoIsInstalled{},
		Build:         &build.GoBuild{DetectVendoring: true},
	},
}

type terraformJsonVersionOutput struct {
	Version *version.Version `json:"terraform_version"`
}

func terraformJsonVersion(ctx context.Context, path string) (*version.Version, error) {
	cmd := exec.CommandContext(ctx, path, "version", "-json")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}

	var vOut terraformJsonVersionOutput
	err = json.Unmarshal(out, &vOut)
	if err != nil {
		return nil, err
	}

	return vOut.Version, nil
}

func legacyTerraformVersion(ctx context.Context, path string) (*version.Version, error) {
	cmd := exec.CommandContext(ctx, path, "version")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	stdout := strings.TrimSpace(string(out))

	submatches := terraformVersionOutputRe.FindStringSubmatch(stdout)
	if len(submatches) != 2 {
		return nil, fmt.Errorf("unexpected number of version matches %d for %s", len(submatches), stdout)
	}
	v, err := version.NewVersion(submatches[1])
	if err != nil {
		return nil, fmt.Errorf("unable to parse version %q: %w", submatches[1], err)
	}

	return v, err
}
