package apidocs

import (
	"path/filepath"
	"sort"
	"strings"
)

// Topic represents an asciibinder topic
type Topic struct {
	Name    string  `yaml:"Name"`
	File    string  `yaml:"File,omitempty"`
	Dir     string  `yaml:"Dir,omitempty"`
	Distros string  `yaml:"Distros,omitempty"`
	Topics  []Topic `yaml:"Topics,omitempty"`
}

func BuildTopics(pages Pages) []Topic {
	m := make(map[string]Topic)

	for _, page := range pages {
		path := page.OutputPath()

		parentName := page.ParentTopicName()
		parent, found := m[parentName]
		if !found {
			parent = Topic{
				Name: parentName,
				Dir:  filepath.Base(filepath.Dir(path)),
			}
		}

		child := Topic{
			Name: page.Title(),
			File: strings.TrimSuffix(filepath.Base(path), ".adoc"),
		}
		parent.Topics = append(parent.Topics, child)

		m[parentName] = parent
	}

	parents := make([]Topic, 0, len(m))
	for _, parent := range m {
		sort.Sort(childTopicsByName(parent.Topics))
		parents = append(parents, parent)
	}
	sort.Sort(parentTopicsByVersion(parents))
	sort.Stable(parentTopicsByGroup(parents))
	sort.Stable(parentTopicsByRoot(parents))

	return parents
}
