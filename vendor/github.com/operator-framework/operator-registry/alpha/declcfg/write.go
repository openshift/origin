package declcfg

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/blang/semver/v4"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/yaml"

	"github.com/operator-framework/operator-registry/alpha/property"
)

type MermaidWriter struct {
	MinEdgeName          string
	SpecifiedPackageName string
}

type MermaidOption func(*MermaidWriter)

func NewMermaidWriter(opts ...MermaidOption) *MermaidWriter {
	const (
		minEdgeName          = ""
		specifiedPackageName = ""
	)
	m := &MermaidWriter{
		MinEdgeName:          minEdgeName,
		SpecifiedPackageName: specifiedPackageName,
	}

	for _, opt := range opts {
		opt(m)
	}
	return m
}

func WithMinEdgeName(minEdgeName string) MermaidOption {
	return func(o *MermaidWriter) {
		o.MinEdgeName = minEdgeName
	}
}

func WithSpecifiedPackageName(specifiedPackageName string) MermaidOption {
	return func(o *MermaidWriter) {
		o.SpecifiedPackageName = specifiedPackageName
	}
}

// writes out the channel edges of the declarative config graph in a mermaid format capable of being pasted into
// mermaid renderers like github, mermaid.live, etc.
// output is sorted lexicographically by package name, and then by channel name
// if provided, minEdgeName will be used as the lower bound for edges in the output graph
//
// Example output:
// graph LR
//
//	  %% package "neuvector-certified-operator-rhmp"
//	  subgraph "neuvector-certified-operator-rhmp"
//	     %% channel "beta"
//	     subgraph neuvector-certified-operator-rhmp-beta["beta"]
//		      neuvector-certified-operator-rhmp-beta-neuvector-operator.v1.2.8["neuvector-operator.v1.2.8"]
//		      neuvector-certified-operator-rhmp-beta-neuvector-operator.v1.2.9["neuvector-operator.v1.2.9"]
//		      neuvector-certified-operator-rhmp-beta-neuvector-operator.v1.3.0["neuvector-operator.v1.3.0"]
//		      neuvector-certified-operator-rhmp-beta-neuvector-operator.v1.3.0["neuvector-operator.v1.3.0"]-- replaces --> neuvector-certified-operator-rhmp-beta-neuvector-operator.v1.2.8["neuvector-operator.v1.2.8"]
//		      neuvector-certified-operator-rhmp-beta-neuvector-operator.v1.3.0["neuvector-operator.v1.3.0"]-- skips --> neuvector-certified-operator-rhmp-beta-neuvector-operator.v1.2.9["neuvector-operator.v1.2.9"]
//	    end
//	  end
//
// end
func (writer *MermaidWriter) WriteChannels(cfg DeclarativeConfig, out io.Writer) error {
	pkgs := map[string]*strings.Builder{}

	sort.Slice(cfg.Channels, func(i, j int) bool {
		return cfg.Channels[i].Name < cfg.Channels[j].Name
	})

	versionMap, err := getBundleVersions(&cfg)
	if err != nil {
		return err
	}

	// establish a 'floor' version, either specified by user or entirely open
	minVersion := semver.Version{Major: 0, Minor: 0, Patch: 0}

	if writer.MinEdgeName != "" {
		if _, ok := versionMap[writer.MinEdgeName]; !ok {
			return fmt.Errorf("unknown minimum edge name: %q", writer.MinEdgeName)
		}
		minVersion = versionMap[writer.MinEdgeName]
	}

	// build increasing-version-ordered bundle names, so we can meaningfully iterate over a range
	orderedBundles := []string{}
	for n := range versionMap {
		orderedBundles = append(orderedBundles, n)
	}
	sort.Slice(orderedBundles, func(i, j int) bool {
		return versionMap[orderedBundles[i]].LT(versionMap[orderedBundles[j]])
	})

	minEdgePackage := writer.getMinEdgePackage(&cfg)

	depByPackage := sets.Set[string]{}
	depByChannel := sets.Set[string]{}
	depByBundle := sets.Set[string]{}

	for _, d := range cfg.Deprecations {
		for _, e := range d.Entries {
			switch e.Reference.Schema {
			case SchemaPackage:
				depByPackage.Insert(d.Package)
			case SchemaChannel:
				depByChannel.Insert(e.Reference.Name)
			case SchemaBundle:
				depByBundle.Insert(e.Reference.Name)
			}
		}
	}

	var deprecatedPackage string
	deprecatedChannels := []string{}

	for _, c := range cfg.Channels {
		filteredChannel := writer.filterChannel(&c, versionMap, minVersion, minEdgePackage)
		// nolint:nestif
		if filteredChannel != nil {
			pkgBuilder, ok := pkgs[c.Package]
			if !ok {
				pkgBuilder = &strings.Builder{}
				pkgs[c.Package] = pkgBuilder
			}

			channelID := fmt.Sprintf("%s-%s", filteredChannel.Package, filteredChannel.Name)
			pkgBuilder.WriteString(fmt.Sprintf("    %%%% channel %q\n", filteredChannel.Name))
			pkgBuilder.WriteString(fmt.Sprintf("    subgraph %s[%q]\n", channelID, filteredChannel.Name))

			if depByPackage.Has(filteredChannel.Package) {
				deprecatedPackage = filteredChannel.Package
			}

			if depByChannel.Has(filteredChannel.Name) {
				deprecatedChannels = append(deprecatedChannels, channelID)
			}

			for _, ce := range filteredChannel.Entries {
				if versionMap[ce.Name].GE(minVersion) {
					bundleDeprecation := ""
					if depByBundle.Has(ce.Name) {
						bundleDeprecation = ":::deprecated"
					}

					entryID := fmt.Sprintf("%s-%s", channelID, ce.Name)
					pkgBuilder.WriteString(fmt.Sprintf("      %s[%q]%s\n", entryID, ce.Name, bundleDeprecation))

					if len(ce.Replaces) > 0 {
						replacesID := fmt.Sprintf("%s-%s", channelID, ce.Replaces)
						pkgBuilder.WriteString(fmt.Sprintf("      %s[%q]-- %s --> %s[%q]\n", replacesID, ce.Replaces, "replace", entryID, ce.Name))
					}
					if len(ce.Skips) > 0 {
						for _, s := range ce.Skips {
							skipsID := fmt.Sprintf("%s-%s", channelID, s)
							pkgBuilder.WriteString(fmt.Sprintf("      %s[%q]-- %s --> %s[%q]\n", skipsID, s, "skip", entryID, ce.Name))
						}
					}
					if len(ce.SkipRange) > 0 {
						skipRange, err := semver.ParseRange(ce.SkipRange)
						if err == nil {
							for _, edgeName := range filteredChannel.Entries {
								if skipRange(versionMap[edgeName.Name]) {
									skipRangeID := fmt.Sprintf("%s-%s", channelID, edgeName.Name)
									pkgBuilder.WriteString(fmt.Sprintf("      %s[%q]-- \"%s(%s)\" --> %s[%q]\n", skipRangeID, edgeName.Name, "skipRange", ce.SkipRange, entryID, ce.Name))
								}
							}
						} else {
							fmt.Fprintf(os.Stderr, "warning: ignoring invalid SkipRange for package/edge %q/%q: %v\n", c.Package, ce.Name, err)
						}
					}
				}
			}
			pkgBuilder.WriteString("    end\n")
		}
	}

	_, _ = out.Write([]byte("graph LR\n"))
	_, _ = out.Write([]byte("  classDef deprecated fill:#E8960F\n"))
	pkgNames := []string{}
	for pname := range pkgs {
		pkgNames = append(pkgNames, pname)
	}
	sort.Slice(pkgNames, func(i, j int) bool {
		return pkgNames[i] < pkgNames[j]
	})
	for _, pkgName := range pkgNames {
		_, _ = out.Write([]byte(fmt.Sprintf("  %%%% package %q\n", pkgName)))
		_, _ = out.Write([]byte(fmt.Sprintf("  subgraph %q\n", pkgName)))
		_, _ = out.Write([]byte(pkgs[pkgName].String()))
		_, _ = out.Write([]byte("  end\n"))
	}

	if deprecatedPackage != "" {
		_, _ = out.Write([]byte(fmt.Sprintf("style %s fill:#989695\n", deprecatedPackage)))
	}

	if len(deprecatedChannels) > 0 {
		for _, deprecatedChannel := range deprecatedChannels {
			_, _ = out.Write([]byte(fmt.Sprintf("style %s fill:#DCD0FF\n", deprecatedChannel)))
		}
	}

	return nil
}

