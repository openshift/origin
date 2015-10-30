package graph

import (
	"sort"

	osgraph "github.com/openshift/origin/pkg/api/graph"
	buildapi "github.com/openshift/origin/pkg/build/api"
	buildgraph "github.com/openshift/origin/pkg/build/graph/nodes"
)

// RelevantBuilds returns the lastSuccessful build, lastUnsuccesful build, and a list of active builds
func RelevantBuilds(g osgraph.Graph, bcNode *buildgraph.BuildConfigNode) (*buildgraph.BuildNode, *buildgraph.BuildNode, []*buildgraph.BuildNode) {
	var (
		lastSuccessfulBuild   *buildgraph.BuildNode
		lastUnsuccessfulBuild *buildgraph.BuildNode
	)
	activeBuilds := []*buildgraph.BuildNode{}
	allBuilds := []*buildgraph.BuildNode{}
	uncastBuilds := g.SuccessorNodesByEdgeKind(bcNode, BuildEdgeKind)

	for i := range uncastBuilds {
		buildNode := uncastBuilds[i].(*buildgraph.BuildNode)
		if belongsToBuildConfig(bcNode.BuildConfig, buildNode.Build) {
			allBuilds = append(allBuilds, buildNode)
		}
	}

	if len(allBuilds) == 0 {
		return nil, nil, []*buildgraph.BuildNode{}
	}

	sort.Sort(RecentBuildReferences(allBuilds))

	for i := range allBuilds {
		switch allBuilds[i].Build.Status.Phase {
		case buildapi.BuildPhaseComplete:
			if lastSuccessfulBuild == nil {
				lastSuccessfulBuild = allBuilds[i]
			}
		case buildapi.BuildPhaseFailed, buildapi.BuildPhaseCancelled, buildapi.BuildPhaseError:
			if lastUnsuccessfulBuild == nil {
				lastUnsuccessfulBuild = allBuilds[i]
			}
		default:
			activeBuilds = append(activeBuilds, allBuilds[i])
		}
	}

	return lastSuccessfulBuild, lastUnsuccessfulBuild, activeBuilds
}

func belongsToBuildConfig(config *buildapi.BuildConfig, b *buildapi.Build) bool {
	if b.Labels == nil {
		return false
	}
	if b.Labels[buildapi.DeprecatedBuildConfigLabel] == config.Name {
		return true
	}
	return false
}

type RecentBuildReferences []*buildgraph.BuildNode

func (m RecentBuildReferences) Len() int      { return len(m) }
func (m RecentBuildReferences) Swap(i, j int) { m[i], m[j] = m[j], m[i] }
func (m RecentBuildReferences) Less(i, j int) bool {
	return m[i].Build.CreationTimestamp.After(m[j].Build.CreationTimestamp.Time)
}

func defaultNamespace(value, defaultValue string) string {
	if len(value) == 0 {
		return defaultValue
	}
	return value
}
