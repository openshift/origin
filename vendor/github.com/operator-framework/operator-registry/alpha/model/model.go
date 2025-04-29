package model

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/h2non/filetype"
	"github.com/h2non/filetype/matchers"
	"github.com/h2non/filetype/types"
	svg "github.com/h2non/go-is-svg"
	"golang.org/x/exp/maps"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/operator-framework/operator-registry/alpha/property"
)

type Deprecation struct {
	Message string `json:"message"`
}

func init() {
	t := types.NewType("svg", "image/svg+xml")
	filetype.AddMatcher(t, svg.Is)
	matchers.Image[types.NewType("svg", "image/svg+xml")] = svg.Is
}

type Model map[string]*Package

func (m Model) Validate() error {
	result := newValidationError("invalid index")

	for name, pkg := range m {
		if name != pkg.Name {
			result.subErrors = append(result.subErrors, fmt.Errorf("package key %q does not match package name %q", name, pkg.Name))
		}
		if err := pkg.Validate(); err != nil {
			result.subErrors = append(result.subErrors, err)
		}
	}
	return result.orNil()
}

type Package struct {
	Name           string
	Description    string
	Icon           *Icon
	DefaultChannel *Channel
	Channels       map[string]*Channel
	Deprecation    *Deprecation
}

func (m *Package) Validate() error {
	result := newValidationError(fmt.Sprintf("invalid package %q", m.Name))

	if m.Name == "" {
		result.subErrors = append(result.subErrors, errors.New("package name must not be empty"))
	}

	if err := m.Icon.Validate(); err != nil {
		result.subErrors = append(result.subErrors, err)
	}

	if m.DefaultChannel == nil {
		result.subErrors = append(result.subErrors, fmt.Errorf("default channel must be set"))
	}

	if len(m.Channels) == 0 {
		result.subErrors = append(result.subErrors, fmt.Errorf("package must contain at least one channel"))
	}

	foundDefault := false
	for name, ch := range m.Channels {
		if name != ch.Name {
			result.subErrors = append(result.subErrors, fmt.Errorf("channel key %q does not match channel name %q", name, ch.Name))
		}
		if err := ch.Validate(); err != nil {
			result.subErrors = append(result.subErrors, err)
		}
		if ch == m.DefaultChannel {
			foundDefault = true
		}
		if ch.Package != m {
			result.subErrors = append(result.subErrors, fmt.Errorf("channel %q not correctly linked to parent package", ch.Name))
		}
	}

	if err := m.validateUniqueBundleVersions(); err != nil {
		result.subErrors = append(result.subErrors, err)
	}

	if m.DefaultChannel != nil && !foundDefault {
		result.subErrors = append(result.subErrors, fmt.Errorf("default channel %q not found in channels list", m.DefaultChannel.Name))
	}

	if err := m.Deprecation.Validate(); err != nil {
		result.subErrors = append(result.subErrors, fmt.Errorf("invalid deprecation: %v", err))
	}

	return result.orNil()
}