// filters the channel edges to include only those which are greater-than-or-equal to the edge named by startVersion
// returns a nil channel if all edges are filtered out
func (writer *MermaidWriter) filterChannel(c *Channel, versionMap map[string]semver.Version, minVersion semver.Version, minEdgePackage string) *Channel {
	// short-circuit if no active filters
	if writer.MinEdgeName == "" && writer.SpecifiedPackageName == "" {
		return c
	}

	// short-circuit if channel's package doesn't match filter
	if writer.SpecifiedPackageName != "" && c.Package != writer.SpecifiedPackageName {
		return nil
	}

	// short-circuit if channel package is mismatch from filter
	if minEdgePackage != "" && c.Package != minEdgePackage {
		return nil
	}

	out := &Channel{Name: c.Name, Package: c.Package, Properties: c.Properties, Entries: []ChannelEntry{}}
	for _, ce := range c.Entries {
		filteredCe := ChannelEntry{Name: ce.Name}
		// nolint:nestif
		if writer.MinEdgeName == "" {
			// no minimum-edge specified
			filteredCe.SkipRange = ce.SkipRange
			filteredCe.Replaces = ce.Replaces
			filteredCe.Skips = append(filteredCe.Skips, ce.Skips...)

			// accumulate IFF there are any relevant skips/skipRange/replaces remaining or there never were any to begin with
			// for the case where all skip/skipRange/replaces are retained, this is effectively the original edge with validated linkages
			if len(filteredCe.Replaces) > 0 || len(filteredCe.Skips) > 0 || len(filteredCe.SkipRange) > 0 {
				out.Entries = append(out.Entries, filteredCe)
			} else {
				if len(ce.Replaces) == 0 && len(ce.SkipRange) == 0 && len(ce.Skips) == 0 {
					out.Entries = append(out.Entries, filteredCe)
				}
			}
		} else {
			if ce.Name == writer.MinEdgeName {
				// edge is the 'floor', meaning that since all references are "backward references", and we don't want any references from this edge
				// accumulate w/o references
				out.Entries = append(out.Entries, filteredCe)
			} else {
				// edge needs to be filtered to determine if it is below the floor (bad) or on/above (good)
				if len(ce.Replaces) > 0 && versionMap[ce.Replaces].GTE(minVersion) {
					filteredCe.Replaces = ce.Replaces
				}
				if len(ce.Skips) > 0 {
					filteredSkips := []string{}
					for _, s := range ce.Skips {
						if versionMap[s].GTE(minVersion) {
							filteredSkips = append(filteredSkips, s)
						}
					}
					if len(filteredSkips) > 0 {
						filteredCe.Skips = filteredSkips
					}
				}
				if len(ce.SkipRange) > 0 {
					skipRange, err := semver.ParseRange(ce.SkipRange)
					// if skipRange can't be parsed, just don't filter based on it
					if err == nil && skipRange(minVersion) {
						// specified range includes our floor
						filteredCe.SkipRange = ce.SkipRange
					}
				}
				// accumulate IFF there are any relevant skips/skipRange/replaces remaining, or there never were any to begin with (NOP)
				// but the edge name satisfies the minimum-edge constraint
				// for the case where all skip/skipRange/replaces are retained, this is effectively `ce` but with validated linkages
				if len(filteredCe.Replaces) > 0 || len(filteredCe.Skips) > 0 || len(filteredCe.SkipRange) > 0 {
					out.Entries = append(out.Entries, filteredCe)
				} else {
					if len(ce.Replaces) == 0 && len(ce.SkipRange) == 0 && len(ce.Skips) == 0 && versionMap[filteredCe.Name].GTE(minVersion) {
						out.Entries = append(out.Entries, filteredCe)
					}
				}
			}
		}
	}

	if len(out.Entries) > 0 {
		return out
	} else {
		return nil
	}
}

