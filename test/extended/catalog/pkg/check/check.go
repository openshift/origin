package check

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	specsgov1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/operator-framework/operator-registry/alpha/declcfg"
	"oras.land/oras-go/v2"
)

// Checks is a collection of checks to be performed on the catalog image.
type Checks struct {
	ImageChecks      []ImageCheck
	FilesystemChecks []FilesystemCheck
	CatalogChecks    []CatalogCheck
}

// ImageCheckFunc define functions to perform tests using the image (i.e. check labels)
type ImageCheckFunc func(ctx context.Context, root specsgov1.Descriptor, target oras.ReadOnlyTarget) error

// FilesystemCheckFunc define functions to perform tests using the filesystem (i.e. check if files exists in directories)
type FilesystemCheckFunc func(ctx context.Context, imageFS fs.FS) error

// CatalogCheckFunc define functions to perform tests using the catalog (i.e. check if the package's metadata in the catalog is valid)
type CatalogCheckFunc func(ctx context.Context, cfg declcfg.DeclarativeConfig) error

// ImageCheck represents a check to be performed on the image level.
type ImageCheck struct {
	Name string
	Fn   ImageCheckFunc
}

// FilesystemCheck represents a check to be performed on the filesystem.
type FilesystemCheck struct {
	Name string
	Fn   FilesystemCheckFunc
}

// CatalogCheck represents a check to be performed on the catalog.
type CatalogCheck struct {
	Name string
	Fn   CatalogCheckFunc
}

// Error represents an error that occurred during a check.
type Error struct {
	CheckName string
	Err       error
}

// Error implements the error interface for Error.
func (e Error) Error() string {
	return fmt.Sprintf("%s: %s", e.CheckName, e.Err)
}

// Check performs the checks defined in the Checks struct on the given a catalog image directory and
// returns any errors that occurred during the checks.
func Check(ctx context.Context, store oras.ReadOnlyTarget, imagePath string, checks Checks) error {
	const (
		fsDir      = "fs"
		configsDir = "configs"
	)

	var (
		// checkErrors store a list of errors that occurred during the checks.
		checkErrors []error
		// imageFsView store a filesystem view (fs.FS) rooted at that unpacked image path.
		imageFsView fs.FS
	)

	// Get the image descriptors used in the image checks.
	desc, err := store.Resolve(ctx, "v1")
	if err != nil {
		return fmt.Errorf("resolve descriptor: %w", err)
	}
	unpackPath := filepath.Join(imagePath, fsDir)

	// Execute image checks
	for _, check := range checks.ImageChecks {
		if err := check.Fn(ctx, desc, store); err != nil {
			checkErrors = append(checkErrors,
				Error{CheckName: fmt.Sprintf("[Image]:%s", check.Name), Err: err})
		}
	}

	// Execute filesystem checks
	if len(checks.FilesystemChecks) > 0 {
		if _, err := os.Stat(unpackPath); err != nil {
			return fmt.Errorf("expected extracted fs at %q: %w", unpackPath, err)
		}
		imageFsView = os.DirFS(unpackPath)
		for _, check := range checks.FilesystemChecks {
			if err := check.Fn(ctx, imageFsView); err != nil {
				checkErrors = append(checkErrors,
					Error{CheckName: fmt.Sprintf("[FS]:%s", check.Name), Err: err})
			}
		}
	}

	// Execute catalog checks
	if len(checks.CatalogChecks) > 0 {
		if imageFsView == nil {
			imageFsView = os.DirFS(unpackPath)
		}
		pkgFS, err := fs.Sub(imageFsView, configsDir)
		if err != nil {
			return fmt.Errorf("create file system for '%s': %w", configsDir, err)
		}
		cfg, err := declcfg.LoadFS(ctx, pkgFS)
		if err != nil {
			return fmt.Errorf("load declcfg: %w", err)
		}
		for _, check := range checks.CatalogChecks {
			if err := check.Fn(ctx, *cfg); err != nil {
				checkErrors = append(checkErrors,
					Error{CheckName: fmt.Sprintf("[FBC Catalog]:%s", check.Name), Err: err})
			}
		}
	}
	return errors.Join(checkErrors...)
}
