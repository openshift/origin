package updateservice

// Graph is a Cincinnati update-service graph as served at the /graph
// endpoint. See https://github.com/openshift/cincinnati.
type Graph struct {
	Nodes            []Node            `json:"nodes"`
	Edges            []Edge            `json:"edges,omitempty"`
	ConditionalEdges []ConditionalEdge `json:"conditionalEdges,omitempty"`
}

// Node is a release in the graph. The CVO requires a node whose Version
// exactly equals its current release version, and copies Payload verbatim
// into status.availableUpdates entries.
type Node struct {
	Version  string            `json:"version"`
	Payload  string            `json:"payload"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Edge is an [origin, destination] pair of node indices.
type Edge [2]int

// ConditionalEdge is a conditional update path with associated risks.
type ConditionalEdge struct {
	Edges []VersionEdge `json:"edges"`
	Risks []Risk        `json:"risks"`
}

// VersionEdge is a conditional edge between two release versions.
type VersionEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// Risk describes why a conditional update is not recommended.
type Risk struct {
	URL           string         `json:"url"`
	Name          string         `json:"name"`
	Message       string         `json:"message"`
	MatchingRules []MatchingRule `json:"matchingRules"`
}

// MatchingRule matches the clusters affected by a Risk.
type MatchingRule struct {
	Type   string  `json:"type"`
	PromQL *PromQL `json:"promql,omitempty"`
}

// PromQL is the payload of a PromQL matching rule.
type PromQL struct {
	PromQL string `json:"promql"`
}

// TwoNodeGraph returns a graph with a current node matching the cluster's
// release (so the CVO, which requires its exact current version in the
// graph, finds it) and a single edge advertising target as an available
// update. channels, when non-empty, is advertised as the current node's
// release channels in its metadata.
func TwoNodeGraph(channels, currentVersion, currentPayload, targetVersion, targetPayload string) Graph {
	var metadata map[string]string
	if channels != "" {
		metadata = map[string]string{"io.openshift.upgrades.graph.release.channels": channels}
	}
	return Graph{
		Nodes: []Node{
			{Version: currentVersion, Payload: currentPayload, Metadata: metadata},
			{Version: targetVersion, Payload: targetPayload},
		},
		Edges: []Edge{{0, 1}},
	}
}