func parseVersionProperty(b *Bundle) (*semver.Version, error) {
	props, err := property.Parse(b.Properties)
	if err != nil {
		return nil, fmt.Errorf("parse properties for bundle %q: %v", b.Name, err)
	}
	if len(props.Packages) != 1 {
		return nil, fmt.Errorf("bundle %q has multiple %q properties, expected exactly 1", b.Name, property.TypePackage)
	}
	v, err := semver.Parse(props.Packages[0].Version)
	if err != nil {
		return nil, fmt.Errorf("bundle %q has invalid version %q: %v", b.Name, props.Packages[0].Version, err)
	}

	return &v, nil
}

func getBundleVersions(cfg *DeclarativeConfig) (map[string]semver.Version, error) {
	entries := make(map[string]semver.Version)
	for index := range cfg.Bundles {
		if _, ok := entries[cfg.Bundles[index].Name]; !ok {
			ver, err := parseVersionProperty(&cfg.Bundles[index])
			if err != nil {
				return entries, err
			}
			entries[cfg.Bundles[index].Name] = *ver
		}
	}

	return entries, nil
}

func (writer *MermaidWriter) getMinEdgePackage(cfg *DeclarativeConfig) string {
	if writer.MinEdgeName == "" {
		return ""
	}

	for _, c := range cfg.Channels {
		for _, ce := range c.Entries {
			if writer.MinEdgeName == ce.Name {
				return c.Package
			}
		}
	}

	return ""
}

