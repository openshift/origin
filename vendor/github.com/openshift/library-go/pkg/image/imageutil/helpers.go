package imageutil

import (
	"fmt"
	"strings"
)

const (
	// DefaultImageTag is used when an image tag is needed and the configuration does not specify a tag to use.
	DefaultImageTag = "latest"
)

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