func (m *Package) validateUniqueBundleVersions() error {
	versionsMap := map[string]semver.Version{}
	bundlesWithVersion := map[string]sets.Set[string]{}
	for _, ch := range m.Channels {
		for _, b := range ch.Bundles {
			versionsMap[b.Version.String()] = b.Version
			if bundlesWithVersion[b.Version.String()] == nil {
				bundlesWithVersion[b.Version.String()] = sets.New[string]()
			}
			bundlesWithVersion[b.Version.String()].Insert(b.Name)
		}
	}

	versionsSlice := maps.Values(versionsMap)
	semver.Sort(versionsSlice)

	var errs []error
	for _, v := range versionsSlice {
		bundles := sets.List(bundlesWithVersion[v.String()])
		if len(bundles) > 1 {
			errs = append(errs, fmt.Errorf("{%s: [%s]}", v, strings.Join(bundles, ", ")))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("duplicate versions found in bundles: %v", errs)
	}
	return nil
}

type Icon struct {
	Data      []byte `json:"base64data"`
	MediaType string `json:"mediatype"`
}

func (i *Icon) Validate() error {
	if i == nil {
		return nil
	}
	// TODO(joelanford): Should we check that data and mediatype are set,
	//   and detect the media type of the data and compare it to the
	//   mediatype listed in the icon field? Currently, some production
	//   index databases are failing these tests, so leaving this
	//   commented out for now.
	result := newValidationError("invalid icon")
	//if len(i.Data) == 0 {
	//	result.subErrors = append(result.subErrors, errors.New("icon data must be set if icon is defined"))
	//}
	//if len(i.MediaType) == 0 {
	//	result.subErrors = append(result.subErrors, errors.New("icon mediatype must be set if icon is defined"))
	//}
	//if len(i.Data) > 0 {
	//	if err := i.validateData(); err != nil {
	//		result.subErrors = append(result.subErrors, err)
	//	}
	//}
	return result.orNil()
}

// nolint:unused
func (i *Icon) validateData() error {
	if !filetype.IsImage(i.Data) {
		return errors.New("icon data is not an image")
	}
	t, err := filetype.Match(i.Data)
	if err != nil {
		return err
	}
	if t.MIME.Value != i.MediaType {
		return fmt.Errorf("icon media type %q does not match detected media type %q", i.MediaType, t.MIME.Value)
	}
	return nil
}

type Channel struct {
	Package     *Package
	Name        string
	Bundles     map[string]*Bundle
	Deprecation *Deprecation
	// NOTICE: The field Properties of the type Channel is for internal use only.
	//   DO NOT use it for any public-facing functionalities.
	//   This API is in alpha stage and it is subject to change.
	Properties []property.Property
}

// TODO(joelanford): This function determines the channel head by finding the bundle that has 0
//
//	incoming edges, based on replaces and skips. It also expects to find exactly one such bundle.
//	Is this the correct algorithm?
func (c Channel) Head() (*Bundle, error) {
	incoming := map[string]int{}
	for _, b := range c.Bundles {
		if b.Replaces != "" {
			incoming[b.Replaces]++
		}
		for _, skip := range b.Skips {
			incoming[skip]++
		}
	}
	var heads []*Bundle
	for _, b := range c.Bundles {
		if _, ok := incoming[b.Name]; !ok {
			heads = append(heads, b)
		}
	}
	if len(heads) == 0 {
		return nil, fmt.Errorf("no channel head found in graph")
	}
	if len(heads) > 1 {
		var headNames []string
		for _, head := range heads {
			headNames = append(headNames, head.Name)
		}
		sort.Strings(headNames)
		return nil, fmt.Errorf("multiple channel heads found in graph: %s", strings.Join(headNames, ", "))
	}
	return heads[0], nil
}

func (c *Channel) Validate() error {
	result := newValidationError(fmt.Sprintf("invalid channel %q", c.Name))

	if c.Name == "" {
		result.subErrors = append(result.subErrors, errors.New("channel name must not be empty"))
	}

	if c.Package == nil {
		result.subErrors = append(result.subErrors, errors.New("package must be set"))
	}

	if len(c.Bundles) == 0 {
		result.subErrors = append(result.subErrors, fmt.Errorf("channel must contain at least one bundle"))
	}

	if len(c.Bundles) > 0 {
		if err := c.validateReplacesChain(); err != nil {
			result.subErrors = append(result.subErrors, err)
		}
	}

	for name, b := range c.Bundles {
		if name != b.Name {
			result.subErrors = append(result.subErrors, fmt.Errorf("bundle key %q does not match bundle name %q", name, b.Name))
		}
		if err := b.Validate(); err != nil {
			result.subErrors = append(result.subErrors, err)
		}
		if b.Channel != c {
			result.subErrors = append(result.subErrors, fmt.Errorf("bundle %q not correctly linked to parent channel", b.Name))
		}
	}

	if err := c.Deprecation.Validate(); err != nil {
		result.subErrors = append(result.subErrors, fmt.Errorf("invalid deprecation: %v", err))
	}

	return result.orNil()
}

// validateReplacesChain checks the replaces chain of a channel.
// Specifically the following rules must be followed:
//  1. There must be exactly 1 channel head.
//  2. Beginning at the head, the replaces chain must reach all non-skipped entries.
//     Non-skipped entries are defined as entries that are not skipped by any other entry in the channel.
//  3. There must be no cycles in the replaces chain.
//  4. The tail entry in the replaces chain is permitted to replace a non-existent entry.
func (c *Channel) validateReplacesChain() error {
	head, err := c.Head()
	if err != nil {
		return err
	}

	allBundles := sets.NewString()
	skippedBundles := sets.NewString()
	for _, b := range c.Bundles {
		allBundles = allBundles.Insert(b.Name)
		skippedBundles = skippedBundles.Insert(b.Skips...)
	}

	chainFrom := map[string][]string{}
	replacesChainFromHead := sets.NewString(head.Name)
	cur := head
	for cur != nil {
		if _, ok := chainFrom[cur.Name]; !ok {
			chainFrom[cur.Name] = []string{cur.Name}
		}
		for k := range chainFrom {
			chainFrom[k] = append(chainFrom[k], cur.Replaces)
		}
		if replacesChainFromHead.Has(cur.Replaces) {
			return fmt.Errorf("detected cycle in replaces chain of upgrade graph: %s", strings.Join(chainFrom[cur.Replaces], " -> "))
		}
		replacesChainFromHead = replacesChainFromHead.Insert(cur.Replaces)
		cur = c.Bundles[cur.Replaces]
	}

	strandedBundles := allBundles.Difference(replacesChainFromHead).Difference(skippedBundles).List()
	if len(strandedBundles) > 0 {
		return fmt.Errorf("channel contains one or more stranded bundles: %s", strings.Join(strandedBundles, ", "))
	}

	return nil
}

type Bundle struct {
	Package       *Package
	Channel       *Channel
	Name          string
	Image         string
	Replaces      string
	Skips         []string
	SkipRange     string
	Properties    []property.Property
	RelatedImages []RelatedImage
	Deprecation   *Deprecation

	// These fields are present so that we can continue serving
	// the GRPC API the way packageserver expects us to in a
	// backwards-compatible way.
	Objects []string
	CsvJSON string

	// These fields are used to compare bundles in a diff.
	PropertiesP *property.Properties
	Version     semver.Version
}

func (b *Bundle) Validate() error {
	result := newValidationError(fmt.Sprintf("invalid bundle %q", b.Name))

	if b.Name == "" {
		result.subErrors = append(result.subErrors, errors.New("name must be set"))
	}
	if b.Channel == nil {
		result.subErrors = append(result.subErrors, errors.New("channel must be set"))
	}
	if b.Package == nil {
		result.subErrors = append(result.subErrors, errors.New("package must be set"))
	}
	if b.Channel != nil && b.Package != nil && b.Package != b.Channel.Package {
		result.subErrors = append(result.subErrors, errors.New("package does not match channel's package"))
	}
	props, err := property.Parse(b.Properties)
	if err != nil {
		result.subErrors = append(result.subErrors, err)
	}
	for i, skip := range b.Skips {
		if skip == "" {
			result.subErrors = append(result.subErrors, fmt.Errorf("skip[%d] is empty", i))
		}
	}
	// TODO(joelanford): Validate related images? It looks like some
	//   CSVs in production databases use incorrect fields ([name,value]
	//   instead of [name,image]), which results in empty image values.
	//   Example is in redhat-operators: 3scale-operator.v0.5.5
	//for i, relatedImage := range b.RelatedImages {
	//	if err := relatedImage.Validate(); err != nil {
	//		result.subErrors = append(result.subErrors, WithIndex(i, err))
	//	}
	//}

	if props != nil && len(props.Packages) != 1 {
		result.subErrors = append(result.subErrors, fmt.Errorf("must be exactly one property with type %q", property.TypePackage))
	}

	if b.Image == "" && len(b.Objects) == 0 {
		result.subErrors = append(result.subErrors, errors.New("bundle image must be set"))
	}

	if err := b.Deprecation.Validate(); err != nil {
		result.subErrors = append(result.subErrors, fmt.Errorf("invalid deprecation: %v", err))
	}

	return result.orNil()
}

type RelatedImage struct {
	Name  string
	Image string
}

func (i RelatedImage) Validate() error {
	result := newValidationError("invalid related image")
	if i.Image == "" {
		result.subErrors = append(result.subErrors, fmt.Errorf("image must be set"))
	}
	return result.orNil()
}

func (m Model) Normalize() {
	for _, pkg := range m {
		for _, ch := range pkg.Channels {
			for _, b := range ch.Bundles {
				for i := range b.Properties {
					// Ensure property value is encoded in a standard way.
					if normalized, err := property.Build(&b.Properties[i]); err == nil {
						b.Properties[i] = *normalized
					}
				}
			}
		}
	}
}

func (m Model) AddBundle(b Bundle) {
	if _, present := m[b.Package.Name]; !present {
		m[b.Package.Name] = b.Package
	}
	p := m[b.Package.Name]
	b.Package = p

	if ch, ok := p.Channels[b.Channel.Name]; ok {
		b.Channel = ch
		ch.Bundles[b.Name] = &b
	} else {
		newCh := &Channel{
			Name:    b.Channel.Name,
			Package: p,
			Bundles: make(map[string]*Bundle),
		}
		b.Channel = newCh
		newCh.Bundles[b.Name] = &b
		p.Channels[newCh.Name] = newCh
	}

	if p.DefaultChannel == nil {
		p.DefaultChannel = b.Channel
	}
}

func (d *Deprecation) Validate() error {
	if d == nil {
		return nil
	}
	if d.Message == "" {
		return errors.New("message must be set")
	}
	return nil
}
