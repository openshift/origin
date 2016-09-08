package dockerfile

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/docker/docker/builder/dockerfile/command"
	"github.com/docker/docker/builder/dockerfile/parser"
)

// TestParseTreeToDockerfile tests calling ParseTreeToDockerfile with multiple
// valid inputs.
func TestParseTreeToDockerfile(t *testing.T) {
	testCases := map[string]struct {
		in   string
		want string
	}{
		"empty input": {
			in:   ``,
			want: ``,
		},
		"only comments": {
			in: `# This is a comment
# and this is another comment
	# while this is an indented comment`,
			want: ``,
		},
		"simple Dockerfile": {
			in: `FROM scratch
LABEL version=1.0
FROM busybox
ENV PATH=/bin
`,
			want: `FROM scratch
LABEL version=1.0
FROM busybox
ENV PATH=/bin
`,
		},
		"Dockerfile with comments": {
			in: `# This is a Dockerfile
FROM scratch
LABEL version=1.0
# Here we start building a second image
FROM busybox
ENV PATH=/bin
`,
			want: `FROM scratch
LABEL version=1.0
FROM busybox
ENV PATH=/bin
`,
		},
		"all Dockerfile instructions": {
			in: `FROM busybox:latest
MAINTAINER nobody@example.com
ONBUILD ADD . /app/src
ONBUILD RUN echo "Hello universe!"
LABEL version=1.0
EXPOSE 8080
VOLUME /var/run/www
ENV PATH=/bin
ADD file /home/
COPY dir/ /tmp/
RUN echo "Hello world!"
ENTRYPOINT /bin/sh
CMD ["-c", "env"]
USER 1001
WORKDIR /home
`,
			want: `FROM busybox:latest
MAINTAINER nobody@example.com
ONBUILD ADD . /app/src
ONBUILD RUN echo "Hello universe!"
LABEL version=1.0
EXPOSE 8080
VOLUME /var/run/www
ENV PATH=/bin
ADD file /home/
COPY dir/ /tmp/
RUN echo "Hello world!"
ENTRYPOINT /bin/sh
CMD ["-c", "env"]
USER 1001
WORKDIR /home
`,
		},
	}
	for name, tc := range testCases {
		node, err := parser.Parse(strings.NewReader(tc.in))
		if err != nil {
			t.Errorf("%s: parse error: %v", name, err)
			continue
		}
		got := ParseTreeToDockerfile(node)
		want := []byte(tc.want)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("ParseTreeToDockerfile: %s:\ngot:\n%swant:\n%s", name, got, want)
		}
	}
}

// TestParseTreeToDockerfileNilNode tests calling ParseTreeToDockerfile with a
// nil *parser.Node.
func TestParseTreeToDockerfileNilNode(t *testing.T) {
	got := ParseTreeToDockerfile(nil)
	if got != nil {
		t.Errorf("ParseTreeToDockerfile(nil) = %#v; want nil", got)
	}
}

// TestFindAll tests calling FindAll with multiple values of cmd.
func TestFindAll(t *testing.T) {
	instructions := `FROM scratch
LABEL version=1.0
FROM busybox
ENV PATH=/bin
`
	node, err := parser.Parse(strings.NewReader(instructions))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	for cmd, want := range map[string][]int{
		command.From:       {0, 2},
		command.Label:      {1},
		command.Env:        {3},
		command.Maintainer: nil,
		"UnknownCommand":   nil,
	} {
		got := FindAll(node, cmd)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("FindAll(node, %q) = %#v; want %#v", cmd, got, want)
		}
	}
}

// TestFindAllNilNode tests calling FindAll with a nil *parser.Node.
func TestFindAllNilNode(t *testing.T) {
	cmd := command.From
	got := FindAll(nil, cmd)
	if got != nil {
		t.Errorf("FindAll(nil, %q) = %#v; want nil", cmd, got)
	}
}