func WriteJSON(cfg DeclarativeConfig, w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "    ")
	enc.SetEscapeHTML(false)
	return writeToEncoder(cfg, enc)
}

func WriteYAML(cfg DeclarativeConfig, w io.Writer) error {
	enc := newYAMLEncoder(w)
	enc.SetEscapeHTML(false)
	return writeToEncoder(cfg, enc)
}

type yamlEncoder struct {
	w          io.Writer
	escapeHTML bool
}

func newYAMLEncoder(w io.Writer) *yamlEncoder {
	return &yamlEncoder{w, true}
}

func (e *yamlEncoder) SetEscapeHTML(on bool) {
	e.escapeHTML = on
}

func (e *yamlEncoder) Encode(v interface{}) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(e.escapeHTML)
	if err := enc.Encode(v); err != nil {
		return err
	}
	yamlData, err := yaml.JSONToYAML(buf.Bytes())
	if err != nil {
		return err
	}
	yamlData = append([]byte("---\n"), yamlData...)
	_, err = e.w.Write(yamlData)
	return err
}

type encoder interface {
	Encode(interface{}) error
}

func writeToEncoder(cfg DeclarativeConfig, enc encoder) error {
	pkgNames := sets.NewString()

	packagesByName := map[string][]Package{}
	for _, p := range cfg.Packages {
		pkgName := p.Name
		pkgNames.Insert(pkgName)
		packagesByName[pkgName] = append(packagesByName[pkgName], p)
	}
	channelsByPackage := map[string][]Channel{}
	for _, c := range cfg.Channels {
		pkgName := c.Package
		pkgNames.Insert(pkgName)
		channelsByPackage[pkgName] = append(channelsByPackage[pkgName], c)
	}
	bundlesByPackage := map[string][]Bundle{}
	for _, b := range cfg.Bundles {
		pkgName := b.Package
		pkgNames.Insert(pkgName)
		bundlesByPackage[pkgName] = append(bundlesByPackage[pkgName], b)
	}
	othersByPackage := map[string][]Meta{}
	for _, o := range cfg.Others {
		pkgName := o.Package
		pkgNames.Insert(pkgName)
		othersByPackage[pkgName] = append(othersByPackage[pkgName], o)
	}
	deprecationsByPackage := map[string][]Deprecation{}
	for _, d := range cfg.Deprecations {
		pkgName := d.Package
		pkgNames.Insert(pkgName)
		deprecationsByPackage[pkgName] = append(deprecationsByPackage[pkgName], d)
	}

	for _, pName := range pkgNames.List() {
		if len(pName) == 0 {
			continue
		}
		pkgs := packagesByName[pName]
		for _, p := range pkgs {
			if err := enc.Encode(p); err != nil {
				return err
			}
		}

		channels := channelsByPackage[pName]
		sort.Slice(channels, func(i, j int) bool {
			return channels[i].Name < channels[j].Name
		})
		for _, c := range channels {
			if err := enc.Encode(c); err != nil {
				return err
			}
		}

		bundles := bundlesByPackage[pName]
		sort.Slice(bundles, func(i, j int) bool {
			return bundles[i].Name < bundles[j].Name
		})
		for _, b := range bundles {
			if err := enc.Encode(b); err != nil {
				return err
			}
		}

		others := othersByPackage[pName]
		sort.SliceStable(others, func(i, j int) bool {
			return others[i].Schema < others[j].Schema
		})
		for _, o := range others {
			if err := enc.Encode(o); err != nil {
				return err
			}
		}

		//
		// Normally we would order the deprecations, but it really doesn't make sense since
		// - there will be 0 or 1 of them for any given package
		// - they have no other useful field for ordering
		//
		// validation is typically via conversion to a model.Model and invoking model.Package.Validate()
		// It's possible that a user of the object could create a slice containing more then 1
		// Deprecation object for a package, and it would bypass validation if this
		// function gets called without conversion.
		//
		deprecations := deprecationsByPackage[pName]
		for _, d := range deprecations {
			if err := enc.Encode(d); err != nil {
				return err
			}
		}
	}

	for _, o := range othersByPackage[""] {
		if err := enc.Encode(o); err != nil {
			return err
		}
	}

	return nil
}

