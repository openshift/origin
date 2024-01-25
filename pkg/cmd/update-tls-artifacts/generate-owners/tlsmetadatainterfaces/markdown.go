package tlsmetadatainterfaces

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphapi"
)

type Markdown struct {
	title           string
	tableOfContents *bytes.Buffer
	body            *bytes.Buffer

	orderedListDepth      int
	orderedListItemStart  bool
	orderedListItemNumber []int
}

func NewMarkdown(topTitle string) *Markdown {
	return &Markdown{
		title:                 topTitle,
		tableOfContents:       &bytes.Buffer{},
		body:                  &bytes.Buffer{},
		orderedListItemNumber: make([]int, 10), // nine nested ordered lists should be enough for anyone
	}
}

func (m *Markdown) Bytes() []byte {
	ret := &bytes.Buffer{}
	fmt.Fprintf(ret, "# %s\n\n", m.title)
	fmt.Fprintf(ret, "## Table of Contents\n")
	fmt.Fprintf(ret, m.tableOfContents.String())
	fmt.Fprintln(ret, "")
	fmt.Fprintln(ret, "")
	fmt.Fprintf(ret, m.body.String())
	return ret.Bytes()
}

// ExactBytes returns markdown with table of contents or title.  Useful for embedding.
func (m *Markdown) ExactBytes() []byte {
	ret := &bytes.Buffer{}
	fmt.Fprintf(ret, m.body.String())
	return ret.Bytes()
}

func (m *Markdown) UnlistedTitle(level int, text string) {
	titlePrefix := strings.Repeat("#", level)
	fmt.Fprintf(m.body, "%s %s\n", titlePrefix, text)
}

func (m *Markdown) Title(level int, text string) {
	m.UnlistedTitle(level, text)

	tocLeadingSpace := strings.Repeat("  ", level-1)
	tocLink := strings.ReplaceAll(text, " ", "-")
	tocLink = strings.ReplaceAll(tocLink, "(", "")
	tocLink = strings.ReplaceAll(tocLink, ")", "")
	fmt.Fprintf(m.tableOfContents, "%s- [%s](#%s)\n", tocLeadingSpace, text, tocLink)
}

func (m *Markdown) ExactText(text string) {
	if m.orderedListDepth == 0 {
		fmt.Fprintf(m.body, "%s\n", text)
		return
	}

	prefix := ""
	if m.orderedListItemStart {
		prefix = fmt.Sprintf("%d. ", m.orderedListItemNumber[m.orderedListDepth])
		m.orderedListItemStart = false
	} else {
		prefix = "      "
	}
	fmt.Fprintf(m.body, "%s%s\n", prefix, text)
}

func (m *Markdown) Text(text string) {
	m.ExactText(EscapeForLiteral(text))
}

func (m *Markdown) ExactTextf(format string, args ...any) {
	line := fmt.Sprintf(format, args...)
	m.ExactText(line)
}

func (m *Markdown) Textf(format string, args ...any) {
	line := fmt.Sprintf(format, args...)
	m.Text(line)
}

func (m *Markdown) OrderedListStart() {
	m.orderedListDepth++
	m.orderedListItemStart = true
	m.orderedListItemNumber[m.orderedListDepth] = 0
}

func (m *Markdown) NewOrderedListItem() {
	m.orderedListItemStart = true
	m.orderedListItemNumber[m.orderedListDepth] += 1
}

func (m *Markdown) OrderedListEnd() {
	m.orderedListDepth--
	if m.orderedListDepth < 0 {
		m.orderedListDepth = 0
	}
}

func (m *Markdown) PrintCABundleName(curr certgraphapi.PKIRegistryCABundle) {
	switch {
	case curr.InClusterLocation != nil:
		m.Textf("ns/%v configmap/%v\n", curr.InClusterLocation.ConfigMapLocation.Namespace, curr.InClusterLocation.ConfigMapLocation.Name)
	case curr.OnDiskLocation != nil:
		m.Textf("%v\n", curr.OnDiskLocation.OnDiskLocation.Path)
	}
}

func (m *Markdown) PrintCertKeyName(curr certgraphapi.PKIRegistryCertKeyPair) {
	switch {
	case curr.InClusterLocation != nil:
		m.Textf("ns/%v secret/%v\n", curr.InClusterLocation.SecretLocation.Namespace, curr.InClusterLocation.SecretLocation.Name)
	case curr.OnDiskLocation != nil:
		m.Textf("%v\n", curr.OnDiskLocation.OnDiskLocation.Path)
	}
}

// EscapeForLiteral escapes common characters so they render properly
func EscapeForLiteral(in string) string {
	ret := strings.ReplaceAll(in, "<", `\<`)
	return ret
}
