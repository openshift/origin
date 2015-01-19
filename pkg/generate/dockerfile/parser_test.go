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
	p := NewParser()
	_, err := p.Parse(invalidDockerfile())

	if err == nil {
		t.Errorf("Expected error to be reported. No error was returned.")
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
