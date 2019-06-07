package imageutil

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/blang/semver"

	digestinternal "github.com/openshift/library-go/pkg/image/internal/digest"
)

const (
	// DefaultImageTag is used when an image tag is needed and the configuration does not specify a tag to use.
	DefaultImageTag = "latest"
)

var ParseDigest = digestinternal.ParseDigest

// SplitImageStreamTag turns the name of an ImageStreamTag into Name and Tag.
// It returns false if the tag was not properly specified in the name.
func SplitImageStreamTag(nameAndTag string) (name string, tag string, ok bool) {
	parts := strings.SplitN(nameAndTag, ":", 2)
	name = parts[0]
	if len(parts) > 1 {
		tag = parts[1]
	}
	if len(tag) == 0 {
		tag = DefaultImageTag
	}
	return name, tag, len(parts) == 2
}

// SplitImageStreamImage turns the name of an ImageStreamImage into Name and ID.
// It returns false if the ID was not properly specified in the name.
func SplitImageStreamImage(nameAndID string) (name string, id string, ok bool) {
	parts := strings.SplitN(nameAndID, "@", 2)
	name = parts[0]
	if len(parts) > 1 {
		id = parts[1]
	}
	return name, id, len(parts) == 2
}

// JoinImageStreamTag turns a name and tag into the name of an ImageStreamTag
func JoinImageStreamTag(name, tag string) string {
	if len(tag) == 0 {
		tag = DefaultImageTag
	}
	return fmt.Sprintf("%s:%s", name, tag)
}

// JoinImageStreamImage creates a name for image stream image object from an image stream name and an id.
func JoinImageStreamImage(name, id string) string {
	return fmt.Sprintf("%s@%s", name, id)
}

// ParseImageStreamTagName splits a string into its name component and tag component, and returns an error
// if the string is not in the right form.
func ParseImageStreamTagName(istag string) (name string, tag string, err error) {
	if strings.Contains(istag, "@") {
		err = fmt.Errorf("%q is an image stream image, not an image stream tag", istag)
		return
	}
	segments := strings.SplitN(istag, ":", 3)
	switch len(segments) {
	case 2:
		name = segments[0]
		tag = segments[1]
		if len(name) == 0 || len(tag) == 0 {
			err = fmt.Errorf("image stream tag name %q must have a name and a tag", istag)
		}
	default:
		err = fmt.Errorf("expected exactly one : delimiter in the istag %q", istag)
	}
	return
}

var (
	reMinorSemantic  = regexp.MustCompile(`^[\d]+\.[\d]+$`)
	reMinorWithPatch = regexp.MustCompile(`^([\d]+\.[\d]+)-\w+$`)
)

type tagPriority int

const (
	// the "latest" tag
	tagPriorityLatest tagPriority = iota

	// a semantic minor version ("5.1", "v5.1", "v5.1-rc1")
	tagPriorityMinor

	// a full semantic version ("5.1.3-other", "v5.1.3-other")
	tagPriorityFull

	// other tags
	tagPriorityOther
)

type prioritizedTag struct {
	tag      string
	priority tagPriority
	semver   semver.Version
	prefix   string
}

func prioritizeTag(tag string) prioritizedTag {
	if tag == "latest" {
		return prioritizedTag{
			tag:      tag,
			priority: tagPriorityLatest,
		}
	}

	short := tag
	prefix := ""
	if strings.HasPrefix(tag, "v") {
		prefix = "v"
		short = tag[1:]
	}

	// 5.1.3
	if v, err := semver.Parse(short); err == nil {
		return prioritizedTag{
			tag:      tag,
			priority: tagPriorityFull,
			semver:   v,
			prefix:   prefix,
		}
	}

	// 5.1
	if reMinorSemantic.MatchString(short) {
		if v, err := semver.Parse(short + ".0"); err == nil {
			return prioritizedTag{
				tag:      tag,
				priority: tagPriorityMinor,
				semver:   v,
				prefix:   prefix,
			}
		}
	}

	// 5.1-rc1
	if match := reMinorWithPatch.FindStringSubmatch(short); match != nil {
		if v, err := semver.Parse(strings.Replace(short, match[1], match[1]+".0", 1)); err == nil {
			return prioritizedTag{
				tag:      tag,
				priority: tagPriorityMinor,
				semver:   v,
				prefix:   prefix,
			}
		}
	}

	// other
	return prioritizedTag{
		tag:      tag,
		priority: tagPriorityOther,
		prefix:   prefix,
	}
}

type prioritizedTags []prioritizedTag

func (t prioritizedTags) Len() int      { return len(t) }
func (t prioritizedTags) Swap(i, j int) { t[i], t[j] = t[j], t[i] }
func (t prioritizedTags) Less(i, j int) bool {
	if t[i].priority != t[j].priority {
		return t[i].priority < t[j].priority
	}

	if t[i].priority == tagPriorityOther {
		return t[i].tag < t[j].tag
	}

	cmp := t[i].semver.Compare(t[j].semver)
	if cmp > 0 { // the newer tag has a higher priority
		return true
	}
	return cmp == 0 && t[i].prefix < t[j].prefix
}

// PrioritizeTags orders a set of image tags with a few conventions:
//
// 1. the "latest" tag, if present, should be first
// 2. any tags that represent a semantic minor version ("5.1", "v5.1", "v5.1-rc1") should be next, in descending order
// 3. any tags that represent a full semantic version ("5.1.3-other", "v5.1.3-other") should be next, in descending order
// 4. any remaining tags should be sorted in lexicographic order
//
// The method updates the tags in place.
func PrioritizeTags(tags []string) {
	ptags := make(prioritizedTags, len(tags))
	for i, tag := range tags {
		ptags[i] = prioritizeTag(tag)
	}
	sort.Sort(ptags)
	for i, pt := range ptags {
		tags[i] = pt.tag
	}
}
