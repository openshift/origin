package imagebuilder

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	docker "github.com/fsouza/go-dockerclient"
)

func TestVolumeSet(t *testing.T) {
	testCases := []struct {
		inputs    []string
		changed   []bool
		result    []string
		covered   []string
		uncovered []string
	}{
		{
			inputs:  []string{"/var/lib", "/var"},
			changed: []bool{true, true},
			result:  []string{"/var"},

			covered:   []string{"/var/lib", "/var/", "/var"},
			uncovered: []string{"/var1", "/", "/va"},
		},
		{
			inputs:  []string{"/var", "/", "/"},
			changed: []bool{true, true, false},
			result:  []string{""},

			covered: []string{"/var/lib", "/var/", "/var", "/"},
		},
		{
			inputs:  []string{"/var", "/var/lib"},
			changed: []bool{true, false},
			result:  []string{"/var"},
		},
	}
	for i, testCase := range testCases {
		s := VolumeSet{}
		for j, path := range testCase.inputs {
			if s.Add(path) != testCase.changed[j] {
				t.Errorf("%d: adding %d %s should have resulted in change %t", i, j, path, testCase.changed[j])
			}
		}
		if !reflect.DeepEqual(testCase.result, []string(s)) {
			t.Errorf("%d: got %v", i, s)
		}
		for _, path := range testCase.covered {
			if !s.Covers(path) {
				t.Errorf("%d: not covered %s", i, path)
			}
		}
		for _, path := range testCase.uncovered {
			if s.Covers(path) {
				t.Errorf("%d: covered %s", i, path)
			}
		}
	}
}

func TestRun(t *testing.T) {
	f, err := os.Open("dockerclient/testdata/Dockerfile.add")
	if err != nil {
		t.Fatal(err)
	}
	node, err := ParseDockerfile(f)
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
		if err := b.Run(step, LogExecutor, false); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("config: %#v", b.Config())
	t.Logf(node.Dump())
}

type testExecutor struct {
	Preserved    []string
	Copies       []Copy
	Runs         []Run
	Configs      []docker.Config
	Unrecognized []Step
	Err          error
}

func (e *testExecutor) Preserve(path string) error {
	e.Preserved = append(e.Preserved, path)
	return e.Err
}

