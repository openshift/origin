package haproxy

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	// listMapHeader is the header added if required to the "show map"
	// output from haproxy, so that we can parse the CSV output.
	// Note: This should match the CSV tags used in mapListEntry.
	showMapListHeader = "id (file) description"

	// showMapHeader is the header we add to the "show map $name"
	// output from haproxy, so that we can parse the CSV output.
	// Note: This should match the CSV tags used in HAProxyMapEntry.
	showMapHeader = "id name value"
)

type mapListEntry struct {
	ID     string `csv:"id"`
	Name   string `csv:"(file)"`
	Unused string `csv:"-"`
}

// HAPrroxyMapEntry is an entry in HAProxyMap.
type HAProxyMapEntry struct {
	// ID is the internal haproxy id associated with this map entry.
	// It is required for deleting map entries.
	ID string `csv:"id"`

	// Name is the entry key.
	Name string `csv:"name"`

	// Value is the entry value.
	Value string `csv:"value"`
}

// HAProxyMap is a structure representing an haproxy map.
type HAProxyMap struct {
	// name is the haproxy specific name for this map.
	name string

	// client is the haproxy dynamic API client.
	client *Client

	// entries are the haproxy map entries.
	// Note: This is _not_ a hashtable/map/dict as it can have
	// duplicate entries with the same key.
	entries []*HAProxyMapEntry

	// dirty indicates the state of the map.
	dirty bool
}

// buildHAProxyMaps builds and returns a list of haproxy maps.
// Note: Maps are lazily populated based on their usage.
func buildHAProxyMaps(c *Client) ([]*HAProxyMap, error) {
	entries := []*mapListEntry{}
	converter := NewCSVConverter(showMapListHeader, &entries, fixupMapListOutput)

	if _, err := c.RunCommand("show map", converter); err != nil {
		return []*HAProxyMap{}, err
	}

	maps := make([]*HAProxyMap, len(entries))
	for k, v := range entries {
		m := newHAProxyMap(v.Name, c)
		maps[k] = m
	}

	return maps, nil
}

// newHAProxyMap returns a new HAProxyMap representing a haproxy map.
func newHAProxyMap(name string, client *Client) *HAProxyMap {
	return &HAProxyMap{
		name:    name,
		client:  client,
		entries: make([]*HAProxyMapEntry, 0),
		dirty:   true,
	}
}

// Refresh refreshes the data in this haproxy map.
func (m *HAProxyMap) Refresh() error {
	cmd := fmt.Sprintf("show map %s", m.name)
	converter := NewCSVConverter(showMapHeader, &m.entries, nil)
	if _, err := m.client.RunCommand(cmd, converter); err != nil {
		return err
	}

	m.dirty = false
	return nil
}

// Commit commits all the pending changes made to this haproxy map.
// We do map changes "in-band" as that's handled dynamically by haproxy.
func (m *HAProxyMap) Commit() error {
	// noop
	return nil
}

// Name returns the name of this map.
func (m *HAProxyMap) Name() string {
	return m.name
}

// Find returns a list of matching entries in the haproxy map.
func (m *HAProxyMap) Find(k string) ([]HAProxyMapEntry, error) {
	found := make([]HAProxyMapEntry, 0)

	if m.dirty {
		if err := m.Refresh(); err != nil {
			return found, err
		}
	}

	for _, entry := range m.entries {
		if entry.Name == k {
			clonedEntry := HAProxyMapEntry{
				ID:    entry.ID,
				Name:  entry.Name,
				Value: entry.Value,
			}
			found = append(found, clonedEntry)
		}
	}

	return found, nil
}

// Add adds a new key and value to the haproxy map and allows all previous
// entries in the map to be deleted (replaced).
func (m *HAProxyMap) Add(k, v string, replace bool) error {
	if replace {
		if err := m.Delete(k); err != nil {
			return err
		}
	}

	return m.addEntry(k, v)
}

// Delete removes all the matching keys from the haproxy map.
func (m *HAProxyMap) Delete(k string) error {
	entries, err := m.Find(k)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if err := m.deleteEntry(entry.ID); err != nil {
			return err
		}
	}

	return nil
}

// DeleteEntry removes a specific haproxy map entry.
func (m *HAProxyMap) DeleteEntry(id string) error {
	return m.deleteEntry(id)
}

// addEntry adds a new haproxy map entry.
func (m *HAProxyMap) addEntry(k, v string) error {
	keyExpr := escapeKeyExpr(k)
	cmd := fmt.Sprintf("add map %s %s %s", m.name, keyExpr, v)
	responseBytes, err := m.client.Execute(cmd)
	if err != nil {
		return err
	}

	response := strings.TrimSpace(string(responseBytes))
	if len(response) > 0 {
		return fmt.Errorf("adding map %s entry %s: %v", m.name, keyExpr, string(response))
	}

	m.dirty = true
	return nil
}

// deleteEntry removes a specific haproxy map entry.
func (m *HAProxyMap) deleteEntry(id string) error {
	cmd := fmt.Sprintf("del map %s #%s", m.name, id)
	if _, err := m.client.Execute(cmd); err != nil {
		return err
	}

	m.dirty = true
	return nil
}

// escapeKeyExpr escapes meta characters in the haproxy map entry key name.
func escapeKeyExpr(k string) string {
	v := strings.Replace(k, `\`, `\\`, -1)
	return strings.Replace(v, `.`, `\.`, -1)
}

// Regular expression to fixup haproxy map list funky output.
var listMapOutputRE *regexp.Regexp = regexp.MustCompile(`(?m)^(-|)([0-9]*) \((.*)?\).*$`)

// fixupMapListOutput fixes up the funky output haproxy "show map" returns.
func fixupMapListOutput(data []byte) ([]byte, error) {
	replacement := []byte(`$1$2 $3 loaded`)
	return listMapOutputRE.ReplaceAll(data, replacement), nil
}
