package cmd

import (
	"testing"

	"github.com/openshift/origin/pkg/oc/generate"
	"github.com/openshift/origin/pkg/oc/generate/app"
)

// TestResolveJenkinsfileAndDockerfile ensures that if a repo has a Jenkinsfile
// and a Dockerfile, we use the Jenkinsfile.
func TestResolveJenkinsfileAndDockerfile(t *testing.T) {
	dockerfile, _ := app.NewDockerfile("FROM centos\n")
	i := app.SourceRepositoryInfo{Dockerfile: dockerfile, Jenkinsfile: true}

	repo := app.SourceRepository{}
	repo.SetInfo(&i)
	repositories := app.SourceRepositories{&repo}

	resolvers := Resolvers{}
	componentrefs, err := AddMissingComponentsToRefBuilder(&app.ReferenceBuilder{}, repositories, resolvers.DockerfileResolver(), resolvers.SourceResolver(), resolvers.PipelineResolver(), &GenerationInputs{})

	checkResolveResult(t, componentrefs, err, generate.StrategyPipeline)
}

// TestResolveJenkinsfileAndSource ensures that if a repo has a Jenkinsfile and
// source, we use the Jenkinsfile.
func TestResolveJenkinsfileAndSource(t *testing.T) {
	i := app.SourceRepositoryInfo{Jenkinsfile: true, Types: []app.SourceLanguageType{{Platform: "foo"}}}

	repo := app.SourceRepository{}
	repo.SetInfo(&i)
	repositories := app.SourceRepositories{&repo}

	resolvers := Resolvers{}
	componentrefs, err := AddMissingComponentsToRefBuilder(&app.ReferenceBuilder{}, repositories, resolvers.DockerfileResolver(), resolvers.SourceResolver(), resolvers.PipelineResolver(), &GenerationInputs{})

	checkResolveResult(t, componentrefs, err, generate.StrategyPipeline)
}

// TestResolveDockerfileAndSource ensures that if a repo has a Dockerfile and
// source, we use the Dockerfile.
func TestResolveDockerfileAndSource(t *testing.T) {
	dockerfile, _ := app.NewDockerfile("FROM centos\n")
	i := app.SourceRepositoryInfo{Dockerfile: dockerfile, Types: []app.SourceLanguageType{{Platform: "foo"}}}

	repo := app.SourceRepository{}
	repo.SetInfo(&i)
	repositories := app.SourceRepositories{&repo}

	resolvers := Resolvers{}
	componentrefs, err := AddMissingComponentsToRefBuilder(&app.ReferenceBuilder{}, repositories, resolvers.DockerfileResolver(), resolvers.SourceResolver(), resolvers.PipelineResolver(), &GenerationInputs{})

	checkResolveResult(t, componentrefs, err, generate.StrategyDocker)
}

func checkResolveResult(t *testing.T, componentrefs app.ComponentReferences, err error, strategy generate.Strategy) {
	if err != nil {
		t.Fatal(err)
	}

	if len(componentrefs) != 1 {
		t.Fatal("expected len(componentrefs) == 1")
	}

	if componentrefs[0].Input().Uses == nil {
		t.Fatal("expected non-nil componentrefs[0].Input().Uses")
	}

	if componentrefs[0].Input().Uses.GetStrategy() != strategy {
		t.Fatalf("expected componentrefs[0].Input().Uses.GetStrategy() == %s", strategy)
	}
}
