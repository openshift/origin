package declcfg

import (
	"fmt"

	"github.com/blang/semver/v4"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation"

	"github.com/operator-framework/operator-registry/alpha/model"
	"github.com/operator-framework/operator-registry/alpha/property"
)

func ConvertToModel(cfg DeclarativeConfig) (model.Model, error) {
	mpkgs := model.Model{}
	defaultChannels := map[string]string{}
	for _, p := range cfg.Packages {
		if p.Name == "" {
			return nil, fmt.Errorf("config contains package with no name")
		}

		if _, ok := mpkgs[p.Name]; ok {
			return nil, fmt.Errorf("duplicate package %q", p.Name)
		}

		if errs := validation.IsDNS1123Label(p.Name); len(errs) > 0 {
			return nil, fmt.Errorf("invalid package name %q: %v", p.Name, errs)
		}

		mpkg := &model.Package{
			Name:        p.Name,
			Description: p.Description,
			Channels:    map[string]*model.Channel{},
		}
		if p.Icon != nil {
			mpkg.Icon = &model.Icon{
				Data:      p.Icon.Data,
				MediaType: p.Icon.MediaType,
			}
		}
		defaultChannels[p.Name] = p.DefaultChannel
		mpkgs[p.Name] = mpkg
	}

	channelDefinedEntries := map[string]sets.Set[string]{}
	for _, c := range cfg.Channels {
		mpkg, ok := mpkgs[c.Package]
		if !ok {
			return nil, fmt.Errorf("unknown package %q for channel %q", c.Package, c.Name)
		}

		if c.Name == "" {
			return nil, fmt.Errorf("package %q contains channel with no name", c.Package)
		}

		if _, ok := mpkg.Channels[c.Name]; ok {
			return nil, fmt.Errorf("package %q has duplicate channel %q", c.Package, c.Name)
		}

		mch := &model.Channel{
			Package: mpkg,
			Name:    c.Name,
			Bundles: map[string]*model.Bundle{},
			// NOTICE: The field Properties of the type Channel is for internal use only.
			//   DO NOT use it for any public-facing functionalities.
			//   This API is in alpha stage and it is subject to change.
			Properties: c.Properties,
		}

		cde := sets.Set[string]{}
		for _, entry := range c.Entries {
			if _, ok := mch.Bundles[entry.Name]; ok {
				return nil, fmt.Errorf("invalid package %q, channel %q: duplicate entry %q", c.Package, c.Name, entry.Name)
			}
			cde = cde.Insert(entry.Name)
			mch.Bundles[entry.Name] = &model.Bundle{
				Package:   mpkg,
				Channel:   mch,
				Name:      entry.Name,
				Replaces:  entry.Replaces,
				Skips:     entry.Skips,
				SkipRange: entry.SkipRange,
			}
		}
		channelDefinedEntries[c.Package] = cde

		mpkg.Channels[c.Name] = mch

		defaultChannelName := defaultChannels[c.Package]
		if defaultChannelName == c.Name {
			mpkg.DefaultChannel = mch
		}
	}

	// packageBundles tracks the set of bundle names for each package
	// and is used to detect duplicate bundles.
	packageBundles := map[string]sets.Set[string]{}

	for _, b := range cfg.Bundles {
		if b.Package == "" {
			return nil, fmt.Errorf("package name must be set for bundle %q", b.Name)
		}
		mpkg, ok := mpkgs[b.Package]
		if !ok {
			return nil, fmt.Errorf("unknown package %q for bundle %q", b.Package, b.Name)
		}

		bundles, ok := packageBundles[b.Package]
		if !ok {
			bundles = sets.Set[string]{}
		}
		if bundles.Has(b.Name) {
			return nil, fmt.Errorf("package %q has duplicate bundle %q", b.Package, b.Name)
		}
		bundles.Insert(b.Name)
		packageBundles[b.Package] = bundles

		props, err := property.Parse(b.Properties)
		if err != nil {
			return nil, fmt.Errorf("parse properties for bundle %q: %v", b.Name, err)
		}

		if len(props.Packages) != 1 {
			return nil, fmt.Errorf("package %q bundle %q must have exactly 1 %q property, found %d", b.Package, b.Name, property.TypePackage, len(props.Packages))
		}

		if b.Package != props.Packages[0].PackageName {
			return nil, fmt.Errorf("package %q does not match %q property %q", b.Package, property.TypePackage, props.Packages[0].PackageName)
		}

		// Parse version from the package property.
		rawVersion := props.Packages[0].Version
		ver, err := semver.Parse(rawVersion)
		if err != nil {
			return nil, fmt.Errorf("error parsing bundle %q version %q: %v", b.Name, rawVersion, err)
		}

		channelDefinedEntries[b.Package] = channelDefinedEntries[b.Package].Delete(b.Name)
		found := false
		for _, mch := range mpkg.Channels {
			if mb, ok := mch.Bundles[b.Name]; ok {
				found = true
				mb.Image = b.Image
				mb.Properties = b.Properties
				mb.RelatedImages = relatedImagesToModelRelatedImages(b.RelatedImages)
				mb.CsvJSON = b.CsvJSON
				mb.Objects = b.Objects
				mb.PropertiesP = props
				mb.Version = ver
			}
		}
		if !found {
			return nil, fmt.Errorf("package %q, bundle %q not found in any channel entries", b.Package, b.Name)
		}
	}

	for pkg, entries := range channelDefinedEntries {
		if entries.Len() > 0 {
			return nil, fmt.Errorf("no olm.bundle blobs found in package %q for olm.channel entries %s", pkg, sets.List[string](entries))
		}
	}

	for _, mpkg := range mpkgs {
		defaultChannelName := defaultChannels[mpkg.Name]
		if defaultChannelName != "" && mpkg.DefaultChannel == nil {
			dch := &model.Channel{
				Package: mpkg,
				Name:    defaultChannelName,
				Bundles: map[string]*model.Bundle{},
			}
			mpkg.DefaultChannel = dch
			mpkg.Channels[dch.Name] = dch
		}
	}

	// deprecationsByPackage tracks the set of package names
	// and is used to detect duplicate packages.
	deprecationsByPackage := sets.New[string]()

	for i, deprecation := range cfg.Deprecations {
		// no need to validate schema, since it could not be unmarshaled if missing/invalid

		if deprecation.Package == "" {
			return nil, fmt.Errorf("package name must be set for deprecation item %v", i)
		}

		// must refer to package in this catalog
		mpkg, ok := mpkgs[deprecation.Package]
		if !ok {
			return nil, fmt.Errorf("cannot apply deprecations to an unknown package %q", deprecation.Package)
		}

		// must be unique per package
		if deprecationsByPackage.Has(deprecation.Package) {
			return nil, fmt.Errorf("expected a maximum of one deprecation per package: %q", deprecation.Package)
		}
		deprecationsByPackage.Insert(deprecation.Package)

		references := sets.New[PackageScopedReference]()

		for j, entry := range deprecation.Entries {
			if entry.Reference.Schema == "" {
				return nil, fmt.Errorf("schema must be set for deprecation entry [%v] for package %q", deprecation.Package, j)
			}

			if references.Has(entry.Reference) {
				return nil, fmt.Errorf("duplicate deprecation entry %#v for package %q", entry.Reference, deprecation.Package)
			}
			references.Insert(entry.Reference)

			switch entry.Reference.Schema {
			case SchemaBundle:
				if !packageBundles[deprecation.Package].Has(entry.Reference.Name) {
					return nil, fmt.Errorf("cannot deprecate bundle %q for package %q: bundle not found", entry.Reference.Name, deprecation.Package)
				}
				for _, mch := range mpkg.Channels {
					if mb, ok := mch.Bundles[entry.Reference.Name]; ok {
						mb.Deprecation = &model.Deprecation{Message: entry.Message}
					}
				}
			case SchemaChannel:
				ch, ok := mpkg.Channels[entry.Reference.Name]
				if !ok {
					return nil, fmt.Errorf("cannot deprecate channel %q for package %q: channel not found", entry.Reference.Name, deprecation.Package)
				}
				ch.Deprecation = &model.Deprecation{Message: entry.Message}

			case SchemaPackage:
				if entry.Reference.Name != "" {
					return nil, fmt.Errorf("package name must be empty for deprecated package %q (specified %q)", deprecation.Package, entry.Reference.Name)
				}
				mpkg.Deprecation = &model.Deprecation{Message: entry.Message}

			default:
				return nil, fmt.Errorf("cannot deprecate object %#v referenced by entry %v for package %q: object schema unknown", entry.Reference, j, deprecation.Package)
			}
		}
	}

	if err := mpkgs.Validate(); err != nil {
		return nil, err
	}
	mpkgs.Normalize()
	return mpkgs, nil
}

func relatedImagesToModelRelatedImages(in []RelatedImage) []model.RelatedImage {
	// nolint:prealloc
	var out []model.RelatedImage
	for _, p := range in {
		out = append(out, model.RelatedImage{
			Name:  p.Name,
			Image: p.Image,
		})
	}
	return out
}
