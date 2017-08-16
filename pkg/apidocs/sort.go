package apidocs

import (
	"strings"
)

// sort by operation.PathName
type byPathName []operation

func (o byPathName) Len() int           { return len(o) }
func (o byPathName) Swap(i, j int)      { o[i], o[j] = o[j], o[i] }
func (o byPathName) Less(i, j int) bool { return o[i].PathName < o[j].PathName }

// sort by operation.Subresource
type bySubresource []operation

func (o bySubresource) Len() int           { return len(o) }
func (o bySubresource) Swap(i, j int)      { o[i], o[j] = o[j], o[i] }
func (o bySubresource) Less(i, j int) bool { return o[i].Subresource() < o[j].Subresource() }

// sort by operation.Namespaced
type byNamespaced []operation

func (o byNamespaced) Len() int           { return len(o) }
func (o byNamespaced) Swap(i, j int)      { o[i], o[j] = o[j], o[i] }
func (o byNamespaced) Less(i, j int) bool { return o[j].Namespaced() && !o[i].Namespaced() }

// sort by operation.IsProxy
type byProxy []operation

func (o byProxy) Len() int           { return len(o) }
func (o byProxy) Swap(i, j int)      { o[i], o[j] = o[j], o[i] }
func (o byProxy) Less(i, j int) bool { return o[j].IsProxy() && !o[i].IsProxy() }

// sort by operation.Plural
type byPlural []operation

func (o byPlural) Len() int           { return len(o) }
func (o byPlural) Swap(i, j int)      { o[i], o[j] = o[j], o[i] }
func (o byPlural) Less(i, j int) bool { return o[j].Plural() && !o[i].Plural() }

// sort by operation.Verb
type byOperationVerb []operation

var byOperationVerbOrder = map[string]int{
	"Options": 1,
	"Create":  2,
	"Head":    3,
	"Get":     4,
	"Watch":   5,
	"Update":  6,
	"Patch":   7,
	"Delete":  8,
}

func (o byOperationVerb) Len() int      { return len(o) }
func (o byOperationVerb) Swap(i, j int) { o[i], o[j] = o[j], o[i] }
func (o byOperationVerb) Less(i, j int) bool {
	return byOperationVerbOrder[o[i].Verb()] < byOperationVerbOrder[o[j].Verb()]
}

// sort parent topic by root (/api, /apis, /oapi)
type parentTopicsByRoot []Topic

func (o parentTopicsByRoot) Len() int      { return len(o) }
func (o parentTopicsByRoot) Swap(i, j int) { o[i], o[j] = o[j], o[i] }
func (o parentTopicsByRoot) Less(i, j int) bool {
	ip, jp := strings.Split(strings.Trim(o[i].Name, "/"), "/"), strings.Split(strings.Trim(o[j].Name, "/"), "/")
	return ip[0] < jp[0]
}

// sort parent topics by group (no group first, then sorted groups with no
// sub-part (apps, extensions, etc.), then groups with sub-parts by reversed
// sub-part
type parentTopicsByGroup []Topic

func (o parentTopicsByGroup) Len() int      { return len(o) }
func (o parentTopicsByGroup) Swap(i, j int) { o[i], o[j] = o[j], o[i] }
func (o parentTopicsByGroup) Less(i, j int) bool {
	ip, jp := strings.Split(strings.Trim(o[i].Name, "/"), "/"), strings.Split(strings.Trim(o[j].Name, "/"), "/")

	var ig, jg string // api groups, reversed by '.'
	if len(ip) == 3 {
		ig = strings.Join(ReverseStringSlice(strings.Split(ip[1], ".")), ".")
	}
	if len(jp) == 3 {
		jg = strings.Join(ReverseStringSlice(strings.Split(jp[1], ".")), ".")
	}

	if strings.Contains(ig, ".") == strings.Contains(jg, ".") {
		return ig < jg
	}
	return strings.Contains(jg, ".") && !strings.Contains(ig, ".") // group without '.' wins
}

// sort parent topics by version
type parentTopicsByVersion []Topic

func (o parentTopicsByVersion) Len() int      { return len(o) }
func (o parentTopicsByVersion) Swap(i, j int) { o[i], o[j] = o[j], o[i] }
func (o parentTopicsByVersion) Less(i, j int) bool {
	ip, jp := strings.Split(strings.Trim(o[i].Name, "/"), "/"), strings.Split(strings.Trim(o[j].Name, "/"), "/")
	return ip[len(ip)-1] < jp[len(jp)-1]
}

// sort child topics by name
type childTopicsByName []Topic

func (o childTopicsByName) Len() int           { return len(o) }
func (o childTopicsByName) Swap(i, j int)      { o[i], o[j] = o[j], o[i] }
func (o childTopicsByName) Less(i, j int) bool { return o[i].Name < o[j].Name }
