package dockerfile

import (
	"bytes"
	"io"
	"reflect"
	"testing"
)

func TestParse(t *testing.T) {
	p := NewParser()
	d, err := p.Parse(testDockerfile())
	if err != nil {
		t.Errorf("Unexpected error parsing docker file: %v", err)
	}
	from, ok := d.GetDirective("FROM")
	if !ok {
		t.Errorf("Did not find expected directive FROM")
	}
	if from[0] != "ubuntu:14.04" {
		t.Errorf("Did not get expected FROM directive value: %s", from[0])
	}
	run, ok := d.GetDirective("run")
	expectedResult := []string{
		"echo hello  world  goodnight    moon  lightning",
		"echo hello    world",
		"echo hello  world",
		"echo hello goodbyefrog",
		"echo hello  world",
		"echo hi   world  goodnight",
		"echo goodbyefrog",
		"echo goodbyefrog",
		"echo hello this is some more useful stuff",
	}
	if !ok {
		t.Errorf("Did not get expected directive RUN")
	}
	if !reflect.DeepEqual(expectedResult, run) {
		t.Errorf("Did not expected RUN directive values: %#v. Actual: %#v", expectedResult, run)
	}
}

func TestParseInvalidFile(t *testing.T) {
	tests := []struct {
		name string
		df   io.Reader
	}{
		{
			name: "invalidated by us",
			df:   invalidDockerfile(),
		},
		{
			name: "invalidated by the Docker parser",
			df:   invalidDockerfile2(),
		},
	}

	p := NewParser()
	for _, test := range tests {
		if _, err := p.Parse(test.df); err == nil {
			t.Errorf("%s: Expected error to be reported. No error was returned.", test.name)
		}
	}
}

func testDockerfile() io.Reader {
	content := `FROM ubuntu:14.04
RUN echo hello\
  world\
  goodnight  \
  moon\
  light\
ning
RUN echo hello  \
  world
RUN echo hello  \
world
RUN echo hello \
goodbye\
frog
RUN echo hello  \  
world
RUN echo hi \
 \
 world \
\
 good\
\
night
RUN echo goodbye\
frog
RUN echo good\
bye\
frog

RUN echo hello \
# this is a comment

# this is a comment with a blank line surrounding it

this is some more useful stuff`

	return bytes.NewBufferString(content)

}

func invalidDockerfile() io.Reader {
	content := `FROM test
TESTERROR`
	return bytes.NewBufferString(content)
}

func invalidDockerfile2() io.Reader {
	content := `FROM test
ENV keyMissingValue`
	return bytes.NewBufferString(content)
}
