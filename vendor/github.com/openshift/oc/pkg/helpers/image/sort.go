package image

import (
	"sort"
	"time"

	imagev1 "github.com/openshift/api/image/v1"
)

type tag struct {
	Name    string
	Created time.Time
}

type byCreationTimestamp []tag

func (t byCreationTimestamp) Len() int {
	return len(t)
}

func (t byCreationTimestamp) Less(i, j int) bool {
	delta := t[i].Created.Sub(t[j].Created)
	if delta > 0 {
		return true
	}
	if delta < 0 {
		return false
	}
	// time has only second precision, so we need to have a secondary sort condition
	// to get stable sorts
	return t[i].Name < t[j].Name
}

func (t byCreationTimestamp) Swap(i, j int) {
	t[i], t[j] = t[j], t[i]
}

// SortStatusTags sorts the status tags of an image stream based on
// the latest created
func SortStatusTags(tags []imagev1.NamedTagEventList) []string {
	tagSlice := make([]tag, len(tags))
	index := 0
	for _, tag := range tags {
		tagSlice[index].Name = tag.Tag
		if len(tag.Items) > 0 {
			tagSlice[index].Created = tag.Items[0].Created.Round(time.Second)
		}
		index++
	}

	sort.Sort(byCreationTimestamp(tagSlice))

	actual := make([]string, len(tagSlice))
	for i, tag := range tagSlice {
		actual[i] = tag.Name
	}

	return actual
}
