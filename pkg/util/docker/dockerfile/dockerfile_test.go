package dockerfile

import (
	"reflect"
	"strings"
	"testing"

	"github.com/docker/docker/builder/command"
	"github.com/docker/docker/builder/parser"
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
		if !reflect.DeepEqual(got, want) {
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