func (e *testExecutor) Copy(excludes []string, copies ...Copy) error {
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
		Args         map[string]string
		Dockerfile   string
		From         string
		Copies       []Copy
		Runs         []Run
		Unrecognized []Step
		Config       docker.Config
		Image        *docker.Image
		ErrFn        func(err error) bool
	}{
		{
			Dockerfile: "dockerclient/testdata/dir/Dockerfile",
			From:       "busybox",
			Copies: []Copy{
				{Src: []string{"."}, Dest: "/", Download: false},
				{Src: []string{"."}, Dest: "/dir"},
				{Src: []string{"subdir/"}, Dest: "/test/", Download: false},
			},
			Config: docker.Config{
				Image: "busybox",
			},
		},
		{
			Dockerfile: "dockerclient/testdata/ignore/Dockerfile",
			From:       "busybox",
			Copies: []Copy{
				{Src: []string{"."}, Dest: "/"},
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
				{Src: []string{"."}, Dest: "/", Download: true},
				{Src: []string{"."}, Dest: "/test/copy"},
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
				Step{Command: "shell", Message: "SHELL /bin/sh -c", Original: "SHELL [\"/bin/sh\", \"-c\"]", Args: []string{"/bin/sh", "-c"}, Flags: []string{}, Env: []string{}, Attrs: map[string]bool{"json": true}},
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
				{Src: []string{"https://github.com/openshift/origin/raw/master/README.md"}, Dest: "/README.md", Download: true},
				{Src: []string{"https://github.com/openshift/origin/raw/master/LICENSE"}, Dest: "/", Download: true},
				{Src: []string{"https://github.com/openshift/origin/raw/master/LICENSE"}, Dest: "/A", Download: true},
				{Src: []string{"https://github.com/openshift/origin/raw/master/LICENSE"}, Dest: "/a", Download: true},
				{Src: []string{"https://github.com/openshift/origin/raw/master/LICENSE"}, Dest: "/b/a", Download: true},
				{Src: []string{"https://github.com/openshift/origin/raw/master/LICENSE"}, Dest: "/b/", Download: true},
				{Src: []string{"https://github.com/openshift/ruby-hello-world/archive/master.zip"}, Dest: "/tmp/", Download: true},
			},
			Runs: []Run{
				{Shell: true, Args: []string{"mkdir ./b"}},
			},
			Config: docker.Config{
				Image: "busybox",
				User:  "root",
			},
		},
		{
			Dockerfile: "dockerclient/testdata/Dockerfile.badhealthcheck",
			From:       "debian",
			Config: docker.Config{
				Image: "busybox",
			},
			ErrFn: func(err error) bool {
				return err != nil && strings.Contains(err.Error(), "HEALTHCHECK requires at least one argument")
			},
		},
		{
			Dockerfile: "dockerclient/testdata/Dockerfile.healthcheck",
			From:       "debian",
			Config: docker.Config{
				Image: "debian",
				Cmd:   []string{"/bin/sh", "-c", "/app/main.sh"},
				Healthcheck: &docker.HealthConfig{
					Interval: 5 * time.Second,
					Timeout:  3 * time.Second,
					Retries:  3,
					Test:     []string{"CMD-SHELL", "/app/check.sh --quiet"},
				},
			},
		},
		{
			Dockerfile: "dockerclient/testdata/Dockerfile.envsubst",
			From:       "busybox",
			Image: &docker.Image{
				ID: "busybox2",
				Config: &docker.Config{
					Env: []string{"FOO=another", "BAR=original"},
				},
			},
			Config: docker.Config{
				Env:    []string{"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin", "FOO=value"},
				Labels: map[string]string{"test": "value"},
			},
		},
		{
			Dockerfile: "dockerclient/testdata/Dockerfile.args",
			Args:       map[string]string{"BAR": "first"},
			From:       "busybox",
			Config: docker.Config{
				Image:  "busybox",
				Env:    []string{"FOO=value", "TEST=", "BAZ=first"},
				Labels: map[string]string{"test": "value"},
			},
			Runs: []Run{
				{Shell: true, Args: []string{"echo $BAR"}},
			},
		},
		{
			Dockerfile: "dockerclient/testdata/volume/Dockerfile",
			From:       "busybox",
			Image: &docker.Image{
				ID:     "busybox2",
				Config: &docker.Config{},
			},
			Config: docker.Config{
				Env: []string{"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"},
				Volumes: map[string]struct{}{
					"/var":     struct{}{},
					"/var/www": struct{}{},
				},
			},
			Copies: []Copy{
				{Src: []string{"file"}, Dest: "/var/www/", Download: true},
				{Src: []string{"file"}, Dest: "/var/", Download: true},
				{Src: []string{"file2"}, Dest: "/var/", Download: true},
			},
		},
		{
			Dockerfile: "dockerclient/testdata/volumerun/Dockerfile",
			From:       "busybox",
			Config: docker.Config{
				Image: "busybox",
				Volumes: map[string]struct{}{
					"/var/www": struct{}{},
				},
			},
			Runs: []Run{
				{Shell: true, Args: []string{"touch /var/www/file3"}},
			},
			Copies: []Copy{
				{Src: []string{"file"}, Dest: "/var/www/", Download: true},
				{Src: []string{"file2"}, Dest: "/var/www/", Download: true},
				{Src: []string{"file4"}, Dest: "/var/www/", Download: true},
			},
		},
	}
	for i, test := range testCases {
		data, err := ioutil.ReadFile(test.Dockerfile)
		if err != nil {
			t.Errorf("%d: %v", i, err)
			continue
		}
		node, err := ParseDockerfile(bytes.NewBuffer(data))
		if err != nil {
			t.Errorf("%d: %v", i, err)
			continue
		}
		b := NewBuilder()
		b.Args = test.Args
		from, err := b.From(node)
		if err != nil {
			t.Errorf("%d: %v", i, err)
			continue
		}
		if from != test.From {
			t.Errorf("%d: unexpected FROM: %s", i, from)
		}
		if test.Image != nil {
			if err := b.FromImage(test.Image, node); err != nil {
				t.Errorf("%d: unexpected error: %v", i, err)
			}
		}

		e := &testExecutor{}
		var lastErr error
		for j, child := range node.Children {
			step := b.Step()
			if err := step.Resolve(child); err != nil {
				lastErr = fmt.Errorf("%d: %d: %s: resolve: %v", i, j, step.Original, err)
				break
			}
			if err := b.Run(step, e, false); err != nil {
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
			data, _ := json.Marshal(lastConfig)
			t.Errorf("%d: unexpected config: %s", i, string(data))
		}
	}
}