type WriteFunc func(config DeclarativeConfig, w io.Writer) error

func WriteFS(cfg DeclarativeConfig, rootDir string, writeFunc WriteFunc, fileExt string) error {
	channelsByPackage := map[string][]Channel{}
	for _, c := range cfg.Channels {
		channelsByPackage[c.Package] = append(channelsByPackage[c.Package], c)
	}
	bundlesByPackage := map[string][]Bundle{}
	for _, b := range cfg.Bundles {
		bundlesByPackage[b.Package] = append(bundlesByPackage[b.Package], b)
	}

	if err := os.MkdirAll(rootDir, 0777); err != nil {
		return err
	}

	for _, p := range cfg.Packages {
		fcfg := DeclarativeConfig{
			Packages: []Package{p},
			Channels: channelsByPackage[p.Name],
			Bundles:  bundlesByPackage[p.Name],
		}
		pkgDir := filepath.Join(rootDir, p.Name)
		if err := os.MkdirAll(pkgDir, 0777); err != nil {
			return err
		}
		filename := filepath.Join(pkgDir, fmt.Sprintf("catalog%s", fileExt))
		if err := writeFile(fcfg, filename, writeFunc); err != nil {
			return err
		}
	}
	return nil
}

func writeFile(cfg DeclarativeConfig, filename string, writeFunc WriteFunc) error {
	buf := &bytes.Buffer{}
	if err := writeFunc(cfg, buf); err != nil {
		return fmt.Errorf("write to buffer for %q: %v", filename, err)
	}
	if err := os.WriteFile(filename, buf.Bytes(), 0600); err != nil {
		return fmt.Errorf("write file %q: %v", filename, err)
	}
	return nil
}
