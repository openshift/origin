package check

import (
	"context"
	"errors"
	"fmt"

	"github.com/operator-framework/operator-registry/alpha/declcfg"
)

// AllCatalogChecks returns a list of all catalog checks to be performed.
func AllCatalogChecks() []CatalogCheck {
	return []CatalogCheck{
		CatalogHasNotBundleObject(),
		CatalogHasValidMetadataForChannelHeads(),
	}
}

// CatalogHasNotBundleObject verifies that any bundle has not olm.bundle.object.
func CatalogHasNotBundleObject() CatalogCheck {
	return CatalogCheck{
		Name: "CatalogHasNotBundleObject",
		Fn: func(ctx context.Context, cfg declcfg.DeclarativeConfig) error {
			var errs []error
			for _, bundle := range cfg.Bundles {
				var found bool
				for _, prop := range bundle.Properties {
					if prop.Type == "olm.bundle.object" {
						found = true
					}
				}
				if found {
					errs = append(errs, fmt.Errorf("bundle %q in package %q has olm.bundle.object set",
						bundle.Name, bundle.Package))
				}
			}
			return errors.Join(errs...)
		},
	}
}

// CatalogHasValidMetadataForChannelHeads checks if the channel heads have olm.csv.metadata
func CatalogHasValidMetadataForChannelHeads() CatalogCheck {
	return CatalogCheck{
		Name: "CatalogHasValidMetadataForChannelHeads",
		Fn: func(ctx context.Context, cfg declcfg.DeclarativeConfig) error {
			var errs []error
			packages, err := declcfg.ConvertToModel(cfg)
			if err != nil {
				return fmt.Errorf("unable to covert: %w", err)
			}

			for _, pkg := range packages {
				for _, ch := range pkg.Channels {
					channelHead, err := ch.Head()
					if err != nil {
						continue
					}

					var found bool
					for _, prop := range channelHead.Properties {
						if prop.Type == "olm.csv.metadata" {
							found = true
						}
					}
					if !found {
						errs = append(errs,
							fmt.Errorf("bundle %q in package %q is missing olm.csv.metadata",
								channelHead.Name,
								channelHead.Package.Name))
					}
				}
			}
			return errors.Join(errs...)
		},
	}
}