// TestInsertInstructions tests calling InsertInstructions with multiple valid
// combinations of input.
func TestInsertInstructions(t *testing.T) {
	testCases := map[string]struct {
		original        string
		index           int
		newInstructions string
		want            string
	}{
		"insert nothing": {
			original: `FROM busybox
ENV PATH=/bin
`,
			index:           0,
			newInstructions: ``,
			want: `FROM busybox
ENV PATH=/bin
`,
		},
		"insert instruction in empty file": {
			original:        ``,
			index:           0,
			newInstructions: `FROM busybox`,
			want: `FROM busybox
`,
		},
		"prepend single instruction": {
			original: `FROM busybox
ENV PATH=/bin
`,
			index:           0,
			newInstructions: `FROM scratch`,
			want: `FROM scratch
FROM busybox
ENV PATH=/bin
`,
		},
		"append single instruction": {
			original: `FROM busybox
ENV PATH=/bin
`,
			index:           2,
			newInstructions: `FROM scratch`,
			want: `FROM busybox
ENV PATH=/bin
FROM scratch
`,
		},
		"insert single instruction in the middle": {
			original: `FROM busybox
ENV PATH=/bin
`,
			index:           1,
			newInstructions: `LABEL version=1.0`,
			want: `FROM busybox
LABEL version=1.0
ENV PATH=/bin
`,
		},
	}
	for name, tc := range testCases {
		got, err := parser.Parse(strings.NewReader(tc.original))
		if err != nil {
			t.Errorf("InsertInstructions: %s: parse error: %v", name, err)
			continue
		}
		err = InsertInstructions(got, tc.index, tc.newInstructions)
		if err != nil {
			t.Errorf("InsertInstructions: %s: %v", name, err)
			continue
		}
		want, err := parser.Parse(strings.NewReader(tc.want))
		if err != nil {
			t.Errorf("InsertInstructions: %s: parse error: %v", name, err)
			continue
		}
		if !bytes.Equal(ParseTreeToDockerfile(got), ParseTreeToDockerfile(want)) {
			t.Errorf("InsertInstructions: %s: got %#v; want %#v", name, got, want)
		}
	}
}

// TestInsertInstructionsNilNode tests calling InsertInstructions with a nil
// *parser.Node.
func TestInsertInstructionsNilNode(t *testing.T) {
	err := InsertInstructions(nil, 0, "")
	if err == nil {
		t.Errorf("InsertInstructions: got nil; want error")
	}
}

// TestInsertInstructionsPosOutOfRange tests calling InsertInstructions with
// invalid values for the pos argument.
func TestInsertInstructionsPosOutOfRange(t *testing.T) {
	original := `FROM busybox
ENV PATH=/bin
`
	node, err := parser.Parse(strings.NewReader(original))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	for _, pos := range []int{-1, 3, 4} {
		err := InsertInstructions(node, pos, "")
		if err == nil {
			t.Errorf("InsertInstructions(node, %d, \"\"): got nil; want error", pos)
		}
	}
}

// TestInsertInstructionsUnparseable tests calling InsertInstructions with
// instructions that the Docker parser cannot handle.
func TestInsertInstructionsUnparseable(t *testing.T) {
	original := `FROM busybox
ENV PATH=/bin
`
	node, err := parser.Parse(strings.NewReader(original))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	for name, instructions := range map[string]string{
		"env without value": `ENV PATH`,
		"nested json":       `CMD [ "echo", [ "nested json" ] ]`,
	} {
		err = InsertInstructions(node, 1, instructions)
		if err == nil {
			t.Errorf("InsertInstructions: %s: got nil; want error", name)
		}
	}
}

// TestLastBaseImage tests calling LastBaseImage with multiple valid
// combinations of input.
func TestLastBaseImage(t *testing.T) {
	testCases := map[string]struct {
		in   string
		want string
	}{
		"empty Dockerfile": {
			in:   ``,
			want: "",
		},
		"FROM missing argument": {
			in:   `FROM`,
			want: "",
		},
		"single FROM": {
			in:   `FROM centos:7`,
			want: "centos:7",
		},
		"multiple FROM": {
			in: `FROM scratch
COPY . /boot
FROM centos:7`,
			want: "centos:7",
		},
	}
	for name, tc := range testCases {
		node, err := parser.Parse(strings.NewReader(tc.in))
		if err != nil {
			t.Errorf("%s: parse error: %v", name, err)
			continue
		}
		got := LastBaseImage(node)
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("LastBaseImage: %s: got %#v; want %#v", name, got, tc.want)
		}
	}
}

