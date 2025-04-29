package declcfg

import (
	"sort"

	"github.com/operator-framework/operator-registry/alpha/model"
	"github.com/operator-framework/operator-registry/alpha/property"
)

func ConvertFromModel(mpkgs model.Model) DeclarativeConfig {
	cfg := DeclarativeConfig{}
	for _, mpkg := range mpkgs {
		channels, bundles := traverseModelChannels(*mpkg)

		var i *Icon
		if mpkg.Icon != nil {
			i = &Icon{
				Data:      mpkg.Icon.Data,
				MediaType: mpkg.Icon.MediaType,
			}
		}
		defaultChannel := ""
		if mpkg.DefaultChannel != nil {
			defaultChannel = mpkg.DefaultChannel.Name
		}
		cfg.Packages = append(cfg.Packages, Package{
			Schema:         SchemaPackage,
			Name:           mpkg.Name,
			DefaultChannel: defaultChannel,
			Icon:           i,
			Description:    mpkg.Description,
		})
		cfg.Channels = append(cfg.Channels, channels...)
		cfg.Bundles = append(cfg.Bundles, bundles...)
	}

	sort.Slice(cfg.Packages, func(i, j int) bool {
		return cfg.Packages[i].Name < cfg.Packages[j].Name
	})
	sort.Slice(cfg.Channels, func(i, j int) bool {
		if cfg.Channels[i].Package != cfg.Channels[j].Package {
			return cfg.Channels[i].Package < cfg.Channels[j].Package
		}
		return cfg.Channels[i].Name < cfg.Channels[j].Name
	})
	sort.Slice(cfg.Bundles, func(i, j int) bool {
		if cfg.Bundles[i].Package != cfg.Bundles[j].Package {
			return cfg.Bundles[i].Package < cfg.Bundles[j].Package
		}
		return cfg.Bundles[i].Name < cfg.Bundles[j].Name
	})

	return cfg
}

func traverseModelChannels(mpkg model.Package) ([]Channel, []Bundle) {
	channels := []Channel{}
	bundleMap := map[string]*Bundle{}

	for _, ch := range mpkg.Channels {
		// initialize channel
		c := Channel{
			Schema:  SchemaChannel,
			Name:    ch.Name,
			Package: ch.Package.Name,
			Entries: []ChannelEntry{},
			// NOTICE: The field Properties of the type Channel is for internal use only.
			//   DO NOT use it for any public-facing functionalities.
			//   This API is in alpha stage and it is subject to change.
			Properties: ch.Properties,
		}

		for _, chb := range ch.Bundles {
			// populate channel entry
			c.Entries = append(c.Entries, ChannelEntry{
				Name:      chb.Name,
				Replaces:  chb.Replaces,
				Skips:     chb.Skips,
				SkipRange: chb.SkipRange,
			})

			// create or update bundle
			b, ok := bundleMap[chb.Name]
			if !ok {
				b = &Bundle{
					Schema:        SchemaBundle,
					Name:          chb.Name,
					Package:       chb.Package.Name,
					Image:         chb.Image,
					RelatedImages: ModelRelatedImagesToRelatedImages(chb.RelatedImages),
					CsvJSON:       chb.CsvJSON,
					Objects:       chb.Objects,
				}
				bundleMap[b.Name] = b
			}
			b.Properties = append(b.Properties, chb.Properties...)
		}

		// sort channel entries by name
		sort.Slice(c.Entries, func(i, j int) bool {
			return c.Entries[i].Name < c.Entries[j].Name
		})
		channels = append(channels, c)
	}

	// nolint:prealloc
	var bundles []Bundle
	for _, b := range bundleMap {
		b.Properties = property.Deduplicate(b.Properties)

		sort.Slice(b.Properties, func(i, j int) bool {
			if b.Properties[i].Type != b.Properties[j].Type {
				return b.Properties[i].Type < b.Properties[j].Type
			}
			return string(b.Properties[i].Value) < string(b.Properties[j].Value)
		})

		bundles = append(bundles, *b)
	}
	return channels, bundles
}

func ModelRelatedImagesToRelatedImages(relatedImages []model.RelatedImage) []RelatedImage {
	// nolint:prealloc
	var out []RelatedImage
	for _, ri := range relatedImages {
		out = append(out, RelatedImage{
			Name:  ri.Name,
			Image: ri.Image,
		})
	}
	return out
}
