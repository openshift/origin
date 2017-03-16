package imagebuilder

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"testing"

	"github.com/docker/docker/builder/dockerfile/parser"
	docker "github.com/fsouza/go-dockerclient"
)

func TestRun(t *testing.T) {
	f, err := os.Open("dockerclient/testdata/Dockerfile.add")
	if err != nil {
		t.Fatal(err)
	}
	node, err := parser.Parse(f)
	if err != nil {
		t.Fatal(err)
	}
	b := NewBuilder()
	from, err := b.From(node)
	if err != nil {
		t.Fatal(err)
	}
	if from != "busybox" {
		t.Fatalf("unexpected from: %s", from)
	}
	for _, child := range node.Children {
		step := b.Step()
		if err := step.Resolve(child); err != nil {
			t.Fatal(err)
		}
		if err := b.Run(step, LogExecutor); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("config: %#v", b.Config())
	t.Logf(node.Dump())
}

type testExecutor struct {
	Copies       []Copy
	Runs         []Run
	Configs      []docker.Config
	Unrecognized []Step
	Err          error
}

func (e *testExecutor) Copy(copies ...Copy) error {
	e.Copies = append(e.Copies, copies...)
	return e.Err
}
func (e *testExecutor) Run(run Run, config docker.Config) error {
	e.Runs = append(e.Runs, run)
	e.Configs = append(e.Configs, config)
	return e.Err
}
func (e *testExecutor) UnrecognizedInstruction(step *Step) error {
	e.Unrecognized = append(e.Unrecognized, *step)
	return e.Err
}

func TestBuilder(t *testing.T) {
	testCases := []struct {
		Dockerfile   string
		From         string
		Copies       []Copy
		Runs         []Run
		Unrecognized []Step
		Config       docker.Config
		ErrFn        func(err error) bool
	}{
		{
			Dockerfile: "dockerclient/testdata/dir/Dockerfile",
			From:       "busybox",
			Copies: []Copy{
				{Src: ".", Dest: []string{"/"}, Download: false},
				{Src: ".", Dest: []string{"/dir"}},
				{Src: "subdir/", Dest: []string{"/test/"}, Download: false},
			},
			Config: docker.Config{
				Image: "busybox",
			},
		},
		{
			Dockerfile: "dockerclient/testdata/ignore/Dockerfile",
			From:       "busybox",
			Copies: []Copy{
				{Src: ".", Dest: []string{"/"}},
			},
			Config: docker.Config{
				Image: "busybox",
			},
		},
		{
			Dockerfile: "dockerclient/testdata/Dockerfile.env",
			From:       "busybox",
			Config: docker.Config{
				Env:   []string{"name=value", "name2=value2a            value2b", "name1=value1", "name3=value3a\\n\"value3b\"", "name4=value4a\\\\nvalue4b"},
				Image: "busybox",
			},
		},
		{
			Dockerfile: "dockerclient/testdata/Dockerfile.edgecases",
			From:       "busybox",
			Copies: []Copy{
				{Src: ".", Dest: []string{"/"}, Download: true},
				{Src: ".", Dest: []string{"/test/copy"}},
			},
			Runs: []Run{
				{Shell: false, Args: []string{"ls", "-la"}},
				{Shell: false, Args: []string{"echo", "'1234'"}},
				{Shell: true, Args: []string{"echo \"1234\""}},
				{Shell: true, Args: []string{"echo 1234"}},
				{Shell: true, Args: []string{"echo '1234' &&     echo \"456\" &&     echo 789"}},
				{Shell: true, Args: []string{"sh -c 'echo root:testpass         > /tmp/passwd'"}},
				{Shell: true, Args: []string{"mkdir -p /test /test2 /test3/test"}},
			},
			Config: docker.Config{
				User:         "docker:root",
				ExposedPorts: map[docker.Port]struct{}{"6000/tcp": {}, "3000/tcp": {}, "9000/tcp": {}, "5000/tcp": {}},
				Env:          []string{"SCUBA=1 DUBA 3"},
				Cmd:          []string{"/bin/sh", "-c", "echo 'test' | wc -"},
				Image:        "busybox",
				Volumes:      map[string]struct{}{"/test2": {}, "/test3": {}, "/test": {}},
				WorkingDir:   "/test",
				OnBuild:      []string{"RUN [\"echo\", \"test\"]", "RUN echo test", "COPY . /"},
			},
		},
		{
			Dockerfile: "dockerclient/testdata/Dockerfile.unknown",
			From:       "busybox",
			Unrecognized: []Step{
				Step{Command: "health", Message: "HEALTH ", Original: "HEALTH NONE", Args: []string{""}, Flags: []string{}, Env: []string{}},
				Step{Command: "shell", Message: "SHELL ", Original: "SHELL [\"/bin/sh\", \"-c\"]", Args: []string{""}, Flags: []string{}, Env: []string{}},
				Step{Command: "unrecognized", Message: "UNRECOGNIZED ", Original: "UNRECOGNIZED", Args: []string{""}, Env: []string{}},
			},
			Config: docker.Config{
				Image: "busybox",
			},
		},
		{
			Dockerfile: "dockerclient/testdata/Dockerfile.exposedefault",
			From:       "busybox",
			Config: docker.Config{
				ExposedPorts: map[docker.Port]struct{}{"3469/tcp": {}},
				Image:        "busybox",
			},
		},
		{
			Dockerfile: "dockerclient/testdata/Dockerfile.add",
			From:       "busybox",
			Copies: []Copy{
				{Src: "https://github.com/openshift/origin/raw/master/README.md", Dest: []string{"/README.md"}, Download: true},
				{Src: "https://github.com/openshift/origin/raw/master/LICENSE", Dest: []string{"/"}, Download: true},
				{Src: "https://github.com/openshift/origin/raw/master/LICENSE", Dest: []string{"/A"}, Download: true},
				{Src: "https://github.com/openshift/origin/raw/master/LICENSE", Dest: []string{"/a"}, Download: true},
				{Src: "https://github.com/openshift/origin/raw/master/LICENSE", Dest: []string{"/b/a"}, Download: true},
				{Src: "https://github.com/openshift/origin/raw/master/LICENSE", Dest: []string{"/b/"}, Download: true},
				{Src: "https://github.com/openshift/ruby-hello-world/archive/master.zip", Dest: []string{"/tmp/"}, Download: true},
			},
			Runs: []Run{
				{Shell: true, Args: []string{"mkdir ./b"}},
			},
			Config: docker.Config{
				Image: "busybox",
				User:  "root",
			},
		},
	}
	for i, test := range testCases {
		data, err := ioutil.ReadFile(test.Dockerfile)
		if err != nil {
			t.Errorf("%d: %v", i, err)
			continue
		}
		node, err := parser.Parse(bytes.NewBuffer(data))
		if err != nil {
			t.Errorf("%d: %v", i, err)
			continue
		}
		b := NewBuilder()
		from, err := b.From(node)
		if err != nil {
			t.Errorf("%d: %v", i, err)
			continue
		}
		if from != test.From {
			t.Errorf("%d: unexpected FROM: %s", i, from)
		}
		e := &testExecutor{}
		var lastErr error
		for j, child := range node.Children {
			step := b.Step()
			if err := step.Resolve(child); err != nil {
				lastErr = fmt.Errorf("%d: %d: %s: resolve: %v", i, j, step.Original, err)
				break
			}
			if err := b.Run(step, e); err != nil {
				lastErr = fmt.Errorf("%d: %d: %s: run: %v", i, j, step.Original, err)
				break
			}
		}
		if lastErr != nil {
			if test.ErrFn == nil || !test.ErrFn(lastErr) {
				t.Errorf("%d: unexpected error: %v", i, lastErr)
			}
			continue
		}
		if !reflect.DeepEqual(test.Copies, e.Copies) {
			t.Errorf("%d: unexpected copies: %#v", i, e.Copies)
		}
		if !reflect.DeepEqual(test.Runs, e.Runs) {
			t.Errorf("%d: unexpected runs: %#v", i, e.Runs)
		}
		if !reflect.DeepEqual(test.Unrecognized, e.Unrecognized) {
			t.Errorf("%d: unexpected unrecognized: %#v", i, e.Unrecognized)
		}
		lastConfig := b.RunConfig
		if !reflect.DeepEqual(test.Config, lastConfig) {
			t.Errorf("%d: unexpected config: %#v", i, lastConfig)
		}
	}
}
