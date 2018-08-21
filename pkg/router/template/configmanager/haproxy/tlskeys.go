package haproxy

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	// showTLSKeysListHeader is a header added if needed to "show tls-keys"
	// command output from haproxy, so that we can parse the CSV output.
	// Note: This should match the CSV tags used in tlsKeysListEntry.
	showTLSKeysListHeader = "id (file)"

	// showTLSKeysGroupHeader is the header added to "show tls-keys #<id>"
	// output from haproxy, so that we can parse the CSV output.
	// Note: This should match the CSV tags used in TLSKeyEntry.
	showTLSKeysGroupHeader = "id secret"
)

type tlsKeysListEntry struct {
	ID   string `csv:"id"`
	Name string `csv:"(file)"`
}

// TLSKeyEntry is an entry in the TLS keys group.
type TLSKeyEntry struct {
	// ID is the internal haproxy id associated with this TLS key entry.
	// It is required for deleting entries.
	ID string `csv:"id"`

	// Secret is the TLS ticket key value.
	Secret string `csv:"secret"`
}

// TLSKeyGroup is a structure representing TLS keys as grouped in haproxy.
type TLSKeyGroup struct {
	// id is the haproxy specific id for this TLS keys group.
	id string

	// name is the haproxy specific name for this TLS keys group.
	name string

	// client is the haproxy dynamic API client.
	client *Client

	// entries are the TLS key entries.
	entries []*TLSKeyEntry

	// dirty indicates the state of this grouping.
	dirty bool
}

// buildTLSKeyGroups builds and returns a list of grouped TLS keys.
// Note: TLS keys are lazily populated based on their usage.
func buildTLSKeyGroups(c *Client) ([]*TLSKeyGroup, error) {
	entries := []*tlsKeysListEntry{}
	converter := NewCSVConverter(showTLSKeysListHeader, &entries, fixupTLSKeysListOutput)

	if _, err := c.RunCommand("show tls-keys", converter); err != nil {
		return []*TLSKeyGroup{}, err
	}

	groups := make([]*TLSKeyGroup, len(entries))
	for k, v := range entries {
		m := newTLSKeyGroup(v.ID, v.Name, c)
		groups[k] = m
	}

	return groups, nil
}

// newTLSKeyGroup returns a new TLS keys grouping.
func newTLSKeyGroup(id, name string, client *Client) *TLSKeyGroup {
	return &TLSKeyGroup{
		id:      id,
		name:    name,
		client:  client,
		entries: make([]*TLSKeyEntry, 0),
		dirty:   true,
	}
}

// Refresh refreshes the data in this TLS keys grouping.
func (m *TLSKeyGroup) Refresh() error {
	cmd := fmt.Sprintf("show tls-keys #%s", m.id)

	converter := NewCSVConverter(showTLSKeysGroupHeader, &m.entries, fixupTLSKeysGroupOutput)
	if _, err := m.client.RunCommand(cmd, converter); err != nil {
		return err
	}

	m.dirty = false
	return nil
}

// Commit commits all the pending changes made to this haproxy TLS keys group.
// We do TLS keys group changes "in-band" as that's handled dynamically by
// haproxy.
func (m *TLSKeyGroup) Commit() error {
	// noop
	return nil
}

// Name returns the name of this TLS keys group.
func (m *TLSKeyGroup) Name() string {
	return m.name
}

// UpdateKeys updates the keys in this TLS keys group.
func (m *TLSKeyGroup) UpdateKeys(keys []string) error {
	for _, v := range keys {
		cmd := fmt.Sprintf("set ssl tls-key %s %s", m.name, v)
		responseBytes, err := m.client.Execute(cmd)
		if err != nil {
			return err
		}

		response := strings.TrimSpace(string(responseBytes))
		if len(response) > 0 && !strings.HasPrefix(response, "TLS ticket key updated") {
			return fmt.Errorf("set ssl tls-key %s: %v", m.name, string(response))
		}

	}
	return nil
}

// Regular expression to fixup TLS keys list output.
var listTLSKeysOutputRE *regexp.Regexp = regexp.MustCompile(`(?m)^([0-9]+)\s+\((.*)?\)\s*$`)

// fixupTLSKeysListOutput fixes up the output haproxy "show tls-keys" returns.
func fixupTLSKeysListOutput(data []byte) ([]byte, error) {
	replacement := []byte(`$1 $2`)
	return listTLSKeysOutputRE.ReplaceAll(data, replacement), nil
}

// Regular expression to fixup TLS keys group output.
var tlsKeysGroupsOutputRE *regexp.Regexp = regexp.MustCompile(`(?m)^#\s*[0-9]+\s*.*?$[\r\n]+`)

// fixupTLSKeysGroupOutput fixes up the output of haproxy "show tls-keys #<id>".
func fixupTLSKeysGroupOutput(data []byte) ([]byte, error) {
	replacement := []byte("")
	return tlsKeysGroupsOutputRE.ReplaceAll(data, replacement), nil
}