// TestLastBaseImageNilNode tests calling LastBaseImage with a nil *parser.Node.
func TestLastBaseImageNilNode(t *testing.T) {
	want := ""
	if got := LastBaseImage(nil); got != want {
		t.Errorf("LastBaseImage(nil) = %#v; want %#v", got, want)
	}
}

// TestBaseImages tests calling baseImages with multiple valid combinations of
// input.
func TestBaseImages(t *testing.T) {
	testCases := map[string]struct {
		in   string
		want []string
	}{
		"empty Dockerfile": {
			in:   ``,
			want: nil,
		},
		"FROM missing argument": {
			in:   `FROM`,
			want: nil,
		},
		"single FROM": {
			in:   `FROM centos:7`,
			want: []string{"centos:7"},
		},
		"multiple FROM": {
			in: `FROM scratch
COPY . /boot
FROM centos:7`,
			want: []string{"scratch", "centos:7"},
		},
	}
	for name, tc := range testCases {
		node, err := parser.Parse(strings.NewReader(tc.in))
		if err != nil {
			t.Errorf("%s: parse error: %v", name, err)
			continue
		}
		got := baseImages(node)
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("baseImages: %s: got %#v; want %#v", name, got, tc.want)
		}
	}
}

// TestBaseImagesNilNode tests calling baseImages with a nil *parser.Node.
func TestBaseImagesNilNode(t *testing.T) {
	if got := baseImages(nil); got != nil {
		t.Errorf("baseImages(nil) = %#v; want nil", got)
	}
}

// TestLastExposedPorts tests calling LastExposedPorts with multiple valid
// combinations of input.
func TestLastExposedPorts(t *testing.T) {
	testCases := map[string]struct {
		in   string
		want []string
	}{
		"empty Dockerfile": {
			in:   ``,
			want: nil,
		},
		"EXPOSE missing argument": {
			in:   `EXPOSE`,
			want: nil,
		},
		"EXPOSE no FROM": {
			in:   `EXPOSE 8080`,
			want: nil,
		},
		"single EXPOSE after FROM": {
			in: `FROM centos:7
		EXPOSE 8080`,
			want: []string{"8080"},
		},
		"multiple EXPOSE and FROM": {
			in: `# EXPOSE before FROM should be ignore
EXPOSE 777
FROM busybox
EXPOSE 8080
COPY . /boot
FROM rhel
# no EXPOSE instruction
FROM centos:7
EXPOSE 8000
EXPOSE 9090 9091
`,
			want: []string{"8000", "9090", "9091"},
		},
	}
	for name, tc := range testCases {
		node, err := parser.Parse(strings.NewReader(tc.in))
		if err != nil {
			t.Errorf("%s: parse error: %v", name, err)
			continue
		}
		got := LastExposedPorts(node)
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("LastExposedPorts: %s: got %#v; want %#v", name, got, tc.want)
		}
	}
}

// TestLastExposedPortsNilNode tests calling LastExposedPorts with a nil
// *parser.Node.
func TestLastExposedPortsNilNode(t *testing.T) {
	if got := LastExposedPorts(nil); got != nil {
		t.Errorf("LastExposedPorts(nil) = %#v; want nil", got)
	}
}

// TestExposedPorts tests calling exposedPorts with multiple valid combinations
// of input.
func TestExposedPorts(t *testing.T) {
	testCases := map[string]struct {
		in   string
		want [][]string
	}{
		"empty Dockerfile": {
			in:   ``,
			want: nil,
		},
		"EXPOSE missing argument": {
			in:   `EXPOSE`,
			want: nil,
		},
		"EXPOSE no FROM": {
			in:   `EXPOSE 8080`,
			want: nil,
		},
		"single EXPOSE after FROM": {
			in: `FROM centos:7
		EXPOSE 8080`,
			want: [][]string{{"8080"}},
		},
		"multiple EXPOSE and FROM": {
			in: `# EXPOSE before FROM should be ignore
EXPOSE 777
FROM busybox
EXPOSE 8080
COPY . /boot
FROM rhel
# no EXPOSE instruction
FROM centos:7
EXPOSE 8000
EXPOSE 9090 9091
`,
			want: [][]string{{"8080"}, nil, {"8000", "9090", "9091"}},
		},
	}
	for name, tc := range testCases {
		node, err := parser.Parse(strings.NewReader(tc.in))
		if err != nil {
			t.Errorf("%s: parse error: %v", name, err)
			continue
		}
		got := exposedPorts(node)
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("exposedPorts: %s: got %#v; want %#v", name, got, tc.want)
		}
	}
}

// TestExposedPortsNilNode tests calling exposedPorts with a nil *parser.Node.
func TestExposedPortsNilNode(t *testing.T) {
	if got := exposedPorts(nil); got != nil {
		t.Errorf("exposedPorts(nil) = %#v; want nil", got)
	}
}

// TestNextValues tests calling nextValues with multiple valid combinations of
// input.
func TestNextValues(t *testing.T) {
	testCases := map[string][]string{
		`FROM busybox:latest`:           {"busybox:latest"},
		`MAINTAINER nobody@example.com`: {"nobody@example.com"},
		`LABEL version=1.0`:             {"version", "1.0"},
		`EXPOSE 8080`:                   {"8080"},
		`VOLUME /var/run/www`:           {"/var/run/www"},
		`ENV PATH=/bin`:                 {"PATH", "/bin"},
		`ADD file /home/`:               {"file", "/home/"},
		`COPY dir/ /tmp/`:               {"dir/", "/tmp/"},
		`RUN echo "Hello world!"`:       {`echo "Hello world!"`},
		`ENTRYPOINT /bin/sh`:            {"/bin/sh"},
		`CMD ["-c", "env"]`:             {"-c", "env"},
		`USER 1001`:                     {"1001"},
		`WORKDIR /home`:                 {"/home"},
	}
	for original, want := range testCases {
		node, err := parser.Parse(strings.NewReader(original))
		if err != nil {
			t.Fatalf("parse error: %s: %v", original, err)
		}
		if len(node.Children) != 1 {
			t.Fatalf("unexpected number of children in test case: %s", original)
		}
		// The Docker parser always wrap instructions in a root node.
		// Look at the node representing the first instruction, the one
		// and only one in each test case.
		node = node.Children[0]
		if got := nextValues(node); !reflect.DeepEqual(got, want) {
			t.Errorf("nextValues(%+v) = %#v; want %#v", node, got, want)
		}
	}
}

// TestNextValuesOnbuild tests calling nextValues with ONBUILD instructions as
// input.
func TestNextValuesOnbuild(t *testing.T) {
	testCases := map[string][]string{
		`ONBUILD ADD . /app/src`:             {".", "/app/src"},
		`ONBUILD RUN echo "Hello universe!"`: {`echo "Hello universe!"`},
	}
	for original, want := range testCases {
		node, err := parser.Parse(strings.NewReader(original))
		if err != nil {
			t.Fatalf("parse error: %s: %v", original, err)
		}
		if len(node.Children) != 1 {
			t.Fatalf("unexpected number of children in test case: %s", original)
		}
		// The Docker parser always wrap instructions in a root node.
		// Look at the node representing the instruction following
		// ONBUILD, the one and only one in each test case.
		node = node.Children[0].Next
		if node == nil || len(node.Children) != 1 {
			t.Fatalf("unexpected number of children in ONBUILD instruction of test case: %s", original)
		}
		node = node.Children[0]
		if got := nextValues(node); !reflect.DeepEqual(got, want) {
			t.Errorf("nextValues(%+v) = %#v; want %#v", node, got, want)
		}
	}
}

// TestNextValuesNilNode tests calling nextValues with a nil *parser.Node.
func TestNextValuesNilNode(t *testing.T) {
	if got := nextValues(nil); got != nil {
		t.Errorf("nextValues(nil) = %#v; want nil", got)
	}
}
